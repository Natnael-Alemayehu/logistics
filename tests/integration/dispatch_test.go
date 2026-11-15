//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// TestBulkAssign: ticked shipments go to one driver, each on its own, so
// the driver's load limit stops some without undoing the rest, and another
// company's shipment can't be slipped in.
func TestBulkAssign(t *testing.T) {
	e := newEnv(t)
	office := e.company("Bulk Haulage")
	driverID, _, _ := office.addDriver("Mulu Tadesse")
	var ids []string
	for range 7 {
		id, _ := office.book("Addis Ababa", "Adama", "")
		ids = append(ids, id)
	}
	theirs, _ := e.company("Rival Movers").book("Addis Ababa", "Hawassa", "")

	list := office.get("/app/shipments?tab=pending").Body
	if !strings.Contains(list, `form="bulk-assign"`) || !strings.Contains(list, `value="`+ids[0]+`"`) {
		t.Fatal("pending rows should have a tick box for the assign form")
	}

	p := office.mustPost("/app/shipments/assign", url.Values{
		"id": append(append([]string{}, ids...), theirs), "driver_id": {driverID}, "back": {"/app/shipments?tab=pending"},
	})
	if p.Location != "/app/shipments?tab=pending" {
		t.Fatalf("redirected to %q", p.Location)
	}
	// The company allows five active loads a driver.
	if n := scalar[int64](t, `SELECT count(*) FROM shipments WHERE driver_id = $1 AND status = 'assigned'`, driverID); n != 5 {
		t.Errorf("%d shipments assigned, want 5: the driver's limit", n)
	}
	if st := scalar[string](t, `SELECT status FROM shipments WHERE id = $1`, theirs); st != "pending" {
		t.Errorf("another company's shipment is %s, want pending", st)
	}
	flash := office.get(p.Location).Body
	for _, want := range []string{"Assigned 5 shipments.", "Not assigned:", "One shipment:"} {
		if !strings.Contains(flash, want) {
			t.Errorf("the result should say %q:\n%s", want, errorText(flash))
		}
	}

	many := make([]string, 51)
	for i := range many {
		many[i] = ids[6]
	}
	p = office.mustPost("/app/shipments/assign", url.Values{"id": many, "driver_id": {driverID}})
	if body := office.get(p.Location).Body; !strings.Contains(body, "Assign at most 50 shipments at a time.") {
		t.Errorf("51 shipments at once should be refused: %s", errorText(body))
	}
}

// TestSavedViews: a view keeps the list's own filters under a name, the
// same name replaces it, and only its owner can remove it.
func TestSavedViews(t *testing.T) {
	e := newEnv(t)
	office := e.company("View Freight")
	p := office.mustPost("/app/shipments/views", url.Values{
		"name": {"  Cement   waiting "}, "query": {"tab=pending&q=cement&page=3&highlight=x&format=csv"},
	})
	if p.Location != "/app/shipments?q=cement&tab=pending" {
		t.Errorf("redirected to %q, want the view itself", p.Location)
	}
	owner := scalar[string](t, `SELECT id::text FROM users WHERE email = $1`, office.email)
	if q := scalar[string](t, `SELECT query FROM saved_filters WHERE user_id = $1 AND name = 'Cement waiting'`, owner); q != "q=cement&tab=pending" {
		t.Errorf("stored query %q, want the filter without page, highlight or format", q)
	}
	if list := office.get("/app/shipments").Body; !strings.Contains(list, ">Cement waiting</a>") {
		t.Error("the view should show above the list")
	}

	office.mustPost("/app/shipments/views", url.Values{"name": {"Cement waiting"}, "query": {"tab=attention"}})
	if n := scalar[int64](t, `SELECT count(*) FROM saved_filters WHERE user_id = $1`, owner); n != 1 {
		t.Errorf("%d views after saving under the same name, want 1", n)
	}
	p = office.mustPost("/app/shipments/views", url.Values{"name": {"Everything"}, "query": {"page=2"}})
	if body := office.get(p.Location).Body; !strings.Contains(body, "Choose a filter or a tab first") {
		t.Errorf("a view of every shipment should be refused: %s", errorText(body))
	}
	for i := range 19 {
		office.mustPost("/app/shipments/views", url.Values{"name": {"View " + string(rune('A'+i))}, "query": {"tab=active"}})
	}
	p = office.mustPost("/app/shipments/views", url.Values{"name": {"One too many"}, "query": {"tab=active"}})
	if body := office.get(p.Location).Body; !strings.Contains(body, "You have 20 saved views") {
		t.Errorf("a 21st view should be refused: %s", errorText(body))
	}

	viewID := scalar[string](t, `SELECT id::text FROM saved_filters WHERE user_id = $1 AND name = 'Cement waiting'`, owner)
	e.company("Nosy Neighbours").mustPost("/app/shipments/views/"+viewID+"/delete", nil)
	if n := scalar[int64](t, `SELECT count(*) FROM saved_filters WHERE id = $1`, viewID); n != 1 {
		t.Fatal("another company removed the view")
	}
	office.mustPost("/app/shipments/views/"+viewID+"/delete", nil)
	if n := scalar[int64](t, `SELECT count(*) FROM saved_filters WHERE id = $1`, viewID); n != 0 {
		t.Error("the owner couldn't remove the view")
	}
}

// TestMessageEveryDriverOnTheRoad: one message reaches every driver with a
// shipment assigned or under way; the app shows it, and a driver whose app
// hasn't synced lately gets it by SMS too.
func TestMessageEveryDriverOnTheRoad(t *testing.T) {
	e := newEnv(t)
	office := e.company("Broadcast Trucking")
	onApp, phoneA, pinA := office.addDriver("Abebe Kebede")
	noApp, phoneB, _ := office.addDriver("Bekele Tola")
	_, phoneC, _ := office.addDriver("Chaltu Haile") // nothing to drive
	office.book("Addis Ababa", "Adama", onApp)
	office.book("Addis Ababa", "Dire Dawa", noApp)
	d := e.signIn(phoneA, pinA)
	if _, code := d.sync(service.SyncRequest{}); code != http.StatusOK {
		t.Fatalf("sync: %d", code)
	}

	// The driver with the app open is on duty; the others aren't.
	if home := office.get("/app").Body; !strings.Contains(home, `1<span class="kpi-of"> of 3</span>`) || !strings.Contains(home, `id="map-meta"`) {
		t.Error("the overview should count one of three drivers on duty")
	}

	p := office.mustPost("/app/drivers/message", url.Values{"body": {"Road closed at Mojo. Wait for instructions."}})
	page := office.get(p.Location).Body
	if !strings.Contains(page, "Sent to 2 drivers.") || !strings.Contains(page, "One of them hasn") {
		t.Errorf("unexpected result: %s", errorText(page))
	}
	if !strings.Contains(page, "Road closed at Mojo.") {
		t.Error("the message should be listed as sent")
	}
	e.received("sms", phoneB, "to all drivers: Road closed at Mojo")

	resp, _ := d.sync(service.SyncRequest{})
	shown := false
	for _, m := range resp.Messages {
		shown = shown || (m.Text == "Road closed at Mojo. Wait for instructions." && m.From != "")
	}
	if !shown {
		t.Errorf("the driver app should show the message: %+v", resp.Messages)
	}
	for _, it := range e.inbox.Items() {
		if it.Kind == "sms" && (it.To == phoneA || it.To == phoneC) && strings.Contains(it.Body, "Road closed") {
			t.Errorf("SMS to %s, who has the app open or nothing to drive", it.To)
		}
	}
	if n := scalar[int64](t, `SELECT count(*) FROM broadcast_recipients r JOIN broadcasts b ON b.id = r.broadcast_id
		WHERE b.body LIKE 'Road closed at Mojo%' AND r.by_sms = (r.driver_id = $1)`, noApp); n != 2 {
		t.Errorf("%d recipients recorded as expected, want 2", n)
	}

	if p := office.post("/app/drivers/message", url.Values{"body": {"   "}}); p.Status == http.StatusSeeOther || !strings.Contains(p.Body, "Write a message.") {
		t.Errorf("an empty message should be refused: %d %s", p.Status, errorText(p.Body))
	}
}

// TestDeliveredAlertCanBeTurnedOff: a delivery shows as an info alert unless
// the company turns that off in Settings.
func TestDeliveredAlertCanBeTurnedOff(t *testing.T) {
	e := newEnv(t)
	office := e.company("Delivered Alerts")
	driverID, phone, pin := office.addDriver("Hiwot Assefa")
	first, _ := office.book("Addis Ababa", "Adama", driverID)
	second, _ := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)
	deliver := func(id string) {
		t.Helper()
		if r := d.status(clientID(), id, "in_transit"); !r.OK {
			t.Fatalf("depart: %+v", r)
		}
		if r := d.pod(service.SyncPOD{ShipmentID: id, RecipientName: "Store keeper"}); !r.OK {
			t.Fatalf("proof of delivery: %+v", r)
		}
	}
	deliver(first)
	if n := scalar[int64](t, `SELECT count(*) FROM alerts WHERE shipment_id = $1 AND alert_type = 'delivered' AND severity = 'info'`, first); n != 1 {
		t.Fatalf("%d delivered alerts, want 1", n)
	}

	office.mustPost("/app/settings", url.Values{
		"name": {"Delivered Alerts"}, "region_city": {"Addis Ababa"}, "max_active_per_driver": {"5"},
		"contact_phone": {office.phone}, "contact_email": {office.email}, "sms_on_delivered": {"1"},
	})
	if scalar[bool](t, `SELECT alert_on_delivery FROM tenants WHERE id = (SELECT tenant_id FROM shipments WHERE id = $1)`, first) {
		t.Fatal("unticking the setting should turn delivery alerts off")
	}
	deliver(second)
	if n := scalar[int64](t, `SELECT count(*) FROM alerts WHERE shipment_id = $1 AND alert_type = 'delivered'`, second); n != 0 {
		t.Errorf("%d delivered alerts with the setting off, want 0", n)
	}
}

// TestAlertEmails: staff choose which alerts reach them by email; only a
// confirmed address gets them.
func TestAlertEmails(t *testing.T) {
	e := newEnv(t)
	office := e.company("Email Alerts")
	office.mustPost("/app/account/alerts", url.Values{"alert": {"issue_reported", "not_a_type"}})
	if got := scalar[string](t, `SELECT array_to_string(alert_emails, ',') FROM users WHERE email = $1`, office.email); got != "issue_reported" {
		t.Errorf("alert emails %q, want only the known type", got)
	}
	driverID, phone, pin := office.addDriver("Getachew Mamo")
	shipmentID, tracking := office.book("Addis Ababa", "Gondar", driverID)
	d := e.signIn(phone, pin)
	report := func(status, reason string) {
		t.Helper()
		resp, code := d.sync(service.SyncRequest{Statuses: []service.SyncStatus{{
			ClientID: clientID(), ShipmentID: shipmentID, Status: status, Reason: reason, RecordedAt: time.Now(),
		}}})
		if code != http.StatusOK || len(resp.Results) != 1 || !resp.Results[0].OK {
			t.Fatalf("report %s: %d %+v", status, code, resp.Results)
		}
	}
	report("in_transit", "")
	report("issue", "breakdown")
	time.Sleep(300 * time.Millisecond)
	for _, it := range e.inbox.Items() {
		if it.Kind == "email" && it.To == office.email && strings.Contains(it.Title, tracking) {
			t.Fatal("an alert was emailed to an address that isn't confirmed")
		}
	}

	office.get(verifyLinkRe.FindString(e.received("email", office.email, "/verify-email/").Body))
	report("delayed", "traffic")
	report("issue", "accident")
	// The accident is news though the breakdown's alert was still open.
	if n := scalar[int64](t, `SELECT count(*) FROM alerts WHERE shipment_id = $1 AND alert_type = 'issue_reported'`, shipmentID); n != 2 {
		t.Errorf("%d problem alerts, want 2", n)
	}
	if n := scalar[int64](t, `SELECT count(*) FROM alerts WHERE shipment_id = $1 AND alert_type = 'issue_reported' AND acknowledged_at IS NULL`, shipmentID); n != 1 {
		t.Errorf("%d open problem alerts, want only the newest", n)
	}
	mail := e.received("email", office.email, tracking)
	if !strings.HasPrefix(mail.Title, "Issue reported") || !strings.Contains(mail.Body, "/app/shipments/"+shipmentID) {
		t.Errorf("alert email %q should name the alert and link to the shipment:\n%s", mail.Title, mail.Body)
	}
	for _, it := range e.inbox.Items() {
		if it.Kind == "email" && it.To == office.email && strings.HasPrefix(it.Title, "Delay") {
			t.Errorf("a delay was emailed, though only problems were chosen: %q", it.Title)
		}
	}
}

// TestHeadingAwayFromTheDestination: a truck getting further from the
// drop-off over an hour raises one alert, and the trip's positions export
// as CSV for the company alone.
func TestHeadingAwayFromTheDestination(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	office := e.company("Deviation Lines")
	driverID, phone, pin := office.addDriver("Kassa Worku")
	shipmentID, _ := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)
	now := time.Now()
	if r := d.status(clientID(), shipmentID, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	// The truck left two hours ago.
	exec(t, `UPDATE shipments SET picked_up_at = NOW() - INTERVAL '2 hours' WHERE id = $1`, shipmentID)
	ping := func(lat, lng float64, at time.Time) {
		t.Helper()
		if _, code := d.sync(service.SyncRequest{Locations: []service.SyncLocation{{
			ClientID: clientID(), ShipmentID: shipmentID, Lat: lat, Lng: lng, RecordedAt: at,
		}}}); code != http.StatusOK {
			t.Fatalf("sync position: %d", code)
		}
	}
	// An hour ago near Mojo, 17 km from Adama; now back in Addis Ababa, 75 km away.
	ping(8.5833, 39.1167, now.Add(-60*time.Minute))
	ping(9.03, 38.74, now.Add(-5*time.Minute))
	for range 2 {
		if err := e.svc.Jobs.CheckRoutes(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if n := scalar[int64](t, `SELECT count(*) FROM alerts WHERE shipment_id = $1 AND alert_type = 'route_deviation'`, shipmentID); n != 1 {
		t.Fatalf("%d route alerts, want 1", n)
	}
	if msg := scalar[string](t, `SELECT message FROM alerts WHERE shipment_id = $1 AND alert_type = 'route_deviation'`, shipmentID); !strings.Contains(msg, "moving away from Adama") {
		t.Errorf("alert %q should say the truck is moving away from Adama", msg)
	}

	p := office.get("/app/shipments/" + shipmentID + "/locations.csv")
	if p.Status != http.StatusOK || !strings.Contains(p.Body, "Recorded,Latitude,Longitude") || !strings.Contains(p.Body, "9.030000,38.740000") {
		t.Errorf("locations CSV: %d\n%s", p.Status, p.Body)
	}
	if q := e.company("Snoop Freight").get("/app/shipments/" + shipmentID + "/locations.csv"); q.Status != http.StatusNotFound {
		t.Errorf("another company got the trip's positions: %d", q.Status)
	}
}

// TestALongTripEndsWhereTheTruckIs: the shipment page draws a long trip's
// newest positions, so the line ends at the truck, and the CSV still has
// every one (AC-030.7).
func TestALongTripEndsWhereTheTruckIs(t *testing.T) {
	e := newEnv(t)
	office := e.company("Long Haul Logistics")
	driverID, phone, pin := office.addDriver("Tesfaye Mamo")
	shipmentID, _ := office.book("Addis Ababa", "Mekelle", driverID)
	d := e.signIn(phone, pin)
	if r := d.status(clientID(), shipmentID, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	exec(t, `UPDATE shipments SET picked_up_at = NOW() - INTERVAL '3 days' WHERE id = $1`, shipmentID)
	// 2,500 positions a minute apart heading north; the newest is at 9.25 N.
	exec(t, `INSERT INTO tracking_events (tenant_id, shipment_id, driver_id, coordinates, event_type, recorded_at)
		SELECT s.tenant_id, s.id, s.driver_id, ST_SetSRID(ST_MakePoint(38.74, 9.0 + i * 0.0001), 4326), 'gps_ping',
		       NOW() - (2500 - i) * INTERVAL '1 minute'
		FROM shipments s, generate_series(1, 2500) i WHERE s.id = $1`, shipmentID)

	p := office.get("/app/shipments/" + shipmentID)
	if !strings.Contains(p.Body, "the newest 2,000 positions") || !strings.Contains(p.Body, "The newest 200 are listed here.") {
		t.Error("the page should say it shows only the newest positions")
	}
	_, rest, _ := strings.Cut(p.Body, `id="shipment-map">`)
	raw, _, _ := strings.Cut(rest, "</script>")
	var m struct{ Route [][]float64 }
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("map data %q: %v", raw, err)
	}
	if len(m.Route) != 2000 || m.Route[len(m.Route)-1][0] != 9.25 {
		t.Errorf("the map has %d positions ending at %v, want 2000 ending at 9.25 N", len(m.Route), m.Route[len(m.Route)-1])
	}
	if n := strings.Count(p.Body, "<tr><td>"); n > 210 {
		t.Errorf("the page lists %d positions", n)
	}

	csv := office.get("/app/shipments/" + shipmentID + "/locations.csv")
	if rows := strings.Count(csv.Body, "\n") - 1; rows < 2500 {
		t.Errorf("the CSV has %d positions, want all 2500", rows)
	}
}

// TestNewDestinationTextsTheDriver: moving an assigned load's drop-off
// texts the driver, who may be driving with the app closed, and shows in
// the app.
func TestNewDestinationTextsTheDriver(t *testing.T) {
	e := newEnv(t)
	office := e.company("Redirect Cargo")
	driverID, phone, pin := office.addDriver("Lemlem Desta")
	shipmentID, tracking := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)
	customerPhone := scalar[string](t, `SELECT customer_phone FROM shipments WHERE id = $1`, shipmentID)
	office.mustPost("/app/shipments/"+shipmentID+"/edit", url.Values{
		"priority": {"normal"}, "origin_city": {"Addis Ababa"}, "origin_address": {"Main depot"},
		"destination_city": {"Hawassa"}, "destination_address": {"Customer warehouse"},
		"customer_name": {"Abyssinia Traders"}, "customer_phone": {customerPhone},
		"cargo_description": {"Cement, 200 bags"}, "cargo_weight": {"10000"},
	})
	sms := e.received("sms", phone, "now goes to Hawassa")
	if !strings.Contains(sms.Body, service.ShortRef(tracking)) {
		t.Errorf("SMS %q should name the shipment", sms.Body)
	}
	resp, _ := d.sync(service.SyncRequest{})
	shown := false
	for _, m := range resp.Messages {
		shown = shown || strings.Contains(m.Text, "destination")
	}
	if !shown {
		t.Errorf("the app should say the destination changed: %+v", resp.Messages)
	}
}

// TestReportFilters: every report narrows to one driver or one route, a
// town found under another name too, and the chart's CSV follows its
// grouping.
func TestReportFilters(t *testing.T) {
	e := newEnv(t)
	office := e.company("Report Filters")
	a, _, _ := office.addDriver("Alemitu Bekele")
	b, _, _ := office.addDriver("Birhanu Tesfaye")
	s1, _ := office.book("Adama", "Addis Ababa", a)
	s2, _ := office.book("Addis Ababa", "Hawassa", a)
	s3, _ := office.book("Adama", "Addis Ababa", b)
	exec(t, `UPDATE shipments SET status = 'delivered', picked_up_at = NOW() - INTERVAL '5 hours',
		actual_delivery = NOW() - INTERVAL '1 hour' WHERE id = ANY($1::uuid[])`, []string{s1, s2, s3})
	report := func(query string) string {
		t.Helper()
		p := office.get("/app/reports?" + query)
		if p.Status != http.StatusOK {
			t.Fatalf("report %s: %d", query, p.Status)
		}
		return p.Body
	}

	routes := report("format=csv&section=routes&driver=" + a)
	if !strings.Contains(routes, "Adama,Addis Ababa,1,") || !strings.Contains(routes, "Addis Ababa,Hawassa,1,") {
		t.Errorf("one driver's routes:\n%s", routes)
	}
	// Nazret is Adama.
	drivers := report("format=csv&section=drivers&origin=nazret")
	if !strings.Contains(drivers, "Alemitu Bekele,1,") || !strings.Contains(drivers, "Birhanu Tesfaye,1,") {
		t.Errorf("drivers from Adama:\n%s", drivers)
	}
	drivers = report("format=csv&section=drivers&destination=hawassa")
	if !strings.Contains(drivers, "Alemitu Bekele,1,") || strings.Contains(drivers, "Birhanu") {
		t.Errorf("drivers to Hawassa:\n%s", drivers)
	}
	month := scalar[string](t, `SELECT to_char((NOW() - INTERVAL '1 hour') AT TIME ZONE 'Africa/Addis_Ababa', 'YYYY-MM')`)
	monthly := strings.TrimPrefix(report("format=csv&section=daily&group=month"), "\xEF\xBB\xBF")
	if !strings.HasPrefix(monthly, "Month,Delivered,On time") || !strings.Contains(monthly, month+",3,") {
		t.Errorf("deliveries per month:\n%s", monthly)
	}

	page := report("driver=" + a + "&origin=Adama&group=week")
	if !strings.Contains(page, "Deliveries per week") || !strings.Contains(page, "Alemitu Bekele · Adama → anywhere") {
		t.Error("the page should say what it covers")
	}
	if !strings.Contains(page, "driver="+a) || !strings.Contains(page, "origin=Adama&amp;section=routes") {
		t.Error("the CSV links should keep the filters")
	}
	// Another company's driver filters nothing.
	theirs, _, _ := e.company("Other Reports").addDriver("Someone Else")
	if all := report("format=csv&section=drivers&driver=" + theirs); !strings.Contains(all, "Alemitu Bekele,2,") || !strings.Contains(all, "Birhanu Tesfaye,1,") {
		t.Errorf("a driver from another company should be ignored:\n%s", all)
	}
}

// TestSearchByPhoneInAnyForm: a customer's number finds their shipments
// however it is typed, and the match is marked.
func TestSearchByPhoneInAnyForm(t *testing.T) {
	e := newEnv(t)
	office := e.company("Phone Search")
	id, tracking := office.book("Addis Ababa", "Adama", "")
	local := scalar[string](t, `SELECT customer_phone FROM shipments WHERE id = $1`, id)
	intl := "+251 " + local[1:4] + " " + local[4:7] + " " + local[7:]
	body := office.get("/app/shipments?q=" + url.QueryEscape(intl)).Body
	if !strings.Contains(body, tracking) {
		t.Fatalf("searching %q didn't find the shipment", intl)
	}
	if !strings.Contains(body, "<mark>"+local+"</mark>") {
		t.Error("the matching phone should be marked")
	}
	if other := office.get("/app/shipments?q=" + url.QueryEscape("+251 900 000 000")).Body; strings.Contains(other, tracking) {
		t.Error("another number found the shipment")
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
