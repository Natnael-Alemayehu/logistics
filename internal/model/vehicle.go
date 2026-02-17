package model

import (
	"time"
)

type Vehicle struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	PlateNumber string    `json:"plate_number"`
	VehicleType string    `json:"vehicle_type,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type VehicleSummary struct {
	ID          string `json:"id"`
	PlateNumber string `json:"plate_number"`
	VehicleType string `json:"vehicle_type,omitempty"`
}
