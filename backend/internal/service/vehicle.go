package service

import (
	"context"
	"fmt"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
)

type VehicleService struct {
	queries *db.Queries
}

func NewVehicleService(queries *db.Queries) *VehicleService {
	return &VehicleService{queries: queries}
}

type CreateVehicleInput struct {
	PlateNumber string `json:"plate_number" validate:"required"`
	VehicleType string `json:"vehicle_type"`
}

type UpdateVehicleInput struct {
	PlateNumber string `json:"plate_number"`
	VehicleType string `json:"vehicle_type"`
	IsActive    *bool  `json:"is_active"`
}

func (s *VehicleService) Create(ctx context.Context, tenantID string, input CreateVehicleInput) (*model.Vehicle, error) {
	isActive := true
	vehicle, err := s.queries.CreateVehicle(ctx, db.CreateVehicleParams{
		TenantID:    toUUID(tenantID),
		PlateNumber: input.PlateNumber,
		VehicleType: toText(input.VehicleType),
		IsActive:    &isActive,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create vehicle: %w", err)
	}

	return dbVehicleToModel(&vehicle), nil
}

func (s *VehicleService) GetByID(ctx context.Context, tenantID, vehicleID string) (*model.Vehicle, error) {
	vehicle, err := s.queries.GetVehicleByID(ctx, db.GetVehicleByIDParams{
		ID:       toUUID(vehicleID),
		TenantID: toUUID(tenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("vehicle not found: %w", err)
	}

	return dbVehicleToModel(&vehicle), nil
}

func (s *VehicleService) ListByTenant(ctx context.Context, tenantID string, page, perPage int) ([]model.Vehicle, int, error) {
	offset := (page - 1) * perPage

	vehicles, err := s.queries.ListVehiclesByTenant(ctx, db.ListVehiclesByTenantParams{
		TenantID: toUUID(tenantID),
		Limit:    int32(perPage),
		Offset:   int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list vehicles: %w", err)
	}

	total, err := s.queries.CountVehiclesByTenant(ctx, toUUID(tenantID))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count vehicles: %w", err)
	}

	result := make([]model.Vehicle, len(vehicles))
	for i, v := range vehicles {
		result[i] = *dbVehicleToModel(&v)
	}

	return result, int(total), nil
}

func (s *VehicleService) ListActive(ctx context.Context, tenantID string) ([]model.Vehicle, error) {
	vehicles, err := s.queries.ListActiveVehiclesByTenant(ctx, toUUID(tenantID))
	if err != nil {
		return nil, fmt.Errorf("failed to list active vehicles: %w", err)
	}

	result := make([]model.Vehicle, len(vehicles))
	for i, v := range vehicles {
		result[i] = *dbVehicleToModel(&v)
	}

	return result, nil
}

func (s *VehicleService) Update(ctx context.Context, tenantID, vehicleID string, input UpdateVehicleInput) (*model.Vehicle, error) {
	vehicle, err := s.queries.UpdateVehicle(ctx, db.UpdateVehicleParams{
		ID:          toUUID(vehicleID),
		TenantID:    toUUID(tenantID),
		PlateNumber: toText(input.PlateNumber),
		VehicleType: toText(input.VehicleType),
		IsActive:    input.IsActive,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update vehicle: %w", err)
	}

	return dbVehicleToModel(&vehicle), nil
}

func (s *VehicleService) Delete(ctx context.Context, tenantID, vehicleID string) error {
	return s.queries.DeleteVehicle(ctx, db.DeleteVehicleParams{
		ID:       toUUID(vehicleID),
		TenantID: toUUID(tenantID),
	})
}

func (s *VehicleService) HardDelete(ctx context.Context, tenantID, vehicleID string) error {
	return s.queries.HardDeleteVehicle(ctx, db.HardDeleteVehicleParams{
		ID:       toUUID(vehicleID),
		TenantID: toUUID(tenantID),
	})
}

func dbVehicleToModel(v *db.Vehicle) *model.Vehicle {
	m := &model.Vehicle{
		ID:          v.ID.String(),
		TenantID:    v.TenantID.String(),
		PlateNumber: v.PlateNumber,
		CreatedAt:   v.CreatedAt.Time,
		UpdatedAt:   v.UpdatedAt.Time,
	}

	if v.VehicleType != nil {
		m.VehicleType = *v.VehicleType
	}
	if v.IsActive != nil {
		m.IsActive = *v.IsActive
	}

	return m
}
