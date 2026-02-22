package handler

import (
	"encoding/json"
	"net/http"

	"github.com/natnael-alemayehu/logistics/internal/middleware"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/pkg/response"
)

// Sync godoc
// @Summary Sync offline data
// @Description Sync offline data from driver app (events, PODs, status updates)
// @Tags sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param input body model.SyncRequest true "Sync data"
// @Success 200 {object} model.SyncResponse
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /sync [post]
func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	var req model.SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	tenantID := middleware.GetTenantID(r.Context())
	driverID := middleware.GetUserID(r.Context())

	result, err := h.SyncSvc.Sync(r.Context(), tenantID, driverID, req)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, result)
}
