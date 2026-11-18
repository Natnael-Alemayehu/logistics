package web

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/geo"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// shipmentMapData is what the shipment page's map draws.
type shipmentMapData struct {
	Origin      *mapPoint   `json:"origin"`
	Destination *mapPoint   `json:"destination"`
	Route       [][]float64 `json:"route"`
	Gaps        []routeGap  `json:"gaps"`
	POD         *mapPoint   `json:"pod"`
	Precise     bool        `json:"precise"`
	Tolerance   float64     `json:"tolerance"`
}

// routeGap is where the trail has a hole: the phone sent nothing for a
// while (the app was closed, or no signal) and the truck moved meanwhile.
// The map draws the hole as a dashed line, not as road driven. At is the
// index of the first position after the hole.
type routeGap struct {
	At      int `json:"at"`
	Minutes int `json:"minutes"`
}

// A hole is a silence longer than the slowest the app reports (every 15
// minutes, saving battery) over which the truck moved more than 2 km.
const (
	gapSilence = 20 * time.Minute
	gapMoved   = 2000.0
)

// trail turns a shipment's positions into the map's line and its holes.
func trail(pts []db.ListShipmentRouteRow) ([][]float64, []routeGap) {
	route, gaps := make([][]float64, 0, len(pts)), []routeGap{}
	for i, pt := range pts {
		if i > 0 {
			prev := pts[i-1]
			silence := pt.RecordedAt.Time.Sub(prev.RecordedAt.Time)
			if silence > gapSilence && geo.DistanceMeters(prev.Latitude, prev.Longitude, pt.Latitude, pt.Longitude) > gapMoved {
				gaps = append(gaps, routeGap{At: i, Minutes: int(silence.Minutes())})
			}
		}
		route = append(route, []float64{pt.Latitude, pt.Longitude})
	}
	return route, gaps
}

// shipmentTrail is a shipment's trail for the overview's map, fetched when
// a load is picked on it.
func (s *Server) shipmentTrail(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	pts, err := s.svc.Shipments.Trail(r.Context(), p.TenantID, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	route, gaps := trail(pts)
	writeJSON(w, http.StatusOK, map[string]any{"route": route, "gaps": gaps})
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
	m := shipmentMapData{Precise: sh.DestinationPrecise, Tolerance: geo.DeliveryTolerance(sh.DestinationPrecise)}
	if sh.OriginLat != nil && sh.OriginLng != nil {
		m.Origin = &mapPoint{*sh.OriginLat, *sh.OriginLng, sh.OriginAddress}
	}
	if sh.DestinationLat != nil && sh.DestinationLng != nil {
		m.Destination = &mapPoint{*sh.DestinationLat, *sh.DestinationLng, sh.DestinationAddress}
	}
	m.Route, m.Gaps = trail(d.Route)
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
	review, err := s.svc.Shipments.ReviewPODs(r.Context(), p.TenantID, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	tenant, err := s.svc.Tenants.Get(r.Context(), p.TenantID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	tab := "overview"
	switch t := r.URL.Query().Get("tab"); t {
	case "activity", "documents", "charges":
		tab = t
	}
	documents, err := s.svc.Shipments.Documents(r.Context(), p.TenantID, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	charges, chargeTotal, err := s.svc.Shipments.Charges(r.Context(), p.TenantID, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(w, r, sh.TrackingNumber, "shipments", map[string]any{
		"D": d, "S": sh, "Timeline": timeline, "Trip": tripFrom(d), "Tab": tab,
		"Documents": documents, "Charges": charges, "ChargeTotal": chargeTotal,
		"DocumentKinds": service.DocumentKinds, "MaxDocumentMB": service.MaxDocumentBytes >> 20, "Map": m, "Drivers": drivers, "Vehicles": vehicles, "Review": review,
		"CanAssign": model.CanAssign(sh.Status), "CanUnassign": sh.DriverID.Valid && model.CanUnassign(sh.Status),
		"CanCancel": model.CanCancel(sh.Status), "CanReopen": model.CanReopen(sh.Status),
		"CanEdit": !model.IsFinalStatus(sh.Status), "Next": d.Next,
		"CancelReasons": model.CancelReasonsFor(sh.Status), "DelayReasons": model.DelayReasons, "IssueReasons": model.IssueReasons,
		"TrackURL": s.cfg.PublicURL + "/track/" + sh.TrackingNumber, "MaxActive": tenant.MaxActivePerDriver,
		"Quick": service.QuickMessages,
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

// shipmentLocationsCSV exports every GPS fix of a shipment's trip (FR-030
// AC-030.7).
func (s *Server) shipmentLocationsCSV(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	sh, track, err := s.svc.Shipments.Positions(r.Context(), p.TenantID, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-locations.csv"`, sh.TrackingNumber))
	_, _ = w.Write([]byte("\xEF\xBB\xBF"))
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"Recorded", "Latitude", "Longitude", "Accuracy (m)", "Speed (km/h)", "Battery (%)", "Kind"})
	for _, pt := range track {
		battery := ""
		if pt.BatteryLevel != nil {
			battery = strconv.Itoa(int(*pt.BatteryLevel))
		}
		_ = cw.Write([]string{
			pt.RecordedAt.Time.In(s.cfg.Timezone).Format("2006-01-02 15:04:05"),
			strconv.FormatFloat(pt.Latitude, 'f', 6, 64), strconv.FormatFloat(pt.Longitude, 'f', 6, 64),
			numStr(toFloat(pt.AccuracyMeters)), numStr(toFloat(pt.SpeedKph)), battery, pt.EventType,
		})
	}
	cw.Flush()
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
		sent, err := s.svc.Shipments.Note(r.Context(), actor(r), id, r.PostFormValue("note"), toDriver, bySMS, "", time.Time{})
		switch {
		case toDriver && sent && !bySMS:
			return "Message sent. The driver's app hasn't synced in the last 15 minutes, so it went by SMS as well.", err
		case toDriver && sent:
			return "Message sent to the driver's app and by SMS.", err
		case toDriver && bySMS:
			return "Message sent to the driver's app. The driver has no phone number for SMS.", err
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

// shipmentAcceptPOD completes a shipment with a proof of delivery held for
// review.
func (s *Server) shipmentAcceptPOD(w http.ResponseWriter, r *http.Request) {
	pod, err := pathID(r, "pod")
	if err != nil {
		s.notFound(w, r)
		return
	}
	s.shipmentAction(w, r, func(id pgtype.UUID) (string, error) {
		_, err := s.svc.Shipments.AcceptHeldPOD(r.Context(), actor(r), id, pod)
		return "Delivery accepted. The shipment is delivered.", err
	})
}

func (s *Server) shipmentDismissPOD(w http.ResponseWriter, r *http.Request) {
	pod, err := pathID(r, "pod")
	if err != nil {
		s.notFound(w, r)
		return
	}
	s.shipmentAction(w, r, func(id pgtype.UUID) (string, error) {
		return "Delivery record dismissed. It stays on file.", s.svc.Shipments.DismissHeldPOD(r.Context(), actor(r), id, pod)
	})
}

// shipmentReviewMedia serves the images of a proof under review.
func (s *Server) shipmentReviewMedia(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	pod, err := pathID(r, "pod")
	if err != nil {
		s.notFound(w, r)
		return
	}
	sig, photos, err := s.svc.Shipments.HeldPODMedia(r.Context(), principal(r).TenantID, id, pod)
	if err != nil {
		s.notFound(w, r)
		return
	}
	s.serveMedia(w, r, podMediaKey(sig, photos, chi.URLParam(r, "n")))
}

// tripStep is one stage of a shipment in the progress strip at the top of
// its page.
type tripStep struct {
	Label   string
	At      time.Time
	Done    bool
	Current bool
}

// tripView sums up where a load is: the stages it has passed, how far it has
// left and what the driver's phone last said.
type tripView struct {
	Steps    []tripStep
	BookedBy string
	Progress *int
	// KmTotal is the straight-line distance from origin to destination,
	// KmLeft from the last position (or the origin, before the trip) to it.
	KmTotal, KmLeft float64
	HasKm           bool
	SpeedKPH        float64
	HasSpeed        bool
	Battery         *int32
	LastAt          time.Time
	// Active: a driver has the load and it isn't delivered, so where their
	// phone was last heard from means something. Fresh: that was in the
	// last half hour, recent enough to show the speed.
	Active, Fresh bool
	Near          string
	NearKm        float64
}

var tripStages = []struct{ status, label string }{
	{model.StatusPending, "Booked"},
	{model.StatusAssigned, "Driver assigned"},
	{model.StatusInTransit, "On the way"},
	{model.StatusArrived, "Arrived"},
	{model.StatusDelivered, "Delivered"},
}

// tripStage is how far along the stages a status is; -1 for cancelled.
func tripStage(status string) int {
	switch status {
	case model.StatusPending:
		return 0
	case model.StatusAssigned:
		return 1
	case model.StatusInTransit, model.StatusDelayed, model.StatusIssue:
		return 2
	case model.StatusArrived:
		return 3
	case model.StatusDelivered:
		return 4
	}
	return -1
}

func tripFrom(d *service.ShipmentDetail) tripView {
	sh := d.Shipment
	var t tripView
	reached := map[string]time.Time{model.StatusPending: sh.CreatedAt.Time}
	for i, e := range d.Events { // oldest first
		if i == 0 {
			t.BookedBy = e.ActorName
		}
		if e.ToStatus == nil {
			continue
		}
		to := *e.ToStatus
		if to == model.StatusDelayed || to == model.StatusIssue {
			to = model.StatusInTransit
		}
		if _, ok := reached[to]; !ok {
			reached[to] = e.CreatedAt.Time
		}
	}
	if sh.PickedUpAt.Valid {
		reached[model.StatusInTransit] = sh.PickedUpAt.Time
	}
	if sh.ActualDelivery.Valid {
		reached[model.StatusDelivered] = sh.ActualDelivery.Time
	}
	stage := tripStage(sh.Status)
	for i, st := range tripStages {
		at, ok := reached[st.status]
		t.Steps = append(t.Steps, tripStep{Label: st.label, At: at, Done: ok && (stage < 0 || i <= stage), Current: i == stage && i < len(tripStages)-1})
	}

	var pos *service.DriverPin
	if n := len(d.Route); n > 0 {
		last := d.Route[n-1]
		pos = &service.DriverPin{Lat: last.Latitude, Lng: last.Longitude, At: last.RecordedAt.Time}
		t.LastAt = last.RecordedAt.Time
		t.Battery = last.BatteryLevel
		if f, err := last.SpeedKph.Float64Value(); err == nil && f.Valid {
			t.SpeedKPH, t.HasSpeed = f.Float64, true
		}
		place, m := geo.Nearest(last.Latitude, last.Longitude)
		t.Near, t.NearKm = place.Name, m/1000
	}
	t.Active = stage >= 1 && stage <= 3
	t.Fresh = t.Active && !t.LastAt.IsZero() && time.Since(t.LastAt) < 30*time.Minute
	t.Progress = service.Progress(sh, pos)
	if sh.OriginLat != nil && sh.OriginLng != nil && sh.DestinationLat != nil && sh.DestinationLng != nil {
		t.KmTotal = geo.DistanceMeters(*sh.OriginLat, *sh.OriginLng, *sh.DestinationLat, *sh.DestinationLng) / 1000
		t.KmLeft, t.HasKm = t.KmTotal, t.KmTotal >= 1
		switch {
		case stage >= 3:
			t.KmLeft = 0
		case stage == 2 && pos != nil:
			t.KmLeft = min(t.KmTotal, geo.DistanceMeters(pos.Lat, pos.Lng, *sh.DestinationLat, *sh.DestinationLng)/1000)
		}
	}
	return t
}
