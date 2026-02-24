package handler

import (
	"net/http"

	"github.com/natnael-alemayehu/logistics/internal/middleware"
	"github.com/natnael-alemayehu/logistics/pkg/response"
)

// GetDriverStats godoc
// @Summary Get driver delivery statistics
// @Description Get delivery counts for today, week, month, and total for the authenticated driver
// @Tags driver
// @Produce json
// @Security BearerAuth
// @Success 200 {object} model.DriverStats
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /driver/stats [get]
func (h *Handler) GetDriverStats(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	driverID := middleware.GetUserID(r.Context())

	stats, err := h.Driver.GetStats(r.Context(), tenantID, driverID)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, stats)
}

// GetDriverVehicle godoc
// @Summary Get driver's current vehicle assignment
// @Description Get the vehicle assigned to the driver for their current active shipment
// @Tags driver
// @Produce json
// @Security BearerAuth
// @Success 200 {object} model.Vehicle
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /driver/vehicle [get]
func (h *Handler) GetDriverVehicle(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	driverID := middleware.GetUserID(r.Context())

	vehicle, err := h.Driver.GetVehicleAssignment(r.Context(), tenantID, driverID)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	if vehicle == nil {
		response.JSON(w, r, http.StatusOK, nil)
		return
	}

	response.JSON(w, r, http.StatusOK, vehicle)
}
