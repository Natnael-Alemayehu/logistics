package geo

import (
	"math"
	"testing"
	"time"
)

func TestLookupNormalisesAndResolvesAliases(t *testing.T) {
	for _, name := range []string{"Adama", "adama", "  ADAMA ", "Nazret", "nazareth"} {
		p, ok := Lookup(name)
		if !ok || p.Name != "Adama" {
			t.Errorf("Lookup(%q) = %v, %v; want Adama", name, p.Name, ok)
		}
	}
	if _, ok := Lookup("Atlantis"); ok {
		t.Error("Lookup should not find an unknown place")
	}
}

func TestDistanceAddisToAdama(t *testing.T) {
	a, _ := Lookup("Addis Ababa")
	b, _ := Lookup("Adama")
	km := DistanceMeters(a.Lat, a.Lng, b.Lat, b.Lng) / 1000
	// Straight line is ~78 km; the road is ~100 km.
	if km < 70 || km > 90 {
		t.Errorf("Addis–Adama straight-line distance = %.1f km, want ~78", km)
	}
}

func TestDistanceIsZeroForSamePoint(t *testing.T) {
	if d := DistanceMeters(9.03, 38.74, 9.03, 38.74); d != 0 {
		t.Errorf("distance to self = %v, want 0", d)
	}
}

func TestNearest(t *testing.T) {
	p, d := Nearest(8.55, 39.28)
	if p.Name != "Adama" {
		t.Errorf("Nearest = %s, want Adama", p.Name)
	}
	if d > 2000 {
		t.Errorf("distance to Adama = %.0f m, want < 2 km", d)
	}
}

func TestRegionsFrameTheirTowns(t *testing.T) {
	regions := Regions()
	seen := map[string]Region{}
	for i, r := range regions {
		if i > 0 && regions[i-1].Name >= r.Name {
			t.Fatalf("regions not sorted: %q before %q", regions[i-1].Name, r.Name)
		}
		if r.South >= r.North || r.West >= r.East {
			t.Errorf("%s has an empty frame: %+v", r.Name, r)
		}
		seen[r.Name] = r
	}
	for _, p := range places {
		r, ok := seen[p.Region]
		if !ok {
			t.Fatalf("no region for %s (%s)", p.Name, p.Region)
		}
		if p.Lat < r.South || p.Lat > r.North || p.Lng < r.West || p.Lng > r.East {
			t.Errorf("%s lies outside the frame of %s", p.Name, r.Name)
		}
	}
}

func TestInServiceArea(t *testing.T) {
	if !InServiceArea(9.03, 38.74) {
		t.Error("Addis Ababa should be in the service area")
	}
	if InServiceArea(38.74, 9.03) {
		t.Error("swapped coordinates should be rejected")
	}
}

func TestEstimateTransitAddisDjiboutiIsMultiDay(t *testing.T) {
	a, _ := Lookup("Addis Ababa")
	d, _ := Lookup("Djibouti")
	eta := EstimateTransit(a.Lat, a.Lng, d.Lat, d.Lng, nil)
	// ~910 km of road: about 20 h of driving split over two days with a night's
	// rest, plus handling — roughly a day and a half to two days.
	if eta < 30*time.Hour || eta > 50*time.Hour {
		t.Errorf("Addis–Djibouti estimate = %v, want 30–50h", eta)
	}
}

func TestEstimateTransitShortHop(t *testing.T) {
	a, _ := Lookup("Addis Ababa")
	b, _ := Lookup("Adama")
	eta := EstimateTransit(a.Lat, a.Lng, b.Lat, b.Lng, nil)
	if eta < 3*time.Hour || eta > 7*time.Hour {
		t.Errorf("Addis–Adama estimate = %v, want 3–7h", eta)
	}
}

func TestEstimateTransitPrefersHistoryMedian(t *testing.T) {
	history := []float64{10, 12, 11, 30, 9}
	eta := EstimateTransit(0, 0, 10, 10, history)
	if math.Abs(eta.Hours()-11) > 0.001 {
		t.Errorf("estimate = %v, want median 11h", eta)
	}
}

func TestEstimateTransitIgnoresThinHistory(t *testing.T) {
	a, _ := Lookup("Addis Ababa")
	b, _ := Lookup("Adama")
	withThin := EstimateTransit(a.Lat, a.Lng, b.Lat, b.Lng, []float64{100, 100})
	without := EstimateTransit(a.Lat, a.Lng, b.Lat, b.Lng, nil)
	if withThin != without {
		t.Errorf("two samples should not override the model: %v vs %v", withThin, without)
	}
}

func TestDeliveryTolerance(t *testing.T) {
	if DeliveryTolerance(true) != 500 {
		t.Error("precise tolerance should be 500 m")
	}
	if DeliveryTolerance(false) <= 500 {
		t.Error("city-level tolerance should be generous")
	}
}
