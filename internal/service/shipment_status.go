package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
)

// StatusChange is a request to move a shipment along its lifecycle.
type StatusChange struct {
	Status string
	Reason string
	Note   string
	Lat    *float64
	Lng    *float64
	// OccurredAt is when it happened, for updates recorded offline. Zero
	// means now.
	OccurredAt time.Time
	// By decides which transitions are allowed.
	By model.Actor
	// ClientID is the device's identifier for the update, making replays
	// harmless.
	ClientID string

	recipient string // set when the change is the result of a POD
	// deliveredAt is when the proof was recorded, for a delivery the office
	// accepts later.
	deliveredAt time.Time
}

// ChangeStatus applies a status change.
//
// It returns model.ErrSameStatus when the shipment is already in that status,
// which callers treat as success, and ErrStaleUpdate when the update was
// recorded before the shipment's current state was set: the update is kept
// in the timeline as a conflict but not applied.
func (s *ShipmentService) ChangeStatus(ctx context.Context, a Actor, id pgtype.UUID, ch StatusChange) (db.Shipment, error) {
	var out db.Shipment
	var result error
	err := s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		cur, err := s.lock(ctx, q, a.TenantID, id)
		if err != nil {
			return err
		}
		out, result = s.applyStatus(ctx, q, after, a, cur, ch)
		if errors.Is(result, ErrStaleUpdate) {
			return nil // commit the conflict record
		}
		return result
	})
	if err != nil {
		return out, err
	}
	return out, result
}

// applyStatus is the single place a shipment's status changes, on a row the
// caller has locked.
func (s *ShipmentService) applyStatus(ctx context.Context, q *db.Queries, after *afterCommit, a Actor, cur db.Shipment, ch StatusChange) (db.Shipment, error) {
	// A change the device sent before, whose answer it never received, gets
	// the same answer again, and nothing happens twice.
	if ch.ClientID != "" {
		kind, err := q.GetShipmentEventByClientID(ctx, db.GetShipmentEventByClientIDParams{
			TenantID: cur.TenantID, ShipmentID: cur.ID, ClientID: ch.ClientID,
		})
		switch {
		case err == nil && kind == "conflict":
			return cur, ErrStaleUpdate
		case err == nil:
			return cur, nil
		case !isNoRows(err):
			return cur, err
		}
	}
	if a.isDriver() && cur.DriverID != a.UserID {
		return cur, ErrNotAssigned
	}

	terr := model.CheckTransition(cur.Status, ch.Status, ch.By)
	if errors.Is(terr, model.ErrSameStatus) {
		return s.updateStatusDetail(ctx, q, after, a, cur, ch)
	}

	now := s.now()
	occurred := now
	if !ch.OccurredAt.IsZero() && ch.OccurredAt.Before(now) {
		occurred = ch.OccurredAt
	}

	// An offline update that happened before the shipment's current state was
	// set — typically because the office changed it in the meantime — must
	// not roll that state back, whether or not the move would be allowed from
	// the new state. Proof of delivery is exempt: a signature is stronger
	// evidence than anything recorded since. So is the office accepting one.
	if ch.By != model.ActorPOD && ch.By != model.ActorReview && cur.StatusChangedAt.Valid && occurred.Before(cur.StatusChangedAt.Time) {
		reason := strings.TrimSpace(ch.Reason)
		if !contains(model.ReasonsFor(ch.Status), reason) {
			reason = ""
		}
		recorded := s.event(ctx, q, a, cur, shipmentEvent{
			Type: "conflict", From: cur.Status, To: ch.Status, Reason: reason, ClientID: ch.ClientID,
			Lat: ch.Lat, Lng: ch.Lng, OccurredAt: occurred,
			Note: fmt.Sprintf("%s update from %s was not applied: the shipment had already changed to %s.",
				model.StatusLabel(ch.Status), occurred.In(s.cfg.Location).Format("2 Jan 15:04"), model.StatusLabel(cur.Status)),
		})
		if recorded && (ch.Status == model.StatusIssue || ch.Status == model.StatusDelayed) {
			s.raise(ctx, q, after, newAlert{
				TenantID: cur.TenantID, ShipmentID: cur.ID, DriverID: cur.DriverID, Type: AlertSyncConflict, Severity: SeverityWarning,
				Message: fmt.Sprintf("%s reported %s on %s while offline, after the shipment had been changed to %s.",
					a.Name, strings.ToLower(model.StatusLabel(ch.Status)), cur.TrackingNumber, model.StatusLabel(cur.Status)),
			})
		}
		return cur, ErrStaleUpdate
	}

	switch err := terr; {
	case err == nil:
	case errors.Is(err, model.ErrPODRequired):
		return cur, invalid("status", "Record proof of delivery to complete this shipment.")
	default:
		var te *model.TransitionError
		if errors.As(err, &te) {
			return cur, conflict(capitalise(te.Error()) + ".")
		}
		return cur, invalid("status", "Choose a valid status.")
	}

	reason := strings.TrimSpace(ch.Reason)
	if reasons := model.ReasonsFor(ch.Status); reasons != nil && ch.Status != model.StatusCancelled {
		if reason == "" {
			if ch.By != model.ActorDriverSMS {
				return cur, invalid("reason", "Choose a reason.")
			}
			reason = "other"
		}
		if !contains(reasons, reason) {
			return cur, invalid("reason", "Choose a reason from the list.")
		}
	} else {
		reason = ""
	}
	note := strings.TrimSpace(truncate(ch.Note, 1000))

	out, err := q.SetShipmentStatus(ctx, db.SetShipmentStatusParams{
		Status: ch.Status, StatusNote: toText(note), StatusReason: toText(reason),
		ChangedAt: toTimestamp(occurred), DeliveredAt: toTimestamp(ch.deliveredAt), ID: cur.ID, TenantID: cur.TenantID,
	})
	if err != nil {
		return cur, err
	}

	firstDeparture := ch.Status == model.StatusInTransit && !cur.PickedUpAt.Valid
	if firstDeparture && !out.EtaManual && out.OriginLat != nil && out.DestinationLat != nil {
		eta := s.estimateArrival(ctx, q, out.TenantID, out.OriginCity, out.DestinationCity,
			*out.OriginLat, *out.OriginLng, *out.DestinationLat, *out.DestinationLng, occurred)
		if err := q.SetShipmentETA(ctx, db.SetShipmentETAParams{ID: out.ID, TenantID: out.TenantID, EstimatedDelivery: toTimestamp(eta)}); err == nil {
			out.EstimatedDelivery = toTimestamp(eta)
		}
	}

	s.event(ctx, q, a, out, shipmentEvent{
		Type: "status_changed", From: cur.Status, To: ch.Status, Reason: reason, Note: note,
		Lat: ch.Lat, Lng: ch.Lng, Public: true, ClientID: ch.ClientID, OccurredAt: occurred,
		DriverID: out.DriverID,
	})

	who := a.Name
	if who == "" {
		who = "The driver"
	}
	switch ch.Status {
	// A report replaces an open alert about an earlier problem or delay on
	// the load, which would otherwise hide it: a breakdown, then a delay,
	// then an accident is news three times.
	case model.StatusIssue:
		_ = q.AcknowledgeShipmentAlerts(ctx, db.AcknowledgeShipmentAlertsParams{TenantID: out.TenantID, ShipmentID: out.ID, AlertTypes: []string{AlertIssueReported}})
		s.raise(ctx, q, after, newAlert{
			TenantID: out.TenantID, ShipmentID: out.ID, DriverID: out.DriverID, Type: AlertIssueReported, Severity: SeverityCritical,
			Message: fmt.Sprintf("%s reported a problem on %s: %s%s", who, out.TrackingNumber, model.ReasonLabel(reason), noteSuffix(note)),
		})
	case model.StatusDelayed:
		_ = q.AcknowledgeShipmentAlerts(ctx, db.AcknowledgeShipmentAlertsParams{TenantID: out.TenantID, ShipmentID: out.ID, AlertTypes: []string{AlertDelayReported}})
		s.raise(ctx, q, after, newAlert{
			TenantID: out.TenantID, ShipmentID: out.ID, DriverID: out.DriverID, Type: AlertDelayReported, Severity: SeverityWarning,
			Message: fmt.Sprintf("%s reported a delay on %s: %s%s", who, out.TrackingNumber, model.ReasonLabel(reason), noteSuffix(note)),
		})
	case model.StatusInTransit:
		_ = q.AcknowledgeShipmentAlerts(ctx, db.AcknowledgeShipmentAlertsParams{
			TenantID: out.TenantID, ShipmentID: out.ID, AlertTypes: []string{AlertDelayReported, AlertIssueReported},
		})
	case model.StatusDelivered:
		_ = q.AcknowledgeShipmentAlerts(ctx, db.AcknowledgeShipmentAlertsParams{TenantID: out.TenantID, ShipmentID: out.ID, AlertTypes: resolvedByCompletion})
		if ch.By == model.ActorDriverSMS {
			s.raise(ctx, q, after, newAlert{
				TenantID: out.TenantID, ShipmentID: out.ID, DriverID: out.DriverID, Type: AlertDeliveredNoPOD, Severity: SeverityWarning,
				Message: fmt.Sprintf("%s reported %s delivered by SMS, without a signature or photos. Confirm with the customer.", who, out.TrackingNumber),
			})
		}
		if out.ConfirmToken == nil {
			token := randomToken(18)
			if err := q.SetShipmentConfirmToken(ctx, db.SetShipmentConfirmTokenParams{ID: out.ID, TenantID: out.TenantID, ConfirmToken: &token}); err == nil {
				out.ConfirmToken = &token
			}
		}
	}

	tenant, err := q.GetTenantByID(ctx, out.TenantID)
	if err != nil {
		return out, err
	}
	if ch.Status == model.StatusDelivered && tenant.AlertOnDelivery {
		to := ""
		if ch.recipient != "" {
			to = " to " + ch.recipient
		}
		s.raise(ctx, q, after, newAlert{
			TenantID: out.TenantID, ShipmentID: out.ID, DriverID: out.DriverID, Type: AlertDelivered, Severity: SeverityInfo,
			Message: fmt.Sprintf("%s was delivered%s in %s.", out.TrackingNumber, to, place(out.DestinationCity, out.DestinationAddress)),
		})
	}
	if text, send := s.customerStatusText(tenant, out, ch.Status, firstDeparture, reason, ch.recipient); send {
		s.notifyCustomer(ctx, q, after, tenant, out, text)
	}

	after.do(func() { s.publishShipment(out, "status") })
	return out, nil
}

// updateStatusDetail handles a report of the status a shipment is already
// in. A repeated delay or problem with a new cause or note is news for the
// office, so it is recorded and alerted; anything else is a replay, answered
// with model.ErrSameStatus.
func (s *ShipmentService) updateStatusDetail(ctx context.Context, q *db.Queries, after *afterCommit, a Actor, cur db.Shipment, ch StatusChange) (db.Shipment, error) {
	var alertType, severity, what string
	switch ch.Status {
	case model.StatusDelayed:
		alertType, severity, what = AlertDelayReported, SeverityWarning, "delay"
	case model.StatusIssue:
		alertType, severity, what = AlertIssueReported, SeverityCritical, "problem"
	default:
		return cur, model.ErrSameStatus
	}
	reason := strings.TrimSpace(ch.Reason)
	if reason == "" && ch.By == model.ActorDriverSMS {
		reason = "other"
	}
	if !contains(model.ReasonsFor(ch.Status), reason) {
		return cur, invalid("reason", "Choose a reason from the list.")
	}
	note := strings.TrimSpace(truncate(ch.Note, 1000))
	if reason == deref(cur.StatusReason) && (note == "" || note == deref(cur.StatusNote)) {
		return cur, model.ErrSameStatus
	}
	occurred := s.now()
	if !ch.OccurredAt.IsZero() && ch.OccurredAt.Before(occurred) {
		occurred = ch.OccurredAt
	}
	// A report older than the current state, such as one queued offline
	// before the office set a cause of its own, is not news.
	if cur.StatusChangedAt.Valid && occurred.Before(cur.StatusChangedAt.Time) {
		return cur, model.ErrSameStatus
	}

	if !s.event(ctx, q, a, cur, shipmentEvent{
		Type: "status_updated", From: cur.Status, To: ch.Status, Reason: reason, Note: note,
		Lat: ch.Lat, Lng: ch.Lng, Public: true, ClientID: ch.ClientID, OccurredAt: occurred, DriverID: cur.DriverID,
	}) {
		return cur, model.ErrSameStatus // this very report, replayed
	}
	out, err := q.SetShipmentStatusDetail(ctx, db.SetShipmentStatusDetailParams{
		StatusReason: toText(reason), StatusNote: toText(note), ID: cur.ID, TenantID: cur.TenantID,
	})
	if err != nil {
		return cur, err
	}
	who := a.Name
	if who == "" {
		who = "The driver"
	}
	// The new alert replaces the open one, which described the old cause.
	_ = q.AcknowledgeShipmentAlerts(ctx, db.AcknowledgeShipmentAlertsParams{TenantID: out.TenantID, ShipmentID: out.ID, AlertTypes: []string{alertType}})
	s.raise(ctx, q, after, newAlert{
		TenantID: out.TenantID, ShipmentID: out.ID, DriverID: out.DriverID, Type: alertType, Severity: severity,
		Message: fmt.Sprintf("%s updated the %s on %s: %s%s", who, what, out.TrackingNumber, model.ReasonLabel(reason), noteSuffix(note)),
	})
	after.do(func() { s.publishShipment(out, "status") })
	return out, nil
}

func noteSuffix(note string) string {
	if note == "" {
		return ""
	}
	return " — " + truncate(note, 140)
}

func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Cancel withdraws a shipment. The customer and, if the cargo is already
// with them, the driver are told.
func (s *ShipmentService) Cancel(ctx context.Context, a Actor, id pgtype.UUID, reason, note string) (db.Shipment, error) {
	reason = strings.TrimSpace(reason)
	if !contains(model.CancelReasons, reason) {
		return db.Shipment{}, invalid("reason", "Choose why the shipment is cancelled.")
	}
	note = strings.TrimSpace(truncate(note, 1000))

	var out db.Shipment
	err := s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		cur, err := s.lock(ctx, q, a.TenantID, id)
		if err != nil {
			return err
		}
		if !model.CanCancel(cur.Status) {
			return conflict(fmt.Sprintf("The shipment is already %s.", strings.ToLower(model.StatusLabel(cur.Status))))
		}
		if !contains(model.CancelReasonsFor(cur.Status), reason) {
			if model.IsOnRoad(cur.Status) {
				return invalid("reason", "That reason is for a shipment that hasn't left yet.")
			}
			return invalid("reason", "That reason is for a shipment already on the road.")
		}
		out, err = q.SetShipmentStatus(ctx, db.SetShipmentStatusParams{
			Status: model.StatusCancelled, StatusNote: toText(note), StatusReason: &reason,
			ChangedAt: toTimestamp(s.now()), ID: id, TenantID: a.TenantID,
		})
		if err != nil {
			return err
		}
		s.event(ctx, q, a, out, shipmentEvent{
			Type: "status_changed", From: cur.Status, To: model.StatusCancelled, Reason: reason, Note: note, Public: true,
			DriverID: cur.DriverID, VisibleToDriver: cur.DriverID.Valid,
		})
		_ = q.AcknowledgeShipmentAlerts(ctx, db.AcknowledgeShipmentAlertsParams{TenantID: a.TenantID, ShipmentID: id, AlertTypes: resolvedByCompletion})

		tenant, err := q.GetTenantByID(ctx, a.TenantID)
		if err != nil {
			return err
		}
		if tenant.SmsOnCancelled {
			s.notifyCustomer(ctx, q, after, tenant, out, s.customerCancelledText(tenant, out))
		}
		if cur.DriverID.Valid && model.IsActiveStatus(cur.Status) {
			if d, err := q.GetUserByID(ctx, db.GetUserByIDParams{ID: cur.DriverID, TenantID: a.TenantID}); err == nil && d.Phone != nil {
				s.enqueueSMS(ctx, q, after, outboundSMS{
					TenantID: a.TenantID, ShipmentID: id, Phone: *d.Phone, Purpose: PurposeDriverMessage,
					Body: fmt.Sprintf("%s: shipment %s (%s) is CANCELLED: %s. Do not deliver it; call the office.",
						tenant.Name, ShortRef(out.TrackingNumber), deref(out.DestinationCity), model.ReasonLabel(reason)),
				})
			}
		}
		after.do(func() { s.publishShipment(out, "cancelled") })
		return nil
	})
	return out, err
}

// Reopen brings back a shipment cancelled by mistake, to pending, or to
// assigned if its driver is still available.
func (s *ShipmentService) Reopen(ctx context.Context, a Actor, id pgtype.UUID) (db.Shipment, error) {
	var out db.Shipment
	err := s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		cur, err := s.lock(ctx, q, a.TenantID, id)
		if err != nil {
			return err
		}
		if !model.CanReopen(cur.Status) {
			return conflict("Only a cancelled shipment can be reopened.")
		}
		if cur.AnonymisedAt.Valid {
			return errShipmentAnonymised
		}
		// Reopening is new work: not in read-only mode, and the driver gets
		// it back only if they could be assigned it now.
		tenant, err := q.GetTenantByID(ctx, a.TenantID)
		if err != nil {
			return err
		}
		if err := s.requireWritable(ctx, q, tenant); err != nil {
			return err
		}
		to := model.StatusPending
		var why string
		if cur.DriverID.Valid {
			if _, err := s.assignableDriver(ctx, q, tenant, cur.DriverID.String(), id); err == nil {
				to = model.StatusAssigned
			} else if de, ok := AsError(err); ok {
				why = de.Message
			} else {
				return err
			}
		}
		if to == model.StatusPending && cur.DriverID.Valid {
			if _, err := q.SetShipmentAssignment(ctx, db.SetShipmentAssignmentParams{ID: id, TenantID: a.TenantID, Status: model.StatusCancelled}); err != nil {
				return err
			}
		}
		out, err = q.SetShipmentStatus(ctx, db.SetShipmentStatusParams{
			Status: to, ChangedAt: toTimestamp(s.now()), ID: id, TenantID: a.TenantID,
		})
		if err != nil {
			return err
		}
		s.event(ctx, q, a, out, shipmentEvent{
			Type: "status_changed", From: model.StatusCancelled, To: to, Note: "Reopened", Public: true,
			DriverID: out.DriverID, VisibleToDriver: out.DriverID.Valid,
		})
		if to == model.StatusPending && cur.DriverID.Valid {
			s.event(ctx, q, a, out, shipmentEvent{
				Type: "unassigned", From: model.StatusCancelled, To: to, PreviousDriverID: cur.DriverID,
				Note: "Reopened without the driver. " + why,
			})
		}
		after.do(func() { s.publishShipment(out, "reopened") })
		return nil
	})
	return out, err
}

// messageBySMSAfter is how long since a driver's last sync before an office
// message to them goes by SMS as well (FR-035 AC-035.1): the app only shows
// messages once it syncs, and a driver out of data would not see it.
const messageBySMSAfter = 15 * time.Minute

// Note adds a comment to a shipment's timeline. A dispatcher's note can be
// addressed to the driver and sent by SMS as well, which happens anyway when
// the driver's app has not synced lately; a driver's note always goes to the
// office. It reports whether the message went by SMS.
func (s *ShipmentService) Note(ctx context.Context, a Actor, id pgtype.UUID, text string, toDriver, bySMS bool, clientID string, at time.Time) (bool, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return false, invalid("note", "Write a message.")
	}
	if len([]rune(text)) > 1000 {
		return false, invalid("note", "Keep it under 1,000 characters.")
	}
	sent := false
	err := s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		sh, err := s.Get(ctx, a.TenantID, id)
		if err != nil {
			return err
		}
		if a.isDriver() {
			if sh.DriverID != a.UserID {
				return ErrNotAssigned
			}
			if s.event(ctx, q, a, sh, shipmentEvent{
				Type: "message", Note: text, DriverID: a.UserID, VisibleToDriver: true, ClientID: clientID, OccurredAt: at,
			}) {
				s.raise(ctx, q, after, newAlert{
					TenantID: sh.TenantID, ShipmentID: sh.ID, DriverID: a.UserID, Type: AlertDriverMessage, Severity: SeverityInfo,
					Message: fmt.Sprintf("%s on %s: %s", a.Name, sh.TrackingNumber, truncate(text, 200)),
				})
				after.do(func() { s.publishShipment(sh, "message") })
			}
			return nil
		}

		if !toDriver {
			s.event(ctx, q, a, sh, shipmentEvent{Type: "note", Note: text})
			return nil
		}
		if !sh.DriverID.Valid {
			return invalid("note", "No driver is assigned to this shipment yet.")
		}
		auto := false
		if !bySMS {
			meta, err := q.GetSyncMetadata(ctx, sh.DriverID)
			switch {
			case isNoRows(err): // never used the app: SMS is all they have
				auto = true
			case err != nil:
				return err
			default:
				auto = !meta.LastSyncAt.Valid || s.now().Sub(meta.LastSyncAt.Time) > messageBySMSAfter
			}
		}
		if bySMS || auto {
			d, err := q.GetUserByID(ctx, db.GetUserByIDParams{ID: sh.DriverID, TenantID: a.TenantID})
			if err == nil && d.Phone != nil {
				tenant, err := q.GetTenantByID(ctx, a.TenantID)
				if err != nil {
					return err
				}
				s.enqueueSMS(ctx, q, after, outboundSMS{
					TenantID: a.TenantID, ShipmentID: sh.ID, Phone: *d.Phone, Purpose: PurposeDriverMessage,
					Body: fmt.Sprintf("%s (%s) re %s: %s", tenant.Name, a.Name, ShortRef(sh.TrackingNumber), truncate(text, 400)),
				})
				sent = true
			}
		}
		var details map[string]any
		if sent {
			details = map[string]any{"sms": true, "auto": auto}
		}
		s.event(ctx, q, a, sh, shipmentEvent{Type: "message", Note: text, DriverID: sh.DriverID, VisibleToDriver: true, Details: details})
		after.do(func() { s.publishShipment(sh, "message") })
		return nil
	})
	return sent, err
}

func formatDistance(m float64) string {
	if m < 1000 {
		return fmt.Sprintf("%.0f m", m)
	}
	return fmt.Sprintf("%.1f km", m/1000)
}
