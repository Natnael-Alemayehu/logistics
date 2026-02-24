package service

import (
	"context"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/pkg/websocket"
)

type SyncService struct {
	queries      *db.Queries
	shipment     *ShipmentService
	eventService *EventService
}

func NewSyncService(queries *db.Queries, shipment *ShipmentService, eventService *EventService) *SyncService {
	return &SyncService{queries: queries, shipment: shipment, eventService: eventService}
}

func (s *SyncService) Sync(ctx context.Context, tenantID, driverID string, req model.SyncRequest) (*model.SyncResponse, error) {
	now := time.Now()
	eventsReceived := 0

	for _, event := range req.Events {
		_, err := s.queries.CreateTrackingEvent(ctx, db.CreateTrackingEventParams{
			TenantID:       toUUID(tenantID),
			ShipmentID:     toUUID(event.ShipmentID),
			DriverID:       toUUID(driverID),
			StMakepoint:    event.Longitude,
			StMakepoint_2:  event.Latitude,
			AccuracyMeters: toNumeric(event.Accuracy),
			SpeedKph:       toNumeric(event.Speed),
			Heading:        toNumeric(event.Heading),
			EventType:      event.EventType,
			Status:         toText(event.Status),
			Note:           toText(event.Note),
			RecordedAt:     toTimestamp(event.RecordedAt),
			DeviceID:       toText(req.DeviceID),
			BatteryLevel:   toInt32Ptr(req.BatteryLevel),
		})
		if err == nil {
			eventsReceived++

			// Publish real-time tracking update via WebSocket
			if s.eventService != nil {
				s.eventService.PublishTrackingUpdate(
					tenantID,
					driverID,
					event.ShipmentID,
					websocket.Location{Lat: event.Latitude, Lng: event.Longitude},
					event.Status,
				)
			}
		}
	}

	for _, pod := range req.PODs {
		_, err := s.queries.CreatePOD(ctx, db.CreatePODParams{
			TenantID:               toUUID(tenantID),
			ShipmentID:             toUUID(pod.ShipmentID),
			DriverID:               toUUID(driverID),
			RecipientName:          pod.RecipientName,
			RecipientPhone:         toText(pod.RecipientPhone),
			SignatureData:          toText(pod.SignatureData),
			PhotoUrls:              pod.PhotoURLs,
			DeliveryAddress:        toText(pod.DeliveryAddress),
			StMakepoint:            pod.DeliveryLng,
			StMakepoint_2:          pod.DeliveryLat,
			DeliveryNotes:          toText(pod.DeliveryNotes),
			LocationVerified:       toBoolPtr(pod.LocationVerified),
			LocationMismatchMeters: toNumeric(pod.LocationMismatchMeters),
			RecordedAt:             toTimestamp(pod.RecordedAt),
		})
		if err == nil {
			eventsReceived++

			// Publish delivery complete event via WebSocket
			if s.eventService != nil {
				// Look up the shipment to get its tracking number
				shipment, shipErr := s.queries.GetShipmentByID(ctx, db.GetShipmentByIDParams{
					ID:       toUUID(pod.ShipmentID),
					TenantID: toUUID(tenantID),
				})
				trackingNumber := ""
				if shipErr == nil {
					trackingNumber = shipment.TrackingNumber
				}

				s.eventService.PublishDeliveryComplete(
					tenantID,
					pod.ShipmentID,
					trackingNumber,
					driverID,
					"", // Photo URLs available in the POD record
					"", // Signature available in the POD record
				)
			}
		}
	}

	for _, status := range req.GetStatusUpdates() {
		// TODO: Pass status.GetRecordedAt() to UpdateStatus once UpdateStatusInput supports RecordedAt field
		_, err := s.shipment.UpdateStatus(ctx, tenantID, driverID, status.ShipmentID, UpdateStatusInput{
			Status:       status.Status,
			StatusNote:   status.Note,
			StatusReason: status.Reason,
		}, "", "")
		if err == nil {
			eventsReceived++
		}
	}

	shipments, _ := s.queries.GetShipmentsForSync(ctx, db.GetShipmentsForSyncParams{
		DriverID:  toUUID(driverID),
		TenantID:  toUUID(tenantID),
		UpdatedAt: toTimestamp(req.LastSyncAt),
	})

	pullData := model.SyncPullData{}
	for _, sh := range shipments {
		m := dbShipmentToModel(&sh)
		pullData.Shipments = append(pullData.Shipments, *m)
	}

	return &model.SyncResponse{
		ServerTime:         now,
		SyncTime:           now,
		EventsReceived:     eventsReceived,
		DeletedShipmentIDs: []string{},
		Pull:               pullData,
	}, nil
}
