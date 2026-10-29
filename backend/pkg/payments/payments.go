// Package payments takes subscription payments from mobile-money providers.
//
// A provider starts a charge, the payer approves it on their phone, and the
// provider reports the outcome to a notification URL. Real telebirr and CBE
// Birr adapters need merchant credentials; until they exist the mock stands
// in. It prints each request to the terminal with a link to a page that
// approves or declines it, and that page posts a signed notification to the
// same URL a provider would.
package payments

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/natnael-alemayehu/logistics/pkg/console"
)

// Outcomes a provider reports.
const (
	StatusPaid   = "paid"
	StatusFailed = "failed"
)

// ErrBadSignature means a notification did not come from the provider.
var ErrBadSignature = errors.New("payment notification signature is invalid")

// Charge is a request for money.
type Charge struct {
	// Reference is ours and unique per attempt; the provider echoes it back.
	Reference string
	// Amount is in santim, hundredths of a birr.
	Amount      int64
	Description string
	// Payer is the phone number that receives the payment prompt.
	Payer string
}

// Checkout is the provider's answer to a charge.
type Checkout struct {
	ProviderRef string
	// RedirectURL is a page to send the payer to; empty when the prompt goes
	// straight to their phone.
	RedirectURL string
}

// Notification is a provider's report of how a charge ended.
type Notification struct {
	Reference   string
	ProviderRef string
	Status      string
	Amount      int64
}

// Provider is a payment method.
type Provider interface {
	// Code names the provider in URLs and records, such as "telebirr".
	Code() string
	// Label is its name for people.
	Label() string
	// Mock reports whether it only pretends to take money.
	Mock() bool
	Start(ctx context.Context, c Charge) (Checkout, error)
	// Verify checks that a notification came from the provider and returns
	// what it reports.
	Verify(r *http.Request) (Notification, error)
}

// Mock imitates a provider whose prompt goes to the payer's phone. The phone
// is the terminal, and the prompt links to a page of this app, at
// PromptURL(reference), standing in for it.
type Mock struct {
	code, label string
	secret      []byte
	promptURL   func(reference string) string
	console     *console.Console
}

// NewMock returns a mock provider. secret signs its notifications.
func NewMock(code, label string, secret []byte, promptURL func(string) string, c *console.Console) *Mock {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("payments-mock:" + code))
	return &Mock{code: code, label: label, secret: m.Sum(nil), promptURL: promptURL, console: c}
}

func (m *Mock) Code() string  { return m.code }
func (m *Mock) Label() string { return m.label }
func (*Mock) Mock() bool      { return true }

func (m *Mock) Start(_ context.Context, c Charge) (Checkout, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return Checkout{}, err
	}
	ref := "MOCK-" + strings.ToUpper(hex.EncodeToString(b))
	m.console.Print(console.Item{
		Kind: "payment", To: c.Payer,
		Title: fmt.Sprintf("%s asks you to pay %s ETB", m.label, FormatETB(c.Amount)),
		Body: c.Description + "\n\nApprove or decline it, as the payer would on their phone:\n" +
			m.promptURL(c.Reference),
	})
	return Checkout{ProviderRef: ref}, nil
}

// Sign returns the signature a notification with these values carries.
func (m *Mock) Sign(n Notification) string {
	h := hmac.New(sha256.New, m.secret)
	fmt.Fprintf(h, "%s|%s|%s|%d", n.Reference, n.ProviderRef, n.Status, n.Amount)
	return hex.EncodeToString(h.Sum(nil))
}

// Verify reads a notification posted as a form: reference, provider_ref,
// status, amount and signature.
func (m *Mock) Verify(r *http.Request) (Notification, error) {
	if err := r.ParseForm(); err != nil {
		return Notification{}, err
	}
	amount, err := strconv.ParseInt(r.PostFormValue("amount"), 10, 64)
	if err != nil {
		return Notification{}, ErrBadSignature
	}
	n := Notification{
		Reference: r.PostFormValue("reference"), ProviderRef: r.PostFormValue("provider_ref"),
		Status: r.PostFormValue("status"), Amount: amount,
	}
	if !hmac.Equal([]byte(r.PostFormValue("signature")), []byte(m.Sign(n))) {
		return Notification{}, ErrBadSignature
	}
	if n.Status != StatusPaid && n.Status != StatusFailed {
		return Notification{}, fmt.Errorf("unknown payment status %q", n.Status)
	}
	return n, nil
}

// FormatETB writes santim as birr with thousands separators: 287500 is
// "2,875.00".
func FormatETB(santim int64) string {
	sign := ""
	if santim < 0 {
		sign, santim = "-", -santim
	}
	birr := strconv.FormatInt(santim/100, 10)
	var b strings.Builder
	for i, r := range birr {
		if i > 0 && (len(birr)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return fmt.Sprintf("%s%s.%02d", sign, b.String(), santim%100)
}
