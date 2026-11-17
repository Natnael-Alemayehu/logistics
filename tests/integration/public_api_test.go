//go:build integration

package integration

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

type apiShipment struct {
	ID             string `json:"id"`
	TrackingNumber string `json:"tracking_number"`
	Status         string `json:"status"`
	StatusReason   string `json:"status_reason"`
	Destination    struct {
		City    string `json:"city"`
		Address string `json:"address"`
	} `json:"destination"`
	Customer struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	} `json:"customer"`
	Receiver *struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	} `json:"receiver"`
	Cargo struct {
		WeightKg float64 `json:"weight_kg"`
	} `json:"cargo"`
	Instructions string  `json:"instructions"`
	DriverID     *string `json:"driver_id"`
	TrackingURL  string  `json:"tracking_url"`
}

type apiFailure struct {
	Error struct {
		Code, Message, Field string
	} `json:"error"`
}

var apiKeyRe = regexp.MustCompile(`data-secret>(kf_[a-z0-9]+)<`)

// makeAPIKey makes a key on the dashboard and returns it.
func (b *browser) makeAPIKey(name string) string {
	b.e.t.Helper()
	p := b.post("/app/settings/api", url.Values{"name": {name}})
	m := apiKeyRe.FindStringSubmatch(p.Body)
	if p.Status != http.StatusOK || m == nil {
		b.e.t.Fatalf("make API key: status %d %s", p.Status, errorText(p.Body))
	}
	return m[1]
}

// TestPublicAPI (FR-046): a company's own system books and follows its
// shipments with a key, under the dashboard's rules, and a revoked key
// stops at once.
func TestPublicAPI(t *testing.T) {
	e := newEnv(t)
	office := e.company("Integrated Freight")
	driverID, _, _ := office.addDriver("Abebe Kebede")

	// Without a key, or with a wrong one, nothing is answered.
	var fail apiFailure
	for _, key := range []string{"", "kf_notakeyatallnotakeyatall", "the dashboard password"} {
		if st := e.api(http.MethodGet, "/api/v1/shipments", key, nil, &fail); st != http.StatusUnauthorized || fail.Error.Code != "UNAUTHORIZED" {
			t.Errorf("key %q: status %d %+v", key, st, fail)
		}
	}
	// The document is public.
	if p := e.browser().get("/api/v1/openapi.json"); p.Status != http.StatusOK || !strings.Contains(p.Body, `"openapi"`) {
		t.Errorf("openapi.json: status %d", p.Status)
	}

	if p := office.get("/app/settings/api"); p.Status != http.StatusOK || !strings.Contains(p.Body, "No keys yet") {
		t.Fatalf("the keys page: status %d", p.Status)
	}
	if p := office.post("/app/settings/api", url.Values{"name": {"  "}}); p.Status != http.StatusBadRequest {
		t.Errorf("a key without a name: status %d", p.Status)
	}
	key := office.makeAPIKey("Warehouse system")
	if p := office.get("/app/settings/api"); strings.Contains(p.Body, key) || !strings.Contains(p.Body, key[:9]+"…") {
		t.Error("the list should show the key's start, never the key")
	}

	// Booking: checked as the form checks it, and a field the API doesn't
	// have is refused rather than ignored.
	customer, receiver := phoneNumber(), phoneNumber()
	booking := map[string]any{
		"origin":      map[string]any{"city": "Addis Ababa", "address": "Main depot"},
		"destination": map[string]any{"city": "Adama", "address": "Customer warehouse"},
		"customer":    map[string]any{"name": "Abyssinia Traders", "phone": customer},
		"receiver":    map[string]any{"name": "Adama Wholesale", "phone": receiver},
		"cargo":       map[string]any{"description": "Cement, 200 bags", "weight_kg": 10000},
	}
	for why, c := range map[string]struct {
		change map[string]any
		field  string
	}{
		"unknown town":     {map[string]any{"destination": map[string]any{"city": "Atlantis", "address": "x"}}, "destination_city"},
		"bad phone":        {map[string]any{"customer": map[string]any{"name": "A", "phone": "12"}}, "customer_phone"},
		"misspelt field":   {map[string]any{"recevier": map[string]any{}}, ""},
		"unknown driver":   {map[string]any{"driver_id": "00000000-0000-0000-0000-000000000000"}, "driver_id"},
		"unknown priority": {map[string]any{"priority": "whenever"}, "priority"},
	} {
		body := map[string]any{}
		for k, v := range booking {
			body[k] = v
		}
		for k, v := range c.change {
			body[k] = v
		}
		fail = apiFailure{}
		if st := e.api(http.MethodPost, "/api/v1/shipments", key, body, &fail); st != http.StatusBadRequest || fail.Error.Field != c.field {
			t.Errorf("%s: status %d, field %q, want %q: %s", why, st, fail.Error.Field, c.field, fail.Error.Message)
		}
	}
	if n := scalar[int64](t, `SELECT count(*) FROM shipments WHERE customer_phone = $1`, "+251"+customer[1:]); n != 0 {
		t.Fatalf("%d shipments booked by refused requests", n)
	}

	var sh apiShipment
	if st := e.api(http.MethodPost, "/api/v1/shipments", key, booking, &sh); st != http.StatusCreated {
		t.Fatalf("book: status %d", st)
	}
	if sh.Status != "pending" || sh.TrackingNumber == "" || sh.Receiver == nil || sh.Receiver.Name != "Adama Wholesale" || !strings.HasSuffix(sh.TrackingURL, "/track/"+sh.TrackingNumber) {
		t.Errorf("booked: %+v", sh)
	}
	e.received("sms", customer, "is booked")
	e.received("sms", receiver, "is booked")
	if who := scalar[string](t, `SELECT actor_name || ' / ' || source FROM shipment_events WHERE shipment_id = $1 AND event_type = 'created'`, sh.ID); who != "API: Warehouse system / api" {
		t.Errorf("the booking is recorded as %q", who)
	}

	// Found by tracking number, lower case too, or by id.
	for _, ref := range []string{sh.TrackingNumber, strings.ToLower(sh.TrackingNumber), sh.ID} {
		var got apiShipment
		if st := e.api(http.MethodGet, "/api/v1/shipments/"+ref, key, nil, &got); st != http.StatusOK || got.ID != sh.ID {
			t.Errorf("GET %s: status %d", ref, st)
		}
	}

	// A change leaves out what it doesn't name, and null removes.
	var changed apiShipment
	st := e.api(http.MethodPatch, "/api/v1/shipments/"+sh.TrackingNumber, key, map[string]any{
		"destination": map[string]any{"city": "Hawassa", "address": "Lake road"}, "receiver": nil, "instructions": "Call before arriving",
	}, &changed)
	if st != http.StatusOK || changed.Destination.City != "Hawassa" || changed.Receiver != nil || changed.Instructions != "Call before arriving" ||
		changed.Customer.Name != "Abyssinia Traders" || changed.Cargo.WeightKg != 10000 {
		t.Errorf("change: status %d %+v", st, changed)
	}
	if st := e.api(http.MethodPatch, "/api/v1/shipments/"+sh.ID, key, map[string]any{"driver_id": driverID}, &fail); st != http.StatusBadRequest {
		t.Errorf("a driver through PATCH: status %d", st)
	}

	// Assigning, and taking the driver off again.
	var assigned apiShipment
	if st := e.api(http.MethodPost, "/api/v1/shipments/"+sh.ID+"/assign", key, map[string]any{"driver_id": driverID}, &assigned); st != http.StatusOK || assigned.Status != "assigned" || assigned.DriverID == nil || *assigned.DriverID != driverID {
		t.Errorf("assign: status %d %+v", st, assigned)
	}
	var drivers struct {
		Data []struct {
			ID              string `json:"id"`
			Name            string `json:"name"`
			ActiveShipments int64  `json:"active_shipments"`
		} `json:"data"`
	}
	if st := e.api(http.MethodGet, "/api/v1/drivers", key, nil, &drivers); st != http.StatusOK || len(drivers.Data) != 1 || drivers.Data[0].Name != "Abebe Kebede" || drivers.Data[0].ActiveShipments != 1 {
		t.Errorf("drivers: status %d %+v", st, drivers)
	}
	if st := e.api(http.MethodPost, "/api/v1/shipments/"+sh.ID+"/assign", key, map[string]any{"driver_id": nil}, &assigned); st != http.StatusOK || assigned.Status != "pending" || assigned.DriverID != nil {
		t.Errorf("unassign: status %d %+v", st, assigned)
	}

	// The list filters.
	_, other := office.book("Addis Ababa", "Adama", driverID)
	var list struct {
		Data  []apiShipment `json:"data"`
		Total int64         `json:"total"`
	}
	if st := e.api(http.MethodGet, "/api/v1/shipments?status=pending", key, nil, &list); st != http.StatusOK || list.Total != 1 || list.Data[0].ID != sh.ID {
		t.Errorf("pending: status %d %+v", st, list)
	}
	if st := e.api(http.MethodGet, "/api/v1/shipments?status=pending,assigned&per_page=1", key, nil, &list); st != http.StatusOK || list.Total != 2 || len(list.Data) != 1 {
		t.Errorf("two statuses, a page of one: status %d %+v", st, list)
	}
	if st := e.api(http.MethodGet, "/api/v1/shipments?status=lost", key, nil, &fail); st != http.StatusBadRequest || fail.Error.Field != "status" {
		t.Errorf("an unknown status: status %d", st)
	}

	// Cancelling, with the reasons the dashboard offers.
	if st := e.api(http.MethodPost, "/api/v1/shipments/"+sh.ID+"/cancel", key, map[string]any{"reason": "recipient_refused"}, &fail); st != http.StatusBadRequest {
		t.Errorf("a reason for the road, before departure: status %d", st)
	}
	var cancelled apiShipment
	if st := e.api(http.MethodPost, "/api/v1/shipments/"+sh.ID+"/cancel", key, map[string]any{"reason": "duplicate", "note": "Booked twice"}, &cancelled); st != http.StatusOK || cancelled.Status != "cancelled" || cancelled.StatusReason != "duplicate" {
		t.Errorf("cancel: status %d %+v", st, cancelled)
	}
	if st := e.api(http.MethodPost, "/api/v1/shipments/"+sh.ID+"/cancel", key, map[string]any{"reason": "duplicate"}, &fail); st != http.StatusConflict {
		t.Errorf("cancelling twice: status %d", st)
	}

	// The history, oldest first.
	var history struct {
		Data []struct {
			Type     string `json:"type"`
			ToStatus string `json:"to_status"`
			Source   string `json:"source"`
			Actor    string `json:"actor"`
		} `json:"data"`
	}
	if st := e.api(http.MethodGet, "/api/v1/shipments/"+sh.TrackingNumber+"/events", key, nil, &history); st != http.StatusOK || len(history.Data) < 4 {
		t.Fatalf("events: status %d %+v", st, history)
	}
	if first, last := history.Data[0], history.Data[len(history.Data)-1]; first.Type != "created" || last.ToStatus != "cancelled" || last.Actor != "API: Warehouse system" || last.Source != "api" {
		t.Errorf("history from %+v to %+v", first, last)
	}

	// Another company's key sees none of it.
	rival := e.company("Rival Freight")
	rivalKey := rival.makeAPIKey("Their system")
	for _, ref := range []string{sh.ID, sh.TrackingNumber, other} {
		if st := e.api(http.MethodGet, "/api/v1/shipments/"+ref, rivalKey, nil, &fail); st != http.StatusNotFound {
			t.Errorf("another company's shipment %s: status %d", ref, st)
		}
		if st := e.api(http.MethodPost, "/api/v1/shipments/"+ref+"/cancel", rivalKey, map[string]any{"reason": "other"}, &fail); st != http.StatusNotFound {
			t.Errorf("cancelling another company's shipment %s: status %d", ref, st)
		}
	}
	if st := e.api(http.MethodGet, "/api/v1/shipments", rivalKey, nil, &list); st != http.StatusOK || list.Total != 0 {
		t.Errorf("another company's list: %d shipments", list.Total)
	}

	// Revoking stops the key at once; the audit log has both.
	if used := scalar[bool](t, `SELECT last_used_at IS NOT NULL FROM api_keys WHERE name = 'Warehouse system'`); !used {
		t.Error("the key's last use should be recorded")
	}
	id := scalar[string](t, `SELECT id::text FROM api_keys WHERE name = 'Warehouse system'`)
	if p := rival.post("/app/settings/api/"+id+"/revoke", nil); p.Status != http.StatusNotFound {
		t.Errorf("another company revoking the key: status %d", p.Status)
	}
	office.mustPost("/app/settings/api/"+id+"/revoke", nil)
	if st := e.api(http.MethodGet, "/api/v1/shipments", key, nil, &fail); st != http.StatusUnauthorized {
		t.Errorf("a revoked key: status %d", st)
	}
	if p := office.get("/app/audit"); !strings.Contains(p.Body, "Made an API key") || !strings.Contains(p.Body, "Revoked an API key") {
		t.Error("the audit log should record making and revoking the key")
	}

	// A suspended company's keys stop too.
	exec(t, `UPDATE tenants SET is_active = false WHERE id = (SELECT tenant_id FROM api_keys WHERE name = 'Their system')`)
	if st := e.api(http.MethodGet, "/api/v1/shipments", rivalKey, nil, &fail); st != http.StatusForbidden {
		t.Errorf("a suspended company's key: status %d", st)
	}
}

// TestAPIKeysAreTheAdminsAlone: a dispatcher can't make, see or revoke
// keys, which act for the whole company.
func TestAPIKeysAreTheAdminsAlone(t *testing.T) {
	e := newEnv(t)
	office := e.company("Careful Freight")
	office.makeAPIKey("Ours")
	email := "dispatch" + phoneNumber() + "@example.et"
	p := office.post("/app/team", url.Values{"full_name": {"Dawit Bekele"}, "email": {email}, "phone": {phoneNumber()}, "role": {"dispatcher"}})
	m := secretRe.FindStringSubmatch(p.Body)
	if m == nil {
		t.Fatalf("add dispatcher: status %d %s", p.Status, errorText(p.Body))
	}
	dispatcher := e.browser()
	dispatcher.mustPost("/login", url.Values{"identifier": {email}, "password": {strings.TrimSpace(m[1])}})
	dispatcher.mustPost("/app/account/password", url.Values{
		"current_password": {strings.TrimSpace(m[1])}, "new_password": {"dispatch-pass-9"}, "confirm_password": {"dispatch-pass-9"},
	})
	if p := dispatcher.get("/app/settings/api"); p.Status != http.StatusForbidden {
		t.Errorf("a dispatcher on the keys page: status %d", p.Status)
	}
	if p := dispatcher.post("/app/settings/api", url.Values{"name": {"Mine"}}); p.Status != http.StatusForbidden {
		t.Errorf("a dispatcher making a key: status %d", p.Status)
	}
	if n := scalar[int64](t, `SELECT count(*) FROM api_keys WHERE name = 'Mine'`); n != 0 {
		t.Error("a dispatcher made a key")
	}
}
