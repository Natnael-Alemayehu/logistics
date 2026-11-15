package web

import (
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/config"
	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/service"
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
	var pages []string
	for _, dir := range []string{"app", "public"} {
		files, err := fs.Glob(tpl, dir+"/*.html")
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			pages = append(pages, strings.TrimSuffix(f, path.Ext(f)))
		}
	}
	if len(pages) < 40 {
		t.Fatalf("found only %d pages", len(pages))
	}
	for _, page := range pages {
		if _, ok := r.pages[page]; !ok {
			t.Errorf("page %s was not loaded", page)
		}
	}
}

func TestDeviceNamesBrowsers(t *testing.T) {
	for ua, want := range map[string]string{
		"Mozilla/5.0 (Linux; Android 13; SM-A135F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36":                     "Chrome on Android",
		"Mozilla/5.0 (Linux; Android 13; SM-A135F) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/23.0 Chrome/115.0.0.0 Mobile Safari/537.36": "Samsung Internet on Android",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Mobile/15E148 Safari/604.1":    "Safari on iPhone",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0":              "Edge on Windows",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:121.0) Gecko/20100101 Firefox/121.0":                                                       "Firefox on Mac",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/120.0.0.0 Safari/537.36":                              "Chrome on Linux",
		"curl/8.4.0": "curl/8.4.0",
	} {
		if got := device(&ua); got != want {
			t.Errorf("device(%q) = %q, want %q", ua, got, want)
		}
	}
	if got := device(nil); got != "" {
		t.Errorf("device(nil) = %q, want nothing", got)
	}
}

func TestNewestFirstCapsTheList(t *testing.T) {
	newestFirst := testServer(t).funcs()["newestFirst"].(func(int, []db.ListShipmentRouteRow) []db.ListShipmentRouteRow)
	var pts []db.ListShipmentRouteRow
	for i := range 5 {
		pts = append(pts, db.ListShipmentRouteRow{Latitude: float64(i)})
	}
	var got []float64
	for _, p := range newestFirst(3, pts) {
		got = append(got, p.Latitude)
	}
	if fmt.Sprint(got) != "[4 3 2]" {
		t.Errorf("newestFirst(3) = %v, want [4 3 2]", got)
	}
	if n := len(newestFirst(10, pts)); n != 5 {
		t.Errorf("newestFirst(10) of 5 positions gave %d", n)
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

// days returns n days of two deliveries, one on time, from 1 January 2026,
// a Thursday.
func days(n int) []service.DailyDeliveries {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var out []service.DailyDeliveries
	for i := range n {
		out = append(out, service.DailyDeliveries{Day: start.AddDate(0, 0, i), Delivered: 2, OnTime: 1})
	}
	return out
}

func TestBuildChartFoldsLongRangesIntoWeeks(t *testing.T) {
	c := buildChart(days(120), "")
	// Thursday 1 January to Thursday 30 April: a partial first and last
	// week around sixteen whole ones.
	if c.Group != groupWeek || len(c.Bars) != 18 {
		t.Fatalf("group=%s bars=%d, want weeks with 18 bars", c.Group, len(c.Bars))
	}
	if c.Bars[0].Delivered != 8 || c.Bars[1].Delivered != 14 || c.Bars[1].OnTime != 7 {
		t.Errorf("weeks = %d, %d/%d; want 8 (Thu–Sun), then 14/7", c.Bars[0].Delivered, c.Bars[1].Delivered, c.Bars[1].OnTime)
	}
	if !c.Bars[1].Day.Equal(time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("second week starts %s, want Monday 5 January", c.Bars[1].Day)
	}
	for _, b := range c.Bars {
		if b.OnTimePath == "" || b.OtherPath == "" {
			t.Fatal("both segments should be drawn")
		}
	}
}

func TestBuildChartGroupsAsAsked(t *testing.T) {
	if c := buildChart(days(30), ""); c.Group != groupDay || len(c.Bars) != 30 {
		t.Errorf("30 days: group=%s bars=%d, want one bar a day", c.Group, len(c.Bars))
	}
	if c := buildChart(days(200), groupDay); c.Group != groupWeek {
		t.Errorf("200 days per day: group=%s, want weeks: too many days to draw", c.Group)
	}
	c := buildChart(days(120), groupMonth)
	if c.Group != groupMonth || len(c.Bars) != 4 {
		t.Fatalf("group=%s bars=%d, want 4 months", c.Group, len(c.Bars))
	}
	if c.Bars[1].Delivered != 56 || !strings.HasPrefix(c.Bars[1].Tip, "February 2026|") {
		t.Errorf("February = %d %q, want 56 deliveries", c.Bars[1].Delivered, c.Bars[1].Tip)
	}
}

func TestBuildChartCapsBarWidth(t *testing.T) {
	days := []service.DailyDeliveries{{Day: time.Now(), Delivered: 3, OnTime: 3}}
	c := buildChart(days, "")
	if !strings.Contains(c.Bars[0].OnTimePath, "H") || c.Bars[0].OtherPath != "" {
		t.Fatalf("unexpected paths %+v", c.Bars[0])
	}
	// One day across the whole plot: the column is still at most 24 units wide.
	b := c.Bars[0]
	if b.SlotW < 700 {
		t.Fatalf("slot %v", b.SlotW)
	}
}

func TestChartTicksAreEvenWholeNumbers(t *testing.T) {
	for peak, want := range map[int64]string{3: "0 1 2 3 4", 9: "0 2 4 6 8 10", 17: "0 5 10 15 20", 41: "0 10 20 30 40 50"} {
		c := buildChart([]service.DailyDeliveries{{Day: time.Now(), Delivered: peak}}, "")
		var got []string
		for _, tk := range c.Ticks {
			got = append(got, strconv.FormatInt(tk.Value, 10))
		}
		if strings.Join(got, " ") != want {
			t.Errorf("peak %d: ticks %v, want %s", peak, got, want)
		}
	}
}

func TestNiceCeil(t *testing.T) {
	for in, want := range map[int64]int64{0: 4, 3: 4, 7: 10, 11: 20, 38: 40, 41: 50, 120: 200} {
		if got := niceCeil(in); got != want {
			t.Errorf("niceCeil(%d) = %d, want %d", in, got, want)
		}
	}
}
