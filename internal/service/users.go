package service

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/phone"
	"github.com/natnael-alemayehu/kiflet/pkg/hash"
)

// UserService manages drivers and office staff (FR-004).
type UserService struct{ *core }

// PersonInput is the add/edit form for a driver or staff member.
type PersonInput struct {
	FullName  string
	Phone     string
	Email     string
	Role      string // staff only
	VehicleID string // drivers only
	// LicenceExpires is the last day a driver's licence is valid, as
	// 2026-12-02; empty when not recorded. Drivers only.
	LicenceExpires string
}

type validPerson struct {
	name  string
	phone *string
	email *string
}

func validatePerson(in PersonInput, phoneRequired bool) (validPerson, error) {
	var v validPerson
	v.name = strings.Join(strings.Fields(in.FullName), " ")
	if v.name == "" {
		return v, invalid("full_name", "Enter the full name.")
	}
	if utf8.RuneCountInString(v.name) > 120 {
		return v, invalid("full_name", "That name is too long.")
	}
	if strings.TrimSpace(in.Phone) != "" || phoneRequired {
		p, err := phone.Normalize(in.Phone)
		if err != nil {
			return v, invalid("phone", err.Error())
		}
		v.phone = &p
	}
	if e := strings.TrimSpace(in.Email); e != "" {
		addr, err := mail.ParseAddress(e)
		if err != nil || addr.Address != e || !strings.Contains(e, ".") {
			return v, invalid("email", "Enter a valid email address.")
		}
		e = strings.ToLower(e)
		v.email = &e
	}
	return v, nil
}

// duplicateUser turns a unique-index failure into a message about the field.
// Phone numbers and email addresses are unique across companies, so when
// the clash is with an account at another company the message says so
// (without naming it): searching this company's own list would never find
// it. The lookup runs outside the caller's transaction, which the failure
// has aborted.
func (c *core) duplicateUser(ctx context.Context, tenantID pgtype.UUID, v validPerson, err error) error {
	con, ok := isUniqueViolation(err)
	if !ok {
		return err
	}
	field, what := "phone", "phone number"
	var holder db.User
	lerr := pgx.ErrNoRows
	if strings.Contains(con, "email") {
		field, what = "email", "email address"
		if v.email != nil {
			holder, lerr = c.q.GetUserByEmail(ctx, *v.email)
		}
	} else if v.phone != nil {
		holder, lerr = c.q.GetUserByPhone(ctx, v.phone)
	}
	if lerr == nil && holder.TenantID != tenantID {
		return invalid(field, fmt.Sprintf("This %s is already used by an account at another company on %s. "+
			"A %s can belong to only one account, so use a different one.", what, c.cfg.AppName, what))
	}
	return invalid(field, "Someone is already registered with this "+what+".")
}

// Get loads a user of a tenant.
func (s *UserService) Get(ctx context.Context, tenantID, id pgtype.UUID) (db.User, error) {
	u, err := s.q.GetUserByID(ctx, db.GetUserByIDParams{ID: id, TenantID: tenantID})
	if isNoRows(err) {
		return u, notFound("Person not found.")
	}
	return u, err
}

// Staff lists the office accounts of a tenant.
func (s *UserService) Staff(ctx context.Context, tenantID pgtype.UUID) ([]db.User, error) {
	return s.q.ListUsersByTenant(ctx, db.ListUsersByTenantParams{TenantID: tenantID, Limit: 500, Offset: 0})
}

// Drivers lists a tenant's drivers with their load and device status.
func (s *UserService) Drivers(ctx context.Context, tenantID pgtype.UUID, activeOnly bool, query string) ([]db.ListDriversWithStatusRow, error) {
	return s.q.ListDriversWithStatus(ctx, db.ListDriversWithStatusParams{
		TenantID: tenantID, ActiveOnly: activeOnly, Query: toText(escapeLike(strings.TrimSpace(query))),
	})
}

// NewAccount is a created account and its one-time credential.
type NewAccount struct {
	User db.User
	// Credential is the initial PIN or password, shown once.
	Credential string
	// Texted reports whether the credential was sent to the user by SMS.
	Texted bool
}

// CreateDriver adds a driver with a generated PIN, texted to them with the
// link to the app.
func (s *UserService) CreateDriver(ctx context.Context, p *Principal, in PersonInput, ci ClientInfo) (*NewAccount, error) {
	if !p.CanManageFleet() {
		return nil, ErrUnauthorized
	}
	v, err := validatePerson(in, true)
	if err != nil {
		return nil, err
	}
	licence, err := parseDay("licence_expires_on", "the licence runs out", in.LicenceExpires)
	if err != nil {
		return nil, err
	}
	out := &NewAccount{}
	err = s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		// Locked, so two drivers added at once can't both take the last place.
		tenant, err := q.GetTenantForUpdate(ctx, p.TenantID)
		if err != nil {
			return err
		}
		if err := s.requireWritable(ctx, q, tenant); err != nil {
			return err
		}
		if err := checkDriverQuota(ctx, q, tenant); err != nil {
			return err
		}
		vehicle, err := s.ownVehicle(ctx, q, p.TenantID, in.VehicleID)
		if err != nil {
			return err
		}
		pin := randomDigits(6)
		for checkPIN(pin) != nil {
			pin = randomDigits(6)
		}
		h, err := hash.PIN(pin)
		if err != nil {
			return err
		}
		u, err := q.CreateUser(ctx, db.CreateUserParams{
			TenantID: p.TenantID, Role: model.RoleDriver, FullName: v.name, Phone: v.phone, Email: v.email,
			PinHash: &h, PasswordResetRequired: true, VehicleID: vehicle,
		})
		if err != nil {
			return s.duplicateUser(ctx, p.TenantID, v, err)
		}
		if licence.Valid {
			if err := q.SetDriverLicence(ctx, db.SetDriverLicenceParams{ID: u.ID, TenantID: p.TenantID, LicenceExpiresOn: licence}); err != nil {
				return err
			}
			u.LicenceExpiresOn = licence
		}
		out.User, out.Credential = u, pin
		s.enqueueSMS(ctx, q, after, outboundSMS{
			TenantID: p.TenantID, Phone: *v.phone, Purpose: PurposeCredential, Sensitive: true,
			Body: fmt.Sprintf("%s added you as a driver. On your phone open %s/driver and sign in with %s and PIN %s.",
				tenant.Name, s.cfg.BaseURL, *v.phone, pin),
		})
		out.Texted = true
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "driver_created", "user", u.ID, nil,
			map[string]any{"name": u.FullName, "phone": deref(u.Phone)})
		return nil
	})
	return out, err
}

// UpdateDriver edits a driver's profile and usual vehicle.
func (s *UserService) UpdateDriver(ctx context.Context, p *Principal, id pgtype.UUID, in PersonInput, ci ClientInfo) (db.User, error) {
	if !p.CanManageFleet() {
		return db.User{}, ErrUnauthorized
	}
	v, err := validatePerson(in, true)
	if err != nil {
		return db.User{}, err
	}
	licence, err := parseDay("licence_expires_on", "the licence runs out", in.LicenceExpires)
	if err != nil {
		return db.User{}, err
	}
	var out db.User
	err = s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		cur, err := s.Get(ctx, p.TenantID, id)
		if err != nil {
			return err
		}
		if cur.Role != model.RoleDriver {
			return notFound("Driver not found.")
		}
		if cur.AnonymisedAt.Valid {
			return errAnonymised
		}
		vehicle, err := s.ownVehicle(ctx, q, p.TenantID, in.VehicleID)
		if err != nil {
			return err
		}
		out, err = q.UpdateUserProfile(ctx, db.UpdateUserProfileParams{ID: id, TenantID: p.TenantID, FullName: v.name, Phone: v.phone, Email: v.email})
		if err != nil {
			return s.duplicateUser(ctx, p.TenantID, v, err)
		}
		if emailChanged(cur, out) {
			if err := s.sendEmailVerification(ctx, q, after, out, "Your "+s.cfg.AppName+" account at "+p.TenantName+" now uses this email address."); err != nil {
				return err
			}
		}
		if err := q.SetDriverVehicle(ctx, db.SetDriverVehicleParams{ID: id, TenantID: p.TenantID, VehicleID: vehicle}); err != nil {
			return err
		}
		out.VehicleID = vehicle
		if !sameDay(cur.LicenceExpiresOn, licence) {
			if err := q.SetDriverLicence(ctx, db.SetDriverLicenceParams{ID: id, TenantID: p.TenantID, LicenceExpiresOn: licence}); err != nil {
				return err
			}
			if err := s.papersRenewed(ctx, q, p.TenantID, AlertLicenceDue, id, pgtype.UUID{}); err != nil {
				return err
			}
		}
		out.LicenceExpiresOn = licence
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "driver_updated", "user", id,
			map[string]any{"name": cur.FullName, "phone": deref(cur.Phone), "licence": FormatDay(cur.LicenceExpiresOn)},
			map[string]any{"name": out.FullName, "phone": deref(out.Phone), "licence": FormatDay(licence)})
		return nil
	})
	return out, err
}

func (s *UserService) ownVehicle(ctx context.Context, q *db.Queries, tenantID pgtype.UUID, vehicleID string) (pgtype.UUID, error) {
	if strings.TrimSpace(vehicleID) == "" {
		return pgtype.UUID{}, nil
	}
	id, err := parseUUID(vehicleID)
	if err != nil {
		return pgtype.UUID{}, invalid("vehicle_id", "Choose a vehicle.")
	}
	v, err := q.GetVehicleByID(ctx, db.GetVehicleByIDParams{ID: id, TenantID: tenantID})
	if err != nil {
		return pgtype.UUID{}, invalid("vehicle_id", "Choose a vehicle.")
	}
	return v.ID, nil
}

func checkDriverQuota(ctx context.Context, q *db.Queries, t db.Tenant) error {
	n, err := q.CountActiveDriversByTenant(ctx, t.ID)
	if err != nil {
		return err
	}
	if n >= int64(t.MaxDrivers) {
		return limitReached(fmt.Sprintf("Your %s plan allows %d active drivers. Deactivate one, or move to a bigger plan on the Billing page.",
			model.PlanName(t.Plan), t.MaxDrivers))
	}
	return nil
}

// CreateStaff adds an office account with a temporary password.
func (s *UserService) CreateStaff(ctx context.Context, p *Principal, in PersonInput, ci ClientInfo) (*NewAccount, error) {
	if !p.CanManageCompany() {
		return nil, ErrUnauthorized
	}
	v, err := validatePerson(in, false)
	if err != nil {
		return nil, err
	}
	if v.email == nil && v.phone == nil {
		return nil, invalid("email", "Enter an email address or phone number to sign in with.")
	}
	if !contains(model.StaffRoles, in.Role) {
		return nil, invalid("role", "Choose a role.")
	}
	out := &NewAccount{}
	err = s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		tenant, err := q.GetTenantByID(ctx, p.TenantID)
		if err != nil {
			return err
		}
		if err := s.requireWritable(ctx, q, tenant); err != nil {
			return err
		}
		pw := temporaryPassword()
		h, err := hash.Password(pw)
		if err != nil {
			return err
		}
		u, err := q.CreateUser(ctx, db.CreateUserParams{
			TenantID: p.TenantID, Role: in.Role, FullName: v.name, Phone: v.phone, Email: v.email,
			PasswordHash: &h, PasswordResetRequired: true,
		})
		if err != nil {
			return s.duplicateUser(ctx, p.TenantID, v, err)
		}
		out.User, out.Credential = u, pw
		if v.phone != nil {
			login := *v.phone
			if v.email != nil {
				login = *v.email
			}
			s.enqueueSMS(ctx, q, after, outboundSMS{
				TenantID: p.TenantID, Phone: *v.phone, Purpose: PurposeCredential, Sensitive: true,
				Body: fmt.Sprintf("%s: your dashboard account is ready. Sign in at %s/login as %s with temporary password %s.",
					p.TenantName, s.cfg.BaseURL, login, pw),
			})
			out.Texted = true
		}
		password := p.Name + " will give you a temporary password for your first sign-in."
		if out.Texted {
			password = "We've texted a temporary password for your first sign-in to your phone."
		}
		intro := fmt.Sprintf("%s added you to %s on %s. Sign in at %s/login with this email address. %s",
			p.Name, p.TenantName, s.cfg.AppName, s.cfg.BaseURL, password)
		if err := s.sendEmailVerification(ctx, q, after, u, intro); err != nil {
			return err
		}
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "staff_created", "user", u.ID, nil,
			map[string]any{"name": u.FullName, "role": u.Role})
		return nil
	})
	return out, err
}

// UpdateStaff edits an office account, including its role.
func (s *UserService) UpdateStaff(ctx context.Context, p *Principal, id pgtype.UUID, in PersonInput, ci ClientInfo) (db.User, error) {
	if !p.CanManageCompany() && id != p.UserID {
		return db.User{}, ErrUnauthorized
	}
	v, err := validatePerson(in, false)
	if err != nil {
		return db.User{}, err
	}
	if v.email == nil && v.phone == nil {
		return db.User{}, invalid("email", "Enter an email address or phone number to sign in with.")
	}
	var out db.User
	err = s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		cur, err := s.Get(ctx, p.TenantID, id)
		if err != nil {
			return err
		}
		if cur.Role == model.RoleDriver {
			return notFound("Staff member not found.")
		}
		if cur.AnonymisedAt.Valid {
			return errAnonymised
		}
		out, err = q.UpdateUserProfile(ctx, db.UpdateUserProfileParams{ID: id, TenantID: p.TenantID, FullName: v.name, Phone: v.phone, Email: v.email})
		if err != nil {
			return s.duplicateUser(ctx, p.TenantID, v, err)
		}
		if emailChanged(cur, out) {
			if err := s.sendEmailVerification(ctx, q, after, out, "Your "+s.cfg.AppName+" account at "+p.TenantName+" now uses this email address."); err != nil {
				return err
			}
		}
		if in.Role != "" && in.Role != cur.Role && p.CanManageCompany() {
			if !contains(model.StaffRoles, in.Role) {
				return invalid("role", "Choose a role.")
			}
			if id == p.UserID {
				return invalid("role", "You can't change your own role.")
			}
			if cur.Role == model.RoleAdmin {
				if err := s.ensureAnotherAdmin(ctx, q, p.TenantID, id); err != nil {
					return err
				}
			}
			if err := q.SetUserRole(ctx, db.SetUserRoleParams{ID: id, TenantID: p.TenantID, Role: in.Role}); err != nil {
				return err
			}
			out.Role = in.Role
		}
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "staff_updated", "user", id,
			map[string]any{"name": cur.FullName, "role": cur.Role}, map[string]any{"name": out.FullName, "role": out.Role})
		return nil
	})
	return out, err
}

func (s *UserService) ensureAnotherAdmin(ctx context.Context, q *db.Queries, tenantID, except pgtype.UUID) error {
	staff, err := q.ListUsersByTenant(ctx, db.ListUsersByTenantParams{TenantID: tenantID, Limit: 500})
	if err != nil {
		return err
	}
	for _, u := range staff {
		if u.ID != except && u.Role == model.RoleAdmin && u.IsActive {
			return nil
		}
	}
	return conflict("The company needs at least one active admin.")
}

// SetActive deactivates or reactivates an account. A deactivated person is
// signed out everywhere at once.
func (s *UserService) SetActive(ctx context.Context, p *Principal, id pgtype.UUID, active bool, ci ClientInfo) error {
	if id == p.UserID {
		return invalid("", "You can't deactivate your own account.")
	}
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		u, err := s.Get(ctx, p.TenantID, id)
		if err != nil {
			return err
		}
		if u.Role == model.RoleDriver && !p.CanManageFleet() || u.Role != model.RoleDriver && !p.CanManageCompany() {
			return ErrUnauthorized
		}
		if u.IsActive == active {
			return nil
		}
		if active && u.AnonymisedAt.Valid {
			return errAnonymised
		}
		if !active {
			if u.Role == model.RoleDriver {
				n, err := q.CountActiveShipmentsByDriver(ctx, db.CountActiveShipmentsByDriverParams{DriverID: id, TenantID: p.TenantID})
				if err != nil {
					return err
				}
				if n > 0 {
					return conflict(fmt.Sprintf("%s has %d active shipments. Reassign them first.", u.FullName, n))
				}
			}
			if u.Role == model.RoleAdmin {
				if err := s.ensureAnotherAdmin(ctx, q, p.TenantID, id); err != nil {
					return err
				}
			}
		} else {
			tenant, err := q.GetTenantForUpdate(ctx, p.TenantID)
			if err != nil {
				return err
			}
			if err := s.requireWritable(ctx, q, tenant); err != nil {
				return err
			}
			if u.Role == model.RoleDriver {
				if err := checkDriverQuota(ctx, q, tenant); err != nil {
					return err
				}
			}
		}
		if err := q.SetUserActive(ctx, db.SetUserActiveParams{ID: id, TenantID: p.TenantID, IsActive: active}); err != nil {
			return err
		}
		if !active {
			if err := q.RevokeAllUserSessions(ctx, id); err != nil {
				return err
			}
			// Someone who has left needs no reminder about their licence.
			if err := s.papersRenewed(ctx, q, p.TenantID, AlertLicenceDue, id, pgtype.UUID{}); err != nil {
				return err
			}
		}
		action := "user_deactivated"
		if active {
			action = "user_reactivated"
		}
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), action, "user", id, nil, map[string]any{"name": u.FullName})
		return nil
	})
}

// DriverDetail is the driver page.
type DriverDetail struct {
	Driver db.User
	// Status is the driver's line on the drivers page: whether they are
	// on duty, how many shipments they carry.
	Status      db.ListDriversWithStatusRow
	Vehicle     *db.Vehicle
	Counts      db.GetDriverDeliveryCountsRow
	Performance db.GetDriverPerformanceRow
	Recent      []db.Shipment
	Location    *db.GetLatestDriverLocationRow
	Device      *db.SyncMetadatum
}

// Driver assembles the driver page: workload, record over 30 days, recent
// shipments, last position and device state.
func (s *UserService) Driver(ctx context.Context, tenantID, id pgtype.UUID) (*DriverDetail, error) {
	u, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if u.Role != model.RoleDriver {
		return nil, notFound("Driver not found.")
	}
	d := &DriverDetail{Driver: u}
	if u.VehicleID.Valid {
		if v, err := s.q.GetVehicleByID(ctx, db.GetVehicleByIDParams{ID: u.VehicleID, TenantID: tenantID}); err == nil {
			d.Vehicle = &v
		}
	}
	if d.Counts, err = s.q.GetDriverDeliveryCounts(ctx, db.GetDriverDeliveryCountsParams{DriverID: id, TenantID: tenantID}); err != nil {
		return nil, err
	}
	if d.Performance, err = s.q.GetDriverPerformance(ctx, db.GetDriverPerformanceParams{
		DriverID: id, TenantID: tenantID, Since: toTimestamp(s.now().Add(-30 * 24 * time.Hour)),
	}); err != nil {
		return nil, err
	}
	if d.Recent, err = s.q.ListRecentShipmentsByDriver(ctx, db.ListRecentShipmentsByDriverParams{DriverID: id, TenantID: tenantID, Limit: 20}); err != nil {
		return nil, err
	}
	if loc, err := s.q.GetLatestDriverLocation(ctx, db.GetLatestDriverLocationParams{TenantID: tenantID, DriverID: id}); err == nil {
		d.Location = &loc
	}
	if m, err := s.q.GetSyncMetadata(ctx, id); err == nil {
		d.Device = &m
	}
	rows, err := s.q.ListDriversWithStatus(ctx, db.ListDriversWithStatusParams{TenantID: tenantID, DriverID: id})
	if err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		d.Status = rows[0]
	}
	return d, nil
}
