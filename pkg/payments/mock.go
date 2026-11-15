package payments

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Mock imitates a checkout provider without taking money. Its checkout page
// is a page of this app, at PromptURL(reference), which approves or
// declines the payment with a signed notification to the same URL a real
// provider would use.
type Mock struct {
	code, label string
	secret      []byte
	promptURL   func(reference string) string
}

// NewMock returns a stand-in provider. secret signs its notifications.
func NewMock(code, label string, secret []byte, promptURL func(string) string) *Mock {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("payments-mock:" + code))
	return &Mock{code: code, label: label, secret: m.Sum(nil), promptURL: promptURL}
}

func (m *Mock) Code() string         { return m.code }
func (m *Mock) Label() string        { return m.label }
func (*Mock) Mock() bool             { return true }
func (*Mock) Flow() Flow             { return FlowCheckout }
func (*Mock) CheckoutOrigin() string { return "" }

func (m *Mock) Start(_ context.Context, c Charge) (Checkout, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return Checkout{}, err
	}
	return Checkout{ProviderRef: "MOCK-" + strings.ToUpper(hex.EncodeToString(b)), RedirectURL: m.promptURL(c.Reference)}, nil
}

// Sign returns the signature a notification with these values carries.
func (m *Mock) Sign(n Notification) string {
	h := hmac.New(sha256.New, m.secret)
	h.Write(fmt.Appendf(nil, "%s|%s|%s|%d", n.Reference, n.ProviderRef, n.Status, n.Amount))
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

// Handler serves POST /notify.
func (m *Mock) Handler(l Ledger, fail ErrorWriter) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /notify", func(w http.ResponseWriter, r *http.Request) {
		n, err := m.Verify(r)
		if err == nil {
			err = l.Settle(r.Context(), m.code, n)
		}
		if err != nil {
			fail(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	return mux
}
