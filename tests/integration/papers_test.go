//go:build integration

package integration

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"
)

func inDays(n int) string { return time.Now().AddDate(0, 0, n).Format("2006-01-02") }

// TestDriverLicence: a driver's licence date is kept, the office is reminded
// 30 days before and again once it has run out, a driver whose licence has
// run out can't be given a trip, and recording the renewal clears it all.
func TestDriverLicence(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	office := e.company("Licensed Haulage")
	driverID, phone, _ := office.addDriver("Birhanu Tadesse")
	edit := func(licence string) page {
		return office.post("/app/drivers/"+driverID+"/edit", url.Values{"full_name": {"Birhanu Tadesse"}, "phone": {phone}, "licence_expires_on": {licence}})
	}
	licence := func() string {
		return scalar[string](t, `SELECT COALESCE(licence_expires_on::text, '') FROM users WHERE id = $1`, driverID)
	}
	open := func() string {
		return scalar[string](t, `SELECT COALESCE(string_agg(severity, ','), '') FROM alerts WHERE driver_id = $1 AND alert_type = 'licence_due' AND acknowledged_at IS NULL`, driverID)
	}

	if edit("2026-02-31"); licence() != "" {
		t.Errorf("a day that doesn't exist was kept: %q", licence())
	}
	edit(inDays(10))
	if licence() != inDays(10) {
		t.Fatalf("licence %q, want %s", licence(), inDays(10))
	}
	if p := office.get("/app/drivers"); !strings.Contains(p.Body, "driving licence runs out on") {
		i := strings.Index(p.Body, "notice")
		t.Errorf("the drivers page should say the licence runs out soon: %d %.400s", p.Status, p.Body[max(i, 0):])
	}

	for range 2 {
		if err := e.svc.Jobs.CheckPapers(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := open(); got != "warning" {
		t.Errorf("open licence alerts %q, want one warning", got)
	}

	// The office texts them to renew it.
	office.mustPost("/app/drivers/"+driverID+"/licence-reminder", nil)
	if n := scalar[int64](t, `SELECT count(*) FROM sms_messages WHERE phone = $1 AND body LIKE '%your driving licence runs out%'`, phone); n != 1 {
		t.Errorf("%d reminders texted, want 1", n)
	}
	other := e.company("Prying Haulage")
	other.post("/app/drivers/"+driverID+"/licence-reminder", nil)
	if n := scalar[int64](t, `SELECT count(*) FROM sms_messages WHERE phone = $1 AND body LIKE '%licence%'`, phone); n != 1 {
		t.Error("another company texted the driver")
	}

	// It runs out: the reminder gives way to a critical alert, and the
	// driver can't be given a trip.
	exec(t, `UPDATE users SET licence_expires_on = CURRENT_DATE - 2 WHERE id = $1`, driverID)
	if err := e.svc.Jobs.CheckPapers(ctx); err != nil {
		t.Fatal(err)
	}
	if got := open(); got != "critical" {
		t.Errorf("open licence alerts %q, want one critical", got)
	}
	id, _ := office.book("Addis Ababa", "Adama", "")
	office.post("/app/shipments/"+id+"/assign", url.Values{"driver_id": {driverID}})
	if assigned := scalar[string](t, `SELECT COALESCE(driver_id::text, '') FROM shipments WHERE id = $1`, id); assigned != "" {
		t.Error("a driver whose licence ran out was given a trip")
	}
	if p := office.get("/app/shipments/" + id); !strings.Contains(p.Body, "driving licence ran out on") {
		t.Errorf("the refusal should say why: %s", errorText(p.Body))
	}

	// Renewed: the alert is resolved and the trip can be given.
	edit(inDays(365 * 5))
	if got := open(); got != "" {
		t.Errorf("open licence alerts after renewal: %q", got)
	}
	office.mustPost("/app/shipments/"+id+"/assign", url.Values{"driver_id": {driverID}})
	if assigned := scalar[string](t, `SELECT COALESCE(driver_id::text, '') FROM shipments WHERE id = $1`, id); assigned != driverID {
		t.Error("the renewed driver couldn't be given the trip")
	}
}

// TestVehiclePapers: a vehicle's insurance and inspection (ቦሎ) dates, and
// their reminders, which are about the vehicle and link to it.
func TestVehiclePapers(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	office := e.company("Inspected Freight")
	form := func(insurance, inspection string) url.Values {
		return url.Values{"plate_number": {"3-B21876 AA"}, "vehicle_type": {"light_truck"}, "capacity_kg": {"3500"}, "active": {"on"},
			"insurance_expires_on": {insurance}, "inspection_due_on": {inspection}}
	}
	office.mustPost("/app/vehicles", form(inDays(400), inDays(6)))
	vehicleID := scalar[string](t, `SELECT id::text FROM vehicles WHERE plate_number = '3-B21876 AA' AND tenant_id = $1`, office.tenant())
	if got := scalar[string](t, `SELECT insurance_expires_on::text || '|' || inspection_due_on::text FROM vehicles WHERE id = $1`, vehicleID); got != inDays(400)+"|"+inDays(6) {
		t.Fatalf("stored %q", got)
	}
	if p := office.get("/app/vehicles"); !strings.Contains(p.Body, "is due on") || !strings.Contains(p.Body, "Papers due in 30 days") {
		t.Error("the vehicles page should say the inspection is due")
	}

	if err := e.svc.Jobs.CheckPapers(ctx); err != nil {
		t.Fatal(err)
	}
	open := func() string {
		return scalar[string](t, `SELECT COALESCE(string_agg(alert_type || ':' || severity, ',' ORDER BY alert_type), '') FROM alerts WHERE vehicle_id = $1 AND acknowledged_at IS NULL`, vehicleID)
	}
	if got := open(); got != "inspection_due:warning" {
		t.Errorf("open alerts %q, want the inspection's", got)
	}
	if p := office.get("/app/alerts"); !strings.Contains(p.Body, "/app/vehicles/"+vehicleID) {
		t.Error("the alert should link to the vehicle")
	}

	// Insurance lapses too: each paper has its own alert.
	office.mustPost("/app/vehicles/"+vehicleID+"/edit", form(inDays(-1), inDays(6)))
	if err := e.svc.Jobs.CheckPapers(ctx); err != nil {
		t.Fatal(err)
	}
	if got := open(); got != "inspection_due:warning,insurance_due:critical" {
		t.Errorf("open alerts %q", got)
	}

	// Inspection done: only its alert is resolved.
	office.mustPost("/app/vehicles/"+vehicleID+"/edit", form(inDays(-1), inDays(365)))
	if got := open(); got != "insurance_due:critical" {
		t.Errorf("after the inspection: %q", got)
	}
	if p := office.post("/app/vehicles/"+vehicleID+"/edit", form("next year", inDays(365))); !strings.Contains(p.Body, "Enter the date the insurance runs out") {
		t.Errorf("a date that isn't one: %s", errorText(p.Body))
	}
}
