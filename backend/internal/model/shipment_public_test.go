package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ToPublic feeds the unauthenticated tracking endpoint, so this test asserts on
// the marshalled JSON rather than the struct: what matters is what actually
// reaches the wire. Adding a sensitive field to Shipment and wiring it into
// PublicShipment should fail here.
func TestShipmentToPublicOmitsSensitiveFields(t *testing.T) {
	shipment := &Shipment{
		ID:                  "8f2b1c94-0000-4000-8000-000000000001",
		TenantID:            "8f2b1c94-0000-4000-8000-000000000002",
		TrackingNumber:      "ET-20260823-K7M2P9QXVW",
		OriginAddress:       "Addis Ababa",
		OriginLat:           9.0320,
		OriginLng:           38.7469,
		DestinationAddress:  "Bahir Dar",
		DestinationLat:      11.5744,
		DestinationLng:      37.3617,
		CustomerName:        "Abebe Bekele",
		CustomerPhone:       "0911234567",
		CargoDescription:    "40 laptops",
		CargoWeight:         120.5,
		CargoValue:          1500000,
		SpecialInstructions: "Call the warehouse manager on arrival",
		DriverID:            "8f2b1c94-0000-4000-8000-000000000003",
		VehicleID:           "8f2b1c94-0000-4000-8000-000000000004",
		Status:              "in_transit",
		StatusNote:          "Departed Addis Ababa",
		CreatedBy:           "8f2b1c94-0000-4000-8000-000000000005",
		UpdatedAt:           time.Now(),
	}

	encoded, err := json.Marshal(shipment.ToPublic())
	if err != nil {
		t.Fatalf("failed to marshal public shipment: %v", err)
	}
	payload := string(encoded)

	// Keys that must never appear. The cargo fields matter most: on a freight
	// platform they turn tracking into a catalogue of what is worth stealing.
	forbiddenKeys := []string{
		"customer_name", "customer_phone",
		"cargo_description", "cargo_weight", "cargo_value",
		"special_instructions",
		"tenant_id", "driver_id", "vehicle_id", "created_by", "id",
		"origin_lat", "origin_lng", "destination_lat", "destination_lng",
	}
	for _, key := range forbiddenKeys {
		if strings.Contains(payload, `"`+key+`"`) {
			t.Errorf("public tracking payload exposes %q: %s", key, payload)
		}
	}

	// Values must not leak under a different key either.
	forbiddenValues := []string{"Abebe Bekele", "0911234567", "40 laptops", "1500000"}
	for _, value := range forbiddenValues {
		if strings.Contains(payload, value) {
			t.Errorf("public tracking payload leaks %q: %s", value, payload)
		}
	}

	// What a recipient legitimately needs must survive the projection.
	for _, key := range []string{"tracking_number", "status", "destination_address"} {
		if !strings.Contains(payload, `"`+key+`"`) {
			t.Errorf("public tracking payload is missing %q: %s", key, payload)
		}
	}
}
