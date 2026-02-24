package model

import (
	"time"
)

type DeviceToken struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	DeviceID   string    `json:"device_id"`
	PushToken  string    `json:"push_token"`
	Platform   string    `json:"platform"`
	AppVersion string    `json:"app_version,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type DriverStats struct {
	DeliveriesToday int64 `json:"deliveries_today"`
	DeliveriesWeek  int64 `json:"deliveries_week"`
	DeliveriesMonth int64 `json:"deliveries_month"`
	TotalDeliveries int64 `json:"total_deliveries"`
	ActiveShipments int64 `json:"active_shipments"`
}
