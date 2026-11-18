//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// TestCargoPieces: a load can say how many pieces it is ("1,200 bags"). The
// count is kept as a number and a unit, and shown, exported and sent to the
// driver as it was written.
func TestCargoPieces(t *testing.T) {
	e := newEnv(t)
	office := e.company("Counted Freight")
	driverID, driverPhone, pin := office.addDriver("Hana Girma")

	form := bookingForm()
	form.Set("driver_id", driverID)
	for _, bad := range []string{"bags", "0 bags", strings.Repeat("9", 8) + " bags", "12 " + strings.Repeat("x", 31)} {
		form.Set("cargo_pieces", bad)
		if p := office.post("/app/shipments", form); p.Status != http.StatusBadRequest {
			t.Errorf("pieces %q: status %d, want 400", bad, p.Status)
		}
	}

	form.Set("cargo_pieces", "1,200 bags")
	id := strings.TrimPrefix(office.mustPost("/app/shipments", form).Location, "/app/shipments/")
	if n, unit := scalar[int32](t, `SELECT cargo_pieces FROM shipments WHERE id = $1`, id), scalar[string](t, `SELECT cargo_unit FROM shipments WHERE id = $1`, id); n != 1200 || unit != "bags" {
		t.Fatalf("stored %d %q, want 1200 bags", n, unit)
	}
	if p := office.get("/app/shipments/" + id); !strings.Contains(p.Body, "<dt>Pieces</dt><dd>1200 bags</dd>") {
		t.Error("the shipment page should show the pieces")
	}
	if p := office.get("/app/shipments/" + id + "/edit"); !strings.Contains(p.Body, `name="cargo_pieces" value="1200 bags"`) {
		t.Error("the edit form should hold the pieces")
	}
	if p := office.get("/app/shipments?format=csv"); !strings.Contains(p.Body, ",Pieces,") || !strings.Contains(p.Body, ",1200 bags,") {
		t.Error("the export should carry the pieces")
	}
	d := e.signIn(driverPhone, pin)
	if resp, _ := d.sync(service.SyncRequest{}); len(resp.Shipments) != 1 || resp.Shipments[0].Pieces != "1200 bags" {
		t.Errorf("the driver app's shipment: %+v", resp.Shipments)
	}

	// Emptying the field removes the count.
	edit := bookingForm()
	edit.Set("cargo_pieces", "")
	office.mustPost("/app/shipments/"+id+"/edit", edit)
	if n := scalar[int](t, `SELECT count(*) FROM shipments WHERE id = $1 AND cargo_pieces IS NULL AND cargo_unit IS NULL`, id); n != 1 {
		t.Error("an empty field should remove the count")
	}
}

// TestBookingEstimate: the booking form's summary asks for the distance and
// arrival between two towns as they are typed.
func TestBookingEstimate(t *testing.T) {
	e := newEnv(t)
	office := e.company("Estimating Freight")
	var got struct {
		Known  bool    `json:"known"`
		RoadKM float64 `json:"road_km"`
		Hours  float64 `json:"hours"`
		Arrive string  `json:"arrive"`
	}
	p := office.get("/app/shipments/estimate?" + url.Values{"from": {"Addis Ababa"}, "to": {"Dire Dawa"}, "pickup": {"2030-01-10T08:00"}}.Encode())
	if err := json.Unmarshal([]byte(p.Body), &got); err != nil || !got.Known || got.RoadKM < 350 || got.RoadKM > 600 || got.Hours <= 0 || !strings.Contains(got.Arrive, "Jan 2030") {
		t.Errorf("Addis Ababa to Dire Dawa: %s", p.Body)
	}
	p = office.get("/app/shipments/estimate?from=Addis+Ababa&to=Nowhere")
	if err := json.Unmarshal([]byte(p.Body), &got); err != nil || got.Known {
		t.Errorf("an unknown town: %s", p.Body)
	}
	if p := e.browser().get("/app/shipments/estimate?from=Addis+Ababa&to=Adama"); p.Status == http.StatusOK && strings.Contains(p.Body, "road_km") {
		t.Error("the estimate is for signed-in users")
	}
}
