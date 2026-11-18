package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
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
	AlertSMSPaused          = "sms_paused"
	AlertDelivered          = "delivered"
	AlertRouteDeviation     = "route_deviation"
	// AlertLicenceDue, AlertInsuranceDue and AlertInspectionDue are in papers.go.
)

// Severities.
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// resolvedByCompletion are the alerts a shipment's completion makes moot.
var resolvedByCompletion = []string{AlertETAExceeded, AlertDelayReported, AlertIssueReported, AlertDriverOffline, AlertPickupOverdue, AlertLongStop}

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
	AlertSMSPaused:          "Texts paused",
	AlertDelivered:          "Delivered",
	AlertRouteDeviation:     "Moving away from the destination",
	AlertLongStop:           "Long stop",
	AlertLicenceDue:         "Driving licence running out",
	AlertInsuranceDue:       "Insurance running out",
	AlertInspectionDue:      "Inspection (ቦሎ) due",
}

// AlertTypes lists the alert types in the order the account page offers
// them for email, most urgent first.
var AlertTypes = []string{
	AlertIssueReported, AlertDeliveryHeld, AlertCustomerIssue, AlertDeliveryOnCanceled,
	AlertDelayReported, AlertETAExceeded, AlertPickupOverdue, AlertRouteDeviation, AlertLongStop, AlertDriverOffline,
	AlertPODLocation, AlertDeliveredNoPOD, AlertSyncConflict,
	AlertLicenceDue, AlertInsuranceDue, AlertInspectionDue,
	AlertDelivered, AlertDriverMessage, AlertDriverLimitNear, AlertSMSQuotaNear, AlertSMSPaused,
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
	VehicleID  pgtype.UUID
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
	if !c.ruleAllows(ctx, q, a.TenantID, a.Type) {
		return
	}
	row, err := q.CreateAlert(ctx, db.CreateAlertParams{
		TenantID:   a.TenantID,
		ShipmentID: a.ShipmentID,
		DriverID:   a.DriverID,
		VehicleID:  a.VehicleID,
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
	c.smsAlert(ctx, q, after, row)
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
	switch {
	case a.ShipmentID.Valid:
		link = c.cfg.BaseURL + "/app/shipments/" + a.ShipmentID.String()
	case a.VehicleID.Valid:
		link = c.cfg.BaseURL + "/app/vehicles/" + a.VehicleID.String()
	case a.DriverID.Valid:
		link = c.cfg.BaseURL + "/app/drivers/" + a.DriverID.String()
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
	OpenOnly bool
	// Type is one alert type, or empty for all.
	Type string
	// ResolvedSince, when set, keeps the alerts resolved since then.
	ResolvedSince time.Time
	Severities    []string
	Page          int
	PerPage       int
}

// List returns a page of alerts and the total matching.
func (s *AlertService) List(ctx context.Context, tenantID pgtype.UUID, f AlertFilter) ([]db.ListAlertsRow, int64, error) {
	limit, offset := pageBounds(f.Page, f.PerPage)
	sev := f.Severities
	if sev == nil {
		sev = []string{}
	}
	var since pgtype.Timestamptz
	if !f.ResolvedSince.IsZero() {
		since = toTimestamp(f.ResolvedSince)
	}
	rows, err := s.q.ListAlerts(ctx, db.ListAlertsParams{
		TenantID: tenantID, OpenOnly: f.OpenOnly, Severities: sev, AlertType: toText(f.Type), ResolvedSince: since,
		RowLimit: limit, RowOffset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountAlerts(ctx, db.CountAlertsParams{TenantID: tenantID, OpenOnly: f.OpenOnly, Severities: sev,
		AlertType: toText(f.Type), ResolvedSince: since})
	return rows, total, err
}

// Unread counts the open alerts raised since this person last opened the
// alert list, for their header badge. Each person's badge is their own.
func (s *AlertService) Unread(ctx context.Context, p *Principal) (int64, error) {
	return s.q.CountUnreadAlerts(ctx, db.CountUnreadAlertsParams{TenantID: p.TenantID, UserID: p.UserID})
}

// Acknowledge marks an alert handled.
func (s *AlertService) Acknowledge(ctx context.Context, p *Principal, id pgtype.UUID, note string) error {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > 300 {
		return invalid("note", "Keep the note to 300 characters.")
	}
	return s.q.AcknowledgeAlert(ctx, db.AcknowledgeAlertParams{ID: id, TenantID: p.TenantID, AcknowledgedBy: p.UserID, ResolutionNote: toText(note)})
}

// TodayStart is when the company's day began, for "resolved today".
func (s *AlertService) TodayStart() time.Time { return s.startOfDay() }

// MarkSeen clears this person's badge, not anyone else's.
func (s *AlertService) MarkSeen(ctx context.Context, p *Principal) error {
	return s.q.MarkAlertsSeen(ctx, db.MarkAlertsSeenParams{UserID: p.UserID, TenantID: p.TenantID})
}
