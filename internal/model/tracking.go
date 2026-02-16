package model

import (
	"time"
)

type TrackingEvent struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
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
