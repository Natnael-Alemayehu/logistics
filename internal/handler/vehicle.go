package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/natnael-alemayehu/logistics/internal/middleware"
	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/pkg/response"
	"github.com/natnael-alemayehu/logistics/pkg/validation"
)

// CreateVehicle godoc
// @Summary Create a vehicle
// @Description Create a new vehicle
// @Tags vehicles
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param input body service.CreateVehicleInput true "Vehicle data"
// @Success 201 {object} model.Vehicle
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /vehicles [post]
func (h *Handler) CreateVehicle(w http.ResponseWriter, r *http.Request) {
	var input service.CreateVehicleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := validation.Get().Struct(input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validation.FormatErrors(err))
		return
	}

	tenantID := middleware.GetTenantID(r.Context())

	vehicle, err := h.Vehicle.Create(r.Context(), tenantID, input)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusCreated, vehicle)
}

// GetVehicle godoc
// @Summary Get a vehicle
// @Description Get vehicle details by ID
// @Tags vehicles
// @Produce json
// @Security BearerAuth
// @Param id path string true "Vehicle ID"
// @Success 200 {object} model.Vehicle
// @Failure 401 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /vehicles/{id} [get]
func (h *Handler) GetVehicle(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "id")
	tenantID := middleware.GetTenantID(r.Context())

	vehicle, err := h.Vehicle.GetByID(r.Context(), tenantID, vehicleID)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusNotFound, "NOT_FOUND", "Vehicle not found")
		return
	}

	response.JSON(w, r, http.StatusOK, vehicle)
}

// ListVehicles godoc
// @Summary List vehicles
// @Description Get paginated list of vehicles
// @Tags vehicles
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(25)
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /vehicles [get]
func (h *Handler) ListVehicles(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}

	vehicles, total, err := h.Vehicle.ListByTenant(r.Context(), tenantID, page, perPage)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.PaginatedJSON(w, r, http.StatusOK, vehicles, page, perPage, total)
}

// ListActiveVehicles godoc
// @Summary List active vehicles
// @Description Get list of all active vehicles
// @Tags vehicles
// @Produce json
// @Security BearerAuth
// @Success 200 {array} model.Vehicle
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /vehicles/active [get]
func (h *Handler) ListActiveVehicles(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	vehicles, err := h.Vehicle.ListActive(r.Context(), tenantID)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, vehicles)
}

// UpdateVehicle godoc
// @Summary Update a vehicle
// @Description Update vehicle details
// @Tags vehicles
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Vehicle ID"
// @Param input body service.UpdateVehicleInput true "Vehicle data"
// @Success 200 {object} model.Vehicle
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /vehicles/{id} [put]
func (h *Handler) UpdateVehicle(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "id")
	tenantID := middleware.GetTenantID(r.Context())

	var input service.UpdateVehicleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	vehicle, err := h.Vehicle.Update(r.Context(), tenantID, vehicleID, input)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, vehicle)
}

// DeleteVehicle godoc
// @Summary Delete a vehicle
// @Description Delete a vehicle (soft delete)
// @Tags vehicles
// @Produce json
// @Security BearerAuth
// @Param id path string true "Vehicle ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /vehicles/{id} [delete]
func (h *Handler) DeleteVehicle(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "id")
	tenantID := middleware.GetTenantID(r.Context())

	if err := h.Vehicle.Delete(r.Context(), tenantID, vehicleID); err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"message": "Vehicle deleted"})
}
