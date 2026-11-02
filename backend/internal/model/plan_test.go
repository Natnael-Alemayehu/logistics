package model

import (
	"testing"
	"time"
)

func TestPeriodPrice(t *testing.T) {
	if got := PeriodPrice(250000, CycleMonthly); got != 250000 {
		t.Errorf("monthly = %d", got)
	}
	if got := PeriodPrice(250000, CycleAnnual); got != 2500000 {
		t.Errorf("a year should cost ten months, got %d", got)
	}
}

func TestPeriodEnd(t *testing.T) {
	addis, _ := time.LoadLocation("Africa/Addis_Ababa")
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 9, 30, 0, 0, addis) }
	cases := []struct {
		start time.Time
		cycle string
		want  time.Time
	}{
		{day(2026, 11, 2), CycleMonthly, day(2026, 12, 2)},
		{day(2026, 12, 15), CycleMonthly, day(2027, 1, 15)},
		{day(2027, 1, 31), CycleMonthly, day(2027, 2, 28)},
		{day(2028, 1, 31), CycleMonthly, day(2028, 2, 29)},
		{day(2027, 3, 31), CycleMonthly, day(2027, 4, 30)},
		{day(2027, 1, 30), CycleMonthly, day(2027, 2, 28)},
		{day(2026, 11, 2), CycleAnnual, day(2027, 11, 2)},
		{day(2028, 2, 29), CycleAnnual, day(2029, 2, 28)},
	}
	for _, c := range cases {
		if got := PeriodEnd(c.start, c.cycle); !got.Equal(c.want) {
			t.Errorf("PeriodEnd(%s, %s) = %s, want %s", c.start.Format("2 Jan 2006"), c.cycle, got.Format("2 Jan 2006 15:04"), c.want.Format("2 Jan 2006 15:04"))
		}
	}
}

func TestVAT(t *testing.T) {
	cases := map[int64]int64{
		0:       0,
		250000:  37500,  // ETB 2,500.00 → 375.00
		900000:  135000, // ETB 9,000.00 → 1,350.00
		50:      8,      // 7.5 santim rounds up
		3:       0,      // 0.45 santim rounds down
		1234567: 185185, // 185,185.05 → 185,185
	}
	for subtotal, want := range cases {
		if got := VAT(subtotal); got != want {
			t.Errorf("VAT(%d) = %d, want %d", subtotal, got, want)
		}
	}
}

func TestProrate(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC) // 30 days
	cases := []struct {
		name string
		now  time.Time
		want int64
	}{
		{"before the period", start.Add(-48 * time.Hour), 650000},
		{"at the start", start, 650000},
		{"a third in", start.Add(10 * 24 * time.Hour), 433333},
		{"half way", start.Add(15 * 24 * time.Hour), 325000},
		{"the last day", end.Add(-24 * time.Hour), 21667},
		{"at the end", end, 0},
		{"after the end", end.Add(time.Hour), 0},
	}
	for _, c := range cases {
		if got := Prorate(650000, start, end, c.now); got != c.want {
			t.Errorf("%s: Prorate = %d, want %d", c.name, got, c.want)
		}
	}
	if got := Prorate(650000, end, start, start); got != 0 {
		t.Errorf("an empty period should prorate to nothing, got %d", got)
	}
}
