package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
)

// VehicleTypes are the kinds of vehicle a fleet registers.
var VehicleTypes = []struct{ Code, Label string }{
	{"pickup", "Pickup"},
	{"light_truck", "Light truck (up to 7 t)"},
	{"medium_truck", "Medium truck (7–15 t)"},
	{"heavy_truck", "Heavy truck (over 15 t)"},
	{"trailer", "Truck and trailer"},
	{"tanker", "Tanker"},
	{"refrigerated", "Refrigerated"},
	{"van", "Van"},
}

// VehicleTypeLabel is the display name of a vehicle type code.
func VehicleTypeLabel(code string) string {
	for _, t := range VehicleTypes {
		if t.Code == code {
			return t.Label
		}
	}
	return code
}

// VehicleService manages the fleet (FR-005).
type VehicleService struct{ *core }

// VehicleInput is the add/edit vehicle form.
type VehicleInput struct {
	PlateNumber string
	VehicleType string
	CapacityKg  float64
	Active      bool
	// InsuranceExpires and InspectionDue are the last days the insurance
	// and the annual inspection (ቦሎ) are valid, as 2026-12-02; empty when
	// not recorded.
	InsuranceExpires string
	InspectionDue    string
}

type vehiclePapers struct{ insurance, inspection pgtype.Date }

func validatePapers(in VehicleInput) (vehiclePapers, error) {
	var p vehiclePapers
	var err error
	if p.insurance, err = parseDay("insurance_expires_on", "the insurance runs out", in.InsuranceExpires); err != nil {
		return p, err
	}
	p.inspection, err = parseDay("inspection_due_on", "the inspection is due", in.InspectionDue)
	return p, err
}

func normalisePlate(s string) string {
	return strings.ToUpper(strings.Join(strings.Fields(s), " "))
}

func validateVehicle(in VehicleInput) (string, *string, pgtype.Numeric, error) {
	plate := normalisePlate(in.PlateNumber)
	if plate == "" {
		return "", nil, pgtype.Numeric{}, invalid("plate_number", "Enter the plate number.")
	}
	if utf8.RuneCountInString(plate) > 20 {
		return "", nil, pgtype.Numeric{}, invalid("plate_number", "That plate number is too long.")
	}
	var vt *string
	if in.VehicleType != "" {
		found := false
		for _, t := range VehicleTypes {
			found = found || t.Code == in.VehicleType
		}
		if !found {
			return "", nil, pgtype.Numeric{}, invalid("vehicle_type", "Choose a vehicle type.")
		}
		vt = &in.VehicleType
	}
	if in.CapacityKg < 0 || in.CapacityKg > 100000 {
		return "", nil, pgtype.Numeric{}, invalid("capacity_kg", "Enter a capacity between 0 and 100,000 kg.")
	}
	return plate, vt, toNumericOmitZero(in.CapacityKg), nil
}

func duplicatePlate(err error) error {
	if _, ok := isUniqueViolation(err); ok {
		return invalid("plate_number", "A vehicle with this plate is already registered.")
	}
	return err
}

// List returns every vehicle with its drivers and current load.
func (s *VehicleService) List(ctx context.Context, tenantID pgtype.UUID) ([]db.ListVehiclesWithDriversRow, error) {
	return s.q.ListVehiclesWithDrivers(ctx, tenantID)
}

// Active returns vehicles in service, for pickers.
func (s *VehicleService) Active(ctx context.Context, tenantID pgtype.UUID) ([]db.Vehicle, error) {
	return s.q.ListActiveVehiclesByTenant(ctx, tenantID)
}

// Get loads one vehicle.
func (s *VehicleService) Get(ctx context.Context, tenantID, id pgtype.UUID) (db.Vehicle, error) {
	v, err := s.q.GetVehicleByID(ctx, db.GetVehicleByIDParams{ID: id, TenantID: tenantID})
	if isNoRows(err) {
		return v, notFound("Vehicle not found.")
	}
	return v, err
}

// Recent lists a vehicle's latest shipments.
func (s *VehicleService) Recent(ctx context.Context, tenantID, id pgtype.UUID) ([]db.ListRecentShipmentsByVehicleRow, error) {
	return s.q.ListRecentShipmentsByVehicle(ctx, db.ListRecentShipmentsByVehicleParams{VehicleID: id, TenantID: tenantID, Limit: 20})
}

// Create registers a vehicle.
func (s *VehicleService) Create(ctx context.Context, p *Principal, in VehicleInput, ci ClientInfo) (db.Vehicle, error) {
	if !p.CanManageFleet() {
		return db.Vehicle{}, ErrUnauthorized
	}
	plate, vt, capacity, err := validateVehicle(in)
	if err != nil {
		return db.Vehicle{}, err
	}
	papers, err := validatePapers(in)
	if err != nil {
		return db.Vehicle{}, err
	}
	tenant, err := s.q.GetTenantByID(ctx, p.TenantID)
	if err != nil {
		return db.Vehicle{}, err
	}
	if err := s.requireWritable(ctx, s.q, tenant); err != nil {
		return db.Vehicle{}, err
	}
	v, err := s.q.CreateVehicle(ctx, db.CreateVehicleParams{TenantID: p.TenantID, PlateNumber: plate, VehicleType: vt, CapacityKg: capacity,
		InsuranceExpiresOn: papers.insurance, InspectionDueOn: papers.inspection})
	if err != nil {
		return v, duplicatePlate(err)
	}
	s.audit(ctx, s.q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "vehicle_created", "vehicle", v.ID, nil, map[string]any{"plate": plate})
	return v, nil
}

// Update edits a vehicle. Taking it out of service unlinks it from drivers
// who had it as their usual truck.
func (s *VehicleService) Update(ctx context.Context, p *Principal, id pgtype.UUID, in VehicleInput, ci ClientInfo) (db.Vehicle, error) {
	if !p.CanManageFleet() {
		return db.Vehicle{}, ErrUnauthorized
	}
	plate, vt, capacity, err := validateVehicle(in)
	if err != nil {
		return db.Vehicle{}, err
	}
	papers, err := validatePapers(in)
	if err != nil {
		return db.Vehicle{}, err
	}
	var out db.Vehicle
	err = s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		cur, err := s.Get(ctx, p.TenantID, id)
		if err != nil {
			return err
		}
		out, err = q.UpdateVehicle(ctx, db.UpdateVehicleParams{
			ID: id, TenantID: p.TenantID, PlateNumber: plate, VehicleType: vt, CapacityKg: capacity, IsActive: in.Active,
			InsuranceExpiresOn: papers.insurance, InspectionDueOn: papers.inspection,
		})
		if err != nil {
			return duplicatePlate(err)
		}
		for _, c := range []struct {
			alert    string
			was, now pgtype.Date
		}{{AlertInsuranceDue, cur.InsuranceExpiresOn, out.InsuranceExpiresOn}, {AlertInspectionDue, cur.InspectionDueOn, out.InspectionDueOn}} {
			if !sameDay(c.was, c.now) || (cur.IsActive && !in.Active) {
				if err := s.papersRenewed(ctx, q, p.TenantID, c.alert, pgtype.UUID{}, id); err != nil {
					return err
				}
			}
		}
		if cur.IsActive && !in.Active {
			if err := q.ClearVehicleFromDrivers(ctx, db.ClearVehicleFromDriversParams{VehicleID: id, TenantID: p.TenantID}); err != nil {
				return err
			}
		}
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "vehicle_updated", "vehicle", id,
			map[string]any{"plate": cur.PlateNumber, "active": cur.IsActive, "insurance": FormatDay(cur.InsuranceExpiresOn), "inspection": FormatDay(cur.InspectionDueOn)},
			map[string]any{"plate": out.PlateNumber, "active": out.IsActive, "insurance": FormatDay(out.InsuranceExpiresOn), "inspection": FormatDay(out.InspectionDueOn)})
		return nil
	})
	return out, err
}
