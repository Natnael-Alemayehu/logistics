//go:build integration

package integration

// The app's real SMS, telebirr and CBE Birr adapters against the provider
// sandbox (internal/sandbox), over HTTP as in production: texts go through
// Ethio telecom's gateway to the sandbox's phones, and invoices are paid at
// telebirr's checkout and by reference with CBE Birr, with the sandbox
// playing the people as well. Faults are turned on through the sandbox's
// API, as a script against a running one would.

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/config"
	"github.com/natnael-alemayehu/logistics/internal/sandbox"
	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/pkg/console"
	"github.com/natnael-alemayehu/logistics/pkg/payments"
)

// What the app and the sandbox agree on, as both read it from sandbox.env.
const (
	smsKey         = "integration-ethio-telecom-key"
	senderID       = "MENGED"
	smsSecret      = "integration-sms-webhook-secret"
	fabricAppID    = "integration-fabric-app"
	telebirrSecret = "integration-telebirr-app-secret"
	merchantAppID  = "1100000000000001"
	merchantCode   = "900001"
	walletPIN      = "1234"
	// startingBalance is what each of a sandbox phone's wallets holds.
	startingBalance = 200_000_00
)

// sandboxKeys are the key pairs `make sandbox-keys` writes: the app reads
// its private key and telebirr's public key from the files, and the
// sandbox the other halves.
var sandboxKeys = sync.OnceValues(func() (keyFiles, error) {
	var k keyFiles
	dir, err := os.MkdirTemp("", "menged-sandbox-keys-")
	if err != nil {
		return k, err
	}
	defer os.RemoveAll(dir)
	if _, err := sandbox.WriteKeys(dir); err != nil {
		return k, err
	}
	if k.merchant, k.telebirr, err = sandbox.LoadKeys(dir); err != nil {
		return k, err
	}
	if k.private, err = os.ReadFile(filepath.Join(dir, sandbox.MerchantPrivateKey)); err != nil {
		return k, err
	}
	k.public, err = os.ReadFile(filepath.Join(dir, sandbox.TelebirrPublicKey))
	return k, err
})

type keyFiles struct {
	merchant        *rsa.PublicKey
	telebirr        *rsa.PrivateKey
	private, public []byte // the app's: its own private key, telebirr's public key
}

// providers is a running sandbox, reached through its API.
type providers struct {
	t   *testing.T
	url string
}

// sandboxed starts a sandbox and an app whose SMS, telebirr and CBE Birr
// adapters are the real ones, pointed at it. change alters the sandbox's
// settings, to make it disagree with the app's.
func sandboxed(t *testing.T, change ...func(*sandbox.Config)) (*env, *providers) {
	t.Helper()
	keys, err := sandboxKeys()
	if err != nil {
		t.Fatal(err)
	}
	// The app reads its keys from files, as in production.
	dir := t.TempDir()
	private, public := filepath.Join(dir, sandbox.MerchantPrivateKey), filepath.Join(dir, sandbox.TelebirrPublicKey)
	for file, pem := range map[string][]byte{private: keys.private, public: keys.public} {
		if err := os.WriteFile(file, pem, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p := &providers{t: t}
	e := newEnv(t, func(s *setup) {
		cfg := sandbox.Config{
			AppURL: s.base,
			SMS:    sandbox.SMSConfig{APIKey: smsKey, SenderID: senderID, Number: "6060", Secret: smsSecret},
			Telebirr: sandbox.TelebirrConfig{
				FabricAppID: fabricAppID, AppSecret: telebirrSecret, MerchantAppID: merchantAppID, MerchantCode: merchantCode,
				MerchantKey: keys.merchant, Key: keys.telebirr,
			},
			CBEBirr: sandbox.CBEBirrConfig{MerchantID: cbeMerchant, Username: cbeUser, Password: cbePassword},
			Timing:  sandbox.Timing{Report: 20 * time.Millisecond, Retry: 50 * time.Millisecond, Delay: 300 * time.Millisecond},
		}
		for _, c := range change {
			c(&cfg)
		}
		sb, err := sandbox.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		srv := httptest.NewServer(sb.Handler())
		t.Cleanup(srv.Close)
		t.Cleanup(sb.Close)
		p.url = srv.URL

		s.cfg.SMSProvider, s.cfg.SMSInboundSecret, s.cfg.SandboxURL = "ethiotelecom", smsSecret, srv.URL
		s.cfg.EthioTelecomAPIURL, s.cfg.EthioTelecomAPIKey, s.cfg.EthioTelecomSenderID = srv.URL+"/ethiotelecom", smsKey, senderID
		s.cfg.PaymentProvider = "telebirr,cbebirr"
		s.cfg.Telebirr = config.TelebirrSettings{
			APIURL: srv.URL + "/telebirr", CheckoutURL: srv.URL + "/telebirr/checkout",
			FabricAppID: fabricAppID, AppSecret: telebirrSecret, MerchantAppID: merchantAppID, MerchantCode: merchantCode,
			PrivateKeyFile: private, PublicKeyFile: public,
		}
		s.texts = p.texts
	})
	return e, p
}

// ---------------------------------------------------------------------------
// The sandbox's API and pages
// ---------------------------------------------------------------------------

// call makes a call to the sandbox's API, decodes the answer into out and
// returns the status.
func (p *providers) call(method, path string, body, out any) int {
	p.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, p.url+path, r)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		p.t.Fatalf("sandbox %s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			p.t.Fatalf("sandbox %s %s: decode %q: %v", method, path, raw, err)
		}
	}
	return res.StatusCode
}

// faults turns the named faults on or off, leaving the others.
func (p *providers) faults(f map[string]any) {
	p.t.Helper()
	if code := p.call(http.MethodPatch, "/api/faults", f, nil); code != http.StatusOK {
		p.t.Fatalf("setting faults %v: status %d", f, code)
	}
}

type sandboxPhone struct {
	Texts    []sandboxText `json:"texts"`
	Telebirr int64         `json:"telebirr_santim"`
	CBEBirr  int64         `json:"cbebirr_santim"`
}

type sandboxText struct {
	At     time.Time `json:"at"`
	From   string    `json:"from"`
	Mine   bool      `json:"mine"`
	Body   string    `json:"body"`
	State  string    `json:"state"`
	Report string    `json:"report"`
}

func (p *providers) phone(number string) sandboxPhone {
	p.t.Helper()
	var ph sandboxPhone
	if code := p.call(http.MethodGet, "/api/phones/"+number, nil, &ph); code != http.StatusOK {
		p.t.Fatalf("sandbox phone %s: status %d", number, code)
	}
	return ph
}

// from is the texts a phone received from a sender.
func (ph sandboxPhone) from(sender string) []sandboxText {
	var out []sandboxText
	for _, tx := range ph.Texts {
		if !tx.Mine && tx.From == sender {
			out = append(out, tx)
		}
	}
	return out
}

// texts are the texts a phone received from the app, for env.received.
func (p *providers) texts(number string) []console.Item {
	var out []console.Item
	for _, tx := range p.phone(number).from(senderID) {
		out = append(out, console.Item{Kind: "sms", To: number, Body: tx.Body, At: tx.At})
	}
	return out
}

// text sends a text from a phone to the app's number, and returns the
// status the app answered the gateway's call with.
func (p *providers) text(from, body string) int {
	p.t.Helper()
	var res struct {
		MengedStatus int `json:"menged_status"`
	}
	if code := p.call(http.MethodPost, "/api/phones/"+from+"/texts", map[string]string{"text": body}, &res); code != http.StatusOK {
		p.t.Fatalf("texting from %s: status %d", from, code)
	}
	return res.MengedStatus
}

type heldCall struct {
	ID   int    `json:"id"`
	What string `json:"what"`
	Why  string `json:"why"`
}

// held is the calls to the app the sandbox kept back.
func (p *providers) held() []heldCall {
	p.t.Helper()
	var held []heldCall
	p.call(http.MethodGet, "/api/held", nil, &held)
	return held
}

// release sends a held call now.
func (p *providers) release(c heldCall) {
	p.t.Helper()
	if code := p.call(http.MethodPost, fmt.Sprintf("/api/held/%d", c.ID), map[string]string{"action": "send"}, nil); code != http.StatusOK {
		p.t.Fatalf("sending %s: status %d", c.What, code)
	}
}

type sandboxCall struct {
	Dir    string `json:"dir"`
	URL    string `json:"url"`
	Status int    `json:"status"`
}

// calls are the calls made to or from a path, oldest first: the log
// without the calls it shows held back.
func (p *providers) calls(path string) []sandboxCall {
	p.t.Helper()
	var all, out []sandboxCall
	p.call(http.MethodGet, "/api/events", nil, &all)
	for _, c := range all {
		if u, err := url.Parse(c.URL); err == nil && u.Path == path && c.Dir != "held" {
			out = append(out, c)
		}
	}
	return out
}

// payerBrowser opens the sandbox's pages as the payer's browser does,
// without following redirects.
var payerBrowser = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

// visit gets a page of the sandbox's, or posts form to it.
func (p *providers) visit(rawURL string, form url.Values) page {
	p.t.Helper()
	req, _ := http.NewRequest(http.MethodGet, rawURL, nil)
	if form != nil {
		req, _ = http.NewRequest(http.MethodPost, rawURL, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	res, err := payerBrowser.Do(req)
	if err != nil {
		p.t.Fatalf("%s %s: %v", req.Method, rawURL, err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return page{Status: res.StatusCode, Location: res.Header.Get("Location"), Body: string(body)}
}

var checkoutFormRe = regexp.MustCompile(`<form[^>]*action="(/telebirr/checkout/[^"]+)"`)

// pay answers telebirr's checkout as the payer, with their phone and PIN:
// it opens the checkout and sends its form.
func (p *providers) pay(checkout, phone, pin string) page {
	p.t.Helper()
	c := p.visit(checkout, nil)
	m := checkoutFormRe.FindStringSubmatch(c.Body)
	if c.Status != http.StatusOK || m == nil {
		p.t.Fatalf("telebirr's checkout: status %d\n%s", c.Status, c.Body)
	}
	return p.visit(p.url+m[1], url.Values{"action": {"pay"}, "phone": {phone}, "pin": {pin}})
}

type cbeOutcome struct {
	status  int
	Error   string `json:"error"`
	Payment struct {
		TransID string `json:"trans_id"`
		Amount  int64  `json:"amount_santim"`
	} `json:"payment"`
	Bill struct {
		ResultCode string `json:"ResultCode"`
	} `json:"bill"`
}

// payCBE pays a reference from a phone's CBE Birr wallet, as the payer
// does on the phone: CBE Birr looks it up, and pays what it says.
func (p *providers) payCBE(phone, reference string) cbeOutcome {
	p.t.Helper()
	var res cbeOutcome
	res.status = p.call(http.MethodPost, "/api/cbebirr/payments", map[string]any{"phone": phone, "reference": reference, "pin": walletPIN}, &res)
	return res
}

// telebirrCheckout starts paying an invoice with telebirr, and returns the
// payment page and telebirr's checkout, where the app sent the payer.
func (b *browser) telebirrCheckout(p *providers, invoiceID string) (status, checkout string) {
	b.e.t.Helper()
	res := b.mustPost("/app/billing/invoices/"+invoiceID+"/pay", url.Values{"provider": {"telebirr"}})
	if !strings.HasPrefix(res.Location, p.url+"/telebirr/checkout?") {
		b.e.t.Fatalf("paying invoice %s redirected to %q", invoiceID, res.Location)
	}
	id := scalar[string](b.e.t, `SELECT id::text FROM payments WHERE invoice_id = $1 AND status = 'pending' ORDER BY created_at DESC LIMIT 1`, invoiceID)
	return "/app/billing/payments/" + id, res.Location
}

// within waits up to d for cond, failing the test with what it waited for.
func within(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(d); !cond(); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("waited %s for %s", d, what)
		}
	}
}

// ---------------------------------------------------------------------------
// Ethio telecom
// ---------------------------------------------------------------------------

// TestTextsThroughTheGateway: the app's texts go through Ethio telecom's
// gateway to the sandbox's phones and are reported delivered, and a
// driver's text from their phone moves the shipment and is answered.
func TestTextsThroughTheGateway(t *testing.T) {
	e, p := sandboxed(t)
	// Signing up takes the code texted to the sandbox's phone.
	office := e.company("Gateway Freight")
	within(t, 5*time.Second, "the code reported delivered", func() bool {
		return scalar[bool](t, `SELECT COUNT(*) > 0 AND bool_and(delivery = 'delivered') FROM sms_messages WHERE phone = $1 AND direction = 'outbound'`, office.phone)
	})
	if n := scalar[int64](t, `SELECT COUNT(*) FROM sms_messages WHERE phone = $1 AND direction = 'outbound'
		AND NOT (status = 'sent' AND provider = 'ethiotelecom' AND provider_message_id LIKE 'ET%')`, office.phone); n != 0 {
		t.Errorf("%d texts aren't recorded as sent through Ethio telecom with the gateway's message ID", n)
	}
	within(t, 5*time.Second, "the gateway learning the report was taken", func() bool {
		tx := p.phone(office.phone).from(senderID)
		return len(tx) == 1 && tx[0].State == "Delivered" && tx[0].Report == "Report: Taken by Menged"
	})

	// The driver is texted the shipment, and texts back from the road.
	driverID, driverPhone, _ := office.addDriver("Almaz Bekele")
	shipmentID, tracking := office.book("Addis Ababa", "Adama", driverID)
	ref := service.ShortRef(tracking)
	e.received("sms", driverPhone, "text ET "+ref+" 1 to 6060")
	if code := p.text(driverPhone, "ET "+ref+" 1"); code != http.StatusOK {
		t.Fatalf("the driver's text: the app answered %d", code)
	}
	if st := scalar[string](t, `SELECT status FROM shipments WHERE id = $1`, shipmentID); st != "in_transit" {
		t.Errorf("after the driver's text the shipment is %s", st)
	}
	e.received("sms", driverPhone, "recorded")
	if n := scalar[int64](t, `SELECT COUNT(*) FROM sms_messages WHERE phone = $1 AND direction = 'inbound'`, driverPhone); n != 1 {
		t.Errorf("%d texts from the driver recorded, want 1", n)
	}

	// A stranger's text is answered and changes nothing.
	if code := p.text(phoneNumber(), "ET "+ref+" 3"); code != http.StatusOK {
		t.Errorf("a stranger's text: the app answered %d", code)
	}
	if st := scalar[string](t, `SELECT status FROM shipments WHERE id = $1`, shipmentID); st != "in_transit" {
		t.Errorf("a stranger's text changed the shipment to %s", st)
	}
}

// TestGatewayFaults: a gateway that is down is tried again later, a number
// it refuses fails at once, one it can't reach is reported undelivered,
// and a token it ends early is replaced.
func TestGatewayFaults(t *testing.T) {
	e, p := sandboxed(t)
	office := e.company("Outage Freight")
	driver := func(name string) (id, phone string) {
		t.Helper()
		id, phone, _ = office.addDriver(name)
		return id, phone
	}
	assign := func(driverID string) { office.book("Addis Ababa", "Hawassa", driverID) }
	// state is the assignment text's status, delivery report and attempts.
	state := func(phone string) string {
		return scalar[string](t, `SELECT status || ' ' || COALESCE(delivery, '-') || ' ' || attempts
			FROM sms_messages WHERE phone = $1 AND purpose = 'driver_assigned'`, phone)
	}

	// Down: the text waits, and goes in the outbox's next round once the
	// gateway is back.
	p.faults(map[string]any{"sms_down": true})
	id, down := driver("Tesfaye Girma")
	assign(id)
	within(t, 5*time.Second, "the first try", func() bool { return state(down) == "queued - 1" })
	if !scalar[bool](t, `SELECT next_attempt_at > NOW() AND last_error LIKE '%503%' FROM sms_messages WHERE phone = $1 AND purpose = 'driver_assigned'`, down) {
		t.Error("the text isn't waiting to be tried again, with the gateway's answer")
	}
	p.faults(map[string]any{"sms_down": false})
	exec(t, `UPDATE sms_messages SET next_attempt_at = NOW() WHERE phone = $1`, down)
	within(t, 10*time.Second, "the second try", func() bool { return state(down) == "sent delivered 2" })
	e.received("sms", down, "new shipment")

	// Barred: refused for good, so it isn't tried again.
	id, barred := driver("Hirut Mekonnen")
	p.faults(map[string]any{"barred": []string{barred}})
	assign(id)
	within(t, 5*time.Second, "the refusal", func() bool { return state(barred) == "failed - 1" })
	if tx := p.phone(barred).from(senderID); len(tx) != 0 {
		t.Errorf("a barred number received %d texts", len(tx))
	}

	// Out of reach: taken, then reported undelivered.
	id, away := driver("Dawit Haile")
	p.faults(map[string]any{"barred": []string{}, "unreachable": []string{away}})
	assign(id)
	within(t, 5*time.Second, "the undelivered report", func() bool { return state(away) == "sent undelivered 1" })

	// The gateway ends its tokens early: the adapter signs in again, and
	// the text goes at the first try.
	p.faults(map[string]any{"unreachable": []string{}})
	if code := p.call(http.MethodPost, "/api/tokens/expire", nil, nil); code != http.StatusNoContent {
		t.Fatalf("ending the tokens: status %d", code)
	}
	logins := len(p.calls("/ethiotelecom/auth/token"))
	id, late := driver("Meron Tadesse")
	assign(id)
	within(t, 5*time.Second, "the text after signing in again", func() bool { return state(late) == "sent delivered 1" })
	if n := len(p.calls("/ethiotelecom/auth/token")); n != logins+1 {
		t.Errorf("%d sign-ins after the token ended, want 1", n-logins)
	}
	refused := 0
	for _, c := range p.calls("/ethiotelecom/sms/send") {
		if c.Status == http.StatusUnauthorized {
			refused++
		}
	}
	if refused != 1 {
		t.Errorf("%d sends refused for the ended token, want 1", refused)
	}
}

// ---------------------------------------------------------------------------
// telebirr
// ---------------------------------------------------------------------------

// TestPayingThroughTelebirr: the app orders the payment from telebirr and
// sends the payer to its checkout, where they pay with their phone and PIN.
// telebirr's signed notification settles the invoice, and the payer comes
// back to the payment page.
func TestPayingThroughTelebirr(t *testing.T) {
	e, p := sandboxed(t)
	office := e.company("Wallet Freight")
	invoiceID := office.choose("business", "monthly")
	number := scalar[string](t, `SELECT number FROM invoices WHERE id = $1`, invoiceID)
	total := scalar[int64](t, `SELECT total_santim FROM invoices WHERE id = $1`, invoiceID)
	office.confirmEmail()

	status, checkout := office.telebirrCheckout(p, invoiceID)
	if pg := office.get(status); !strings.Contains(pg.Body, "Waiting for telebirr to confirm") {
		t.Errorf("the payment page doesn't wait for telebirr: status %d", pg.Status)
	}
	if pg := p.visit(checkout, nil); pg.Status != http.StatusOK || !strings.Contains(pg.Body, "ETB "+payments.FormatETB(total)) ||
		!strings.Contains(pg.Body, strings.ReplaceAll(number, "-", " ")) {
		t.Errorf("telebirr's checkout: status %d, without the amount or the invoice", pg.Status)
	}

	// A wrong PIN takes nothing, and the payment waits.
	if pg := p.pay(checkout, office.phone, "0000"); pg.Status != http.StatusUnprocessableEntity || !strings.Contains(pg.Body, "Wrong PIN") {
		t.Errorf("a wrong PIN: status %d", pg.Status)
	}
	if st := scalar[string](t, `SELECT status FROM invoices WHERE id = $1`, invoiceID); st != "open" {
		t.Fatalf("after a wrong PIN the invoice is %s", st)
	}

	pg := p.pay(checkout, office.phone, walletPIN)
	if pg.Status != http.StatusSeeOther || pg.Location != e.srv.URL+status {
		t.Fatalf("paying: status %d to %q, want the payment page", pg.Status, pg.Location)
	}
	if st := scalar[string](t, `SELECT status FROM invoices WHERE id = $1`, invoiceID); st != "paid" {
		t.Fatalf("after paying at telebirr's checkout the invoice is %s", st)
	}
	if pg := office.get(status); !strings.Contains(pg.Body, "Paid. Thank you") {
		t.Error("the payment page doesn't say it's paid")
	}
	e.received("email", office.email, "Receipt for invoice "+number)
	if !scalar[bool](t, `SELECT provider = 'telebirr' AND provider_ref IS NOT NULL FROM payments WHERE invoice_id = $1 AND status = 'paid'`, invoiceID) {
		t.Error("the payment isn't recorded as paid with telebirr, with telebirr's reference")
	}
	ph := p.phone(office.phone)
	if ph.Telebirr != startingBalance-total || len(ph.from("127")) != 1 {
		t.Errorf("the payer's wallet holds %d and they have %d texts from telebirr, want %d and 1", ph.Telebirr, len(ph.from("127")), startingBalance-total)
	}
	if calls := p.calls("/api/v1/payments/telebirr/notify"); len(calls) != 1 || calls[0].Status != http.StatusOK {
		t.Errorf("telebirr's notifications: %+v", calls)
	}
}

// TestTelebirrNotificationFaults: a notification that never comes is made
// up for by asking telebirr, when the payer comes back or in the billing
// job; one sent twice pays once; and a forged one is refused.
func TestTelebirrNotificationFaults(t *testing.T) {
	e, p := sandboxed(t)
	const notify = "/api/v1/payments/telebirr/notify"
	// paid takes a new company from choosing a plan to paying at telebirr's
	// checkout, and returns its invoice and payment page.
	paid := func(name string) (office *browser, invoiceID, status string) {
		t.Helper()
		office = e.company(name)
		invoiceID = office.choose("starter", "monthly")
		office.confirmEmail()
		status, checkout := office.telebirrCheckout(p, invoiceID)
		if pg := p.pay(checkout, office.phone, walletPIN); pg.Status != http.StatusSeeOther {
			t.Fatalf("paying: status %d\n%s", pg.Status, pg.Body)
		}
		return office, invoiceID, status
	}
	invoice := func(id string) string { return scalar[string](t, `SELECT status FROM invoices WHERE id = $1`, id) }
	// once checks an invoice was paid once: one payment, one receipt.
	once := func(office *browser, invoiceID string) {
		t.Helper()
		if n := scalar[int64](t, `SELECT COUNT(*) FROM payments WHERE invoice_id = $1 AND status = 'paid'`, invoiceID); n != 1 {
			t.Errorf("%d paid payments, want 1", n)
		}
		if n := scalar[int64](t, `SELECT COUNT(*) FROM emails WHERE to_address = $1 AND purpose = 'receipt'`, office.email); n != 1 {
			t.Errorf("%d receipts, want 1", n)
		}
	}

	// Lost: the payer coming back to the payment page asks telebirr.
	p.faults(map[string]any{"drop_notifications": true})
	office, invoiceID, status := paid("Lost Notice Freight")
	if st := invoice(invoiceID); st != "open" {
		t.Fatalf("with the notification lost the invoice is %s", st)
	}
	if pg := office.get(status); !strings.Contains(pg.Body, "Paid. Thank you") {
		t.Error("coming back to the payment page didn't find the payment made")
	}
	once(office, invoiceID)
	// The notification arriving after all changes nothing.
	held := p.held()
	if len(held) != 1 {
		t.Fatalf("%d calls held, want the notification", len(held))
	}
	p.release(held[0])
	if calls := p.calls(notify); len(calls) != 1 || calls[0].Status != http.StatusOK {
		t.Errorf("the late notification: %+v", calls)
	}
	once(office, invoiceID)

	// Lost, and the payer never comes back: the billing job asks telebirr
	// about payments a minute old.
	office, invoiceID, _ = paid("Gone Payer Freight")
	exec(t, `UPDATE payments SET created_at = NOW() - INTERVAL '2 minutes' WHERE invoice_id = $1`, invoiceID)
	if err := e.svc.Billing.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st := invoice(invoiceID); st != "paid" {
		t.Fatalf("after the billing job the invoice is %s", st)
	}
	once(office, invoiceID)

	// Twice: one payment, one receipt.
	p.faults(map[string]any{"drop_notifications": false, "duplicate_notifications": true})
	before := len(p.calls(notify))
	office, invoiceID, _ = paid("Twice Notice Freight")
	if calls := p.calls(notify)[before:]; len(calls) != 2 || calls[0].Status != http.StatusOK || calls[1].Status != http.StatusOK {
		t.Errorf("the notifications sent twice: %+v", calls)
	}
	once(office, invoiceID)

	// Forged: refused, so the payment waits until telebirr is asked.
	p.faults(map[string]any{"duplicate_notifications": false, "wrong_key": true})
	before = len(p.calls(notify))
	office, invoiceID, status = paid("Forged Notice Freight")
	if calls := p.calls(notify)[before:]; len(calls) == 0 || calls[0].Status != http.StatusUnauthorized {
		t.Errorf("a notification signed with another key: %+v", calls)
	}
	if st := invoice(invoiceID); st != "open" {
		t.Fatalf("a forged notification made the invoice %s", st)
	}
	if pg := office.get(status); !strings.Contains(pg.Body, "Paid. Thank you") {
		t.Error("asking telebirr didn't find the payment made")
	}
	once(office, invoiceID)
}

// ---------------------------------------------------------------------------
// CBE Birr
// ---------------------------------------------------------------------------

// TestPayingByReferenceThroughCBEBirr: the payer pays the app's merchant ID
// from their phone, quoting the invoice's reference, and CBE Birr's servers
// log in to the app to look the reference up, check the payment and
// confirm it.
func TestPayingByReferenceThroughCBEBirr(t *testing.T) {
	e, p := sandboxed(t)
	office := e.company("Reference Wallet Freight")
	invoiceID := office.choose("starter", "monthly")
	number := scalar[string](t, `SELECT number FROM invoices WHERE id = $1`, invoiceID)
	total := scalar[int64](t, `SELECT total_santim FROM invoices WHERE id = $1`, invoiceID)
	ref := service.PayReference(number)
	office.confirmEmail()

	// The invoice says what to key in, and links to the sandbox's phone.
	pg := office.get("/app/billing/invoices/" + invoiceID)
	for _, want := range []string{cbeMerchant, ref, p.url + "/phones/" + office.phone + "?merchant=" + cbeMerchant} {
		if !strings.Contains(pg.Body, want) {
			t.Errorf("the invoice page doesn't show %q", want)
		}
	}

	// An unknown reference is turned away before money moves.
	if res := p.payCBE(office.phone, "202699999"); res.status != http.StatusUnprocessableEntity || res.Bill.ResultCode != payments.CBEBirrUnknownBill {
		t.Errorf("an unknown reference: %+v", res)
	}

	res := p.payCBE(office.phone, ref)
	if res.status != http.StatusOK || res.Payment.Amount != total {
		t.Fatalf("paying by reference: %+v", res)
	}
	if st := scalar[string](t, `SELECT status FROM invoices WHERE id = $1`, invoiceID); st != "paid" {
		t.Fatalf("after paying by reference the invoice is %s", st)
	}
	if !scalar[bool](t, `SELECT provider = 'cbebirr' AND status = 'paid' AND payer_phone = $2 FROM payments WHERE provider_ref = $1`, res.Payment.TransID, office.phone) {
		t.Error("the payment isn't recorded as paid with CBE Birr, from the payer's phone")
	}
	e.received("email", office.email, "Receipt for invoice "+number)
	ph := p.phone(office.phone)
	if ph.CBEBirr != startingBalance-total || len(ph.from("CBE Birr")) != 1 {
		t.Errorf("the payer's wallet holds %d and they have %d texts from CBE Birr, want %d and 1", ph.CBEBirr, len(ph.from("CBE Birr")), startingBalance-total)
	}

	// Paid already: CBE Birr says so, and takes nothing.
	if res := p.payCBE(office.phone, ref); res.status != http.StatusUnprocessableEntity || res.Bill.ResultCode != payments.CBEBirrBillPaid {
		t.Errorf("paying a paid invoice: %+v", res)
	}
	if ph := p.phone(office.phone); ph.CBEBirr != startingBalance-total {
		t.Errorf("paying a paid invoice took money: the wallet holds %d", ph.CBEBirr)
	}

	// A confirmation that doesn't arrive is sent later, and pays the
	// invoice once.
	late := e.company("Late Confirmation Freight")
	lateInvoice := late.choose("starter", "monthly")
	lateRef := service.PayReference(scalar[string](t, `SELECT number FROM invoices WHERE id = $1`, lateInvoice))
	p.faults(map[string]any{"drop_confirmations": true})
	if res := p.payCBE(late.phone, lateRef); res.status != http.StatusOK {
		t.Fatalf("paying with the confirmation held back: %+v", res)
	}
	if st := scalar[string](t, `SELECT status FROM invoices WHERE id = $1`, lateInvoice); st != "open" {
		t.Fatalf("before the confirmation arrives the invoice is %s", st)
	}
	held := p.held()
	if len(held) != 1 {
		t.Fatalf("%d calls held, want the confirmation", len(held))
	}
	p.release(held[0])
	if st := scalar[string](t, `SELECT status FROM invoices WHERE id = $1`, lateInvoice); st != "paid" {
		t.Errorf("after the late confirmation the invoice is %s", st)
	}

	// Confirmed twice: one payment.
	twice := e.company("Twice Confirmed Freight")
	twiceInvoice := twice.choose("starter", "monthly")
	twiceRef := service.PayReference(scalar[string](t, `SELECT number FROM invoices WHERE id = $1`, twiceInvoice))
	p.faults(map[string]any{"drop_confirmations": false, "duplicate_confirmations": true})
	before := len(p.calls("/api/v1/payments/cbebirr/confirmation"))
	if res := p.payCBE(twice.phone, twiceRef); res.status != http.StatusOK {
		t.Fatalf("paying with the confirmation sent twice: %+v", res)
	}
	if n := len(p.calls("/api/v1/payments/cbebirr/confirmation")) - before; n != 2 {
		t.Errorf("%d confirmations, want 2", n)
	}
	for _, id := range []string{lateInvoice, twiceInvoice} {
		if n := scalar[int64](t, `SELECT COUNT(*) FROM payments WHERE invoice_id = $1`, id); n != 1 {
			t.Errorf("%d payments for invoice %s, want 1", n, id)
		}
	}
}

// ---------------------------------------------------------------------------
// Settings that disagree
// ---------------------------------------------------------------------------

// TestProvidersWithOtherSettings: when the sandbox's credentials and the
// app's disagree, as when one is copied wrong at onboarding, the calls are
// refused on both sides and nothing is recorded.
func TestProvidersWithOtherSettings(t *testing.T) {
	e, p := sandboxed(t, func(c *sandbox.Config) {
		c.SMS.Secret = "another-webhook-secret"
		c.CBEBirr.Password = "another-password"
		c.Telebirr.AppSecret = "another-app-secret"
	})
	// The app's texts still go, but the gateway's reports are refused...
	office := e.company("Misconfigured Freight")
	within(t, 5*time.Second, "the refused delivery report", func() bool {
		tx := p.phone(office.phone).from(senderID)
		return len(tx) == 1 && strings.Contains(tx[0].Report, "answered 401")
	})
	if n := scalar[int64](t, `SELECT COUNT(*) FROM sms_messages WHERE phone = $1 AND delivery IS NOT NULL`, office.phone); n != 0 {
		t.Errorf("%d refused delivery reports recorded", n)
	}
	// ...and so are texts from phones.
	if code := p.text(office.phone, "STOP"); code != http.StatusUnauthorized {
		t.Errorf("a text with the wrong webhook secret: the app answered %d, want 401", code)
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM sms_opt_outs WHERE phone = $1`, office.phone); n != 0 {
		t.Error("a refused text opted the phone out")
	}

	// CBE Birr can't log in to look the reference up.
	invoiceID := office.choose("starter", "monthly")
	ref := service.PayReference(scalar[string](t, `SELECT number FROM invoices WHERE id = $1`, invoiceID))
	if res := p.payCBE(office.phone, ref); res.status != http.StatusUnprocessableEntity || !strings.Contains(res.Error, "answered 401") {
		t.Errorf("CBE Birr with the wrong login: %+v", res)
	}
	// telebirr turns the app's sign-in away, so telebirr can't be reached.
	office.confirmEmail()
	if msg, failed := office.result("/app/billing/invoices/"+invoiceID+"/pay", url.Values{"provider": {"telebirr"}}); !failed || !strings.Contains(msg, "telebirr could not be reached") {
		t.Errorf("paying with telebirr's app secret wrong: %q", msg)
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM payments WHERE invoice_id = $1 AND status <> 'failed'`, invoiceID); n != 0 {
		t.Errorf("%d payments left waiting", n)
	}
	if st := scalar[string](t, `SELECT status FROM invoices WHERE id = $1`, invoiceID); st != "open" {
		t.Errorf("the invoice is %s", st)
	}

	// The gateway turns the app's key away: the sign-up code waits to be
	// tried again.
	e2, _ := sandboxed(t, func(c *sandbox.Config) { c.SMS.APIKey = "another-api-key" })
	b := e2.browser()
	phone := phoneNumber()
	b.mustPost("/signup", url.Values{
		"company": {"Wrong Key Freight"}, "city": {"Addis Ababa"}, "name": {"Owner"},
		"email": {fmt.Sprintf("wrongkey%d@example.et", time.Now().UnixNano())}, "phone": {phone}, "password": {"correct-horse-1"},
	})
	within(t, 5*time.Second, "the refused code", func() bool {
		return scalar[string](t, `SELECT status || ' ' || attempts FROM sms_messages WHERE phone = $1`, phone) == "queued 1"
	})
	if !scalar[bool](t, `SELECT last_error LIKE '%login%401%' FROM sms_messages WHERE phone = $1`, phone) {
		t.Errorf("the error is %q", scalar[string](t, `SELECT COALESCE(last_error, '') FROM sms_messages WHERE phone = $1`, phone))
	}
}
