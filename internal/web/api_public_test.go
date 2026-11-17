package web

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// The OpenAPI document describes the public API as it is: every route and
// no other, and every field the API sends or takes under the name it has.
func TestOpenAPIMatchesTheAPI(t *testing.T) {
	raw, err := embedded.ReadFile("static/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths      map[string]map[string]json.RawMessage
		Components struct {
			Schemas map[string]struct {
				Properties map[string]json.RawMessage
				AllOf      []struct {
					Ref        string `json:"$ref"`
					Properties map[string]json.RawMessage
				}
			}
		}
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("openapi.json: %v", err)
	}

	// Routes.
	inCode := map[string]bool{}
	for _, r := range publicAPI {
		inCode[strings.ToLower(r.Method)+" "+r.Path] = true
	}
	inSpec := map[string]bool{}
	for path, ops := range spec.Paths {
		for method := range ops {
			if method != "parameters" {
				inSpec[method+" "+path] = true
			}
		}
	}
	for r := range inCode {
		if !inSpec[r] {
			t.Errorf("%s is served but not in openapi.json", r)
		}
	}
	for r := range inSpec {
		if !inCode[r] {
			t.Errorf("%s is in openapi.json but not served", r)
		}
	}

	// Fields.
	props := func(name string) []string {
		s, ok := spec.Components.Schemas[name]
		if !ok {
			t.Fatalf("openapi.json has no schema %s", name)
		}
		return propsOf(t, s.Properties)
	}
	for schema, v := range map[string]any{
		"Shipment": apiShipment{}, "Place": apiPlace{}, "Party": apiParty{}, "Cargo": apiCargo{},
		"Event": apiEvent{}, "Driver": apiDriver{}, "ShipmentChange": changeFields{},
	} {
		want, got := jsonFields(reflect.TypeOf(v)), props(schema)
		slices.Sort(want)
		slices.Sort(got)
		if !slices.Equal(want, got) {
			t.Errorf("schema %s has %v; the code has %v", schema, got, want)
		}
	}
	// A booking is a change and a driver.
	booking := spec.Components.Schemas["ShipmentBooking"]
	var got []string
	for _, part := range booking.AllOf {
		got = append(got, propsOf(t, part.Properties)...)
		if part.Ref != "" {
			got = append(got, propsOf(t, spec.Components.Schemas[strings.TrimPrefix(part.Ref, "#/components/schemas/")].Properties)...)
		}
	}
	want := jsonFields(reflect.TypeOf(apiShipmentInput{}))
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		t.Errorf("schema ShipmentBooking has %v; the code takes %v", got, want)
	}
}

// changeFields is what a PATCH takes: a booking without the driver, which
// has a route of its own.
type changeFields struct {
	Priority          any `json:"priority"`
	Origin            any `json:"origin"`
	Destination       any `json:"destination"`
	Customer          any `json:"customer"`
	Receiver          any `json:"receiver"`
	Cargo             any `json:"cargo"`
	Instructions      any `json:"instructions"`
	PlannedPickup     any `json:"planned_pickup"`
	EstimatedDelivery any `json:"estimated_delivery"`
}

func TestChangeFieldsAreABookingWithoutTheDriver(t *testing.T) {
	booking := jsonFields(reflect.TypeOf(apiShipmentInput{}))
	booking = slices.DeleteFunc(booking, func(f string) bool { return f == "driver_id" || f == "vehicle_id" })
	change := jsonFields(reflect.TypeOf(changeFields{}))
	slices.Sort(booking)
	slices.Sort(change)
	if !slices.Equal(booking, change) {
		t.Errorf("a change takes %v; a booking without its driver is %v", change, booking)
	}
}

func propsOf(t *testing.T, m map[string]json.RawMessage) []string {
	t.Helper()
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func jsonFields(rt reflect.Type) []string {
	var out []string
	for f := range rt.Fields() {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name != "" && name != "-" {
			out = append(out, name)
		}
	}
	return out
}
