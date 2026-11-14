//go:build integration

package integration

// Regression tests for the findings of the November 2026 audit
// (fable-audit.md at the repository root). Each test names the finding it
// covers.

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
)

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

// TestCodeLimitsLookTheSame (S5): asking for track-by-phone codes reaches
// the same limit whether or not the number has shipments.
func TestCodeLimitsLookTheSame(t *testing.T) {
	e := newEnv(t)
	office := e.company("Quiet Numbers")
	customer := phoneNumber()
	office.bookFor("Addis Ababa", "Adama", "", customer)
	for _, number := range []string{customer, phoneNumber()} {
		b := e.browser()
		for i := 1; i <= 3; i++ {
			if p := b.post("/track/phone", url.Values{"phone": {number}}); p.Status != http.StatusOK {
				t.Fatalf("request %d for %s: status %d", i, number, p.Status)
			}
		}
		if p := b.post("/track/phone", url.Values{"phone": {number}}); p.Status != http.StatusPaymentRequired {
			t.Errorf("fourth request for %s: status %d, want the limit", number, p.Status)
		}
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM sms_messages WHERE purpose = 'otp' AND phone = $1`, customer); n != 3 {
		t.Errorf("%d codes texted to the customer, want 3", n)
	}
}

// TestCodeTriesCannotRace (S5): wrong guesses sent together still get only
// five tries between them.
func TestCodeTriesCannotRace(t *testing.T) {
	e := newEnv(t)
	office := e.company("Racing Guesses")
	_, phone, pin := office.addDriver("Hirut Mekonnen")
	e.signIn(phone, pin)
	if code := e.api("POST", "/api/v1/driver/pin-reset/request", "", map[string]string{"phone": phone}, nil); code != http.StatusOK {
		t.Fatalf("request a code: status %d", code)
	}
	right := e.code(phone)
	wrong := "000000"
	if right == wrong {
		wrong = "111111"
	}
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.api("POST", "/api/v1/driver/pin-reset/confirm", "", map[string]string{"phone": phone, "code": wrong, "pin": "5937"}, nil)
		}()
	}
	wg.Wait()
	if n := scalar[int32](t, `SELECT attempts FROM verification_codes WHERE phone = $1 AND purpose = 'pin_reset'`, phone); n != 5 {
		t.Errorf("%d tries counted, want 5", n)
	}
	if code := e.api("POST", "/api/v1/driver/pin-reset/confirm", "", map[string]string{"phone": phone, "code": right, "pin": "5937"}, nil); code == http.StatusOK {
		t.Error("the right code worked after five wrong tries")
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
