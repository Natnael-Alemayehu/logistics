//go:build integration

package integration

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/geo"
	"github.com/natnael-alemayehu/logistics/internal/service"
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

// TestSyncReplayIsIdempotent: a phone that loses the response resends the
// same change, which must not be applied twice.
func TestSyncReplayIsIdempotent(t *testing.T) {
	e := newEnv(t)
	office := e.company("Replay Logistics")
	driverID, phone, pin := office.addDriver("Tigist Haile")
	shipmentID, _ := office.book("Addis Ababa", "Hawassa", driverID)
	d := e.signIn(phone, pin)

	id := clientID()
	for i := range 2 {
		if r := d.status(id, shipmentID, "in_transit"); !r.OK {
			t.Fatalf("attempt %d: %+v", i+1, r)
		}
	}
	if n := scalar[int64](t, `SELECT count(*) FROM shipment_events WHERE shipment_id = $1 AND to_status = 'in_transit'`, shipmentID); n != 1 {
		t.Errorf("in_transit recorded %d times, want 1", n)
	}
}

// TestOfficeDecisionWinsOverStaleDriverUpdate: once the office cancels, an
// update queued on a phone that was offline must not revive the shipment.
func TestOfficeDecisionWinsOverStaleDriverUpdate(t *testing.T) {
	e := newEnv(t)
	office := e.company("Conflict Haulage")
	driverID, phone, pin := office.addDriver("Mulugeta Alemu")
	shipmentID, _ := office.book("Addis Ababa", "Dire Dawa", driverID)
	d := e.signIn(phone, pin)

	tappedOffline := time.Now()
	time.Sleep(10 * time.Millisecond)
	office.mustPost("/app/shipments/"+shipmentID+"/cancel", url.Values{"reason": {"customer_request"}})

	// Made before the cancellation, sent after it.
	r := d.statusAt(clientID(), shipmentID, "in_transit", tappedOffline)
	if r.OK || r.Retry || r.Code != "STALE" {
		t.Errorf("queued update: %+v, want a final STALE rejection", r)
	}
	if n := scalar[int64](t, `SELECT count(*) FROM shipment_events WHERE shipment_id = $1 AND event_type = 'conflict'`, shipmentID); n != 1 {
		t.Errorf("%d conflict events recorded, want 1", n)
	}

	// Made after it: not a move a driver can make on a cancelled shipment.
	if r := d.status(clientID(), shipmentID, "in_transit"); r.OK || r.Retry {
		t.Errorf("update after cancellation: %+v, want a final rejection", r)
	}
	if got := scalar[string](t, `SELECT status FROM shipments WHERE id = $1`, shipmentID); got != "cancelled" {
		t.Errorf("status %q, want cancelled", got)
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
