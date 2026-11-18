package service

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
)

// What the public API sends, and webhooks with it: one shape for a
// shipment and one for an event, so that an integration reads the same
// fields from both. static/openapi.json describes them, and a test in
// internal/web holds the two together.

// PlaceJSON is an origin or a destination.
type PlaceJSON struct {
	City    string   `json:"city"`
	Address string   `json:"address"`
	Lat     *float64 `json:"lat"`
	Lng     *float64 `json:"lng"`
}

// PartyJSON is a customer or a receiver.
type PartyJSON struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// CargoJSON is what is carried.
type CargoJSON struct {
	Description string  `json:"description"`
	WeightKg    float64 `json:"weight_kg"`
	ValueETB    float64 `json:"value_etb"`
	// Pieces is how many pieces the load is, 0 when not counted, and Unit
	// what they are ("bales").
	Pieces int    `json:"pieces"`
	Unit   string `json:"unit"`
}

// ShipmentJSON is a shipment as the API and webhooks send it.
type ShipmentJSON struct {
	ID                string     `json:"id"`
	TrackingNumber    string     `json:"tracking_number"`
	Status            string     `json:"status"`
	StatusReason      string     `json:"status_reason"`
	Priority          string     `json:"priority"`
	Origin            PlaceJSON  `json:"origin"`
	Destination       PlaceJSON  `json:"destination"`
	Customer          PartyJSON  `json:"customer"`
	Receiver          *PartyJSON `json:"receiver"`
	Cargo             CargoJSON  `json:"cargo"`
	Instructions      string     `json:"instructions"`
	DriverID          *string    `json:"driver_id"`
	VehicleID         *string    `json:"vehicle_id"`
	PlannedPickup     *time.Time `json:"planned_pickup"`
	EstimatedDelivery *time.Time `json:"estimated_delivery"`
	PickedUpAt        *time.Time `json:"picked_up_at"`
	DeliveredAt       *time.Time `json:"delivered_at"`
	ReceiptConfirmed  bool       `json:"receipt_confirmed"`
	TrackingURL       string     `json:"tracking_url"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// EventJSON is one entry of a shipment's history.
type EventJSON struct {
	Type       string    `json:"type"`
	FromStatus string    `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	Reason     string    `json:"reason"`
	Note       string    `json:"note"`
	Source     string    `json:"source"`
	Actor      string    `json:"actor"`
	Lat        *float64  `json:"lat"`
	Lng        *float64  `json:"lng"`
	At         time.Time `json:"at"`
}

// AsJSON is a shipment as the API sends it. baseURL is the public origin,
// for the tracking page's address.
func AsJSON(sh db.Shipment, baseURL string) ShipmentJSON {
	out := ShipmentJSON{
		ID: sh.ID.String(), TrackingNumber: sh.TrackingNumber, Status: sh.Status, StatusReason: deref(sh.StatusReason), Priority: sh.Priority,
		Origin:      PlaceJSON{City: deref(sh.OriginCity), Address: sh.OriginAddress, Lat: sh.OriginLat, Lng: sh.OriginLng},
		Destination: PlaceJSON{City: deref(sh.DestinationCity), Address: sh.DestinationAddress, Lat: sh.DestinationLat, Lng: sh.DestinationLng},
		Customer:    PartyJSON{Name: sh.CustomerName, Phone: sh.CustomerPhone},
		Cargo: CargoJSON{Description: deref(sh.CargoDescription), WeightKg: numericOr(sh.CargoWeight), ValueETB: numericOr(sh.CargoValue),
			Pieces: int(derefInt(sh.CargoPieces)), Unit: deref(sh.CargoUnit)},
		Instructions: deref(sh.SpecialInstructions), DriverID: uuidJSON(sh.DriverID), VehicleID: uuidJSON(sh.VehicleID),
		PlannedPickup: utcJSON(sh.PlannedPickupAt), EstimatedDelivery: utcJSON(sh.EstimatedDelivery),
		PickedUpAt: utcJSON(sh.PickedUpAt), DeliveredAt: utcJSON(sh.ActualDelivery),
		ReceiptConfirmed: sh.CustomerConfirmedAt.Valid,
		TrackingURL:      baseURL + "/track/" + sh.TrackingNumber,
		CreatedAt:        sh.CreatedAt.Time.UTC(), UpdatedAt: sh.UpdatedAt.Time.UTC(),
	}
	if sh.ConsigneePhone != nil {
		out.Receiver = &PartyJSON{Name: deref(sh.ConsigneeName), Phone: *sh.ConsigneePhone}
	}
	return out
}

func utcJSON(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	u := t.Time.UTC()
	return &u
}

func uuidJSON(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := u.String()
	return &s
}
