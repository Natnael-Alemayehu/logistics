package web

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"path"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/pkg/metrics"
	"github.com/natnael-alemayehu/logistics/pkg/payments"
)

type ctxKey int

const (
	ctxPrincipal ctxKey = iota
	ctxIP
)

const sessionCookie = "session"

func principal(r *http.Request) *service.Principal {
	p, _ := r.Context().Value(ctxPrincipal).(*service.Principal)
	return p
}

func withPrincipal(r *http.Request, p *service.Principal) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ctxPrincipal, p))
}

func remoteIP(r *http.Request) string {
	ip, _ := r.Context().Value(ctxIP).(string)
	return ip
}

func clientInfo(r *http.Request) service.ClientInfo {
	return service.ClientInfo{IP: remoteIP(r), UserAgent: r.UserAgent()}
}

func actor(r *http.Request) service.Actor {
	return principal(r).Actor(service.SourceDashboard, remoteIP(r), r.UserAgent())
}

// clientIP determines the caller's address. Forwarding headers are believed
// only when the deployment says a proxy sets them; otherwise anyone could
// claim any address and walk around the rate limits. Even then, only the
// last X-Forwarded-For entry: X-Real-IP is one the client can send too, and
// proxies such as Caddy pass it through.
func (s *Server) clientIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if xff := r.Header.Values("X-Forwarded-For"); s.cfg.TrustProxy && len(xff) > 0 {
			// The proxy appends the address it saw, to the last header;
			// earlier entries are whatever the client chose to send.
			parts := strings.Split(xff[len(xff)-1], ",")
			if last := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(last) != nil {
				ip = last
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxIP, ip)))
	})
}

func ipKey(r *http.Request) (string, error) { return remoteIP(r), nil }

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				s.log.Error().Interface("panic", v).Bytes("stack", debug.Stack()).Str("path", r.URL.Path).Msg("handler panic")
				http.Error(w, "Something went wrong on our side.", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// logRequests logs each request and counts it for /metrics.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		// A WebSocket stays open as long as its page: it is counted in
		// menged_websocket_connections instead.
		counted := r.Header.Get("Upgrade") == ""
		if counted {
			metrics.HTTPInFlight.Inc()
			defer metrics.HTTPInFlight.Dec()
		}
		next.ServeHTTP(ww, r)
		if counted {
			observeRequest(r, ww.Status(), time.Since(start))
		}
		quiet := strings.HasPrefix(r.URL.Path, "/static/") || r.URL.Path == "/healthz" || r.URL.Path == "/readyz"
		ev := s.log.Info()
		if quiet && ww.Status() < 400 {
			ev = s.log.Debug()
		}
		if ww.Status() >= 500 {
			ev = s.log.Error()
		}
		ev.Str("method", r.Method).Str("path", r.URL.Path).Int("status", ww.Status()).
			Int("bytes", ww.BytesWritten()).Dur("duration", time.Since(start)).
			Str("ip", remoteIP(r)).Str("request_id", chimw.GetReqID(r.Context())).Msg("request")
	})
}

// observeRequest counts a request under its route pattern, such as
// /app/shipments/{id}, so that IDs in paths don't each become a series.
func observeRequest(r *http.Request, status int, took time.Duration) {
	route := "unmatched"
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if p := rc.RoutePattern(); p != "" {
			route = p
		}
	}
	if status == 0 {
		status = http.StatusOK
	}
	metrics.HTTPRequests.WithLabelValues(route, r.Method, strconv.Itoa(status)).Inc()
	metrics.HTTPDuration.WithLabelValues(route, r.Method).Observe(took.Seconds())
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	// Pay redirects the form to the provider's checkout, which form-action
	// governs too.
	formAction := []string{"form-action 'self'"}
	for _, p := range s.svc.Billing.Providers() {
		if c, ok := p.(payments.Starter); ok && c.CheckoutOrigin() != "" {
			formAction = append(formAction, c.CheckoutOrigin())
		}
	}
	csp := strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob: https:",
		"connect-src 'self'",
		"font-src 'self'",
		"worker-src 'self'",
		"manifest-src 'self'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		strings.Join(formAction, " "),
		"object-src 'none'",
	}, "; ")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Permissions-Policy", "geolocation=(self), camera=(self), microphone=(), payment=()")
		if s.cfg.SecureCookies() {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// sameOrigin is the WebSocket origin check: a page on another site must not
// be able to open a live feed with the visitor's cookie.
func (s *Server) sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

// maxUploadBytes bounds a request body after decompression. A sync carrying
// a few deliveries with photos is well under this.
const maxUploadBytes = 32 << 20

// decompressBody accepts gzip-compressed request bodies, which the driver app
// uses to keep syncs small on slow networks.
func (s *Server) decompressBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
		if strings.EqualFold(r.Header.Get("Content-Encoding"), "gzip") {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "The request body is not valid gzip.")
				return
			}
			defer gz.Close()
			r.Body = struct {
				io.Reader
				io.Closer
			}{io.LimitReader(gz, maxUploadBytes), gz}
			r.Header.Del("Content-Encoding")
		}
		next.ServeHTTP(w, r)
	})
}

// optionalStaff attaches the signed-in staff member, if any, so public pages
// can offer a way back to the dashboard.
func (s *Server) optionalStaff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookie); err == nil {
			if p, err := s.svc.Auth.Authenticate(r.Context(), c.Value, service.SessionWeb); err == nil {
				r = withPrincipal(r, p)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireStaff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		var p *service.Principal
		if err == nil {
			p, err = s.svc.Auth.Authenticate(r.Context(), c.Value, service.SessionWeb)
		}
		if err != nil {
			s.clearSessionCookie(w)
			// Someone whose session just ended is told why; someone who was
			// never signed in simply lands on the login page.
			if c != nil {
				msg := "Please sign in again."
				if de, ok := service.AsError(err); ok && !errors.Is(err, service.ErrSessionRevoked) {
					msg = de.Message
				}
				s.setFlash(w, "info", msg)
			}
			if r.Method == http.MethodGet && !strings.HasPrefix(r.URL.Path, "/app/api/") && r.URL.Path != "/app/live" {
				http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
				return
			}
			http.Error(w, "Sign in required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, withPrincipal(r, p))
	})
}

// requireCredentialChanged holds someone signed in with an issued password
// on the change-password page until they pick their own.
func (s *Server) requireCredentialChanged(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if principal(r).MustChangeCredential {
			http.Redirect(w, r, "/app/account/password", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireRole(allowed func(*service.Principal) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !allowed(principal(r)) {
				s.renderError(w, r, http.StatusForbidden, "You don't have access to this page", "Ask your company admin if you need it.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) requireFleetManager(next http.Handler) http.Handler {
	return s.requireRole((*service.Principal).CanManageFleet)(next)
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return s.requireRole((*service.Principal).CanManageCompany)(next)
}

func (s *Server) requirePlatformAdmin(next http.Handler) http.Handler {
	return s.requireRole((*service.Principal).IsPlatformAdmin)(next)
}

// requireCompanyStaff keeps the platform admin out of a company's pages:
// they would show the platform's own empty company, not a customer's.
func (s *Server) requireCompanyStaff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if principal(r).IsPlatformAdmin() {
			if r.Method == http.MethodGet && !strings.HasPrefix(r.URL.Path, "/app/api/") && r.URL.Path != "/app/live" {
				http.Redirect(w, r, "/app/platform", http.StatusSeeOther)
				return
			}
			s.renderError(w, r, http.StatusForbidden, "This is a company page", "The platform admin manages companies from the Companies page.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireDriver authenticates the driver app's bearer token.
func (s *Server) requireDriver(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Sign in required.")
			return
		}
		p, err := s.svc.Auth.Authenticate(r.Context(), token, service.SessionAPI)
		if err != nil {
			s.apiError(w, r, err)
			return
		}
		if !p.IsDriver() {
			s.apiError(w, r, service.ErrNotDriver)
			return
		}
		next.ServeHTTP(w, withPrincipal(r, p))
	})
}

func (s *Server) requireMetricsToken(next http.Handler) http.Handler {
	want := []byte("Bearer " + s.cfg.MetricsToken)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) tooManyRequests(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSONError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests. Wait a minute and try again.")
		return
	}
	s.renderError(w, r, http.StatusTooManyRequests, "Slow down", "Too many requests from your network. Wait a minute and try again.")
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string, persistent time.Time) {
	c := &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/",
		HttpOnly: true, Secure: s.cfg.SecureCookies(), SameSite: http.SameSiteLaxMode,
	}
	if !persistent.IsZero() {
		c.Expires = persistent
	}
	http.SetCookie(w, c)
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.SecureCookies(), SameSite: http.SameSiteLaxMode,
	})
}

// staticFiles serves embedded assets. Versioned URLs (?v=) are cached
// forever; anything else is revalidated.
func (s *Server) staticFiles() http.Handler {
	files := http.FileServerFS(s.assets)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") || strings.HasPrefix(r.URL.Path, "driver/") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("v") == s.version && !s.cfg.DevReload {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}

// driverApp serves the driver PWA. Everything is revalidated on each load so
// a deploy reaches drivers the next time they are online; the service worker
// keeps the app usable when they are not.
func (s *Server) driverApp(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/driver")
	name = strings.TrimPrefix(name, "/")
	if name == "" {
		name = "index.html"
	}
	f := "driver/" + name
	if _, err := fs.Stat(s.assets, f); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	if name == "sw.js" {
		w.Header().Set("Service-Worker-Allowed", "/driver/")
	}
	http.ServeFileFS(w, r, s.assets, f)
}

// assetVersion fingerprints the static files so their URLs change on deploy.
func assetVersion(fsys fs.FS) string {
	h := sha256.New()
	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		b, err := fs.ReadFile(fsys, p)
		if err == nil {
			h.Write([]byte(p))
			h.Write(b)
		}
		return nil
	})
	return hex.EncodeToString(h.Sum(nil))[:10]
}
