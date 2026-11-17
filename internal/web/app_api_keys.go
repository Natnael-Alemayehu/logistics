package web

import (
	"net/http"

	"github.com/natnael-alemayehu/kiflet/internal/db"
)

// The keys of the public API (FR-046), on a page of their own under
// Settings. A key is shown once, on the page that made it.

func (s *Server) apiKeysView(w http.ResponseWriter, r *http.Request, created string, key db.ApiKey) (*View, error) {
	keys, err := s.svc.API.Keys(r.Context(), principal(r).TenantID)
	if err != nil {
		return nil, err
	}
	return s.view(w, r, "API keys", "settings", map[string]any{
		"Keys": keys, "Created": created, "CreatedKey": key, "BaseURL": s.cfg.PublicURL + "/api/v1",
	}), nil
}

func (s *Server) apiKeysPage(w http.ResponseWriter, r *http.Request) {
	v, err := s.apiKeysView(w, r, "", db.ApiKey{})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "app/api_keys", v)
}

// apiKeyCreate makes a key and shows it in the page's answer, not after a
// redirect: it isn't kept, so it can't be shown again.
func (s *Server) apiKeyCreate(w http.ResponseWriter, r *http.Request) {
	key, k, err := s.svc.API.CreateKey(r.Context(), principal(r), r.PostFormValue("name"), clientInfo(r))
	v, verr := s.apiKeysView(w, r, key, k)
	if verr != nil {
		s.fail(w, r, verr)
		return
	}
	if err != nil {
		if !s.formError(w, r, "app/api_keys", v, err) {
			s.fail(w, r, err)
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	s.render(w, r, http.StatusOK, "app/api_keys", v)
}

func (s *Server) apiKeyRevoke(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	if err := s.svc.API.RevokeKey(r.Context(), principal(r), id, clientInfo(r)); err != nil {
		s.fail(w, r, err)
		return
	}
	s.setFlash(w, "success", "The key is revoked. Requests that send it are refused from now on.")
	http.Redirect(w, r, "/app/settings/api", http.StatusSeeOther)
}
