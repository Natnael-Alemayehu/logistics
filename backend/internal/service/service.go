// Package service holds the business rules: what may happen to a shipment,
// who may do it, and what follows from it (history, alerts, SMS, live
// updates). Handlers translate HTTP into calls here and render the result.
package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/pkg/storage"
)

// Settings are the deployment-wide values the business layer needs.
type Settings struct {
	// AppName is the product name used in SMS and page titles.
	AppName string
	// BaseURL is the public origin, without a trailing slash, used to build
	// links sent by SMS.
	BaseURL string
	// Location is the operating time zone for "today", reports and SMS times.
	Location *time.Location
	// Secret keys the HMAC that protects one-time codes and lookup cookies.
	Secret []byte
	// SMSNumber is the number or short code drivers text status updates to.
	SMSNumber string
}

// Publisher delivers live updates to dashboards. The WebSocket hub
// implements it.
type Publisher interface {
	BroadcastToTenant(tenantID string, message []byte)
}

// Deps is everything the services share.
type Deps struct {
	Pool      *pgxpool.Pool
	Log       zerolog.Logger
	Publisher Publisher
	Storage   storage.Storage
	SMS       SMSSender
	Mail      MailSender
	Settings  Settings
	// Now is the clock, replaceable in tests.
	Now func() time.Time
}

// Services is the business layer.
type Services struct {
	Auth      *AuthService
	Shipments *ShipmentService
	Users     *UserService
	Vehicles  *VehicleService
	Tenants   *TenantService
	Alerts    *AlertService
	SMS       *SMSService
	Mail      *MailService
	Reports   *ReportService
	Tracking  *TrackingService
	Sync      *SyncService
	Media     *MediaService
	Jobs      *Jobs
	Audit     *AuditService
}

// New wires the services together.
func New(d Deps) *Services {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Settings.Location == nil {
		d.Settings.Location = time.UTC
	}
	if d.Publisher == nil {
		d.Publisher = nopPublisher{}
	}
	c := &core{pool: d.Pool, q: db.New(d.Pool), log: d.Log, pub: d.Publisher, now: d.Now, cfg: d.Settings}

	s := &Services{}
	s.Audit = &AuditService{core: c}
	s.SMS = newSMSService(c, d.SMS)
	c.sms = s.SMS
	s.Mail = newMailService(c, d.Mail)
	c.mail = s.Mail
	s.Alerts = &AlertService{core: c}
	c.alerts = s.Alerts
	s.Media = &MediaService{core: c, store: d.Storage, pod: podStorage(d.Storage)}
	s.Shipments = &ShipmentService{core: c, media: s.Media}
	s.SMS.shipments = s.Shipments
	s.Auth = &AuthService{core: c}
	s.Users = &UserService{core: c}
	s.Vehicles = &VehicleService{core: c}
	s.Tenants = &TenantService{core: c, auth: s.Auth}
	s.Reports = &ReportService{core: c}
	s.Tracking = &TrackingService{core: c}
	s.Sync = &SyncService{core: c, shipments: s.Shipments, media: s.Media}
	s.Jobs = &Jobs{core: c}
	return s
}

// core is the state every service embeds.
type core struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	log    zerolog.Logger
	pub    Publisher
	now    func() time.Time
	cfg    Settings
	sms    *SMSService
	mail   *MailService
	alerts *AlertService
}

// tx runs fn in a transaction, committing if it returns nil. Work that must
// happen only once the data is durable — live updates, waking the SMS and
// email senders — is queued with after and runs after commit.
func (c *core) tx(ctx context.Context, fn func(q *db.Queries, after *afterCommit) error) error {
	tx, err := c.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	after := &afterCommit{}
	if err := fn(c.q.WithTx(tx), after); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	for _, f := range after.fns {
		f()
	}
	return nil
}

type afterCommit struct{ fns []func() }

func (a *afterCommit) do(f func()) { a.fns = append(a.fns, f) }

// publish sends a live update to every dashboard of a tenant.
func (c *core) publish(tenantID pgtype.UUID, kind string, data any) {
	msg, err := json.Marshal(map[string]any{"type": kind, "data": data})
	if err != nil {
		c.log.Error().Err(err).Str("type", kind).Msg("marshal live update")
		return
	}
	c.pub.BroadcastToTenant(tenantID.String(), msg)
}

type nopPublisher struct{}

func (nopPublisher) BroadcastToTenant(string, []byte) {}

// Principal is the authenticated caller of a request.
type Principal struct {
	SessionID  pgtype.UUID
	UserID     pgtype.UUID
	TenantID   pgtype.UUID
	Role       string
	Name       string
	Phone      string
	Email      string
	TenantName string
	TenantPlan string
	TrialEnds  *time.Time
	// MustChangeCredential is set after an admin reset or on first login with
	// an issued credential. The caller may do nothing else until it is changed.
	MustChangeCredential bool
	// EmailVerified reports whether Email has been confirmed by a link.
	EmailVerified bool
}

func (p *Principal) IsDriver() bool         { return p.Role == model.RoleDriver }
func (p *Principal) CanManageFleet() bool   { return model.CanManageFleet(p.Role) }
func (p *Principal) CanManageCompany() bool { return model.CanManageCompany(p.Role) }
func (p *Principal) IsPlatformAdmin() bool  { return p.Role == model.RolePlatformAdmin }

// Actor builds the record of who did something, for history and audit.
func (p *Principal) Actor(source, ip, userAgent string) Actor {
	return Actor{
		UserID: p.UserID, TenantID: p.TenantID, Name: p.Name, Role: p.Role,
		Source: source, IP: ip, UserAgent: userAgent,
	}
}

// Actor is who performed an action and through what channel.
type Actor struct {
	UserID    pgtype.UUID
	TenantID  pgtype.UUID
	Name      string
	Role      string
	Source    string
	IP        string
	UserAgent string
}

// Event sources, matching the shipment_events check constraint.
const (
	SourceDashboard = "dashboard"
	SourceApp       = "app"
	SourceSMS       = "sms"
	SourceSystem    = "system"
	SourceCustomer  = "customer"
	SourceAPI       = "api"
)

// SystemActor is the platform acting on its own, such as a scheduled check.
func SystemActor(tenantID pgtype.UUID) Actor {
	return Actor{TenantID: tenantID, Name: "System", Source: SourceSystem}
}

func (a Actor) isDriver() bool { return a.Role == model.RoleDriver }

// randomToken returns n random bytes, URL-safe base64 encoded.
func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// isNoRows reports whether a query found nothing.
func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// isUniqueViolation reports whether err is a unique-constraint failure, and
// on which constraint.
func isUniqueViolation(err error) (string, bool) {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return pg.ConstraintName, true
	}
	return "", false
}

// mustJSON marshals a value whose type cannot fail to marshal.
func mustJSON(v any) []byte {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
