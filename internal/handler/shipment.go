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

func (h *Handler) CreateShipment(w http.ResponseWriter, r *http.Request) {
	var input service.CreateShipmentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := validation.Get().Struct(input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validation.FormatErrors(err))
		return
	}

	tenantID := middleware.GetTenantID(r.Context())
	userID := middleware.GetUserID(r.Context())

	shipment, err := h.Shipment.Create(r.Context(), tenantID, userID, input)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusCreated, shipment)
}

func (h *Handler) GetShipment(w http.ResponseWriter, r *http.Request) {
	shipmentID := chi.URLParam(r, "id")
	tenantID := middleware.GetTenantID(r.Context())

	shipment, err := h.Shipment.GetByID(r.Context(), tenantID, shipmentID)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusNotFound, "NOT_FOUND", "Shipment not found")
		return
	}

	response.JSON(w, r, http.StatusOK, shipment)
}

func (h *Handler) ListShipments(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}

	shipments, total, err := h.Shipment.ListByTenant(r.Context(), tenantID, page, perPage)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.PaginatedJSON(w, r, http.StatusOK, shipments, page, perPage, total)
}

func (h *Handler) ListMyShipments(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())
	driverID := middleware.GetUserID(r.Context())

	shipments, err := h.Shipment.ListActiveByDriver(r.Context(), tenantID, driverID)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, shipments)
}

func (h *Handler) AssignDriver(w http.ResponseWriter, r *http.Request) {
	shipmentID := chi.URLParam(r, "id")
	tenantID := middleware.GetTenantID(r.Context())

	var input service.AssignDriverInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := validation.Get().Struct(input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validation.FormatErrors(err))
		return
	}

	shipment, err := h.Shipment.AssignDriver(r.Context(), tenantID, shipmentID, input.DriverID)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, shipment)
}

func (h *Handler) UpdateShipmentStatus(w http.ResponseWriter, r *http.Request) {
	shipmentID := chi.URLParam(r, "id")
	tenantID := middleware.GetTenantID(r.Context())

	var input service.UpdateStatusInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := validation.Get().Struct(input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validation.FormatErrors(err))
		return
	}

	shipment, err := h.Shipment.UpdateStatus(r.Context(), tenantID, shipmentID, input)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, shipment)
}

func (h *Handler) TrackShipment(w http.ResponseWriter, r *http.Request) {
	trackingNumber := chi.URLParam(r, "tracking_number")

	shipment, err := h.Shipment.GetByTrackingNumber(r.Context(), trackingNumber)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusNotFound, "NOT_FOUND", "Shipment not found")
		return
	}

	response.JSON(w, r, http.StatusOK, shipment)
}
