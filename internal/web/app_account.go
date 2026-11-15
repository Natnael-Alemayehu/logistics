package web

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/natnael-alemayehu/kiflet/internal/service"
)

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
