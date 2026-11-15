package service

import (
	"testing"
	"time"
)

func TestTrackingNumberFormat(t *testing.T) {
	day := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	n := newTrackingNumber(day)
	if len(n) != len("ET-20261028-")+10 || n[:12] != "ET-20261028-" {
		t.Fatalf("tracking number %q", n)
	}
	for _, c := range n[12:] {
		found := false
		for _, a := range trackingAlphabet {
			found = found || a == c
		}
		if !found {
			t.Fatalf("tracking number %q uses %q outside the alphabet", n, c)
		}
	}
	if newTrackingNumber(day) == n {
		t.Error("two tracking numbers should differ")
	}
}

func TestResolvePlace(t *testing.T) {
	p, err := resolvePlace("origin", "nazret", "", nil, nil)
	if err != nil || p.City == nil || *p.City != "Adama" || p.Precise || p.Address != "Adama" {
		t.Fatalf("town only: %+v, %v", p, err)
	}
	lat, lng := 9.0105, 38.7613
	p, err = resolvePlace("destination", "", "Merkato, block 12", &lat, &lng)
	if err != nil || !p.Precise || p.City == nil || *p.City != "Addis Ababa" {
		t.Fatalf("pin: %+v, %v", p, err)
	}
	if _, err := resolvePlace("origin", "Atlantis", "", nil, nil); err == nil {
		t.Error("unknown town without a pin should be refused")
	}
	far, farLng := 40.0, -3.0
	if _, err := resolvePlace("origin", "", "", &far, &farLng); err == nil {
		t.Error("pin outside the service area should be refused")
	}
}
