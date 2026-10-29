package web

import (
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/config"
	"github.com/natnael-alemayehu/logistics/internal/service"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	loc, err := time.LoadLocation("Africa/Addis_Ababa")
	if err != nil {
		t.Fatal(err)
	}
	return &Server{cfg: &config.Config{Timezone: loc}, version: "test"}
}

// Every page template must parse together with the layouts and partials.
func TestTemplatesParse(t *testing.T) {
	s := testServer(t)
	tpl, err := fs.Sub(embedded, "templates")
	if err != nil {
		t.Fatal(err)
	}
	r, err := newRenderer(tpl, s.funcs(), false)
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range []string{
		"app/dashboard", "app/shipments", "app/shipment", "app/shipment_form", "app/waybill",
		"app/drivers", "app/driver", "app/driver_form", "app/credential", "app/team", "app/team_form",
		"app/vehicles", "app/vehicle", "app/vehicle_form", "app/reports", "app/alerts", "app/settings",
		"app/sms", "app/audit", "app/account", "app/password", "app/platform", "app/error",
		"public/landing", "public/login", "public/signup", "public/track_form", "public/track",
		"public/track_phone", "public/track_mine", "public/confirm", "public/error",
	} {
		if _, ok := r.pages[page]; !ok {
			t.Errorf("missing page %s", page)
		}
	}
}

func TestCSVSafeDefusesFormulas(t *testing.T) {
	for in, want := range map[string]string{
		"=HYPERLINK(\"x\")": "'=HYPERLINK(\"x\")",
		"+251911":           "'+251911",
		"@SUM(A1)":          "'@SUM(A1)",
		"Abebe Kebede":      "Abebe Kebede",
		"":                  "",
	} {
		if got := csvSafe(in); got != want {
			t.Errorf("csvSafe(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildChartFoldsLongRangesIntoWeeks(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var days []service.DailyDeliveries
	for i := range 120 {
		days = append(days, service.DailyDeliveries{Day: start.AddDate(0, 0, i), Delivered: 2, OnTime: 1})
	}
	c := buildChart(days)
	if !c.Weekly || len(c.Bars) != 18 {
		t.Fatalf("weekly=%v bars=%d, want weekly with 18 bars", c.Weekly, len(c.Bars))
	}
	if c.Bars[0].Delivered != 14 || c.Bars[0].OnTime != 7 {
		t.Errorf("first week = %d/%d, want 14/7", c.Bars[0].Delivered, c.Bars[0].OnTime)
	}
	for _, b := range c.Bars {
		if b.OnTimePath == "" || b.OtherPath == "" {
			t.Fatal("both segments should be drawn")
		}
	}
}

func TestBuildChartCapsBarWidth(t *testing.T) {
	days := []service.DailyDeliveries{{Day: time.Now(), Delivered: 3, OnTime: 3}}
	c := buildChart(days)
	if !strings.Contains(c.Bars[0].OnTimePath, "H") || c.Bars[0].OtherPath != "" {
		t.Fatalf("unexpected paths %+v", c.Bars[0])
	}
	// One day across the whole plot: the column is still at most 24 units wide.
	b := c.Bars[0]
	if b.SlotW < 700 {
		t.Fatalf("slot %v", b.SlotW)
	}
}

func TestNiceCeil(t *testing.T) {
	for in, want := range map[int64]int64{0: 4, 3: 4, 7: 10, 11: 20, 38: 40, 41: 50, 120: 200} {
		if got := niceCeil(in); got != want {
			t.Errorf("niceCeil(%d) = %d, want %d", in, got, want)
		}
	}
}
