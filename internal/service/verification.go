package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/pkg/metrics"
)

// One-time codes sent by SMS: driver PIN reset, staff password reset,
// customer lookups, and the phone check at sign-up.
const (
	codePurposePINReset    = "pin_reset"
	codePurposeStaffReset  = "staff_reset"
	codePurposeTrackLookup = "track_lookup"
	codePurposeSignup      = "signup"

	codeTTL          = 10 * time.Minute
	codeMaxAttempts  = 5
	codeMaxPerWindow = 3
	codeWindow       = time.Hour
)

var (
	errCodeInvalid  = &Error{Kind: KindInvalid, Field: "code", Message: "That code is wrong or has expired. Request a new one.", Key: "error.code_invalid"}
	errCodeTooMany  = &Error{Kind: KindLimit, Message: "Too many codes requested for this number. Try again in an hour.", Key: "error.code_too_many"}
	errCodeAttempts = &Error{Kind: KindInvalid, Field: "code", Message: "Too many wrong attempts. Request a new code.", Key: "error.code_attempts"}
	errCodeBudget   = &Error{Kind: KindLimit, Message: "We can't send codes right now. Please try again in a few hours.", Key: "error.code_budget"}
)

// hashCode keys a code to its phone and purpose with the deployment secret,
// so a leaked table of six-digit codes cannot be reversed by brute force.
func (c *core) hashCode(phone, purpose, code string) string {
	m := hmac.New(sha256.New, c.cfg.Secret)
	m.Write(fmt.Appendf(nil, "%s|%s|%s", purpose, phone, code))
	return hex.EncodeToString(m.Sum(nil))
}

func randomDigits(n int) string {
	b := make([]byte, n)
	for i := range b {
		d, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			panic(fmt.Sprintf("crypto/rand: %v", err))
		}
		b[i] = byte('0' + d.Int64())
	}
	return string(b)
}

// issueCode creates a code and queues it by SMS; text formats the message
// around the code. With a nil text the code is only recorded, not sent:
// callers use that for a number with no account behind it, so that it
// counts towards the hourly limit like any other and the answer to the
// request is the same whoever holds the number.
func (c *core) issueCode(ctx context.Context, tenantID pgtype.UUID, phone, purpose string, text func(code string) string) error {
	code := randomDigits(6)
	return c.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		// One request at a time for a number and purpose, so that requests
		// sent together can't all pass the count below.
		if err := q.LockVerificationSubject(ctx, purpose+"|"+phone); err != nil {
			return err
		}
		recent, err := q.CountVerificationCodesSince(ctx, db.CountVerificationCodesSinceParams{
			Phone: phone, Purpose: purpose, CreatedAt: toTimestamp(c.now().Add(-codeWindow)),
		})
		if err != nil {
			return err
		}
		if recent >= codeMaxPerWindow {
			return errCodeTooMany
		}
		// Checked for codes recorded but not sent too, so that the answer
		// stays the same for every number.
		if err := c.withinCodeBudget(ctx, q); err != nil {
			return err
		}
		if _, err := q.CreateVerificationCode(ctx, db.CreateVerificationCodeParams{
			Phone: phone, Purpose: purpose, CodeHash: c.hashCode(phone, purpose, code),
			ExpiresAt: toTimestamp(c.now().Add(codeTTL)),
		}); err != nil {
			return err
		}
		if text != nil {
			c.enqueueSMS(ctx, q, after, outboundSMS{
				TenantID: tenantID, Phone: phone, Body: text(code), Purpose: PurposeOTP, Sensitive: true,
			})
		}
		return nil
	})
}

// withinCodeBudget refuses a code once the platform has texted its daily
// budget of them (Settings.CodesPerDay). Codes go to any number someone
// types in, so without a ceiling a script could run up the SMS bill. The
// share used is exported for the monitoring alert.
func (c *core) withinCodeBudget(ctx context.Context, q *db.Queries) error {
	budget := int64(c.cfg.CodesPerDay)
	if budget <= 0 {
		return nil
	}
	sent, err := q.CountCodesTextedSince(ctx, toTimestamp(c.now().Add(-24*time.Hour)))
	if err != nil {
		return err
	}
	if sent >= budget {
		metrics.VerificationBudgetUsed.Set(1)
		c.log.Error().Int64("sent", sent).Int64("budget", budget).Msg("daily budget of verification codes reached: codes refused")
		return errCodeBudget
	}
	metrics.VerificationBudgetUsed.Set(float64(sent+1) / float64(budget))
	if (sent+1)*5 == budget*4 {
		c.log.Warn().Int64("sent", sent+1).Int64("budget", budget).Msg("80% of the daily budget of verification codes used")
	}
	return nil
}

// verifyCode checks a code without using it up, and returns its ID for
// consumeCode. The try is counted first, in one statement that also enforces
// the limit, so guesses sent together can't get more than codeMaxAttempts
// tries between them; it is counted on its own, outside any transaction of
// the caller's, so that a wrong guess always counts.
func (c *core) verifyCode(ctx context.Context, phone, purpose, code string) (pgtype.UUID, error) {
	vc, err := c.q.GetActiveVerificationCode(ctx, db.GetActiveVerificationCodeParams{Phone: phone, Purpose: purpose})
	if isNoRows(err) {
		return pgtype.UUID{}, errCodeInvalid
	}
	if err != nil {
		return pgtype.UUID{}, err
	}
	if _, err := c.q.CountVerificationAttempt(ctx, db.CountVerificationAttemptParams{ID: vc.ID, MaxAttempts: codeMaxAttempts}); isNoRows(err) {
		return pgtype.UUID{}, errCodeAttempts
	} else if err != nil {
		return pgtype.UUID{}, err
	}
	if !hmac.Equal([]byte(vc.CodeHash), []byte(c.hashCode(phone, purpose, code))) {
		return pgtype.UUID{}, errCodeInvalid
	}
	return vc.ID, nil
}

// consumeCode uses up a code verifyCode accepted. In the caller's
// transaction, the code is spent only if what it unlocks is saved too; a
// code used meanwhile by another request is refused.
func consumeCode(ctx context.Context, q *db.Queries, id pgtype.UUID) error {
	n, err := q.ConsumeVerificationCode(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return errCodeInvalid
	}
	return nil
}

// checkCode verifies and consumes a code.
func (c *core) checkCode(ctx context.Context, phone, purpose, code string) error {
	id, err := c.verifyCode(ctx, phone, purpose, code)
	if err != nil {
		return err
	}
	return consumeCode(ctx, c.q, id)
}

// Sign returns an HMAC of value under the deployment secret, for short-lived
// cookies such as a verified lookup phone.
func (c *core) Sign(value string) string {
	m := hmac.New(sha256.New, c.cfg.Secret)
	m.Write([]byte(value))
	return hex.EncodeToString(m.Sum(nil))
}
