package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/internal/phone"
	"github.com/natnael-alemayehu/logistics/pkg/hash"
)

// Session kinds: "api" sessions carry a bearer token (the driver app);
// "web" sessions a cookie (the dashboard).
const (
	SessionAPI = "api"
	SessionWeb = "web"
)

const (
	maxFailedLogins = 5
	lockoutDuration = 15 * time.Minute

	// Drivers stay signed in while they use the app: a driver days into a
	// trip without signal must not come back to a login screen.
	apiSessionTTL = 30 * 24 * time.Hour
	// Dashboard sessions end after eight idle hours, or last thirty days with
	// "remember me" (NFR security).
	webIdleTimeout = 8 * time.Hour
	webRememberTTL = 30 * 24 * time.Hour

	touchInterval = time.Minute
)

var (
	errSessionIdle = &Error{Kind: KindUnauthorized, Message: "You were signed out after 8 hours without activity."}
	errWeakPIN     = &Error{Kind: KindInvalid, Field: "new_pin", Message: "Choose a PIN of 4 to 6 digits that isn't a simple sequence like 1234 or 1111."}
	errWeakPass    = &Error{Kind: KindInvalid, Field: "new_password", Message: "Use at least 8 characters."}
)

// Bcrypt hashes of nothing in particular, compared against when the account
// does not exist (or has no credential of that kind) so that response time
// does not reveal which accounts do. One per cost, as PINs are hashed more
// cheaply than passwords.
var (
	dummyHash, _    = hash.Password("timing-equaliser")
	dummyPINHash, _ = hash.PIN("timing-equaliser")
)

// credential is one way of proving who you are: a driver's PIN or a staff
// password.
type credential struct {
	stored *string
	check  func(candidate, hashed string) bool
	dummy  string
}

func pinOf(u db.User) credential {
	return credential{u.PinHash, hash.CheckPIN, dummyPINHash}
}

func passwordOf(u db.User) credential {
	return credential{u.PasswordHash, hash.CheckPassword, dummyHash}
}

var errOfficeAccount = &Error{Kind: KindUnauthorized, Message: "This number belongs to an office account. Sign in to the dashboard instead."}

// AuthService signs people in and keeps track of their sessions.
type AuthService struct{ *core }

// ClientInfo describes where a request came from.
type ClientInfo struct {
	IP        string
	UserAgent string
	DeviceID  string
}

// LoginResult is a new session.
type LoginResult struct {
	Token     string
	ExpiresAt time.Time
	Principal *Principal
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// DriverLogin signs a driver into the app with phone and PIN (FR-002). A
// driver has one device: signing in elsewhere ends the other sessions.
func (s *AuthService) DriverLogin(ctx context.Context, rawPhone, pin string, ci ClientInfo) (*LoginResult, error) {
	p, err := phone.Normalize(rawPhone)
	if err != nil {
		return nil, invalid("phone", err.Error())
	}
	u, err := s.q.GetUserByPhone(ctx, &p)
	if isNoRows(err) {
		hash.CheckPIN(pin, dummyPINHash)
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if u.Role != model.RoleDriver {
		// Only the account's own password earns the pointer to the
		// dashboard: anything else is wrong login details, as for a number
		// nobody has, so a stranger learns nothing about whose number it is.
		if err := s.checkCredential(ctx, u, passwordOf(u), pin, ci); err != nil {
			return nil, err
		}
		return nil, errOfficeAccount
	}
	if err := s.checkCredential(ctx, u, pinOf(u), pin, ci); err != nil {
		return nil, err
	}
	if err := s.admit(ctx, u, ci); err != nil {
		return nil, err
	}
	res, err := s.startSession(ctx, u, SessionAPI, false, ci)
	if err != nil {
		return nil, err
	}
	_ = s.q.RevokeOtherUserSessions(ctx, db.RevokeOtherUserSessionsParams{UserID: u.ID, ID: res.Principal.SessionID})
	return res, nil
}

// StaffLogin signs office staff into the dashboard with email or phone and
// password (FR-001).
func (s *AuthService) StaffLogin(ctx context.Context, identifier, password string, remember bool, ci ClientInfo) (*LoginResult, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || password == "" {
		return nil, invalid("identifier", "Enter your email or phone and your password.")
	}
	var u db.User
	var err error
	if strings.Contains(identifier, "@") {
		u, err = s.q.GetUserByEmail(ctx, identifier)
	} else if p, perr := phone.Normalize(identifier); perr == nil {
		u, err = s.q.GetUserByPhone(ctx, &p)
	} else {
		return nil, invalid("identifier", "Enter a valid email address or phone number.")
	}
	if isNoRows(err) {
		hash.CheckPassword(password, dummyHash)
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if u.Role == model.RoleDriver {
		// As in DriverLogin: only the driver's own PIN gets the pointer to
		// the app.
		if err := s.checkCredential(ctx, u, pinOf(u), password, ci); err != nil {
			return nil, err
		}
		return nil, ErrWrongApp
	}
	if err := s.checkCredential(ctx, u, passwordOf(u), password, ci); err != nil {
		return nil, err
	}
	if err := s.admit(ctx, u, ci); err != nil {
		return nil, err
	}
	return s.startSession(ctx, u, SessionWeb, remember, ci)
}

// checkCredential checks a credential, maintaining the failed-attempt
// lockout. The lock clears the count, so once it has expired the account
// has its full number of attempts again.
func (s *AuthService) checkCredential(ctx context.Context, u db.User, c credential, candidate string, ci ClientInfo) error {
	now := s.now()
	if u.LockedUntil.Valid && u.LockedUntil.Time.After(now) {
		c.check(candidate, c.dummy)
		return ErrAccountLocked
	}
	ok := false
	if c.stored == nil {
		c.check(candidate, c.dummy)
	} else {
		ok = c.check(candidate, *c.stored)
	}
	if ok {
		return nil
	}
	actor := userActor(u, ci)
	updated, err := s.q.IncrementFailedLoginAttempts(ctx, u.ID)
	if err != nil {
		return err
	}
	if updated.FailedLoginAttempts >= maxFailedLogins {
		if err := s.q.LockUserAccount(ctx, db.LockUserAccountParams{ID: u.ID, LockedUntil: toTimestamp(now.Add(lockoutDuration))}); err != nil {
			return err
		}
		s.audit(ctx, s.q, actor, "login_locked", "user", u.ID, nil, map[string]any{"attempts": updated.FailedLoginAttempts})
		return ErrAccountLocked
	}
	s.audit(ctx, s.q, actor, "login_failed", "user", u.ID, nil, nil)
	return ErrInvalidCredentials
}

// admit lets in someone whose credential was right, if their account and
// company are active.
func (s *AuthService) admit(ctx context.Context, u db.User, ci ClientInfo) error {
	if !u.IsActive {
		return ErrAccountInactive
	}
	if u.Role != model.RolePlatformAdmin {
		t, err := s.q.GetTenantByID(ctx, u.TenantID)
		if err != nil {
			return err
		}
		if !t.IsActive {
			return s.suspended()
		}
	}
	if err := s.q.RecordSuccessfulLogin(ctx, u.ID); err != nil {
		return err
	}
	s.audit(ctx, s.q, userActor(u, ci), "login", "user", u.ID, nil, nil)
	return nil
}

func (s *AuthService) startSession(ctx context.Context, u db.User, kind string, remember bool, ci ClientInfo) (*LoginResult, error) {
	token := randomToken(32)
	now := s.now()
	var expires time.Time
	switch {
	case kind == SessionAPI:
		expires = now.Add(apiSessionTTL)
	case remember:
		expires = now.Add(webRememberTTL)
	default:
		expires = now.Add(webIdleTimeout)
	}
	sess, err := s.q.CreateSession(ctx, db.CreateSessionParams{
		UserID:           u.ID,
		TenantID:         u.TenantID,
		RefreshTokenHash: hashToken(token),
		DeviceID:         toText(truncate(ci.DeviceID, 100)),
		UserAgent:        toText(truncate(ci.UserAgent, 300)),
		IpAddress:        toText(ci.IP),
		ExpiresAt:        toTimestamp(expires),
		Kind:             kind,
		Remember:         remember,
	})
	if err != nil {
		return nil, err
	}
	p, err := s.Authenticate(ctx, token, kind)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: token, ExpiresAt: sess.ExpiresAt.Time, Principal: p}, nil
}

// Authenticate resolves a session token to its principal, enforcing
// revocation, deactivation, suspension and the idle timeout on every request.
func (s *AuthService) Authenticate(ctx context.Context, token, kind string) (*Principal, error) {
	if token == "" {
		return nil, ErrSessionRevoked
	}
	row, err := s.q.GetSessionPrincipal(ctx, db.GetSessionPrincipalParams{RefreshTokenHash: hashToken(token), Kind: kind})
	if isNoRows(err) {
		return nil, ErrSessionRevoked
	}
	if err != nil {
		return nil, err
	}
	if !row.UserActive {
		_ = s.q.RevokeAllUserSessions(ctx, row.UserID)
		return nil, ErrAccountInactive
	}
	if !row.TenantActive && row.Role != model.RolePlatformAdmin {
		return nil, s.suspended()
	}

	now := s.now()
	lastSeen := row.LastSeenAt.Time
	if kind == SessionWeb && !row.Remember && row.LastSeenAt.Valid && now.Sub(lastSeen) > webIdleTimeout {
		_ = s.q.RevokeSession(ctx, row.SessionID)
		return nil, errSessionIdle
	}
	if !row.LastSeenAt.Valid || now.Sub(lastSeen) > touchInterval {
		expires := row.ExpiresAt.Time
		switch {
		case kind == SessionAPI:
			expires = now.Add(apiSessionTTL)
		case !row.Remember:
			expires = now.Add(webIdleTimeout)
		}
		_ = s.q.TouchSession(ctx, db.TouchSessionParams{ID: row.SessionID, ExpiresAt: toTimestamp(expires)})
	}

	return &Principal{
		SessionID:            row.SessionID,
		UserID:               row.UserID,
		TenantID:             row.TenantID,
		Role:                 row.Role,
		Name:                 row.FullName,
		Phone:                deref(row.Phone),
		Email:                deref(row.Email),
		TenantName:           row.TenantName,
		TenantPlan:           row.TenantPlan,
		TrialEnds:            timePtr(row.TrialEndsAt),
		DebtNumber:           row.DebtNumber,
		DebtDue:              timePtr(row.DebtDueAt),
		MustChangeCredential: row.PasswordResetRequired,
		EmailVerified:        row.EmailVerified,
	}, nil
}

// Logout ends the current session.
func (s *AuthService) Logout(ctx context.Context, p *Principal, ci ClientInfo) error {
	if err := s.q.RevokeSession(ctx, p.SessionID); err != nil {
		return err
	}
	source := SourceDashboard
	if p.IsDriver() {
		source = SourceApp
	}
	s.audit(ctx, s.q, p.Actor(source, ci.IP, ci.UserAgent), "logout", "user", p.UserID, nil, nil)
	return nil
}

// Sessions lists the caller's active sessions.
func (s *AuthService) Sessions(ctx context.Context, p *Principal) ([]db.Session, error) {
	return s.q.ListActiveSessionsByUser(ctx, p.UserID)
}

// RevokeSession ends one of the caller's own sessions.
func (s *AuthService) RevokeSession(ctx context.Context, p *Principal, id pgtype.UUID) error {
	return s.q.RevokeSessionByUser(ctx, db.RevokeSessionByUserParams{ID: id, UserID: p.UserID})
}

// RevokeOtherSessions signs the caller out everywhere else.
func (s *AuthService) RevokeOtherSessions(ctx context.Context, p *Principal) error {
	return s.q.RevokeOtherUserSessions(ctx, db.RevokeOtherUserSessionsParams{UserID: p.UserID, ID: p.SessionID})
}

// ChangePassword sets a staff member's password. After an admin reset the
// current password is the temporary one just used to sign in, so it is not
// asked for again.
func (s *AuthService) ChangePassword(ctx context.Context, p *Principal, current, next string, ci ClientInfo) error {
	if p.IsDriver() {
		return ErrUnauthorized
	}
	if err := checkPassword(next); err != nil {
		return err
	}
	u, err := s.q.GetUserByID(ctx, db.GetUserByIDParams{ID: p.UserID, TenantID: p.TenantID})
	if err != nil {
		return err
	}
	if !u.PasswordResetRequired && (u.PasswordHash == nil || !hash.CheckPassword(current, *u.PasswordHash)) {
		return &Error{Kind: KindInvalid, Field: "current_password", Message: "Your current password is incorrect."}
	}
	if u.PasswordHash != nil && hash.CheckPassword(next, *u.PasswordHash) {
		return ErrSamePassword
	}
	h, err := hash.Password(next)
	if err != nil {
		return err
	}
	if err := s.q.UpdatePassword(ctx, db.UpdatePasswordParams{ID: u.ID, PasswordHash: &h, PasswordResetRequired: false}); err != nil {
		return err
	}
	_ = s.q.RevokeOtherUserSessions(ctx, db.RevokeOtherUserSessionsParams{UserID: u.ID, ID: p.SessionID})
	s.audit(ctx, s.q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "password_changed", "user", u.ID, nil, nil)
	return nil
}

// ChangePIN sets a driver's PIN.
func (s *AuthService) ChangePIN(ctx context.Context, p *Principal, current, next string, ci ClientInfo) error {
	if !p.IsDriver() {
		return ErrUnauthorized
	}
	if err := checkPIN(next); err != nil {
		return err
	}
	u, err := s.q.GetUserByID(ctx, db.GetUserByIDParams{ID: p.UserID, TenantID: p.TenantID})
	if err != nil {
		return err
	}
	if !u.PasswordResetRequired && (u.PinHash == nil || !hash.CheckPIN(current, *u.PinHash)) {
		return &Error{Kind: KindInvalid, Field: "current_pin", Message: "Your current PIN is incorrect."}
	}
	if u.PinHash != nil && hash.CheckPIN(next, *u.PinHash) {
		return &Error{Kind: KindInvalid, Field: "new_pin", Message: "Choose a PIN different from the current one."}
	}
	h, err := hash.PIN(next)
	if err != nil {
		return err
	}
	if err := s.q.UpdatePIN(ctx, db.UpdatePINParams{ID: u.ID, PinHash: &h, PasswordResetRequired: false}); err != nil {
		return err
	}
	s.audit(ctx, s.q, p.Actor(SourceApp, ci.IP, ci.UserAgent), "pin_changed", "user", u.ID, nil, nil)
	return nil
}

// RequestPINReset texts a reset code to a driver who forgot their PIN. It
// says nothing about whether the number belongs to a driver, even once the
// hourly limit is reached.
func (s *AuthService) RequestPINReset(ctx context.Context, rawPhone string) error {
	p, err := phone.Normalize(rawPhone)
	if err != nil {
		return invalid("phone", err.Error())
	}
	u, err := s.q.GetUserByPhone(ctx, &p)
	if err != nil && !isNoRows(err) {
		return err
	}
	if err != nil || u.Role != model.RoleDriver || !u.IsActive {
		// Recorded, not sent: the hourly limit then applies to every
		// number alike, and doesn't single out drivers' numbers.
		return s.issueCode(ctx, pgtype.UUID{}, p, codePurposePINReset, nil)
	}
	return s.issueCode(ctx, u.TenantID, p, codePurposePINReset, func(code string) string {
		return fmt.Sprintf("%s: your PIN reset code is %s. It expires in 10 minutes. Never share it.", s.cfg.AppName, code)
	})
}

// ConfirmPINReset sets a new PIN with a reset code and signs the driver in.
func (s *AuthService) ConfirmPINReset(ctx context.Context, rawPhone, code, newPIN string, ci ClientInfo) (*LoginResult, error) {
	p, err := phone.Normalize(rawPhone)
	if err != nil {
		return nil, invalid("phone", err.Error())
	}
	if err := checkPIN(newPIN); err != nil {
		return nil, err
	}
	if err := s.checkCode(ctx, p, codePurposePINReset, strings.TrimSpace(code)); err != nil {
		return nil, err
	}
	u, err := s.q.GetUserByPhone(ctx, &p)
	if err != nil || u.Role != model.RoleDriver || !u.IsActive {
		return nil, errCodeInvalid
	}
	h, err := hash.PIN(newPIN)
	if err != nil {
		return nil, err
	}
	if err := s.q.UpdatePIN(ctx, db.UpdatePINParams{ID: u.ID, PinHash: &h, PasswordResetRequired: false}); err != nil {
		return nil, err
	}
	_ = s.q.RevokeAllUserSessions(ctx, u.ID)
	s.audit(ctx, s.q, Actor{UserID: u.ID, TenantID: u.TenantID, Name: u.FullName, Role: u.Role, IP: ci.IP, UserAgent: ci.UserAgent},
		"pin_reset", "user", u.ID, nil, nil)
	u.LockedUntil = pgtype.Timestamptz{}
	return s.startSession(ctx, u, SessionAPI, false, ci)
}

// ResetCredential issues a temporary PIN (drivers) or password (staff) that
// must be changed at next sign-in. It is texted to the user when they have a
// phone and returned so the admin can pass it on; it is never stored in
// plain text or shown again.
func (s *AuthService) ResetCredential(ctx context.Context, p *Principal, userID pgtype.UUID, ci ClientInfo) (temp string, texted bool, err error) {
	if userID == p.UserID {
		return "", false, invalid("", "Change your own password from your account page.")
	}
	u, err := s.q.GetUserByID(ctx, db.GetUserByIDParams{ID: userID, TenantID: p.TenantID})
	if isNoRows(err) {
		return "", false, notFound("User not found.")
	}
	if err != nil {
		return "", false, err
	}
	if u.Role == model.RoleDriver && !p.CanManageFleet() || u.Role != model.RoleDriver && !p.CanManageCompany() {
		return "", false, ErrUnauthorized
	}
	if u.AnonymisedAt.Valid {
		return "", false, errAnonymised
	}

	var body string
	err = s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		if u.Role == model.RoleDriver {
			temp = randomDigits(6)
			for checkPIN(temp) != nil {
				temp = randomDigits(6)
			}
			h, err := hash.PIN(temp)
			if err != nil {
				return err
			}
			if err := q.UpdatePIN(ctx, db.UpdatePINParams{ID: u.ID, PinHash: &h, PasswordResetRequired: true}); err != nil {
				return err
			}
			body = fmt.Sprintf("%s: your temporary driver app PIN is %s. You'll choose a new one when you sign in.", p.TenantName, temp)
		} else {
			temp = temporaryPassword()
			h, err := hash.Password(temp)
			if err != nil {
				return err
			}
			if err := q.UpdatePassword(ctx, db.UpdatePasswordParams{ID: u.ID, PasswordHash: &h, PasswordResetRequired: true}); err != nil {
				return err
			}
			body = fmt.Sprintf("%s: your temporary dashboard password is %s. You'll choose a new one when you sign in.", p.TenantName, temp)
		}
		if err := q.RevokeAllUserSessions(ctx, u.ID); err != nil {
			return err
		}
		if u.Phone != nil {
			s.enqueueSMS(ctx, q, after, outboundSMS{TenantID: p.TenantID, Phone: *u.Phone, Body: body, Purpose: PurposeCredential, Sensitive: true})
			texted = true
		}
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "credential_reset", "user", u.ID, nil, nil)
		return nil
	})
	return temp, texted, err
}

func checkPassword(pw string) error {
	if utf8.RuneCountInString(pw) < 8 {
		return errWeakPass
	}
	if len(pw) > 72 { // bcrypt ignores anything longer
		return &Error{Kind: KindInvalid, Field: "new_password", Message: "Use at most 72 characters."}
	}
	return nil
}

// checkPIN rejects PINs that are not 4–6 digits or are trivially guessable.
func checkPIN(pin string) error {
	if len(pin) < 4 || len(pin) > 6 {
		return errWeakPIN
	}
	same, up, down := true, true, true
	for i := 0; i < len(pin); i++ {
		if pin[i] < '0' || pin[i] > '9' {
			return errWeakPIN
		}
		if i > 0 {
			same = same && pin[i] == pin[i-1]
			up = up && pin[i] == pin[i-1]+1
			down = down && pin[i] == pin[i-1]-1
		}
	}
	if same || up || down {
		return errWeakPIN
	}
	return nil
}

const passwordAlphabet = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789"

// temporaryPassword is ten characters with nothing easily misread.
func temporaryPassword() string {
	b := []byte(randomToken(16))
	out := make([]byte, 0, 10)
	for _, c := range b {
		if strings.IndexByte(passwordAlphabet, c) >= 0 {
			out = append(out, c)
		}
		if len(out) == 10 {
			return string(out)
		}
	}
	return temporaryPassword()
}
