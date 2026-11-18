package service

import (
	"testing"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
)

func TestProgress(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	// Addis Ababa to Adama, about 75 km east.
	sh := db.Shipment{Status: model.StatusInTransit, OriginLat: f(9.03), OriginLng: f(38.74), DestinationLat: f(8.54), DestinationLng: f(39.27)}
	halfway := &DriverPin{Lat: 8.785, Lng: 39.005}
	if p := Progress(sh, halfway); p == nil || *p < 45 || *p > 55 {
		t.Fatalf("halfway: got %v", p)
	}
	if p := Progress(sh, &DriverPin{Lat: 9.03, Lng: 38.74}); p == nil || *p != 1 {
		t.Errorf("at the start shows as begun: got %v", p)
	}
	if p := Progress(sh, &DriverPin{Lat: 8.54, Lng: 39.27}); p == nil || *p != 99 {
		t.Errorf("at the gate but not arrived: got %v", p)
	}
	if p := Progress(sh, nil); p != nil {
		t.Errorf("no position: got %d", *p)
	}
	sh.Status = model.StatusArrived
	if p := Progress(sh, nil); p == nil || *p != 100 {
		t.Errorf("arrived: got %v", p)
	}
	sh.Status = model.StatusAssigned
	if p := Progress(sh, halfway); p != nil {
		t.Errorf("not under way: got %d", *p)
	}
}
