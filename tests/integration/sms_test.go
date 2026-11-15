//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// TestSMSTexts: what drivers and customers receive says what to do next.
func TestSMSTexts(t *testing.T) {
	e := newEnv(t)
	office := e.company("Texting Transport")
	driverID, phone, pin := office.addDriver("Bereket Assefa")
	shipmentID, tracking := office.book("Addis Ababa", "Adama", driverID)
	ref := service.ShortRef(tracking)

	e.received("sms", phone, "text ET "+ref+" 1 to 6060")

	d := e.signIn(phone, pin)
	if r := d.status(clientID(), shipmentID, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	if r := d.pod(service.SyncPOD{ShipmentID: shipmentID, RecipientName: "Ruth Alemu"}); !r.OK {
		t.Fatalf("proof: %+v", r)
	}
	body := scalar[string](t, `SELECT body FROM sms_messages WHERE shipment_id = $1 AND body LIKE '%was delivered%'`, shipmentID)
	if !strings.Contains(body, "delivered to Ruth Alemu on ") {
		t.Errorf("delivered SMS %q should say when", body)
	}

	// The simulator says what happened instead of always succeeding.
	for _, c := range []struct{ from, body, want string }{
		{"not-a-number", "1", "is not a phone number"},
		{phoneNumber(), "1", "not an active driver"},
		{phone, "HELP", "list of codes"},
	} {
		office.mustPost("/app/settings/sms/simulate", url.Values{"from": {c.from}, "body": {c.body}})
		if p := office.get("/app/settings/sms"); !strings.Contains(p.Body, c.want) {
			t.Errorf("simulating %q from %q: page should say %q", c.body, c.from, c.want)
		}
	}

	// The webhook reports the outcome too.
	_, next := office.book("Addis Ababa", "Bishoftu", driverID)
	nextRef := service.ShortRef(next)
	for _, c := range []struct{ body, want string }{
		{"ET " + nextRef + " 3", "refused"}, // arrived before setting off
		{"ET " + nextRef + " 1", "recorded"},
		{"ET " + nextRef + " 1", "unchanged"},
		{"ET " + ref + " 1", "replied"}, // delivered already: not active
	} {
		req, _ := http.NewRequest("POST", e.srv.URL+"/api/v1/sms/inbound", strings.NewReader(`{"from":"`+phone+`","body":"`+c.body+`"}`))
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var out service.InboundResult
		err = json.NewDecoder(res.Body).Decode(&out)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if out.Outcome != c.want {
			t.Errorf("%q: outcome %+v, want %s", c.body, out, c.want)
		}
	}
}

// TestOfficeMessageFallsBackToSMS: a driver whose app hasn't synced lately
// gets office messages by SMS as well (AC-035.1).
func TestOfficeMessageFallsBackToSMS(t *testing.T) {
	e := newEnv(t)
	office := e.company("Message Movers")
	driverID, phone, pin := office.addDriver("Tsion Bekele")
	shipmentID, _ := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)
	d.sync(service.SyncRequest{})

	office.mustPost("/app/shipments/"+shipmentID+"/note", url.Values{"to": {"driver"}, "note": {"Gate 3 please"}})
	if n := scalar[int64](t, `SELECT count(*) FROM sms_messages WHERE shipment_id = $1 AND body LIKE '%Gate 3 please%'`, shipmentID); n != 0 {
		t.Error("a driver who just synced should get the message in the app only")
	}

	exec(t, `UPDATE sync_metadata SET last_sync_at = NOW() - INTERVAL '20 minutes' WHERE driver_id = $1`, driverID)
	office.mustPost("/app/shipments/"+shipmentID+"/note", url.Values{"to": {"driver"}, "note": {"Call the gatekeeper"}})
	e.received("sms", phone, "Call the gatekeeper")
	if p := office.get("/app/shipments/" + shipmentID); !strings.Contains(p.Body, "hadn&#39;t synced for 15 minutes") {
		t.Error("the timeline should say the message also went by SMS")
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

// TestExpiredCodesAreNotSent (L5): a code still waiting when it expires
// isn't sent late.
func TestExpiredCodesAreNotSent(t *testing.T) {
	newEnv(t) // its SMS worker picks the code up
	phone := phoneNumber()
	exec(t, `INSERT INTO sms_messages (direction, phone, body, purpose, sensitive, status, created_at)
		VALUES ('outbound', $1, 'Kiflet: your code is 123456', 'otp', true, 'queued', NOW() - INTERVAL '11 minutes')`, phone)
	within(t, 10*time.Second, "the expired code to fail", func() bool {
		return scalar[string](t, `SELECT status FROM sms_messages WHERE phone = $1`, phone) == "failed"
	})
	if why := scalar[string](t, `SELECT last_error FROM sms_messages WHERE phone = $1`, phone); !strings.Contains(why, "expired") {
		t.Errorf("failed because %q", why)
	}
}
