package service

import (
	"context"
	"fmt"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
)

type ShipmentService struct {
	queries *db.Queries
}

func NewShipmentService(queries *db.Queries) *ShipmentService {
	return &ShipmentService{queries: queries}
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

type UpdateStatusInput struct {
	Status       string `json:"status" validate:"required,oneof=pending assigned in_transit delayed arrived delivered issue cancelled"`
	StatusNote   string `json:"status_note"`
	StatusReason string `json:"status_reason"`
}

type AssignDriverInput struct {
	DriverID string `json:"driver_id" validate:"required,uuid"`
}

func (s *ShipmentService) Create(ctx context.Context, tenantID, createdBy string, input CreateShipmentInput) (*model.Shipment, error) {
	trackingNumber := generateTrackingNumber()

	shipment, err := s.queries.CreateShipment(ctx, db.CreateShipmentParams{
		TenantID:            toUUID(tenantID),
		TrackingNumber:      trackingNumber,
		OriginAddress:       input.OriginAddress,
		DestinationAddress:  input.DestinationAddress,
		CustomerName:        input.CustomerName,
		CustomerPhone:       input.CustomerPhone,
		CargoDescription:    toText(input.CargoDescription),
		CargoWeight:         toNumeric(input.CargoWeight),
		CargoValue:          toNumeric(input.CargoValue),
		SpecialInstructions: toText(input.SpecialInstructions),
		DriverID:            toUUID(input.DriverID),
		VehicleID:           toUUID(input.VehicleID),
		Status:              "pending",
		CreatedBy:           toUUID(createdBy),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create shipment: %w", err)
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

func (s *ShipmentService) AssignDriver(ctx context.Context, tenantID, shipmentID, driverID string) (*model.Shipment, error) {
	shipment, err := s.queries.AssignDriverToShipment(ctx, db.AssignDriverToShipmentParams{
		DriverID: toUUID(driverID),
		ID:       toUUID(shipmentID),
		TenantID: toUUID(tenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to assign driver: %w", err)
	}

	return dbShipmentToModel(&shipment), nil
}

func (s *ShipmentService) UpdateStatus(ctx context.Context, tenantID, shipmentID string, input UpdateStatusInput) (*model.Shipment, error) {
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

	return dbShipmentToModel(&shipment), nil
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
