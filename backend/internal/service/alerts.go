package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/logistics/internal/db"
)

// Alert types.
const (
	AlertIssueReported      = "issue_reported"
	AlertDelayReported      = "delay_reported"
	AlertDriverOffline      = "driver_offline"
	AlertETAExceeded        = "eta_exceeded"
	AlertPODLocation        = "pod_location_mismatch"
	AlertDeliveredNoPOD     = "delivered_without_pod"
	AlertCustomerIssue      = "customer_reported_problem"
	AlertDriverMessage      = "driver_message"
	AlertSyncConflict       = "sync_conflict"
	AlertDeliveryOnCanceled = "delivery_on_cancelled"
	AlertDeliveryHeld       = "delivery_held"
	AlertPickupOverdue      = "pickup_overdue"
	AlertDriverLimitNear    = "driver_limit_near"
	AlertSMSQuotaNear       = "sms_quota_near"
	AlertDelivered          = "delivered"
	AlertRouteDeviation     = "route_deviation"
)

// Severities.
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// resolvedByCompletion are the alerts a shipment's completion makes moot.
var resolvedByCompletion = []string{AlertETAExceeded, AlertDelayReported, AlertIssueReported, AlertDriverOffline, AlertPickupOverdue}

var alertLabels = map[string]string{
	AlertIssueReported:      "Issue reported",
	AlertDelayReported:      "Delay reported",
	AlertDriverOffline:      "Driver offline",
	AlertETAExceeded:        "Past ETA",
	AlertPODLocation:        "Delivery location mismatch",
	AlertDeliveredNoPOD:     "Delivered without proof",
	AlertCustomerIssue:      "Customer reported a problem",
	AlertDriverMessage:      "Message from driver",
	AlertSyncConflict:       "Offline update not applied",
	AlertDeliveryOnCanceled: "Delivery recorded on a cancelled shipment",
	AlertDeliveryHeld:       "Delivery needs review",
	AlertPickupOverdue:      "Not collected",
	AlertDriverLimitNear:    "Driver places running out",
	AlertSMSQuotaNear:       "Text allowance running out",
	AlertDelivered:          "Delivered",
	AlertRouteDeviation:     "Moving away from the destination",
}

// AlertTypes lists the alert types in the order the account page offers
// them for email, most urgent first.
var AlertTypes = []string{
	AlertIssueReported, AlertDeliveryHeld, AlertCustomerIssue, AlertDeliveryOnCanceled,
	AlertDelayReported, AlertETAExceeded, AlertPickupOverdue, AlertRouteDeviation, AlertDriverOffline,
	AlertPODLocation, AlertDeliveredNoPOD, AlertSyncConflict,
	AlertDelivered, AlertDriverMessage, AlertDriverLimitNear, AlertSMSQuotaNear,
}

// AlertLabel is the display name of an alert type.
func AlertLabel(t string) string {
	if l, ok := alertLabels[t]; ok {
		return l
	}
	return t
}

// AlertService is the dispatcher notification centre (FR-033).
type AlertService struct{ *core }

type newAlert struct {
	TenantID   pgtype.UUID
	ShipmentID pgtype.UUID
	DriverID   pgtype.UUID
	Type       string
	Severity   string
	Message    string
	// Episode, for alerts raised by a periodic check, names the situation the
	// alert is about, such as the ETA that passed. The check raises it once:
	// dismissed, it does not return until the episode changes.
	Episode string
}

// raise records an alert within the caller's transaction and pushes it to
// dashboards after commit. An identical open alert, or an earlier alert of
// the same episode, makes this a no-op.
func (c *core) raise(ctx context.Context, q *db.Queries, after *afterCommit, a newAlert) {
	row, err := q.CreateAlert(ctx, db.CreateAlertParams{
		TenantID:   a.TenantID,
		ShipmentID: a.ShipmentID,
		DriverID:   a.DriverID,
		AlertType:  a.Type,
		Severity:   a.Severity,
		Message:    a.Message,
		Episode:    toText(a.Episode),
	})
	if isNoRows(err) {
		return
	}
	if err != nil {
		c.log.Error().Err(err).Str("type", a.Type).Msg("create alert")
		return
	}
	c.emailAlert(ctx, q, after, row)
	after.do(func() {
		c.publish(row.TenantID, "alert", map[string]any{
			"id":          row.ID.String(),
			"type":        row.AlertType,
			"label":       AlertLabel(row.AlertType),
			"severity":    row.Severity,
			"message":     row.Message,
			"shipment_id": uuidString(row.ShipmentID),
		})
	})
}

// emailAlert sends a copy of an alert to the office staff who asked for
// alerts of its type by email (FR-033 AC-033.4).
func (c *core) emailAlert(ctx context.Context, q *db.Queries, after *afterCommit, a db.Alert) {
	to, err := q.ListAlertEmailRecipients(ctx, db.ListAlertEmailRecipientsParams{TenantID: a.TenantID, AlertType: a.AlertType})
	if err != nil {
		c.log.Error().Err(err).Msg("alert email recipients")
		return
	}
	link := c.cfg.BaseURL + "/app/alerts"
	if a.ShipmentID.Valid {
		link = c.cfg.BaseURL + "/app/shipments/" + a.ShipmentID.String()
	}
	for _, r := range to {
		c.enqueueEmail(ctx, q, after, outboundEmail{
			TenantID: a.TenantID, To: deref(r.Email), Purpose: EmailAlert,
			Subject: AlertLabel(a.AlertType) + ": " + truncate(a.Message, 80),
			Body: c.emailBody(r.FullName, a.Message, "Open it in "+c.cfg.AppName+": "+link,
				"You get this because you chose to have these alerts emailed. Change that under Account in "+c.cfg.AppName+"."),
		})
	}
}

// SetAlertEmails chooses which alert types are emailed to the caller.
func (s *AlertService) SetAlertEmails(ctx context.Context, p *Principal, types []string) error {
	if p.IsDriver() || p.IsPlatformAdmin() {
		return ErrUnauthorized
	}
	keep := []string{}
	for _, t := range AlertTypes {
		if contains(types, t) {
			keep = append(keep, t)
		}
	}
	return s.q.SetAlertEmails(ctx, db.SetAlertEmailsParams{AlertTypes: keep, ID: p.UserID, TenantID: p.TenantID})
}

// AlertFilter narrows the alert list.
type AlertFilter struct {
	OpenOnly   bool
	Severities []string
	Page       int
	PerPage    int
}

// List returns a page of alerts and the total matching.
func (s *AlertService) List(ctx context.Context, tenantID pgtype.UUID, f AlertFilter) ([]db.ListAlertsRow, int64, error) {
	limit, offset := pageBounds(f.Page, f.PerPage)
	sev := f.Severities
	if sev == nil {
		sev = []string{}
	}
	rows, err := s.q.ListAlerts(ctx, db.ListAlertsParams{
		TenantID: tenantID, OpenOnly: f.OpenOnly, Severities: sev, RowLimit: limit, RowOffset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountAlerts(ctx, db.CountAlertsParams{TenantID: tenantID, OpenOnly: f.OpenOnly, Severities: sev})
	return rows, total, err
}

// Unread counts alerts nobody has looked at, for the header badge.
func (s *AlertService) Unread(ctx context.Context, tenantID pgtype.UUID) (int64, error) {
	return s.q.CountUnreadAlerts(ctx, tenantID)
}

// Acknowledge marks an alert handled.
func (s *AlertService) Acknowledge(ctx context.Context, p *Principal, id pgtype.UUID) error {
	return s.q.AcknowledgeAlert(ctx, db.AcknowledgeAlertParams{ID: id, TenantID: p.TenantID, AcknowledgedBy: p.UserID})
}

// MarkAllRead clears the unread badge.
func (s *AlertService) MarkAllRead(ctx context.Context, tenantID pgtype.UUID) error {
	return s.q.MarkAllAlertsRead(ctx, tenantID)
}
