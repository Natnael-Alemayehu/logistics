package payments

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/natnael-alemayehu/logistics/pkg/console"
)

func newMock(c *console.Console) *Mock {
	return NewMock("telebirr", "telebirr", []byte("test-secret-0123456789abcdef0123"),
		func(ref string) string { return "http://localhost:8090/dev/payments/" + ref }, c)
}

func notifyRequest(form url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/payments/telebirr/notify", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestMockPromptsThePayerOnTheConsole(t *testing.T) {
	c := console.New(nil)
	m := newMock(c)
	co, err := m.Start(context.Background(), Charge{Reference: "PAY-1", Amount: 287500, Description: "Invoice INV-2026-00001", Payer: "0911223344"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(co.ProviderRef, "MOCK-") || co.RedirectURL != "" {
		t.Errorf("checkout = %+v", co)
	}
	it, ok := c.Last("payment", "0911223344")
	if !ok || !strings.Contains(it.Title, "2,875.00 ETB") || !strings.Contains(it.Body, "/dev/payments/PAY-1") {
		t.Fatalf("console has %+v", it)
	}
}

func TestMockVerifiesSignedNotifications(t *testing.T) {
	m := newMock(console.New(nil))
	n := Notification{Reference: "PAY-1", ProviderRef: "MOCK-AB", Status: StatusPaid, Amount: 287500}
	form := url.Values{
		"reference": {n.Reference}, "provider_ref": {n.ProviderRef}, "status": {n.Status},
		"amount": {strconv.FormatInt(n.Amount, 10)}, "signature": {m.Sign(n)},
	}
	got, err := m.Verify(notifyRequest(form))
	if err != nil || got != n {
		t.Fatalf("Verify = %+v, %v", got, err)
	}

	// Changing anything the signature covers is caught.
	for field, value := range map[string]string{"amount": "100", "status": StatusFailed, "reference": "PAY-2", "signature": "00"} {
		f := url.Values{}
		for k, v := range form {
			f[k] = v
		}
		f.Set(field, value)
		if _, err := m.Verify(notifyRequest(f)); err == nil {
			t.Errorf("accepted a notification with %s changed", field)
		}
	}

	// Another provider's (or deployment's) signature is not valid here.
	other := NewMock("cbebirr", "CBE Birr", []byte("test-secret-0123456789abcdef0123"), nil, nil)
	form.Set("signature", other.Sign(n))
	if _, err := m.Verify(notifyRequest(form)); err != ErrBadSignature {
		t.Errorf("err = %v, want ErrBadSignature", err)
	}
}

func TestFormatETB(t *testing.T) {
	for in, want := range map[int64]string{0: "0.00", 5: "0.05", 250000: "2,500.00", 287550: "2,875.50", 123456789: "1,234,567.89", -37500: "-375.00"} {
		if got := FormatETB(in); got != want {
			t.Errorf("FormatETB(%d) = %q, want %q", in, got, want)
		}
	}
}
