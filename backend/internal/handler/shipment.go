package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/natnael-alemayehu/logistics/internal/middleware"
	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/pkg/response"
	"github.com/natnael-alemayehu/logistics/pkg/validation"
)

// CreateShipment godoc
//
//	@Summary		Create a shipment
//	@Description	Create a new shipment
//	@Tags			shipments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			input	body		service.CreateShipmentInput	true	"Shipment data"
//	@Success		201		{object}	model.Shipment
//	@Failure		400		{object}	response.Response
//	@Failure		401		{object}	response.Response
//	@Failure		500		{object}	response.Response
//	@Router			/shipments [post]
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
	ipAddress := getIPAddress(r)
	userAgent := r.Header.Get("User-Agent")

	shipment, err := h.Shipment.Create(r.Context(), tenantID, userID, input, ipAddress, userAgent)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusCreated, shipment)
}

// GetShipment godoc
//
//	@Summary		Get a shipment
//	@Description	Get shipment details by ID
//	@Tags			shipments
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Shipment ID"
//	@Success		200	{object}	model.Shipment
//	@Failure		401	{object}	response.Response
//	@Failure		404	{object}	response.Response
//	@Router			/shipments/{id} [get]
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

// ListShipments godoc
//
//	@Summary		List shipments
//	@Description	Get paginated list of shipments
//	@Tags			shipments
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page		query		int	false	"Page number"		default(1)
//	@Param			per_page	query		int	false	"Items per page"	default(25)
//	@Success		200			{object}	response.Response
//	@Failure		401			{object}	response.Response
//	@Failure		500			{object}	response.Response
//	@Router			/shipments [get]
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

// ListMyShipments godoc
//
//	@Summary		List driver's shipments
//	@Description	Get active shipments for the logged-in driver
//	@Tags			shipments
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{array}		model.Shipment
//	@Failure		401	{object}	response.Response
//	@Failure		500	{object}	response.Response
//	@Router			/my-shipments [get]
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

// UpdateShipment godoc
//
//	@Summary		Update a shipment
//	@Description	Update shipment details
//	@Tags			shipments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string						true	"Shipment ID"
//	@Param			input	body		service.UpdateShipmentInput	true	"Shipment data"
//	@Success		200		{object}	model.Shipment
//	@Failure		400		{object}	response.Response
//	@Failure		401		{object}	response.Response
//	@Failure		500		{object}	response.Response
//	@Router			/shipments/{id} [put]
func (h *Handler) UpdateShipment(w http.ResponseWriter, r *http.Request) {
	shipmentID := chi.URLParam(r, "id")
	tenantID := middleware.GetTenantID(r.Context())
	userID := middleware.GetUserID(r.Context())

	var input service.UpdateShipmentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	ipAddress := getIPAddress(r)
	userAgent := r.Header.Get("User-Agent")

	shipment, err := h.Shipment.Update(r.Context(), tenantID, userID, shipmentID, input, ipAddress, userAgent)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, shipment)
}

// AssignDriver godoc
//
//	@Summary		Assign driver to shipment
//	@Description	Assign a driver to a shipment
//	@Tags			shipments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string						true	"Shipment ID"
//	@Param			input	body		service.AssignDriverInput	true	"Driver ID"
//	@Success		200		{object}	model.Shipment
//	@Failure		400		{object}	response.Response
//	@Failure		401		{object}	response.Response
//	@Failure		500		{object}	response.Response
//	@Router			/shipments/{id}/assign [put]
func (h *Handler) AssignDriver(w http.ResponseWriter, r *http.Request) {
	shipmentID := chi.URLParam(r, "id")
	tenantID := middleware.GetTenantID(r.Context())
	userID := middleware.GetUserID(r.Context())

	var input service.AssignDriverInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := validation.Get().Struct(input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validation.FormatErrors(err))
		return
	}

	ipAddress := getIPAddress(r)
	userAgent := r.Header.Get("User-Agent")

	shipment, err := h.Shipment.AssignDriver(r.Context(), tenantID, userID, shipmentID, input.DriverID, ipAddress, userAgent)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, shipment)
}

// UpdateShipmentStatus godoc
//
//	@Summary		Update shipment status
//	@Description	Update the status of a shipment
//	@Tags			shipments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string						true	"Shipment ID"
//	@Param			input	body		service.UpdateStatusInput	true	"Status data"
//	@Success		200		{object}	model.Shipment
//	@Failure		400		{object}	response.Response
//	@Failure		401		{object}	response.Response
//	@Failure		500		{object}	response.Response
//	@Router			/shipments/{id}/status [put]
//	@Router			/shipments/{id}/status [patch]
func (h *Handler) UpdateShipmentStatus(w http.ResponseWriter, r *http.Request) {
	shipmentID := chi.URLParam(r, "id")
	tenantID := middleware.GetTenantID(r.Context())
	userID := middleware.GetUserID(r.Context())
	userRole := middleware.GetRole(r.Context())

	var input service.UpdateStatusInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := validation.Get().Struct(input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validation.FormatErrors(err))
		return
	}

	ipAddress := getIPAddress(r)
	userAgent := r.Header.Get("User-Agent")

	shipment, err := h.Shipment.UpdateStatus(r.Context(), tenantID, userID, userRole, shipmentID, input, ipAddress, userAgent)
	if err != nil {
		if err == service.ErrNotAssigned {
			response.ErrorJSON(w, r, http.StatusForbidden, "FORBIDDEN", "You are not assigned to this shipment")
			return
		}
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, shipment)
}

// CancelShipment godoc
//
//	@Summary		Cancel a shipment
//	@Description	Cancel a shipment with a reason
//	@Tags			shipments
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string						true	"Shipment ID"
//	@Param			input	body		service.CancelShipmentInput	true	"Cancellation reason"
//	@Success		200		{object}	model.Shipment
//	@Failure		400		{object}	response.Response
//	@Failure		401		{object}	response.Response
//	@Router			/shipments/{id}/cancel [post]
func (h *Handler) CancelShipment(w http.ResponseWriter, r *http.Request) {
	shipmentID := chi.URLParam(r, "id")
	tenantID := middleware.GetTenantID(r.Context())
	userID := middleware.GetUserID(r.Context())

	var input service.CancelShipmentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := validation.Get().Struct(input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validation.FormatErrors(err))
		return
	}

	ipAddress := getIPAddress(r)
	userAgent := r.Header.Get("User-Agent")

	shipment, err := h.Shipment.Cancel(r.Context(), tenantID, userID, shipmentID, input, ipAddress, userAgent)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, shipment)
}

// SearchShipments godoc
//
//	@Summary		Search shipments
//	@Description	Search shipments with various filters
//	@Tags			shipments
//	@Produce		json
//	@Security		BearerAuth
//	@Param			status		query		string	false	"Filter by status"
//	@Param			driver_id	query		string	false	"Filter by driver ID"
//	@Param			origin		query		string	false	"Filter by origin address"
//	@Param			destination	query		string	false	"Filter by destination address"
//	@Param			q			query		string	false	"Search query (tracking number, customer name, phone)"
//	@Param			date_from	query		string	false	"From date (RFC3339)"
//	@Param			date_to		query		string	false	"To date (RFC3339)"
//	@Param			page		query		int		false	"Page number"		default(1)
//	@Param			per_page	query		int		false	"Items per page"	default(25)
//	@Success		200			{object}	response.Response
//	@Failure		401			{object}	response.Response
//	@Failure		500			{object}	response.Response
//	@Router			/shipments/search [get]
func (h *Handler) SearchShipments(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	input := service.SearchShipmentsInput{
		Status:      r.URL.Query().Get("status"),
		DriverID:    r.URL.Query().Get("driver_id"),
		Origin:      r.URL.Query().Get("origin"),
		Destination: r.URL.Query().Get("destination"),
		Query:       r.URL.Query().Get("q"),
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	input.Page = page

	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}
	input.PerPage = perPage

	if dateFrom := r.URL.Query().Get("date_from"); dateFrom != "" {
		if t, err := time.Parse(time.RFC3339, dateFrom); err == nil {
			input.DateFrom = &t
		}
	}

	if dateTo := r.URL.Query().Get("date_to"); dateTo != "" {
		if t, err := time.Parse(time.RFC3339, dateTo); err == nil {
			input.DateTo = &t
		}
	}

	shipments, total, err := h.Shipment.Search(r.Context(), tenantID, input)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.PaginatedJSON(w, r, http.StatusOK, shipments, page, perPage, total)
}

// TrackShipment godoc
//
//	@Summary		Track a shipment
//	@Description	Get customer-facing shipment status by tracking number (public, unauthenticated). Returns a reduced projection: no customer contact details, cargo information or internal identifiers.
//	@Tags			tracking
//	@Produce		json
//	@Param			tracking_number	path		string	true	"Tracking number"
//	@Success		200				{object}	model.PublicShipment
//	@Failure		404				{object}	response.Response
//	@Router			/track/{tracking_number} [get]
func (h *Handler) TrackShipment(w http.ResponseWriter, r *http.Request) {
	trackingNumber := chi.URLParam(r, "tracking_number")

	shipment, err := h.Shipment.GetByTrackingNumber(r.Context(), trackingNumber)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusNotFound, "NOT_FOUND", "Shipment not found")
		return
	}

	// Unauthenticated endpoint: never serve the full record here.
	response.JSON(w, r, http.StatusOK, shipment.ToPublic())
}
