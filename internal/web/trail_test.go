package web

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
)

// A trail has a gap only where the phone was quiet for a while and the
// truck moved meanwhile: a long stop, or a position every 15 minutes while
// saving battery, is not one.
func TestTrailGaps(t *testing.T) {
	start := time.Date(2026, 11, 18, 8, 0, 0, 0, time.UTC)
	at := func(min int, lat float64) db.ListShipmentRouteRow {
		return db.ListShipmentRouteRow{Latitude: lat, Longitude: 38.74, RecordedAt: pgtype.Timestamptz{Time: start.Add(time.Duration(min) * time.Minute), Valid: true}}
	}
	route, gaps := trail([]db.ListShipmentRouteRow{
		at(0, 9.00),
		at(1, 9.01),    // a minute on, 1 km on
		at(16, 9.10),   // 15 minutes on, saving battery
		at(76, 9.1001), // an hour parked
		at(136, 9.60),  // an hour without positions, 55 km on: a gap
		at(137, 9.61),
	})
	if len(route) != 6 || route[4][0] != 9.60 {
		t.Fatalf("route = %v", route)
	}
	if len(gaps) != 1 || gaps[0] != (routeGap{At: 4, Minutes: 60}) {
		t.Errorf("gaps = %+v, want one before position 4, of 60 minutes", gaps)
	}
	if route, gaps := trail(nil); len(route) != 0 || len(gaps) != 0 || route == nil || gaps == nil {
		t.Errorf("no positions: %v %v, want empty lists for the JSON", route, gaps)
	}
}
