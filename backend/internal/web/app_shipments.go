package web

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/geo"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/internal/service"
)

// shipmentTabs are the preset status groups above the shipment list.
var shipmentTabs = []struct {
	Key, Label string
	Statuses   []string
}{
	{"", "All", nil},
	{"pending", "Unassigned", []string{model.StatusPending}},
	{"active", "Active", model.ActiveStatuses},
	{"attention", "Needs attention", []string{model.StatusDelayed, model.StatusIssue}},
	{"delivered", "Delivered", []string{model.StatusDelivered}},
	{"cancelled", "Cancelled", []string{model.StatusCancelled}},
}

func (s *Server) parseDate(v string, endOfDay bool) *time.Time {
	t, err := time.ParseInLocation("2006-01-02", v, s.cfg.Timezone)
	if err != nil {
		return nil
	}
	if endOfDay {
		t = t.AddDate(0, 0, 1)
	}
	return &t
}

func (s *Server) shipmentFilter(q url.Values) service.ShipmentFilter {
	f := service.ShipmentFilter{
		Statuses:    q["status"],
		DriverID:    q.Get("driver"),
		VehicleID:   q.Get("vehicle"),
		Priority:    q.Get("priority"),
		From:        s.parseDate(q.Get("from"), false),
		To:          s.parseDate(q.Get("to"), true),
		Origin:      q.Get("origin"),
		Destination: q.Get("destination"),
		Query:       q.Get("q"),
		Sort:        q.Get("sort"),
	}
	f.Page, _ = strconv.Atoi(q.Get("page"))
	f.PerPage = 25
	if tab := q.Get("tab"); tab != "" && len(f.Statuses) == 0 {
		for _, t := range shipmentTabs {
			if t.Key == tab {
				f.Statuses = t.Statuses
			}
		}
	}
	return f
}

func (s *Server) shipmentList(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	q := r.URL.Query()
	f := s.shipmentFilter(q)
	if q.Get("format") == "csv" {
		s.shipmentCSV(w, r, f)
		return
	}
	rows, total, err := s.svc.Shipments.List(r.Context(), p.TenantID, f)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	drivers, err := s.svc.Users.Drivers(r.Context(), p.TenantID, false, "")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	page := max(f.Page, 1)
	pages := int((total + int64(f.PerPage) - 1) / int64(f.PerPage))
	filtered := q.Get("q") != "" || q.Get("driver") != "" || q.Get("priority") != "" || q.Get("from") != "" ||
		q.Get("to") != "" || q.Get("origin") != "" || q.Get("destination") != "" || len(q["status"]) > 0
	v := s.view(w, r, "Shipments", "shipments", map[string]any{
		"Rows": rows, "Total": total, "Page": page, "Pages": pages, "Tabs": shipmentTabs, "Tab": q.Get("tab"),
		"Drivers": drivers, "Statuses": model.AllStatuses, "Priorities": model.Priorities, "Filtered": filtered,
		"Towns": geo.Places(), "Highlight": q.Get("highlight"),
	})
	s.render(w, r, http.StatusOK, "app/shipments", v)
}

// shipmentCSV exports every shipment matching the filter (FR-031 AC-031.5).
func (s *Server) shipmentCSV(w http.ResponseWriter, r *http.Request, f service.ShipmentFilter) {
	p := principal(r)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="shipments-%s.csv"`, time.Now().In(s.cfg.Timezone).Format("2006-01-02")))
	_, _ = w.Write([]byte("\xEF\xBB\xBF")) // so Excel reads UTF-8
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"Tracking number", "Status", "Priority", "Created", "Customer", "Customer phone",
		"Origin", "Origin address", "Destination", "Destination address", "Driver", "Vehicle",
		"Cargo", "Weight (kg)", "Value (ETB)", "ETA", "Picked up", "Delivered", "On time"})
	loc := s.cfg.Timezone
	ts := func(v any) string {
		t, ok := toTime(v)
		if !ok {
			return ""
		}
		return t.In(loc).Format("2006-01-02 15:04")
	}
	f.PerPage = 200
	for f.Page = 1; f.Page <= 50; f.Page++ {
		rows, _, err := s.svc.Shipments.List(r.Context(), p.TenantID, f)
		if err != nil {
			s.log.Error().Err(err).Msg("csv export")
			break
		}
		for _, row := range rows {
			sh := row.Shipment
			onTime := ""
			if sh.ActualDelivery.Valid && sh.EstimatedDelivery.Valid {
				onTime = "no"
				if !sh.ActualDelivery.Time.After(sh.EstimatedDelivery.Time) {
					onTime = "yes"
				}
			}
			_ = cw.Write([]string{
				sh.TrackingNumber, model.StatusLabel(sh.Status), sh.Priority, ts(sh.CreatedAt),
				csvSafe(sh.CustomerName), sh.CustomerPhone,
				deref(sh.OriginCity), csvSafe(sh.OriginAddress), deref(sh.DestinationCity), csvSafe(sh.DestinationAddress),
				csvSafe(deref(row.DriverName)), deref(row.VehiclePlate),
				csvSafe(deref(sh.CargoDescription)), numStr(toFloat(sh.CargoWeight)), numStr(toFloat(sh.CargoValue)),
				ts(sh.EstimatedDelivery), ts(sh.PickedUpAt), ts(sh.ActualDelivery), onTime,
			})
		}
		if len(rows) < f.PerPage {
			break
		}
	}
	cw.Flush()
}

// csvSafe defuses spreadsheet formula injection: a customer name beginning
// with "=" must not run as a formula when the export is opened.
func csvSafe(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func numStr(f float64) string {
	if f == 0 {
		return ""
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

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
	if eta := r.PostFormValue("estimated_delivery"); eta != "" {
		if t, err := time.ParseInLocation("2006-01-02T15:04", eta, s.cfg.Timezone); err == nil {
			in.EstimatedDelivery = &t
		}
	}
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

// shipmentMapData is what the shipment page's map draws.
type shipmentMapData struct {
	Origin      *mapPoint   `json:"origin"`
	Destination *mapPoint   `json:"destination"`
	Route       [][]float64 `json:"route"`
	POD         *mapPoint   `json:"pod"`
	Precise     bool        `json:"precise"`
	Tolerance   float64     `json:"tolerance"`
}

type mapPoint struct {
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
	Label string  `json:"label"`
}

func (s *Server) shipmentDetail(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	d, err := s.svc.Shipments.Detail(r.Context(), p.TenantID, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	names := s.driverNames(r.Context(), p.TenantID)
	timeline := make([]timelineItem, 0, len(d.Events))
	for i := len(d.Events) - 1; i >= 0; i-- {
		timeline = append(timeline, timelineFrom(d.Events[i], names))
	}

	sh := d.Shipment
	m := shipmentMapData{Route: [][]float64{}, Precise: sh.DestinationPrecise, Tolerance: geo.DeliveryTolerance(sh.DestinationPrecise)}
	if sh.OriginLat != nil && sh.OriginLng != nil {
		m.Origin = &mapPoint{*sh.OriginLat, *sh.OriginLng, sh.OriginAddress}
	}
	if sh.DestinationLat != nil && sh.DestinationLng != nil {
		m.Destination = &mapPoint{*sh.DestinationLat, *sh.DestinationLng, sh.DestinationAddress}
	}
	for _, pt := range d.Route {
		m.Route = append(m.Route, []float64{pt.Latitude, pt.Longitude})
	}
	if d.POD != nil && (d.POD.DeliveryLatitude != 0 || d.POD.DeliveryLongitude != 0) {
		m.POD = &mapPoint{d.POD.DeliveryLatitude, d.POD.DeliveryLongitude, "Delivered to " + d.POD.RecipientName}
	}

	var drivers []db.ListDriversWithStatusRow
	var vehicles []db.Vehicle
	if model.CanAssign(sh.Status) {
		if drivers, err = s.svc.Users.Drivers(r.Context(), p.TenantID, true, ""); err != nil {
			s.fail(w, r, err)
			return
		}
		if vehicles, err = s.svc.Vehicles.Active(r.Context(), p.TenantID); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	tenant, err := s.svc.Tenants.Get(r.Context(), p.TenantID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(w, r, sh.TrackingNumber, "shipments", map[string]any{
		"D": d, "S": sh, "Timeline": timeline, "Map": m, "Drivers": drivers, "Vehicles": vehicles,
		"CanAssign": model.CanAssign(sh.Status), "CanUnassign": sh.DriverID.Valid && model.CanUnassign(sh.Status),
		"CanCancel": model.CanCancel(sh.Status), "CanReopen": model.CanReopen(sh.Status),
		"CanEdit": !model.IsFinalStatus(sh.Status), "Next": d.Next,
		"CancelReasons": model.CancelReasons, "DelayReasons": model.DelayReasons, "IssueReasons": model.IssueReasons,
		"TrackURL": s.cfg.PublicURL + "/track/" + sh.TrackingNumber, "MaxActive": tenant.MaxActivePerDriver,
	})
	s.render(w, r, http.StatusOK, "app/shipment", v)
}

func (s *Server) shipmentWaybill(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	d, err := s.svc.Shipments.Detail(r.Context(), p.TenantID, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	tenant, err := s.svc.Tenants.Get(r.Context(), p.TenantID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(w, r, "Waybill "+d.Shipment.TrackingNumber, "shipments", map[string]any{
		"D": d, "S": d.Shipment, "Tenant": tenant, "TrackURL": s.cfg.PublicURL + "/track/" + d.Shipment.TrackingNumber,
	})
	s.render(w, r, http.StatusOK, "app/waybill", v)
}

// shipmentAction runs a POST action on a shipment and returns to its page.
func (s *Server) shipmentAction(w http.ResponseWriter, r *http.Request, do func(id pgtype.UUID) (string, error)) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	msg, err := do(id)
	s.actionResult(w, r, "/app/shipments/"+id.String(), err, msg)
}

func (s *Server) shipmentAssign(w http.ResponseWriter, r *http.Request) {
	s.shipmentAction(w, r, func(id pgtype.UUID) (string, error) {
		_, err := s.svc.Shipments.Assign(r.Context(), actor(r), id, r.PostFormValue("driver_id"), r.PostFormValue("vehicle_id"))
		return "Driver assigned.", err
	})
}

func (s *Server) shipmentUnassign(w http.ResponseWriter, r *http.Request) {
	s.shipmentAction(w, r, func(id pgtype.UUID) (string, error) {
		_, err := s.svc.Shipments.Unassign(r.Context(), actor(r), id)
		return "Driver removed. The shipment is unassigned.", err
	})
}

func (s *Server) shipmentStatus(w http.ResponseWriter, r *http.Request) {
	s.shipmentAction(w, r, func(id pgtype.UUID) (string, error) {
		status := r.PostFormValue("status")
		_, err := s.svc.Shipments.ChangeStatus(r.Context(), actor(r), id, service.StatusChange{
			Status: status, Reason: r.PostFormValue("reason"), Note: r.PostFormValue("note"), By: model.ActorDispatcher,
		})
		if errors.Is(err, model.ErrSameStatus) {
			return "The shipment is already " + strings.ToLower(model.StatusLabel(status)) + ".", nil
		}
		return "Status changed to " + strings.ToLower(model.StatusLabel(status)) + ".", err
	})
}

func (s *Server) shipmentCancel(w http.ResponseWriter, r *http.Request) {
	s.shipmentAction(w, r, func(id pgtype.UUID) (string, error) {
		_, err := s.svc.Shipments.Cancel(r.Context(), actor(r), id, r.PostFormValue("reason"), r.PostFormValue("note"))
		return "Shipment cancelled.", err
	})
}

func (s *Server) shipmentReopen(w http.ResponseWriter, r *http.Request) {
	s.shipmentAction(w, r, func(id pgtype.UUID) (string, error) {
		_, err := s.svc.Shipments.Reopen(r.Context(), actor(r), id)
		return "Shipment reopened.", err
	})
}

func (s *Server) shipmentNote(w http.ResponseWriter, r *http.Request) {
	s.shipmentAction(w, r, func(id pgtype.UUID) (string, error) {
		toDriver := r.PostFormValue("to") == "driver"
		bySMS := r.PostFormValue("sms") != ""
		err := s.svc.Shipments.Note(r.Context(), actor(r), id, r.PostFormValue("note"), toDriver, bySMS, "", time.Time{})
		switch {
		case toDriver && bySMS:
			return "Message sent to the driver's app and by SMS.", err
		case toDriver:
			return "Message sent. The driver sees it on their next sync.", err
		}
		return "Note added.", err
	})
}

func (s *Server) shipmentMedia(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	d, err := s.svc.Shipments.Detail(r.Context(), principal(r).TenantID, id)
	if err != nil || d.POD == nil {
		s.notFound(w, r)
		return
	}
	s.serveMedia(w, r, podMediaKey(d.POD.SignatureUrl, d.POD.PhotoUrls, chi.URLParam(r, "n")))
}
