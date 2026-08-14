package websocket

import (
	"encoding/json"
	"time"
)

type MessageType string

const (
	TypeTrackingUpdate   MessageType = "tracking_update"
	TypeStatusChange     MessageType = "status_change"
	TypeNewShipment      MessageType = "new_shipment"
	TypeDeliveryComplete MessageType = "delivery_complete"
	TypeAlert            MessageType = "alert"
)

type Message struct {
	Type      MessageType     `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type TrackingUpdatePayload struct {
	DriverID   string    `json:"driver_id"`
	ShipmentID string    `json:"shipment_id"`
	Location   Location  `json:"location"`
	Status     string    `json:"status"`
	Timestamp  time.Time `json:"timestamp"`
}

type StatusChangePayload struct {
	ShipmentID string    `json:"shipment_id"`
	OldStatus  string    `json:"old_status"`
	NewStatus  string    `json:"new_status"`
	Timestamp  time.Time `json:"timestamp"`
}

type NewShipmentPayload struct {
	ShipmentID     string    `json:"shipment_id"`
	TrackingNumber string    `json:"tracking_number"`
	CustomerName   string    `json:"customer_name"`
	Destination    string    `json:"destination"`
	Status         string    `json:"status"`
	Timestamp      time.Time `json:"timestamp"`
}

type DeliveryCompletePayload struct {
	ShipmentID     string    `json:"shipment_id"`
	TrackingNumber string    `json:"tracking_number"`
	DriverID       string    `json:"driver_id"`
	DeliveredAt    time.Time `json:"delivered_at"`
	PODImageURL    string    `json:"pod_image_url,omitempty"`
	SignatureURL   string    `json:"signature_url,omitempty"`
}

type AlertPayload struct {
	AlertType string    `json:"alert_type"`
	Message   string    `json:"message"`
	Severity  string    `json:"severity"`
	Timestamp time.Time `json:"timestamp"`
}

type Location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

func NewMessage(msgType MessageType, payload interface{}) (*Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Message{
		Type:      msgType,
		Timestamp: time.Now().UTC(),
		Payload:   payloadBytes,
	}, nil
}

func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

func ParseMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
