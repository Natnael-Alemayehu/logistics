package handler

import (
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/natnael-alemayehu/logistics/internal/middleware"
	"github.com/natnael-alemayehu/logistics/pkg/jwt"
	"github.com/rs/zerolog"
)

func (h *Handler) Routes(logger zerolog.Logger, jwtManager *jwt.JWTManager) *chi.Mux {
	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(middleware.Logger(logger))
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Throttle(100))
	r.Use(middleware.CORS())

	r.Get("/health", h.Health)

	r.Group(func(r chi.Router) {
		r.Post("/auth/login/driver", h.DriverLogin)
		r.Post("/auth/login/dispatcher", h.DispatcherLogin)
		r.Post("/auth/refresh", h.RefreshToken)
		r.Get("/track/{tracking_number}", h.TrackShipment)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(jwtManager))

		r.Post("/auth/logout", h.Logout)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireDriver())
			r.Post("/sync", h.Sync)
			r.Get("/my-shipments", h.ListMyShipments)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireDispatcher())

			r.Post("/shipments", h.CreateShipment)
			r.Get("/shipments", h.ListShipments)
			r.Get("/shipments/{id}", h.GetShipment)
			r.Put("/shipments/{id}/assign", h.AssignDriver)
			r.Put("/shipments/{id}/status", h.UpdateShipmentStatus)

			r.Get("/drivers", h.ListDrivers)
			r.Post("/drivers", h.CreateDriver)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAdmin())
			r.Get("/users", h.ListUsers)
			r.Post("/users", h.CreateUser)
		})
	})

	return r
}
