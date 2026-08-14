package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/natnael-alemayehu/logistics/internal/db"
)

type TrackingService struct {
	queries      *db.Queries
	auditService *AuditService
}

func NewTrackingService(queries *db.Queries, auditService *AuditService) *TrackingService {
	return &TrackingService{queries: queries, auditService: auditService}
}

type TrackingEventOutput struct {
	ID             string     `json:"id"`
	ShipmentID     string     `json:"shipment_id"`
	DriverID       string     `json:"driver_id,omitempty"`
	Latitude       float64    `json:"latitude"`
	Longitude      float64    `json:"longitude"`
	AccuracyMeters float64    `json:"accuracy_meters,omitempty"`
	SpeedKph       float64    `json:"speed_kph,omitempty"`
	Heading        float64    `json:"heading,omitempty"`
	EventType      string     `json:"event_type"`
	Status         string     `json:"status,omitempty"`
	Note           string     `json:"note,omitempty"`
	RecordedAt     time.Time  `json:"recorded_at"`
	SyncedAt       *time.Time `json:"synced_at,omitempty"`
	DeviceID       string     `json:"device_id,omitempty"`
	BatteryLevel   int        `json:"battery_level,omitempty"`
}

type DriverLocationOutput struct {
	DriverID     string     `json:"driver_id"`
	Latitude     float64    `json:"latitude"`
	Longitude    float64    `json:"longitude"`
	RecordedAt   time.Time  `json:"recorded_at"`
	SyncedAt     *time.Time `json:"synced_at,omitempty"`
	Accuracy     float64    `json:"accuracy_meters,omitempty"`
	SpeedKph     float64    `json:"speed_kph,omitempty"`
	Heading      float64    `json:"heading,omitempty"`
	EventType    string     `json:"event_type"`
	BatteryLevel int        `json:"battery_level,omitempty"`
}

type PODOutput struct {
	ID                     string     `json:"id"`
	ShipmentID             string     `json:"shipment_id"`
	DriverID               string     `json:"driver_id"`
	RecipientName          string     `json:"recipient_name"`
	RecipientPhone         string     `json:"recipient_phone,omitempty"`
	SignatureURL           string     `json:"signature_url,omitempty"`
	PhotoURLs              []string   `json:"photo_urls,omitempty"`
	DeliveryAddress        string     `json:"delivery_address,omitempty"`
	DeliveryLat            float64    `json:"delivery_lat,omitempty"`
	DeliveryLng            float64    `json:"delivery_lng,omitempty"`
	DeliveryNotes          string     `json:"delivery_notes,omitempty"`
	LocationVerified       bool       `json:"location_verified"`
	LocationMismatchMeters float64    `json:"location_mismatch_meters,omitempty"`
	RecordedAt             time.Time  `json:"recorded_at"`
	SyncedAt               *time.Time `json:"synced_at,omitempty"`
}

func (s *TrackingService) ListTrackingEventsByShipment(ctx context.Context, tenantID, shipmentID string, page, perPage int) ([]TrackingEventOutput, int, error) {
	offset := (page - 1) * perPage

	events, err := s.queries.ListTrackingEventsByShipment(ctx, db.ListTrackingEventsByShipmentParams{
		ShipmentID: toUUID(shipmentID),
		TenantID:   toUUID(tenantID),
		Limit:      int32(perPage),
		Offset:     int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list tracking events: %w", err)
	}

	total, err := s.queries.CountTrackingEventsByShipment(ctx, toUUID(shipmentID))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count tracking events: %w", err)
	}

	result := make([]TrackingEventOutput, len(events))
	for i, e := range events {
		result[i] = dbTrackingEventToOutput(&e)
	}

	return result, int(total), nil
}

func (s *TrackingService) GetDriverLocation(ctx context.Context, tenantID, driverID string) (*DriverLocationOutput, error) {
	events, err := s.queries.ListTrackingEventsByDriver(ctx, db.ListTrackingEventsByDriverParams{
		DriverID: toUUID(driverID),
		TenantID: toUUID(tenantID),
		Limit:    1,
		Offset:   0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get driver location: %w", err)
	}

	if len(events) == 0 {
		return nil, fmt.Errorf("no tracking events found for driver")
	}

	e := events[0]
	output := &DriverLocationOutput{
		DriverID:   e.DriverID.String(),
		Latitude:   toFloat64(e.Latitude),
		Longitude:  toFloat64(e.Longitude),
		RecordedAt: e.RecordedAt.Time,
		EventType:  e.EventType,
	}

	if e.AccuracyMeters.Valid {
		output.Accuracy = toFloat64FromNumeric(e.AccuracyMeters)
	}
	if e.SpeedKph.Valid {
		output.SpeedKph = toFloat64FromNumeric(e.SpeedKph)
	}
	if e.Heading.Valid {
		output.Heading = toFloat64FromNumeric(e.Heading)
	}
	if e.SyncedAt.Valid {
		output.SyncedAt = &e.SyncedAt.Time
	}
	if e.BatteryLevel != nil {
		output.BatteryLevel = int(*e.BatteryLevel)
	}

	return output, nil
}

func (s *TrackingService) GetPODByShipment(ctx context.Context, tenantID, shipmentID string) (*PODOutput, error) {
	pod, err := s.queries.GetPODByShipmentID(ctx, db.GetPODByShipmentIDParams{
		ShipmentID: toUUID(shipmentID),
		TenantID:   toUUID(tenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("POD not found: %w", err)
	}

	output := &PODOutput{
		ID:            pod.ID.String(),
		ShipmentID:    pod.ShipmentID.String(),
		DriverID:      pod.DriverID.String(),
		RecipientName: pod.RecipientName,
		PhotoURLs:     pod.PhotoUrls,
		RecordedAt:    pod.RecordedAt.Time,
	}

	if pod.RecipientPhone != nil {
		output.RecipientPhone = *pod.RecipientPhone
	}
	if pod.SignatureUrl != nil {
		output.SignatureURL = *pod.SignatureUrl
	}
	if pod.DeliveryAddress != nil {
		output.DeliveryAddress = *pod.DeliveryAddress
	}
	if pod.DeliveryLongitude != nil {
		output.DeliveryLng = toFloat64(pod.DeliveryLongitude)
	}
	if pod.DeliveryLatitude != nil {
		output.DeliveryLat = toFloat64(pod.DeliveryLatitude)
	}
	if pod.DeliveryNotes != nil {
		output.DeliveryNotes = *pod.DeliveryNotes
	}
	if pod.LocationVerified != nil {
		output.LocationVerified = *pod.LocationVerified
	}
	if pod.LocationMismatchMeters.Valid {
		output.LocationMismatchMeters = toFloat64FromNumeric(pod.LocationMismatchMeters)
	}
	if pod.SyncedAt.Valid {
		output.SyncedAt = &pod.SyncedAt.Time
	}

	return output, nil
}

func dbTrackingEventToOutput(e *db.ListTrackingEventsByShipmentRow) TrackingEventOutput {
	output := TrackingEventOutput{
		ID:         e.ID.String(),
		ShipmentID: e.ShipmentID.String(),
		DriverID:   e.DriverID.String(),
		Latitude:   toFloat64(e.Latitude),
		Longitude:  toFloat64(e.Longitude),
		EventType:  e.EventType,
		RecordedAt: e.RecordedAt.Time,
	}

	if e.AccuracyMeters.Valid {
		output.AccuracyMeters = toFloat64FromNumeric(e.AccuracyMeters)
	}
	if e.SpeedKph.Valid {
		output.SpeedKph = toFloat64FromNumeric(e.SpeedKph)
	}
	if e.Heading.Valid {
		output.Heading = toFloat64FromNumeric(e.Heading)
	}
	if e.Status != nil {
		output.Status = *e.Status
	}
	if e.Note != nil {
		output.Note = *e.Note
	}
	if e.SyncedAt.Valid {
		output.SyncedAt = &e.SyncedAt.Time
	}
	if e.DeviceID != nil {
		output.DeviceID = *e.DeviceID
	}
	if e.BatteryLevel != nil {
		output.BatteryLevel = int(*e.BatteryLevel)
	}

	return output
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	default:
		return 0
	}
}

func toFloat64FromNumeric(n pgtype.Numeric) float64 {
	var f float64
	n.Scan(&f)
	return f
}
