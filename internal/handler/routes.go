package handler

import (
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/natnael-alemayehu/logistics/internal/middleware"
	"github.com/natnael-alemayehu/logistics/pkg/jwt"
	"github.com/rs/zerolog"
	httpSwagger "github.com/swaggo/http-swagger"
)

func (h *Handler) Routes(logger zerolog.Logger, jwtManager *jwt.JWTManager) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(middleware.Logger(logger))
	r.Use(chiMiddleware.Recoverer)
	r.Use(httprate.LimitByIP(100, time.Minute))
	r.Use(chiMiddleware.Throttle(100))
	r.Use(middleware.CORS())
	r.Get("/health", h.Health)

	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	r.Group(func(r chi.Router) {
		r.Post("/auth/login/driver", h.DriverLogin)
		r.Post("/auth/login/dispatcher", h.DispatcherLogin)
		r.Post("/auth/refresh", h.RefreshToken)
		r.Get("/track/{tracking_number}", h.TrackShipment)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(jwtManager))

		r.Post("/auth/logout", h.Logout)

		r.Get("/sessions", h.ListSessions)
		r.Delete("/sessions/{id}", h.RevokeSession)
		r.Delete("/sessions/others", h.RevokeOtherSessions)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireDriver())
			r.Post("/sync", h.Sync)
			r.Get("/my-shipments", h.ListMyShipments)
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
			r.Post("/shipments/{id}/cancel", h.CancelShipment)

			r.Get("/drivers", h.ListDrivers)
			r.Post("/drivers", h.CreateDriver)

			r.Get("/vehicles", h.ListVehicles)
			r.Get("/vehicles/active", h.ListActiveVehicles)
			r.Post("/vehicles", h.CreateVehicle)
			r.Get("/vehicles/{id}", h.GetVehicle)
			r.Put("/vehicles/{id}", h.UpdateVehicle)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAdmin())
			r.Get("/users", h.ListUsers)
			r.Post("/users", h.CreateUser)

			r.Delete("/vehicles/{id}", h.DeleteVehicle)
		})
	})

	return r
}
