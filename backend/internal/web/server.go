// Package web is the HTTP interface: the dispatcher dashboard, the public
// tracking and confirmation pages, the driver app and its JSON API.
package web

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	gorillaws "github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"

	"github.com/natnael-alemayehu/logistics/internal/config"
	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/pkg/websocket"
)

//go:embed templates static
var embedded embed.FS

// Server holds what the handlers need.
type Server struct {
	cfg      *config.Config
	svc      *service.Services
	log      zerolog.Logger
	hub      *websocket.Hub
	pool     *pgxpool.Pool
	views    *renderer
	assets   fs.FS
	version  string
	upgrader gorillaws.Upgrader
	mockSMS  bool
}

// New builds the server.
func New(cfg *config.Config, svc *service.Services, pool *pgxpool.Pool, hub *websocket.Hub, log zerolog.Logger) (*Server, error) {
	var root fs.FS = embedded
	if cfg.DevReload {
		root = os.DirFS("internal/web")
	}
	tpl, err := fs.Sub(root, "templates")
	if err != nil {
		return nil, err
	}
	assets, err := fs.Sub(root, "static")
	if err != nil {
		return nil, err
	}
	s := &Server{
		cfg: cfg, svc: svc, log: log, hub: hub, pool: pool, assets: assets,
		mockSMS: svc.SMS.ProviderName() == "mock",
	}
	s.version = assetVersion(assets)
	s.views, err = newRenderer(tpl, s.funcs(), cfg.DevReload)
	if err != nil {
		return nil, err
	}
	s.upgrader = gorillaws.Upgrader{
		ReadBufferSize:   1024,
		WriteBufferSize:  4096,
		HandshakeTimeout: 10 * time.Second,
		CheckOrigin:      s.sameOrigin,
	}
	return s, nil
}

// Handler is the complete HTTP interface.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(s.clientIP)
	r.Use(chimw.RequestID)
	r.Use(s.recoverer)
	r.Use(s.logRequests)
	r.Use(s.securityHeaders)

	// Browsers send Sec-Fetch-Site / Origin with every form post; anything
	// cross-site that would change state is refused. Bearer-token API calls
	// carry no ambient credential and server-to-server webhooks no Origin, so
	// neither needs or gets the check.
	csrf := http.NewCrossOriginProtection()
	csrf.AddInsecureBypassPattern("/api/")
	r.Use(func(next http.Handler) http.Handler { return csrf.Handler(next) })

	r.Get("/healthz", s.healthz)
	r.Get("/readyz", s.readyz)
	if s.cfg.MetricsToken != "" {
		r.With(s.requireMetricsToken).Handle("/metrics", promhttp.Handler())
	}

	r.Handle("/static/*", http.StripPrefix("/static/", s.staticFiles()))
	r.Get("/favicon.ico", s.favicon)
	r.Get("/robots.txt", s.robots)

	strict := httprate.Limit(10, time.Minute, httprate.WithKeyFuncs(ipKey), httprate.WithLimitHandler(s.tooManyRequests))
	lookup := httprate.Limit(60, time.Minute, httprate.WithKeyFuncs(ipKey), httprate.WithLimitHandler(s.tooManyRequests))
	general := httprate.Limit(600, time.Minute, httprate.WithKeyFuncs(ipKey), httprate.WithLimitHandler(s.tooManyRequests))

	// Public pages.
	r.Group(func(r chi.Router) {
		r.Use(general)
		r.Use(s.optionalStaff)
		r.Get("/", s.landing)
		r.Get("/login", s.loginPage)
		r.With(strict).Post("/login", s.login)
		r.Post("/logout", s.logout)
		r.Get("/signup", s.signupPage)
		r.With(strict).Post("/signup", s.signup)

		r.With(lookup).Get("/track", s.trackForm)
		r.With(lookup).Post("/track", s.trackSubmit)
		r.With(lookup).Get("/track/{number}", s.trackPage)
		r.Get("/track/phone", s.trackPhonePage)
		r.With(strict).Post("/track/phone", s.trackPhoneRequest)
		r.With(strict).Post("/track/phone/verify", s.trackPhoneVerify)
		r.Get("/track/mine", s.trackMine)

		r.With(lookup).Get("/c/{token}", s.confirmPage)
		r.With(strict).Post("/c/{token}", s.confirmSubmit)
		r.With(lookup).Get("/c/{token}/media/{n}", s.confirmMedia)
	})

	// The driver app shell. The service worker is served from /driver/ so
	// that its scope covers the app.
	r.Get("/driver", http.RedirectHandler("/driver/", http.StatusMovedPermanently).ServeHTTP)
	r.Get("/driver/*", s.driverApp)

	// JSON API used by the driver app.
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(general)
		r.Use(s.decompressBody)
		r.Get("/driver/config", s.apiDriverConfig)
		r.With(strict).Post("/driver/login", s.apiDriverLogin)
		r.With(strict).Post("/driver/pin-reset/request", s.apiPINResetRequest)
		r.With(strict).Post("/driver/pin-reset/confirm", s.apiPINResetConfirm)
		r.Group(func(r chi.Router) {
			r.Use(s.requireDriver)
			r.Post("/driver/logout", s.apiDriverLogout)
			r.Post("/driver/pin", s.apiChangePIN)
			r.Post("/driver/sync", s.apiSync)
		})
		r.Post("/sms/inbound", s.apiSMSInbound)
	})

	// The dashboard.
	r.Route("/app", func(r chi.Router) {
		r.Use(general)
		r.Use(s.requireStaff)

		r.Get("/account/password", s.passwordPage)
		r.Post("/account/password", s.passwordChange)

		r.Group(func(r chi.Router) {
			r.Use(s.requireCredentialChanged)

			r.Get("/account", s.accountPage)
			r.Post("/account", s.accountUpdate)
			r.Post("/account/sessions/{id}/revoke", s.sessionRevoke)
			r.Post("/account/sessions/revoke-others", s.sessionRevokeOthers)

			r.Group(func(r chi.Router) {
				r.Use(s.requirePlatformAdmin)
				r.Get("/platform", s.platform)
				r.Post("/platform/tenants/{id}/active", s.platformTenantActive)
				r.Post("/platform/tenants/{id}/plan", s.platformTenantPlan)
			})

			// A company's own pages. The platform admin runs the service, not
			// a fleet, and is sent to the platform page instead.
			r.Group(func(r chi.Router) {
				r.Use(s.requireCompanyStaff)

				r.Get("/", s.dashboard)
				r.Get("/live", s.live)
				r.Get("/api/map", s.apiMap)
				r.Get("/api/badge", s.apiBadge)

				r.Get("/shipments", s.shipmentList)
				r.Get("/shipments/new", s.shipmentNew)
				r.Post("/shipments", s.shipmentCreate)
				r.Get("/shipments/{id}", s.shipmentDetail)
				r.Get("/shipments/{id}/edit", s.shipmentEdit)
				r.Post("/shipments/{id}/edit", s.shipmentUpdate)
				r.Get("/shipments/{id}/waybill", s.shipmentWaybill)
				r.Post("/shipments/{id}/assign", s.shipmentAssign)
				r.Post("/shipments/{id}/unassign", s.shipmentUnassign)
				r.Post("/shipments/{id}/status", s.shipmentStatus)
				r.Post("/shipments/{id}/cancel", s.shipmentCancel)
				r.Post("/shipments/{id}/reopen", s.shipmentReopen)
				r.Post("/shipments/{id}/note", s.shipmentNote)
				r.Get("/shipments/{id}/media/{n}", s.shipmentMedia)
				r.Post("/shipments/{id}/pods/{pod}/accept", s.shipmentAcceptPOD)
				r.Post("/shipments/{id}/pods/{pod}/dismiss", s.shipmentDismissPOD)
				r.Get("/shipments/{id}/pods/{pod}/media/{n}", s.shipmentReviewMedia)

				r.Get("/drivers", s.driverList)
				r.Get("/drivers/{id}", s.driverDetail)
				r.Get("/alerts", s.alertList)
				r.Post("/alerts/{id}/ack", s.alertAck)
				r.Post("/alerts/read", s.alertsRead)
				r.Get("/reports", s.reports)
				r.Get("/vehicles", s.vehicleList)
				r.Get("/vehicles/{id}", s.vehicleDetail)

				r.Group(func(r chi.Router) {
					r.Use(s.requireFleetManager)
					r.Get("/drivers/new", s.driverNew)
					r.Post("/drivers", s.driverCreate)
					r.Get("/drivers/{id}/edit", s.driverEdit)
					r.Post("/drivers/{id}/edit", s.driverUpdate)
					r.Post("/drivers/{id}/active", s.personActive)
					r.Post("/drivers/{id}/reset", s.personReset)
					r.Get("/vehicles/new", s.vehicleNew)
					r.Post("/vehicles", s.vehicleCreate)
					r.Get("/vehicles/{id}/edit", s.vehicleEdit)
					r.Post("/vehicles/{id}/edit", s.vehicleUpdate)
				})

				r.Group(func(r chi.Router) {
					r.Use(s.requireAdmin)
					r.Get("/team", s.teamList)
					r.Get("/team/new", s.teamNew)
					r.Post("/team", s.teamCreate)
					r.Get("/team/{id}/edit", s.teamEdit)
					r.Post("/team/{id}/edit", s.teamUpdate)
					r.Post("/team/{id}/active", s.personActive)
					r.Post("/team/{id}/reset", s.personReset)
					r.Get("/settings", s.settingsPage)
					r.Post("/settings", s.settingsUpdate)
					r.Get("/settings/sms", s.smsLog)
					r.Post("/settings/sms/simulate", s.smsSimulate)
					r.Get("/audit", s.auditLog)
				})
			})
		})
	})

	r.NotFound(s.notFound)
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})
	return r
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("ok"))
}

// readyz reports whether the database is reachable, for load balancers.
func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.pool.Ping(ctx); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("ready"))
}

func (s *Server) robots(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("User-agent: *\nDisallow: /app/\nDisallow: /track/\nDisallow: /c/\nDisallow: /api/\n"))
}

func (s *Server) favicon(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/static/icon.svg", http.StatusMovedPermanently)
}
