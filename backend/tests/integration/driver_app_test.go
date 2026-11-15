//go:build integration

package integration

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// TestDriverSettingsFollowTheDriver: what the driver sets on one phone is
// there on the next, and a phone sends only what was changed on it, so the
// rest stay (AC-026.6). The office sees how the phone is set up.
func TestDriverSettingsFollowTheDriver(t *testing.T) {
	e := newEnv(t)
	office := e.company("Settings Transport")
	driverID, phone, pin := office.addDriver("Hirut Bekele")
	first := e.signIn(phone, pin)

	resp, _ := first.sync(service.SyncRequest{})
	if got := resp.Driver.Settings; got.Language != "" || got.BatterySaver || !got.Vibrate {
		t.Fatalf("a new driver's settings %+v, want no language yet, battery saver off, vibrate on", got)
	}
	am, on, off, fr := "am", true, false, "fr"
	resp, _ = first.sync(service.SyncRequest{Settings: &service.SettingsChange{Language: &am, BatterySaver: &on}})
	if got := resp.Driver.Settings; got.Language != "am" || !got.BatterySaver || !got.Vibrate {
		t.Fatalf("after choosing Amharic and battery saver: %+v", got)
	}

	// A new phone: only vibrate changes there. A language the server
	// doesn't know is ignored.
	var login struct {
		Token string `json:"token"`
	}
	if code := e.api("POST", "/api/v1/driver/login", "", map[string]string{"phone": phone, "pin": "4826", "device_id": "second-phone"}, &login); code != http.StatusOK {
		t.Fatalf("sign in on the second phone: %d", code)
	}
	second := &driverApp{e: e, token: login.Token, device: "second-phone"}
	resp, _ = second.sync(service.SyncRequest{})
	if got := resp.Driver.Settings; got.Language != "am" || !got.BatterySaver {
		t.Fatalf("the new phone got %+v, want the first phone's choices", got)
	}
	resp, _ = second.sync(service.SyncRequest{Settings: &service.SettingsChange{Vibrate: &off, Language: &fr}})
	if got := resp.Driver.Settings; got.Language != "am" || !got.BatterySaver || got.Vibrate {
		t.Fatalf("after turning vibrate off: %+v, want the others unchanged", got)
	}

	kb := 48 * 1024
	if _, code := second.sync(service.SyncRequest{PowerSaving: true, StorageKB: &kb}); code != http.StatusOK {
		t.Fatalf("sync: %d", code)
	}
	page := office.get("/app/drivers/" + driverID)
	for _, want := range []string{"Saving battery: a position every 15 minutes", "Amharic", "48 MB"} {
		if !strings.Contains(page.Body, want) {
			t.Errorf("the driver page should show %q", want)
		}
	}
}

// TestDeliveryAwayFromTheDestination: the driver says why they delivered
// far from the drop-off, and the office sees it with the alert (AC-022.2).
func TestDeliveryAwayFromTheDestination(t *testing.T) {
	e := newEnv(t)
	office := e.company("Elsewhere Freight")
	driverID, phone, pin := office.addDriver("Dawit Mekonnen")
	shipmentID, _ := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)
	resp, _ := d.sync(service.SyncRequest{})
	if len(resp.Shipments) != 1 || resp.Shipments[0].ToleranceM != 15000 {
		t.Fatalf("shipments %+v, want one with the city-wide tolerance", resp.Shipments)
	}
	if r := d.status(clientID(), shipmentID, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	// In Addis Ababa, some 75 km from Adama.
	lat, lng := 9.03, 38.74
	if r := d.pod(service.SyncPOD{ShipmentID: shipmentID, RecipientName: "Tigist Alemu", Lat: &lat, Lng: &lng, LocationReason: "met_elsewhere"}); !r.OK {
		t.Fatalf("POD: %+v", r)
	}
	if got := scalar[string](t, `SELECT location_reason || ' ' || location_verified::text FROM proof_of_deliveries WHERE shipment_id = $1`, shipmentID); got != "met_elsewhere false" {
		t.Errorf("stored %q, want the reason on an unverified location", got)
	}
	msg := scalar[string](t, `SELECT message FROM alerts WHERE shipment_id = $1 AND alert_type = 'pod_location_mismatch'`, shipmentID)
	if !strings.Contains(msg, "The driver's reason: Recipient met the truck elsewhere.") {
		t.Errorf("alert %q should give the driver's reason", msg)
	}
	page := office.get("/app/shipments/" + shipmentID)
	if !strings.Contains(page.Body, "Away from the destination because") || !strings.Contains(page.Body, "Recipient met the truck elsewhere") {
		t.Error("the shipment page should show why the delivery was away from the destination")
	}
}

// TestRecentRecipientsAreSuggested: the app suggests who signed for the
// same customer before, from this company only (AC-020.4).
func TestRecentRecipientsAreSuggested(t *testing.T) {
	e := newEnv(t)
	office := e.company("Regular Routes")
	driverID, phone, pin := office.addDriver("Meron Tadesse")
	customer := phoneNumber()
	firstID, _ := office.bookFor("Addis Ababa", "Adama", driverID, customer)
	d := e.signIn(phone, pin)
	if r := d.status(clientID(), firstID, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	if r := d.pod(service.SyncPOD{ShipmentID: firstID, RecipientName: "Tigist Alemu"}); !r.OK {
		t.Fatalf("POD: %+v", r)
	}
	againID, _ := office.bookFor("Addis Ababa", "Adama", driverID, customer)
	otherID, _ := office.book("Addis Ababa", "Adama", driverID)

	// Another company delivering to the same customer.
	rival := e.company("Rival Routes")
	rivalDriver, rivalPhone, rivalPIN := rival.addDriver("Yonas Girma")
	rivalShipment, _ := rival.bookFor("Addis Ababa", "Adama", rivalDriver, customer)

	resp, _ := d.sync(service.SyncRequest{})
	got := map[string][]string{}
	for _, sh := range resp.Shipments {
		got[sh.ID] = sh.RecentRecipients
	}
	if !slices.Equal(got[againID], []string{"Tigist Alemu"}) {
		t.Errorf("repeat customer: suggestions %v, want Tigist Alemu", got[againID])
	}
	if other, ok := got[otherID]; !ok || other == nil || len(other) != 0 {
		t.Errorf("new customer: suggestions %#v, want an empty list", other)
	}
	resp, _ = e.signIn(rivalPhone, rivalPIN).sync(service.SyncRequest{})
	for _, sh := range resp.Shipments {
		if sh.ID == rivalShipment && len(sh.RecentRecipients) != 0 {
			t.Errorf("another company's driver was offered %v", sh.RecentRecipients)
		}
	}
}
