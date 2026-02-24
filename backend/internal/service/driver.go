package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
)

type DriverService struct {
	queries *db.Queries
	db      *pgxpool.Pool
}

func NewDriverService(queries *db.Queries, database *pgxpool.Pool) *DriverService {
	return &DriverService{queries: queries, db: database}
}

func (s *DriverService) GetStats(ctx context.Context, tenantID, driverID string) (*model.DriverStats, error) {
	params := db.GetDriverDeliveriesTodayParams{
		DriverID: toUUID(driverID),
		TenantID: toUUID(tenantID),
	}

	today, err := s.queries.GetDriverDeliveriesToday(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get today's deliveries: %w", err)
	}

	week, err := s.queries.GetDriverDeliveriesThisWeek(ctx, db.GetDriverDeliveriesThisWeekParams{
		DriverID: toUUID(driverID),
		TenantID: toUUID(tenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get week deliveries: %w", err)
	}

	month, err := s.queries.GetDriverDeliveriesThisMonth(ctx, db.GetDriverDeliveriesThisMonthParams{
		DriverID: toUUID(driverID),
		TenantID: toUUID(tenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get month deliveries: %w", err)
	}

	total, err := s.queries.GetDriverTotalDeliveries(ctx, db.GetDriverTotalDeliveriesParams{
		DriverID: toUUID(driverID),
		TenantID: toUUID(tenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get total deliveries: %w", err)
	}

	active, err := s.queries.GetDriverActiveShipmentCount(ctx, db.GetDriverActiveShipmentCountParams{
		DriverID: toUUID(driverID),
		TenantID: toUUID(tenantID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get active shipments: %w", err)
	}

	return &model.DriverStats{
		DeliveriesToday: today,
		DeliveriesWeek:  week,
		DeliveriesMonth: month,
		TotalDeliveries: total,
		ActiveShipments: active,
	}, nil
}

func (s *DriverService) GetVehicleAssignment(ctx context.Context, tenantID, driverID string) (*model.Vehicle, error) {
	vehicle, err := s.queries.GetDriverVehicleAssignment(ctx, db.GetDriverVehicleAssignmentParams{
		DriverID: toUUID(driverID),
		TenantID: toUUID(tenantID),
	})
	if err != nil {
		return nil, nil
	}

	return dbVehicleToModel(&vehicle), nil
}
