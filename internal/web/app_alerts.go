package web

import (
	"net/http"
	"strconv"

	"github.com/natnael-alemayehu/kiflet/internal/service"
)

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
	err = s.svc.Alerts.Acknowledge(r.Context(), principal(r), id)
	s.actionResult(w, r, "/app/alerts", err, "Alert dismissed.")
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
