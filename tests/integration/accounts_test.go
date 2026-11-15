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
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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

	// The address is taken now. That is said only once the code comes
	// back, to whoever holds the phone, so the form can't be used to find
	// out who has an account.
	again := e.browser()
	form.Set("phone", phoneNumber())
	if p := again.mustPost("/signup", form); p.Location != "/signup/verify" {
		t.Fatalf("signing up again with the same email redirected to %q", p.Location)
	}
	if p := again.post("/signup/verify", url.Values{"code": {e.code(form.Get("phone"))}}); p.Status != http.StatusBadRequest || !strings.Contains(p.Body, "already registered") {
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

	// An address not yet confirmed gets no link: it may be mistyped, and
	// belong to someone else (audit R5).
	if p := e.browser().post("/forgot", url.Values{"email": {office.email}}); p.Status != http.StatusOK || !strings.Contains(p.Body, "Check your email") {
		t.Fatalf("forgot, unconfirmed: status %d", p.Status)
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM emails WHERE to_address = $1 AND purpose = 'password_reset'`, office.email); n != 0 {
		t.Fatalf("%d reset links queued for an unconfirmed address", n)
	}
	office.confirmEmail()

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

// TestRateLimits: one address may try 30 sign-ins a minute, more than the
// five wrong passwords that lock an account, and signing in doesn't use up
// its allowance for what texts a code, which is 10.
func TestRateLimits(t *testing.T) {
	e := newEnv(t)
	b := e.browser()
	// The limiter's window is the wall-clock minute, and its estimate drops
	// once a new one starts; the 30 refused passwords take seconds, so
	// start them early in a minute.
	if left := time.Until(time.Now().Truncate(time.Minute).Add(time.Minute)); left < 20*time.Second {
		time.Sleep(left)
	}
	for i := range 30 {
		if p := b.post("/login", url.Values{"identifier": {fmt.Sprintf("nobody%d@example.et", i)}, "password": {"wrong-password"}}); p.Status != http.StatusUnauthorized {
			t.Fatalf("sign-in %d: status %d, want it refused for the password", i+1, p.Status)
		}
	}
	p := b.post("/login", url.Values{"identifier": {"nobody@example.et"}, "password": {"wrong-password"}})
	if p.Status != http.StatusTooManyRequests || !strings.Contains(p.Body, "Wait a minute") {
		t.Fatalf("the 31st sign-in in a minute: status %d, want 429", p.Status)
	}
	var api struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if code := e.api("POST", "/api/v1/driver/login", "", map[string]string{"phone": phoneNumber(), "pin": "1234", "device_id": "d"}, &api); code != http.StatusTooManyRequests || api.Error.Code != "RATE_LIMITED" {
		t.Errorf("a driver signing in from the same address: status %d %q, want 429", code, api.Error.Code)
	}

	for i := range 10 {
		if p := b.post("/forgot", url.Values{"email": {fmt.Sprintf("nobody%d@example.et", i)}}); p.Status != http.StatusOK {
			t.Fatalf("reset request %d: status %d, want it taken", i+1, p.Status)
		}
	}
	if p := b.post("/forgot", url.Values{"email": {"nobody@example.et"}}); p.Status != http.StatusTooManyRequests {
		t.Errorf("the 11th reset request in a minute: status %d, want 429", p.Status)
	}
}

// TestLockoutStartsAgainAfterExpiry (R1): once a lockout has expired, one
// mistyped PIN doesn't lock the account again, and the right PIN works.
func TestLockoutStartsAgainAfterExpiry(t *testing.T) {
	e := newEnv(t)
	office := e.company("Lockout Logistics")
	_, phone, pin := office.addDriver("Abebe Kebede")
	e.signIn(phone, pin) // the driver's PIN is now 4826
	login := func(pin string) int {
		return e.api("POST", "/api/v1/driver/login", "", map[string]string{"phone": phone, "pin": pin, "device_id": "d1"}, nil)
	}
	for i := 1; i <= 4; i++ {
		if code := login("0000"); code != http.StatusUnauthorized {
			t.Fatalf("wrong PIN %d: status %d", i, code)
		}
	}
	if code := login("0000"); code != http.StatusForbidden {
		t.Fatalf("fifth wrong PIN: status %d, want the lockout", code)
	}
	exec(t, `UPDATE users SET locked_until = NOW() - INTERVAL '1 second' WHERE phone = $1`, phone)
	if code := login("0000"); code != http.StatusUnauthorized {
		t.Errorf("a mistype after the lock expired: status %d, want 401", code)
	}
	if code := login("4826"); code != http.StatusOK {
		t.Errorf("the right PIN after the lock expired: status %d", code)
	}
}

// TestUnconfirmedStaffEmailGetsNoReset (R5): an address an admin typed in
// for a colleague, never confirmed, gets no reset link.
func TestUnconfirmedStaffEmailGetsNoReset(t *testing.T) {
	e := newEnv(t)
	office := e.company("Typo Transport")
	email := "dawit" + phoneNumber() + "@example.et"
	if p := office.post("/app/team", url.Values{"full_name": {"Dawit Bekele"}, "email": {email}, "phone": {phoneNumber()}, "role": {"admin"}}); p.Status != http.StatusOK {
		t.Fatalf("add staff: status %d %s", p.Status, errorText(p.Body))
	}
	if p := e.browser().post("/forgot", url.Values{"identifier": {email}}); p.Status != http.StatusOK {
		t.Fatalf("forgot: status %d", p.Status)
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM emails WHERE to_address = $1 AND purpose = 'password_reset'`, email); n != 0 {
		t.Errorf("%d reset links queued for an unconfirmed address", n)
	}
}

// TestWrongAppNeedsTheRightCredential (S5): being told to use the other
// app takes that app's credential, so a number alone reveals nothing.
func TestWrongAppNeedsTheRightCredential(t *testing.T) {
	e := newEnv(t)
	office := e.company("Two Doors")
	_, driverPhone, pin := office.addDriver("Kebede Alemu")
	e.signIn(driverPhone, pin)

	staffLogin := func(id, pw string) page {
		return e.browser().post("/login", url.Values{"identifier": {id}, "password": {pw}})
	}
	if p := staffLogin(driverPhone, "not-the-pin"); p.Status != http.StatusUnauthorized || strings.Contains(errorText(p.Body), "driver app") {
		t.Errorf("a driver's number with a wrong password: status %d, %q", p.Status, errorText(p.Body))
	}
	if p := staffLogin(driverPhone, "4826"); p.Status != http.StatusForbidden || !strings.Contains(errorText(p.Body), "driver app") {
		t.Errorf("a driver's number with their PIN: status %d, %q", p.Status, errorText(p.Body))
	}

	var res struct {
		Error struct{ Message string } `json:"error"`
	}
	driverLogin := func(pin string) int {
		res.Error.Message = ""
		return e.api("POST", "/api/v1/driver/login", "", map[string]string{"phone": office.phone, "pin": pin, "device_id": "d"}, &res)
	}
	if code := driverLogin("1357"); code != http.StatusUnauthorized || strings.Contains(res.Error.Message, "office") {
		t.Errorf("an office number with a wrong PIN: status %d, %q", code, res.Error.Message)
	}
	if code := driverLogin("correct-horse-1"); code != http.StatusUnauthorized || !strings.Contains(res.Error.Message, "office account") {
		t.Errorf("an office number with its password: status %d, %q", code, res.Error.Message)
	}
}

// TestFailedSignupKeepsItsCode (S5, L10): a sign-up whose email was taken
// meanwhile is refused after the code, and the code isn't used up.
func TestFailedSignupKeepsItsCode(t *testing.T) {
	e := newEnv(t)
	office := e.company("Early Bird")
	email := "late" + phoneNumber() + "@example.et"
	phone := phoneNumber()
	b := e.browser()
	if p := b.mustPost("/signup", url.Values{
		"company": {"Late Bird"}, "city": {"Addis Ababa"}, "name": {"Late Owner"},
		"email": {email}, "phone": {phone}, "password": {"correct-horse-1"},
	}); p.Location != "/signup/verify" {
		t.Fatalf("signup redirected to %q", p.Location)
	}
	code := e.code(phone)
	if p := office.post("/app/team", url.Values{"full_name": {"Taken Address"}, "email": {email}, "role": {"dispatcher"}}); p.Status != http.StatusOK {
		t.Fatalf("add staff: status %d %s", p.Status, errorText(p.Body))
	}
	if p := b.post("/signup/verify", url.Values{"code": {code}}); p.Status != http.StatusBadRequest || !strings.Contains(p.Body, "already registered") {
		t.Fatalf("verify: status %d %q", p.Status, errorText(p.Body))
	}
	if !scalar[bool](t, `SELECT consumed_at IS NULL FROM verification_codes WHERE phone = $1 AND purpose = 'signup'`, phone) {
		t.Error("the code was used up by a sign-up that failed")
	}
}

// TestStaffResetByPhone (W6): staff without a confirmed email reset their
// password with a code texted to their phone.
func TestStaffResetByPhone(t *testing.T) {
	e := newEnv(t)
	office := e.company("Phone First")
	if p := e.browser().post("/forgot", url.Values{"identifier": {office.phone}}); p.Status != http.StatusOK || !strings.Contains(p.Body, "texted") {
		t.Fatalf("forgot by phone: status %d %q", p.Status, errorText(p.Body))
	}
	code := e.code(office.phone)

	// A number with no office account gets the same page, and no text.
	stranger := phoneNumber()
	if p := e.browser().post("/forgot", url.Values{"identifier": {stranger}}); p.Status != http.StatusOK || !strings.Contains(p.Body, "texted") {
		t.Fatalf("forgot by an unknown phone: status %d", p.Status)
	}

	b := e.browser()
	form := url.Values{"phone": {office.phone}, "code": {code}, "new_password": {"phone-horse-9"}, "confirm_password": {"phone-horse-9"}}
	if p := b.mustPost("/reset/phone", form); p.Location != "/app" {
		t.Fatalf("reset by phone redirected to %q", p.Location)
	}
	if p := office.get("/app"); !strings.HasPrefix(p.Location, "/login") {
		t.Errorf("the session from before the reset still works: status %d", p.Status)
	}
	if p := e.browser().post("/login", url.Values{"identifier": {office.phone}, "password": {"phone-horse-9"}}); p.Status != http.StatusSeeOther {
		t.Errorf("signing in with the new password: status %d", p.Status)
	}
	if p := e.browser().post("/reset/phone", form); p.Status != http.StatusBadRequest {
		t.Errorf("the code worked twice: status %d", p.Status)
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM sms_messages WHERE phone = $1`, stranger); n != 0 {
		t.Errorf("%d texts to a number with no account", n)
	}
}

// TestPhoneTakenElsewhereIsExplained (W7): a number used at another
// company gets a message that says so, without naming it.
func TestPhoneTakenElsewhereIsExplained(t *testing.T) {
	e := newEnv(t)
	first := e.company("First Haulage")
	second := e.company("Second Haulage")
	_, phone, _ := first.addDriver("Shared Driver")
	p := second.post("/app/drivers", url.Values{"full_name": {"Shared Driver"}, "phone": {phone}})
	if p.Status != http.StatusBadRequest || !strings.Contains(p.Body, "another company") || strings.Contains(p.Body, "First Haulage") {
		t.Errorf("a number taken at another company: status %d %q", p.Status, errorText(p.Body))
	}
}

// TestLiveFeedSessionCheck (S3): what the live feed asks every half
// minute. A session stops letting its holder in on sign-out, deactivation
// or the company's suspension, and the feed is then closed.
func TestLiveFeedSessionCheck(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	office := e.company("Live Feed Logistics")
	session := scalar[pgtype.UUID](t, `SELECT s.id FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE u.email = $1 AND s.revoked_at IS NULL ORDER BY s.created_at DESC LIMIT 1`, office.email)
	active := func(want bool, when string) {
		t.Helper()
		ok, err := e.svc.Auth.SessionActive(ctx, session)
		if err != nil {
			t.Fatal(err)
		}
		if ok != want {
			t.Errorf("%s: session active %v, want %v", when, ok, want)
		}
	}
	active(true, "signed in")
	exec(t, `UPDATE tenants SET is_active = false WHERE id = $1`, office.tenant())
	active(false, "company suspended")
	exec(t, `UPDATE tenants SET is_active = true WHERE id = $1`, office.tenant())
	active(true, "company reinstated")
	exec(t, `UPDATE users SET is_active = false WHERE email = $1`, office.email)
	active(false, "deactivated")
	exec(t, `UPDATE users SET is_active = true WHERE email = $1`, office.email)
	active(true, "reactivated")
	office.post("/logout", nil)
	active(false, "signed out")
}
