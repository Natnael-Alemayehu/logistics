package model

import (
	"time"
)

type Shipment struct {
	ID                  string     `json:"id"`
	TenantID            string     `json:"tenant_id"`
	TrackingNumber      string     `json:"tracking_number"`
	OriginAddress       string     `json:"origin_address"`
	OriginLat           float64    `json:"origin_lat,omitempty"`
	OriginLng           float64    `json:"origin_lng,omitempty"`
	DestinationAddress  string     `json:"destination_address"`
	DestinationLat      float64    `json:"destination_lat,omitempty"`
	DestinationLng      float64    `json:"destination_lng,omitempty"`
	CustomerName        string     `json:"customer_name"`
	CustomerPhone       string     `json:"customer_phone"`
	CargoDescription    string     `json:"cargo_description,omitempty"`
	CargoWeight         float64    `json:"cargo_weight,omitempty"`
	CargoValue          float64    `json:"cargo_value,omitempty"`
	SpecialInstructions string     `json:"special_instructions,omitempty"`
	DriverID            string     `json:"driver_id,omitempty"`
	VehicleID           string     `json:"vehicle_id,omitempty"`
	Status              string     `json:"status"`
	StatusNote          string     `json:"status_note,omitempty"`
	StatusReason        string     `json:"status_reason,omitempty"`
	EstimatedDelivery   *time.Time `json:"estimated_delivery,omitempty"`
	ActualDelivery      *time.Time `json:"actual_delivery,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	CreatedBy           string     `json:"created_by,omitempty"`
}

type ShipmentSummary struct {
	ID             string    `json:"id"`
	TrackingNumber string    `json:"tracking_number"`
	CustomerName   string    `json:"customer_name"`
	Status         string    `json:"status"`
	Destination    string    `json:"destination_address"`
	CreatedAt      time.Time `json:"created_at"`
}
