package model

import (
	"time"
)

type AuditLog struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id,omitempty"`
	UserID     string    `json:"user_id,omitempty"`
	Action     string    `json:"action"`
	EntityType string    `json:"entity_type"`
	EntityID   string    `json:"entity_id,omitempty"`
	OldValue   string    `json:"old_value,omitempty"`
	NewValue   string    `json:"new_value,omitempty"`
	IPAddress  string    `json:"ip_address,omitempty"`
	UserAgent  string    `json:"user_agent,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

const (
	ActionLogin       = "LOGIN"
	ActionLogout      = "LOGOUT"
	ActionLoginFailed = "LOGIN_FAILED"

	ActionShipmentCreated   = "SHIPMENT_CREATED"
	ActionShipmentUpdated   = "SHIPMENT_UPDATED"
	ActionShipmentCancelled = "SHIPMENT_CANCELLED"
	ActionStatusChanged     = "STATUS_CHANGED"
	ActionDriverAssigned    = "DRIVER_ASSIGNED"

	ActionPODCaptured = "POD_CAPTURED"

	ActionUserCreated = "USER_CREATED"
	ActionUserUpdated = "USER_UPDATED"

	ActionVehicleCreated = "VEHICLE_CREATED"
	ActionVehicleUpdated = "VEHICLE_UPDATED"
	ActionVehicleDeleted = "VEHICLE_DELETED"
)

const (
	EntityUser     = "user"
	EntityShipment = "shipment"
	EntityVehicle  = "vehicle"
	EntityPOD      = "proof_of_delivery"
	EntitySession  = "session"
)
