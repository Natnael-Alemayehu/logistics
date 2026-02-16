package handler

import (
	"encoding/json"
	"net/http"

	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/pkg/response"
	"github.com/natnael-alemayehu/logistics/pkg/validation"
)

func (h *Handler) DriverLogin(w http.ResponseWriter, r *http.Request) {
	var input service.DriverLoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := validation.Get().Struct(input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validation.FormatErrors(err))
		return
	}

	result, err := h.Auth.DriverLogin(r.Context(), input)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, result)
}

func (h *Handler) DispatcherLogin(w http.ResponseWriter, r *http.Request) {
	var input service.DispatcherLoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := validation.Get().Struct(input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validation.FormatErrors(err))
		return
	}

	result, err := h.Auth.DispatcherLogin(r.Context(), input)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, result)
}

func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	result, err := h.Auth.RefreshToken(r.Context(), input.RefreshToken)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, result)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := h.Auth.Logout(r.Context(), input.RefreshToken); err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to logout")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}
