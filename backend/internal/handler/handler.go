// Package handler handles all the http handlers
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/service"
)

type Handler struct {
	Auth        *service.AuthService
	Shipment    *service.ShipmentService
	SyncSvc     *service.SyncService
	User        *service.UserService
	Vehicle     *service.VehicleService
	Tracking    *service.TrackingService
	WS          *WSHandler
	Dashboard   *DashboardService
	Driver      *service.DriverService
	DeviceToken *service.DeviceTokenService
}

func New(
	auth *service.AuthService,
	shipment *service.ShipmentService,
	sync *service.SyncService,
	user *service.UserService,
	vehicle *service.VehicleService,
	tracking *service.TrackingService,
	ws *WSHandler,
	dashboard *DashboardService,
	driver *service.DriverService,
	deviceToken *service.DeviceTokenService,
) *Handler {
	return &Handler{
		Auth:        auth,
		Shipment:    shipment,
		SyncSvc:     sync,
		User:        user,
		Vehicle:     vehicle,
		Tracking:    tracking,
		WS:          ws,
		Dashboard:   dashboard,
		Driver:      driver,
		DeviceToken: deviceToken,
	}
}

type HealthResponse struct {
	Status    string `json:"status" example:"healthy"`
	Version   string `json:"version" example:"1.0.0"`
	Database  string `json:"database" example:"connected"`
	Timestamp string `json:"timestamp" example:"2024-01-01T00:00:00Z"`
}

// Health godoc
// @Summary Health check
// @Description Check if the service is running and healthy
// @Tags system
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Version:   "1.0.0",
		Database:  "connected",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
