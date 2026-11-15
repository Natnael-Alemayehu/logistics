package service

import (
	"testing"
	"time"
)

func TestHumanDuration(t *testing.T) {
	cases := map[time.Duration]string{
		30 * time.Minute: "30 minutes",
		time.Hour:        "an hour",
		5 * time.Hour:    "5 hours",
		72 * time.Hour:   "3 days",
	}
	for d, want := range cases {
		if got := humanDuration(d); got != want {
			t.Errorf("humanDuration(%v) = %q, want %q", d, got, want)
		}
	}
}
