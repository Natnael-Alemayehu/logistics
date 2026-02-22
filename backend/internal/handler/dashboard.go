package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/middleware"
	"github.com/natnael-alemayehu/logistics/pkg/response"
)

type DashboardService struct {
	queries *db.Queries
}

func NewDashboardService(queries *db.Queries) *DashboardService {
	return &DashboardService{queries: queries}
}

type DashboardStatsOutput struct {
	ActiveShipments int64 `json:"active_shipments"`
	DriversOnDuty   int64 `json:"drivers_on_duty"`
	DeliveriesToday int64 `json:"deliveries_today"`
	IssuesCount     int64 `json:"issues_count"`
}

type AlertOutput struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type ActivityOutput struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type DriverLocationOutput struct {
	DriverID   string    `json:"driver_id"`
	DriverName string    `json:"driver_name"`
	Lat        float64   `json:"lat"`
	Lng        float64   `json:"lng"`
	LastUpdate time.Time `json:"last_update"`
	Status     string    `json:"status"`
}

func (s *DashboardService) GetStats(ctx context.Context, tenantID string) (*DashboardStatsOutput, error) {
	tid := toUUID(tenantID)
	startOfDay := getStartOfDay()

	activeShipments, err := s.queries.CountActiveShipmentsByTenant(ctx, db.CountActiveShipmentsByTenantParams{
		TenantID: tid,
		Column2:  []string{"assigned", "in_transit", "delayed", "arrived"},
	})
	if err != nil {
		activeShipments = 0
	}

	driversOnDuty, err := s.queries.CountDriversWithActiveShipments(ctx, tid)
	if err != nil {
		driversOnDuty = 0
	}

	deliveriesToday, err := s.queries.CountDeliveriesToday(ctx, db.CountDeliveriesTodayParams{
		TenantID:       tid,
		ActualDelivery: pgtype.Timestamptz{Time: startOfDay, Valid: true},
	})
	if err != nil {
		deliveriesToday = 0
	}

	issuesCount, err := s.queries.CountShipmentsByStatus(ctx, db.CountShipmentsByStatusParams{
		TenantID: tid,
		Status:   "issue",
	})
	if err != nil {
		issuesCount = 0
	}

	return &DashboardStatsOutput{
		ActiveShipments: activeShipments,
		DriversOnDuty:   driversOnDuty,
		DeliveriesToday: deliveriesToday,
		IssuesCount:     issuesCount,
	}, nil
}

func (s *DashboardService) GetAlerts(ctx context.Context, tenantID string) ([]AlertOutput, error) {
	tid := toUUID(tenantID)

	shipments, err := s.queries.GetDashboardAlerts(ctx, db.GetDashboardAlertsParams{
		TenantID: tid,
		Limit:    10,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get alerts: %w", err)
	}

	var alerts []AlertOutput
	for _, sh := range shipments {
		msg := fmt.Sprintf("Shipment %s is %s", sh.TrackingNumber, sh.Status)
		if sh.StatusNote != nil && *sh.StatusNote != "" {
			msg += ": " + *sh.StatusNote
		}
		alerts = append(alerts, AlertOutput{
			ID:        sh.ID.String(),
			Message:   msg,
			CreatedAt: sh.UpdatedAt.Time,
		})
	}

	return alerts, nil
}

func (s *DashboardService) GetActivity(ctx context.Context, tenantID string) ([]ActivityOutput, error) {
	tid := toUUID(tenantID)

	shipments, err := s.queries.GetDashboardActivity(ctx, db.GetDashboardActivityParams{
		TenantID: tid,
		Limit:    10,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get activity: %w", err)
	}

	var activity []ActivityOutput
	for _, sh := range shipments {
		msg := fmt.Sprintf("Shipment %s status updated to %s", sh.TrackingNumber, sh.Status)
		if sh.Status == "pending" {
			msg = fmt.Sprintf("New shipment %s created", sh.TrackingNumber)
		}

		activity = append(activity, ActivityOutput{
			ID:          sh.ID.String(),
			Description: msg,
			CreatedAt:   sh.UpdatedAt.Time,
		})
	}

	return activity, nil
}

func (s *DashboardService) GetLocations(ctx context.Context, tenantID string) ([]DriverLocationOutput, error) {
	tid := toUUID(tenantID)

	locations, err := s.queries.GetLatestDriverLocations(ctx, tid)
	if err != nil {
		return nil, fmt.Errorf("failed to get driver locations: %w", err)
	}

	var outputs []DriverLocationOutput
	for _, loc := range locations {
		outputs = append(outputs, DriverLocationOutput{
			DriverID:   loc.DriverID.String(),
			DriverName: loc.DriverName,
			Lat:        loc.Latitude.(float64),
			Lng:        loc.Longitude.(float64),
			LastUpdate: loc.RecordedAt.Time,
			Status:     "Active",
		})
	}

	return outputs, nil
}

func getStartOfDay() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func toUUID(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{}
	}
	var u pgtype.UUID
	u.Scan(s)
	return u
}

// GetDashboardStats godoc
// @Summary Get dashboard statistics
// @Description Get overview statistics for the dispatcher dashboard including active shipments, drivers on duty, deliveries today, and issues count
// @Tags dashboard
// @Produce json
// @Security BearerAuth
// @Success 200 {object} DashboardStatsOutput
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /dashboard/stats [get]
func (h *Handler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	stats, err := h.Dashboard.GetStats(r.Context(), tenantID)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.JSON(w, r, http.StatusOK, stats)
}

// GetDashboardAlerts godoc
// @Summary Get dashboard alerts
// @Description Get recent alerts for the dispatcher dashboard
// @Tags dashboard
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string][]AlertOutput
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /dashboard/alerts [get]
func (h *Handler) GetDashboardAlerts(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	alerts, err := h.Dashboard.GetAlerts(r.Context(), tenantID)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	if alerts == nil {
		alerts = []AlertOutput{}
	}

	response.JSON(w, r, http.StatusOK, map[string]interface{}{"alerts": alerts})
}

// GetDashboardActivity godoc
// @Summary Get dashboard activity
// @Description Get recent activity for the dispatcher dashboard
// @Tags dashboard
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string][]ActivityOutput
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /dashboard/activity [get]
func (h *Handler) GetDashboardActivity(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	activity, err := h.Dashboard.GetActivity(r.Context(), tenantID)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	if activity == nil {
		activity = []ActivityOutput{}
	}

	response.JSON(w, r, http.StatusOK, map[string]interface{}{"events": activity})
}

// GetDriverLocations godoc
// @Summary Get latest driver locations
// @Description Get the latest known locations of all drivers
// @Tags drivers
// @Produce json
// @Security BearerAuth
// @Success 200 {array} DriverLocationOutput
// @Failure 401 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /drivers/locations [get]
func (h *Handler) GetDriverLocations(w http.ResponseWriter, r *http.Request) {
	tenantID := middleware.GetTenantID(r.Context())

	locations, err := h.Dashboard.GetLocations(r.Context(), tenantID)
	if err != nil {
		response.ErrorJSON(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	if locations == nil {
		locations = []DriverLocationOutput{}
	}

	response.JSON(w, r, http.StatusOK, locations)
}
