package payments

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func newMock() *Mock {
	return NewMock("telebirr", "telebirr", []byte("test-secret-0123456789abcdef0123"),
		func(ref string) string { return "http://localhost:8090/dev/payments/" + ref })
}

func notifyRequest(form url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/notify", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestMockSendsThePayerToItsCheckoutPage(t *testing.T) {
	co, err := newMock().Start(context.Background(), Charge{Reference: "PAY-1", Amount: 287500, Description: "Invoice INV-2026-00001"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(co.ProviderRef, "MOCK-") || co.RedirectURL != "http://localhost:8090/dev/payments/PAY-1" {
		t.Errorf("checkout = %+v", co)
	}
}

func TestMockVerifiesSignedNotifications(t *testing.T) {
	m := newMock()
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
	other := NewMock("cbebirr", "CBE Birr", []byte("test-secret-0123456789abcdef0123"), nil)
	form.Set("signature", other.Sign(n))
	if _, err := m.Verify(notifyRequest(form)); err != ErrBadSignature {
		t.Errorf("err = %v, want ErrBadSignature", err)
	}

	// Its handler settles what it verified.
	l := &ledger{}
	res := httptest.NewRecorder()
	form.Set("signature", m.Sign(n))
	m.Handler(l, l.fail).ServeHTTP(res, notifyRequest(form))
	if res.Code != http.StatusOK || len(l.settled) != 1 || l.settled[0] != n {
		t.Errorf("notify: status %d, settled %+v", res.Code, l.settled)
	}
}

func TestFormatETB(t *testing.T) {
	for in, want := range map[int64]string{0: "0.00", 5: "0.05", 250000: "2,500.00", 287550: "2,875.50", 123456789: "1,234,567.89", -37500: "-375.00"} {
		if got := FormatETB(in); got != want {
			t.Errorf("FormatETB(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestDecimals(t *testing.T) {
	for in, want := range map[int64]string{0: "0.00", 5: "0.05", 287500: "2875.00", 123456789: "1234567.89"} {
		if got := Decimal(in); got != want {
			t.Errorf("Decimal(%d) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]int64{"2875": 287500, "2875.5": 287550, "2875.50": 287550, " 10.00 ": 1000, "0.05": 5} {
		if got, err := ParseDecimal(in); err != nil || got != want {
			t.Errorf("ParseDecimal(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "-5", "1,000.00", "10.005", "ten", "1e3", ".5"} {
		if got, err := ParseDecimal(in); err == nil {
			t.Errorf("ParseDecimal(%q) = %d, want an error", in, got)
		}
	}
}

// ledger records what providers' calls asked of billing.
type ledger struct {
	mu      sync.Mutex
	bills   map[string]Bill
	settled []Notification
	paid    []BillPayment
	billErr error
	failed  []error
}

func (l *ledger) Settle(_ context.Context, _ string, n Notification) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.settled = append(l.settled, n)
	return nil
}

func (l *ledger) Bill(_ context.Context, _ string, reference string) (Bill, error) {
	if l.billErr != nil {
		return Bill{}, l.billErr
	}
	b, ok := l.bills[reference]
	if !ok {
		return Bill{}, ErrUnknownBill
	}
	return b, nil
}

func (l *ledger) BillPaid(_ context.Context, _ string, p BillPayment) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.bills[p.Reference]; !ok {
		return ErrUnknownBill
	}
	l.paid = append(l.paid, p)
	return nil
}

func (l *ledger) fail(w http.ResponseWriter, _ *http.Request, err error) {
	l.mu.Lock()
	l.failed = append(l.failed, err)
	l.mu.Unlock()
	switch {
	case errors.Is(err, ErrBadSignature), errors.Is(err, ErrUnauthorized):
		w.WriteHeader(http.StatusUnauthorized)
	case errors.Is(err, ErrMalformed):
		w.WriteHeader(http.StatusBadRequest)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}
