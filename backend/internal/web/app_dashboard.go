package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/pkg/websocket"
)

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	if p.IsPlatformAdmin() {
		http.Redirect(w, r, "/app/platform", http.StatusSeeOther)
		return
	}
	ctx := r.Context()
	counts, err := s.svc.Shipments.StatusCounts(ctx, p.TenantID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	now := time.Now().In(s.cfg.Timezone)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.cfg.Timezone)
	deliveredToday, err := s.svc.Shipments.DeliveredSince(ctx, p.TenantID, today)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	alerts, openAlerts, err := s.svc.Alerts.List(ctx, p.TenantID, service.AlertFilter{OpenOnly: true, PerPage: 6})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	activity, err := s.svc.Shipments.Activity(ctx, p.TenantID, 15)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	names := s.driverNames(ctx, p.TenantID)
	items := make([]timelineItem, 0, len(activity))
	for _, a := range activity {
		it := timelineFrom(db.ShipmentEvent{
			ID: a.ID, ShipmentID: a.ShipmentID, EventType: a.EventType, FromStatus: a.FromStatus, ToStatus: a.ToStatus,
			DriverID: a.DriverID, PreviousDriverID: a.PreviousDriverID, ActorName: a.ActorName, Source: a.Source,
			Note: a.Note, Reason: a.Reason, Details: a.Details, OccurredAt: a.OccurredAt, CreatedAt: a.CreatedAt,
		}, names)
		it.TrackingNumber = a.TrackingNumber
		it.ShipmentID = a.ShipmentID.String()
		items = append(items, it)
	}
	mapData, err := s.svc.Tracking.Map(ctx, p.TenantID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	usage, err := s.svc.Tenants.Usage(ctx, p.TenantID)
	if err != nil {
		s.fail(w, r, err)
		return
	}

	active := int64(0)
	for _, st := range model.ActiveStatuses {
		active += counts[st]
	}
	v := s.view(w, r, "Overview", "dashboard", map[string]any{
		"Counts":         counts,
		"Active":         active,
		"Attention":      counts[model.StatusDelayed] + counts[model.StatusIssue],
		"DeliveredToday": deliveredToday,
		"Alerts":         alerts,
		"OpenAlerts":     openAlerts,
		"Activity":       items,
		"Map":            mapData,
		"Usage":          usage,
		"GettingStarted": usage.ActiveDrivers == 0 || usage.Vehicles == 0 || len(counts) == 0,
	})
	s.render(w, r, http.StatusOK, "app/dashboard", v)
}

// driverNames maps driver IDs to names, for rendering history.
func (s *Server) driverNames(ctx context.Context, tenantID pgtype.UUID) map[pgtype.UUID]string {
	out := map[pgtype.UUID]string{}
	drivers, err := s.svc.Users.Drivers(ctx, tenantID, false, "")
	if err != nil {
		return out
	}
	for _, d := range drivers {
		out[d.ID] = d.FullName
	}
	return out
}

func (s *Server) apiMap(w http.ResponseWriter, r *http.Request) {
	m, err := s.svc.Tracking.Map(r.Context(), principal(r).TenantID)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) apiBadge(w http.ResponseWriter, r *http.Request) {
	n, err := s.svc.Alerts.Unread(r.Context(), principal(r).TenantID)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"unread": n})
}

// live upgrades to a WebSocket that streams the tenant's updates.
func (s *Server) live(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // the upgrader has already answered
	}
	c := websocket.NewClient(s.hub, conn, p.TenantID.String(), p.UserID.String(), p.Role)
	c.Rooms[websocket.GetTenantRoomID(p.TenantID.String())] = true
	s.hub.Register <- c
	go c.WritePump()
	go c.ReadPump()
}

// ---------------------------------------------------------------------------
// Timeline
// ---------------------------------------------------------------------------

type timelineItem struct {
	Kind           string
	Title          string
	Detail         string
	Note           string
	Actor          string
	Source         string
	At             time.Time
	Status         string
	TrackingNumber string
	ShipmentID     string
	Lat, Lng       *float64
}

var sourceLabels = map[string]string{
	service.SourceDashboard: "Dashboard",
	service.SourceApp:       "Driver app",
	service.SourceSMS:       "SMS",
	service.SourceSystem:    "System",
	service.SourceCustomer:  "Customer",
	service.SourceAPI:       "API",
}

func timelineFrom(e db.ShipmentEvent, names map[pgtype.UUID]string) timelineItem {
	it := timelineItem{
		Kind: e.EventType, Actor: e.ActorName, Source: sourceLabels[e.Source], At: e.OccurredAt.Time,
		Note: deref(e.Note), Lat: e.Latitude, Lng: e.Longitude,
	}
	name := func(id pgtype.UUID) string {
		if n, ok := names[id]; ok {
			return n
		}
		return "a driver"
	}
	var details map[string]any
	if len(e.Details) > 0 {
		_ = json.Unmarshal(e.Details, &details)
	}
	switch e.EventType {
	case "created":
		it.Title = "Shipment booked"
	case "edited":
		it.Title = "Details edited"
		if f, ok := details["fields"].([]any); ok {
			parts := make([]string, 0, len(f))
			for _, x := range f {
				parts = append(parts, fmt.Sprint(x))
			}
			it.Detail = "Changed " + strings.Join(parts, ", ")
		}
		if it.Note != "" {
			it.Note = "" // the field list says it already
			it.Detail += " · driver notified"
		}
	case "assigned":
		if e.PreviousDriverID.Valid {
			it.Title = fmt.Sprintf("Reassigned from %s to %s", name(e.PreviousDriverID), name(e.DriverID))
		} else {
			it.Title = "Assigned to " + name(e.DriverID)
		}
		if v, ok := details["vehicle"].(string); ok {
			it.Detail = "Vehicle " + v
		}
	case "unassigned":
		it.Title = name(e.PreviousDriverID) + " taken off the shipment"
	case "status_changed":
		to := deref(e.ToStatus)
		it.Status = to
		it.Title = model.StatusLabel(to)
		if e.Reason != nil {
			it.Detail = model.ReasonLabel(*e.Reason)
		}
	case "note":
		it.Title = "Internal note"
	case "message":
		if e.Source == service.SourceApp || e.Source == service.SourceSMS {
			it.Title = "Message from driver"
		} else {
			it.Title = "Message to driver"
		}
	case "pod_recorded":
		it.Title = "Proof of delivery recorded"
		if v, ok := details["location_verified"].(bool); ok {
			if v {
				it.Detail = "Location verified"
			} else if has, _ := details["has_fix"].(bool); has {
				if d, ok := details["distance_m"].(float64); ok {
					it.Detail = fmt.Sprintf("Recorded %s from the destination", formatMeters(d))
				}
			} else {
				it.Detail = "No GPS fix"
			}
		}
	case "customer_confirmed":
		it.Title = "Customer confirmed receipt"
	case "customer_issue":
		it.Title = "Customer reported a problem"
	case "conflict":
		it.Title = "Offline update not applied"
		it.Status = deref(e.ToStatus)
	default:
		it.Title = e.EventType
	}
	return it
}

func formatMeters(m float64) string {
	if m < 1000 {
		return fmt.Sprintf("%.0f m", m)
	}
	return fmt.Sprintf("%.1f km", m/1000)
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
