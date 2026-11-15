package websocket

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMessage(t *testing.T) {
	t.Run("creates valid message with correct type and timestamp", func(t *testing.T) {
		payload := TrackingUpdatePayload{
			DriverID:   "driver-123",
			ShipmentID: "shipment-456",
			Location:   Location{Lat: 40.7128, Lng: -74.0060},
			Status:     "in_transit",
			Timestamp:  time.Now().UTC(),
		}

		before := time.Now().UTC()
		msg, err := NewMessage(TypeTrackingUpdate, payload)
		after := time.Now().UTC()

		require.NoError(t, err)
		require.NotNil(t, msg)

		assert.Equal(t, TypeTrackingUpdate, msg.Type)
		assert.True(t, msg.Timestamp.After(before) || msg.Timestamp.Equal(before))
		assert.True(t, msg.Timestamp.Before(after) || msg.Timestamp.Equal(after))
		assert.NotNil(t, msg.Payload)
	})

	t.Run("returns error for invalid payload", func(t *testing.T) {
		invalidPayload := make(chan int)
		msg, err := NewMessage(TypeTrackingUpdate, invalidPayload)

		require.Error(t, err)
		assert.Nil(t, msg)
	})
}

func TestMessage_ToJSON(t *testing.T) {
	t.Run("serializes correctly", func(t *testing.T) {
		payload := StatusChangePayload{
			ShipmentID: "shipment-123",
			OldStatus:  "pending",
			NewStatus:  "in_transit",
			Timestamp:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		}

		msg, err := NewMessage(TypeStatusChange, payload)
		require.NoError(t, err)

		jsonBytes, err := msg.ToJSON()
		require.NoError(t, err)
		assert.NotNil(t, jsonBytes)

		var result map[string]interface{}
		err = json.Unmarshal(jsonBytes, &result)
		require.NoError(t, err)

		assert.Equal(t, string(TypeStatusChange), result["type"])
		assert.NotNil(t, result["timestamp"])
		assert.NotNil(t, result["payload"])
	})
}

func TestParseMessage(t *testing.T) {
	t.Run("deserializes correctly", func(t *testing.T) {
		original := Message{
			Type:      TypeNewShipment,
			Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			Payload:   json.RawMessage(`{"shipment_id":"s-123"}`),
		}

		jsonBytes, err := original.ToJSON()
		require.NoError(t, err)

		parsed, err := ParseMessage(jsonBytes)
		require.NoError(t, err)
		require.NotNil(t, parsed)

		assert.Equal(t, original.Type, parsed.Type)
		assert.True(t, original.Timestamp.Equal(parsed.Timestamp))
		assert.JSONEq(t, string(original.Payload), string(parsed.Payload))
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		msg, err := ParseMessage([]byte("invalid json"))
		require.Error(t, err)
		assert.Nil(t, msg)
	})
}

func TestTrackingUpdatePayload(t *testing.T) {
	t.Run("serializes and deserializes correctly", func(t *testing.T) {
		now := time.Now().UTC()
		payload := TrackingUpdatePayload{
			DriverID:   "driver-123",
			ShipmentID: "shipment-456",
			Location:   Location{Lat: 40.7128, Lng: -74.0060},
			Status:     "in_transit",
			Timestamp:  now,
		}

		msg, err := NewMessage(TypeTrackingUpdate, payload)
		require.NoError(t, err)

		var parsedPayload TrackingUpdatePayload
		err = json.Unmarshal(msg.Payload, &parsedPayload)
		require.NoError(t, err)

		assert.Equal(t, payload.DriverID, parsedPayload.DriverID)
		assert.Equal(t, payload.ShipmentID, parsedPayload.ShipmentID)
		assert.Equal(t, payload.Location.Lat, parsedPayload.Location.Lat)
		assert.Equal(t, payload.Location.Lng, parsedPayload.Location.Lng)
		assert.Equal(t, payload.Status, parsedPayload.Status)
	})
}

func TestStatusChangePayload(t *testing.T) {
	t.Run("serializes and deserializes correctly", func(t *testing.T) {
		now := time.Now().UTC()
		payload := StatusChangePayload{
			ShipmentID: "shipment-789",
			OldStatus:  "pending",
			NewStatus:  "delivered",
			Timestamp:  now,
		}

		msg, err := NewMessage(TypeStatusChange, payload)
		require.NoError(t, err)

		var parsedPayload StatusChangePayload
		err = json.Unmarshal(msg.Payload, &parsedPayload)
		require.NoError(t, err)

		assert.Equal(t, payload.ShipmentID, parsedPayload.ShipmentID)
		assert.Equal(t, payload.OldStatus, parsedPayload.OldStatus)
		assert.Equal(t, payload.NewStatus, parsedPayload.NewStatus)
	})
}

func TestNewShipmentPayload(t *testing.T) {
	t.Run("serializes and deserializes correctly", func(t *testing.T) {
		now := time.Now().UTC()
		payload := NewShipmentPayload{
			ShipmentID:     "shipment-100",
			TrackingNumber: "TRACK-123456",
			CustomerName:   "John Doe",
			Destination:    "123 Main St, New York, NY",
			Status:         "pending",
			Timestamp:      now,
		}

		msg, err := NewMessage(TypeNewShipment, payload)
		require.NoError(t, err)

		var parsedPayload NewShipmentPayload
		err = json.Unmarshal(msg.Payload, &parsedPayload)
		require.NoError(t, err)

		assert.Equal(t, payload.ShipmentID, parsedPayload.ShipmentID)
		assert.Equal(t, payload.TrackingNumber, parsedPayload.TrackingNumber)
		assert.Equal(t, payload.CustomerName, parsedPayload.CustomerName)
		assert.Equal(t, payload.Destination, parsedPayload.Destination)
		assert.Equal(t, payload.Status, parsedPayload.Status)
	})
}

func TestDeliveryCompletePayload(t *testing.T) {
	t.Run("serializes and deserializes correctly", func(t *testing.T) {
		now := time.Now().UTC()
		payload := DeliveryCompletePayload{
			ShipmentID:     "shipment-200",
			TrackingNumber: "TRACK-789012",
			DriverID:       "driver-456",
			DeliveredAt:    now,
			PODImageURL:    "https://example.com/pod.jpg",
			SignatureURL:   "https://example.com/signature.png",
		}

		msg, err := NewMessage(TypeDeliveryComplete, payload)
		require.NoError(t, err)

		var parsedPayload DeliveryCompletePayload
		err = json.Unmarshal(msg.Payload, &parsedPayload)
		require.NoError(t, err)

		assert.Equal(t, payload.ShipmentID, parsedPayload.ShipmentID)
		assert.Equal(t, payload.TrackingNumber, parsedPayload.TrackingNumber)
		assert.Equal(t, payload.DriverID, parsedPayload.DriverID)
		assert.Equal(t, payload.PODImageURL, parsedPayload.PODImageURL)
		assert.Equal(t, payload.SignatureURL, parsedPayload.SignatureURL)
	})

	t.Run("serializes without optional fields", func(t *testing.T) {
		now := time.Now().UTC()
		payload := DeliveryCompletePayload{
			ShipmentID:     "shipment-200",
			TrackingNumber: "TRACK-789012",
			DriverID:       "driver-456",
			DeliveredAt:    now,
		}

		msg, err := NewMessage(TypeDeliveryComplete, payload)
		require.NoError(t, err)

		var parsedPayload DeliveryCompletePayload
		err = json.Unmarshal(msg.Payload, &parsedPayload)
		require.NoError(t, err)

		assert.Equal(t, payload.ShipmentID, parsedPayload.ShipmentID)
		assert.Empty(t, parsedPayload.PODImageURL)
		assert.Empty(t, parsedPayload.SignatureURL)
	})
}

func TestAlertPayload(t *testing.T) {
	t.Run("serializes and deserializes correctly", func(t *testing.T) {
		now := time.Now().UTC()
		payload := AlertPayload{
			AlertType: "delay",
			Message:   "Shipment delayed due to weather",
			Severity:  "high",
			Timestamp: now,
		}

		msg, err := NewMessage(TypeAlert, payload)
		require.NoError(t, err)

		var parsedPayload AlertPayload
		err = json.Unmarshal(msg.Payload, &parsedPayload)
		require.NoError(t, err)

		assert.Equal(t, payload.AlertType, parsedPayload.AlertType)
		assert.Equal(t, payload.Message, parsedPayload.Message)
		assert.Equal(t, payload.Severity, parsedPayload.Severity)
	})
}

func TestLocation(t *testing.T) {
	t.Run("serializes correctly", func(t *testing.T) {
		loc := Location{
			Lat: 40.7128,
			Lng: -74.0060,
		}

		bytes, err := json.Marshal(loc)
		require.NoError(t, err)

		var parsed Location
		err = json.Unmarshal(bytes, &parsed)
		require.NoError(t, err)

		assert.Equal(t, loc.Lat, parsed.Lat)
		assert.Equal(t, loc.Lng, parsed.Lng)
	})

	t.Run("deserializes from JSON string", func(t *testing.T) {
		jsonStr := `{"lat": 51.5074, "lng": -0.1278}`

		var loc Location
		err := json.Unmarshal([]byte(jsonStr), &loc)
		require.NoError(t, err)

		assert.Equal(t, 51.5074, loc.Lat)
		assert.Equal(t, -0.1278, loc.Lng)
	})
}
