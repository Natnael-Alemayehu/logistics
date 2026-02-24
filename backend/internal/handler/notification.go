package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/natnael-alemayehu/logistics/internal/middleware"
	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/pkg/response"
	"github.com/natnael-alemayehu/logistics/pkg/validation"
)

type RegisterPushTokenInput struct {
	DeviceID   string `json:"device_id" validate:"required"`
	PushToken  string `json:"push_token" validate:"required"`
	Platform   string `json:"platform" validate:"required,oneof=ios android web"`
	AppVersion string `json:"app_version"`
}

// RegisterPushToken godoc
// @Summary Register push notification token
// @Description Register or update a device's push notification token
// @Tags notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param input body RegisterPushTokenInput true "Push token data"
// @Success 200 {object} model.DeviceToken
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /notifications/register [post]
func (h *Handler) RegisterPushToken(w http.ResponseWriter, r *http.Request) {
	var input RegisterPushTokenInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := validation.Get().Struct(input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validation.FormatErrors(err))
		return
	}

	userID := middleware.GetUserID(r.Context())

	token, err := h.DeviceToken.Register(r.Context(), userID, service.RegisterPushTokenInput{
		DeviceID:   input.DeviceID,
		PushToken:  input.PushToken,
		Platform:   input.Platform,
		AppVersion: input.AppVersion,
	})
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, token)
}

// UnregisterDevice godoc
// @Summary Unregister a device
// @Description Remove a device's push notification token
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Param deviceId path string true "Device ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /notifications/device/{deviceId} [delete]
func (h *Handler) UnregisterDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceId")
	if deviceID == "" {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Device ID required")
		return
	}

	userID := middleware.GetUserID(r.Context())

	if err := h.DeviceToken.Unregister(r.Context(), userID, deviceID); err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"message": "Device unregistered successfully"})
}
