package service

import (
	"context"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
)

type SyncService struct {
	queries  *db.Queries
	shipment *ShipmentService
}

func NewSyncService(queries *db.Queries, shipment *ShipmentService) *SyncService {
	return &SyncService{queries: queries, shipment: shipment}
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
		}
	}

	for _, pod := range req.PODs {
		_, err := s.queries.CreatePOD(ctx, db.CreatePODParams{
			TenantID:        toUUID(tenantID),
			ShipmentID:      toUUID(pod.ShipmentID),
			DriverID:        toUUID(driverID),
			RecipientName:   pod.RecipientName,
			RecipientPhone:  toText(pod.RecipientPhone),
			SignatureData:   toText(pod.SignatureData),
			PhotoUrls:       pod.PhotoURLs,
			DeliveryAddress: toText(pod.DeliveryAddress),
			StMakepoint:     pod.DeliveryLng,
			StMakepoint_2:   pod.DeliveryLat,
			DeliveryNotes:   toText(pod.DeliveryNotes),
			RecordedAt:      toTimestamp(pod.RecordedAt),
		})
		if err == nil {
			eventsReceived++
		}
	}

	for _, status := range req.Statuses {
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
		ServerTime:     now,
		EventsReceived: eventsReceived,
		Pull:           pullData,
	}, nil
}
