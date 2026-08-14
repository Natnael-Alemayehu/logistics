package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/natnael-alemayehu/logistics/internal/middleware"
	"github.com/natnael-alemayehu/logistics/pkg/response"
)

// ListShipmentTrackingEvents godoc
//
//	@Summary		List tracking events for a shipment
//	@Description	Get paginated list of tracking events for a shipment, sorted by recorded_at descending
//	@Tags			tracking
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id			path		string	true	"Shipment ID"
//	@Param			page		query		int		false	"Page number"		default(1)
//	@Param			per_page	query		int		false	"Items per page"	default(25)
//	@Success		200			{object}	response.Response
//	@Failure		400			{object}	response.Response
//	@Failure		401			{object}	response.Response
//	@Failure		404			{object}	response.Response
//	@Failure		500			{object}	response.Response
//	@Router			/shipments/{id}/tracking [get]
func (h *Handler) ListShipmentTrackingEvents(w http.ResponseWriter, r *http.Request) {
	shipmentID := chi.URLParam(r, "id")
	if shipmentID == "" {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Shipment ID required")
		return
	}

	tenantID := middleware.GetTenantID(r.Context())

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}

	events, total, err := h.Tracking.ListTrackingEventsByShipment(r.Context(), tenantID, shipmentID, page, perPage)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.PaginatedJSON(w, r, http.StatusOK, events, page, perPage, total)
}

// GetShipmentPOD godoc
//
//	@Summary		Get proof of delivery for a shipment
//	@Description	Get POD details for a delivered shipment including recipient info, signature URL, and photo URLs
//	@Tags			tracking
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Shipment ID"
//	@Success		200	{object}	service.PODOutput
//	@Failure		400	{object}	response.Response
//	@Failure		401	{object}	response.Response
//	@Failure		404	{object}	response.Response
//	@Failure		500	{object}	response.Response
//	@Router			/shipments/{id}/pod [get]
func (h *Handler) GetShipmentPOD(w http.ResponseWriter, r *http.Request) {
	shipmentID := chi.URLParam(r, "id")
	if shipmentID == "" {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Shipment ID required")
		return
	}

	tenantID := middleware.GetTenantID(r.Context())

	pod, err := h.Tracking.GetPODByShipment(r.Context(), tenantID, shipmentID)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusNotFound, "NOT_FOUND", "POD not found for shipment")
		return
	}

	response.JSON(w, r, http.StatusOK, pod)
}

// GetDriverLocation godoc
//
//	@Summary		Get driver's last known location
//	@Description	Get the last tracking event with coordinates for a driver, for dispatcher dashboard
//	@Tags			tracking
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Driver ID"
//	@Success		200	{object}	service.DriverLocationOutput
//	@Failure		400	{object}	response.Response
//	@Failure		401	{object}	response.Response
//	@Failure		404	{object}	response.Response
//	@Failure		500	{object}	response.Response
//	@Router			/drivers/{id}/location [get]
func (h *Handler) GetDriverLocation(w http.ResponseWriter, r *http.Request) {
	driverID := chi.URLParam(r, "id")
	if driverID == "" {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Driver ID required")
		return
	}

	tenantID := middleware.GetTenantID(r.Context())

	location, err := h.Tracking.GetDriverLocation(r.Context(), tenantID, driverID)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusNotFound, "NOT_FOUND", "No location data found for driver")
		return
	}

	response.JSON(w, r, http.StatusOK, location)
}
