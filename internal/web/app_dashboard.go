package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/geo"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/service"
	"github.com/natnael-alemayehu/kiflet/pkg/websocket"
)

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
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
	alerts, openAlerts, err := s.svc.Alerts.List(ctx, p.TenantID, service.AlertFilter{OpenOnly: true, PerPage: 4})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	activity, err := s.svc.Shipments.Activity(ctx, p.TenantID, 20)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	drivers, err := s.svc.Users.Drivers(ctx, p.TenantID, false, "")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	names := make(map[pgtype.UUID]string, len(drivers))
	var onDuty, activeDrivers int
	for _, d := range drivers {
		names[d.ID] = d.FullName
		if d.IsActive {
			activeDrivers++
		}
		if d.OnDuty {
			onDuty++
		}
	}
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
	tenant, err := s.svc.Tenants.Get(ctx, p.TenantID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	board := dashboardBoards[0]
	for _, b := range dashboardBoards {
		if b.Key == r.URL.Query().Get("board") {
			board = b
		}
	}
	rows, boardTotal, err := s.svc.Shipments.List(ctx, p.TenantID, service.ShipmentFilter{Statuses: board.Statuses, Sort: board.Sort, PerPage: 8})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	onRoad := make([]boardRow, 0, len(rows))
	progress := progressFrom(mapData, rows)
	for _, row := range rows {
		onRoad = append(onRoad, boardRow{ListShipmentsRow: row, Progress: progress[row.Shipment.ID.String()]})
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
		"Board":          board,
		"Boards":         dashboardBoards,
		"OnRoad":         onRoad,
		"BoardTotal":     boardTotal,
		"BoardCounts":    boardCounts(counts),
		"Map":            mapData,
		"MapMeta":        fleetMapMeta(tenant.RegionCity),
		"Drivers":        drivers,
		"OnDuty":         onDuty,
		"ActiveDrivers":  activeDrivers,
		"Usage":          usage,
		"GettingStarted": usage.ActiveDrivers == 0 || usage.Vehicles == 0 || len(counts) == 0,
	})
	s.render(w, r, http.StatusOK, "app/dashboard", v)
}

// boardRow is a shipment in the overview's table, with how far along it is.
type boardRow struct {
	db.ListShipmentsRow
	Progress *int
}

// dashboardBoard is a tab of the overview's table of shipments.
type dashboardBoard struct {
	Key, Label string
	Statuses   []string
	Sort       string
	// Tab is the same list on the Shipments page.
	Tab string
}

var dashboardBoards = []dashboardBoard{
	{"active", "On the road", []string{model.StatusInTransit, model.StatusDelayed, model.StatusIssue, model.StatusArrived}, "eta_asc", "active"},
	{"attention", "Attention", []string{model.StatusDelayed, model.StatusIssue}, "eta_asc", "attention"},
	{"waiting", "Waiting", []string{model.StatusPending, model.StatusAssigned}, "created_asc", "pending"},
	{"delivered", "Delivered", []string{model.StatusDelivered}, "updated_desc", "delivered"},
}

// progress says how far along its route each of the rows is, by shipment ID.
func (s *Server) progress(ctx context.Context, tenantID pgtype.UUID, rows []db.ListShipmentsRow) (map[string]*int, error) {
	m, err := s.svc.Tracking.Map(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return progressFrom(m, rows), nil
}

func progressFrom(m *service.MapData, rows []db.ListShipmentsRow) map[string]*int {
	positions := make(map[string]*service.DriverPin, len(m.Drivers))
	for i := range m.Drivers {
		positions[m.Drivers[i].ID] = &m.Drivers[i]
	}
	out := make(map[string]*int, len(rows))
	for _, row := range rows {
		var pos *service.DriverPin
		if row.Shipment.DriverID.Valid {
			pos = positions[row.Shipment.DriverID.String()]
		}
		out[row.Shipment.ID.String()] = service.Progress(row.Shipment, pos)
	}
	return out
}

// boardCounts counts the shipments under each tab of the overview's table;
// delivered ones are many and old, so that tab has no count.
func boardCounts(counts map[string]int64) map[string]int64 {
	out := map[string]int64{}
	for _, b := range dashboardBoards {
		if b.Key == "delivered" {
			continue
		}
		for _, st := range b.Statuses {
			out[b.Key] += counts[st]
		}
	}
	return out
}

// mapMeta frames the fleet map: where it opens when there is nothing on it
// (AC-028.8), and the regions its filter offers (AC-028.5).
type mapMeta struct {
	Center  mapCenter   `json:"center"`
	Regions []mapRegion `json:"regions"`
}

type mapCenter struct {
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	Zoom int     `json:"zoom"`
}

type mapRegion struct {
	Name string `json:"name"`
	// Bounds are the south-west and north-east corners.
	Bounds [2][2]float64 `json:"bounds"`
}

// fleetMapMeta centres the map on the company's home town, or on the whole
// country when the town is unknown.
func fleetMapMeta(homeTown string) mapMeta {
	m := mapMeta{Center: mapCenter{Lat: 9.0, Lng: 39.5, Zoom: 6}}
	if p, ok := geo.Lookup(homeTown); ok {
		m.Center = mapCenter{Lat: p.Lat, Lng: p.Lng, Zoom: 8}
	}
	for _, r := range geo.Regions() {
		m.Regions = append(m.Regions, mapRegion{Name: r.Name, Bounds: [2][2]float64{{r.South, r.West}, {r.North, r.East}}})
	}
	return m
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
	n, err := s.svc.Alerts.Unread(r.Context(), principal(r))
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
	s.hub.Register <- c
	go c.WritePump()
	go c.ReadPump()
	go s.watchSession(c, p.SessionID)
}

// liveSessionCheck is how often an open live feed checks that the session
// it was opened with still lets its holder in.
const liveSessionCheck = 30 * time.Second

// watchSession closes a live feed whose session has ended: signed out,
// expired, the person deactivated or their company suspended. Otherwise the
// feed would carry on for as long as the browser kept it open.
func (s *Server) watchSession(c *websocket.Client, session pgtype.UUID) {
	t := time.NewTicker(liveSessionCheck)
	defer t.Stop()
	for {
		select {
		case <-c.Done():
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			ok, err := s.svc.Auth.SessionActive(ctx, session)
			cancel()
			if err == nil && !ok {
				c.Disconnect()
				return
			}
		}
	}
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
	case "status_updated":
		to := deref(e.ToStatus)
		it.Status = to
		it.Title = "Delay updated"
		if to == model.StatusIssue {
			it.Title = "Problem updated"
		}
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
		if sms, _ := details["sms"].(bool); sms {
			it.Detail = "Also sent by SMS"
			if auto, _ := details["auto"].(bool); auto {
				it.Detail += ": the driver's app hadn't synced for 15 minutes"
			}
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
	case "pod_held":
		it.Title = "Proof of delivery held for review"
		if why, ok := details["reason"].(string); ok {
			it.Detail = service.HeldReasonLabel(why)
		}
	case "pod_accepted":
		it.Title = "Held proof of delivery accepted"
		if who, ok := details["driver"].(string); ok {
			it.Detail = "Recorded by " + who
		}
	case "pod_dismissed":
		it.Title = "Held proof of delivery dismissed"
		if who, ok := details["driver"].(string); ok {
			it.Detail = "Recorded by " + who
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
