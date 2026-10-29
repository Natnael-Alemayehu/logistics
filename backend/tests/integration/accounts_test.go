//go:build integration

package integration

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// TestDriverPINReset: a driver who forgot their PIN gets a code by SMS and
// signs in with a new PIN; the code works once.
func TestDriverPINReset(t *testing.T) {
	e := newEnv(t)
	office := e.company("Reset Haulage")
	_, phone, pin := office.addDriver("Forgetful Driver")
	e.signIn(phone, pin)

	if code := e.api("POST", "/api/v1/driver/pin-reset/request", "", map[string]string{"phone": phone}, nil); code != http.StatusOK {
		t.Fatalf("request: status %d", code)
	}
	code := e.code(phone)

	confirm := map[string]string{"phone": phone, "code": code, "pin": "7391", "device_id": "new-phone"}
	var login struct {
		Token string `json:"token"`
	}
	if status := e.api("POST", "/api/v1/driver/pin-reset/confirm", "", confirm, &login); status != http.StatusOK || login.Token == "" {
		t.Fatalf("confirm: status %d", status)
	}
	if status := e.api("POST", "/api/v1/driver/login", "", map[string]string{"phone": phone, "pin": "7391", "device_id": "new-phone"}, nil); status != http.StatusOK {
		t.Errorf("sign in with the new PIN: status %d", status)
	}
	if status := e.api("POST", "/api/v1/driver/pin-reset/confirm", "", confirm, nil); status == http.StatusOK {
		t.Error("a used code worked a second time")
	}

	// The SMS log keeps the message but not the code.
	if body := scalar[string](t, `SELECT body FROM sms_messages WHERE phone = $1 AND sensitive ORDER BY created_at DESC LIMIT 1`, phone); strings.Contains(body, code) {
		t.Error("the code is still readable in the SMS log after sending")
	}
}

var (
	verifyLinkRe = regexp.MustCompile(`/verify-email/[A-Za-z0-9_-]+`)
	resetLinkRe  = regexp.MustCompile(`/reset/[A-Za-z0-9_-]+`)
)

// TestSignupConfirmsPhoneAndEmail: a company exists only once the code texted
// to its admin's phone comes back to the browser that signed up, and the
// admin is asked to confirm their email address until they open its link.
func TestSignupConfirmsPhoneAndEmail(t *testing.T) {
	e := newEnv(t)
	b := e.browser()
	ph, email := phoneNumber(), fmt.Sprintf("owner%d@example.et", rand.IntN(1e9))
	form := url.Values{
		"company": {"Gondar Express"}, "city": {"Gondar"}, "name": {"Tigist Alemu"},
		"email": {email}, "phone": {ph}, "password": {"correct-horse-1"},
	}
	if p := b.mustPost("/signup", form); p.Location != "/signup/verify" {
		t.Fatalf("signup redirected to %q", p.Location)
	}
	code := e.code(ph)
	if n := scalar[int64](t, `SELECT COUNT(*) FROM users WHERE email = $1`, email); n != 0 {
		t.Fatal("the account exists before the phone was confirmed")
	}
	if p := b.get("/signup/verify"); !strings.Contains(p.Body, ph) {
		t.Error("the code page doesn't say where the code went")
	}
	if p := b.get("/signup"); !strings.Contains(p.Body, `value="Gondar Express"`) {
		t.Error("going back to correct the form loses what was typed")
	}

	// The code alone, without the browser's sign-up, creates nothing.
	if p := e.browser().post("/signup/verify", url.Values{"code": {code}}); p.Location != "/signup" {
		t.Errorf("a code without its sign-up: status %d, location %q", p.Status, p.Location)
	}
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	if p := b.post("/signup/verify", url.Values{"code": {wrong}}); p.Status != http.StatusBadRequest {
		t.Errorf("a wrong code: status %d", p.Status)
	}
	if p := b.mustPost("/signup/verify", url.Values{"code": {code}}); p.Location != "/app" {
		t.Fatalf("the right code redirected to %q", p.Location)
	}
	if city := scalar[string](t, `SELECT t.region_city FROM tenants t JOIN users u ON u.tenant_id = t.id WHERE u.email = $1`, email); city != "Gondar" {
		t.Errorf("the company's town is %q", city)
	}

	const reminder = "Please confirm your email address"
	if !strings.Contains(b.get("/app").Body, reminder) {
		t.Error("no reminder to confirm the email address")
	}
	link := verifyLinkRe.FindString(e.received("email", email, "/verify-email/").Body)
	if p := b.get(link); p.Location != "/app/account" {
		t.Fatalf("the confirmation link: status %d, location %q", p.Status, p.Location)
	}
	if strings.Contains(b.get("/app").Body, reminder) {
		t.Error("still asked to confirm after opening the link")
	}
	// Opening it again, signed out, says the same thing.
	if p := e.browser().get(link); p.Location != "/login" {
		t.Errorf("the link opened again: status %d, location %q", p.Status, p.Location)
	}

	// The address and the phone are taken now.
	form.Set("phone", phoneNumber())
	if p := e.browser().post("/signup", form); p.Status != http.StatusBadRequest || !strings.Contains(p.Body, "already registered") {
		t.Errorf("signing up again with the same email: status %d", p.Status)
	}
}

// TestPasswordReset: staff who forgot their password get a link by email.
// It works once, ends every other session, and the form says the same thing
// whether or not the address has an account.
func TestPasswordReset(t *testing.T) {
	e := newEnv(t)
	office := e.company("Forgetful Freight")
	unknown := fmt.Sprintf("nobody%d@example.et", rand.IntN(1e9))
	for _, addr := range []string{office.email, unknown} {
		if p := e.browser().post("/forgot", url.Values{"email": {addr}}); p.Status != http.StatusOK || !strings.Contains(p.Body, "Check your email") {
			t.Fatalf("forgot %s: status %d\n%s", addr, p.Status, errorText(p.Body))
		}
	}
	link := resetLinkRe.FindString(e.received("email", office.email, "/reset/").Body)
	for _, it := range e.inbox.Items() {
		if it.To == unknown {
			t.Error("an email went to an address with no account")
		}
	}

	// Opening the link doesn't use it: mail scanners open links too.
	if p := e.browser().get(link); p.Status != http.StatusOK || !strings.Contains(p.Body, "Owner of Forgetful Freight") {
		t.Fatalf("the reset page: status %d", p.Status)
	}
	fresh := e.browser()
	if p := fresh.post(link, url.Values{"new_password": {"new-horse-22"}, "confirm_password": {"new-horse-23"}}); p.Status != http.StatusBadRequest {
		t.Errorf("mismatched passwords: status %d", p.Status)
	}
	if p := fresh.post(link, url.Values{"new_password": {"short"}, "confirm_password": {"short"}}); p.Status != http.StatusBadRequest {
		t.Errorf("a short password: status %d", p.Status)
	}
	if p := fresh.mustPost(link, url.Values{"new_password": {"new-horse-22"}, "confirm_password": {"new-horse-22"}}); p.Location != "/app" {
		t.Fatalf("the reset redirected to %q", p.Location)
	}
	if p := fresh.get("/app"); p.Status != http.StatusOK {
		t.Errorf("not signed in after the reset: status %d", p.Status)
	}
	if p := office.get("/app"); !strings.HasPrefix(p.Location, "/login") {
		t.Errorf("the session from before the reset still works: status %d", p.Status)
	}
	if p := e.browser().get(link); p.Status != http.StatusNotFound {
		t.Errorf("the link worked a second time: status %d", p.Status)
	}
	if p := e.browser().post("/login", url.Values{"identifier": {office.email}, "password": {"correct-horse-1"}}); p.Status == http.StatusSeeOther {
		t.Error("the old password still works")
	}
	if p := e.browser().post("/login", url.Values{"identifier": {office.email}, "password": {"new-horse-22"}}); p.Status != http.StatusSeeOther {
		t.Errorf("signing in with the new password: status %d", p.Status)
	}
}

// TestChangedEmailNeedsConfirming: a new address has to be confirmed again,
// and links sent to the old one stop working.
func TestChangedEmailNeedsConfirming(t *testing.T) {
	e := newEnv(t)
	office := e.company("Moving Address")
	office.get(verifyLinkRe.FindString(e.received("email", office.email, "/verify-email/").Body))
	e.browser().post("/forgot", url.Values{"email": {office.email}})
	oldReset := resetLinkRe.FindString(e.received("email", office.email, "/reset/").Body)

	next := fmt.Sprintf("new%d@example.et", rand.IntN(1e9))
	office.mustPost("/app/account", url.Values{"full_name": {"Owner of Moving Address"}, "email": {next}, "phone": {office.phone}})
	if !strings.Contains(office.get("/app").Body, "Please confirm your email address") {
		t.Error("no reminder to confirm the new address")
	}
	if p := e.browser().get(oldReset); p.Status != http.StatusNotFound {
		t.Errorf("a reset link sent to the old address still works: status %d", p.Status)
	}
	link := verifyLinkRe.FindString(e.received("email", next, "/verify-email/").Body)

	// Asking again sends another link; both work until the address changes.
	office.mustPost("/app/account/verify-email", nil)
	e.received("email", next, "/verify-email/")
	if p := office.get(link); p.Location != "/app/account" {
		t.Errorf("the confirmation link: status %d, location %q", p.Status, p.Location)
	}
	if verified := scalar[bool](t, `SELECT email_verified_at IS NOT NULL FROM users WHERE email = $1`, next); !verified {
		t.Error("the new address isn't confirmed")
	}
}

// TestRetentionClearsAccountLinks: expired sign-ups and old email links are
// deleted by the nightly retention job.
func TestRetentionClearsAccountLinks(t *testing.T) {
	e := newEnv(t)
	office := e.company("Tidy Transport")
	exec(t, `UPDATE email_tokens SET created_at = NOW() - INTERVAL '31 days' WHERE email = $1`, office.email)
	ph := phoneNumber()
	exec(t, `INSERT INTO pending_signups (token_hash, company, city, full_name, email, phone, password_hash, expires_at)
		VALUES ($1, 'Stale Co', 'Adama', 'Someone', $2, $3, 'x', NOW() - INTERVAL '1 minute')`, "stale-"+ph, ph+"@example.et", ph)

	if err := e.svc.Jobs.Retention(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM email_tokens WHERE email = $1`, office.email); n != 0 {
		t.Errorf("%d old email links kept", n)
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM pending_signups WHERE phone = $1`, ph); n != 0 {
		t.Error("an expired sign-up was kept")
	}
}
