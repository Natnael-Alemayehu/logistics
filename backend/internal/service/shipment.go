package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/pkg/validation"
)

type ShipmentService struct {
	queries             *db.Queries
	auditService        *AuditService
	notificationService *NotificationService
	eventService        *EventService
}

func NewShipmentService(queries *db.Queries, auditService *AuditService, notificationService *NotificationService, eventService *EventService) *ShipmentService {
	return &ShipmentService{
		queries:             queries,
		auditService:        auditService,
		notificationService: notificationService,
		eventService:        eventService,
	}
}

type CreateShipmentInput struct {
	OriginAddress       string  `json:"origin_address" validate:"required"`
	DestinationAddress  string  `json:"destination_address" validate:"required"`
	CustomerName        string  `json:"customer_name" validate:"required"`
	CustomerPhone       string  `json:"customer_phone" validate:"required,ethiopian_phone"`
	CargoDescription    string  `json:"cargo_description"`
	CargoWeight         float64 `json:"cargo_weight"`
	CargoValue          float64 `json:"cargo_value"`
	SpecialInstructions string  `json:"special_instructions"`
	DriverID            string  `json:"driver_id"`
	VehicleID           string  `json:"vehicle_id"`
}

type UpdateShipmentInput struct {
	OriginAddress       string     `json:"origin_address"`
	DestinationAddress  string     `json:"destination_address"`
	CustomerName        string     `json:"customer_name"`
	CustomerPhone       string     `json:"customer_phone"`
	CargoDescription    string     `json:"cargo_description"`
	CargoWeight         float64    `json:"cargo_weight"`
	CargoValue          float64    `json:"cargo_value"`
	SpecialInstructions string     `json:"special_instructions"`
	DriverID            string     `json:"driver_id"`
	VehicleID           string     `json:"vehicle_id"`
	EstimatedDelivery   *time.Time `json:"estimated_delivery"`
}

type UpdateStatusInput struct {
	Status       string `json:"status" validate:"required,oneof=pending assigned in_transit delayed arrived delivered issue cancelled"`
	StatusNote   string `json:"status_note"`
	StatusReason string `json:"status_reason"`

	// RecordedAt is when the client observed the change. Offline drivers queue
	// updates for hours, so one that predates the shipment's last modification
	// describes a world that has already moved on and must not overwrite newer
	// state. The zero value means "not supplied" and skips the check, which is
	// what online callers submitting a change right now send.
	RecordedAt time.Time `json:"recorded_at"`
}

var (
	ErrNotAssigned = errors.New("driver not assigned to this shipment")

	// ErrStaleUpdate is returned when a status update is older than the state it
	// would overwrite.
	ErrStaleUpdate = errors.New("status update is older than the current shipment state")
)

type AssignDriverInput struct {
	DriverID string `json:"driver_id" validate:"required,uuid"`
}

type CancelShipmentInput struct {
	Reason string `json:"reason" validate:"required"`
}

type SearchShipmentsInput struct {
	Status      string
	DriverID    string
	DateFrom    *time.Time
	DateTo      *time.Time
	Origin      string
	Destination string
	Query       string
	Page        int
	PerPage     int
}

func (s *ShipmentService) Create(ctx context.Context, tenantID, createdBy string, input CreateShipmentInput, ipAddress, userAgent string) (*model.Shipment, error) {
	trackingNumber := generateTrackingNumber()
	customerPhone := validation.NormalizeEthiopianPhone(input.CustomerPhone)

	// A shipment created with a driver is already assigned. Leaving it 'pending'
	// kept it out of ListActiveShipmentsByDriver and the driver's sync pull, so
	// dispatching at creation time produced work the driver never saw — while
	// assigning the same driver a moment later, via AssignDriverToShipment, set
	// 'assigned' correctly.
	status := "pending"
	if input.DriverID != "" {
		status = "assigned"
	}

	shipment, err := s.queries.CreateShipment(ctx, db.CreateShipmentParams{
		TenantID:            toUUID(tenantID),
		TrackingNumber:      trackingNumber,
		OriginAddress:       input.OriginAddress,
		DestinationAddress:  input.DestinationAddress,
		CustomerName:        input.CustomerName,
		CustomerPhone:       customerPhone,
		CargoDescription:    toText(input.CargoDescription),
		CargoWeight:         toNumericOmitZero(input.CargoWeight),
		CargoValue:          toNumericOmitZero(input.CargoValue),
		SpecialInstructions: toText(input.SpecialInstructions),
		DriverID:            toUUID(input.DriverID),
		VehicleID:           toUUID(input.VehicleID),
		Status:              status,
		CreatedBy:           toUUID(createdBy),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create shipment: %w", err)
	}

	s.auditService.Log(ctx, AuditLogInput{
		TenantID:   tenantID,
		UserID:     createdBy,
		Action:     model.ActionShipmentCreated,
		EntityType: model.EntityShipment,
		EntityID:   shipment.ID.String(),
		NewValue:   shipment,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	})

	if s.notificationService != nil {
		go func() {
			notifCtx := context.Background()
			if err := s.notificationService.SendShipmentCreatedNotification(notifCtx, customerPhone, trackingNumber); err != nil {
				// Log error but don't fail the request
				fmt.Printf("\nNotificaion fired for: %x with Phone: %x\n", input.CustomerName, customerPhone)
			}
		}()
	}

	// Publish WebSocket event for real-time dashboard updates
	if s.eventService != nil {
		go s.eventService.PublishNewShipment(
			tenantID,
			shipment.ID.String(),
			trackingNumber,
			input.CustomerName,
			input.DestinationAddress,
			"pending",
		)
	}

	return dbShipmentToModel(&shipment), nil
}

func (s *ShipmentService) GetByID(ctx context.Context, tenantID, shipmentID string) (*model.Shipment, error) {
	shipment, err := s.queries.GetShipmentByID(ctx, db.GetShipmentByIDParams{
		ID:       toUUID(shipmentID),
		TenantID: toUUID(tenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("shipment not found: %w", err)
	}

	return dbShipmentToModel(&shipment), nil
}

func (s *ShipmentService) GetByTrackingNumber(ctx context.Context, trackingNumber string) (*model.Shipment, error) {
	shipment, err := s.queries.GetShipmentByTrackingNumber(ctx, trackingNumber)
	if err != nil {
		return nil, fmt.Errorf("shipment not found: %w", err)
	}

	return dbShipmentToModel(&shipment), nil
}

func (s *ShipmentService) ListByTenant(ctx context.Context, tenantID string, page, perPage int) ([]model.Shipment, int, error) {
	offset := (page - 1) * perPage

	shipments, err := s.queries.ListShipmentsByTenant(ctx, db.ListShipmentsByTenantParams{
		TenantID: toUUID(tenantID),
		Limit:    int32(perPage),
		Offset:   int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list shipments: %w", err)
	}

	total, err := s.queries.CountShipmentsByTenant(ctx, toUUID(tenantID))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count shipments: %w", err)
	}

	result := make([]model.Shipment, len(shipments))
	for i, sh := range shipments {
		result[i] = *dbShipmentToModel(&sh)
	}

	return result, int(total), nil
}

func (s *ShipmentService) ListActiveByDriver(ctx context.Context, tenantID, driverID string) ([]model.Shipment, error) {
	shipments, err := s.queries.ListActiveShipmentsByDriver(ctx, db.ListActiveShipmentsByDriverParams{
		DriverID: toUUID(driverID),
		TenantID: toUUID(tenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list shipments: %w", err)
	}

	result := make([]model.Shipment, len(shipments))
	for i, sh := range shipments {
		result[i] = *dbShipmentToModel(&sh)
	}

	return result, nil
}

func (s *ShipmentService) Update(ctx context.Context, tenantID, userID, shipmentID string, input UpdateShipmentInput, ipAddress, userAgent string) (*model.Shipment, error) {
	oldShipment, err := s.queries.GetShipmentByID(ctx, db.GetShipmentByIDParams{
		ID:       toUUID(shipmentID),
		TenantID: toUUID(tenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("shipment not found: %w", err)
	}

	var estimatedDelivery pgtype.Timestamptz
	if input.EstimatedDelivery != nil {
		estimatedDelivery = pgtype.Timestamptz{Time: *input.EstimatedDelivery, Valid: true}
	}

	var customerPhone *string
	if input.CustomerPhone != "" {
		phone := validation.NormalizeEthiopianPhone(input.CustomerPhone)
		customerPhone = &phone
	}

	shipment, err := s.queries.UpdateShipment(ctx, db.UpdateShipmentParams{
		ID:                  toUUID(shipmentID),
		TenantID:            toUUID(tenantID),
		OriginAddress:       toText(input.OriginAddress),
		DestinationAddress:  toText(input.DestinationAddress),
		CustomerName:        toText(input.CustomerName),
		CustomerPhone:       customerPhone,
		CargoDescription:    toText(input.CargoDescription),
		CargoWeight:         toNumericOmitZero(input.CargoWeight),
		CargoValue:          toNumericOmitZero(input.CargoValue),
		SpecialInstructions: toText(input.SpecialInstructions),
		DriverID:            toUUID(input.DriverID),
		VehicleID:           toUUID(input.VehicleID),
		EstimatedDelivery:   estimatedDelivery,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update shipment: %w", err)
	}

	s.auditService.Log(ctx, AuditLogInput{
		TenantID:   tenantID,
		UserID:     userID,
		Action:     model.ActionShipmentUpdated,
		EntityType: model.EntityShipment,
		EntityID:   shipment.ID.String(),
		OldValue:   oldShipment,
		NewValue:   shipment,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	})

	return dbShipmentToModel(&shipment), nil
}

func (s *ShipmentService) AssignDriver(ctx context.Context, tenantID, userID, shipmentID, driverID string, ipAddress, userAgent string) (*model.Shipment, error) {
	oldShipment, err := s.queries.GetShipmentByID(ctx, db.GetShipmentByIDParams{
		ID:       toUUID(shipmentID),
		TenantID: toUUID(tenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("shipment not found: %w", err)
	}

	shipment, err := s.queries.AssignDriverToShipment(ctx, db.AssignDriverToShipmentParams{
		DriverID: toUUID(driverID),
		ID:       toUUID(shipmentID),
		TenantID: toUUID(tenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to assign driver: %w", err)
	}

	s.auditService.Log(ctx, AuditLogInput{
		TenantID:   tenantID,
		UserID:     userID,
		Action:     model.ActionDriverAssigned,
		EntityType: model.EntityShipment,
		EntityID:   shipment.ID.String(),
		OldValue:   oldShipment,
		NewValue:   shipment,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	})

	if s.notificationService != nil {
		go func() {
			notifCtx := context.Background()
			if err := s.notificationService.SendDriverAssignmentNotification(notifCtx, "", shipment.TrackingNumber); err != nil {
				// Log error but don't fail the request
				fmt.Printf("\nNotificaion fired : Tracking Number: %x\n", shipment.TrackingNumber)
			}
		}()
	}

	// Publish WebSocket event for driver assignment (status change from current to assigned)
	if s.eventService != nil {
		go s.eventService.PublishStatusChange(
			tenantID,
			shipment.ID.String(),
			oldShipment.Status,
			"assigned",
		)
	}

	return dbShipmentToModel(&shipment), nil
}

func (s *ShipmentService) UpdateStatus(ctx context.Context, tenantID, userID, userRole, shipmentID string, input UpdateStatusInput, ipAddress, userAgent string) (*model.Shipment, error) {
	oldShipment, err := s.queries.GetShipmentByID(ctx, db.GetShipmentByIDParams{
		ID:       toUUID(shipmentID),
		TenantID: toUUID(tenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("shipment not found: %w", err)
	}

	if userRole == "driver" {
		userUUID := toUUID(userID)
		if !oldShipment.DriverID.Valid || !userUUID.Valid {
			return nil, ErrNotAssigned
		}
		if oldShipment.DriverID.Bytes != userUUID.Bytes {
			return nil, ErrNotAssigned
		}
	}

	// Compare against the last status change, not updated_at: the latter moves
	// for any modification at all, so using it would reject a driver reporting a
	// transition they genuinely observed before an unrelated edit. A NULL means
	// the status has never been changed, so there is nothing to be stale against.
	if !input.RecordedAt.IsZero() && oldShipment.StatusChangedAt.Valid &&
		input.RecordedAt.Before(oldShipment.StatusChangedAt.Time) {
		return nil, ErrStaleUpdate
	}

	shipment, err := s.queries.UpdateShipmentStatus(ctx, db.UpdateShipmentStatusParams{
		Status:       input.Status,
		StatusNote:   toText(input.StatusNote),
		StatusReason: toText(input.StatusReason),
		ID:           toUUID(shipmentID),
		TenantID:     toUUID(tenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update status: %w", err)
	}

	s.auditService.Log(ctx, AuditLogInput{
		TenantID:   tenantID,
		UserID:     userID,
		Action:     model.ActionStatusChanged,
		EntityType: model.EntityShipment,
		EntityID:   shipment.ID.String(),
		OldValue:   oldShipment,
		NewValue:   shipment,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	})

	if s.notificationService != nil && oldShipment.Status != input.Status {
		go func() {
			notifCtx := context.Background()
			switch input.Status {
			case "in_transit":
				s.notificationService.SendShipmentDispatchedNotification(notifCtx, shipment.CustomerPhone, shipment.TrackingNumber)
			case "delivered":
				recipientName := input.StatusNote
				if recipientName == "" {
					recipientName = "recipient"
				}
				s.notificationService.SendShipmentDeliveredNotification(notifCtx, shipment.CustomerPhone, shipment.TrackingNumber, recipientName)
			}
		}()
	}

	// Publish WebSocket events for status changes
	if s.eventService != nil && oldShipment.Status != input.Status {
		go func() {
			s.eventService.PublishStatusChange(
				tenantID,
				shipment.ID.String(),
				oldShipment.Status,
				input.Status,
			)

			// If delivered, also publish a delivery complete event
			if input.Status == "delivered" {
				s.eventService.PublishDeliveryComplete(
					tenantID,
					shipment.ID.String(),
					shipment.TrackingNumber,
					shipment.DriverID.String(),
					"", // POD image URL (available via separate query)
					"", // Signature URL (available via separate query)
				)
			}

			// If issue or delayed, publish an alert
			if input.Status == "issue" || input.Status == "delayed" {
				severity := "warning"
				if input.Status == "issue" {
					severity = "error"
				}
				message := fmt.Sprintf("Shipment %s is %s", shipment.TrackingNumber, input.Status)
				if input.StatusNote != "" {
					message += ": " + input.StatusNote
				}
				s.eventService.PublishAlert(tenantID, "shipment_"+input.Status, message, severity)
			}
		}()
	}

	return dbShipmentToModel(&shipment), nil
}

func (s *ShipmentService) Cancel(ctx context.Context, tenantID, userID, shipmentID string, input CancelShipmentInput, ipAddress, userAgent string) (*model.Shipment, error) {
	oldShipment, err := s.queries.GetShipmentByID(ctx, db.GetShipmentByIDParams{
		ID:       toUUID(shipmentID),
		TenantID: toUUID(tenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("shipment not found: %w", err)
	}

	if oldShipment.Status == "cancelled" {
		return nil, fmt.Errorf("shipment already cancelled")
	}
	if oldShipment.Status == "delivered" {
		return nil, fmt.Errorf("cannot cancel delivered shipment")
	}

	shipment, err := s.queries.CancelShipment(ctx, db.CancelShipmentParams{
		ID:           toUUID(shipmentID),
		StatusReason: toText(input.Reason),
		TenantID:     toUUID(tenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to cancel shipment: %w", err)
	}

	s.auditService.Log(ctx, AuditLogInput{
		TenantID:   tenantID,
		UserID:     userID,
		Action:     model.ActionShipmentCancelled,
		EntityType: model.EntityShipment,
		EntityID:   shipment.ID.String(),
		OldValue:   oldShipment,
		NewValue:   shipment,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	})

	// Publish WebSocket event for cancellation
	if s.eventService != nil {
		go s.eventService.PublishStatusChange(
			tenantID,
			shipment.ID.String(),
			oldShipment.Status,
			"cancelled",
		)
	}

	return dbShipmentToModel(&shipment), nil
}

func (s *ShipmentService) Search(ctx context.Context, tenantID string, input SearchShipmentsInput) ([]model.Shipment, int, error) {
	offset := (input.Page - 1) * input.PerPage

	var dateFrom, dateTo pgtype.Timestamptz
	if input.DateFrom != nil {
		dateFrom = pgtype.Timestamptz{Time: *input.DateFrom, Valid: true}
	}
	if input.DateTo != nil {
		dateTo = pgtype.Timestamptz{Time: *input.DateTo, Valid: true}
	}

	// toText maps an empty filter to nil, which the query reads as "no
	// constraint". Passing "" instead would compare against the empty string and
	// match nothing.
	shipments, err := s.queries.AdvancedSearchShipments(ctx, db.AdvancedSearchShipmentsParams{
		TenantID:    toUUID(tenantID),
		Status:      toText(input.Status),
		DriverID:    toUUID(input.DriverID),
		DateFrom:    dateFrom,
		DateTo:      dateTo,
		Origin:      toText(input.Origin),
		Destination: toText(input.Destination),
		Query:       toText(input.Query),
		RowLimit:    int32(input.PerPage),
		RowOffset:   int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search shipments: %w", err)
	}

	result := make([]model.Shipment, len(shipments))
	for i, sh := range shipments {
		result[i] = *dbShipmentToModel(&sh)
	}

	return result, len(result), nil
}

func dbShipmentToModel(sh *db.Shipment) *model.Shipment {
	m := &model.Shipment{
		ID:                 sh.ID.String(),
		TenantID:           sh.TenantID.String(),
		TrackingNumber:     sh.TrackingNumber,
		OriginAddress:      sh.OriginAddress,
		DestinationAddress: sh.DestinationAddress,
		CustomerName:       sh.CustomerName,
		CustomerPhone:      sh.CustomerPhone,
		Status:             sh.Status,
		CreatedAt:          sh.CreatedAt.Time,
		UpdatedAt:          sh.UpdatedAt.Time,
	}

	if sh.CargoDescription != nil {
		m.CargoDescription = *sh.CargoDescription
	}
	if sh.SpecialInstructions != nil {
		m.SpecialInstructions = *sh.SpecialInstructions
	}
	if sh.StatusNote != nil {
		m.StatusNote = *sh.StatusNote
	}
	if sh.StatusReason != nil {
		m.StatusReason = *sh.StatusReason
	}
	if sh.DriverID.Valid {
		m.DriverID = sh.DriverID.String()
	}
	if sh.VehicleID.Valid {
		m.VehicleID = sh.VehicleID.String()
	}
	if sh.CreatedBy.Valid {
		m.CreatedBy = sh.CreatedBy.String()
	}
	if sh.EstimatedDelivery.Valid {
		m.EstimatedDelivery = &sh.EstimatedDelivery.Time
	}
	if sh.ActualDelivery.Valid {
		m.ActualDelivery = &sh.ActualDelivery.Time
	}

	return m
}

func generateTrackingNumber() string {
	return fmt.Sprintf("ET-%s-%04d", time.Now().Format("20060102"), time.Now().Nanosecond()/100000)
}
