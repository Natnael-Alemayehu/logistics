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

// CreateDriver godoc
// @Summary Create a driver
// @Description Create a new driver account
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param input body service.CreateDriverInput true "Driver data"
// @Success 201 {object} model.User
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /drivers [post]
func (h *Handler) CreateDriver(w http.ResponseWriter, r *http.Request) {
	var input service.CreateDriverInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := validation.Get().Struct(input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validation.FormatErrors(err))
		return
	}

	tenantID := middleware.GetTenantID(r.Context())

	user, err := h.User.CreateDriver(r.Context(), tenantID, input)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusCreated, user)
}

// ListDrivers godoc
// @Summary List drivers
// @Description Get paginated list of drivers
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(25)
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /drivers [get]
func (h *Handler) ListDrivers(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}

	users, total, err := h.User.ListDrivers(r.Context(), tenantID, page, perPage)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.PaginatedJSON(w, r, http.StatusOK, users, page, perPage, total)
}

// CreateUser godoc
// @Summary Create a user
// @Description Create a new user (dispatcher/admin)
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param input body service.CreateUserInput true "User data"
// @Success 201 {object} model.User
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /users [post]
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var input service.CreateUserInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := validation.Get().Struct(input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Validation failed", validation.FormatErrors(err))
		return
	}

	tenantID := middleware.GetTenantID(r.Context())

	user, err := h.User.CreateUser(r.Context(), tenantID, input)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusCreated, user)
}

// ListUsers godoc
// @Summary List users
// @Description Get paginated list of all users (admin only)
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(25)
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /users [get]
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 || perPage > 100 {
		perPage = 25
	}

	users, total, err := h.User.ListUsers(r.Context(), tenantID, page, perPage)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.PaginatedJSON(w, r, http.StatusOK, users, page, perPage, total)
}

type UpdateUserInput struct {
	FullName string `json:"full_name"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
}

// UpdateUser godoc
// @Summary Update user details
// @Description Update user's name, phone, or email. Password change should use /auth/change-password.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Param input body UpdateUserInput true "User update data"
// @Success 200 {object} model.User
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /users/{id} [put]
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	if userID == "" {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "User ID required")
		return
	}

	var input UpdateUserInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorJSON(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	tenantID := middleware.GetTenantID(r.Context())
	requesterID := middleware.GetUserID(r.Context())
	requesterRole := middleware.GetRole(r.Context())
	ipAddress := getIPAddress(r)
	userAgent := r.Header.Get("User-Agent")

	user, err := h.User.UpdateUser(r.Context(), tenantID, requesterID, requesterRole, userID, input.FullName, input.Phone, input.Email, ipAddress, userAgent)
	if err != nil {
		if err == service.ErrUnauthorized {
			response.ErrorJSON(w, r, http.StatusForbidden, "FORBIDDEN", "Not authorized to update this user")
			return
		}
		if err == service.ErrNotFound {
			response.ErrorJSON(w, r, http.StatusNotFound, "NOT_FOUND", "User not found")
			return
		}
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, user)
}
