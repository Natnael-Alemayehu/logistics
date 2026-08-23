package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/middleware"
	"github.com/natnael-alemayehu/logistics/pkg/jwt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	httpSwagger "github.com/swaggo/http-swagger"
)

func (h *Handler) Routes(logger zerolog.Logger, jwtManager *jwt.JWTManager, rateLimiter *middleware.RateLimiter, queries *db.Queries) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(middleware.Logger(logger))
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.Metrics())
	r.Use(middleware.RequestSizeLimit(10 * 1024 * 1024))
	r.Use(middleware.CORS())

	if rateLimiter != nil {
		r.Use(rateLimiter.RateLimit())
	} else {
		r.Use(httprate.LimitByIP(100, time.Minute))
	}

	r.Use(chiMiddleware.Throttle(100))

	r.Get("/health", h.Health)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			if rateLimiter != nil {
				r.Use(rateLimiter.RateLimitAuth())
			}
			r.Post("/auth/login/driver", h.DriverLogin)
			r.Post("/auth/login/dispatcher", h.DispatcherLogin)
			r.Post("/auth/refresh", h.RefreshToken)
		})

		r.Get("/track/{tracking_number}", h.TrackShipment)

		r.Get("/ws", middleware.WsAuth(jwtManager)(http.HandlerFunc(h.WS.HandleWebSocket)).ServeHTTP)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(jwtManager))
			// Auth alone only proves the token was signed and has not expired.
			// Revocation — logout, or an admin killing a lost device's session —
			// takes effect only because of this.
			r.Use(middleware.ValidateSession(queries))

			r.Post("/auth/logout", h.Logout)
			r.Post("/auth/change-password", h.ChangePassword)
			r.Post("/auth/forgot-pin", h.ForgotPIN)

			r.Post("/notifications/register", h.RegisterPushToken)
			r.Delete("/notifications/device/{deviceId}", h.UnregisterDevice)

			r.Get("/sessions", h.ListSessions)
			r.Delete("/sessions/{id}", h.RevokeSession)
			r.Delete("/sessions/others", h.RevokeOtherSessions)

			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireDriver())
				r.Post("/sync", h.Sync)
				r.Get("/my-shipments", h.ListMyShipments)
				r.Get("/driver/stats", h.GetDriverStats)
				r.Get("/driver/vehicle", h.GetDriverVehicle)
				r.Put("/shipments/{id}/status", h.UpdateShipmentStatus)
				r.Patch("/shipments/{id}/status", h.UpdateShipmentStatus)
			})

			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireDispatcher())

				r.Post("/shipments", h.CreateShipment)
				r.Get("/shipments", h.ListShipments)
				r.Get("/shipments/search", h.SearchShipments)
				r.Get("/shipments/{id}", h.GetShipment)
				r.Put("/shipments/{id}", h.UpdateShipment)
				r.Put("/shipments/{id}/assign", h.AssignDriver)
				r.Put("/shipments/{id}/status", h.UpdateShipmentStatus)
				r.Patch("/shipments/{id}/status", h.UpdateShipmentStatus)
				r.Post("/shipments/{id}/cancel", h.CancelShipment)
				r.Get("/shipments/{id}/tracking", h.ListShipmentTrackingEvents)
				r.Get("/shipments/{id}/pod", h.GetShipmentPOD)

				r.Get("/drivers", h.ListDrivers)
				r.Post("/drivers", h.CreateDriver)
				r.Get("/drivers/locations", h.GetDriverLocations)
				r.Get("/drivers/{id}/location", h.GetDriverLocation)

				r.Get("/vehicles", h.ListVehicles)
				r.Get("/vehicles/active", h.ListActiveVehicles)
				r.Post("/vehicles", h.CreateVehicle)
				r.Get("/vehicles/{id}", h.GetVehicle)
				r.Put("/vehicles/{id}", h.UpdateVehicle)

				r.Get("/dashboard/stats", h.GetDashboardStats)
				r.Get("/dashboard/alerts", h.GetDashboardAlerts)
				r.Get("/dashboard/activity", h.GetDashboardActivity)
			})

			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAdmin())
				r.Get("/users", h.ListUsers)
				r.Post("/users", h.CreateUser)
				r.Put("/users/{id}", h.UpdateUser)

				r.Delete("/vehicles/{id}", h.DeleteVehicle)
			})
		})
	})

	return r
}
