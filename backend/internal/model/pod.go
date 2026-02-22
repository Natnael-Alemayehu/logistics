package model

import (
	"time"
)

type POD struct {
	ID                     string     `json:"id"`
	TenantID               string     `json:"tenant_id"`
	ShipmentID             string     `json:"shipment_id"`
	DriverID               string     `json:"driver_id"`
	RecipientName          string     `json:"recipient_name"`
	RecipientPhone         string     `json:"recipient_phone,omitempty"`
	SignatureData          string     `json:"signature_data,omitempty"`
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
