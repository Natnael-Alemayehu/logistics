//go:build e2e

package e2e

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// TestPayingWithTelebirr: a company pays its invoice at telebirr's checkout.
// The app sends the payer there, the payer approves with their PIN, telebirr
// notifies the app over HTTPS at its public address and sends the payer back,
// and the receipt is emailed.
func TestPayingWithTelebirr(t *testing.T) {
	office := signUp(t, "Telebirr Paying Freight")
	invoice := choosePlan(t, office, "starter")
	number := invoiceNumberRe.FindString(office.get(invoice).Body)

	p := office.mustPost(invoice+"/pay", url.Values{"provider": {"telebirr"}})
	checkout := p.Location
	if !strings.HasPrefix(checkout, sandboxURL+"/telebirr/checkout?") {
		t.Fatalf("paying went to %q, want telebirr's checkout", checkout)
	}
	// The checkout may receive the form: the browser would refuse the
	// redirect otherwise.
	if csp := office.get(invoice).Header.Get("Content-Security-Policy"); !strings.Contains(csp, "form-action 'self' "+sandboxURL) {
		t.Errorf("the invoice page's form-action doesn't allow telebirr's checkout: %q", csp)
	}

	payer := newBrowser(t)
	page := payer.get(checkout)
	form := checkoutFormRe.FindStringSubmatch(page.Body)
	if page.Status != http.StatusOK || form == nil {
		t.Fatalf("telebirr's checkout: status %d", page.Status)
	}
	before := phoneOf(t, office.phone).Telebirr
	p = payer.post(sandboxURL+form[1], url.Values{"action": {"pay"}, "phone": {office.phone}, "pin": {walletPIN}})
	if p.Status != http.StatusSeeOther || !strings.HasPrefix(p.Location, appURL+"/app/billing/payments/") {
		t.Fatalf("paying at the checkout: status %d to %q, want the app's payment page", p.Status, p.Location)
	}
	if page := office.get(p.Location); !strings.Contains(page.Body, "Paid. Thank you") {
		t.Errorf("the payment page doesn't say it's paid: status %d", page.Status)
	}
	if !strings.Contains(office.get(invoice).Body, "Paid") {
		t.Error("the invoice isn't paid")
	}
	waitForEmail(t, office.email, "Receipt for invoice "+number)
	if after := phoneOf(t, office.phone).Telebirr; after >= before {
		t.Errorf("the payer's wallet went from %d to %d santim", before, after)
	}
}

// TestPayingWithCBEBirr: a company pays by reference from a CBE Birr wallet.
// CBE Birr's servers ask the app about the reference, check the amount and
// confirm the payment, each over HTTPS with their login.
func TestPayingWithCBEBirr(t *testing.T) {
	office := signUp(t, "Reference Paying Freight")
	invoice := choosePlan(t, office, "starter")
	page := office.get(invoice)
	number := invoiceNumberRe.FindString(page.Body)
	ref := referenceRe.FindStringSubmatch(page.Body)
	if ref == nil {
		t.Fatal("the invoice shows no reference to pay with CBE Birr")
	}

	var res struct {
		Error   string `json:"error"`
		Payment struct {
			TransID string `json:"trans_id"`
		} `json:"payment"`
	}
	if code := sandbox(t, http.MethodPost, "/api/cbebirr/payments", map[string]any{"phone": office.phone, "reference": ref[1], "pin": walletPIN}, &res); code != http.StatusOK || res.Payment.TransID == "" {
		t.Fatalf("paying by reference: status %d, %q", code, res.Error)
	}
	if !strings.Contains(office.get(invoice).Body, "Paid") {
		t.Error("the invoice isn't paid")
	}
	waitForEmail(t, office.email, "Receipt for invoice "+number)

	// Paid already: CBE Birr is told so, and takes nothing more.
	before := phoneOf(t, office.phone).CBEBirr
	if code := sandbox(t, http.MethodPost, "/api/cbebirr/payments", map[string]any{"phone": office.phone, "reference": ref[1], "pin": walletPIN}, &res); code == http.StatusOK {
		t.Error("an invoice already paid was paid again")
	}
	if after := phoneOf(t, office.phone).CBEBirr; after != before {
		t.Errorf("paying a paid invoice took %d santim", before-after)
	}
}

var (
	invoicePathRe   = regexp.MustCompile(`^/app/billing/invoices/[0-9a-f-]{36}$`)
	invoiceNumberRe = regexp.MustCompile(`INV-\d{4}-\d+`)
	checkoutFormRe  = regexp.MustCompile(`<form[^>]*action="(/telebirr/checkout/[^"]+)"`)
	referenceRe     = regexp.MustCompile(`<dt>Reference</dt><dd class="mono">([^<]+)</dd>`)
)

// choosePlan moves a company onto a plan and returns its invoice's page.
func choosePlan(t *testing.T, b *browser, plan string) string {
	t.Helper()
	p := b.mustPost("/app/billing/plan", url.Values{"plan": {plan}, "cycle": {"monthly"}})
	if !invoicePathRe.MatchString(p.Location) {
		t.Fatalf("choosing %s went to %q, want its invoice", plan, p.Location)
	}
	return p.Location
}
