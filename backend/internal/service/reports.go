package service

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/logistics/internal/db"
)

// ReportService computes the performance reports (FR-034).
type ReportService struct{ *core }

// Report covers one date range.
type Report struct {
	From, To  time.Time // To is exclusive
	Summary   db.ReportSummaryRow
	Daily     []DailyDeliveries
	Drivers   []db.ReportDriverPerformanceRow
	Routes    []db.ReportRoutesRow
	Customers []db.ReportCustomersRow
	// Cancellations counts cancelled shipments by reason.
	Cancellations []db.ReportCancellationsRow
}

// DailyDeliveries is one day of the deliveries chart.
type DailyDeliveries struct {
	Day       time.Time
	Delivered int64
	OnTime    int64
}

// OnTimeRate is the share of deliveries with an ETA that met it, or -1 when
// none had one.
func OnTimeRate(onTime, withETA int64) float64 {
	if withETA == 0 {
		return -1
	}
	return float64(onTime) / float64(withETA)
}

// Range turns a pair of calendar dates in the operating time zone into a
// half-open interval, defaulting to the last 30 days.
func (s *ReportService) Range(from, to string) (time.Time, time.Time) {
	loc := s.cfg.Location
	now := s.now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	end := today.AddDate(0, 0, 1)
	if t, err := time.ParseInLocation("2006-01-02", to, loc); err == nil {
		end = t.AddDate(0, 0, 1)
	}
	start := end.AddDate(0, 0, -30)
	if f, err := time.ParseInLocation("2006-01-02", from, loc); err == nil && f.Before(end) {
		start = f
	}
	if end.Sub(start) > 366*24*time.Hour {
		start = end.AddDate(-1, 0, 0)
	}
	return start, end
}

// Build computes every section of the report.
func (s *ReportService) Build(ctx context.Context, tenantID pgtype.UUID, from, to time.Time) (*Report, error) {
	r := &Report{From: from, To: to}
	f, t := toTimestamp(from), toTimestamp(to)
	var err error
	if r.Summary, err = s.q.ReportSummary(ctx, db.ReportSummaryParams{TenantID: tenantID, DateFrom: f, DateTo: t}); err != nil {
		return nil, err
	}
	daily, err := s.q.ReportDeliveriesPerDay(ctx, db.ReportDeliveriesPerDayParams{
		Tz: s.cfg.Location.String(), TenantID: tenantID, DateFrom: f, DateTo: t,
	})
	if err != nil {
		return nil, err
	}
	byDay := make(map[string]db.ReportDeliveriesPerDayRow, len(daily))
	for _, d := range daily {
		byDay[d.Day.Time.Format("2006-01-02")] = d
	}
	for day := from; day.Before(to); day = day.AddDate(0, 0, 1) {
		row := byDay[day.Format("2006-01-02")]
		r.Daily = append(r.Daily, DailyDeliveries{Day: day, Delivered: row.Delivered, OnTime: row.OnTime})
	}
	if r.Drivers, err = s.q.ReportDriverPerformance(ctx, db.ReportDriverPerformanceParams{TenantID: tenantID, DateFrom: f, DateTo: t}); err != nil {
		return nil, err
	}
	if r.Routes, err = s.q.ReportRoutes(ctx, db.ReportRoutesParams{TenantID: tenantID, DateFrom: f, DateTo: t}); err != nil {
		return nil, err
	}
	if r.Customers, err = s.q.ReportCustomers(ctx, db.ReportCustomersParams{TenantID: tenantID, DateFrom: f, DateTo: t}); err != nil {
		return nil, err
	}
	if r.Cancellations, err = s.q.ReportCancellations(ctx, db.ReportCancellationsParams{TenantID: tenantID, DateFrom: f, DateTo: t}); err != nil {
		return nil, err
	}
	return r, nil
}
