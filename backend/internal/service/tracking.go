package service

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/geo"
)

// TrackingService records driver positions and serves the live map
// (FR-025–028).
type TrackingService struct{ *core }

// Driver connectivity on the map: live within 15 minutes, stale within four
// hours (the offline-alert threshold), offline beyond.
const (
	liveWindow  = 15 * time.Minute
	staleWindow = 4 * time.Hour
)

// DriverPin is a driver's last known position on the dispatcher map.
type DriverPin struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Phone    string     `json:"phone"`
	Lat      float64    `json:"lat"`
	Lng      float64    `json:"lng"`
	SpeedKPH float64    `json:"speed_kph"`
	Battery  *int32     `json:"battery"`
	At       time.Time  `json:"at"`
	LastSync *time.Time `json:"last_sync"`
	Paused   bool       `json:"paused"`
	State    string     `json:"state"`
}

// ShipmentPin is an active shipment's destination on the map.
type ShipmentPin struct {
	ID             string   `json:"id"`
	TrackingNumber string   `json:"tracking_number"`
	Status         string   `json:"status"`
	Priority       string   `json:"priority"`
	Destination    string   `json:"destination"`
	Lat            *float64 `json:"lat"`
	Lng            *float64 `json:"lng"`
	DriverID       string   `json:"driver_id"`
	DriverName     string   `json:"driver_name"`
}

// MapData is everything the live map draws.
type MapData struct {
	Drivers   []DriverPin   `json:"drivers"`
	Shipments []ShipmentPin `json:"shipments"`
}

// ConnectionState classifies how recently a device was heard from.
func ConnectionState(last time.Time, now time.Time) string {
	switch age := now.Sub(last); {
	case last.IsZero():
		return "never"
	case age <= liveWindow:
		return "live"
	case age <= staleWindow:
		return "stale"
	default:
		return "offline"
	}
}

// Map returns driver positions from the last week and active shipments.
func (s *TrackingService) Map(ctx context.Context, tenantID pgtype.UUID) (*MapData, error) {
	locs, err := s.q.GetLatestDriverLocations(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	m := &MapData{Drivers: make([]DriverPin, 0, len(locs)), Shipments: []ShipmentPin{}}
	for _, l := range locs {
		last := l.RecordedAt.Time
		if l.LastSyncAt.Valid && l.LastSyncAt.Time.After(last) {
			last = l.LastSyncAt.Time
		}
		m.Drivers = append(m.Drivers, DriverPin{
			ID: l.DriverID.String(), Name: l.DriverName, Phone: deref(l.DriverPhone),
			Lat: l.Latitude, Lng: l.Longitude, SpeedKPH: numericOr(l.SpeedKph), Battery: l.BatteryLevel,
			At: l.RecordedAt.Time, LastSync: timePtr(l.LastSyncAt), Paused: deref(l.TrackingPaused),
			State: ConnectionState(last, now),
		})
	}
	ships, err := s.q.ListActiveShipmentsForMap(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, sh := range ships {
		m.Shipments = append(m.Shipments, ShipmentPin{
			ID: sh.ID.String(), TrackingNumber: sh.TrackingNumber, Status: sh.Status, Priority: sh.Priority,
			Destination: deref(sh.DestinationCity), Lat: sh.DestinationLat, Lng: sh.DestinationLng,
			DriverID: uuidString(sh.DriverID), DriverName: deref(sh.DriverName),
		})
	}
	return m, nil
}

// Ping is one GPS fix from a driver's device.
type Ping struct {
	ClientID   string
	ShipmentID pgtype.UUID
	Lat, Lng   float64
	Accuracy   *float64
	Speed      *float64
	Heading    *float64
	Battery    *int
	RecordedAt time.Time
}

// record stores a batch of fixes for a driver, discarding ones that cannot be
// real, and pushes the newest to the map. allowed is the set of shipments the
// driver may attribute positions to.
func (s *TrackingService) record(ctx context.Context, a Actor, deviceID string, pings []Ping, allowed map[pgtype.UUID]bool) (stored int, rejected []string) {
	var newest *Ping
	now := s.now()
	for i := range pings {
		p := &pings[i]
		switch {
		case !allowed[p.ShipmentID],
			math.IsNaN(p.Lat) || math.IsNaN(p.Lng) || !geo.InServiceArea(p.Lat, p.Lng),
			p.Accuracy != nil && *p.Accuracy > 5000,
			p.RecordedAt.After(now.Add(time.Minute)),
			p.RecordedAt.Before(now.Add(-30 * 24 * time.Hour)):
			rejected = append(rejected, p.ClientID)
			continue
		}
		var battery *int32
		if p.Battery != nil && *p.Battery >= 0 && *p.Battery <= 100 {
			b := int32(*p.Battery)
			battery = &b
		}
		params := db.CreateTrackingEventParams{
			TenantID: a.TenantID, ShipmentID: p.ShipmentID, DriverID: a.UserID,
			Longitude: p.Lng, Latitude: p.Lat, EventType: "gps_ping", RecordedAt: toTimestamp(p.RecordedAt),
			DeviceID: toText(deviceID), BatteryLevel: battery, ClientID: toText(p.ClientID),
		}
		if p.Accuracy != nil {
			params.AccuracyMeters = toNumeric(*p.Accuracy)
		}
		if p.Speed != nil && *p.Speed >= 0 && *p.Speed < 200 {
			params.SpeedKph = toNumeric(*p.Speed)
		}
		if p.Heading != nil && *p.Heading >= 0 && *p.Heading < 360 {
			params.Heading = toNumeric(*p.Heading)
		}
		if _, err := s.q.CreateTrackingEvent(ctx, params); err != nil && !isNoRows(err) {
			s.log.Error().Err(err).Msg("store gps fix")
			continue
		}
		stored++
		if newest == nil || p.RecordedAt.After(newest.RecordedAt) {
			newest = p
		}
	}
	if newest != nil {
		s.publish(a.TenantID, "location", map[string]any{
			"driver_id": a.UserID.String(), "name": a.Name, "lat": newest.Lat, "lng": newest.Lng,
			"at": newest.RecordedAt, "speed_kph": newest.Speed, "battery": newest.Battery,
		})
	}
	return stored, rejected
}
