package service

import (
	"testing"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/model"
)

func TestStanding(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { v := now.Add(d); return &v }
	const day = 24 * time.Hour
	cases := []struct {
		name     string
		plan     string
		trial    *time.Time
		debt     string
		due      *time.Time
		state    string
		days     int
		readOnly bool
	}{
		{"trial with days left", model.PlanTrial, at(36 * time.Hour), "", nil, StandingTrialing, 2, false},
		{"trial ending within the hour", model.PlanTrial, at(time.Hour), "", nil, StandingTrialing, 1, false},
		{"trial without an end", model.PlanTrial, nil, "", nil, StandingTrialing, 0, false},
		{"trial ended", model.PlanTrial, at(-time.Minute), "", nil, StandingTrialEnded, 0, true},
		{"trial ending this instant", model.PlanTrial, at(0), "", nil, StandingTrialEnded, 0, true},
		{"paid up", model.PlanStarter, at(-30 * day), "", nil, StandingActive, 0, false},
		{"invoice not yet due", model.PlanBusiness, nil, "INV-2026-00007", at(3 * day), StandingDue, 0, false},
		{"invoice overdue", model.PlanBusiness, nil, "INV-2026-00007", at(-2 * day), StandingPastDue, model.GraceDays - 2, false},
		{"grace ends this instant", model.PlanBusiness, nil, "INV-2026-00007", at(-model.GraceDays * day), StandingReadOnly, 0, true},
		{"invoice long unpaid", model.PlanBusiness, nil, "INV-2026-00007", at(-40 * day), StandingReadOnly, 0, true},
	}
	for _, c := range cases {
		st := standingOf(c.plan, c.trial, c.debt, c.due, now)
		if st.State != c.state || st.DaysLeft != c.days || st.ReadOnly() != c.readOnly {
			t.Errorf("%s: %s with %d days left (read-only %v), want %s with %d (read-only %v)",
				c.name, st.State, st.DaysLeft, st.ReadOnly(), c.state, c.days, c.readOnly)
		}
	}
	if st := standingOf(model.PlanBusiness, nil, "INV-2026-00007", at(-2*day), now); !st.ReadOnlyAt.Equal(now.Add((model.GraceDays - 2) * day)) {
		t.Errorf("read-only from %s", st.ReadOnlyAt)
	}
}

func TestMoney(t *testing.T) {
	cases := map[int64]string{0: "ETB 0.00", 50: "ETB 0.50", 287500: "ETB 2,875.00", 1035000000: "ETB 10,350,000.00"}
	for santim, want := range cases {
		if got := money(santim); got != want {
			t.Errorf("money(%d) = %q, want %q", santim, got, want)
		}
	}
	if got := grouped(-1234567); got != "-1,234,567" {
		t.Errorf("grouped = %q", got)
	}
}

func TestPayReferences(t *testing.T) {
	if got := PayReference("INV-2026-00012"); got != "202600012" {
		t.Errorf("PayReference = %q, want 202600012", got)
	}
	// What a payer may key in for INV-2026-00012, and a counter past 99999.
	for in, want := range map[string]string{
		"202600012":      "INV-2026-00012",
		"INV-2026-00012": "INV-2026-00012",
		"inv 2026 00012": "INV-2026-00012",
		"2026-12":        "INV-2026-00012",
		"202612":         "INV-2026-00012",
		"2026123456":     "INV-2026-123456",
	} {
		if got := invoiceNumberOf(in); got != want {
			t.Errorf("invoiceNumberOf(%q) = %q, want %q", in, got, want)
		}
	}
	for _, in := range []string{"", "2026", "202600000", "INV-2026-0001A", "PAY-ABCDEF", "2026/00012"} {
		if got := invoiceNumberOf(in); got != "" {
			t.Errorf("invoiceNumberOf(%q) = %q, want nothing", in, got)
		}
	}
}
