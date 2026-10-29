package service

import (
	"context"
	"net/mail"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/geo"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/internal/phone"
	"github.com/natnael-alemayehu/logistics/pkg/hash"
)

// TrialLength is how long a new company may use the product before choosing
// a plan.
const TrialLength = 14 * 24 * time.Hour

// TenantService manages companies: sign-up, settings, usage, and the
// platform operator's view across them.
type TenantService struct {
	*core
	auth *AuthService
}

// SignupInput is the self-service registration form.
type SignupInput struct {
	Company   string
	City      string
	AdminName string
	Email     string
	Phone     string
	Password  string
}

// Signup creates a company on a free trial with its first admin, and signs
// that admin in.
func (s *TenantService) Signup(ctx context.Context, in SignupInput, ci ClientInfo) (*LoginResult, error) {
	company := strings.Join(strings.Fields(in.Company), " ")
	if len(company) < 2 {
		return nil, invalid("company", "Enter your company's name.")
	}
	if len(company) > 120 {
		return nil, invalid("company", "That name is too long.")
	}
	city := "Addis Ababa"
	if strings.TrimSpace(in.City) != "" {
		p, ok := geo.Lookup(in.City)
		if !ok {
			return nil, invalid("city", "Pick your base town from the list.")
		}
		city = p.Name
	}
	person, err := validatePerson(PersonInput{FullName: in.AdminName, Phone: in.Phone, Email: in.Email}, true)
	if err != nil {
		return nil, err
	}
	if person.email == nil {
		return nil, invalid("email", "Enter your email address.")
	}
	if err := checkPassword(in.Password); err != nil {
		e := *err.(*Error) // a copy: checkPassword may return a shared value
		e.Field = "password"
		return nil, &e
	}
	h, err := hash.Password(in.Password)
	if err != nil {
		return nil, err
	}

	var admin db.User
	err = s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		var tenant db.Tenant
		base := slugify(company)
		for attempt := 0; ; attempt++ {
			slug := base
			if attempt > 0 {
				slug = base + "-" + strings.ToLower(randomDigits(4))
			}
			tenant, err = q.CreateTenant(ctx, db.CreateTenantParams{
				Name: company, Slug: slug, Plan: model.PlanTrial, MaxDrivers: 5,
				ContactPhone: person.phone, ContactEmail: person.email, RegionCity: city,
				TrialEndsAt: toTimestamp(s.now().Add(TrialLength)),
			})
			if _, dup := isUniqueViolation(err); dup && attempt < 5 {
				continue
			}
			if err != nil {
				return err
			}
			break
		}
		admin, err = q.CreateUser(ctx, db.CreateUserParams{
			TenantID: tenant.ID, Role: model.RoleAdmin, FullName: person.name,
			Phone: person.phone, Email: person.email, PasswordHash: &h,
		})
		if err != nil {
			return duplicateUser(err)
		}
		s.audit(ctx, q, Actor{UserID: admin.ID, TenantID: tenant.ID, Name: admin.FullName, Role: admin.Role, IP: ci.IP, UserAgent: ci.UserAgent},
			"company_signup", "tenant", tenant.ID, nil, map[string]any{"name": company})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.auth.startSession(ctx, admin, SessionWeb, false, ci)
}

func slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	if out == "" {
		out = "company"
	}
	return out
}

// Get loads a tenant.
func (s *TenantService) Get(ctx context.Context, id pgtype.UUID) (db.Tenant, error) {
	t, err := s.q.GetTenantByID(ctx, id)
	if isNoRows(err) {
		return t, notFound("Company not found.")
	}
	return t, err
}

// SettingsInput is the company settings form.
type SettingsInput struct {
	Name               string
	ContactPhone       string
	ContactEmail       string
	RegionCity         string
	MaxActivePerDriver int
	SMSOnCreated       bool
	SMSOnDispatched    bool
	SMSOnArrived       bool
	SMSOnDelivered     bool
	SMSOnCancelled     bool
	SMSOnDelayed       bool
	SMSDriverOnAssign  bool
}

// UpdateSettings saves the company profile and notification preferences.
func (s *TenantService) UpdateSettings(ctx context.Context, p *Principal, in SettingsInput, ci ClientInfo) (db.Tenant, error) {
	if !p.CanManageCompany() {
		return db.Tenant{}, ErrUnauthorized
	}
	name := strings.Join(strings.Fields(in.Name), " ")
	if len(name) < 2 || len(name) > 120 {
		return db.Tenant{}, invalid("name", "Enter the company name.")
	}
	var contactPhone, contactEmail *string
	if strings.TrimSpace(in.ContactPhone) != "" {
		ph, err := phone.Normalize(in.ContactPhone)
		if err != nil {
			return db.Tenant{}, invalid("contact_phone", err.Error())
		}
		contactPhone = &ph
	}
	if e := strings.TrimSpace(in.ContactEmail); e != "" {
		if _, err := mail.ParseAddress(e); err != nil {
			return db.Tenant{}, invalid("contact_email", "Enter a valid email address.")
		}
		contactEmail = &e
	}
	city, ok := geo.Lookup(in.RegionCity)
	if !ok {
		return db.Tenant{}, invalid("region_city", "Pick your base town from the list.")
	}
	if in.MaxActivePerDriver < 1 || in.MaxActivePerDriver > 50 {
		return db.Tenant{}, invalid("max_active_per_driver", "Choose between 1 and 50.")
	}
	cur, err := s.Get(ctx, p.TenantID)
	if err != nil {
		return db.Tenant{}, err
	}
	t, err := s.q.UpdateTenantSettings(ctx, db.UpdateTenantSettingsParams{
		ID: p.TenantID, Name: name, ContactPhone: contactPhone, ContactEmail: contactEmail,
		RegionCity: city.Name, MaxActivePerDriver: int32(in.MaxActivePerDriver),
		SmsOnCreated: in.SMSOnCreated, SmsOnDispatched: in.SMSOnDispatched, SmsOnArrived: in.SMSOnArrived,
		SmsOnDelivered: in.SMSOnDelivered, SmsOnCancelled: in.SMSOnCancelled, SmsOnDelayed: in.SMSOnDelayed,
		SmsDriverOnAssign: in.SMSDriverOnAssign,
	})
	if err != nil {
		return t, err
	}
	s.audit(ctx, s.q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "settings_updated", "tenant", t.ID, cur, t)
	return t, nil
}

// Usage is a company's consumption against its plan.
type Usage struct {
	Plan               model.Plan
	ActiveDrivers      int64
	MaxDrivers         int32
	Vehicles           int64
	Staff              int64
	ShipmentsThisMonth int64
	SMSThisMonth       int64
	TrialEnds          *time.Time
}

// Usage reports a company's consumption this calendar month.
func (s *TenantService) Usage(ctx context.Context, tenantID pgtype.UUID) (*Usage, error) {
	t, err := s.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	now := s.now().In(s.cfg.Location)
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.cfg.Location)
	u := &Usage{MaxDrivers: t.MaxDrivers, TrialEnds: timePtr(t.TrialEndsAt)}
	u.Plan, _ = model.PlanByCode(t.Plan)
	if u.Plan.Code == "" {
		u.Plan = model.Plan{Code: t.Plan, Name: t.Plan, MaxDrivers: t.MaxDrivers}
	}
	if u.ActiveDrivers, err = s.q.CountActiveDriversByTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	if u.Vehicles, err = s.q.CountVehiclesByTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	if u.Staff, err = s.q.CountStaffByTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	if u.ShipmentsThisMonth, err = s.q.CountShipmentsCreatedSince(ctx, db.CountShipmentsCreatedSinceParams{TenantID: tenantID, CreatedAt: toTimestamp(month)}); err != nil {
		return nil, err
	}
	if u.SMSThisMonth, err = s.q.CountSMSSentSince(ctx, db.CountSMSSentSinceParams{TenantID: tenantID, SentAt: toTimestamp(month)}); err != nil {
		return nil, err
	}
	return u, nil
}

// All lists every company with activity figures, for the platform operator.
func (s *TenantService) All(ctx context.Context, p *Principal) ([]db.ListTenantsWithStatsRow, error) {
	if !p.IsPlatformAdmin() {
		return nil, ErrUnauthorized
	}
	return s.q.ListTenantsWithStats(ctx)
}

// SetActive suspends or restores a company. Suspension signs everyone in it
// out on their next request.
func (s *TenantService) SetActive(ctx context.Context, p *Principal, id pgtype.UUID, active bool, ci ClientInfo) error {
	if !p.IsPlatformAdmin() {
		return ErrUnauthorized
	}
	if id == p.TenantID {
		return invalid("", "You can't suspend the platform's own account.")
	}
	if err := s.q.SetTenantActive(ctx, db.SetTenantActiveParams{ID: id, IsActive: active}); err != nil {
		return err
	}
	action := "tenant_suspended"
	if active {
		action = "tenant_restored"
	}
	s.audit(ctx, s.q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), action, "tenant", id, nil, nil)
	return nil
}

// SetPlan moves a company to a plan, with an optional custom driver limit.
func (s *TenantService) SetPlan(ctx context.Context, p *Principal, id pgtype.UUID, code string, maxDrivers int, ci ClientInfo) error {
	if !p.IsPlatformAdmin() {
		return ErrUnauthorized
	}
	plan, ok := model.PlanByCode(code)
	if !ok {
		return invalid("plan", "Choose a plan.")
	}
	limit := plan.MaxDrivers
	if maxDrivers > 0 {
		if maxDrivers > 10000 {
			return invalid("max_drivers", "That limit is too high.")
		}
		limit = int32(maxDrivers)
	}
	if err := s.q.UpdateTenantPlan(ctx, db.UpdateTenantPlanParams{ID: id, Plan: plan.Code, MaxDrivers: limit}); err != nil {
		return err
	}
	s.audit(ctx, s.q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "tenant_plan_changed", "tenant", id, nil,
		map[string]any{"plan": plan.Code, "max_drivers": limit})
	return nil
}
