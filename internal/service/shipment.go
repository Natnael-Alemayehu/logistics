package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/geo"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/phone"
)

// ShipmentService owns the shipment lifecycle: creation, edits, assignment,
// status changes, proof of delivery and the customer's view of it.
type ShipmentService struct {
	*core
	media *MediaService
}

// ShipmentInput is the dispatcher's shipment form.
type ShipmentInput struct {
	Priority string

	OriginCity    string
	OriginAddress string
	OriginLat     *float64
	OriginLng     *float64

	DestinationCity    string
	DestinationAddress string
	DestinationLat     *float64
	DestinationLng     *float64

	CustomerName  string
	CustomerPhone string

	CargoDescription    string
	CargoWeightKg       float64
	CargoValueETB       float64
	SpecialInstructions string

	// PlannedPickup is when the load is to be collected, if agreed. The
	// computed estimate counts from it.
	PlannedPickup *time.Time
	// EstimatedDelivery overrides the computed estimate when set.
	EstimatedDelivery *time.Time

	// DriverID and VehicleID assign the shipment on creation. Ignored on edit,
	// which has its own assignment action.
	DriverID  string
	VehicleID string
}

type resolvedPlace struct {
	City    *string
	Address string
	Lat     float64
	Lng     float64
	Precise bool
}

// resolvePlace turns a town name and optional map pin into stored location
// fields. A pin is used as given; a town alone resolves to its centre, which
// is enough for the map, the ETA and a city-level delivery check.
func resolvePlace(field, city, address string, lat, lng *float64) (resolvedPlace, error) {
	city = strings.TrimSpace(city)
	address = strings.TrimSpace(address)

	if lat != nil && lng != nil && (*lat != 0 || *lng != 0) {
		if !geo.InServiceArea(*lat, *lng) {
			return resolvedPlace{}, invalid(field+"_city", "That map point is outside the area we serve.")
		}
		p := resolvedPlace{Address: address, Lat: *lat, Lng: *lng, Precise: true}
		if known, ok := geo.Lookup(city); ok {
			p.City = &known.Name
		} else if near, d := geo.Nearest(*lat, *lng); d < 30000 {
			p.City = &near.Name
		} else if city != "" {
			p.City = &city
		}
		if p.Address == "" {
			p.Address = deref(p.City)
		}
		return p, nil
	}

	if city == "" {
		return resolvedPlace{}, invalid(field+"_city", "Choose a town.")
	}
	known, ok := geo.Lookup(city)
	if !ok {
		return resolvedPlace{}, invalid(field+"_city",
			fmt.Sprintf("We don't have %q in our list of towns. Pick one from the list, or drop a pin on the map.", city))
	}
	if address == "" {
		address = known.Name
	}
	return resolvedPlace{City: &known.Name, Address: address, Lat: known.Lat, Lng: known.Lng}, nil
}

type validShipment struct {
	priority      string
	origin, dest  resolvedPlace
	customerName  string
	customerPhone string
	cargo         *string
	weight, value pgtype.Numeric
	instructions  *string
	pickup        *time.Time
	etaManual     *time.Time
}

// start is when the trip is expected to begin: the planned pickup, unless
// that has passed.
func (v validShipment) start(now time.Time) time.Time {
	if v.pickup != nil && v.pickup.After(now) {
		return *v.pickup
	}
	return now
}

func validateShipment(in ShipmentInput, now time.Time) (validShipment, error) {
	var v validShipment
	v.priority = strings.TrimSpace(in.Priority)
	if v.priority == "" {
		v.priority = model.PriorityNormal
	}
	if !contains(model.Priorities, v.priority) {
		return v, invalid("priority", "Choose a priority.")
	}

	var err error
	if v.origin, err = resolvePlace("origin", in.OriginCity, in.OriginAddress, in.OriginLat, in.OriginLng); err != nil {
		return v, err
	}
	if v.dest, err = resolvePlace("destination", in.DestinationCity, in.DestinationAddress, in.DestinationLat, in.DestinationLng); err != nil {
		return v, err
	}
	if geo.DistanceMeters(v.origin.Lat, v.origin.Lng, v.dest.Lat, v.dest.Lng) < 50 &&
		strings.EqualFold(v.origin.Address, v.dest.Address) {
		return v, invalid("destination_city", "The destination is the same as the origin.")
	}

	v.customerName = strings.TrimSpace(in.CustomerName)
	if v.customerName == "" {
		return v, invalid("customer_name", "Enter the customer's name.")
	}
	if utf8.RuneCountInString(v.customerName) > 200 {
		return v, invalid("customer_name", "That name is too long.")
	}
	if v.customerPhone, err = phone.Normalize(in.CustomerPhone); err != nil {
		return v, invalid("customer_phone", err.Error())
	}

	if in.CargoWeightKg < 0 || math.IsNaN(in.CargoWeightKg) || in.CargoWeightKg > 200000 {
		return v, invalid("cargo_weight", "Enter a weight between 0 and 200,000 kg.")
	}
	if in.CargoValueETB < 0 || math.IsNaN(in.CargoValueETB) || in.CargoValueETB > 1e12 {
		return v, invalid("cargo_value", "Enter a valid value.")
	}
	v.weight = toNumericOmitZero(in.CargoWeightKg)
	v.value = toNumericOmitZero(in.CargoValueETB)
	v.cargo = toText(strings.TrimSpace(truncate(in.CargoDescription, 500)))
	v.instructions = toText(strings.TrimSpace(truncate(in.SpecialInstructions, 1000)))

	if in.PlannedPickup != nil && !in.PlannedPickup.IsZero() {
		if in.PlannedPickup.After(now.AddDate(1, 0, 0)) {
			return v, invalid("planned_pickup", "The pickup is more than a year away.")
		}
		t := *in.PlannedPickup
		v.pickup = &t
	}
	if in.EstimatedDelivery != nil && !in.EstimatedDelivery.IsZero() {
		if in.EstimatedDelivery.Before(now.Add(-24 * time.Hour)) {
			return v, invalid("estimated_delivery", "The estimated delivery is in the past.")
		}
		if v.pickup != nil && in.EstimatedDelivery.Before(*v.pickup) {
			return v, invalid("estimated_delivery", "The estimated delivery is before the pickup.")
		}
		t := *in.EstimatedDelivery
		v.etaManual = &t
	}
	return v, nil
}

func timestampPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return toTimestamp(*t)
}

// estimateArrival predicts delivery for a shipment leaving at start, from the
// tenant's history on the route when there is enough of it.
func (s *ShipmentService) estimateArrival(ctx context.Context, q *db.Queries, tenantID pgtype.UUID,
	originCity, destCity *string, oLat, oLng, dLat, dLng float64, start time.Time) time.Time {
	var history []float64
	if originCity != nil && destCity != nil {
		h, err := q.RouteTransitHours(ctx, db.RouteTransitHoursParams{TenantID: tenantID, OriginCity: originCity, DestinationCity: destCity})
		if err == nil {
			history = h
		}
	}
	return start.Add(geo.EstimateTransit(oLat, oLng, dLat, dLng, history)).Truncate(time.Minute)
}

// Create books a shipment, optionally assigning it straight away.
func (s *ShipmentService) Create(ctx context.Context, a Actor, in ShipmentInput) (db.Shipment, error) {
	now := s.now()
	v, err := validateShipment(in, now)
	if err != nil {
		return db.Shipment{}, err
	}

	var out db.Shipment
	err = s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		tenant, err := q.GetTenantByID(ctx, a.TenantID)
		if err != nil {
			return err
		}
		if err := s.requireWritable(ctx, q, tenant); err != nil {
			return err
		}

		var driver *db.User
		var vehicleID pgtype.UUID
		status := model.StatusPending
		if strings.TrimSpace(in.DriverID) != "" {
			d, err := s.assignableDriver(ctx, q, tenant, in.DriverID, pgtype.UUID{})
			if err != nil {
				return err
			}
			driver = &d
			if vehicleID, err = s.assignableVehicle(ctx, q, a.TenantID, in.VehicleID, d, pgtype.UUID{}, in.CargoWeightKg); err != nil {
				return err
			}
			status = model.StatusAssigned
		}

		eta := v.etaManual
		if eta == nil {
			e := s.estimateArrival(ctx, q, a.TenantID, v.origin.City, v.dest.City, v.origin.Lat, v.origin.Lng, v.dest.Lat, v.dest.Lng, v.start(now))
			eta = &e
		}

		params := db.CreateShipmentParams{
			TenantID:            a.TenantID,
			Priority:            v.priority,
			OriginCity:          v.origin.City,
			OriginAddress:       v.origin.Address,
			OriginLat:           &v.origin.Lat,
			OriginLng:           &v.origin.Lng,
			DestinationCity:     v.dest.City,
			DestinationAddress:  v.dest.Address,
			DestinationLat:      &v.dest.Lat,
			DestinationLng:      &v.dest.Lng,
			DestinationPrecise:  v.dest.Precise,
			CustomerName:        v.customerName,
			CustomerPhone:       v.customerPhone,
			CargoDescription:    v.cargo,
			CargoWeight:         v.weight,
			CargoValue:          v.value,
			SpecialInstructions: v.instructions,
			VehicleID:           vehicleID,
			Status:              status,
			EstimatedDelivery:   toTimestamp(*eta),
			EtaManual:           v.etaManual != nil,
			CreatedBy:           a.UserID,
			PlannedPickupAt:     timestampPtr(v.pickup),
		}
		if driver != nil {
			params.DriverID = driver.ID
		}

		// A collision is a 1-in-2^50 event, but retrying costs nothing.
		for attempt := 0; ; attempt++ {
			params.TrackingNumber = newTrackingNumber(now.In(s.cfg.Location))
			out, err = q.CreateShipment(ctx, params)
			if c, dup := isUniqueViolation(err); dup && strings.Contains(c, "tracking") && attempt < 3 {
				continue
			}
			if err != nil {
				return err
			}
			break
		}

		s.event(ctx, q, a, out, shipmentEvent{Type: "created", To: model.StatusPending, Public: true})
		if driver != nil {
			s.event(ctx, q, a, out, shipmentEvent{Type: "assigned", To: model.StatusAssigned, DriverID: driver.ID, Public: true})
		}

		if tenant.SmsOnCreated {
			s.notifyCustomer(ctx, q, after, tenant, out, s.customerCreatedText(tenant, out))
		}
		if driver != nil && tenant.SmsDriverOnAssign {
			s.notifyDriverAssigned(ctx, q, after, tenant, out, *driver)
		}
		after.do(func() { s.publishShipment(out, "created") })
		return nil
	})
	return out, err
}

// Update edits a shipment's details. Delivered and cancelled shipments are
// records and stay as they were.
func (s *ShipmentService) Update(ctx context.Context, a Actor, id pgtype.UUID, in ShipmentInput) (db.Shipment, error) {
	now := s.now()
	v, err := validateShipment(in, now)
	if err != nil {
		return db.Shipment{}, err
	}

	var out db.Shipment
	err = s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		cur, err := s.lock(ctx, q, a.TenantID, id)
		if err != nil {
			return err
		}
		if model.IsFinalStatus(cur.Status) {
			return conflict(fmt.Sprintf("A %s shipment can't be edited.", strings.ToLower(model.StatusLabel(cur.Status))))
		}

		eta := v.etaManual
		manual := eta != nil
		if !manual {
			start := v.start(now)
			if cur.PickedUpAt.Valid {
				start = cur.PickedUpAt.Time
			}
			routeChanged := deref(cur.OriginCity) != deref(v.origin.City) || deref(cur.DestinationCity) != deref(v.dest.City) ||
				deref(cur.DestinationLat) != v.dest.Lat || deref(cur.DestinationLng) != v.dest.Lng
			pickupChanged := !cur.PickedUpAt.Valid && !sameTime(cur.PlannedPickupAt, timestampPtr(v.pickup))
			if cur.EtaManual || routeChanged || pickupChanged || !cur.EstimatedDelivery.Valid {
				e := s.estimateArrival(ctx, q, a.TenantID, v.origin.City, v.dest.City, v.origin.Lat, v.origin.Lng, v.dest.Lat, v.dest.Lng, start)
				eta = &e
			} else {
				e := cur.EstimatedDelivery.Time
				eta = &e
			}
		}

		out, err = q.UpdateShipmentDetails(ctx, db.UpdateShipmentDetailsParams{
			ID:                  id,
			TenantID:            a.TenantID,
			Priority:            v.priority,
			OriginCity:          v.origin.City,
			OriginAddress:       v.origin.Address,
			OriginLat:           &v.origin.Lat,
			OriginLng:           &v.origin.Lng,
			DestinationCity:     v.dest.City,
			DestinationAddress:  v.dest.Address,
			DestinationLat:      &v.dest.Lat,
			DestinationLng:      &v.dest.Lng,
			DestinationPrecise:  v.dest.Precise,
			CustomerName:        v.customerName,
			CustomerPhone:       v.customerPhone,
			CargoDescription:    v.cargo,
			CargoWeight:         v.weight,
			CargoValue:          v.value,
			SpecialInstructions: v.instructions,
			EstimatedDelivery:   toTimestamp(*eta),
			EtaManual:           manual,
			PlannedPickupAt:     timestampPtr(v.pickup),
		})
		if err != nil {
			return err
		}

		changed := changedFields(cur, out)
		if len(changed) == 0 {
			return nil
		}
		ev := shipmentEvent{Type: "edited", Details: map[string]any{"fields": changed}}
		// The driver needs to know when where they're going or who they're
		// meeting changes under them.
		if cur.DriverID.Valid && driverRelevant(changed) {
			ev.DriverID = cur.DriverID
			ev.VisibleToDriver = true
			ev.Note = "Shipment details updated: " + strings.Join(changed, ", ")
		}
		s.event(ctx, q, a, out, ev)
		// A new drop-off matters enough to text as well (FR-011 AC-011.4):
		// the driver may be on the road with the app closed.
		if cur.DriverID.Valid && contains(changed, "destination") {
			if err := s.textNewDestination(ctx, q, after, out); err != nil {
				return err
			}
		}
		after.do(func() { s.publishShipment(out, "edited") })
		return nil
	})
	return out, err
}

// textNewDestination tells the assigned driver by SMS where a shipment now
// goes.
func (s *ShipmentService) textNewDestination(ctx context.Context, q *db.Queries, after *afterCommit, sh db.Shipment) error {
	d, err := q.GetUserByID(ctx, db.GetUserByIDParams{ID: sh.DriverID, TenantID: sh.TenantID})
	if err != nil || d.Phone == nil {
		return nil
	}
	t, err := q.GetTenantByID(ctx, sh.TenantID)
	if err != nil {
		return err
	}
	s.enqueueSMS(ctx, q, after, outboundSMS{
		TenantID: sh.TenantID, ShipmentID: sh.ID, Phone: *d.Phone, Purpose: PurposeDriverMessage,
		Body: fmt.Sprintf("%s: shipment %s now goes to %s. Customer %s %s.",
			t.Name, ShortRef(sh.TrackingNumber), place(sh.DestinationCity, sh.DestinationAddress), sh.CustomerName, sh.CustomerPhone),
	})
	return nil
}

func changedFields(a, b db.Shipment) []string {
	var out []string
	add := func(cond bool, name string) {
		if cond {
			out = append(out, name)
		}
	}
	add(a.Priority != b.Priority, "priority")
	add(deref(a.OriginCity) != deref(b.OriginCity) || a.OriginAddress != b.OriginAddress ||
		deref(a.OriginLat) != deref(b.OriginLat) || deref(a.OriginLng) != deref(b.OriginLng), "origin")
	add(deref(a.DestinationCity) != deref(b.DestinationCity) || a.DestinationAddress != b.DestinationAddress ||
		deref(a.DestinationLat) != deref(b.DestinationLat) || deref(a.DestinationLng) != deref(b.DestinationLng), "destination")
	add(a.CustomerName != b.CustomerName, "customer name")
	add(a.CustomerPhone != b.CustomerPhone, "customer phone")
	add(deref(a.CargoDescription) != deref(b.CargoDescription) || numericOr(a.CargoWeight) != numericOr(b.CargoWeight), "cargo")
	add(numericOr(a.CargoValue) != numericOr(b.CargoValue), "cargo value")
	add(deref(a.SpecialInstructions) != deref(b.SpecialInstructions), "instructions")
	add(!sameTime(a.PlannedPickupAt, b.PlannedPickupAt), "pickup time")
	add(!sameTime(a.EstimatedDelivery, b.EstimatedDelivery), "ETA")
	return out
}

// sameTime compares two optional instants to the second.
func sameTime(a, b pgtype.Timestamptz) bool {
	return a.Valid == b.Valid && (!a.Valid || a.Time.Unix() == b.Time.Unix())
}

func driverRelevant(fields []string) bool {
	for _, f := range fields {
		switch f {
		case "origin", "destination", "customer name", "customer phone", "cargo", "instructions", "priority", "pickup time":
			return true
		}
	}
	return false
}

// Assign gives a shipment to a driver, or hands it to another one. The
// vehicle defaults to the driver's usual truck.
func (s *ShipmentService) Assign(ctx context.Context, a Actor, id pgtype.UUID, driverID, vehicleID string) (db.Shipment, error) {
	var out db.Shipment
	err := s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		cur, err := s.lock(ctx, q, a.TenantID, id)
		if err != nil {
			return err
		}
		if !model.CanAssign(cur.Status) {
			return conflict(fmt.Sprintf("A %s shipment can't be reassigned.", strings.ToLower(model.StatusLabel(cur.Status))))
		}
		tenant, err := q.GetTenantByID(ctx, a.TenantID)
		if err != nil {
			return err
		}
		// Read-only mode stops new work, not work under way: a load already
		// assigned can still change hands.
		if cur.Status == model.StatusPending {
			if err := s.requireWritable(ctx, q, tenant); err != nil {
				return err
			}
		}
		driver, err := s.assignableDriver(ctx, q, tenant, driverID, id)
		if err != nil {
			return err
		}
		weight, _ := numericFloat(cur.CargoWeight)
		vid, err := s.assignableVehicle(ctx, q, a.TenantID, vehicleID, driver, id, weight)
		if err != nil {
			return err
		}
		if cur.DriverID == driver.ID && cur.VehicleID == vid {
			out = cur
			return nil
		}

		status := cur.Status
		if status == model.StatusPending {
			status = model.StatusAssigned
		}
		out, err = q.SetShipmentAssignment(ctx, db.SetShipmentAssignmentParams{
			ID: id, TenantID: a.TenantID, DriverID: driver.ID, VehicleID: vid, Status: status,
		})
		if err != nil {
			return err
		}

		ev := shipmentEvent{Type: "assigned", From: cur.Status, To: status, DriverID: driver.ID, Public: cur.Status == model.StatusPending}
		if cur.DriverID.Valid && cur.DriverID != driver.ID {
			ev.PreviousDriverID = cur.DriverID
		}
		if v, err := q.GetVehicleByID(ctx, db.GetVehicleByIDParams{ID: vid, TenantID: a.TenantID}); err == nil {
			ev.Details = map[string]any{"vehicle": v.PlateNumber}
		}
		s.event(ctx, q, a, out, ev)

		if cur.DriverID != driver.ID && tenant.SmsDriverOnAssign {
			s.notifyDriverAssigned(ctx, q, after, tenant, out, driver)
		}
		if ev.PreviousDriverID.Valid && model.IsOnRoad(cur.Status) {
			if prev, err := q.GetUserByID(ctx, db.GetUserByIDParams{ID: cur.DriverID, TenantID: a.TenantID}); err == nil && prev.Phone != nil {
				s.enqueueSMS(ctx, q, after, outboundSMS{
					TenantID: a.TenantID, ShipmentID: id, Phone: *prev.Phone, Purpose: PurposeDriverMessage,
					Body: fmt.Sprintf("%s: shipment %s has been handed to %s.", tenant.Name, ShortRef(out.TrackingNumber), driver.FullName),
				})
			}
		}
		after.do(func() { s.publishShipment(out, "assigned") })
		return nil
	})
	return out, err
}

// Unassign takes the driver off a shipment that has not left yet.
func (s *ShipmentService) Unassign(ctx context.Context, a Actor, id pgtype.UUID) (db.Shipment, error) {
	var out db.Shipment
	err := s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		cur, err := s.lock(ctx, q, a.TenantID, id)
		if err != nil {
			return err
		}
		if !cur.DriverID.Valid {
			out = cur
			return nil
		}
		if !model.CanUnassign(cur.Status) {
			return conflict("The shipment is already on the road. Reassign it to another driver instead.")
		}
		out, err = q.SetShipmentAssignment(ctx, db.SetShipmentAssignmentParams{
			ID: id, TenantID: a.TenantID, Status: model.StatusPending,
		})
		if err != nil {
			return err
		}
		s.event(ctx, q, a, out, shipmentEvent{
			Type: "unassigned", From: cur.Status, To: model.StatusPending, PreviousDriverID: cur.DriverID,
		})
		after.do(func() { s.publishShipment(out, "unassigned") })
		return nil
	})
	return out, err
}

// assignableDriver loads a driver of this tenant who can take another
// shipment. exclude is the shipment being assigned, which does not count
// against the driver's limit if they already have it.
func (s *ShipmentService) assignableDriver(ctx context.Context, q *db.Queries, tenant db.Tenant, driverID string, exclude pgtype.UUID) (db.User, error) {
	id, err := parseUUID(driverID)
	if err != nil {
		return db.User{}, invalid("driver_id", "Choose a driver.")
	}
	d, err := q.GetUserByID(ctx, db.GetUserByIDParams{ID: id, TenantID: tenant.ID})
	if err != nil || d.Role != model.RoleDriver {
		return db.User{}, invalid("driver_id", "Choose a driver.")
	}
	if !d.IsActive {
		return db.User{}, invalid("driver_id", d.FullName+" is deactivated.")
	}
	active, err := q.CountActiveShipmentsByDriver(ctx, db.CountActiveShipmentsByDriverParams{DriverID: d.ID, TenantID: tenant.ID})
	if err != nil {
		return db.User{}, err
	}
	if exclude.Valid {
		if cur, err := q.GetShipmentByID(ctx, db.GetShipmentByIDParams{ID: exclude, TenantID: tenant.ID}); err == nil &&
			cur.DriverID == d.ID && model.IsActiveStatus(cur.Status) {
			active--
		}
	}
	if tenant.MaxActivePerDriver > 0 && active >= int64(tenant.MaxActivePerDriver) {
		return db.User{}, invalid("driver_id", fmt.Sprintf(
			"%s already has %d active shipments, the most your company allows. Complete or reassign one, or raise the limit in Settings.",
			d.FullName, active))
	}
	return d, nil
}

// assignableVehicle resolves the vehicle for an assignment of shipment (none
// yet when booking) weighing weightKg: the one chosen, or the driver's usual
// truck. A vehicle out of service, on another driver's trip, or that the
// load would take over its capacity is refused when chosen. The driver's
// usual truck in that state is left off instead, so that assigning the
// driver still works.
func (s *ShipmentService) assignableVehicle(ctx context.Context, q *db.Queries, tenantID pgtype.UUID, vehicleID string, driver db.User, shipment pgtype.UUID, weightKg float64) (pgtype.UUID, error) {
	chosen := strings.TrimSpace(vehicleID) != ""
	refuse := func(msg string) (pgtype.UUID, error) {
		if !chosen {
			return pgtype.UUID{}, nil
		}
		return pgtype.UUID{}, invalid("vehicle_id", msg)
	}
	id := driver.VehicleID
	if chosen {
		var err error
		if id, err = parseUUID(vehicleID); err != nil {
			return refuse("Choose a vehicle.")
		}
	}
	if !id.Valid {
		return pgtype.UUID{}, nil
	}
	v, err := q.GetVehicleByID(ctx, db.GetVehicleByIDParams{ID: id, TenantID: tenantID})
	if err != nil {
		return refuse("Choose a vehicle.")
	}
	if !v.IsActive {
		return refuse(v.PlateNumber + " is out of service.")
	}
	load, err := q.GetVehicleLoad(ctx, db.GetVehicleLoadParams{TenantID: tenantID, VehicleID: v.ID, DriverID: driver.ID, Exclude: shipment})
	if err != nil {
		return pgtype.UUID{}, err
	}
	if load.OtherDriver != "" {
		return refuse(fmt.Sprintf("%s is on %s's trip. Choose another vehicle, or finish or reassign that trip first.", v.PlateNumber, load.OtherDriver))
	}
	if capacity, ok := numericFloat(v.CapacityKg); ok && capacity > 0 && load.WeightKg+weightKg > capacity {
		kg := func(x float64) string { return grouped(int64(math.Round(x))) }
		if load.WeightKg > 0 {
			return refuse(fmt.Sprintf("%s carries up to %s kg and already has %s kg on board: this load would take it to %s kg.",
				v.PlateNumber, kg(capacity), kg(load.WeightKg), kg(load.WeightKg+weightKg)))
		}
		return refuse(fmt.Sprintf("%s carries up to %s kg; this load is %s kg.", v.PlateNumber, kg(capacity), kg(weightKg)))
	}
	return v.ID, nil
}

func (s *ShipmentService) lock(ctx context.Context, q *db.Queries, tenantID, id pgtype.UUID) (db.Shipment, error) {
	sh, err := q.GetShipmentByIDForUpdate(ctx, db.GetShipmentByIDForUpdateParams{ID: id, TenantID: tenantID})
	if isNoRows(err) {
		return sh, notFound("Shipment not found.")
	}
	return sh, err
}

// ---------------------------------------------------------------------------
// Queries
// ---------------------------------------------------------------------------

// ShipmentFilter narrows the shipment list (FR-031).
type ShipmentFilter struct {
	Statuses    []string
	DriverID    string
	VehicleID   string
	Priority    string
	From, To    *time.Time
	Origin      string
	Destination string
	Query       string
	Sort        string
	Page        int
	PerPage     int
}

// ShipmentSorts are the accepted sort keys.
var ShipmentSorts = []string{"created_desc", "created_asc", "updated_desc", "eta_asc", "eta_desc", "status", "priority", "customer", "tracking"}

// List returns a page of shipments and the total matching.
func (s *ShipmentService) List(ctx context.Context, tenantID pgtype.UUID, f ShipmentFilter) ([]db.ListShipmentsRow, int64, error) {
	limit, offset := pageBounds(f.Page, f.PerPage)
	statuses := []string{}
	for _, st := range f.Statuses {
		if model.IsValidStatus(st) {
			statuses = append(statuses, st)
		}
	}
	if !contains(ShipmentSorts, f.Sort) {
		f.Sort = "created_desc"
	}
	var from, to pgtype.Timestamptz
	if f.From != nil {
		from = toTimestamp(*f.From)
	}
	if f.To != nil {
		to = toTimestamp(*f.To)
	}
	var priority *string
	if contains(model.Priorities, f.Priority) {
		priority = &f.Priority
	}
	// Phones are stored as 09…: a number typed as +251… finds them too.
	query := strings.TrimSpace(f.Query)
	if local, err := phone.Normalize(query); err == nil {
		query = local
	}

	list := db.ListShipmentsParams{
		TenantID:    tenantID,
		Statuses:    statuses,
		DriverID:    toUUID(f.DriverID),
		VehicleID:   toUUID(f.VehicleID),
		Priority:    priority,
		DateFrom:    from,
		DateTo:      to,
		Origin:      toText(strings.TrimSpace(f.Origin)),
		Destination: toText(strings.TrimSpace(f.Destination)),
		Query:       toText(escapeLike(query)),
		Sort:        f.Sort,
		RowLimit:    limit,
		RowOffset:   offset,
	}
	rows, err := s.q.ListShipments(ctx, list)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountShipments(ctx, db.CountShipmentsParams{
		TenantID: list.TenantID, Statuses: list.Statuses, DriverID: list.DriverID, VehicleID: list.VehicleID,
		Priority: list.Priority, DateFrom: list.DateFrom, DateTo: list.DateTo, Origin: list.Origin,
		Destination: list.Destination, Query: list.Query,
	})
	return rows, total, err
}

// escapeLike neutralises LIKE wildcards in a search term.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// Get loads one shipment of a tenant.
func (s *ShipmentService) Get(ctx context.Context, tenantID, id pgtype.UUID) (db.Shipment, error) {
	sh, err := s.q.GetShipmentByID(ctx, db.GetShipmentByIDParams{ID: id, TenantID: tenantID})
	if isNoRows(err) {
		return sh, notFound("Shipment not found.")
	}
	return sh, err
}

// ShipmentDetail is everything the shipment page shows.
type ShipmentDetail struct {
	Shipment db.Shipment
	Driver   *db.User
	Vehicle  *db.Vehicle
	Events   []db.ShipmentEvent
	POD      *db.GetPODByShipmentIDRow
	// Route is the trip's newest routePoints positions, oldest first.
	Route []db.ListShipmentRouteRow
	// RouteCapped says older positions were left out of Route.
	RouteCapped bool
	// Next lists the status changes a dispatcher can make from here.
	Next []string
}

// The shipment page draws a trip's newest 2,000 positions (about a day and a
// half of driving at one a minute). The CSV export has every one.
const (
	routePoints  = 2000
	exportPoints = 200000
)

// Detail assembles the shipment page.
func (s *ShipmentService) Detail(ctx context.Context, tenantID, id pgtype.UUID) (*ShipmentDetail, error) {
	sh, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	d := &ShipmentDetail{Shipment: sh, Next: model.AllowedTransitions(sh.Status, model.ActorDispatcher)}
	if sh.DriverID.Valid {
		if u, err := s.q.GetUserByID(ctx, db.GetUserByIDParams{ID: sh.DriverID, TenantID: tenantID}); err == nil {
			d.Driver = &u
		}
	}
	if sh.VehicleID.Valid {
		if v, err := s.q.GetVehicleByID(ctx, db.GetVehicleByIDParams{ID: sh.VehicleID, TenantID: tenantID}); err == nil {
			d.Vehicle = &v
		}
	}
	if d.Events, err = s.q.ListShipmentEvents(ctx, db.ListShipmentEventsParams{ShipmentID: id, TenantID: tenantID}); err != nil {
		return nil, err
	}
	if pod, err := s.q.GetPODByShipmentID(ctx, db.GetPODByShipmentIDParams{ShipmentID: id, TenantID: tenantID}); err == nil {
		d.POD = &pod
	}
	if d.Route, err = s.route(ctx, sh, routePoints); err != nil {
		return nil, err
	}
	d.RouteCapped = len(d.Route) == routePoints
	return d, nil
}

// Positions is every recorded position of a shipment's trip, oldest first, for
// the location export (AC-030.7).
func (s *ShipmentService) Positions(ctx context.Context, tenantID, id pgtype.UUID) (db.Shipment, []db.ListShipmentRouteRow, error) {
	sh, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return db.Shipment{}, nil, err
	}
	pts, err := s.route(ctx, sh, exportPoints)
	return sh, pts, err
}

func (s *ShipmentService) route(ctx context.Context, sh db.Shipment, max int32) ([]db.ListShipmentRouteRow, error) {
	start, end := shipmentWindow(sh, s.now())
	return s.q.ListShipmentRoute(ctx, db.ListShipmentRouteParams{
		TenantID: sh.TenantID, ShipmentID: sh.ID, DriverID: sh.DriverID,
		WindowStart: toTimestamp(start), WindowEnd: toTimestamp(end), MaxPoints: max,
	})
}

// shipmentWindow is the span during which the shipment was on its driver's
// truck, for attributing the driver's GPS track to it.
func shipmentWindow(sh db.Shipment, now time.Time) (time.Time, time.Time) {
	start := sh.CreatedAt.Time
	if sh.PickedUpAt.Valid {
		start = sh.PickedUpAt.Time
	}
	end := now
	if sh.ActualDelivery.Valid {
		end = sh.ActualDelivery.Time
	} else if model.IsFinalStatus(sh.Status) && sh.StatusChangedAt.Valid {
		end = sh.StatusChangedAt.Time
	}
	return start, end
}

// Activity is the dashboard's recent-events feed.
func (s *ShipmentService) Activity(ctx context.Context, tenantID pgtype.UUID, limit int32) ([]db.ListTenantActivityRow, error) {
	return s.q.ListTenantActivity(ctx, db.ListTenantActivityParams{TenantID: tenantID, Limit: limit})
}

// StatusCounts returns how many shipments are in each status.
func (s *ShipmentService) StatusCounts(ctx context.Context, tenantID pgtype.UUID) (map[string]int64, error) {
	rows, err := s.q.CountShipmentsByStatus(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.Status] = r.Total
	}
	return out, nil
}

// DeliveredSince counts deliveries since a moment, for "delivered today".
func (s *ShipmentService) DeliveredSince(ctx context.Context, tenantID pgtype.UUID, since time.Time) (int64, error) {
	return s.q.CountDeliveredSince(ctx, db.CountDeliveredSinceParams{TenantID: tenantID, ActualDelivery: toTimestamp(since)})
}

// ---------------------------------------------------------------------------
// History
// ---------------------------------------------------------------------------

type shipmentEvent struct {
	Type             string
	From, To         string
	DriverID         pgtype.UUID
	PreviousDriverID pgtype.UUID
	Note             string
	Reason           string
	Lat, Lng         *float64
	VisibleToDriver  bool
	Public           bool
	Details          map[string]any
	ClientID         string
	OccurredAt       time.Time
}

// event appends to a shipment's timeline. It reports false when the event is
// a replay of one already recorded (same client_id).
func (s *ShipmentService) event(ctx context.Context, q *db.Queries, a Actor, sh db.Shipment, e shipmentEvent) bool {
	occurred := e.OccurredAt
	if occurred.IsZero() {
		occurred = s.now()
	}
	var details []byte
	if len(e.Details) > 0 {
		details = mustJSON(e.Details)
	}
	_, err := q.CreateShipmentEvent(ctx, db.CreateShipmentEventParams{
		TenantID:         sh.TenantID,
		ShipmentID:       sh.ID,
		EventType:        e.Type,
		FromStatus:       toText(e.From),
		ToStatus:         toText(e.To),
		DriverID:         e.DriverID,
		PreviousDriverID: e.PreviousDriverID,
		ActorID:          a.UserID,
		ActorName:        a.Name,
		Source:           a.Source,
		Note:             toText(e.Note),
		Reason:           toText(e.Reason),
		Latitude:         e.Lat,
		Longitude:        e.Lng,
		VisibleToDriver:  e.VisibleToDriver,
		Public:           e.Public,
		Details:          details,
		ClientID:         toText(e.ClientID),
		OccurredAt:       toTimestamp(occurred),
	})
	if isNoRows(err) {
		return false
	}
	if err != nil {
		s.log.Error().Err(err).Str("type", e.Type).Msg("record shipment event")
		return false
	}
	return true
}

func (s *ShipmentService) publishShipment(sh db.Shipment, change string) {
	s.publish(sh.TenantID, "shipment", map[string]any{
		"id":              sh.ID.String(),
		"tracking_number": sh.TrackingNumber,
		"status":          sh.Status,
		"status_label":    model.StatusLabel(sh.Status),
		"change":          change,
	})
}

// Tracking numbers look like ET-20261028-7K3M9QX2PA. The ten-character code
// uses Crockford's alphabet, which drops letters easily misread or misheard
// between 1/I/L and 0/O: customers read these out over the phone and receive
// them by SMS. 32^10 (~2^50) codes per day keep them unguessable, which
// matters because the tracking page is public.
const trackingAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func newTrackingNumber(day time.Time) string {
	code := make([]byte, 10)
	if _, err := rand.Read(code); err != nil {
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	// 256 is a multiple of 32, so the modulo adds no bias.
	for i, b := range code {
		code[i] = trackingAlphabet[b%32]
	}
	return "ET-" + day.Format("20060102") + "-" + string(code)
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
