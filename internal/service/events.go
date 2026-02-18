package service

import (
	"github.com/natnael-alemayehu/logistics/pkg/websocket"
	"github.com/rs/zerolog"
)

type EventService struct {
	hub    *websocket.Hub
	logger zerolog.Logger
}

func NewEventService(hub *websocket.Hub, logger zerolog.Logger) *EventService {
	return &EventService{
		hub:    hub,
		logger: logger,
	}
}

func (s *EventService) PublishTrackingUpdate(tenantID, driverID, shipmentID string, location websocket.Location, status string) {
	payload := websocket.TrackingUpdatePayload{
		DriverID:   driverID,
		ShipmentID: shipmentID,
		Location:   location,
		Status:     status,
	}

	msg, err := websocket.NewMessage(websocket.TypeTrackingUpdate, payload)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to create tracking update message")
		return
	}

	msgBytes, err := msg.ToJSON()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to marshal tracking update message")
		return
	}

	s.hub.BroadcastToTenant(tenantID, msgBytes)
	s.hub.BroadcastToDriver(driverID, msgBytes)

	s.logger.Debug().
		Str("tenant_id", tenantID).
		Str("driver_id", driverID).
		Str("shipment_id", shipmentID).
		Msg("Published tracking update")
}

func (s *EventService) PublishStatusChange(tenantID, shipmentID, oldStatus, newStatus string) {
	payload := websocket.StatusChangePayload{
		ShipmentID: shipmentID,
		OldStatus:  oldStatus,
		NewStatus:  newStatus,
	}

	msg, err := websocket.NewMessage(websocket.TypeStatusChange, payload)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to create status change message")
		return
	}

	msgBytes, err := msg.ToJSON()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to marshal status change message")
		return
	}

	s.hub.BroadcastToTenant(tenantID, msgBytes)

	s.logger.Debug().
		Str("tenant_id", tenantID).
		Str("shipment_id", shipmentID).
		Str("old_status", oldStatus).
		Str("new_status", newStatus).
		Msg("Published status change")
}

func (s *EventService) PublishNewShipment(tenantID, shipmentID, trackingNumber, customerName, destination, status string) {
	payload := websocket.NewShipmentPayload{
		ShipmentID:     shipmentID,
		TrackingNumber: trackingNumber,
		CustomerName:   customerName,
		Destination:    destination,
		Status:         status,
	}

	msg, err := websocket.NewMessage(websocket.TypeNewShipment, payload)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to create new shipment message")
		return
	}

	msgBytes, err := msg.ToJSON()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to marshal new shipment message")
		return
	}

	s.hub.BroadcastToTenant(tenantID, msgBytes)

	s.logger.Debug().
		Str("tenant_id", tenantID).
		Str("shipment_id", shipmentID).
		Msg("Published new shipment")
}

func (s *EventService) PublishDeliveryComplete(tenantID, shipmentID, trackingNumber, driverID, podImageURL, signatureURL string) {
	payload := websocket.DeliveryCompletePayload{
		ShipmentID:     shipmentID,
		TrackingNumber: trackingNumber,
		DriverID:       driverID,
		PODImageURL:    podImageURL,
		SignatureURL:   signatureURL,
	}

	msg, err := websocket.NewMessage(websocket.TypeDeliveryComplete, payload)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to create delivery complete message")
		return
	}

	msgBytes, err := msg.ToJSON()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to marshal delivery complete message")
		return
	}

	s.hub.BroadcastToTenant(tenantID, msgBytes)

	s.logger.Debug().
		Str("tenant_id", tenantID).
		Str("shipment_id", shipmentID).
		Str("driver_id", driverID).
		Msg("Published delivery complete")
}

func (s *EventService) PublishAlert(tenantID, alertType, message, severity string) {
	payload := websocket.AlertPayload{
		AlertType: alertType,
		Message:   message,
		Severity:  severity,
	}

	msg, err := websocket.NewMessage(websocket.TypeAlert, payload)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to create alert message")
		return
	}

	msgBytes, err := msg.ToJSON()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to marshal alert message")
		return
	}

	s.hub.BroadcastToTenant(tenantID, msgBytes)

	s.logger.Debug().
		Str("tenant_id", tenantID).
		Str("alert_type", alertType).
		Str("severity", severity).
		Msg("Published alert")
}

func (s *EventService) Hub() *websocket.Hub {
	return s.hub
}
