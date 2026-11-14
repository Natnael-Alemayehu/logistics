package web

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/geo"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/internal/service"
)

// ---------------------------------------------------------------------------
// Reports (FR-034)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Alerts (FR-033)
// ---------------------------------------------------------------------------

func (s *Server) alertList(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	page = max(page, 1)
	f := service.AlertFilter{OpenOnly: q.Get("all") == "", Page: page, PerPage: 30}
	if sev := q.Get("severity"); sev != "" {
		f.Severities = []string{sev}
	}
	alerts, total, err := s.svc.Alerts.List(r.Context(), p.TenantID, f)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	pages := int((total + int64(f.PerPage) - 1) / int64(f.PerPage))
	v := s.view(w, r, "Alerts", "alerts", map[string]any{
		"Alerts": alerts, "Total": total, "Page": page, "Pages": pages, "All": !f.OpenOnly, "Severity": q.Get("severity"),
	})
	// Opening the list counts as having seen the alerts, so the badge clears.
	if err := s.svc.Alerts.MarkAllRead(r.Context(), p.TenantID); err == nil {
		v.Unread = 0
	}
	s.render(w, r, http.StatusOK, "app/alerts", v)
}

func (s *Server) alertAck(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	err = s.svc.Alerts.Acknowledge(r.Context(), principal(r), id)
	s.actionResult(w, r, "/app/alerts", err, "Alert dismissed.")
}

func (s *Server) alertsRead(w http.ResponseWriter, r *http.Request) {
	err := s.svc.Alerts.MarkAllRead(r.Context(), principal(r).TenantID)
	if r.Header.Get("Accept") == "application/json" {
		if err != nil {
			s.apiError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	s.actionResult(w, r, "/app/alerts", err, "")
}

// ---------------------------------------------------------------------------
// Company settings
// ---------------------------------------------------------------------------

func (s *Server) settingsData(r *http.Request) (map[string]any, error) {
	p := principal(r)
	tenant, err := s.svc.Tenants.Get(r.Context(), p.TenantID)
	if err != nil {
		return nil, err
	}
	usage, err := s.svc.Tenants.Usage(r.Context(), p.TenantID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"Tenant": tenant, "Usage": usage, "Towns": geo.Places(),
		"RetainDays": int(s.svc.Data.RetainShipments().Hours() / 24),
	}, nil
}

func (s *Server) settingsPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.settingsData(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	tenant := data["Tenant"].(db.Tenant)
	v := s.view(w, r, "Settings", "settings", data)
	v.Form = url.Values{
		"name":                  {tenant.Name},
		"contact_phone":         {deref(tenant.ContactPhone)},
		"contact_email":         {deref(tenant.ContactEmail)},
		"region_city":           {tenant.RegionCity},
		"max_active_per_driver": {strconv.Itoa(int(tenant.MaxActivePerDriver))},
		"tin":                   {deref(tenant.Tin)},
	}
	for name, on := range map[string]bool{
		"sms_on_created": tenant.SmsOnCreated, "sms_on_dispatched": tenant.SmsOnDispatched,
		"sms_on_arrived": tenant.SmsOnArrived, "sms_on_delivered": tenant.SmsOnDelivered,
		"sms_on_cancelled": tenant.SmsOnCancelled, "sms_on_delayed": tenant.SmsOnDelayed,
		"sms_driver_on_assign": tenant.SmsDriverOnAssign, "alert_on_delivery": tenant.AlertOnDelivery,
	} {
		if on {
			v.Form.Set(name, "on")
		}
	}
	s.render(w, r, http.StatusOK, "app/settings", v)
}

func (s *Server) settingsUpdate(w http.ResponseWriter, r *http.Request) {
	maxActive, _ := strconv.Atoi(r.PostFormValue("max_active_per_driver"))
	on := func(name string) bool { return r.PostFormValue(name) != "" }
	_, err := s.svc.Tenants.UpdateSettings(r.Context(), principal(r), service.SettingsInput{
		Name: r.PostFormValue("name"), TIN: r.PostFormValue("tin"), ContactPhone: r.PostFormValue("contact_phone"),
		ContactEmail: r.PostFormValue("contact_email"), RegionCity: r.PostFormValue("region_city"),
		MaxActivePerDriver: maxActive,
		SMSOnCreated:       on("sms_on_created"), SMSOnDispatched: on("sms_on_dispatched"),
		SMSOnArrived: on("sms_on_arrived"), SMSOnDelivered: on("sms_on_delivered"),
		SMSOnCancelled: on("sms_on_cancelled"), SMSOnDelayed: on("sms_on_delayed"),
		SMSDriverOnAssign: on("sms_driver_on_assign"), AlertOnDelivery: on("alert_on_delivery"),
	}, clientInfo(r))
	if err != nil {
		data, derr := s.settingsData(r)
		if derr != nil {
			s.fail(w, r, derr)
			return
		}
		if !s.formError(w, r, "app/settings", s.view(w, r, "Settings", "settings", data), err) {
			s.fail(w, r, err)
		}
		return
	}
	s.setFlash(w, "success", "Settings saved.")
	http.Redirect(w, r, "/app/settings", http.StatusSeeOther)
}

// smsLog lists the company's text messages (FR-030).
func (s *Server) smsLog(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	page = max(page, 1)
	const per = 50
	rows, total, err := s.svc.SMS.Log(r.Context(), p.TenantID, page, per)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var drivers any
	if s.mockSMS {
		if drivers, err = s.svc.Users.Drivers(r.Context(), p.TenantID, true, ""); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	v := s.view(w, r, "Text messages", "settings", map[string]any{
		"Messages": rows, "Total": total, "Page": page, "Pages": int((total + per - 1) / per),
		"Provider": s.svc.SMS.ProviderName(), "Drivers": drivers,
	})
	s.render(w, r, http.StatusOK, "app/sms", v)
}

// smsSimulate plays an inbound driver SMS through the real handler. It only
// exists with the mock provider, so a demo or test can exercise SMS reporting
// without a phone network.
func (s *Server) smsSimulate(w http.ResponseWriter, r *http.Request) {
	if !s.mockSMS {
		s.notFound(w, r)
		return
	}
	res, err := s.svc.SMS.Inbound(r.Context(), service.InboundSMS{From: r.PostFormValue("from"), Body: r.PostFormValue("body")})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	kind, msg := "success", res.Detail
	switch res.Outcome {
	case "bad_number", "unknown_sender", "refused":
		kind = "error"
	}
	if res.Reply != "" {
		msg += " Reply: “" + res.Reply + "”"
	}
	s.back(w, r, "/app/settings/sms", kind, msg)
}

// ---------------------------------------------------------------------------
// The signed-in user's account
// ---------------------------------------------------------------------------

// alertChoice is one alert type on the account page's email preferences.
type alertChoice struct {
	Type, Label string
	On          bool
}

func (s *Server) accountData(r *http.Request) (map[string]any, error) {
	p := principal(r)
	sessions, err := s.svc.Auth.Sessions(r.Context(), p)
	if err != nil {
		return nil, err
	}
	data := map[string]any{"Sessions": sessions, "Current": p.SessionID}
	if !p.IsPlatformAdmin() {
		u, err := s.svc.Users.Get(r.Context(), p.TenantID, p.UserID)
		if err != nil {
			return nil, err
		}
		choices := make([]alertChoice, 0, len(service.AlertTypes))
		for _, t := range service.AlertTypes {
			choices = append(choices, alertChoice{Type: t, Label: service.AlertLabel(t), On: slices.Contains(u.AlertEmails, t)})
		}
		data["Alerts"] = choices
	}
	return data, nil
}

// accountAlertEmails saves which alerts are emailed to the caller.
func (s *Server) accountAlertEmails(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	err := s.svc.Alerts.SetAlertEmails(r.Context(), principal(r), r.PostForm["alert"])
	msg := "Alert emails saved."
	if len(r.PostForm["alert"]) == 0 {
		msg = "No alerts will be emailed to you."
	}
	s.actionResult(w, r, "/app/account", err, msg)
}

func (s *Server) accountPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.accountData(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	p := principal(r)
	v := s.view(w, r, "Your account", "account", data)
	v.Form = url.Values{"full_name": {p.Name}, "phone": {p.Phone}, "email": {p.Email}}
	s.render(w, r, http.StatusOK, "app/account", v)
}

func (s *Server) accountUpdate(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	in := personInput(r)
	in.Role = ""
	if _, err := s.svc.Users.UpdateStaff(r.Context(), p, p.UserID, in, clientInfo(r)); err != nil {
		data, derr := s.accountData(r)
		if derr != nil {
			s.fail(w, r, derr)
			return
		}
		if !s.formError(w, r, "app/account", s.view(w, r, "Your account", "account", data), err) {
			s.fail(w, r, err)
		}
		return
	}
	s.setFlash(w, "success", "Your details are saved.")
	http.Redirect(w, r, "/app/account", http.StatusSeeOther)
}

func (s *Server) accountVerifyEmail(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	err := s.svc.Auth.ResendVerification(r.Context(), p)
	s.actionResult(w, r, "/app/account", err, "We've sent a new link to "+p.Email+". It works for 7 days.")
}

func (s *Server) sessionRevoke(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	p := principal(r)
	if id == p.SessionID {
		s.logout(w, r)
		return
	}
	err = s.svc.Auth.RevokeSession(r.Context(), p, id)
	s.actionResult(w, r, "/app/account", err, "That device has been signed out.")
}

func (s *Server) sessionRevokeOthers(w http.ResponseWriter, r *http.Request) {
	err := s.svc.Auth.RevokeOtherSessions(r.Context(), principal(r))
	s.actionResult(w, r, "/app/account", err, "Every other device has been signed out.")
}

// ---------------------------------------------------------------------------
// Audit log (NFR security: who changed what)
// ---------------------------------------------------------------------------

type auditRow struct {
	At      time.Time
	Who     string
	Action  string
	Entity  string
	Link    string
	Changes []auditChange
	IP      string
}

type auditChange struct{ Field, From, To string }

var auditActions = map[string]string{
	"login":                      "Signed in",
	"login_failed":               "Failed sign-in",
	"login_locked":               "Account locked after failed sign-ins",
	"password_changed":           "Changed their password",
	"pin_changed":                "Changed their PIN",
	"pin_reset":                  "Reset their PIN by SMS",
	"credential_reset":           "Issued a temporary sign-in",
	"driver_created":             "Added a driver",
	"driver_updated":             "Edited a driver",
	"staff_created":              "Added a team member",
	"staff_updated":              "Edited a team member",
	"user_deactivated":           "Deactivated an account",
	"user_reactivated":           "Reactivated an account",
	"vehicle_created":            "Added a vehicle",
	"vehicle_updated":            "Edited a vehicle",
	"settings_updated":           "Changed company settings",
	"company_signup":             "Created the company",
	"password_reset_requested":   "Asked for a password reset link",
	"password_reset":             "Reset their password by email",
	"email_verified":             "Confirmed their email address",
	"pod_accepted":               "Accepted a held delivery",
	"pod_dismissed":              "Dismissed a held delivery",
	"tenant_suspended":           "Suspended a company",
	"tenant_restored":            "Restored a company",
	"plan_chosen":                "Chose a plan",
	"payment_started":            "Started a payment",
	"invoice_paid":               "Invoice paid",
	"invoice_marked_paid":        "Recorded a bank transfer",
	"invoice_voided":             "Cancelled an invoice",
	"payment_refund_due":         "Payment to refund",
	"payment_refunded":           "Payment refunded",
	"subscription_renewed":       "Subscription renewed",
	"trial_extended":             "Extended a free trial",
	"enterprise_offer_set":       "Set Enterprise terms",
	"enterprise_offer_withdrawn": "Withdrew Enterprise terms",
	"drivers_messaged":           "Messaged every driver on the road",
	"logout":                     "Signed out",
	"user_anonymised":            "Removed a former member's details",
	"data_exported":              "Downloaded the company's data",
	"tenant_deleted":             "Deleted a company",
}

func (s *Server) userNames(ctx context.Context, tenantID pgtype.UUID) map[pgtype.UUID]string {
	out := s.driverNames(ctx, tenantID)
	if staff, err := s.svc.Users.Staff(ctx, tenantID); err == nil {
		for _, u := range staff {
			out[u.ID] = u.FullName
		}
	}
	return out
}

func (s *Server) auditLog(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	page = max(page, 1)
	const per = 50
	logs, err := s.svc.Audit.List(r.Context(), p.TenantID, page, per+1)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	more := len(logs) > per
	if more {
		logs = logs[:per]
	}
	names := s.userNames(r.Context(), p.TenantID)
	drivers := s.driverNames(r.Context(), p.TenantID)
	rows := make([]auditRow, 0, len(logs))
	for _, l := range logs {
		row := auditRow{At: l.CreatedAt.Time, Who: names[l.UserID], Action: auditActions[l.Action], Entity: l.EntityType, IP: deref(l.IpAddress)}
		if row.Who == "" {
			row.Who = "System"
		}
		if row.Action == "" {
			row.Action = strings.ReplaceAll(l.Action, "_", " ")
		}
		if l.EntityID.Valid {
			switch l.EntityType {
			case "user":
				if n, ok := names[l.EntityID]; ok {
					row.Entity = n
				}
				if _, isDriver := drivers[l.EntityID]; isDriver {
					row.Link = "/app/drivers/" + l.EntityID.String()
				}
			case "vehicle":
				row.Link = "/app/vehicles/" + l.EntityID.String()
				row.Entity = "vehicle"
			case "shipment":
				row.Link = "/app/shipments/" + l.EntityID.String()
			}
		}
		row.Changes = auditDiff(l.OldValue, l.NewValue)
		rows = append(rows, row)
	}
	v := s.view(w, r, "Audit log", "settings", map[string]any{"Rows": rows, "Page": page, "More": more})
	s.render(w, r, http.StatusOK, "app/audit", v)
}

// auditDiff lists the fields that differ between the old and new values.
func auditDiff(oldRaw, newRaw []byte) []auditChange {
	var before, after map[string]any
	_ = json.Unmarshal(oldRaw, &before)
	_ = json.Unmarshal(newRaw, &after)
	var out []auditChange
	for k, nv := range after {
		ov, had := before[k]
		if had && fmt.Sprint(ov) == fmt.Sprint(nv) {
			continue
		}
		c := auditChange{Field: strings.ReplaceAll(k, "_", " "), To: fmt.Sprint(nv)}
		if had {
			c.From = fmt.Sprint(ov)
		}
		out = append(out, c)
	}
	return out
}

// ---------------------------------------------------------------------------
// Platform operator
// ---------------------------------------------------------------------------

func (s *Server) platform(w http.ResponseWriter, r *http.Request) {
	tenants, err := s.svc.Tenants.All(r.Context(), principal(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var drivers, shipments int64
	for _, t := range tenants {
		drivers += t.Drivers
		shipments += t.Shipments30d
	}
	v := s.view(w, r, "Companies", "platform", map[string]any{
		"Tenants": tenants, "Plans": model.Plans, "Drivers": drivers, "Shipments": shipments,
	})
	s.render(w, r, http.StatusOK, "app/platform", v)
}

// platformTenant is one company as the platform admin manages it: its plan
// and invoices, questions, and the actions on it (AC-049.3).
func (s *Server) platformTenant(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	p := principal(r)
	tenants, err := s.svc.Tenants.All(r.Context(), p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	i := slices.IndexFunc(tenants, func(t db.ListTenantsWithStatsRow) bool { return t.ID == id })
	if i < 0 || id == p.TenantID {
		s.notFound(w, r)
		return
	}
	invoices, err := s.svc.Billing.CompanyInvoices(r.Context(), p, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	tickets, err := s.svc.Support.ForCompany(r.Context(), p, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	usage, err := s.svc.Tenants.Usage(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(w, r, tenants[i].Name, "platform", map[string]any{
		"T": tenants[i], "Invoices": invoices, "Tickets": tickets, "Usage": usage,
	})
	s.render(w, r, http.StatusOK, "app/platform_tenant", v)
}

func (s *Server) platformTenantActive(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	active, _ := strconv.ParseBool(r.PostFormValue("active"))
	err = s.svc.Tenants.SetActive(r.Context(), principal(r), id, active, clientInfo(r))
	msg := "Company suspended. Its users are signed out."
	if active {
		msg = "Company restored."
	}
	s.actionResult(w, r, "/app/platform", err, msg)
}

// tenantPath is a company's page for the platform admin.
func tenantPath(id pgtype.UUID) string { return "/app/platform/tenants/" + id.String() }
