//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// TestDismissedAlertsStayDismissed: the checks run every minute, so an alert
// the office dismissed must not come straight back. A new ETA is a new
// situation and alerts again.
func TestDismissedAlertsStayDismissed(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	office := e.company("Episode Transport")
	driverID, phone, pin := office.addDriver("Kebede Alemu")
	shipmentID, _ := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)
	if r := d.status(clientID(), shipmentID, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}

	exec(t, `UPDATE shipments SET estimated_delivery = NOW() - INTERVAL '1 hour' WHERE id = $1`, shipmentID)
	if err := e.svc.Jobs.CheckETAs(ctx); err != nil {
		t.Fatal(err)
	}
	alertID := scalar[string](t, `SELECT id::text FROM alerts WHERE shipment_id = $1 AND alert_type = 'eta_exceeded' AND acknowledged_at IS NULL`, shipmentID)
	office.mustPost("/app/alerts/"+alertID+"/ack", nil)

	for range 3 {
		if err := e.svc.Jobs.CheckETAs(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if n := scalar[int64](t, `SELECT count(*) FROM alerts WHERE shipment_id = $1 AND alert_type = 'eta_exceeded'`, shipmentID); n != 1 {
		t.Errorf("%d ETA alerts after dismissing one, want 1", n)
	}

	exec(t, `UPDATE shipments SET estimated_delivery = NOW() - INTERVAL '10 minutes' WHERE id = $1`, shipmentID)
	if err := e.svc.Jobs.CheckETAs(ctx); err != nil {
		t.Fatal(err)
	}
	if n := scalar[int64](t, `SELECT count(*) FROM alerts WHERE shipment_id = $1 AND alert_type = 'eta_exceeded' AND acknowledged_at IS NULL`, shipmentID); n != 1 {
		t.Errorf("%d open ETA alerts after the ETA moved, want 1", n)
	}

	// Back on time: the alert closes itself.
	exec(t, `UPDATE shipments SET estimated_delivery = NOW() + INTERVAL '2 hours' WHERE id = $1`, shipmentID)
	if err := e.svc.Jobs.CheckETAs(ctx); err != nil {
		t.Fatal(err)
	}
	if n := scalar[int64](t, `SELECT count(*) FROM alerts WHERE shipment_id = $1 AND alert_type = 'eta_exceeded' AND acknowledged_at IS NULL`, shipmentID); n != 0 {
		t.Errorf("%d open ETA alerts once back on time, want 0", n)
	}
}

// TestPlannedPickup: a load booked days ahead is not late before it leaves,
// its ETA counts from the pickup, and a missed pickup is flagged.
func TestPlannedPickup(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	office := e.company("Pickup Freight")
	driverID, phone, pin := office.addDriver("Hana Girma")
	tz, _ := time.LoadLocation("Africa/Addis_Ababa")
	pickup := time.Now().In(tz).Add(72 * time.Hour).Truncate(time.Minute)
	p := office.mustPost("/app/shipments", url.Values{
		"priority": {"normal"}, "origin_city": {"Addis Ababa"}, "origin_address": {"Main depot"},
		"destination_city": {"Hawassa"}, "destination_address": {"Customer warehouse"},
		"customer_name": {"Sidama Coffee"}, "customer_phone": {phoneNumber()}, "driver_id": {driverID},
		"planned_pickup": {pickup.Format("2006-01-02T15:04")},
	})
	shipmentID := strings.TrimPrefix(p.Location, "/app/shipments/")

	if !scalar[bool](t, `SELECT estimated_delivery > planned_pickup_at FROM shipments WHERE id = $1`, shipmentID) {
		t.Error("the booking ETA should count from the planned pickup")
	}
	if err := e.svc.Jobs.CheckPickups(ctx); err != nil {
		t.Fatal(err)
	}
	// Even with its ETA in the past, a load not yet on the road is not "late".
	exec(t, `UPDATE shipments SET estimated_delivery = NOW() - INTERVAL '1 hour' WHERE id = $1`, shipmentID)
	if err := e.svc.Jobs.CheckETAs(ctx); err != nil {
		t.Fatal(err)
	}
	if n := scalar[int64](t, `SELECT count(*) FROM alerts WHERE shipment_id = $1`, shipmentID); n != 0 {
		t.Fatalf("%d alerts for a load that isn't due yet, want 0", n)
	}

	exec(t, `UPDATE shipments SET planned_pickup_at = NOW() - INTERVAL '3 hours' WHERE id = $1`, shipmentID)
	if err := e.svc.Jobs.CheckPickups(ctx); err != nil {
		t.Fatal(err)
	}
	msg := scalar[string](t, `SELECT message FROM alerts WHERE shipment_id = $1 AND alert_type = 'pickup_overdue' AND acknowledged_at IS NULL`, shipmentID)
	if !strings.Contains(msg, "the driver has not set off") {
		t.Errorf("pickup alert %q should say the driver hasn't left", msg)
	}

	d := e.signIn(phone, pin)
	if r := d.status(clientID(), shipmentID, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	if err := e.svc.Jobs.CheckPickups(ctx); err != nil {
		t.Fatal(err)
	}
	if n := scalar[int64](t, `SELECT count(*) FROM alerts WHERE shipment_id = $1 AND alert_type = 'pickup_overdue' AND acknowledged_at IS NULL`, shipmentID); n != 0 {
		t.Errorf("pickup alert still open after departure")
	}
}

// TestRepeatDelayIsRecorded: a second delay report with a new cause is news
// for the office, though the customer isn't texted again.
func TestRepeatDelayIsRecorded(t *testing.T) {
	e := newEnv(t)
	office := e.company("Delay Logistics")
	driverID, phone, pin := office.addDriver("Dawit Bekele")
	shipmentID, _ := office.book("Addis Ababa", "Bahir Dar", driverID)
	exec(t, `UPDATE tenants SET sms_on_delayed = true WHERE id = (SELECT tenant_id FROM shipments WHERE id = $1)`, shipmentID)
	d := e.signIn(phone, pin)
	if r := d.status(clientID(), shipmentID, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	report := func(id, reason, note string) service.SyncResult {
		t.Helper()
		resp, code := d.sync(service.SyncRequest{Statuses: []service.SyncStatus{{
			ClientID: id, ShipmentID: shipmentID, Status: "delayed", Reason: reason, Note: note, RecordedAt: time.Now(),
		}}})
		if code != http.StatusOK || len(resp.Results) != 1 {
			t.Fatalf("sync: %d %+v", code, resp.Results)
		}
		return resp.Results[0]
	}

	if r := report(clientID(), "traffic", ""); !r.OK {
		t.Fatalf("first delay: %+v", r)
	}
	second := clientID()
	for range 2 { // the second time is a replay
		if r := report(second, "mechanical_issue", "Gearbox"); !r.OK {
			t.Fatalf("second delay: %+v", r)
		}
	}
	if r := report(clientID(), "mechanical_issue", ""); !r.OK {
		t.Fatalf("same delay again: %+v", r)
	}

	if n := scalar[int64](t, `SELECT count(*) FROM shipment_events WHERE shipment_id = $1 AND event_type = 'status_updated'`, shipmentID); n != 1 {
		t.Errorf("%d delay updates recorded, want 1", n)
	}
	if got := scalar[string](t, `SELECT status_reason FROM shipments WHERE id = $1`, shipmentID); got != "mechanical_issue" {
		t.Errorf("status reason %q, want the new cause", got)
	}
	open := scalar[string](t, `SELECT message FROM alerts WHERE shipment_id = $1 AND alert_type = 'delay_reported' AND acknowledged_at IS NULL`, shipmentID)
	if !strings.Contains(open, "updated the delay") || !strings.Contains(open, "Gearbox") {
		t.Errorf("open delay alert %q should describe the new cause", open)
	}
	if n := scalar[int64](t, `SELECT count(*) FROM sms_messages WHERE shipment_id = $1 AND body LIKE '%is delayed%'`, shipmentID); n != 1 {
		t.Errorf("customer texted %d times about the delay, want 1", n)
	}
	if p := office.get("/app/shipments/" + shipmentID); !strings.Contains(p.Body, "Delay updated") {
		t.Error("the timeline should show the delay update")
	}
}

// TestHeldPODAfterReassignment: a driver who delivered while offline, after
// the office gave the load to someone else, doesn't lose the proof. The
// office accepts it and the delivery is credited to them.
func TestHeldPODAfterReassignment(t *testing.T) {
	e := newEnv(t)
	office := e.company("Handover Haulage")
	firstID, firstPhone, firstPIN := office.addDriver("Abel Tesfaye")
	secondID, secondPhone, secondPIN := office.addDriver("Selam Worku")
	shipmentID, _ := office.book("Addis Ababa", "Adama", firstID)
	first := e.signIn(firstPhone, firstPIN)
	second := e.signIn(secondPhone, secondPIN)
	if r := first.status(clientID(), shipmentID, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	resp, _ := second.sync(service.SyncRequest{})
	cursor := resp.Cursor

	delivered := time.Now().Add(-30 * time.Minute)
	office.mustPost("/app/shipments/"+shipmentID+"/assign", url.Values{"driver_id": {secondID}})

	pod := service.SyncPOD{ClientID: clientID(), ShipmentID: shipmentID, RecipientName: "Almaz Desta", RecordedAt: delivered}
	for i := range 2 { // the second time is a replay
		r := first.pod(pod)
		if r.OK || r.Retry || r.Code != "CONFLICT" || !strings.Contains(r.Message, "given to another driver") {
			t.Fatalf("attempt %d: %+v, want a final conflict about the handover", i+1, r)
		}
	}
	if n := scalar[int64](t, `SELECT count(*) FROM proof_of_deliveries WHERE shipment_id = $1`, shipmentID); n != 1 {
		t.Fatalf("%d proofs stored, want 1", n)
	}
	podID := scalar[string](t, `SELECT id::text FROM proof_of_deliveries WHERE shipment_id = $1 AND status = 'held' AND held_reason = 'reassigned'`, shipmentID)
	if got := scalar[string](t, `SELECT status FROM shipments WHERE id = $1`, shipmentID); got != "in_transit" {
		t.Errorf("a held proof changed the shipment to %q", got)
	}
	scalar[string](t, `SELECT id::text FROM alerts WHERE shipment_id = $1 AND alert_type = 'delivery_held' AND severity = 'critical' AND acknowledged_at IS NULL`, shipmentID)

	page := office.get("/app/shipments/" + shipmentID)
	if !strings.Contains(page.Body, "Delivery to review") || !strings.Contains(page.Body, "Almaz Desta") {
		t.Fatal("the shipment page should offer the held proof for review")
	}
	if p := office.get("/app/shipments/" + shipmentID + "/pods/" + podID + "/media/0"); p.Status != http.StatusOK || !strings.HasPrefix(p.Body, "\x89PNG") {
		t.Errorf("held signature: status %d", p.Status)
	}

	office.mustPost("/app/shipments/"+shipmentID+"/pods/"+podID+"/accept", nil)
	if got := scalar[string](t, `SELECT status || ' ' || driver_id::text FROM shipments WHERE id = $1`, shipmentID); got != "delivered "+firstID {
		t.Errorf("after accepting: %q, want delivered by the first driver", got)
	}
	if lag := scalar[float64](t, `SELECT abs(extract(epoch FROM actual_delivery - $2::timestamptz)) FROM shipments WHERE id = $1`, shipmentID, delivered); lag > 1 {
		t.Errorf("delivery time is %.0fs off the proof's time", lag)
	}
	if got := scalar[string](t, `SELECT status FROM proof_of_deliveries WHERE id = $1`, podID); got != "accepted" {
		t.Errorf("proof status %q, want accepted", got)
	}
	if n := scalar[int64](t, `SELECT count(*) FROM alerts WHERE shipment_id = $1 AND alert_type = 'delivery_held' AND acknowledged_at IS NULL`, shipmentID); n != 0 {
		t.Error("the review alert should close once the proof is accepted")
	}
	if p := office.post("/app/shipments/"+shipmentID+"/pods/"+podID+"/accept", nil); p.Status != http.StatusSeeOther {
		t.Errorf("accepting twice: status %d", p.Status)
	}

	// The second driver's phone lets go of the shipment.
	resp, _ = second.sync(service.SyncRequest{Since: &cursor})
	if len(resp.RemovedShipmentIDs) != 1 || resp.RemovedShipmentIDs[0] != shipmentID {
		t.Errorf("second driver's removed shipments %v, want the delivered one", resp.RemovedShipmentIDs)
	}
}

// TestHeldPODOnCancelledShipment: the office can dismiss a proof recorded
// on a load it had cancelled; the shipment stays cancelled.
func TestHeldPODOnCancelledShipment(t *testing.T) {
	e := newEnv(t)
	office := e.company("Dismissal Cargo")
	driverID, phone, pin := office.addDriver("Yonas Kebede")
	shipmentID, _ := office.book("Addis Ababa", "Mekelle", driverID)
	d := e.signIn(phone, pin)
	if r := d.status(clientID(), shipmentID, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	office.mustPost("/app/shipments/"+shipmentID+"/cancel", url.Values{"reason": {"returned_to_origin"}})

	r := d.pod(service.SyncPOD{ShipmentID: shipmentID, RecipientName: "Gebre Hailu"})
	if r.OK || r.Code != "CONFLICT" || !strings.Contains(r.Message, "cancelled") {
		t.Fatalf("proof on a cancelled shipment: %+v", r)
	}
	podID := scalar[string](t, `SELECT id::text FROM proof_of_deliveries WHERE shipment_id = $1 AND status = 'held' AND held_reason = 'cancelled'`, shipmentID)
	office.mustPost("/app/shipments/"+shipmentID+"/pods/"+podID+"/dismiss", nil)

	if got := scalar[string](t, `SELECT status FROM shipments WHERE id = $1`, shipmentID); got != "cancelled" {
		t.Errorf("status %q after dismissing, want cancelled", got)
	}
	if got := scalar[string](t, `SELECT status FROM proof_of_deliveries WHERE id = $1`, podID); got != "dismissed" {
		t.Errorf("proof status %q, want dismissed", got)
	}
	if n := scalar[int64](t, `SELECT count(*) FROM alerts WHERE shipment_id = $1 AND alert_type = 'delivery_held' AND acknowledged_at IS NULL`, shipmentID); n != 0 {
		t.Error("the review alert should close once the proof is dismissed")
	}
	if p := office.get("/app/shipments/" + shipmentID); !strings.Contains(p.Body, "Dismissed delivery record") {
		t.Error("a dismissed proof should stay on the shipment page")
	}
}

// TestRefusedPODLeavesNoFiles: images of a proof that is refused are not
// kept.
func TestRefusedPODLeavesNoFiles(t *testing.T) {
	e := newEnv(t)
	office := e.company("Clean Storage")
	driverID, phone, pin := office.addDriver("Liya Mamo")
	_, strangerPhone, strangerPIN := office.addDriver("Not Assigned")
	shipmentID, _ := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)
	stranger := e.signIn(strangerPhone, strangerPIN)
	if r := d.status(clientID(), shipmentID, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}

	for name, try := range map[string]func() service.SyncResult{
		"no recipient": func() service.SyncResult { return d.pod(service.SyncPOD{ShipmentID: shipmentID, RecipientName: " "}) },
		"bad phone": func() service.SyncResult {
			return d.pod(service.SyncPOD{ShipmentID: shipmentID, RecipientName: "A", RecipientPhone: "12"})
		},
		"bad photo": func() service.SyncResult {
			return d.pod(service.SyncPOD{ShipmentID: shipmentID, RecipientName: "A", Photos: []string{"data:image/jpeg;base64,!!"}})
		},
		"never theirs": func() service.SyncResult {
			return stranger.pod(service.SyncPOD{ShipmentID: shipmentID, RecipientName: "A"})
		},
	} {
		if r := try(); r.OK || r.Retry {
			t.Errorf("%s: %+v, want a final refusal", name, r)
		}
	}
	if files := storedFiles(t, e.files); len(files) != 0 {
		t.Errorf("refused proofs left files behind: %v", files)
	}
	if n := scalar[int64](t, `SELECT count(*) FROM proof_of_deliveries WHERE shipment_id = $1`, shipmentID); n != 0 {
		t.Errorf("%d proofs stored, want 0", n)
	}

	if r := d.pod(service.SyncPOD{ShipmentID: shipmentID, RecipientName: "Meron Abebe"}); !r.OK {
		t.Fatalf("valid proof: %+v", r)
	}
	if files := storedFiles(t, e.files); len(files) != 1 {
		t.Errorf("stored files %v, want the one signature", files)
	}
}

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

// TestCancelReasonsFollowTheRoad: a load that has left is cancelled for
// what happened on the trip, and reports count the reasons.
func TestCancelReasonsFollowTheRoad(t *testing.T) {
	e := newEnv(t)
	office := e.company("Refusal Road")
	driverID, phone, pin := office.addDriver("Fikru Tola")
	waiting, _ := office.book("Addis Ababa", "Adama", driverID)
	rolling, _ := office.book("Addis Ababa", "Hawassa", driverID)

	if p := office.post("/app/shipments/"+waiting+"/cancel", url.Values{"reason": {"returned_to_origin"}}); p.Status == http.StatusSeeOther && scalar[string](t, `SELECT status FROM shipments WHERE id = $1`, waiting) == "cancelled" {
		t.Error("a load that hasn't left can't be returned to origin")
	}
	d := e.signIn(phone, pin)
	if r := d.status(clientID(), rolling, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	office.post("/app/shipments/"+rolling+"/cancel", url.Values{"reason": {"duplicate"}})
	if got := scalar[string](t, `SELECT status FROM shipments WHERE id = $1`, rolling); got == "cancelled" {
		t.Error("a load on the road can't be cancelled as a duplicate booking")
	}
	office.mustPost("/app/shipments/"+rolling+"/cancel", url.Values{"reason": {"recipient_refused"}})
	office.mustPost("/app/shipments/"+waiting+"/cancel", url.Values{"reason": {"customer_request"}})

	p := office.get("/app/reports")
	if !strings.Contains(p.Body, "Recipient refused delivery") || !strings.Contains(p.Body, "Customer request") {
		t.Error("reports should count cancellations by reason")
	}
}

// TestPlatformAdminStaysOnPlatform: the platform admin's own company is
// empty; its pages send them to the companies list.
func TestPlatformAdminStaysOnPlatform(t *testing.T) {
	e := newEnv(t)
	admin := e.company("Platform Operator")
	exec(t, `UPDATE users SET role = 'platform_admin' WHERE tenant_id = (SELECT tenant_id FROM users WHERE full_name = 'Owner of Platform Operator' LIMIT 1)`)

	for _, path := range []string{"/app", "/app/shipments", "/app/settings"} {
		if p := admin.get(path); p.Status != http.StatusSeeOther || p.Location != "/app/platform" {
			t.Errorf("GET %s: %d → %q, want a redirect to /app/platform", path, p.Status, p.Location)
		}
	}
	if p := admin.post("/app/shipments", url.Values{}); p.Status != http.StatusForbidden {
		t.Errorf("POST /app/shipments: status %d, want 403", p.Status)
	}
	if p := admin.get("/app/platform"); p.Status != http.StatusOK {
		t.Errorf("platform page: status %d", p.Status)
	}
	if p := admin.get("/app/account"); p.Status != http.StatusOK {
		t.Errorf("account page: status %d", p.Status)
	}
}

func exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// storedFiles lists every file under a storage directory.
func storedFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, de os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !de.IsDir() {
			out = append(out, strings.TrimPrefix(p, dir))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
