package web

import (
	"encoding/csv"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/geo"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// chartBar is one column of the deliveries chart, in SVG plot units. The
// on-time part sits at the baseline in the accent colour and the rest of the
// day's deliveries stack above it in grey, separated by a 2px gap.
type chartBar struct {
	SlotX, SlotW float64 // the hover target: the whole slot
	CenterX      float64
	OnTimePath   string
	OtherPath    string
	TopY         float64
	Label        string // x-axis label, on every few bars
	Tip          string // "title|line|line" for the tooltip
	Day          time.Time
	Delivered    int64
	OnTime       int64
	Peak         bool
}

type chart struct {
	Bars   []chartBar
	Ticks  []chartTick
	Width  float64
	Height float64
	// Group is what one column covers: groupDay, groupWeek or groupMonth.
	Group string
	Empty bool
}

type chartTick struct {
	Y     float64
	Value int64
}

const chartW, chartH = 720.0, 200.0

// What one column of the deliveries chart covers (AC-034.1).
const (
	groupDay   = "day"
	groupWeek  = "week"
	groupMonth = "month"
)

// maxDayBars is the most days drawn one column each; beyond, columns get
// too thin to read and the chart shows weeks.
const maxDayBars = 92

// chartGroup is the grouping asked for, or the readable one for the range.
func chartGroup(asked string, days int) string {
	switch {
	case asked == groupWeek, asked == groupMonth:
		return asked
	case days <= maxDayBars:
		return groupDay
	default:
		return groupWeek
	}
}

// group folds days into calendar weeks, from Monday, or calendar months. A
// group starts at its first day in the range, so the first and the last
// can be partial.
func group(days []service.DailyDeliveries, by string) []service.DailyDeliveries {
	if by == groupDay {
		return days
	}
	key := func(t time.Time) int {
		if by == groupMonth {
			return t.Year()*100 + int(t.Month())
		}
		y, w := t.ISOWeek()
		return y*100 + w
	}
	var out []service.DailyDeliveries
	last := -1
	for _, d := range days {
		if k := key(d.Day); k != last {
			out = append(out, service.DailyDeliveries{Day: d.Day})
			last = k
		}
		g := &out[len(out)-1]
		g.Delivered += d.Delivered
		g.OnTime += d.OnTime
	}
	return out
}

// groupLabel names the period a column covers, for the tooltip and table.
func groupLabel(day time.Time, by string) string {
	switch by {
	case groupMonth:
		return day.Format("January 2006")
	case groupWeek:
		return "Week of " + day.Format("Mon 2 Jan 2006")
	}
	return day.Format("Mon 2 Jan 2006")
}

// barPath draws a column with a rounded data end and a square baseline.
func barPath(x, y, w, h float64, round bool) string {
	if h <= 0 {
		return ""
	}
	r := 0.0
	if round {
		r = math.Min(4, math.Min(w/2, h))
	}
	return fmt.Sprintf("M%.1f %.1fV%.1fQ%.1f %.1f %.1f %.1fH%.1fQ%.1f %.1f %.1f %.1fV%.1fZ",
		x, y+h, y+r, x, y, x+r, y, x+w-r, x+w, y, x+w, y+r, y+h)
}

func buildChart(days []service.DailyDeliveries, by string) chart {
	c := chart{Width: chartW, Height: chartH, Group: chartGroup(by, len(days))}
	days = group(days, c.Group)
	var peak int64
	peakAt := -1
	for i, d := range days {
		if d.Delivered > peak {
			peak, peakAt = d.Delivered, i
		}
	}
	c.Empty = peak == 0
	top := niceCeil(peak)
	// Whole-number ticks at even steps: 10 is 0, 2, 4…, not 0, 2, 5, 7.
	steps := int64(4)
	if top%4 != 0 {
		steps = 5
	}
	for i := int64(0); i <= steps; i++ {
		v := top * i / steps
		c.Ticks = append(c.Ticks, chartTick{Y: chartH - float64(v)/float64(top)*chartH, Value: v})
	}
	if len(days) == 0 {
		return c
	}
	const gap = 2.0
	slot := chartW / float64(len(days))
	bw := math.Max(1, math.Min(24, slot-gap))
	every := int(math.Ceil(float64(len(days)) / 8))
	for i, d := range days {
		x := float64(i)*slot + (slot-bw)/2
		hAll := float64(d.Delivered) / float64(top) * chartH
		hOn := float64(d.OnTime) / float64(top) * chartH
		hOther := hAll - hOn
		b := chartBar{
			SlotX: float64(i) * slot, SlotW: slot, CenterX: float64(i)*slot + slot/2, TopY: chartH - hAll,
			Day: d.Day, Delivered: d.Delivered, OnTime: d.OnTime, Peak: i == peakAt,
		}
		switch {
		case hOther > 0 && hOn > 0:
			b.OnTimePath = barPath(x, chartH-hOn, bw, hOn, false)
			b.OtherPath = barPath(x, chartH-hAll, bw, math.Max(1, hOther-gap), true)
		case hOn > 0:
			b.OnTimePath = barPath(x, chartH-hOn, bw, hOn, true)
		default:
			b.OtherPath = barPath(x, chartH-hAll, bw, hAll, true)
		}
		b.Tip = fmt.Sprintf("%s|%d delivered|%d on time", groupLabel(d.Day, c.Group), d.Delivered, d.OnTime)
		if i%every == 0 {
			b.Label = d.Day.Format("2 Jan")
			if c.Group == groupMonth {
				b.Label = d.Day.Format("Jan 2006")
			}
		}
		c.Bars = append(c.Bars, b)
	}
	return c
}

// niceCeil rounds up to 1, 2 or 5 times a power of ten, for axis ticks.
func niceCeil(n int64) int64 {
	if n <= 4 {
		return 4
	}
	p := int64(1)
	for p*10 <= n {
		p *= 10
	}
	for _, m := range []int64{1, 2, 4, 5, 10} {
		if m*p >= n {
			return m * p
		}
	}
	return 10 * p
}

func (s *Server) reports(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	q := r.URL.Query()
	from, to := s.svc.Reports.Range(q.Get("from"), q.Get("to"))
	rf := service.ReportFilter{From: from, To: to, Origin: service.Town(q.Get("origin")), Destination: service.Town(q.Get("destination"))}
	drivers, err := s.svc.Users.Drivers(r.Context(), p.TenantID, false, "")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	driverName := ""
	if id, err := parseID(q.Get("driver")); err == nil {
		for _, d := range drivers {
			if d.ID == id {
				rf.DriverID, driverName = id, d.FullName
			}
		}
	}
	rep, err := s.svc.Reports.Build(r.Context(), p.TenantID, rf)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	by := chartGroup(q.Get("group"), len(rep.Daily))
	if q.Get("format") == "csv" {
		s.reportCSV(w, rep, q.Get("section"), by)
		return
	}
	last := to.AddDate(0, 0, -1)

	// Links keep the filters: the date presets change only the dates, and
	// each CSV covers what the page shows.
	keep := url.Values{}
	keep.Set("from", from.Format("2006-01-02"))
	keep.Set("to", last.Format("2006-01-02"))
	if rf.DriverID.Valid {
		keep.Set("driver", rf.DriverID.String())
	}
	if rf.Origin != "" {
		keep.Set("origin", rf.Origin)
	}
	if rf.Destination != "" {
		keep.Set("destination", rf.Destination)
	}
	if g := q.Get("group"); g == groupDay || g == groupWeek || g == groupMonth {
		keep.Set("group", g)
	}
	link := func(set ...string) string {
		v := url.Values{}
		for k, vs := range keep {
			v[k] = vs
		}
		for i := 0; i+1 < len(set); i += 2 {
			v.Set(set[i], set[i+1])
		}
		return "/app/reports?" + v.Encode()
	}
	csvLinks := map[string]string{}
	for _, section := range []string{"daily", "drivers", "routes", "customers", "cancellations"} {
		csvLinks[section] = link("format", "csv", "section", section)
	}
	presets := reportPresets(s.cfg.Timezone)
	for i := range presets {
		presets[i].Href = link("from", presets[i].From, "to", presets[i].To)
	}
	v := s.view(w, r, "Reports", "reports", map[string]any{
		"R": rep, "Chart": buildChart(rep.Daily, by), "FromInput": from.Format("2006-01-02"), "ToInput": last.Format("2006-01-02"),
		"Last": last, "OnTime": service.OnTimeRate(rep.Summary.OnTime, rep.Summary.WithEta),
		"PODRate": ratio(rep.Summary.WithPod, rep.Summary.Delivered),
		"Presets": presets, "CSV": csvLinks, "Drivers": drivers, "DriverName": driverName,
		"Group": q.Get("group"), "Towns": geo.Places(),
		"Clear": "/app/reports?" + url.Values{"from": {keep.Get("from")}, "to": {keep.Get("to")}}.Encode(),
	})
	s.render(w, r, http.StatusOK, "app/reports", v)
}

func ratio(a, b int64) float64 {
	if b == 0 {
		return -1
	}
	return float64(a) / float64(b)
}

type preset struct{ Label, From, To, Href string }

func reportPresets(loc *time.Location) []preset {
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	f := func(t time.Time) string { return t.Format("2006-01-02") }
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	prevStart := monthStart.AddDate(0, -1, 0)
	return []preset{
		{"Last 7 days", f(today.AddDate(0, 0, -6)), f(today), ""},
		{"Last 30 days", f(today.AddDate(0, 0, -29)), f(today), ""},
		{"This month", f(monthStart), f(today), ""},
		{"Last month", f(prevStart), f(monthStart.AddDate(0, 0, -1)), ""},
		{"Last 90 days", f(today.AddDate(0, 0, -89)), f(today), ""},
		{"Last 12 months", f(today.AddDate(-1, 0, 1)), f(today), ""},
	}
}

func (s *Server) reportCSV(w http.ResponseWriter, rep *service.Report, section, by string) {
	name := map[string]string{"routes": "routes", "customers": "customers", "daily": "daily", "cancellations": "cancellations"}[section]
	if name == "" {
		name = "drivers"
	}
	file := name
	if name == "daily" {
		file = "deliveries-per-" + by
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="report-%s-%s-to-%s.csv"`,
		file, rep.From.Format("2006-01-02"), rep.To.AddDate(0, 0, -1).Format("2006-01-02")))
	_, _ = w.Write([]byte("\xEF\xBB\xBF"))
	cw := csv.NewWriter(w)
	rate := func(a, b int64) string {
		if b == 0 {
			return ""
		}
		return strconv.FormatFloat(float64(a)/float64(b)*100, 'f', 1, 64)
	}
	hrs := func(h float64) string {
		if h <= 0 {
			return ""
		}
		return strconv.FormatFloat(h, 'f', 1, 64)
	}
	switch name {
	case "routes":
		_ = cw.Write([]string{"Origin", "Destination", "Delivered", "On time %", "Average transit (h)"})
		for _, x := range rep.Routes {
			_ = cw.Write([]string{x.OriginCity, x.DestinationCity, strconv.FormatInt(x.Delivered, 10), rate(x.OnTime, x.WithEta), hrs(x.AvgTransitHours)})
		}
	case "customers":
		_ = cw.Write([]string{"Customer", "Phone", "Shipments", "Delivered", "Last shipment"})
		for _, x := range rep.Customers {
			_ = cw.Write([]string{csvSafe(x.CustomerName), x.CustomerPhone, strconv.FormatInt(x.Shipments, 10), strconv.FormatInt(x.Delivered, 10),
				x.LastShipmentAt.Time.In(s.cfg.Timezone).Format("2006-01-02")})
		}
	case "cancellations":
		_ = cw.Write([]string{"Reason", "Cancelled", "After departure"})
		for _, x := range rep.Cancellations {
			_ = cw.Write([]string{csvSafe(model.ReasonLabel(x.Reason)), strconv.FormatInt(x.Total, 10), strconv.FormatInt(x.OnRoad, 10)})
		}
	case "daily":
		// The deliveries as the chart groups them: by day, week or month.
		head := map[string]string{groupDay: "Day", groupWeek: "Week starting", groupMonth: "Month"}[by]
		_ = cw.Write([]string{head, "Delivered", "On time"})
		for _, x := range group(rep.Daily, by) {
			period := x.Day.Format("2006-01-02")
			if by == groupMonth {
				period = x.Day.Format("2006-01")
			}
			_ = cw.Write([]string{period, strconv.FormatInt(x.Delivered, 10), strconv.FormatInt(x.OnTime, 10)})
		}
	default:
		_ = cw.Write([]string{"Driver", "Delivered", "On time %", "Average transit (h)", "Issues"})
		for _, x := range rep.Drivers {
			_ = cw.Write([]string{csvSafe(x.DriverName), strconv.FormatInt(x.Delivered, 10), rate(x.OnTime, x.WithEta), hrs(x.AvgTransitHours), strconv.FormatInt(x.Issues, 10)})
		}
	}
	cw.Flush()
}
