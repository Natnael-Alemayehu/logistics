package web

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// The alerts page's tabs.
const (
	alertsOpen     = ""
	alertsResolved = "resolved"
	alertsAll      = "all"
)

// alertRecipient is one member of staff as "who gets them" shows them.
type alertRecipient struct {
	User  db.User
	Email string // a service.Channel*
	SMS   string
}

func (s *Server) alertRecipientList(r *http.Request) ([]alertRecipient, error) {
	staff, err := s.svc.Users.Staff(r.Context(), principal(r).TenantID)
	if err != nil {
		return nil, err
	}
	var out []alertRecipient
	for _, u := range staff {
		if u.Role == model.RoleDriver || !u.IsActive || u.AnonymisedAt.Valid {
			continue
		}
		out = append(out, alertRecipient{User: u, Email: service.ChannelOf(u.AlertEmails), SMS: service.ChannelOf(u.AlertSms)})
	}
	return out, nil
}

func (s *Server) alertList(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	page = max(page, 1)
	tab := q.Get("tab")
	if q.Get("all") != "" { // the old address
		tab = alertsAll
	}
	f := service.AlertFilter{OpenOnly: tab == alertsOpen, Page: page, PerPage: 30, Type: q.Get("type")}
	if f.Type != "" && !slices.Contains(service.AlertTypes, f.Type) {
		f.Type = ""
	}
	if sev := q.Get("severity"); sev != "" {
		f.Severities = []string{sev}
	}
	if tab == alertsResolved {
		f.ResolvedSince = s.svc.Alerts.TodayStart()
	}
	alerts, total, err := s.svc.Alerts.List(r.Context(), p.TenantID, f)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	// The tab counts, and the morning's resolved alerts beside the open ones.
	counts := map[string]int64{}
	_, counts[alertsOpen], err = s.svc.Alerts.List(r.Context(), p.TenantID, service.AlertFilter{OpenOnly: true, PerPage: 1})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	resolved, nResolved, err := s.svc.Alerts.List(r.Context(), p.TenantID, service.AlertFilter{ResolvedSince: s.svc.Alerts.TodayStart(), PerPage: 8})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	counts[alertsResolved] = nResolved
	rules, err := s.svc.Alerts.Rules(r.Context(), p.TenantID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	recipients, err := s.alertRecipientList(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	pages := int((total + int64(f.PerPage) - 1) / int64(f.PerPage))
	v := s.view(w, r, "Alerts", "alerts", map[string]any{
		"Alerts": alerts, "Total": total, "Page": page, "Pages": pages, "Tab": tab, "Type": f.Type,
		"Counts": counts, "Resolved": resolved, "Rules": rules, "Recipients": recipients, "Types": service.AlertTypes,
		"Severity": q.Get("severity"),
	})
	// Opening the list counts as having seen the alerts, so this person's
	// badge clears.
	if err := s.svc.Alerts.MarkSeen(r.Context(), p); err == nil {
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
	back := "/app/alerts"
	if b := r.PostFormValue("back"); strings.HasPrefix(b, "/app/") && !strings.HasPrefix(b, "//") {
		back = b
	}
	err = s.svc.Alerts.Acknowledge(r.Context(), principal(r), id, r.PostFormValue("note"))
	s.actionResult(w, r, back, err, "Alert resolved.")
}

func (s *Server) alertsRead(w http.ResponseWriter, r *http.Request) {
	err := s.svc.Alerts.MarkSeen(r.Context(), principal(r))
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

// alertRulesUpdate saves when the company is alerted.
func (s *Server) alertRulesUpdate(w http.ResponseWriter, r *http.Request) {
	num := func(name string) int {
		n, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue(name)))
		if err != nil {
			return -1
		}
		return n
	}
	on := func(name string) bool { return r.PostFormValue(name) != "" }
	in := service.AlertRules{
		ProblemsOn: on("problems_on"),
		LateOn:     on("late_on"), LateMinutes: num("late_minutes"),
		OfflineOn: on("offline_on"), OfflineMinutes: num("offline_minutes"),
		StopOn: on("stop_on"), StopMinutes: num("stop_minutes"),
		RouteOn: on("route_on"), RouteKm: num("route_km"),
	}
	err := s.svc.Alerts.SetRules(r.Context(), principal(r), in, clientInfo(r))
	s.actionResult(w, r, "/app/alerts#rules", err, "Alert rules saved.")
}

// alertRecipients is the form for who gets alerts, and how.
func (s *Server) alertRecipients(w http.ResponseWriter, r *http.Request) {
	people, err := s.alertRecipientList(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "app/alert_recipients", s.view(w, r, "Who gets alerts", "alerts", map[string]any{"People": people}))
}

func (s *Server) alertRecipientsUpdate(w http.ResponseWriter, r *http.Request) {
	people, err := s.alertRecipientList(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var in []service.Recipient
	for _, p := range people {
		id := p.User.ID.String()
		in = append(in, service.Recipient{UserID: p.User.ID, Email: r.PostFormValue("email_" + id), SMS: r.PostFormValue("sms_" + id)})
	}
	err = s.svc.Alerts.SetRecipients(r.Context(), principal(r), in, clientInfo(r))
	if err != nil {
		v := s.view(w, r, "Who gets alerts", "alerts", map[string]any{"People": people})
		if !s.formError(w, r, "app/alert_recipients", v, err) {
			s.fail(w, r, err)
		}
		return
	}
	s.setFlash(w, "success", "Saved who gets alerts.")
	http.Redirect(w, r, "/app/alerts", http.StatusSeeOther)
}
