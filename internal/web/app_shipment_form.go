package web

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/geo"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// shipmentFormData is what the booking form needs besides the values.
func (s *Server) shipmentFormData(r *http.Request, sh *db.Shipment) (map[string]any, error) {
	p := principal(r)
	drivers, err := s.svc.Users.Drivers(r.Context(), p.TenantID, true, "")
	if err != nil {
		return nil, err
	}
	vehicles, err := s.svc.Vehicles.Active(r.Context(), p.TenantID)
	if err != nil {
		return nil, err
	}
	tenant, err := s.svc.Tenants.Get(r.Context(), p.TenantID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"Towns": geo.Places(), "Drivers": drivers, "Vehicles": vehicles, "Priorities": model.Priorities,
		"Shipment": sh, "MaxActive": tenant.MaxActivePerDriver, "Region": tenant.RegionCity,
	}, nil
}

func shipmentFormValues(sh db.Shipment, loc *time.Location) url.Values {
	f := func(p *float64) string {
		if p == nil {
			return ""
		}
		return strconv.FormatFloat(*p, 'f', 6, 64)
	}
	v := url.Values{
		"priority":             {sh.Priority},
		"origin_city":          {deref(sh.OriginCity)},
		"origin_address":       {sh.OriginAddress},
		"destination_city":     {deref(sh.DestinationCity)},
		"destination_address":  {sh.DestinationAddress},
		"customer_name":        {sh.CustomerName},
		"customer_phone":       {sh.CustomerPhone},
		"cargo_description":    {deref(sh.CargoDescription)},
		"cargo_weight":         {numStr(toFloat(sh.CargoWeight))},
		"cargo_value":          {numStr(toFloat(sh.CargoValue))},
		"special_instructions": {deref(sh.SpecialInstructions)},
	}
	if sh.DestinationPrecise {
		v.Set("destination_lat", f(sh.DestinationLat))
		v.Set("destination_lng", f(sh.DestinationLng))
	}
	if sh.OriginLat != nil {
		if p, ok := geo.Lookup(deref(sh.OriginCity)); !ok || p.Lat != *sh.OriginLat || p.Lng != *sh.OriginLng {
			v.Set("origin_lat", f(sh.OriginLat))
			v.Set("origin_lng", f(sh.OriginLng))
		}
	}
	if sh.PlannedPickupAt.Valid {
		v.Set("planned_pickup", sh.PlannedPickupAt.Time.In(loc).Format("2006-01-02T15:04"))
	}
	if sh.EtaManual && sh.EstimatedDelivery.Valid {
		v.Set("estimated_delivery", sh.EstimatedDelivery.Time.In(loc).Format("2006-01-02T15:04"))
	}
	return v
}

func (s *Server) shipmentNew(w http.ResponseWriter, r *http.Request) {
	data, err := s.shipmentFormData(r, nil)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(w, r, "New shipment", "shipments", data)
	v.Form.Set("origin_city", data["Region"].(string))
	// "Book again" pre-fills from an earlier shipment for repeat customers.
	if id := r.URL.Query().Get("copy"); id != "" {
		if sid, err := parseID(id); err == nil {
			if sh, err := s.svc.Shipments.Get(r.Context(), principal(r).TenantID, sid); err == nil {
				v.Form = shipmentFormValues(sh, s.cfg.Timezone)
				v.Form.Del("estimated_delivery")
				v.Form.Del("planned_pickup")
			}
		}
	}
	s.render(w, r, http.StatusOK, "app/shipment_form", v)
}

func (s *Server) shipmentInput(r *http.Request) service.ShipmentInput {
	opt := func(name string) *float64 {
		v := strings.TrimSpace(r.PostFormValue(name))
		if v == "" {
			return nil
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil
		}
		return &f
	}
	num := func(name string) float64 {
		f, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(r.PostFormValue(name)), ",", ""), 64)
		return f
	}
	in := service.ShipmentInput{
		Priority:            r.PostFormValue("priority"),
		OriginCity:          r.PostFormValue("origin_city"),
		OriginAddress:       r.PostFormValue("origin_address"),
		OriginLat:           opt("origin_lat"),
		OriginLng:           opt("origin_lng"),
		DestinationCity:     r.PostFormValue("destination_city"),
		DestinationAddress:  r.PostFormValue("destination_address"),
		DestinationLat:      opt("destination_lat"),
		DestinationLng:      opt("destination_lng"),
		CustomerName:        r.PostFormValue("customer_name"),
		CustomerPhone:       r.PostFormValue("customer_phone"),
		CargoDescription:    r.PostFormValue("cargo_description"),
		CargoWeightKg:       num("cargo_weight"),
		CargoValueETB:       num("cargo_value"),
		SpecialInstructions: r.PostFormValue("special_instructions"),
		DriverID:            r.PostFormValue("driver_id"),
		VehicleID:           r.PostFormValue("vehicle_id"),
	}
	when := func(name string) *time.Time {
		t, err := time.ParseInLocation("2006-01-02T15:04", r.PostFormValue(name), s.cfg.Timezone)
		if err != nil {
			return nil
		}
		return &t
	}
	in.PlannedPickup = when("planned_pickup")
	in.EstimatedDelivery = when("estimated_delivery")
	return in
}

func (s *Server) shipmentCreate(w http.ResponseWriter, r *http.Request) {
	sh, err := s.svc.Shipments.Create(r.Context(), actor(r), s.shipmentInput(r))
	if err != nil {
		data, derr := s.shipmentFormData(r, nil)
		if derr != nil {
			s.fail(w, r, derr)
			return
		}
		if !s.formError(w, r, "app/shipment_form", s.view(w, r, "New shipment", "shipments", data), err) {
			s.fail(w, r, err)
		}
		return
	}
	msg := "Shipment " + sh.TrackingNumber + " booked."
	if sh.DriverID.Valid {
		msg = "Shipment " + sh.TrackingNumber + " booked and assigned."
	}
	s.setFlash(w, "success", msg)
	http.Redirect(w, r, "/app/shipments/"+sh.ID.String(), http.StatusSeeOther)
}

func (s *Server) shipmentEdit(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	sh, err := s.svc.Shipments.Get(r.Context(), principal(r).TenantID, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if model.IsFinalStatus(sh.Status) {
		s.setFlash(w, "error", "A "+strings.ToLower(model.StatusLabel(sh.Status))+" shipment can't be edited.")
		http.Redirect(w, r, "/app/shipments/"+sh.ID.String(), http.StatusSeeOther)
		return
	}
	data, err := s.shipmentFormData(r, &sh)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(w, r, "Edit "+sh.TrackingNumber, "shipments", data)
	v.Form = shipmentFormValues(sh, s.cfg.Timezone)
	s.render(w, r, http.StatusOK, "app/shipment_form", v)
}

func (s *Server) shipmentUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	sh, err := s.svc.Shipments.Update(r.Context(), actor(r), id, s.shipmentInput(r))
	if err != nil {
		cur, gerr := s.svc.Shipments.Get(r.Context(), principal(r).TenantID, id)
		if gerr != nil {
			s.fail(w, r, gerr)
			return
		}
		data, derr := s.shipmentFormData(r, &cur)
		if derr != nil {
			s.fail(w, r, derr)
			return
		}
		if !s.formError(w, r, "app/shipment_form", s.view(w, r, "Edit "+cur.TrackingNumber, "shipments", data), err) {
			s.fail(w, r, err)
		}
		return
	}
	s.setFlash(w, "success", "Changes saved.")
	http.Redirect(w, r, "/app/shipments/"+sh.ID.String(), http.StatusSeeOther)
}
