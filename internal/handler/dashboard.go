package handler

import (
	"context"
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
