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

	"github.com/natnael-alemayehu/logistics/internal/db"
)

// One-time codes sent by SMS, for driver PIN reset and customer lookups.
const (
	codePurposePINReset    = "pin_reset"
	codePurposeTrackLookup = "track_lookup"

	codeTTL          = 10 * time.Minute
	codeMaxAttempts  = 5
	codeMaxPerWindow = 3
	codeWindow       = time.Hour
)

var (
	errCodeInvalid  = &Error{Kind: KindInvalid, Field: "code", Message: "That code is wrong or has expired. Request a new one."}
	errCodeTooMany  = &Error{Kind: KindLimit, Message: "Too many codes requested for this number. Try again in an hour."}
	errCodeAttempts = &Error{Kind: KindInvalid, Field: "code", Message: "Too many wrong attempts. Request a new code."}
)

// hashCode keys a code to its phone and purpose with the deployment secret,
// so a leaked table of six-digit codes cannot be reversed by brute force.
func (c *core) hashCode(phone, purpose, code string) string {
	m := hmac.New(sha256.New, c.cfg.Secret)
	fmt.Fprintf(m, "%s|%s|%s", purpose, phone, code)
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

// issueCode creates a code and queues it by SMS. text formats the message
// around the code.
func (c *core) issueCode(ctx context.Context, tenantID pgtype.UUID, phone, purpose string, text func(code string) string) error {
	recent, err := c.q.CountVerificationCodesSince(ctx, db.CountVerificationCodesSinceParams{
		Phone: phone, Purpose: purpose, CreatedAt: toTimestamp(c.now().Add(-codeWindow)),
	})
	if err != nil {
		return err
	}
	if recent >= codeMaxPerWindow {
		return errCodeTooMany
	}
	code := randomDigits(6)
	return c.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		if _, err := q.CreateVerificationCode(ctx, db.CreateVerificationCodeParams{
			Phone: phone, Purpose: purpose, CodeHash: c.hashCode(phone, purpose, code),
			ExpiresAt: toTimestamp(c.now().Add(codeTTL)),
		}); err != nil {
			return err
		}
		c.enqueueSMS(ctx, q, after, outboundSMS{
			TenantID: tenantID, Phone: phone, Body: text(code), Purpose: PurposeOTP, Sensitive: true,
		})
		return nil
	})
}

// checkCode verifies and consumes a code.
func (c *core) checkCode(ctx context.Context, phone, purpose, code string) error {
	vc, err := c.q.GetActiveVerificationCode(ctx, db.GetActiveVerificationCodeParams{Phone: phone, Purpose: purpose})
	if isNoRows(err) {
		return errCodeInvalid
	}
	if err != nil {
		return err
	}
	if vc.Attempts >= codeMaxAttempts {
		return errCodeAttempts
	}
	if !hmac.Equal([]byte(vc.CodeHash), []byte(c.hashCode(phone, purpose, code))) {
		_ = c.q.IncrementVerificationAttempts(ctx, vc.ID)
		return errCodeInvalid
	}
	return c.q.ConsumeVerificationCode(ctx, vc.ID)
}

// Sign returns an HMAC of value under the deployment secret, for short-lived
// cookies such as a verified lookup phone.
func (c *core) Sign(value string) string {
	m := hmac.New(sha256.New, c.cfg.Secret)
	m.Write([]byte(value))
	return hex.EncodeToString(m.Sum(nil))
}
