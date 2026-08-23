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
	StatusUpdates    []StatusUpdateInput  `json:"status_updates"`
	BatteryLevel     int                  `json:"battery_level"`
	StorageRemaining int                  `json:"storage_remaining_kb"`
}

func (r *SyncRequest) GetStatusUpdates() []StatusUpdateInput {
	if len(r.StatusUpdates) > 0 {
		return r.StatusUpdates
	}
	return r.Statuses
}

type TrackingEventInput struct {
	ClientID   string    `json:"client_id"`
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
	ClientID               string    `json:"client_id"`
	ShipmentID             string    `json:"shipment_id"`
	RecipientName          string    `json:"recipient_name"`
	RecipientPhone         string    `json:"recipient_phone"`
	SignatureData          string    `json:"signature_data"`
	PhotoURLs              []string  `json:"photo_urls"`
	DeliveryAddress        string    `json:"delivery_address"`
	DeliveryLat            float64   `json:"delivery_lat"`
	DeliveryLng            float64   `json:"delivery_lng"`
	DeliveryNotes          string    `json:"delivery_notes"`
	RecordedAt             time.Time `json:"recorded_at"`
	LocationVerified       bool      `json:"location_verified"`
	LocationMismatchMeters float64   `json:"location_mismatch_meters"`
}

type StatusUpdateInput struct {
	ClientID   string    `json:"client_id"`
	ShipmentID string    `json:"shipment_id"`
	Status     string    `json:"status"`
	Note       string    `json:"note"`
	Reason     string    `json:"reason"`
	RecordedAt time.Time `json:"recorded_at"`
	Timestamp  time.Time `json:"timestamp"`
}

func (s *StatusUpdateInput) GetRecordedAt() time.Time {
	if !s.RecordedAt.IsZero() {
		return s.RecordedAt
	}
	return s.Timestamp
}

type SyncResponse struct {
	ServerTime time.Time `json:"server_time"`
	SyncTime   time.Time `json:"sync_time"`

	// EventsReceived counts accepted items across all three collections. It
	// predates the per-item results below and is retained so that clients which
	// have not yet shipped result handling keep working.
	EventsReceived int `json:"events_received"`

	// Per-item outcomes, positionally aligned with the corresponding request
	// collection. A client must only mark a record synced when its result says
	// accepted or duplicate.
	Events   []SyncItemResult `json:"events"`
	PODs     []SyncItemResult `json:"pods"`
	Statuses []SyncItemResult `json:"statuses"`

	Conflicts          []Conflict   `json:"conflicts,omitempty"`
	DeletedShipmentIDs []string     `json:"deleted_shipment_ids,omitempty"`
	Pull               SyncPullData `json:"pull"`
}

// Outcomes reported for a single submitted item.
const (
	// SyncItemAccepted means the item was persisted by this request.
	SyncItemAccepted = "accepted"
	// SyncItemDuplicate means the item was already persisted by an earlier
	// request. Offline clients retry batches whose response was lost, so this is
	// a success for the client: the data is on the server either way.
	SyncItemDuplicate = "duplicate"
	// SyncItemRejected means the item will never be accepted as submitted and
	// retrying it unchanged is pointless.
	SyncItemRejected = "rejected"
)

// Reasons accompanying a rejected item.
const (
	SyncErrInvalidShipmentID = "invalid_shipment_id"
	SyncErrNotAssigned       = "not_assigned"
	SyncErrStale             = "stale"
	SyncErrInvalidPayload    = "invalid_payload"
	SyncErrStorageFailed     = "storage_failed"
	SyncErrInternal          = "internal_error"
)

// SyncItemResult reports the outcome of one submitted event, POD or status
// update. ClientID echoes the identifier the client sent so results can be
// matched without relying on ordering; Index is the item's position in the
// request collection and is the fallback for clients that send no ClientID.
type SyncItemResult struct {
	ClientID string `json:"client_id,omitempty"`
	Index    int    `json:"index"`
	Status   string `json:"status"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
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
