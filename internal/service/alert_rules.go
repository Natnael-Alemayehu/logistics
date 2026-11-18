package service

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/geo"
)

// AlertLongStop is a truck on the road standing still outside a town.
const AlertLongStop = "long_stop"

// PurposeAlert is an alert texted to the office.
const PurposeAlert = "alert"

// AlertRules are what a company is alerted about on the road, and when
// (D-Alerts). A company that never changed them has DefaultAlertRules,
// which are what the checks did before the rules could be set.
type AlertRules struct {
	ProblemsOn     bool
	LateOn         bool
	LateMinutes    int
	OfflineOn      bool
	OfflineMinutes int
	StopOn         bool
	StopMinutes    int
	RouteOn        bool
	RouteKm        int
}

// DefaultAlertRules match the migration's column defaults.
var DefaultAlertRules = AlertRules{
	ProblemsOn: true, LateOn: true, LateMinutes: 0, OfflineOn: true, OfflineMinutes: 240,
	StopOn: true, StopMinutes: 45, RouteOn: true, RouteKm: 20,
}

// stopTownRadius is how close to a town's centre a stop counts as in the
// town, where trucks stand to load, unload and wait without anything wrong.
const stopTownRadius = 3000.0

// problemAlerts are the alerts a driver's report raises, which the
// company's ProblemsOn rule turns off.
var problemAlerts = []string{AlertIssueReported, AlertDelayReported}

// ProblemAlertTypes are the alerts that something has gone wrong with a
// load, for a person who wants only those.
var ProblemAlertTypes = []string{
	AlertIssueReported, AlertDeliveryHeld, AlertCustomerIssue, AlertDeliveryOnCanceled, AlertDelayReported,
}

func rulesFrom(r db.AlertRule) AlertRules {
	return AlertRules{
		ProblemsOn: r.ProblemsOn, LateOn: r.LateOn, LateMinutes: int(r.LateMinutes),
		OfflineOn: r.OfflineOn, OfflineMinutes: int(r.OfflineMinutes),
		StopOn: r.StopOn, StopMinutes: int(r.StopMinutes), RouteOn: r.RouteOn, RouteKm: int(r.RouteKm),
	}
}

func (c *core) alertRules(ctx context.Context, q *db.Queries, tenantID pgtype.UUID) (AlertRules, error) {
	r, err := q.GetAlertRules(ctx, tenantID)
	if isNoRows(err) {
		return DefaultAlertRules, nil
	}
	if err != nil {
		return DefaultAlertRules, err
	}
	return rulesFrom(r), nil
}

// Rules are the company's alert rules.
func (s *AlertService) Rules(ctx context.Context, tenantID pgtype.UUID) (AlertRules, error) {
	return s.alertRules(ctx, s.q, tenantID)
}

// SetRules changes the company's alert rules. Each threshold has bounds
// that keep the alert useful: a minute late is noise, a day late is news.
func (s *AlertService) SetRules(ctx context.Context, p *Principal, in AlertRules, ci ClientInfo) error {
	if !p.CanManageCompany() {
		return ErrUnauthorized
	}
	for _, c := range []struct {
		field     string
		v, lo, hi int
		what      string
	}{
		{"late_minutes", in.LateMinutes, 0, 1440, "Running late takes 0 to 1,440 minutes."},
		{"offline_minutes", in.OfflineMinutes, 15, 1440, "Phone offline takes 15 to 1,440 minutes."},
		{"stop_minutes", in.StopMinutes, 10, 720, "A long stop takes 10 to 720 minutes."},
		{"route_km", in.RouteKm, 2, 200, "Off the route takes 2 to 200 km."},
	} {
		if c.v < c.lo || c.v > c.hi {
			return invalid(c.field, c.what)
		}
	}
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		before, err := s.alertRules(ctx, q, p.TenantID)
		if err != nil {
			return err
		}
		err = q.UpsertAlertRules(ctx, db.UpsertAlertRulesParams{
			TenantID: p.TenantID, ProblemsOn: in.ProblemsOn, LateOn: in.LateOn, LateMinutes: int32(in.LateMinutes),
			OfflineOn: in.OfflineOn, OfflineMinutes: int32(in.OfflineMinutes), StopOn: in.StopOn, StopMinutes: int32(in.StopMinutes),
			RouteOn: in.RouteOn, RouteKm: int32(in.RouteKm), UpdatedBy: p.UserID,
		})
		if err != nil {
			return err
		}
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "alert_rules_changed", "tenant", p.TenantID, before, in)
		return nil
	})
}

// ruleAllows is whether the company's rules let an alert of this type be
// raised. The periodic checks apply their own rules in their queries; this
// covers what a driver reports.
func (c *core) ruleAllows(ctx context.Context, q *db.Queries, tenantID pgtype.UUID, alertType string) bool {
	if !slices.Contains(problemAlerts, alertType) {
		return true
	}
	r, err := c.alertRules(ctx, q, tenantID)
	if err != nil {
		c.log.Error().Err(err).Msg("alert rules")
		return true
	}
	return r.ProblemsOn
}

// CheckStops alerts on trucks that have stood still outside a town for
// longer than their company's rule allows, and resolves the alert once the
// truck moves on.
func (j *Jobs) CheckStops(ctx context.Context) error {
	stops, err := j.q.ListLongStops(ctx)
	if err != nil {
		return err
	}
	now := j.now()
	still := []pgtype.UUID{}
	var raise []newAlert
	for _, st := range stops {
		place, m := geo.Nearest(st.Latitude, st.Longitude)
		if m < stopTownRadius {
			continue
		}
		still = append(still, st.ID)
		where := "on the road"
		if place.Name != "" {
			where = fmt.Sprintf("%s from %s", formatDistance(m), place.Name)
		}
		raise = append(raise, newAlert{
			TenantID: st.TenantID, ShipmentID: st.ID, DriverID: st.DriverID, Type: AlertLongStop, Severity: SeverityWarning,
			Message: fmt.Sprintf("%s has stood still for %s, %s.", st.TrackingNumber, humanDuration(now.Sub(st.StoppedSince.Time)), where),
			Episode: st.Episode,
		})
	}
	return j.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		if err := q.AcknowledgeMovedStops(ctx, still); err != nil {
			return err
		}
		for _, a := range raise {
			j.raise(ctx, q, after, a)
		}
		return nil
	})
}

// smsAlert texts an alert to the office staff who asked for alerts of its
// type by SMS. It counts against the company's texts like any other.
func (c *core) smsAlert(ctx context.Context, q *db.Queries, after *afterCommit, a db.Alert) {
	to, err := q.ListAlertSMSRecipients(ctx, db.ListAlertSMSRecipientsParams{TenantID: a.TenantID, AlertType: a.AlertType})
	if err != nil {
		c.log.Error().Err(err).Msg("alert sms recipients")
		return
	}
	if len(to) == 0 {
		return
	}
	body := AlertLabel(a.AlertType) + ": " + truncate(a.Message, 220)
	if a.ShipmentID.Valid {
		body += " " + c.cfg.BaseURL + "/app/shipments/" + a.ShipmentID.String()
	}
	for _, r := range to {
		c.enqueueSMS(ctx, q, after, outboundSMS{TenantID: a.TenantID, ShipmentID: a.ShipmentID, Phone: deref(r.Phone), Body: body, Purpose: PurposeAlert})
	}
}

// AlertChannel is how much of the alerts a person gets one way.
const (
	ChannelAll      = "all"
	ChannelProblems = "problems"
	ChannelNone     = "none"
	// ChannelOwn is the person's own choice of types, made on their
	// account page, which matches neither all nor problems.
	ChannelOwn = "own"
)

// ChannelOf names the set of alert types a person chose.
func ChannelOf(types []string) string {
	has := func(want []string) bool {
		if len(types) != len(want) {
			return false
		}
		for _, t := range want {
			if !slices.Contains(types, t) {
				return false
			}
		}
		return true
	}
	switch {
	case len(types) == 0:
		return ChannelNone
	case has(AlertTypes):
		return ChannelAll
	case has(ProblemAlertTypes):
		return ChannelProblems
	}
	return ChannelOwn
}

func channelTypes(channel string, own []string) ([]string, error) {
	switch channel {
	case ChannelAll:
		return AlertTypes, nil
	case ChannelProblems:
		return ProblemAlertTypes, nil
	case ChannelNone:
		return []string{}, nil
	case ChannelOwn, "":
		return own, nil
	}
	return nil, invalid("channel", "Choose how each person gets alerts.")
}

// Recipient is how one member of staff gets alerts.
type Recipient struct {
	UserID pgtype.UUID
	Email  string // a channel
	SMS    string
}

// SetRecipients sets how each member of staff gets alerts by email and by
// SMS (D-Alerts, "who gets them"). Anyone not listed is left as they are.
func (s *AlertService) SetRecipients(ctx context.Context, p *Principal, in []Recipient, ci ClientInfo) error {
	if !p.CanManageCompany() {
		return ErrUnauthorized
	}
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		changes := map[string]any{}
		for _, r := range in {
			u, err := q.GetUserByID(ctx, db.GetUserByIDParams{ID: r.UserID, TenantID: p.TenantID})
			if err != nil || u.Role == "driver" {
				return notFound("Person not found.")
			}
			emails, err := channelTypes(r.Email, u.AlertEmails)
			if err != nil {
				return err
			}
			texts, err := channelTypes(r.SMS, u.AlertSms)
			if err != nil {
				return err
			}
			if len(texts) > 0 && u.Phone == nil {
				return invalid("sms", u.FullName+" has no phone number for texts. Add one on the Team page first.")
			}
			if err := q.SetAlertChannels(ctx, db.SetAlertChannelsParams{ID: u.ID, TenantID: p.TenantID, AlertEmails: emails, AlertSms: texts}); err != nil {
				return err
			}
			changes[u.FullName] = map[string]string{"email": ChannelOf(emails), "sms": ChannelOf(texts)}
		}
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "alert_recipients_changed", "tenant", p.TenantID, nil, changes)
		return nil
	})
}

// startOfDay is midnight today in the company's time zone.
func (c *core) startOfDay() time.Time {
	t := c.Today()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
