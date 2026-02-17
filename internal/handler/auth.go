package handler

import (
	"encoding/json"
	"net/http"

	"github.com/natnael-alemayehu/logistics/internal/middleware"
	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/pkg/response"
	"github.com/natnael-alemayehu/logistics/pkg/validation"
)

func getIPAddress(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}

// DriverLogin godoc
// @Summary Driver login
// @Description Authenticate a driver using phone number and PIN
// @Tags auth
// @Accept json
// @Produce json
// @Param input body service.DriverLoginInput true "Login credentials"
// @Success 200 {object} service.LoginOutput
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /auth/login/driver [post]
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

	ipAddress := getIPAddress(r)
	userAgent := r.Header.Get("User-Agent")

	result, err := h.Auth.DriverLogin(r.Context(), input, ipAddress, userAgent)
	if err != nil {
		if err == service.ErrAccountLocked {
			response.ErrorJSON(w, r, http.StatusForbidden, "ACCOUNT_LOCKED", err.Error())
			return
		}
		response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, result)
}

// DispatcherLogin godoc
// @Summary Dispatcher login
// @Description Authenticate a dispatcher or admin using email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param input body service.DispatcherLoginInput true "Login credentials"
// @Success 200 {object} service.LoginOutput
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /auth/login/dispatcher [post]
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

	ipAddress := getIPAddress(r)
	userAgent := r.Header.Get("User-Agent")

	result, err := h.Auth.DispatcherLogin(r.Context(), input, ipAddress, userAgent)
	if err != nil {
		if err == service.ErrAccountLocked {
			response.ErrorJSON(w, r, http.StatusForbidden, "ACCOUNT_LOCKED", err.Error())
			return
		}
		if err == service.ErrPasswordResetRequired {
			response.ErrorJSON(w, r, http.StatusForbidden, "PASSWORD_RESET_REQUIRED", err.Error())
			return
		}
		response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, result)
}

// RefreshToken godoc
// @Summary Refresh access token
// @Description Get a new access token using a refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param input body map[string]string true "Refresh token"
// @Success 200 {object} service.LoginOutput
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /auth/refresh [post]
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
		if err == service.ErrSessionRevoked || err == service.ErrSessionNotFound {
			response.ErrorJSON(w, r, http.StatusUnauthorized, response.CodeSessionRevoked, err.Error())
			return
		}
		response.ErrorJSON(w, r, http.StatusUnauthorized, response.CodeUnauthorized, err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, result)
}

// Logout godoc
// @Summary Logout user
// @Description Logout and revoke the current session
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param input body map[string]string true "Refresh token"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /auth/logout [post]
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := h.Auth.Logout(r.Context(), input.RefreshToken); err != nil {
		if err == service.ErrSessionNotFound {
			response.ErrorJSON(w, r, http.StatusNotFound, response.CodeSessionNotFound, err.Error())
			return
		}
		response.ErrorJSON(w, r, http.StatusInternalServerError, response.CodeInternalError, "Failed to logout")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

// ListSessions godoc
// @Summary List active sessions
// @Description Get all active sessions for the current user
// @Tags sessions
// @Produce json
// @Security BearerAuth
// @Success 200 {array} model.Session
// @Failure 401 {object} response.Response
// @Router /sessions [get]
func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "User not found")
		return
	}

	sessions, err := h.Auth.ListSessions(r.Context(), userID)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list sessions")
		return
	}

	response.JSON(w, r, http.StatusOK, sessions)
}

// RevokeSession godoc
// @Summary Revoke a session
// @Description Revoke a specific session by ID
// @Tags sessions
// @Produce json
// @Security BearerAuth
// @Param id path string true "Session ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /sessions/{id} [delete]
func (h *Handler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "User not found")
		return
	}

	sessionID := r.PathValue("id")
	if sessionID == "" {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Session ID required")
		return
	}

	if err := h.Auth.RevokeSession(r.Context(), userID, sessionID); err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to revoke session")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"message": "Session revoked"})
}

// RevokeOtherSessions godoc
// @Summary Revoke other sessions
// @Description Revoke all sessions except the current one
// @Tags sessions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param input body map[string]string true "Current session ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /sessions/others [delete]
func (h *Handler) RevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		response.ErrorJSON(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "User not found")
		return
	}

	var input struct {
		CurrentSessionID string `json:"current_session_id" validate:"required"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := h.Auth.RevokeOtherSessions(r.Context(), userID, input.CurrentSessionID); err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to revoke sessions")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"message": "Other sessions revoked"})
}
