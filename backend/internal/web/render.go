package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/service"
)

// renderer holds the parsed templates: one set per page, each combining the
// page with the layouts and partials. Pages under app/ use the dashboard
// layout and pages under public/ the public one.
type renderer struct {
	fsys  fs.FS
	funcs template.FuncMap
	dev   bool
	mu    sync.RWMutex
	pages map[string]*template.Template
}

func newRenderer(fsys fs.FS, funcs template.FuncMap, dev bool) (*renderer, error) {
	r := &renderer{fsys: fsys, funcs: funcs, dev: dev}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *renderer) load() error {
	base := template.New("").Funcs(r.funcs)
	for _, pattern := range []string{"layouts/*.html", "partials/*.html"} {
		if _, err := base.ParseFS(r.fsys, pattern); err != nil {
			return fmt.Errorf("parse %s: %w", pattern, err)
		}
	}
	pages := map[string]*template.Template{}
	for _, dir := range []string{"app", "public"} {
		files, err := fs.Glob(r.fsys, dir+"/*.html")
		if err != nil {
			return err
		}
		for _, f := range files {
			t, err := base.Clone()
			if err != nil {
				return err
			}
			if _, err := t.ParseFS(r.fsys, f); err != nil {
				return fmt.Errorf("parse %s: %w", f, err)
			}
			pages[strings.TrimSuffix(f, ".html")] = t
		}
	}
	r.mu.Lock()
	r.pages = pages
	r.mu.Unlock()
	return nil
}

func (r *renderer) render(w *bytes.Buffer, page string, v *View) error {
	if r.dev {
		if err := r.load(); err != nil {
			return err
		}
	}
	r.mu.RLock()
	t, ok := r.pages[page]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no template %q", page)
	}
	layout := path.Dir(page) // "app" or "public"
	return t.ExecuteTemplate(w, layout, v)
}

// View is the data every page template receives.
type View struct {
	Title  string
	Nav    string
	Me     *service.Principal
	App    AppInfo
	Flash  *Flash
	Unread int64
	// Waiting counts support tickets waiting for an answer, for the
	// platform admin.
	Waiting int64
	// Form holds submitted values to redisplay after a validation error.
	Form   url.Values
	Errors map[string]string
	// Error is a message about the form as a whole.
	Error string
	Data  any
	Path  string
	Query url.Values
	Now   time.Time
	// Standing is the company's billing state, for the banner; zero for the
	// platform operator.
	Standing service.Standing
}

// AppInfo is deployment information templates use.
type AppInfo struct {
	Name       string
	Version    string
	TileURL    string
	TileCredit string
	// SatelliteURL is empty when the maps offer no satellite view.
	SatelliteURL    string
	SatelliteCredit string
	MockSMS         bool
	MockMail        bool
	MockPay         bool
	SMSNumber       string
	PublicURL       string
	SupportEmail    string
	BankDetails     string
}

// Flash is a one-time message shown after a redirect.
type Flash struct {
	Kind    string `json:"k"`
	Message string `json:"m"`
}

const flashCookie = "flash"

func (s *Server) setFlash(w http.ResponseWriter, kind, msg string) {
	b, _ := json.Marshal(Flash{Kind: kind, Message: msg})
	http.SetCookie(w, &http.Cookie{
		Name: flashCookie, Value: base64.RawURLEncoding.EncodeToString(b), Path: "/",
		HttpOnly: true, Secure: s.cfg.SecureCookies(), SameSite: http.SameSiteLaxMode, MaxAge: 60,
	})
}

func (s *Server) takeFlash(w http.ResponseWriter, r *http.Request) *Flash {
	c, err := r.Cookie(flashCookie)
	if err != nil {
		return nil
	}
	http.SetCookie(w, &http.Cookie{Name: flashCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cfg.SecureCookies(), SameSite: http.SameSiteLaxMode})
	b, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return nil
	}
	var f Flash
	if json.Unmarshal(b, &f) != nil || f.Message == "" {
		return nil
	}
	return &f
}

// satelliteURL is the satellite imagery the maps offer, or "" for none.
func (s *Server) satelliteURL() string {
	if strings.EqualFold(s.cfg.MapSatelliteURL, "off") {
		return ""
	}
	return s.cfg.MapSatelliteURL
}

// view starts a View for the request.
func (s *Server) view(w http.ResponseWriter, r *http.Request, title, nav string, data any) *View {
	v := &View{
		Title: title, Nav: nav, Me: principal(r), Data: data, Path: r.URL.Path, Query: r.URL.Query(),
		Now:  time.Now(),
		Form: url.Values{}, Errors: map[string]string{},
		App: AppInfo{
			Name: s.cfg.AppName, Version: s.version, TileURL: s.cfg.MapTileURL, TileCredit: s.cfg.MapTileAttribution,
			SatelliteURL: s.satelliteURL(), SatelliteCredit: s.cfg.MapSatelliteAttribution,
			MockSMS: s.mockSMS, MockMail: s.mockMail, MockPay: s.mockPay, SMSNumber: s.cfg.SMSNumber, PublicURL: s.cfg.PublicURL,
			SupportEmail: s.cfg.SupportEmail, BankDetails: s.cfg.BankDetails,
		},
		Flash: s.takeFlash(w, r),
	}
	if v.Me != nil {
		if n, err := s.svc.Alerts.Unread(r.Context(), v.Me); err == nil {
			v.Unread = n
		}
		if !v.Me.IsPlatformAdmin() {
			v.Standing = v.Me.Standing(v.Now)
		} else if n, err := s.svc.Support.Waiting(r.Context()); err == nil {
			v.Waiting = n
		}
	}
	return v
}

// render writes a page. Rendering into a buffer first means a template error
// produces a clean error page rather than half a page.
func (s *Server) render(w http.ResponseWriter, r *http.Request, status int, page string, v *View) {
	var buf bytes.Buffer
	if err := s.views.render(&buf, page, v); err != nil {
		s.log.Error().Err(err).Str("page", page).Msg("render")
		http.Error(w, "Something went wrong on our side.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// formError redisplays a form with a validation failure, or returns false if
// err is not one.
func (s *Server) formError(w http.ResponseWriter, r *http.Request, page string, v *View, err error) bool {
	de, ok := service.AsError(err)
	if !ok {
		return false
	}
	_ = r.ParseForm()
	v.Form = r.PostForm
	if de.Field != "" {
		v.Errors[de.Field] = de.Message
	}
	v.Error = de.Message
	s.render(w, r, de.Kind.HTTPStatus(), page, v)
	return true
}

func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	layout := "public/error"
	if principal(r) != nil && strings.HasPrefix(r.URL.Path, "/app") {
		layout = "app/error"
	}
	v := s.view(w, r, title, "", map[string]any{"Status": status, "Title": title, "Detail": detail})
	s.render(w, r, status, layout, v)
}

// fail reports an error from a page handler: a domain error is shown to the
// user; anything else is logged and shown as a generic failure.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	if de, ok := service.AsError(err); ok {
		title := "That didn't work"
		if de.Kind == service.KindNotFound {
			title = "Not found"
		}
		s.renderError(w, r, de.Kind.HTTPStatus(), title, de.Message)
		return
	}
	if errors.Is(err, errBadID) {
		s.notFound(w, r)
		return
	}
	s.log.Error().Err(err).Str("path", r.URL.Path).Msg("request failed")
	detail := "We couldn't complete that. Please try again."
	if p := principal(r); p != nil && !p.IsPlatformAdmin() && strings.HasPrefix(r.URL.Path, "/app") {
		detail += " If it keeps happening, tell us on the Help page."
	}
	s.renderError(w, r, http.StatusInternalServerError, "Something went wrong", detail)
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "Not found.")
		return
	}
	s.renderError(w, r, http.StatusNotFound, "Page not found", "The page you asked for doesn't exist or has moved.")
}

// back redirects after a POST with a flash message: to the form's "back"
// value if it is a local path, else to fallback.
func (s *Server) back(w http.ResponseWriter, r *http.Request, fallback, kind, msg string) {
	if msg != "" {
		s.setFlash(w, kind, msg)
	}
	to := fallback
	if b := r.PostFormValue("back"); localPath(b) {
		to = b
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

// actionResult finishes a POST from a detail page: success flashes msg,
// a domain error flashes its message, anything else fails the request.
func (s *Server) actionResult(w http.ResponseWriter, r *http.Request, fallback string, err error, msg string) {
	if err == nil {
		s.back(w, r, fallback, "success", msg)
		return
	}
	if de, ok := service.AsError(err); ok {
		s.back(w, r, fallback, "error", de.Message)
		return
	}
	s.fail(w, r, err)
}

func localPath(p string) bool {
	return strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "/\\")
}

// JSON API helpers.

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}

func (s *Server) apiError(w http.ResponseWriter, r *http.Request, err error) {
	if de, ok := service.AsError(err); ok {
		writeJSON(w, de.Kind.HTTPStatus(), map[string]any{"error": map[string]string{
			"code": de.Kind.Code(), "message": de.Message, "field": de.Field,
		}})
		return
	}
	s.log.Error().Err(err).Str("path", r.URL.Path).Msg("api request failed")
	writeJSONError(w, http.StatusInternalServerError, "INTERNAL", "Something went wrong on our side. Please try again.")
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		return &service.Error{Kind: service.KindInvalid, Message: "The request body is not valid JSON."}
	}
	return nil
}
