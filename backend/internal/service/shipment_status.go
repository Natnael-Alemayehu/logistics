package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/geo"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/internal/phone"
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
	if a.isDriver() && cur.DriverID != a.UserID {
		return cur, ErrNotAssigned
	}

	terr := model.CheckTransition(cur.Status, ch.Status, ch.By)
	if errors.Is(terr, model.ErrSameStatus) {
		return cur, terr
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
	// evidence than anything recorded since.
	if ch.By != model.ActorPOD && cur.StatusChangedAt.Valid && occurred.Before(cur.StatusChangedAt.Time) {
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
		ChangedAt: toTimestamp(occurred), ID: cur.ID, TenantID: cur.TenantID,
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
	case model.StatusIssue:
		s.raise(ctx, q, after, newAlert{
			TenantID: out.TenantID, ShipmentID: out.ID, DriverID: out.DriverID, Type: AlertIssueReported, Severity: SeverityCritical,
			Message: fmt.Sprintf("%s reported a problem on %s: %s%s", who, out.TrackingNumber, model.ReasonLabel(reason), noteSuffix(note)),
		})
	case model.StatusDelayed:
		s.raise(ctx, q, after, newAlert{
			TenantID: out.TenantID, ShipmentID: out.ID, DriverID: out.DriverID, Type: AlertDelayReported, Severity: SeverityWarning,
			Message: fmt.Sprintf("%s reported a delay on %s: %s%s", who, out.TrackingNumber, model.ReasonLabel(reason), noteSuffix(note)),
		})
	case model.StatusInTransit:
		_ = q.AcknowledgeShipmentAlerts(ctx, db.AcknowledgeShipmentAlertsParams{
			ShipmentID: out.ID, AlertTypes: []string{AlertDelayReported, AlertIssueReported},
		})
	case model.StatusDelivered:
		_ = q.AcknowledgeShipmentAlerts(ctx, db.AcknowledgeShipmentAlertsParams{ShipmentID: out.ID, AlertTypes: resolvedByCompletion})
		if ch.By == model.ActorDriverSMS {
			s.raise(ctx, q, after, newAlert{
				TenantID: out.TenantID, ShipmentID: out.ID, DriverID: out.DriverID, Type: AlertDeliveredNoPOD, Severity: SeverityWarning,
				Message: fmt.Sprintf("%s reported %s delivered by SMS, without a signature or photos. Confirm with the customer.", who, out.TrackingNumber),
			})
		}
		if out.ConfirmToken == nil {
			token := randomToken(18)
			if err := q.SetShipmentConfirmToken(ctx, db.SetShipmentConfirmTokenParams{ID: out.ID, ConfirmToken: &token}); err == nil {
				out.ConfirmToken = &token
			}
		}
	}

	tenant, err := q.GetTenantByID(ctx, out.TenantID)
	if err != nil {
		return out, err
	}
	if text, send := s.customerStatusText(tenant, out, ch.Status, firstDeparture, reason, ch.recipient); send {
		s.notifyCustomer(ctx, q, after, tenant, out, text)
	}

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
		_ = q.AcknowledgeShipmentAlerts(ctx, db.AcknowledgeShipmentAlertsParams{ShipmentID: id, AlertTypes: resolvedByCompletion})

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
		to := model.StatusPending
		if cur.DriverID.Valid {
			if d, err := q.GetUserByID(ctx, db.GetUserByIDParams{ID: cur.DriverID, TenantID: a.TenantID}); err == nil && d.IsActive {
				to = model.StatusAssigned
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
		after.do(func() { s.publishShipment(out, "reopened") })
		return nil
	})
	return out, err
}

// Note adds a comment to a shipment's timeline. A dispatcher's note can be
// addressed to the driver, and optionally sent by SMS as well; a driver's
// note always goes to the office.
func (s *ShipmentService) Note(ctx context.Context, a Actor, id pgtype.UUID, text string, toDriver, bySMS bool, clientID string, at time.Time) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return invalid("note", "Write a message.")
	}
	if len([]rune(text)) > 1000 {
		return invalid("note", "Keep it under 1,000 characters.")
	}
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
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
		s.event(ctx, q, a, sh, shipmentEvent{Type: "message", Note: text, DriverID: sh.DriverID, VisibleToDriver: true})
		if bySMS {
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
			}
		}
		after.do(func() { s.publishShipment(sh, "message") })
		return nil
	})
}

// PODInput is a recorded proof of delivery whose media is already stored.
type PODInput struct {
	RecipientName  string
	RecipientPhone string
	SignatureKey   string
	PhotoKeys      []string
	Notes          string
	Lat, Lng       *float64
	RecordedAt     time.Time
	ClientID       string
}

// RecordPOD stores proof of delivery and completes the shipment (FR-020–023).
// Replays are harmless: a shipment has one POD and a second is ignored.
func (s *ShipmentService) RecordPOD(ctx context.Context, a Actor, id pgtype.UUID, in PODInput) (db.Shipment, error) {
	in.RecipientName = strings.TrimSpace(in.RecipientName)
	if in.RecipientName == "" {
		return db.Shipment{}, invalid("recipient_name", "Enter who received the delivery.")
	}
	if in.SignatureKey == "" {
		return db.Shipment{}, invalid("signature", "The recipient's signature is required.")
	}
	var recipientPhone *string
	if strings.TrimSpace(in.RecipientPhone) != "" {
		p, err := phone.Normalize(in.RecipientPhone)
		if err != nil {
			return db.Shipment{}, invalid("recipient_phone", err.Error())
		}
		recipientPhone = &p
	}
	now := s.now()
	recorded := in.RecordedAt
	if recorded.IsZero() || recorded.After(now) {
		recorded = now
	}

	var out db.Shipment
	var result error
	err := s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		cur, err := s.lock(ctx, q, a.TenantID, id)
		if err != nil {
			return err
		}
		if a.isDriver() && cur.DriverID != a.UserID {
			return ErrNotAssigned
		}
		if _, err := q.GetPODByShipmentID(ctx, db.GetPODByShipmentIDParams{ShipmentID: id, TenantID: a.TenantID}); err == nil {
			out = cur
			return nil
		} else if !isNoRows(err) {
			return err
		}

		verified := false
		var mismatch pgtype.Numeric
		var distance float64
		haveFix := in.Lat != nil && in.Lng != nil && (*in.Lat != 0 || *in.Lng != 0)
		if haveFix && cur.DestinationLat != nil && cur.DestinationLng != nil {
			distance = geo.DistanceMeters(*in.Lat, *in.Lng, *cur.DestinationLat, *cur.DestinationLng)
			verified = distance <= geo.DeliveryTolerance(cur.DestinationPrecise)
			mismatch = toNumeric(float64(int(distance)))
		}

		if _, err := q.CreatePOD(ctx, db.CreatePODParams{
			TenantID:               a.TenantID,
			ShipmentID:             id,
			DriverID:               cur.DriverID,
			RecipientName:          truncate(in.RecipientName, 200),
			RecipientPhone:         recipientPhone,
			SignatureUrl:           &in.SignatureKey,
			PhotoUrls:              in.PhotoKeys,
			DeliveryAddress:        &cur.DestinationAddress,
			Latitude:               in.Lat,
			Longitude:              in.Lng,
			DeliveryNotes:          toText(strings.TrimSpace(truncate(in.Notes, 1000))),
			LocationVerified:       &verified,
			LocationMismatchMeters: mismatch,
			RecordedAt:             toTimestamp(recorded),
		}); err != nil {
			return err
		}
		s.event(ctx, q, a, cur, shipmentEvent{
			Type: "pod_recorded", Lat: in.Lat, Lng: in.Lng, OccurredAt: recorded,
			Note: "Received by " + in.RecipientName,
			Details: map[string]any{
				"recipient": in.RecipientName, "photos": len(in.PhotoKeys),
				"location_verified": verified, "distance_m": int(distance), "has_fix": haveFix,
			},
		})

		switch {
		case haveFix && !verified && cur.DestinationLat != nil:
			s.raise(ctx, q, after, newAlert{
				TenantID: cur.TenantID, ShipmentID: cur.ID, DriverID: cur.DriverID, Type: AlertPODLocation, Severity: SeverityWarning,
				Message: fmt.Sprintf("Delivery of %s was recorded %s from the destination.", cur.TrackingNumber, formatDistance(distance)),
			})
		case !haveFix:
			s.raise(ctx, q, after, newAlert{
				TenantID: cur.TenantID, ShipmentID: cur.ID, DriverID: cur.DriverID, Type: AlertPODLocation, Severity: SeverityInfo,
				Message: fmt.Sprintf("Delivery of %s was recorded without a GPS fix, so its location could not be checked.", cur.TrackingNumber),
			})
		}

		switch cur.Status {
		case model.StatusDelivered:
			out = cur // the office confirmed it first; the POD is still evidence
			return nil
		case model.StatusCancelled:
			s.raise(ctx, q, after, newAlert{
				TenantID: cur.TenantID, ShipmentID: cur.ID, DriverID: cur.DriverID, Type: AlertDeliveryOnCanceled, Severity: SeverityCritical,
				Message: fmt.Sprintf("%s recorded delivery of %s to %s, but the shipment is cancelled.", a.Name, cur.TrackingNumber, in.RecipientName),
			})
			out = cur
			result = conflict("This shipment was cancelled. The delivery record was kept and the office has been alerted.")
			return nil
		}

		out, err = s.applyStatus(ctx, q, after, a, cur, StatusChange{
			Status: model.StatusDelivered, By: model.ActorPOD, OccurredAt: recorded,
			Lat: in.Lat, Lng: in.Lng, recipient: in.RecipientName,
		})
		if errors.Is(err, model.ErrSameStatus) {
			return nil
		}
		return err
	})
	if err != nil {
		return out, err
	}
	return out, result
}

func formatDistance(m float64) string {
	if m < 1000 {
		return fmt.Sprintf("%.0f m", m)
	}
	return fmt.Sprintf("%.1f km", m/1000)
}
