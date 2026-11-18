//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/geo"
	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// TestDeliveryLifecycle follows one load from booking to the customer's
// confirmation, through every interface that touches it.
func TestDeliveryLifecycle(t *testing.T) {
	e := newEnv(t)
	office := e.company("Lifecycle Freight")
	driverID, phone, pin := office.addDriver("Abebe Kebede")
	shipmentID, tracking := office.book("Addis Ababa", "Adama", driverID)

	if got := scalar[string](t, `SELECT status FROM shipments WHERE id = $1`, shipmentID); got != "assigned" {
		t.Fatalf("booked with a driver: status %q, want assigned", got)
	}

	d := e.signIn(phone, pin)
	resp, code := d.sync(service.SyncRequest{})
	if code != http.StatusOK || len(resp.Shipments) != 1 || resp.Shipments[0].ID != shipmentID {
		t.Fatalf("first sync: status %d, shipments %+v", code, resp.Shipments)
	}

	for _, st := range []string{"in_transit", "arrived"} {
		if r := d.status(clientID(), shipmentID, st); !r.OK {
			t.Fatalf("%s rejected: %+v", st, r)
		}
	}

	dest, _ := geo.Lookup("Adama")
	lat, lng := dest.Lat+0.001, dest.Lng
	resp, code = d.sync(service.SyncRequest{PODs: []service.SyncPOD{{
		ClientID: clientID(), ShipmentID: shipmentID, RecipientName: "Meseret Tadesse",
		Signature: signaturePNG, Lat: &lat, Lng: &lng, RecordedAt: time.Now(),
	}}})
	if code != http.StatusOK || len(resp.Results) != 1 || !resp.Results[0].OK {
		t.Fatalf("proof of delivery: status %d, results %+v", code, resp.Results)
	}
	if got := scalar[string](t, `SELECT status FROM shipments WHERE id = $1`, shipmentID); got != "delivered" {
		t.Fatalf("after proof of delivery: status %q", got)
	}
	if !scalar[bool](t, `SELECT location_verified FROM proof_of_deliveries WHERE shipment_id = $1`, shipmentID) {
		t.Error("a delivery 100 m from the destination should be location-verified")
	}

	// The office sees the proof, including the signature image.
	p := office.get("/app/shipments/" + shipmentID)
	if p.Status != http.StatusOK || !strings.Contains(p.Body, "Meseret Tadesse") {
		t.Fatalf("shipment page: status %d, recipient missing", p.Status)
	}
	if p := office.get("/app/shipments/" + shipmentID + "/media/0"); p.Status != http.StatusOK || !strings.HasPrefix(p.Body, "\x89PNG") {
		t.Errorf("signature image: status %d", p.Status)
	}

	// The public tracking page shows the outcome to anyone with the number.
	visitor := e.browser()
	if p := visitor.get("/track/" + tracking); p.Status != http.StatusOK || !strings.Contains(p.Body, "Delivered") {
		t.Errorf("tracking page: status %d", p.Status)
	}

	// The customer's delivery text carries a confirmation link.
	body := scalar[string](t, `SELECT body FROM sms_messages WHERE shipment_id = $1 AND body LIKE '%/c/%' ORDER BY created_at DESC LIMIT 1`, shipmentID)
	m := regexp.MustCompile(`/c/([A-Za-z0-9_-]+)`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no confirmation link in %q", body)
	}
	if p := visitor.get("/c/" + m[1]); p.Status != http.StatusOK {
		t.Fatalf("confirmation page: status %d", p.Status)
	}
	visitor.mustPost("/c/"+m[1], url.Values{"answer": {"ok"}})
	if !scalar[bool](t, `SELECT customer_confirmed_at IS NOT NULL FROM shipments WHERE id = $1`, shipmentID) {
		t.Error("customer confirmation was not recorded")
	}
}

// TestCompaniesAreIsolated: nothing of one company is reachable from another.
func TestCompaniesAreIsolated(t *testing.T) {
	e := newEnv(t)
	a := e.company("Alpha Transport")
	driverA, _, _ := a.addDriver("Hana Girma")
	shipmentA, _ := a.book("Addis Ababa", "Mekelle", driverA)

	b := e.company("Beta Transport")
	_, phoneB, pinB := b.addDriver("Yonas Tadesse")

	for _, path := range []string{"/app/shipments/" + shipmentA, "/app/shipments/" + shipmentA + "/edit", "/app/drivers/" + driverA} {
		if p := b.get(path); p.Status != http.StatusNotFound {
			t.Errorf("other company GET %s: status %d, want 404", path, p.Status)
		}
	}
	b.post("/app/shipments/"+shipmentA+"/cancel", url.Values{"reason": {"other"}})

	dB := e.signIn(phoneB, pinB)
	if r := dB.status(clientID(), shipmentA, "in_transit"); r.OK {
		t.Errorf("another company's driver moved the shipment: %+v", r)
	}
	resp, _ := dB.sync(service.SyncRequest{})
	if len(resp.Shipments) != 0 {
		t.Errorf("driver of another company sees %d shipments", len(resp.Shipments))
	}
	if got := scalar[string](t, `SELECT status FROM shipments WHERE id = $1`, shipmentA); got != "assigned" {
		t.Errorf("shipment changed to %q by another company", got)
	}
}

// TestDeactivatedDriverIsSignedOut: switching a driver off ends their
// sessions at once, not when the token expires.
func TestDeactivatedDriverIsSignedOut(t *testing.T) {
	e := newEnv(t)
	office := e.company("Offboarding Cargo")
	driverID, phone, pin := office.addDriver("Samuel Getachew")
	d := e.signIn(phone, pin)
	if _, code := d.sync(service.SyncRequest{}); code != http.StatusOK {
		t.Fatalf("sync before: %d", code)
	}
	office.mustPost("/app/drivers/"+driverID+"/active", url.Values{"active": {"false"}})
	if _, code := d.sync(service.SyncRequest{}); code != http.StatusUnauthorized {
		t.Errorf("sync after deactivation: status %d, want 401", code)
	}
	if code := e.api("POST", "/api/v1/driver/login", "", map[string]string{"phone": phone, "pin": "4826", "device_id": "x"}, nil); code == http.StatusOK {
		t.Error("a deactivated driver could sign in")
	}
}

// TestDriverSMSCommands: a driver without a smartphone reports by text.
func TestDriverSMSCommands(t *testing.T) {
	e := newEnv(t)
	office := e.company("Feature Phone Freight")
	driverID, phone, _ := office.addDriver("Kebede Alemu")
	shipmentID, tracking := office.book("Addis Ababa", "Jimma", driverID)
	ref := tracking[len(tracking)-4:]

	webhook := func(from, body string) {
		t.Helper()
		form := url.Values{"from": {from}, "body": {body}}
		res, err := http.PostForm(e.srv.URL+"/api/v1/sms/inbound", form)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("webhook %q: status %d", body, res.StatusCode)
		}
	}
	webhook(phone, "ET "+ref+" 1")
	if got := scalar[string](t, `SELECT status FROM shipments WHERE id = $1`, shipmentID); got != "in_transit" {
		t.Fatalf("after 'ET %s 1': status %q", ref, got)
	}
	webhook(phone, "et "+strings.ToLower(ref)+" 2 bridge closed at Gibe")
	if got := scalar[string](t, `SELECT status FROM shipments WHERE id = $1`, shipmentID); got != "delayed" {
		t.Errorf("after lower-case delay command: status %q", got)
	}
	if n := scalar[int64](t, `SELECT count(*) FROM sms_messages WHERE phone = $1 AND direction = 'outbound' AND body LIKE '%recorded%'`, phone); n != 2 {
		t.Errorf("%d confirmations texted back, want 2", n)
	}

	// A stranger's text changes nothing.
	webhook(phoneNumber(), "ET "+ref+" 3")
	if got := scalar[string](t, `SELECT status FROM shipments WHERE id = $1`, shipmentID); got != "delayed" {
		t.Errorf("unknown sender changed the status to %q", got)
	}

	// The provider's delivery report marks a reply delivered.
	var sent string
	for deadline := time.Now().Add(5 * time.Second); sent == "" && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		sent = scalar[string](t, `SELECT COALESCE(MIN(provider_message_id), '') FROM sms_messages WHERE phone = $1 AND direction = 'outbound' AND status = 'sent'`, phone)
	}
	if sent == "" {
		t.Fatal("no reply was sent with the provider's message id")
	}
	report := func(id, status string) bool {
		t.Helper()
		body, _ := json.Marshal(map[string]string{"message_id": id, "status": status, "done_at": time.Now().Format(time.RFC3339)})
		res, err := http.Post(e.srv.URL+"/api/v1/sms/status", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out struct{ Updated bool }
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil || res.StatusCode != http.StatusOK {
			t.Fatalf("delivery report %s %s: status %d, %v", id, status, res.StatusCode, err)
		}
		return out.Updated
	}
	if !report(sent, "DELIVRD") {
		t.Error("the delivery report changed nothing")
	}
	if got := scalar[string](t, `SELECT COALESCE(delivery, '') FROM sms_messages WHERE provider_message_id = $1`, sent); got != "delivered" {
		t.Errorf("after the delivery report the text is %q", got)
	}
	// Delivered is final, and a report on a text the platform didn't send
	// is answered all the same.
	if report(sent, "failed") || report("NO-SUCH-TEXT", "delivered") {
		t.Error("a late failure, or a report on an unknown text, changed a text")
	}
	if p := office.get("/app/settings/sms"); !strings.Contains(p.Body, "Delivered") {
		t.Error("the SMS log doesn't show the text delivered")
	}
}

// TestPublicTrackingIsMinimal: the tracking page is for anyone with the
// number, so it must not expose people's phone numbers.
func TestPublicTrackingIsMinimal(t *testing.T) {
	e := newEnv(t)
	office := e.company("Privacy Movers")
	driverID, driverPhone, _ := office.addDriver("Rahel Mekonnen")
	_, tracking := office.book("Addis Ababa", "Bahir Dar", driverID)
	customerPhone := scalar[string](t, `SELECT customer_phone FROM shipments WHERE tracking_number = $1`, tracking)

	p := e.browser().get("/track/" + tracking)
	if p.Status != http.StatusOK {
		t.Fatalf("tracking page: status %d", p.Status)
	}
	for _, secret := range []string{driverPhone, customerPhone, "Rahel"} {
		if strings.Contains(p.Body, secret) {
			t.Errorf("tracking page shows %q", secret)
		}
	}
	if p := e.browser().get("/track/ET-20260101-ZZZZZZZZZZ"); p.Status != http.StatusNotFound {
		t.Errorf("unknown tracking number: status %d, want 404", p.Status)
	}
}

// TestStaffRoles: a dispatcher runs shipments but not the company.
func TestStaffRoles(t *testing.T) {
	e := newEnv(t)
	admin := e.company("Role Check Logistics")
	email := "dispatch" + phoneNumber() + "@example.et"
	p := admin.post("/app/team", url.Values{"full_name": {"Dawit Bekele"}, "email": {email}, "phone": {phoneNumber()}, "role": {"dispatcher"}})
	m := secretRe.FindStringSubmatch(p.Body)
	if m == nil {
		t.Fatalf("add dispatcher: status %d %s", p.Status, errorText(p.Body))
	}
	// The new member is told how to sign in, and asked to confirm the address.
	if body := e.received("email", email, "/verify-email/").Body; !strings.Contains(body, "Owner of Role Check Logistics added you") {
		t.Errorf("the invitation email doesn't say who added them:\n%s", body)
	}

	disp := e.browser()
	disp.mustPost("/login", url.Values{"identifier": {email}, "password": {strings.TrimSpace(m[1])}})
	if p := disp.get("/app/shipments"); p.Status != http.StatusSeeOther || !strings.Contains(p.Location, "/account/password") {
		t.Fatalf("temporary password: GET /app/shipments = %d %s, want redirect to change it", p.Status, p.Location)
	}
	disp.mustPost("/app/account/password", url.Values{
		"current_password": {strings.TrimSpace(m[1])}, "new_password": {"dispatch-pass-9"}, "confirm_password": {"dispatch-pass-9"},
	})
	for path, want := range map[string]int{
		"/app/shipments": http.StatusOK, "/app/shipments/new": http.StatusOK,
		"/app/team": http.StatusForbidden, "/app/settings": http.StatusForbidden, "/app/audit": http.StatusForbidden,
		"/app/platform": http.StatusForbidden,
	} {
		if p := disp.get(path); p.Status != want {
			t.Errorf("dispatcher GET %s: status %d, want %d", path, p.Status, want)
		}
	}
}

// TestTrackingPageDetails: on the road, the tracking page says how far the
// truck has still to go, when each stage was reached, and how to call the
// carrier; the steps not yet reached have no time.
func TestTrackingPageDetails(t *testing.T) {
	e := newEnv(t)
	office := e.company("Detail Haulage")
	driverID, phone, pin := office.addDriver("Bekele Tadesse")
	id, tracking := office.book("Addis Ababa", "Hawassa", driverID)
	d := e.signIn(phone, pin)
	if r := d.status(clientID(), id, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	// Last seen near Batu, about 160 km by road from Hawassa.
	exec(t, `INSERT INTO tracking_events (tenant_id, shipment_id, driver_id, coordinates, event_type, recorded_at)
		SELECT tenant_id, id, driver_id, ST_SetSRID(ST_MakePoint(38.72, 7.93), 4326), 'gps_ping', NOW() - INTERVAL '5 minutes'
		FROM shipments WHERE id = $1`, id)

	p := e.browser().get("/track/" + tracking)
	m := regexp.MustCompile(`Distance left</dt><dd>(\d+) km`).FindStringSubmatch(p.Body)
	if m == nil {
		t.Fatal("the page should say how far is left")
	}
	if km, _ := strconv.Atoi(m[1]); km < 60 || km > 200 {
		t.Errorf("%d km left from Batu to Hawassa", km)
	}
	_, steps, _ := strings.Cut(p.Body, `<ol class="progress"`)
	steps, _, _ = strings.Cut(steps, "</ol>")
	if n := strings.Count(steps, "<small"); n != 3 {
		t.Errorf("%d steps have a time, want 3 (booked, preparing, on the way)", n)
	}
	for _, want := range []string{"Detail Haulage answers for this shipment.", `href="tel:`, "Tracked with Kiflet"} {
		if !strings.Contains(p.Body, want) {
			t.Errorf("the tracking page lacks %q", want)
		}
	}
}
