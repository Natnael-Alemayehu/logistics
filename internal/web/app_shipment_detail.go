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
	v := s.view(w, r, sh.TrackingNumber, "shipments", map[string]any{
		"D": d, "S": sh, "Timeline": timeline, "Map": m, "Drivers": drivers, "Vehicles": vehicles, "Review": review,
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
