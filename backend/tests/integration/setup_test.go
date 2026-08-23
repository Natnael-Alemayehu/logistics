//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/handler"
	"github.com/natnael-alemayehu/logistics/internal/middleware"
	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/pkg/hash"
	"github.com/natnael-alemayehu/logistics/pkg/jwt"
	"github.com/natnael-alemayehu/logistics/pkg/storage"
	"github.com/natnael-alemayehu/logistics/pkg/validation"
	"github.com/rs/zerolog"
	"github.com/testcontainers/testcontainers-go"
	postgresdriver "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func toUUID(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{}
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

func toInt32(i int) *int32 {
	v := int32(i)
	return &v
}

// apiPrefix is the version prefix every application route is mounted under.
// Tests drive the real router, so request paths must include it.
const apiPrefix = "/api/v1"

// migrationsDir is the production migration directory, resolved relative to
// this package. Tests run against the same files the API migrates with so the
// two schemas cannot drift.
const migrationsDir = "../../migrations"

type TestEnv struct {
	pool       *pgxpool.Pool
	queries    *db.Queries
	container  testcontainers.Container
	jwtManager *jwt.JWTManager
	router     *chi.Mux
	handler    *handler.Handler
	podStorage *service.PODStorageService
	t          *testing.T
	cleanupFns []func()
}

type TestUser struct {
	ID       string
	TenantID string
	Role     string
	Phone    string
	Email    string
	IsActive bool
}

type TestShipment struct {
	ID             string
	TenantID       string
	TrackingNumber string
	Status         string
	DriverID       string
}

type TestClaims struct {
	UserID    string
	TenantID  string
	SessionID string
	Role      string
}

func SetupTestEnv(t *testing.T) *TestEnv {
	ctx := context.Background()

	container, err := postgresdriver.Run(ctx,
		"postgis/postgis:15-3.3-alpine",
		postgresdriver.WithDatabase("logistics_test"),
		postgresdriver.WithUsername("test"),
		postgresdriver.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	env := &TestEnv{
		pool:      pool,
		queries:   db.New(pool),
		container: container,
		t:         t,
	}

	env.runMigrations(ctx)
	env.setupServices()

	return env
}

func (e *TestEnv) runMigrations(ctx context.Context) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		e.t.Fatalf("failed to read migrations directory: %v", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		content, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			e.t.Fatalf("failed to read migration %s: %v", name, err)
		}
		if _, err := e.pool.Exec(ctx, gooseUp(string(content))); err != nil {
			e.t.Fatalf("failed to execute migration %s: %v", name, err)
		}
	}
}

// gooseUp returns the Up half of a goose migration. The directives themselves
// are ordinary SQL comments and pass through harmlessly, but the Down half must
// be discarded: executing it would tear down the schema Up just created.
func gooseUp(migration string) string {
	if i := strings.Index(migration, "-- +goose Down"); i >= 0 {
		return migration[:i]
	}
	return migration
}

func (e *TestEnv) setupServices() {
	validation.Init()
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	e.jwtManager = jwt.NewManager(privateKey, &privateKey.PublicKey, "test.logistics.et", time.Hour, 24*time.Hour)

	auditService := service.NewAuditService(e.queries)
	authService := service.NewAuthService(e.queries, e.jwtManager, auditService)
	shipmentService := service.NewShipmentService(e.queries, auditService, nil, nil)

	// Real local-filesystem storage rooted in the test's temp dir, so the POD
	// upload path is exercised rather than stubbed out.
	podStore, err := storage.NewLocalStorage(e.t.TempDir())
	if err != nil {
		e.t.Fatalf("failed to create POD storage: %v", err)
	}
	e.podStorage = service.NewPODStorageServiceWithStorage(e.queries, podStore)

	syncService := service.NewSyncService(e.queries, shipmentService, nil, e.podStorage, zerolog.Nop())
	userService := service.NewUserService(e.queries, auditService)
	vehicleService := service.NewVehicleService(e.queries)
	trackingService := service.NewTrackingService(e.queries, auditService)
	driverService := service.NewDriverService(e.queries, e.pool)
	deviceTokenService := service.NewDeviceTokenService(e.queries)

	e.handler = handler.New(authService, shipmentService, syncService, userService, vehicleService, trackingService, nil, nil, driverService, deviceTokenService)
	e.router = e.handler.Routes(zerolog.Nop(), e.jwtManager, nil, e.queries)
}

func (e *TestEnv) Cleanup() {
	for _, fn := range e.cleanupFns {
		fn()
	}
	if e.pool != nil {
		e.pool.Close()
	}
	if e.container != nil {
		e.container.Terminate(context.Background())
	}
}

func (e *TestEnv) RegisterCleanup(fn func()) {
	e.cleanupFns = append(e.cleanupFns, fn)
}

func (e *TestEnv) CreateTestTenant(ctx context.Context, slug string) db.Tenant {
	tenant, err := e.queries.CreateTenant(ctx, db.CreateTenantParams{
		Name:       fmt.Sprintf("Test Tenant %s", slug),
		Slug:       slug,
		Plan:       "starter",
		MaxDrivers: toInt32(5),
	})
	if err != nil {
		e.t.Fatalf("failed to create tenant: %v", err)
	}
	return tenant
}

func (e *TestEnv) CreateTestDriver(ctx context.Context, tenantID, phone string) TestUser {
	pinHash, _ := hash.PIN("1234")
	isActive := true
	user, err := e.queries.CreateUser(ctx, db.CreateUserParams{
		TenantID: toUUID(tenantID),
		Role:     "driver",
		FullName: fmt.Sprintf("Test Driver %s", phone),
		Phone:    &phone,
		PinHash:  &pinHash,
		IsActive: &isActive,
	})
	if err != nil {
		e.t.Fatalf("failed to create driver: %v", err)
	}
	return TestUser{
		ID:       user.ID.String(),
		TenantID: user.TenantID.String(),
		Role:     "driver",
		Phone:    phone,
		IsActive: true,
	}
}

func (e *TestEnv) CreateTestDispatcher(ctx context.Context, tenantID, email string) TestUser {
	passwordHash, _ := hash.Password("password123")
	isActive := true
	user, err := e.queries.CreateUser(ctx, db.CreateUserParams{
		TenantID:     toUUID(tenantID),
		Role:         "dispatcher",
		FullName:     fmt.Sprintf("Test Dispatcher %s", email),
		Email:        &email,
		PasswordHash: &passwordHash,
		IsActive:     &isActive,
	})
	if err != nil {
		e.t.Fatalf("failed to create dispatcher: %v", err)
	}
	return TestUser{
		ID:       user.ID.String(),
		TenantID: user.TenantID.String(),
		Role:     "dispatcher",
		Email:    email,
		IsActive: true,
	}
}

func (e *TestEnv) CreateTestAdmin(ctx context.Context, tenantID, email string) TestUser {
	passwordHash, _ := hash.Password("admin123")
	isActive := true
	user, err := e.queries.CreateUser(ctx, db.CreateUserParams{
		TenantID:     toUUID(tenantID),
		Role:         "admin",
		FullName:     fmt.Sprintf("Test Admin %s", email),
		Email:        &email,
		PasswordHash: &passwordHash,
		IsActive:     &isActive,
	})
	if err != nil {
		e.t.Fatalf("failed to create admin: %v", err)
	}
	return TestUser{
		ID:       user.ID.String(),
		TenantID: user.TenantID.String(),
		Role:     "admin",
		Email:    email,
		IsActive: true,
	}
}

func (e *TestEnv) CreateTestShipment(ctx context.Context, tenantID, createdBy string, driverID ...string) TestShipment {
	trackingNumber := fmt.Sprintf("ET-%s-%04d", time.Now().Format("20060102"), time.Now().Nanosecond()/100000)

	var driverUUID pgtype.UUID
	if len(driverID) > 0 && driverID[0] != "" {
		driverUUID = toUUID(driverID[0])
	}

	shipment, err := e.queries.CreateShipment(ctx, db.CreateShipmentParams{
		TenantID:           toUUID(tenantID),
		TrackingNumber:     trackingNumber,
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Dire Dawa",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0911111111",
		Status:             "pending",
		DriverID:           driverUUID,
		CreatedBy:          toUUID(createdBy),
	})
	if err != nil {
		e.t.Fatalf("failed to create shipment: %v", err)
	}
	return TestShipment{
		ID:             shipment.ID.String(),
		TenantID:       shipment.TenantID.String(),
		TrackingNumber: shipment.TrackingNumber,
		Status:         shipment.Status,
	}
}

func (e *TestEnv) CreateTestVehicle(ctx context.Context, tenantID, plateNumber string) db.Vehicle {
	vehicle, err := e.queries.CreateVehicle(ctx, db.CreateVehicleParams{
		TenantID:    toUUID(tenantID),
		PlateNumber: plateNumber,
	})
	if err != nil {
		e.t.Fatalf("failed to create vehicle: %v", err)
	}
	return vehicle
}

func (e *TestEnv) CreateTestSession(ctx context.Context, userID, tenantID string) string {
	refreshToken, _ := e.jwtManager.GenerateRefreshToken(userID, tenantID)
	session, err := e.queries.CreateSession(ctx, db.CreateSessionParams{
		UserID:           toUUID(userID),
		TenantID:         toUUID(tenantID),
		RefreshTokenHash: refreshToken,
		ExpiresAt:        pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	})
	if err != nil {
		e.t.Fatalf("failed to create session: %v", err)
	}
	return session.ID.String()
}

func (e *TestEnv) GenerateTestToken(user TestUser, sessionID string) string {
	token, err := e.jwtManager.GenerateAccessToken(
		user.ID,
		user.TenantID,
		sessionID,
		user.Role,
		user.Phone,
		user.Email,
	)
	if err != nil {
		e.t.Fatalf("failed to generate test token: %v", err)
	}
	return token
}

func (e *TestEnv) GetAuthService() *service.AuthService {
	auditService := service.NewAuditService(e.queries)
	return service.NewAuthService(e.queries, e.jwtManager, auditService)
}

func (e *TestEnv) GetShipmentService() *service.ShipmentService {
	auditService := service.NewAuditService(e.queries)
	return service.NewShipmentService(e.queries, auditService, nil, nil)
}

func (e *TestEnv) GetSyncService() *service.SyncService {
	shipmentService := e.GetShipmentService()
	return service.NewSyncService(e.queries, shipmentService, nil, e.podStorage, zerolog.Nop())
}

func (e *TestEnv) GetPODStorageService() *service.PODStorageService {
	return e.podStorage
}

func (e *TestEnv) GetQueries() *db.Queries {
	return e.queries
}

func (e *TestEnv) GetRouter() *chi.Mux {
	return e.router
}

func (e *TestEnv) GetHandler() *handler.Handler {
	return e.handler
}

func (e *TestEnv) GetJWTManager() *jwt.JWTManager {
	return e.jwtManager
}

func (e *TestEnv) CreateContextWithClaims(claims TestClaims) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, middleware.UserIDKey, claims.UserID)
	ctx = context.WithValue(ctx, middleware.TenantIDKey, claims.TenantID)
	ctx = context.WithValue(ctx, middleware.SessionIDKey, claims.SessionID)
	ctx = context.WithValue(ctx, middleware.RoleKey, claims.Role)
	return ctx
}

func (e *TestEnv) LockUserAccount(ctx context.Context, userID string) {
	lockUntil := time.Now().Add(15 * time.Minute)
	err := e.queries.LockUserAccount(ctx, db.LockUserAccountParams{
		ID:          toUUID(userID),
		LockedUntil: pgtype.Timestamptz{Time: lockUntil, Valid: true},
	})
	if err != nil {
		e.t.Fatalf("failed to lock user account: %v", err)
	}
}

func (e *TestEnv) IncrementFailedAttempts(ctx context.Context, userID string, attempts int) {
	for i := 0; i < attempts; i++ {
		_, err := e.queries.IncrementFailedLoginAttempts(ctx, toUUID(userID))
		if err != nil {
			e.t.Fatalf("failed to increment failed attempts: %v", err)
		}
	}
}

func (e *TestEnv) GetUserByID(ctx context.Context, userID string) db.User {
	user, err := e.queries.GetUserByID(ctx, db.GetUserByIDParams{
		ID: toUUID(userID),
	})
	if err != nil {
		e.t.Fatalf("failed to get user: %v", err)
	}
	return user
}

func (e *TestEnv) AssignDriverToShipment(ctx context.Context, tenantID, shipmentID, driverID string) {
	_, err := e.queries.AssignDriverToShipment(ctx, db.AssignDriverToShipmentParams{
		ID:       toUUID(shipmentID),
		TenantID: toUUID(tenantID),
		DriverID: toUUID(driverID),
	})
	if err != nil {
		e.t.Fatalf("failed to assign driver: %v", err)
	}
}

func (e *TestEnv) UpdateShipmentStatus(ctx context.Context, tenantID, shipmentID, status string) {
	_, err := e.queries.UpdateShipmentStatus(ctx, db.UpdateShipmentStatusParams{
		ID:       toUUID(shipmentID),
		TenantID: toUUID(tenantID),
		Status:   status,
	})
	if err != nil {
		e.t.Fatalf("failed to update shipment status: %v", err)
	}
}
