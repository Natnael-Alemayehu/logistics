package service

import (
	"testing"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/model"
)

func TestTrackingSteps(t *testing.T) {
	steps := trackingSteps(model.StatusArrived)
	if !steps[3].Current || !steps[2].Done || steps[4].Done {
		t.Errorf("arrived steps = %+v", steps)
	}
	for _, s := range trackingSteps(model.StatusCancelled) {
		if s.Done {
			t.Error("a cancelled shipment has no progress")
		}
	}
}

func TestCoarsePositionHidesExactLocation(t *testing.T) {
	p := coarsePosition(8.5432, 39.2701, time.Now())
	if p.Near != "near Adama" {
		t.Errorf("near = %q, want %q", p.Near, "near Adama")
	}
	if p.Lat == 8.5432 || p.Lng == 39.2701 {
		t.Error("public position should be rounded")
	}
}
