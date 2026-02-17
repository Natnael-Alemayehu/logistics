//go:build integration
// +build integration

package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/pkg/hash"
	"github.com/natnael-alemayehu/logistics/pkg/jwt"
	"github.com/testcontainers/testcontainers-go"
	postgresdriver "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func toUUID(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{}
	}
	var u pgtype.UUID
	u.Scan(s)
	return u
}

func toInt32(i int) *int32 {
	v := int32(i)
	return &v
}

func generateTestKeys() (*rsa.PrivateKey, *rsa.PublicKey) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	return privateKey, &privateKey.PublicKey
}

type TestEnv struct {
	pool       *pgxpool.Pool
	queries    *db.Queries
	container  testcontainers.Container
	jwtManager *jwt.JWTManager
}

func SetupTestEnv(t *testing.T) *TestEnv {
	ctx := context.Background()

	container, err := postgresdriver.Run(ctx,
		"postgres:15-alpine",
		postgresdriver.WithDatabase("logistics_test"),
		postgresdriver.WithUsername("test"),
		postgresdriver.WithPassword("test"),
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

	queries := db.New(pool)

	privateKey, publicKey := generateTestKeys()
	jwtManager := jwt.NewManager(privateKey, publicKey, "test.logistics.et", time.Hour, 24*time.Hour)

	return &TestEnv{
		pool:       pool,
		queries:    queries,
		container:  container,
		jwtManager: jwtManager,
	}
}

func (e *TestEnv) Cleanup() {
	if e.pool != nil {
		e.pool.Close()
	}
	if e.container != nil {
		e.container.Terminate(context.Background())
	}
}

func (e *TestEnv) CreateTestTenant(ctx context.Context, slug string) db.Tenant {
	tenant, err := e.queries.CreateTenant(ctx, db.CreateTenantParams{
		Name:       fmt.Sprintf("Test Tenant %s", slug),
		Slug:       slug,
		Plan:       "starter",
		MaxDrivers: toInt32(5),
	})
	if err != nil {
		panic(err)
	}
	return tenant
}

func (e *TestEnv) CreateTestDriver(ctx context.Context, tenantID string, phone string) db.User {
	pinHash, _ := hash.PIN("1234")
	isActive := true
	user, err := e.queries.CreateUser(ctx, db.CreateUserParams{
		TenantID: toUUID(tenantID),
		Role:     "driver",
		FullName: "Test Driver",
		Phone:    &phone,
		PinHash:  &pinHash,
		IsActive: &isActive,
	})
	if err != nil {
		panic(err)
	}
	return user
}

func (e *TestEnv) CreateTestDispatcher(ctx context.Context, tenantID string, email string) db.User {
	passwordHash, _ := hash.Password("password123")
	isActive := true
	user, err := e.queries.CreateUser(ctx, db.CreateUserParams{
		TenantID:     toUUID(tenantID),
		Role:         "dispatcher",
		FullName:     "Test Dispatcher",
		Email:        &email,
		PasswordHash: &passwordHash,
		IsActive:     &isActive,
	})
	if err != nil {
		panic(err)
	}
	return user
}

func (e *TestEnv) CreateAuthService() *service.AuthService {
	auditService := service.NewAuditService(e.queries)
	return service.NewAuthService(e.queries, e.jwtManager, auditService)
}

func (e *TestEnv) CreateShipmentService() *service.ShipmentService {
	auditService := service.NewAuditService(e.queries)
	return service.NewShipmentService(e.queries, auditService)
}

func (e *TestEnv) CreateVehicleService() *service.VehicleService {
	return service.NewVehicleService(e.queries)
}

func TestHealthCheck(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	err := env.pool.Ping(ctx)
	if err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}
}

func TestDriverLogin(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "test-driver-login")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0912345678")

	authService := env.CreateAuthService()

	result, err := authService.DriverLogin(ctx, service.DriverLoginInput{
		Phone: "0912345678",
		PIN:   "1234",
	}, "127.0.0.1", "test-agent")

	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	if result.User.ID != driver.ID.String() {
		t.Errorf("expected user ID %s, got %s", driver.ID.String(), result.User.ID)
	}

	if result.AccessToken == "" {
		t.Error("expected access token to be set")
	}

	if result.RefreshToken == "" {
		t.Error("expected refresh token to be set")
	}
}

func TestDispatcherLogin(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "test-dispatcher-login")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "dispatcher@test.com")

	authService := env.CreateAuthService()

	result, err := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "dispatcher@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")

	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	if result.User.ID != dispatcher.ID.String() {
		t.Errorf("expected user ID %s, got %s", dispatcher.ID.String(), result.User.ID)
	}
}

func TestFailedLoginAccountLockout(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "test-lockout")
	_ = env.CreateTestDispatcher(ctx, tenant.ID.String(), "lockout@test.com")

	authService := env.CreateAuthService()

	for i := 0; i < 5; i++ {
		_, err := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
			Email:    "lockout@test.com",
			Password: "wrongpassword",
		}, "127.0.0.1", "test-agent")

		if err == nil {
			t.Fatal("expected login to fail with wrong password")
		}
	}

	_, err := authService.DispatcherLogin(ctx, service.DispatcherLoginInput{
		Email:    "lockout@test.com",
		Password: "password123",
	}, "127.0.0.1", "test-agent")

	if err != service.ErrAccountLocked {
		t.Errorf("expected account locked error, got: %v", err)
	}
}

func TestShipmentLifecycle(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "test-shipment-lifecycle")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "ship-test@test.com")
	driver := env.CreateTestDriver(ctx, tenant.ID.String(), "0987654321")

	shipmentService := env.CreateShipmentService()

	shipment, err := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID.String(), service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Dire Dawa",
		CustomerName:       "Test Customer",
		CustomerPhone:      "0911111111",
	}, "127.0.0.1", "test-agent")

	if err != nil {
		t.Fatalf("failed to create shipment: %v", err)
	}

	if shipment.Status != "pending" {
		t.Errorf("expected status pending, got %s", shipment.Status)
	}

	assigned, err := shipmentService.AssignDriver(ctx, tenant.ID.String(), dispatcher.ID.String(), shipment.ID, driver.ID.String(), "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to assign driver: %v", err)
	}

	if assigned.Status != "assigned" {
		t.Errorf("expected status assigned, got %s", assigned.Status)
	}

	inTransit, err := shipmentService.UpdateStatus(ctx, tenant.ID.String(), dispatcher.ID.String(), shipment.ID, service.UpdateStatusInput{
		Status: "in_transit",
	}, "127.0.0.1", "test-agent")
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	if inTransit.Status != "in_transit" {
		t.Errorf("expected status in_transit, got %s", inTransit.Status)
	}
}

func TestShipmentCancellation(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "test-cancellation")
	dispatcher := env.CreateTestDispatcher(ctx, tenant.ID.String(), "cancel-test@test.com")

	shipmentService := env.CreateShipmentService()

	shipment, err := shipmentService.Create(ctx, tenant.ID.String(), dispatcher.ID.String(), service.CreateShipmentInput{
		OriginAddress:      "Addis Ababa",
		DestinationAddress: "Bahir Dar",
		CustomerName:       "Cancel Test",
		CustomerPhone:      "0922222222",
	}, "127.0.0.1", "test-agent")

	if err != nil {
		t.Fatalf("failed to create shipment: %v", err)
	}

	cancelled, err := shipmentService.Cancel(ctx, tenant.ID.String(), dispatcher.ID.String(), shipment.ID, service.CancelShipmentInput{
		Reason: "Customer requested cancellation",
	}, "127.0.0.1", "test-agent")

	if err != nil {
		t.Fatalf("failed to cancel shipment: %v", err)
	}

	if cancelled.Status != "cancelled" {
		t.Errorf("expected status cancelled, got %s", cancelled.Status)
	}

	if cancelled.StatusReason != "Customer requested cancellation" {
		t.Errorf("expected cancellation reason, got %s", cancelled.StatusReason)
	}
}

func TestVehicleManagement(t *testing.T) {
	env := SetupTestEnv(t)
	defer env.Cleanup()

	ctx := context.Background()
	tenant := env.CreateTestTenant(ctx, "test-vehicle")

	vehicleService := env.CreateVehicleService()

	vehicle, err := vehicleService.Create(ctx, tenant.ID.String(), service.CreateVehicleInput{
		PlateNumber: "AA-1234-A",
		VehicleType: "truck",
	})

	if err != nil {
		t.Fatalf("failed to create vehicle: %v", err)
	}

	if vehicle.PlateNumber != "AA-1234-A" {
		t.Errorf("expected plate number AA-1234-A, got %s", vehicle.PlateNumber)
	}

	fetched, err := vehicleService.GetByID(ctx, tenant.ID.String(), vehicle.ID)
	if err != nil {
		t.Fatalf("failed to get vehicle: %v", err)
	}

	if fetched.ID != vehicle.ID {
		t.Errorf("expected vehicle ID %s, got %s", vehicle.ID, fetched.ID)
	}

	updated, err := vehicleService.Update(ctx, tenant.ID.String(), vehicle.ID, service.UpdateVehicleInput{
		VehicleType: "van",
	})
	if err != nil {
		t.Fatalf("failed to update vehicle: %v", err)
	}

	if updated.VehicleType != "van" {
		t.Errorf("expected vehicle type van, got %s", updated.VehicleType)
	}
}
