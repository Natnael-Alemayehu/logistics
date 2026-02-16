package model

import (
	"time"
)

type SyncRequest struct {
	DeviceID         string               `json:"device_id"`
	LastSyncAt       time.Time            `json:"last_sync_at"`
	Events           []TrackingEventInput `json:"events"`
	PODs             []PODInput           `json:"pods"`
	Statuses         []StatusUpdateInput  `json:"statuses"`
	BatteryLevel     int                  `json:"battery_level"`
	StorageRemaining int                  `json:"storage_remaining_kb"`
}

type TrackingEventInput struct {
	ShipmentID string    `json:"shipment_id"`
	Latitude   float64   `json:"latitude"`
	Longitude  float64   `json:"longitude"`
	Accuracy   float64   `json:"accuracy"`
	Speed      float64   `json:"speed"`
	Heading    float64   `json:"heading"`
	EventType  string    `json:"event_type"`
	Status     string    `json:"status,omitempty"`
	Note       string    `json:"note,omitempty"`
	RecordedAt time.Time `json:"recorded_at"`
}

type PODInput struct {
	ShipmentID      string    `json:"shipment_id"`
	RecipientName   string    `json:"recipient_name"`
	RecipientPhone  string    `json:"recipient_phone"`
	SignatureData   string    `json:"signature_data"`
	PhotoURLs       []string  `json:"photo_urls"`
	DeliveryAddress string    `json:"delivery_address"`
	DeliveryLat     float64   `json:"delivery_lat"`
	DeliveryLng     float64   `json:"delivery_lng"`
	DeliveryNotes   string    `json:"delivery_notes"`
	RecordedAt      time.Time `json:"recorded_at"`
}

type StatusUpdateInput struct {
	ShipmentID string    `json:"shipment_id"`
	Status     string    `json:"status"`
	Note       string    `json:"note"`
	Reason     string    `json:"reason"`
	RecordedAt time.Time `json:"recorded_at"`
}

type SyncResponse struct {
	ServerTime     time.Time    `json:"server_time"`
	EventsReceived int          `json:"events_received"`
	Conflicts      []Conflict   `json:"conflicts,omitempty"`
	Pull           SyncPullData `json:"pull"`
}

type Conflict struct {
	Type        string `json:"type"`
	ShipmentID  string `json:"shipment_id"`
	LocalValue  string `json:"local_value"`
	ServerValue string `json:"server_value"`
	Resolution  string `json:"resolution"`
}

type SyncPullData struct {
	Shipments []Shipment `json:"shipments"`
	Messages  []string   `json:"messages"`
	Updates   []string   `json:"updates"`
}
