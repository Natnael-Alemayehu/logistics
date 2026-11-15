//go:build integration

package integration

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

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
