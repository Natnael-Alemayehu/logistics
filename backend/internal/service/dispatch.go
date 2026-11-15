package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
)

// Dispatch tools on top of single shipments: assigning several at once,
// saved views of the shipment list, and one message to every driver on the
// road.

// maxBulkAssign caps one bulk assignment.
const maxBulkAssign = 50

// AssignOutcome is what became of one shipment in a bulk assignment.
type AssignOutcome struct {
	ID             pgtype.UUID
	TrackingNumber string
	// Err is the reason the shipment was not assigned, written for the
	// dispatcher; nil when it was.
	Err *Error
}

// AssignMany gives several shipments to one driver (FR-007 AC-007.1). Each
// is assigned in its own transaction, so one that can't be, because the
// driver has reached the company's active-load limit for instance, leaves
// the others assigned.
func (s *ShipmentService) AssignMany(ctx context.Context, a Actor, ids []pgtype.UUID, driverID, vehicleID string) ([]AssignOutcome, error) {
	if len(ids) == 0 {
		return nil, invalid("id", "Tick the shipments to assign.")
	}
	if len(ids) > maxBulkAssign {
		return nil, invalid("id", fmt.Sprintf("Assign at most %d shipments at a time.", maxBulkAssign))
	}
	if strings.TrimSpace(driverID) == "" {
		return nil, invalid("driver_id", "Choose a driver.")
	}
	out := make([]AssignOutcome, 0, len(ids))
	for _, id := range ids {
		sh, err := s.Assign(ctx, a, id, driverID, vehicleID)
		o := AssignOutcome{ID: id, TrackingNumber: sh.TrackingNumber}
		if err != nil {
			de, ok := AsError(err)
			if !ok {
				return out, err
			}
			o.Err = de
			if cur, gerr := s.Get(ctx, a.TenantID, id); gerr == nil {
				o.TrackingNumber = cur.TrackingNumber
			}
		}
		out = append(out, o)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Saved views
// ---------------------------------------------------------------------------

// maxSavedFilters caps the views one person keeps.
const maxSavedFilters = 20

// SavedFilters are the caller's saved views of the shipment list.
func (s *ShipmentService) SavedFilters(ctx context.Context, p *Principal) ([]db.SavedFilter, error) {
	return s.q.ListSavedFilters(ctx, db.ListSavedFiltersParams{TenantID: p.TenantID, UserID: p.UserID})
}

// SaveFilter keeps a view of the shipment list under a name (FR-009
// AC-009.6). query is the list's query string, already reduced by the
// caller to the filter's own parameters; saving under a name in use
// replaces that view.
func (s *ShipmentService) SaveFilter(ctx context.Context, p *Principal, name, query string) (db.SavedFilter, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return db.SavedFilter{}, invalid("name", "Name the view, for example “Late to Djibouti”.")
	}
	if len([]rune(name)) > 40 {
		return db.SavedFilter{}, invalid("name", "Keep the name under 40 characters.")
	}
	if query == "" {
		return db.SavedFilter{}, invalid("name", "Choose a filter or a tab first: this view shows every shipment.")
	}
	if len(query) > 1000 {
		return db.SavedFilter{}, invalid("name", "That filter is too long to save.")
	}
	n, err := s.q.CountSavedFilters(ctx, db.CountSavedFiltersParams{TenantID: p.TenantID, UserID: p.UserID})
	if err != nil {
		return db.SavedFilter{}, err
	}
	if n >= maxSavedFilters {
		existing, err := s.SavedFilters(ctx, p)
		if err != nil {
			return db.SavedFilter{}, err
		}
		replaces := false
		for _, f := range existing {
			replaces = replaces || f.Name == name
		}
		if !replaces {
			return db.SavedFilter{}, invalid("name", fmt.Sprintf("You have %d saved views, the most there can be. Remove one first.", maxSavedFilters))
		}
	}
	return s.q.SaveFilter(ctx, db.SaveFilterParams{TenantID: p.TenantID, UserID: p.UserID, Name: name, Query: query})
}

// DeleteFilter removes one of the caller's saved views.
func (s *ShipmentService) DeleteFilter(ctx context.Context, p *Principal, id pgtype.UUID) error {
	n, err := s.q.DeleteSavedFilter(ctx, db.DeleteSavedFilterParams{ID: id, TenantID: p.TenantID, UserID: p.UserID})
	if err != nil {
		return err
	}
	if n == 0 {
		return notFound("That view was already removed.")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Messages to every driver on the road
// ---------------------------------------------------------------------------

// QuickMessages are the messages a dispatcher sends most, one tap away
// (FR-035 AC-035.5).
var QuickMessages = []string{
	"Call the office as soon as you can.",
	"Open the driver app so we can see where you are.",
	"Report your status in the app or by SMS.",
	"The road ahead is closed. Wait for instructions.",
	"Checkpoint delays reported ahead. Drive carefully.",
	"Return to the depot after your delivery.",
}

// DriversOnTheRoad are the drivers a message to everyone on the road
// reaches: those with a shipment assigned or under way.
func (s *ShipmentService) DriversOnTheRoad(ctx context.Context, tenantID pgtype.UUID) ([]db.ListDriversOnTheRoadRow, error) {
	return s.q.ListDriversOnTheRoad(ctx, tenantID)
}

// Broadcasts are the latest messages sent to every driver on the road.
func (s *ShipmentService) Broadcasts(ctx context.Context, tenantID pgtype.UUID) ([]db.ListBroadcastsRow, error) {
	return s.q.ListBroadcasts(ctx, db.ListBroadcastsParams{TenantID: tenantID, Limit: 20})
}

// BroadcastResult says how a message to every driver went out.
type BroadcastResult struct {
	Drivers int
	BySMS   int
}

// Broadcast sends one message to every driver with a shipment assigned or
// under way (FR-035 AC-035.4). The app shows it on the next sync; a driver
// whose app hasn't synced lately, or who never used it, gets it by SMS as
// well, as with a message about one shipment.
func (s *ShipmentService) Broadcast(ctx context.Context, a Actor, text string) (BroadcastResult, error) {
	var res BroadcastResult
	text = strings.TrimSpace(text)
	if text == "" {
		return res, invalid("body", "Write a message.")
	}
	if len([]rune(text)) > 500 {
		return res, invalid("body", "Keep it under 500 characters: it may go by SMS.")
	}
	err := s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		drivers, err := q.ListDriversOnTheRoad(ctx, a.TenantID)
		if err != nil {
			return err
		}
		if len(drivers) == 0 {
			return conflict("No driver has a shipment assigned or on the road right now.")
		}
		tenant, err := q.GetTenantByID(ctx, a.TenantID)
		if err != nil {
			return err
		}
		b, err := q.CreateBroadcast(ctx, db.CreateBroadcastParams{TenantID: a.TenantID, Body: text, SentBy: a.UserID, SenderName: a.Name})
		if err != nil {
			return err
		}
		now := s.now()
		for _, d := range drivers {
			sms := d.Phone != nil && (!d.LastSyncAt.Valid || now.Sub(d.LastSyncAt.Time) > messageBySMSAfter)
			if sms {
				s.enqueueSMS(ctx, q, after, outboundSMS{
					TenantID: a.TenantID, Phone: *d.Phone, Purpose: PurposeDriverMessage,
					Body: fmt.Sprintf("%s (%s) to all drivers: %s", tenant.Name, a.Name, text),
				})
				res.BySMS++
			}
			if err := q.AddBroadcastRecipient(ctx, db.AddBroadcastRecipientParams{BroadcastID: b.ID, DriverID: d.ID, BySms: sms}); err != nil {
				return err
			}
		}
		res.Drivers = len(drivers)
		s.audit(ctx, q, a, "drivers_messaged", "broadcast", b.ID, nil,
			map[string]any{"message": text, "drivers": res.Drivers, "by_sms": res.BySMS})
		return nil
	})
	return res, err
}

// ---------------------------------------------------------------------------
// Route deviation
// ---------------------------------------------------------------------------

// deviationGain is how much further from the destination a truck must get
// over about an hour to count as heading the wrong way: more than GPS
// error, a detour around a closed road, or a stop off the road.
const deviationGain = 20000.0

// CheckRoutes alerts on loads whose truck is heading away from the
// destination (FR-033 "route deviation"), at most once per load every six
// hours.
func (j *Jobs) CheckRoutes(ctx context.Context) error {
	off, err := j.q.ListRouteDeviations(ctx, deviationGain)
	if err != nil {
		return err
	}
	if len(off) == 0 {
		return nil
	}
	return j.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		for _, d := range off {
			dest := "its destination"
			if d.DestinationCity != nil {
				dest = *d.DestinationCity
			}
			j.raise(ctx, q, after, newAlert{
				TenantID: d.TenantID, ShipmentID: d.ID, DriverID: d.DriverID, Type: AlertRouteDeviation, Severity: SeverityWarning,
				Message: fmt.Sprintf("%s is moving away from %s: %s from it now, %s an hour earlier.",
					d.TrackingNumber, dest, formatDistance(d.DistanceNow), formatDistance(d.DistanceBefore)),
				Episode: d.Episode,
			})
		}
		return nil
	})
}

// retainBroadcasts is how long messages to every driver are kept.
const retainBroadcasts = 180 * 24 * time.Hour
