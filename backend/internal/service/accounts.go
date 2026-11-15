package service

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/phone"
	"github.com/natnael-alemayehu/kiflet/pkg/hash"
)

// Links sent by email. A reset link lets staff who forgot their password
// choose a new one (they sign in with email, so that is where help goes); a
// confirmation link proves the address on an account receives mail, since
// password resets, invoices and receipts are sent there.
const (
	linkPasswordReset = "password_reset"
	linkEmailVerify   = "email_verify"

	resetLinkTTL  = time.Hour
	verifyLinkTTL = 7 * 24 * time.Hour
	linksPerHour  = 3
)

var (
	errLinkExpired  = &Error{Kind: KindNotFound, Message: "This link has expired or has already been used."}
	errLinksTooMany = &Error{Kind: KindLimit, Message: "We've sent several emails in the last hour. Check your inbox and spam folder, or try again later."}
)

// RequestPasswordReset emails a reset link to the staff member who signs in
// with this address. The answer is the same whether or not the address has
// an account, so the form can't be used to find out who works where; for
// the same reason, requests beyond the hourly limit are dropped silently
// (the links already sent still work).
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string, ci ClientInfo) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
		return invalid("email", "Enter the email address you sign in with.")
	}
	u, err := s.q.GetUserByEmail(ctx, email)
	if isNoRows(err) {
		return nil
	}
	if err != nil {
		return err
	}
	// Only a confirmed address gets a link: an unconfirmed one may be a
	// mistyped address that belongs to someone else. Such staff reset by
	// phone, or ask an administrator.
	if u.Role == model.RoleDriver || !u.IsActive || u.Email == nil || !u.EmailVerifiedAt.Valid {
		return nil
	}
	recent, err := s.q.CountEmailTokensSince(ctx, db.CountEmailTokensSinceParams{
		TenantID: u.TenantID, UserID: u.ID, Purpose: linkPasswordReset, CreatedAt: toTimestamp(s.now().Add(-time.Hour)),
	})
	if err != nil || recent >= linksPerHour {
		return err
	}
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		link, err := s.emailLink(ctx, q, u, linkPasswordReset, "/reset/", resetLinkTTL)
		if err != nil {
			return err
		}
		s.enqueueEmail(ctx, q, after, outboundEmail{
			TenantID: u.TenantID, To: *u.Email, Purpose: EmailPasswordReset, Sensitive: true,
			Subject: "Reset your " + s.cfg.AppName + " password",
			Body: s.emailBody(u.FullName,
				"Someone asked to reset the password for your "+s.cfg.AppName+" account. To choose a new one, open this link within the hour:",
				link,
				"If it wasn't you, ignore this email: your password stays as it is."),
		})
		s.audit(ctx, q, userActor(u, ci), "password_reset_requested", "user", u.ID, nil, nil)
		return nil
	})
}

// ResetFor is whose password a reset link changes.
type ResetFor struct {
	Name  string
	Email string
}

// PasswordResetFor reports whose password a reset link is for, without
// using it, so the form can greet them. Opening the link must not use it up:
// mail scanners open links too.
func (s *AuthService) PasswordResetFor(ctx context.Context, token string) (ResetFor, error) {
	t, err := s.q.GetEmailToken(ctx, db.GetEmailTokenParams{TokenHash: hashToken(token), Purpose: linkPasswordReset})
	if isNoRows(err) {
		return ResetFor{}, errLinkExpired
	}
	if err != nil {
		return ResetFor{}, err
	}
	if !linkStillValid(t) {
		return ResetFor{}, errLinkExpired
	}
	return ResetFor{Name: t.FullName, Email: t.Email}, nil
}

// ResetPassword sets a new password from a reset link. Every other session
// ends, as the old password may be what someone else used, and the person
// is signed in here.
func (s *AuthService) ResetPassword(ctx context.Context, token, password string, ci ClientInfo) (*LoginResult, error) {
	if err := checkPassword(password); err != nil {
		return nil, err
	}
	h, err := hash.Password(password)
	if err != nil {
		return nil, err
	}
	var u db.User
	err = s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		t, err := q.GetEmailToken(ctx, db.GetEmailTokenParams{TokenHash: hashToken(token), Purpose: linkPasswordReset})
		if isNoRows(err) {
			return errLinkExpired
		}
		if err != nil {
			return err
		}
		if !linkStillValid(t) {
			return errLinkExpired
		}
		if !t.TenantActive {
			return s.suspended()
		}
		if err := q.ResetUserPassword(ctx, db.ResetUserPasswordParams{ID: t.UserID, TenantID: t.TenantID, PasswordHash: &h}); err != nil {
			return err
		}
		if err := q.ExpireEmailTokens(ctx, db.ExpireEmailTokensParams{TenantID: t.TenantID, UserID: t.UserID, Purpose: linkPasswordReset}); err != nil {
			return err
		}
		if err := q.RevokeAllUserSessions(ctx, t.UserID); err != nil {
			return err
		}
		if u, err = q.GetUserByID(ctx, db.GetUserByIDParams{ID: t.UserID, TenantID: t.TenantID}); err != nil {
			return err
		}
		s.audit(ctx, q, userActor(u, ci), "password_reset", "user", u.ID, nil, nil)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.startSession(ctx, u, SessionWeb, false, ci)
}

// RequestPasswordCode texts a reset code to the staff member who signs in
// with this phone: staff without a confirmed email address have no other
// way to reset their password themselves. As with RequestPasswordReset, the
// answer is the same whoever holds the number, hourly limit included. It
// returns the number as stored, for the page that takes the code.
func (s *AuthService) RequestPasswordCode(ctx context.Context, rawPhone string) (string, error) {
	p, err := phone.Normalize(rawPhone)
	if err != nil {
		return "", invalid("phone", err.Error())
	}
	u, err := s.q.GetUserByPhone(ctx, &p)
	if err != nil && !isNoRows(err) {
		return "", err
	}
	if err != nil || u.Role == model.RoleDriver || !u.IsActive || u.PasswordHash == nil {
		return p, s.issueCode(ctx, pgtype.UUID{}, p, codePurposeStaffReset, nil)
	}
	return p, s.issueCode(ctx, u.TenantID, p, codePurposeStaffReset, func(code string) string {
		return fmt.Sprintf("%s: your password reset code is %s. It expires in 10 minutes. Never share it.", s.cfg.AppName, code)
	})
}

// ResetPasswordByCode sets a new password with a texted code. As with a
// reset link, every other session ends and the person is signed in here.
func (s *AuthService) ResetPasswordByCode(ctx context.Context, rawPhone, code, password string, ci ClientInfo) (*LoginResult, error) {
	p, err := phone.Normalize(rawPhone)
	if err != nil {
		return nil, invalid("phone", err.Error())
	}
	if err := checkPassword(password); err != nil {
		return nil, err
	}
	codeID, err := s.verifyCode(ctx, p, codePurposeStaffReset, strings.TrimSpace(code))
	if err != nil {
		return nil, err
	}
	h, err := hash.Password(password)
	if err != nil {
		return nil, err
	}
	var u db.User
	err = s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		if err := consumeCode(ctx, q, codeID); err != nil {
			return err
		}
		if u, err = q.GetUserByPhone(ctx, &p); isNoRows(err) {
			return errCodeInvalid
		} else if err != nil {
			return err
		}
		if u.Role == model.RoleDriver || !u.IsActive || u.PasswordHash == nil {
			return errCodeInvalid
		}
		if u.Role != model.RolePlatformAdmin {
			t, err := q.GetTenantByID(ctx, u.TenantID)
			if err != nil {
				return err
			}
			if !t.IsActive {
				return s.suspended()
			}
		}
		if err := q.UpdatePassword(ctx, db.UpdatePasswordParams{ID: u.ID, PasswordHash: &h, PasswordResetRequired: false}); err != nil {
			return err
		}
		if err := q.RevokeAllUserSessions(ctx, u.ID); err != nil {
			return err
		}
		s.audit(ctx, q, userActor(u, ci), "password_reset", "user", u.ID, nil, map[string]any{"by": "sms"})
		return nil
	})
	if err != nil {
		return nil, err
	}
	u.LockedUntil = pgtype.Timestamptz{}
	return s.startSession(ctx, u, SessionWeb, false, ci)
}

// linkStillValid checks a link against the account as it is now: a
// deactivated person can't use one, nor can anyone once the address it was
// sent to is no longer theirs.
func linkStillValid(t db.GetEmailTokenRow) bool {
	return t.IsActive && t.Role != model.RoleDriver && t.CurrentEmail != nil && *t.CurrentEmail == t.Email
}

// ResendVerification sends the caller a new link to confirm their address.
func (s *AuthService) ResendVerification(ctx context.Context, p *Principal) error {
	u, err := s.q.GetUserByID(ctx, db.GetUserByIDParams{ID: p.UserID, TenantID: p.TenantID})
	if err != nil {
		return err
	}
	if u.Email == nil {
		return invalid("email", "Add an email address to your account first.")
	}
	if u.EmailVerifiedAt.Valid {
		return conflict("Your email address is already confirmed.")
	}
	recent, err := s.q.CountEmailTokensSince(ctx, db.CountEmailTokensSinceParams{
		TenantID: u.TenantID, UserID: u.ID, Purpose: linkEmailVerify, CreatedAt: toTimestamp(s.now().Add(-time.Hour)),
	})
	if err != nil {
		return err
	}
	if recent >= linksPerHour {
		return errLinksTooMany
	}
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		return s.sendEmailVerification(ctx, q, after, u, "")
	})
}

// VerifyEmail confirms an address from its link. Whoever opens it need not
// be signed in. The link keeps working until it expires, so opening it again
// says the same thing; it stops counting if the address has changed since.
func (s *AuthService) VerifyEmail(ctx context.Context, token string, ci ClientInfo) (email string, err error) {
	t, err := s.q.GetEmailToken(ctx, db.GetEmailTokenParams{TokenHash: hashToken(token), Purpose: linkEmailVerify})
	if isNoRows(err) {
		return "", errLinkExpired
	}
	if err != nil {
		return "", err
	}
	if !linkStillValid(t) {
		return "", errLinkExpired
	}
	n, err := s.q.SetEmailVerified(ctx, db.SetEmailVerifiedParams{ID: t.UserID, TenantID: t.TenantID, Email: &t.Email})
	if err != nil {
		return "", err
	}
	if n > 0 {
		s.audit(ctx, s.q, Actor{UserID: t.UserID, TenantID: t.TenantID, Name: t.FullName, Role: t.Role, IP: ci.IP, UserAgent: ci.UserAgent},
			"email_verified", "user", t.UserID, nil, map[string]any{"email": t.Email})
	}
	return t.Email, nil
}

// sendEmailVerification queues a confirmation link to u's address, within
// the transaction that set it. intro, if given, explains why it was sent.
func (c *core) sendEmailVerification(ctx context.Context, q *db.Queries, after *afterCommit, u db.User, intro string) error {
	if u.Email == nil {
		return nil
	}
	link, err := c.emailLink(ctx, q, u, linkEmailVerify, "/verify-email/", verifyLinkTTL)
	if err != nil {
		return err
	}
	paragraphs := []string{}
	if intro != "" {
		paragraphs = append(paragraphs, intro)
	}
	paragraphs = append(paragraphs,
		"Please confirm that "+*u.Email+" is your email address. Password resets, invoices and receipts are sent here. Open this link within 7 days:",
		link,
		"If you don't know why you got this email, ignore it.")
	c.enqueueEmail(ctx, q, after, outboundEmail{
		TenantID: u.TenantID, To: *u.Email, Purpose: EmailVerification, Sensitive: true,
		Subject: "Confirm your email address", Body: c.emailBody(u.FullName, paragraphs...),
	})
	return nil
}

// emailLink stores a new link for u and returns its URL. Only a hash of the
// token is kept.
func (c *core) emailLink(ctx context.Context, q *db.Queries, u db.User, purpose, path string, ttl time.Duration) (string, error) {
	token := randomToken(32)
	err := q.CreateEmailToken(ctx, db.CreateEmailTokenParams{
		TenantID: u.TenantID, UserID: u.ID, Purpose: purpose, Email: *u.Email,
		TokenHash: hashToken(token), ExpiresAt: toTimestamp(c.now().Add(ttl)),
	})
	return c.cfg.BaseURL + path + token, err
}

// userActor is a person acting on their own account before they are signed in.
func userActor(u db.User, ci ClientInfo) Actor {
	return Actor{UserID: u.ID, TenantID: u.TenantID, Name: u.FullName, Role: u.Role, IP: ci.IP, UserAgent: ci.UserAgent}
}

// emailChanged reports whether an edit gave an account a new address that
// now needs confirming.
func emailChanged(before, after db.User) bool {
	return after.Email != nil && !after.EmailVerifiedAt.Valid && (before.Email == nil || *before.Email != *after.Email)
}
