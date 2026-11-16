//go:build integration

package integration

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// TestConsignee: a shipment can name who takes delivery apart from the
// customer who booked it. Both are kept informed, and what belongs to the
// handover goes to the receiver: the arrival, the driver's call, and the
// link to confirm receipt (audit W4).
func TestConsignee(t *testing.T) {
	e := newEnv(t)
	office := e.company("Two Party Freight")
	driverID, driverPhone, pin := office.addDriver("Hana Girma")
	customer, receiver := phoneNumber(), phoneNumber()
	form := func(name, number string) url.Values {
		return url.Values{
			"priority": {"normal"}, "origin_city": {"Addis Ababa"}, "origin_address": {"Main depot"},
			"destination_city": {"Hawassa"}, "destination_address": {"Lakeside store"},
			"customer_name": {"Abyssinia Traders"}, "customer_phone": {customer},
			"consignee_name": {name}, "consignee_phone": {number},
			"cargo_description": {"Cement, 200 bags"}, "cargo_weight": {"10000"}, "driver_id": {driverID},
		}
	}
	for why, f := range map[string]url.Values{
		"a receiver with no phone":        form("Lakeside Hardware", ""),
		"a phone with no name":            form("", receiver),
		"a bad phone":                     form("Lakeside Hardware", "12"),
		"the customer's own number again": form("Lakeside Hardware", customer),
	} {
		if p := office.post("/app/shipments", f); p.Status != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", why, p.Status)
		}
	}
	p := office.mustPost("/app/shipments", form("Lakeside Hardware", receiver))
	id := strings.TrimPrefix(p.Location, "/app/shipments/")
	tracking := scalar[string](t, `SELECT tracking_number FROM shipments WHERE id = $1`, id)

	// Both are told it is booked, and the driver is told who to deliver to.
	e.received("sms", customer, "is booked")
	e.received("sms", receiver, "is booked")
	if got := e.received("sms", driverPhone, "new shipment").Body; !strings.Contains(got, "Deliver to Lakeside Hardware "+receiver) || strings.Contains(got, customer) {
		t.Errorf("the driver's text should name the receiver: %q", got)
	}

	// The office sees both, finds the shipment by either, and prints both.
	if p := office.get("/app/shipments/" + id); !strings.Contains(p.Body, "<dt>Receiver</dt><dd>Lakeside Hardware") {
		t.Error("the shipment page should show the receiver")
	}
	for _, q := range []string{"Lakeside", receiver, "Abyssinia"} {
		if p := office.get("/app/shipments?q=" + url.QueryEscape(q)); !strings.Contains(p.Body, tracking) {
			t.Errorf("searching for %q should find the shipment", q)
		}
	}
	if p := office.get("/app/shipments/" + id + "/waybill"); !strings.Contains(p.Body, "Shipper / ላኪ") || !strings.Contains(p.Body, "<strong>Lakeside Hardware</strong>") {
		t.Error("the waybill should name the shipper and the consignee")
	}
	if p := office.get("/app/shipments?format=csv"); !strings.Contains(p.Body, "Receiver,Receiver phone") || !strings.Contains(p.Body, "Lakeside Hardware,"+receiver) {
		t.Error("the export should carry the receiver")
	}

	// The driver app is given both.
	d := e.signIn(driverPhone, pin)
	resp, _ := d.sync(service.SyncRequest{})
	if len(resp.Shipments) != 1 || resp.Shipments[0].ConsigneeName != "Lakeside Hardware" || resp.Shipments[0].ConsigneePhone != receiver || resp.Shipments[0].CustomerPhone != customer {
		t.Fatalf("the driver app's shipment: %+v", resp.Shipments)
	}

	// On the way: both. Arrived: only the one being handed to.
	if r := d.status(clientID(), id, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	e.received("sms", customer, "on its way")
	e.received("sms", receiver, "on its way")
	if r := d.status(clientID(), id, "arrived"); !r.OK {
		t.Fatalf("arrive: %+v", r)
	}
	e.received("sms", receiver, "has arrived in Hawassa")
	if n := scalar[int64](t, `SELECT count(*) FROM sms_messages WHERE shipment_id = $1 AND phone = $2 AND body LIKE '%has arrived%'`, id, customer); n != 0 {
		t.Error("the customer who booked isn't the one the driver hands over to, and should get no arrival text")
	}

	// Delivered: the receiver confirms, the customer is told.
	if r := d.pod(service.SyncPOD{ShipmentID: id, RecipientName: "Meron at Lakeside"}); !r.OK {
		t.Fatalf("proof: %+v", r)
	}
	told := e.received("sms", customer, "was delivered to Meron at Lakeside").Body
	if strings.Contains(told, "/c/") || !strings.Contains(told, "/track/"+tracking) {
		t.Errorf("the customer's text should carry the tracking link, not the confirmation link: %q", told)
	}
	asked := e.received("sms", receiver, "Confirm receipt").Body
	link := asked[strings.Index(asked, "/c/"):]
	receiverBrowser := e.browser()
	receiverBrowser.mustPost(link, url.Values{"answer": {"problem"}, "feedback": {"Two bags torn"}})
	if got := scalar[string](t, `SELECT message FROM alerts WHERE shipment_id = $1 AND alert_type = 'customer_reported_problem'`, id); !strings.HasPrefix(got, "Lakeside Hardware reported a problem") {
		t.Errorf("the alert should name the receiver: %q", got)
	}
	if got := scalar[string](t, `SELECT actor_name FROM shipment_events WHERE shipment_id = $1 AND event_type = 'customer_issue'`, id); got != "Lakeside Hardware" {
		t.Errorf("the report should be recorded under the receiver's name, not %q", got)
	}

	// Each finds the shipment with their own phone.
	for _, number := range []string{customer, receiver} {
		b := e.browser()
		b.post("/track/phone", url.Values{"phone": {number}})
		b.mustPost("/track/phone/verify", url.Values{"phone": {number}, "code": {e.code(number)}})
		if p := b.get("/track/mine"); !strings.Contains(p.Body, tracking) {
			t.Errorf("%s should find the shipment by phone", number)
		}
	}

	// The next delivery to the same receiver suggests who signed last time.
	next := office.mustPost("/app/shipments", form("Lakeside Hardware", receiver))
	nextID := strings.TrimPrefix(next.Location, "/app/shipments/")
	resp, _ = d.sync(service.SyncRequest{})
	var suggested []string
	for _, sh := range resp.Shipments {
		if sh.ID == nextID {
			suggested = sh.RecentRecipients
		}
	}
	if len(suggested) != 1 || suggested[0] != "Meron at Lakeside" {
		t.Errorf("recent recipients for the receiver: %v", suggested)
	}

	// Clearing the receiver on edit gives the shipment back to the customer.
	edit := form("", "")
	edit.Del("driver_id")
	office.mustPost("/app/shipments/"+nextID+"/edit", edit)
	if n := scalar[int64](t, `SELECT count(*) FROM shipments WHERE id = $1 AND consignee_name IS NULL AND consignee_phone IS NULL`, nextID); n != 1 {
		t.Error("clearing the receiver should clear both columns")
	}
	if got := scalar[string](t, `SELECT details::text FROM shipment_events WHERE shipment_id = $1 AND event_type = 'edited'`, nextID); !strings.Contains(got, "receiver") {
		t.Errorf("the edit should be recorded as a change of receiver: %s", got)
	}
}
