package web

import (
	"fmt"
	"net/http"

	"github.com/natnael-alemayehu/logistics/internal/service"
)

// ---------------------------------------------------------------------------
// Help: a company's questions to the people running Menged, and the
// platform's inbox for them (AC-049.5)
// ---------------------------------------------------------------------------

func (s *Server) helpList(w http.ResponseWriter, r *http.Request) {
	tickets, err := s.svc.Support.List(r.Context(), principal(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(w, r, "Help", "help", map[string]any{"Tickets": tickets})
	s.render(w, r, http.StatusOK, "app/help", v)
}

func (s *Server) helpNew(w http.ResponseWriter, r *http.Request) {
	v := s.view(w, r, "Ask for help", "help", nil)
	v.Form.Set("subject", r.URL.Query().Get("subject"))
	s.render(w, r, http.StatusOK, "app/help_new", v)
}

func (s *Server) helpCreate(w http.ResponseWriter, r *http.Request) {
	id, err := s.svc.Support.Open(r.Context(), principal(r), r.PostFormValue("subject"), r.PostFormValue("body"))
	if err != nil {
		if s.formError(w, r, "app/help_new", s.view(w, r, "Ask for help", "help", nil), err) {
			return
		}
		s.fail(w, r, err)
		return
	}
	s.setFlash(w, "success", "Your question was sent. You'll get an email when it's answered.")
	http.Redirect(w, r, "/app/help/"+id.String(), http.StatusSeeOther)
}

// ticketPage shows a ticket to either side: the company under Help, the
// platform admin under Support.
func (s *Server) ticketPage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	p := principal(r)
	t, err := s.svc.Support.Get(r.Context(), p, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	nav, base := "help", "/app/help/"
	if p.IsPlatformAdmin() {
		nav, base = "platform-support", "/app/platform/support/"
	}
	v := s.view(w, r, fmt.Sprintf("#%d %s", t.Number, t.Subject), nav, map[string]any{"Ticket": t, "Base": base + id.String()})
	s.render(w, r, http.StatusOK, "app/ticket", v)
}

func (s *Server) ticketReply(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	p := principal(r)
	err = s.svc.Support.Reply(r.Context(), p, id, r.PostFormValue("body"))
	msg := "Your reply was sent."
	if p.IsPlatformAdmin() {
		msg = "Answer sent, and emailed to the company."
	}
	s.actionResult(w, r, ticketPath(p, id.String()), err, msg)
}

func (s *Server) ticketClose(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	p := principal(r)
	err = s.svc.Support.Close(r.Context(), p, id)
	s.actionResult(w, r, ticketPath(p, id.String()), err, "The ticket is closed. Writing to it again reopens it.")
}

func ticketPath(p *service.Principal, id string) string {
	if p.IsPlatformAdmin() {
		return "/app/platform/support/" + id
	}
	return "/app/help/" + id
}

// supportInbox is every company's tickets, those waiting for an answer
// first.
func (s *Server) supportInbox(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = service.TicketOpen
	}
	tickets, err := s.svc.Support.Inbox(r.Context(), principal(r), status)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(w, r, "Support", "platform-support", map[string]any{"Tickets": tickets, "Status": status})
	s.render(w, r, http.StatusOK, "app/platform_support", v)
}
