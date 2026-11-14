//go:build integration

package integration

// Regression tests for the findings of the November 2026 audit
// (fable-audit.md at the repository root). Each test names the finding it
// covers.

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/internal/service"
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

// TestTrialTextsStopAtTheAllowance (S1): a trial company's texts stop at
// the trial's allowance and at the hourly ceiling, and the company is told;
// a paying company's texts go on past its monthly allowance.
func TestTrialTextsStopAtTheAllowance(t *testing.T) {
	e := newEnv(t)
	sent := func(tenant string, parts int, at string) {
		exec(t, `INSERT INTO sms_messages (tenant_id, direction, phone, body, purpose, status, segments, created_at, sent_at)
			VALUES ($1, 'outbound', '0911000000', 'earlier texts', 'customer_update', 'sent', $2, `+at+`, `+at+`)`, tenant, parts)
	}
	textTo := func(customer string) (status, why string) {
		err := pool.QueryRow(t.Context(), `SELECT status, COALESCE(last_error, '') FROM sms_messages WHERE phone = $1 AND purpose = 'customer_update'`, customer).Scan(&status, &why)
		if err != nil {
			t.Fatal(err)
		}
		return status, why
	}

	trial := e.company("Spam Free Trial")
	exec(t, `UPDATE tenants SET created_at = NOW() - INTERVAL '3 days' WHERE id = $1`, trial.tenant())
	sent(trial.tenant(), 500, "NOW() - INTERVAL '2 days'")
	customer := phoneNumber()
	trial.bookFor("Addis Ababa", "Adama", "", customer)
	if status, why := textTo(customer); status != "skipped" || !strings.Contains(why, "trial") {
		t.Errorf("a trial past its allowance: text %s (%s)", status, why)
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM alerts WHERE tenant_id = $1 AND alert_type = 'sms_paused'`, trial.tenant()); n != 1 {
		t.Errorf("%d alerts about paused texts, want 1", n)
	}

	hourly := e.company("Busy Hour Trial")
	sent(hourly.tenant(), 100, "NOW()")
	customer = phoneNumber()
	hourly.bookFor("Addis Ababa", "Adama", "", customer)
	if status, why := textTo(customer); status != "skipped" || !strings.Contains(why, "an hour") {
		t.Errorf("a trial past its hourly ceiling: text %s (%s)", status, why)
	}

	paying := e.subscribed("Paying Customer", "starter", e.operator())
	sent(paying.tenant(), 600, "NOW() - INTERVAL '2 hours'")
	customer = phoneNumber()
	paying.bookFor("Addis Ababa", "Adama", "", customer)
	if status, why := textTo(customer); status == "skipped" {
		t.Errorf("a paying company past its monthly allowance had a text skipped: %s", why)
	}
}

// TestCodeBudget (S1): once the platform has texted its daily budget of
// codes, more are refused, for every number alike.
func TestCodeBudget(t *testing.T) {
	sent := scalar[int64](t, `SELECT COUNT(*) FROM sms_messages WHERE purpose = 'otp' AND created_at >= NOW() - INTERVAL '1 day'`)
	e := newEnv(t, func(s *setup) { s.cfg.CodesPerDay = int(sent) + 1 })
	office := e.company("Last Code") // uses the last code of the budget
	customer := phoneNumber()
	office.bookFor("Addis Ababa", "Adama", "", customer)
	for _, number := range []string{customer, phoneNumber()} {
		if p := e.browser().post("/track/phone", url.Values{"phone": {number}}); p.Status != http.StatusPaymentRequired || !strings.Contains(p.Body, "send codes right now") {
			t.Errorf("a code for %s past the budget: status %d %q", number, p.Status, errorText(p.Body))
		}
	}
}

// TestInboundSMSOnceAndRepliesLimited (S2): the provider sending a message
// again doesn't make it count twice, and one number gets only so many
// replies an hour.
func TestInboundSMSOnceAndRepliesLimited(t *testing.T) {
	e := newEnv(t)
	office := e.company("Echo Freight")
	_, phone, _ := office.addDriver("Auto Reply")
	inbound := func(id, body string) service.InboundResult {
		var out service.InboundResult
		if code := e.api("POST", "/api/v1/sms/inbound", "", map[string]string{"message_id": id, "from": phone, "message": body}, &out); code != http.StatusOK {
			t.Fatalf("inbound: status %d", code)
		}
		return out
	}
	if out := inbound("MSG-1", "HELP"); out.Outcome != "replied" {
		t.Fatalf("first delivery: %+v", out)
	}
	if out := inbound("MSG-1", "HELP"); out.Outcome != "duplicate" {
		t.Errorf("the same message again: %+v", out)
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM sms_messages WHERE phone = $1 AND direction = 'inbound'`, phone); n != 1 {
		t.Errorf("%d inbound messages recorded, want 1", n)
	}
	for i := 2; i <= 14; i++ {
		inbound(fmt.Sprintf("MSG-%d", i), "what?")
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM sms_messages WHERE phone = $1 AND purpose = 'driver_reply'`, phone); n != 10 {
		t.Errorf("%d replies in an hour, want 10", n)
	}
}

// TestExpiredCodesAreNotSent (L5): a code still waiting when it expires
// isn't sent late.
func TestExpiredCodesAreNotSent(t *testing.T) {
	newEnv(t) // its SMS worker picks the code up
	phone := phoneNumber()
	exec(t, `INSERT INTO sms_messages (direction, phone, body, purpose, sensitive, status, created_at)
		VALUES ('outbound', $1, 'Menged: your code is 123456', 'otp', true, 'queued', NOW() - INTERVAL '11 minutes')`, phone)
	within(t, 10*time.Second, "the expired code to fail", func() bool {
		return scalar[string](t, `SELECT status FROM sms_messages WHERE phone = $1`, phone) == "failed"
	})
	if why := scalar[string](t, `SELECT last_error FROM sms_messages WHERE phone = $1`, phone); !strings.Contains(why, "expired") {
		t.Errorf("failed because %q", why)
	}
}

// TestSlowPhoneClock (R2): a phone whose clock is a minute and a half slow
// can start a trip just assigned to it. Its times are corrected by the
// difference between its clock and the server's, however small.
func TestSlowPhoneClock(t *testing.T) {
	e := newEnv(t)
	office := e.company("Slow Clock Freight")
	driverID, phone, pin := office.addDriver("Hana Girma")
	shipmentID, _ := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)

	slow := time.Now().Add(-90 * time.Second)
	resp, code := d.sync(service.SyncRequest{DeviceTime: &slow, Statuses: []service.SyncStatus{{
		ClientID: clientID(), ShipmentID: shipmentID, Status: "in_transit", RecordedAt: slow,
	}}})
	if code != http.StatusOK || len(resp.Results) != 1 || !resp.Results[0].OK {
		t.Fatalf("start trip from a slow phone: status %d, results %+v", code, resp.Results)
	}
	if got := scalar[string](t, `SELECT status FROM shipments WHERE id = $1`, shipmentID); got != "in_transit" {
		t.Errorf("status %q, want in_transit", got)
	}
}

// TestResentSyncIsNotAConflict (R3): a phone that never got the answer to a
// sync sends it again. Each change gets the answer it got the first time,
// and nothing is recorded twice.
func TestResentSyncIsNotAConflict(t *testing.T) {
	e := newEnv(t)
	office := e.company("Resend Transport")
	driverID, phone, pin := office.addDriver("Dawit Alemu")
	shipmentID, _ := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)

	// Recorded after the booking: anything before it is out of date.
	now := time.Now()
	req := service.SyncRequest{Statuses: []service.SyncStatus{
		{ClientID: clientID(), ShipmentID: shipmentID, Status: "in_transit", RecordedAt: now},
		{ClientID: clientID(), ShipmentID: shipmentID, Status: "arrived", RecordedAt: now.Add(time.Millisecond)},
	}}
	time.Sleep(5 * time.Millisecond)
	for i := range 2 {
		resp, code := d.sync(req)
		if code != http.StatusOK || len(resp.Results) != 2 {
			t.Fatalf("attempt %d: status %d, results %+v", i+1, code, resp.Results)
		}
		for _, r := range resp.Results {
			if !r.OK {
				t.Errorf("attempt %d: %+v, want ok", i+1, r)
			}
		}
	}
	if n := scalar[int64](t, `SELECT count(*) FROM shipment_events WHERE shipment_id = $1 AND event_type IN ('status_changed', 'conflict')`, shipmentID); n != 2 {
		t.Errorf("%d status events, want 2", n)
	}

	// A change refused as out of date is refused again, once.
	stale := clientID()
	office.mustPost("/app/shipments/"+shipmentID+"/status", url.Values{"status": {"in_transit"}})
	for i := range 2 {
		if r := d.statusAt(stale, shipmentID, "issue", now.Add(-time.Minute)); r.OK || r.Code != "STALE" {
			t.Errorf("attempt %d: %+v, want STALE", i+1, r)
		}
	}
	if n := scalar[int64](t, `SELECT count(*) FROM shipment_events WHERE shipment_id = $1 AND event_type = 'conflict'`, shipmentID); n != 1 {
		t.Errorf("%d conflicts recorded, want 1", n)
	}
}

// TestDriverMessagesAreTheOffice (R4): the driver's own notes don't come
// back as messages from the office, and a cancellation without a note
// still says something.
func TestDriverMessagesAreTheOffice(t *testing.T) {
	e := newEnv(t)
	office := e.company("Message Movers")
	driverID, phone, pin := office.addDriver("Tigist Bekele")
	shipmentID, _ := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)

	resp, code := d.sync(service.SyncRequest{Messages: []service.SyncMessage{{
		ClientID: clientID(), ShipmentID: shipmentID, Text: "Loading now", RecordedAt: time.Now(),
	}}})
	if code != http.StatusOK || len(resp.Results) != 1 || !resp.Results[0].OK {
		t.Fatalf("send note: status %d, results %+v", code, resp.Results)
	}
	office.mustPost("/app/shipments/"+shipmentID+"/cancel", url.Values{"reason": {"customer_request"}})

	resp, _ = d.sync(service.SyncRequest{})
	var cancelled *service.DriverMessage
	for i, m := range resp.Messages {
		if strings.Contains(m.Text, "Loading now") {
			t.Errorf("the driver's own note came back from %q", m.From)
		}
		if m.Kind == "cancelled" {
			cancelled = &resp.Messages[i]
		}
	}
	if cancelled == nil {
		t.Fatalf("no cancellation among %+v", resp.Messages)
	}
	if cancelled.Text != "The office cancelled this shipment." || cancelled.Note != "" || cancelled.ShipmentID != shipmentID {
		t.Errorf("cancellation %+v", *cancelled)
	}
}

// TestManyMessagesArriveInPages (L8): more messages than one sync carries
// come in the next ones, none skipped.
func TestManyMessagesArriveInPages(t *testing.T) {
	e := newEnv(t)
	office := e.company("Chatty Cargo")
	driverID, phone, pin := office.addDriver("Meron Tadesse")
	shipmentID, _ := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)
	exec(t, `INSERT INTO shipment_events (tenant_id, shipment_id, event_type, driver_id, actor_name, note, visible_to_driver, created_at)
		SELECT tenant_id, id, 'note', driver_id, 'Dispatch', 'Note ' || g, true, NOW() - make_interval(secs => 300 - g)
		FROM shipments, generate_series(1, 250) g WHERE id = $1`, shipmentID)

	got := map[string]bool{}
	var since *time.Time
	for round := 1; ; round++ {
		if round > 5 {
			t.Fatal("still more messages after five syncs")
		}
		resp, code := d.sync(service.SyncRequest{Since: since})
		if code != http.StatusOK {
			t.Fatalf("sync %d: status %d", round, code)
		}
		for _, m := range resp.Messages {
			if strings.HasPrefix(m.Text, "Note ") {
				got[m.Text] = true
			}
		}
		since = &resp.Cursor
		if !resp.More {
			break
		}
	}
	if len(got) != 250 {
		t.Errorf("%d of 250 messages arrived", len(got))
	}
}

// TestPositionsKeptAfterReassignment (L9): positions a driver recorded
// offline for a load the office meanwhile gave to someone else are kept.
func TestPositionsKeptAfterReassignment(t *testing.T) {
	e := newEnv(t)
	office := e.company("Handover Positions")
	firstID, phone, pin := office.addDriver("Yonas Haile")
	secondID, _, _ := office.addDriver("Sara Mekonnen")
	shipmentID, _ := office.book("Addis Ababa", "Adama", firstID)
	d := e.signIn(phone, pin)
	if r := d.status(clientID(), shipmentID, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	office.mustPost("/app/shipments/"+shipmentID+"/assign", url.Values{"driver_id": {secondID}})

	resp, code := d.sync(service.SyncRequest{Locations: []service.SyncLocation{{
		ClientID: clientID(), ShipmentID: shipmentID, Lat: 8.75, Lng: 38.98, RecordedAt: time.Now().Add(-10 * time.Minute),
	}}})
	if code != http.StatusOK || resp.LocationsStored != 1 {
		t.Fatalf("sync: status %d, %d positions stored, want 1", code, resp.LocationsStored)
	}
	if n := scalar[int64](t, `SELECT count(*) FROM tracking_events WHERE shipment_id = $1 AND driver_id = $2`, shipmentID, firstID); n != 1 {
		t.Errorf("%d positions for the first driver, want 1", n)
	}

	// A load the driver never had takes no positions from them.
	other, _ := office.book("Addis Ababa", "Hawassa", secondID)
	resp, _ = d.sync(service.SyncRequest{Locations: []service.SyncLocation{{
		ClientID: clientID(), ShipmentID: other, Lat: 8.75, Lng: 38.98, RecordedAt: time.Now(),
	}}})
	if resp.LocationsStored != 0 {
		t.Errorf("%d positions stored for someone else's load", resp.LocationsStored)
	}
}

// TestHugePhotoRefused (R6, S4): a delivery photo claiming 8000×8000
// pixels is refused before it is decoded, for good, and so is an SVG
// signature.
func TestHugePhotoRefused(t *testing.T) {
	e := newEnv(t)
	office := e.company("Big Picture Freight")
	driverID, phone, pin := office.addDriver("Lulit Kebede")
	shipmentID, _ := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)
	if r := d.status(clientID(), shipmentID, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}

	r := d.pod(service.SyncPOD{ShipmentID: shipmentID, RecipientName: "Almaz Desta", Photos: []string{blankPNG(8000, 8000)}})
	if r.OK || r.Retry || !strings.Contains(r.Message, "too large") {
		t.Errorf("huge photo: %+v, want refused for good as too large", r)
	}
	svg := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))
	if r := d.pod(service.SyncPOD{ShipmentID: shipmentID, RecipientName: "Almaz Desta", Signature: svg}); r.OK || r.Retry {
		t.Errorf("SVG signature: %+v, want refused for good", r)
	}
	if n := scalar[int64](t, `SELECT count(*) FROM proof_of_deliveries WHERE shipment_id = $1`, shipmentID); n != 0 {
		t.Errorf("%d proofs stored, want none", n)
	}
}

// blankPNG is a valid, blank one-bit PNG of the given size: a few
// kilobytes, however large it claims to be.
func blankPNG(w, h int) string {
	var out bytes.Buffer
	out.WriteString("\x89PNG\r\n\x1a\n")
	chunk := func(kind string, data []byte) {
		_ = binary.Write(&out, binary.BigEndian, uint32(len(data)))
		c := crc32.NewIEEE()
		c.Write([]byte(kind))
		c.Write(data)
		out.WriteString(kind)
		out.Write(data)
		_ = binary.Write(&out, binary.BigEndian, c.Sum32())
	}
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], uint32(w))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(h))
	ihdr[8] = 1 // one bit, greyscale
	chunk("IHDR", ihdr)
	var idat bytes.Buffer
	z := zlib.NewWriter(&idat)
	row := make([]byte, 1+(w+7)/8)
	for range h {
		_, _ = z.Write(row)
	}
	_ = z.Close()
	chunk("IDAT", idat.Bytes())
	chunk("IEND", nil)
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(out.Bytes())
}

// TestAlertBadgeIsPerPerson (W3): one dispatcher opening the alert list
// clears their own badge, not a colleague's.
func TestAlertBadgeIsPerPerson(t *testing.T) {
	e := newEnv(t)
	admin := e.company("Badge Logistics")
	email := "dispatch" + phoneNumber() + "@example.et"
	p := admin.post("/app/team", url.Values{"full_name": {"Dawit Bekele"}, "email": {email}, "phone": {phoneNumber()}, "role": {"dispatcher"}})
	m := secretRe.FindStringSubmatch(p.Body)
	if m == nil {
		t.Fatalf("add dispatcher: status %d %s", p.Status, errorText(p.Body))
	}
	disp := e.browser()
	disp.mustPost("/login", url.Values{"identifier": {email}, "password": {strings.TrimSpace(m[1])}})
	disp.mustPost("/app/account/password", url.Values{
		"current_password": {strings.TrimSpace(m[1])}, "new_password": {"dispatch-pass-9"}, "confirm_password": {"dispatch-pass-9"},
	})
	badge := func(b *browser) int {
		t.Helper()
		var out struct{ Unread int }
		p := b.get("/app/api/badge")
		if p.Status != http.StatusOK {
			t.Fatalf("badge: status %d", p.Status)
		}
		if err := json.Unmarshal([]byte(p.Body), &out); err != nil {
			t.Fatal(err)
		}
		return out.Unread
	}
	exec(t, `INSERT INTO alerts (tenant_id, alert_type, severity, message) VALUES ($1, 'driver_offline', 'warning', 'Quiet driver')`, admin.tenant())
	if a, d := badge(admin), badge(disp); a != 1 || d != 1 {
		t.Fatalf("badges %d and %d, want 1 each", a, d)
	}
	admin.get("/app/alerts")
	if a, d := badge(admin), badge(disp); a != 0 || d != 1 {
		t.Errorf("after the admin opened the list: badges %d and %d, want 0 and 1", a, d)
	}
	// A new alert shows on both again.
	exec(t, `INSERT INTO alerts (tenant_id, alert_type, severity, message, created_at) VALUES ($1, 'sms_paused', 'warning', 'Texts paused', NOW() + INTERVAL '1 second')`, admin.tenant())
	if a, d := badge(admin), badge(disp); a != 1 || d != 2 {
		t.Errorf("after a new alert: badges %d and %d, want 1 and 2", a, d)
	}
}

// TestAmharicNamesFit (W9): name limits count letters, not bytes. An
// Amharic letter takes three bytes, so 50 letters used to be refused as
// "too long".
func TestAmharicNamesFit(t *testing.T) {
	e := newEnv(t)
	office := e.company("Amharic Names")
	name := strings.Repeat("ሀ", 100) // 300 bytes
	p := office.post("/app/drivers", url.Values{"full_name": {name}, "phone": {phoneNumber()}})
	if strings.Contains(errorText(p.Body), "too long") {
		t.Fatalf("a 100-letter Amharic name was refused: %s", errorText(p.Body))
	}
	if n := scalar[int64](t, `SELECT count(*) FROM users WHERE tenant_id = $1 AND full_name = $2`, office.tenant(), name); n != 1 {
		t.Errorf("%d drivers with the name, want 1 (%d %s)", n, p.Status, errorText(p.Body))
	}
	if p := office.post("/app/drivers", url.Values{"full_name": {strings.Repeat("ሀ", 121)}, "phone": {phoneNumber()}}); !strings.Contains(errorText(p.Body), "too long") {
		t.Errorf("121 letters: %d %s, want too long", p.Status, errorText(p.Body))
	}
}

// TestDeliveredStraightFromAssigned (L1): a proof of delivery sent before
// the trip was started still records when the load left.
func TestDeliveredStraightFromAssigned(t *testing.T) {
	e := newEnv(t)
	office := e.company("Direct Delivery")
	driverID, phone, pin := office.addDriver("Bereket Tadesse")
	shipmentID, _ := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)
	if r := d.pod(service.SyncPOD{ShipmentID: shipmentID, RecipientName: "Almaz Desta"}); !r.OK {
		t.Fatalf("deliver: %+v", r)
	}
	if !scalar[bool](t, `SELECT status = 'delivered' AND picked_up_at IS NOT NULL AND picked_up_at <= actual_delivery FROM shipments WHERE id = $1`, shipmentID) {
		t.Error("a delivery straight from assigned should record a departure no later than the delivery")
	}
}

// TestReopenFollowsTheRules (L2): reopening is new work. A driver at their
// limit doesn't get the load back, and a read-only company can't reopen.
func TestReopenFollowsTheRules(t *testing.T) {
	e := newEnv(t)
	office := e.company("Reopen Rules")
	driverID, _, _ := office.addDriver("Mulu Alemu")
	exec(t, `UPDATE tenants SET max_active_per_driver = 1 WHERE id = $1`, office.tenant())
	first, _ := office.book("Addis Ababa", "Adama", driverID)
	office.mustPost("/app/shipments/"+first+"/cancel", url.Values{"reason": {"customer_request"}})
	office.book("Addis Ababa", "Hawassa", driverID) // the driver's one place

	if msg, failed := office.result("/app/shipments/"+first+"/reopen", nil); failed {
		t.Fatalf("reopen: %s", msg)
	}
	if !scalar[bool](t, `SELECT status = 'pending' AND driver_id IS NULL FROM shipments WHERE id = $1`, first) {
		t.Error("a driver at their limit should not get the reopened load back")
	}
	if note := scalar[string](t, `SELECT note FROM shipment_events WHERE shipment_id = $1 AND event_type = 'unassigned'`, first); !strings.Contains(note, "Mulu Alemu already has 1 active") {
		t.Errorf("unassigned note %q should say why", note)
	}

	office.mustPost("/app/shipments/"+first+"/cancel", url.Values{"reason": {"customer_request"}})
	exec(t, `UPDATE tenants SET trial_ends_at = NOW() - INTERVAL '1 minute' WHERE id = $1`, office.tenant())
	if msg, failed := office.result("/app/shipments/"+first+"/reopen", nil); !failed || !strings.Contains(msg, "read-only") {
		t.Errorf("reopen in read-only mode: %q", msg)
	}
}

// TestVehicleOnOneTripWithinCapacity (L3): a truck on one driver's trip
// can't be given to another, nor loaded past its capacity. A driver's usual
// truck in that state is left off rather than blocking the assignment.
func TestVehicleOnOneTripWithinCapacity(t *testing.T) {
	e := newEnv(t)
	office := e.company("One Truck Freight")
	office.mustPost("/app/vehicles", url.Values{"plate_number": {"3-B22222"}, "vehicle_type": {"heavy_truck"}, "capacity_kg": {"15000"}, "active": {"on"}})
	truck := scalar[string](t, `SELECT id::text FROM vehicles WHERE tenant_id = $1`, office.tenant())
	first, _, _ := office.addDriver("Girma Wolde")
	second, _, _ := office.addDriver("Hirut Bekele")

	a, _ := office.book("Addis Ababa", "Adama", "") // 10,000 kg
	if msg, failed := office.result("/app/shipments/"+a+"/assign", url.Values{"driver_id": {first}, "vehicle_id": {truck}}); failed {
		t.Fatalf("assign: %s", msg)
	}
	b, _ := office.book("Addis Ababa", "Hawassa", "")
	if msg, failed := office.result("/app/shipments/"+b+"/assign", url.Values{"driver_id": {second}, "vehicle_id": {truck}}); !failed || !strings.Contains(msg, "on Girma Wolde's trip") {
		t.Errorf("a truck on another driver's trip: %q", msg)
	}
	if msg, failed := office.result("/app/shipments/"+b+"/assign", url.Values{"driver_id": {first}, "vehicle_id": {truck}}); !failed || !strings.Contains(msg, "carries up to 15,000 kg") {
		t.Errorf("a second 10,000 kg load on a 15,000 kg truck: %q", msg)
	}

	// The truck is the second driver's usual one: assigning them still works,
	// without it.
	exec(t, `UPDATE users SET vehicle_id = $1 WHERE id = $2`, truck, second)
	if msg, failed := office.result("/app/shipments/"+b+"/assign", url.Values{"driver_id": {second}}); failed {
		t.Fatalf("assign with a busy usual truck: %s", msg)
	}
	if !scalar[bool](t, `SELECT driver_id = $2 AND vehicle_id IS NULL FROM shipments WHERE id = $1`, b, second) {
		t.Error("the busy usual truck should have been left off")
	}
}

// TestRenewalKeepsTheBillingDay (L6): a company's billing day is kept and
// each renewal ends on it, not on whatever day the last period ended.
func TestRenewalKeepsTheBillingDay(t *testing.T) {
	e := newEnv(t)
	office := e.subscribed("Billing Day Haulage", "starter", e.operator())
	tenant := office.tenant()
	addis, _ := time.LoadLocation("Africa/Addis_Ababa")
	if day := scalar[int32](t, `SELECT billing_anchor_day FROM tenants WHERE id = $1`, tenant); day != int32(time.Now().In(addis).Day()) {
		t.Errorf("billing day %d, want today's", day)
	}

	// As if the company started on the 31st and this period ended early in
	// a short month.
	exec(t, `UPDATE tenants SET billing_anchor_day = 31, period_start = NOW() - INTERVAL '1 month', period_end = NOW() - INTERVAL '1 minute' WHERE id = $1`, tenant)
	start := scalar[time.Time](t, `SELECT period_end FROM tenants WHERE id = $1`, tenant)
	if err := e.svc.Billing.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	end := scalar[time.Time](t, `SELECT period_end FROM tenants WHERE id = $1`, tenant)
	if want := model.PeriodEnd(start.In(addis), model.CycleMonthly, 31); !end.Equal(want) {
		t.Errorf("renewed to %s, want %s", end.In(addis).Format("2 Jan 2006 15:04"), want.Format("2 Jan 2006 15:04"))
	}
	if day := scalar[int32](t, `SELECT billing_anchor_day FROM tenants WHERE id = $1`, tenant); day != 31 {
		t.Errorf("billing day %d after renewal, want 31", day)
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
