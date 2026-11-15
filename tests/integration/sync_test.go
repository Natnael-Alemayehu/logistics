//go:build integration

package integration

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/service"
)

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
