//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/service"
)

func rulesForm(over map[string]string) url.Values {
	v := url.Values{
		"problems_on": {"1"}, "late_on": {"1"}, "late_minutes": {"0"}, "offline_on": {"1"}, "offline_minutes": {"240"},
		"stop_on": {"1"}, "stop_minutes": {"45"}, "route_on": {"1"}, "route_km": {"20"},
	}
	for k, x := range over {
		if x == "" {
			v.Del(k)
		} else {
			v.Set(k, x)
		}
	}
	return v
}

// TestAlertRules: a company decides what it is alerted about, and when; an
// administrator sets the rules, and the checks follow them.
func TestAlertRules(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	office := e.company("Rulebook Transport")
	driverID, phone, pin := office.addDriver("Kebede Alemu")
	id, _ := office.book("Addis Ababa", "Hawassa", driverID)
	d := e.signIn(phone, pin)
	if r := d.status(clientID(), id, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	count := func(kind string) int64 {
		return scalar[int64](t, `SELECT count(*) FROM alerts WHERE shipment_id = $1 AND alert_type = $2`, id, kind)
	}

	// Out of bounds, and not for a dispatcher.
	if p := office.post("/app/alerts/rules", rulesForm(map[string]string{"stop_minutes": "2"})); !strings.Contains(office.get("/app/alerts").Body, "A long stop takes 10 to 720 minutes") {
		t.Errorf("a two-minute long stop was taken: %d", p.Status)
	}
	if p := office.dispatcher().post("/app/alerts/rules", rulesForm(nil)); p.Status != http.StatusForbidden {
		t.Errorf("a dispatcher changing the rules: status %d, want 403", p.Status)
	}

	// Late only after an hour; problems reported by drivers off.
	office.mustPost("/app/alerts/rules", rulesForm(map[string]string{"late_minutes": "60", "problems_on": ""}))
	if p := office.get("/app/alerts"); !strings.Contains(p.Body, `name="late_minutes" type="number" inputmode="numeric" min="0" max="1440" value="60"`) {
		t.Error("the page should show the saved rule")
	}
	exec(t, `UPDATE shipments SET estimated_delivery = NOW() - INTERVAL '30 minutes' WHERE id = $1`, id)
	if err := e.svc.Jobs.CheckETAs(ctx); err != nil {
		t.Fatal(err)
	}
	if n := count("eta_exceeded"); n != 0 {
		t.Errorf("alerted %d times 30 minutes past the ETA, with the rule at an hour", n)
	}
	exec(t, `UPDATE shipments SET estimated_delivery = NOW() - INTERVAL '61 minutes' WHERE id = $1`, id)
	if err := e.svc.Jobs.CheckETAs(ctx); err != nil {
		t.Fatal(err)
	}
	if n := count("eta_exceeded"); n != 1 {
		t.Errorf("%d late alerts an hour past the ETA, want 1", n)
	}
	resp, code := d.sync(service.SyncRequest{Statuses: []service.SyncStatus{{
		ClientID: clientID(), ShipmentID: id, Status: "delayed", Reason: "traffic", RecordedAt: time.Now(),
	}}})
	if code != http.StatusOK || !resp.Results[0].OK {
		t.Fatalf("delay: %d %+v", code, resp.Results)
	}
	if n := count("delay_reported"); n != 0 {
		t.Error("a reported delay was alerted with the rule off")
	}
	if got := scalar[string](t, `SELECT status FROM shipments WHERE id = $1`, id); got != "delayed" {
		t.Errorf("the delay itself should still be recorded: %q", got)
	}

	// Phone offline after the company's time.
	office.mustPost("/app/alerts/rules", rulesForm(map[string]string{"offline_minutes": "30"}))
	exec(t, `UPDATE sync_metadata SET last_sync_at = NOW() - INTERVAL '40 minutes' WHERE driver_id = $1`, driverID)
	exec(t, `UPDATE shipments SET status_changed_at = NOW() - INTERVAL '40 minutes', created_at = NOW() - INTERVAL '1 hour' WHERE id = $1`, id)
	if err := e.svc.Jobs.CheckDrivers(ctx); err != nil {
		t.Fatal(err)
	}
	if n := scalar[int64](t, `SELECT count(*) FROM alerts WHERE driver_id = $1 AND alert_type = 'driver_offline'`, driverID); n != 1 {
		t.Errorf("%d offline alerts after 40 minutes with the rule at 30, want 1", n)
	}
}

// TestLongStop: a truck standing still outside a town is alerted once per
// stop, and the alert resolves itself when the truck moves on.
func TestLongStop(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	office := e.company("Standstill Freight")
	driverID, phone, pin := office.addDriver("Hana Girma")
	id, _ := office.book("Addis Ababa", "Mekelle", driverID)
	d := e.signIn(phone, pin)
	if r := d.status(clientID(), id, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	exec(t, `UPDATE shipments SET picked_up_at = NOW() - INTERVAL '3 hours' WHERE id = $1`, id)
	ping := func(lat, lng float64, ago string) {
		exec(t, `INSERT INTO tracking_events (tenant_id, shipment_id, driver_id, coordinates, event_type, recorded_at)
			SELECT tenant_id, id, driver_id, ST_SetSRID(ST_MakePoint($2, $3), 4326), 'gps_ping', NOW() - $4::interval
			FROM shipments WHERE id = $1`, id, lng, lat, ago)
	}
	open := func() int64 {
		return scalar[int64](t, `SELECT count(*) FROM alerts WHERE shipment_id = $1 AND alert_type = 'long_stop' AND acknowledged_at IS NULL`, id)
	}
	// Moving, then an hour on one spot in the open country of Afar.
	ping(10.20, 39.95, "90 minutes")
	for m := 60; m >= 0; m -= 5 {
		ping(10.30, 40.00, (time.Duration(m) * time.Minute).String())
	}
	for range 2 {
		if err := e.svc.Jobs.CheckStops(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if n := open(); n != 1 {
		t.Fatalf("%d open long-stop alerts, want 1", n)
	}
	if msg := scalar[string](t, `SELECT message FROM alerts WHERE shipment_id = $1 AND alert_type = 'long_stop'`, id); !strings.Contains(msg, "stood still for an hour") {
		t.Errorf("message %q", msg)
	}

	// It moves on.
	ping(10.33, 40.00, "0s")
	if err := e.svc.Jobs.CheckStops(ctx); err != nil {
		t.Fatal(err)
	}
	if n := open(); n != 0 {
		t.Errorf("the alert stayed open after the truck moved: %d", n)
	}
	if p := office.get("/app/alerts?tab=resolved"); !strings.Contains(p.Body, "Resolved by itself") {
		t.Error("the alert should be listed as resolved by itself")
	}

	// With the rule off, a stop raises nothing.
	office.mustPost("/app/alerts/rules", rulesForm(map[string]string{"stop_on": ""}))
	exec(t, `DELETE FROM alerts WHERE shipment_id = $1`, id)
	exec(t, `DELETE FROM tracking_events WHERE shipment_id = $1`, id)
	for m := 60; m >= 0; m -= 5 {
		ping(10.30, 40.00, (time.Duration(m) * time.Minute).String())
	}
	if err := e.svc.Jobs.CheckStops(ctx); err != nil {
		t.Fatal(err)
	}
	if n := open(); n != 0 {
		t.Error("a long stop was alerted with the rule off")
	}
}

// TestResolveAndRecipients: an alert is resolved with a note of what was
// done, and an administrator chooses who also gets alerts by SMS.
func TestResolveAndRecipients(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	office := e.company("Notified Logistics")
	owner := scalar[string](t, `SELECT id::text FROM users WHERE phone = $1`, office.phone)
	driverID, phone, pin := office.addDriver("Abebe Kebede")
	id, _ := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)
	if r := d.status(clientID(), id, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}

	// Problems only, by SMS, to the owner.
	if p := office.get("/app/alerts/recipients"); !strings.Contains(p.Body, "Alerts by SMS for Owner of Notified Logistics") {
		t.Fatal("the recipients page should list the owner")
	}
	office.mustPost("/app/alerts/recipients", url.Values{"email_" + owner: {"none"}, "sms_" + owner: {"problems"}})
	if p := office.get("/app/alerts"); !strings.Contains(p.Body, "problems only by SMS") {
		t.Error("who gets them should say the owner gets problems by SMS")
	}
	texts := func() int64 {
		return scalar[int64](t, `SELECT count(*) FROM sms_messages WHERE phone = $1 AND purpose = 'alert'`, office.phone)
	}
	exec(t, `UPDATE shipments SET estimated_delivery = NOW() - INTERVAL '5 minutes' WHERE id = $1`, id)
	if err := e.svc.Jobs.CheckETAs(ctx); err != nil {
		t.Fatal(err)
	}
	if n := texts(); n != 0 {
		t.Errorf("a late load was texted to someone who wants problems only (%d)", n)
	}
	resp, code := d.sync(service.SyncRequest{Statuses: []service.SyncStatus{{
		ClientID: clientID(), ShipmentID: id, Status: "issue", Reason: "breakdown", Note: "Flat tyre", RecordedAt: time.Now(),
	}}})
	if code != http.StatusOK || !resp.Results[0].OK {
		t.Fatalf("problem: %d %+v", code, resp.Results)
	}
	if n := texts(); n != 1 {
		t.Errorf("%d problem alerts texted, want 1", n)
	}

	// Resolved with a note.
	alertID := scalar[string](t, `SELECT id::text FROM alerts WHERE shipment_id = $1 AND alert_type = 'issue_reported'`, id)
	other := e.company("Meddling Logistics")
	other.post("/app/alerts/"+alertID+"/ack", url.Values{"note": {"Not mine"}})
	office.mustPost("/app/alerts/"+alertID+"/ack", url.Values{"note": {"Tyre changed, back on the road"}})
	if got := scalar[string](t, `SELECT COALESCE(resolution_note, '') FROM alerts WHERE id = $1`, alertID); got != "Tyre changed, back on the road" {
		t.Errorf("note %q", got)
	}
	p := office.get("/app/alerts")
	if !strings.Contains(p.Body, "Resolved today") || !strings.Contains(p.Body, "Tyre changed, back on the road") || !strings.Contains(p.Body, "Owner of Notified Logistics") {
		t.Error("the alerts page should list the resolved alert with its note and who resolved it")
	}
	if p := office.get("/app/alerts?type=issue_reported&tab=all"); strings.Contains(p.Body, "is past its estimated delivery") {
		t.Error("filtering by type still lists other types")
	}
	if p := office.dispatcher().get("/app/alerts/recipients"); p.Status != http.StatusForbidden {
		t.Errorf("a dispatcher opening who gets alerts: status %d, want 403", p.Status)
	}
}
