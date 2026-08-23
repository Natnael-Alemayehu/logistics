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

// PublicShipment is the shipment view served to unauthenticated customers by the
// tracking endpoint.
//
// The full Shipment model was returned there, exposing customer_name,
// customer_phone, cargo_description, cargo_value, special_instructions and the
// internal tenant, driver, vehicle and creator IDs to anyone holding a tracking
// number. For a freight platform the cargo fields are the dangerous ones: they
// turn tracking into a catalogue of what is worth stealing and where it is going.
//
// Only what a recipient needs to answer "where is my delivery and when will it
// arrive" is included. Customers already know their own name and what they
// ordered, so echoing those back buys nothing and costs a great deal if the
// tracking number ever leaks.
type PublicShipment struct {
	TrackingNumber     string     `json:"tracking_number"`
	Status             string     `json:"status"`
	StatusNote         string     `json:"status_note,omitempty"`
	OriginAddress      string     `json:"origin_address"`
	DestinationAddress string     `json:"destination_address"`
	EstimatedDelivery  *time.Time `json:"estimated_delivery,omitempty"`
	ActualDelivery     *time.Time `json:"actual_delivery,omitempty"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// ToPublic projects a shipment down to its customer-facing fields.
func (s *Shipment) ToPublic() *PublicShipment {
	return &PublicShipment{
		TrackingNumber:     s.TrackingNumber,
		Status:             s.Status,
		StatusNote:         s.StatusNote,
		OriginAddress:      s.OriginAddress,
		DestinationAddress: s.DestinationAddress,
		EstimatedDelivery:  s.EstimatedDelivery,
		ActualDelivery:     s.ActualDelivery,
		UpdatedAt:          s.UpdatedAt,
	}
}
