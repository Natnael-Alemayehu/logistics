package web

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/geo"
	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// paperView is a licence, insurance or inspection date as the lists show
// it: the month while it is far off, the day once it is close.
type paperView struct {
	State string // service.PaperValid, PaperDue, PaperLapsed, or empty
	Text  string
	Date  pgtype.Date
}

func (s *Server) paper(d pgtype.Date) paperView {
	st := service.PaperState(d, time.Now().In(s.cfg.Timezone))
	switch st {
	case service.PaperNone:
		return paperView{}
	case service.PaperValid:
		return paperView{State: st, Text: d.Time.Format("Jan 2006"), Date: d}
	}
	return paperView{State: st, Text: d.Time.Format("2 Jan 2006"), Date: d}
}

// urgent is whether papers want attention.
func (p paperView) urgent() bool {
	return p.State == service.PaperDue || p.State == service.PaperLapsed
}

// Driver groups, the drivers page's tabs.
const (
	driverOnTrip    = "trip"
	driverAvailable = "available"
	driverOffDuty   = "off"
	driverInactive  = "inactive"
)

type driverRow struct {
	db.ListDriversWithStatusRow
	Group   string
	Near    string
	Licence paperView
}

func (s *Server) driverRows(rows []db.ListDriversWithStatusRow) []driverRow {
	out := make([]driverRow, len(rows))
	for i, d := range rows {
		r := driverRow{ListDriversWithStatusRow: d, Licence: s.paper(d.LicenceExpiresOn)}
		switch {
		case !d.IsActive:
			r.Group = driverInactive
		case d.ActiveShipments > 0:
			r.Group = driverOnTrip
		case d.OnDuty:
			r.Group = driverAvailable
		default:
			r.Group = driverOffDuty
		}
		if d.PositionAt.Valid && time.Since(d.PositionAt.Time) < 7*24*time.Hour {
			place, m := geo.Nearest(d.PositionLat, d.PositionLng)
			if m < 2000 {
				r.Near = place.Name
			} else {
				r.Near = "Near " + place.Name
			}
		}
		out[i] = r
	}
	return out
}

func (s *Server) driverList(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	q := r.URL.Query()
	tab := q.Get("tab")
	if q.Get("inactive") != "" {
		tab = driverInactive
	}
	all, err := s.svc.Users.Drivers(r.Context(), p.TenantID, false, strings.TrimSpace(q.Get("q")))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	usage, err := s.svc.Tenants.Usage(r.Context(), p.TenantID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	rows := s.driverRows(all)
	counts := map[string]int{}
	var shown, due []driverRow
	for _, d := range rows {
		counts[d.Group]++
		if d.Group != driverInactive {
			counts["all"]++
			if d.Licence.urgent() {
				due = append(due, d)
			}
		}
		if (tab == "" && d.Group != driverInactive) || d.Group == tab {
			shown = append(shown, d)
		}
	}
	sort.SliceStable(due, func(i, j int) bool { return due[i].LicenceExpiresOn.Time.Before(due[j].LicenceExpiresOn.Time) })
	v := s.view(w, r, "Drivers", "drivers", map[string]any{
		"Drivers": shown, "Tab": tab, "Counts": counts, "Usage": usage, "Due": due,
		"Search": strings.TrimSpace(q.Get("q")),
	})
	s.render(w, r, http.StatusOK, "app/drivers", v)
}

// driverRemind texts a driver to renew their licence.
func (s *Server) driverRemind(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	back := "/app/drivers"
	if strings.HasPrefix(r.PostFormValue("back"), "/app/drivers/") {
		back = r.PostFormValue("back")
	}
	err = s.svc.Users.RemindLicence(r.Context(), principal(r), id, clientInfo(r))
	s.actionResult(w, r, back, err, "Reminder sent by SMS.")
}

// Vehicle groups, the vehicles page's tabs.
const (
	vehicleOnRoad    = "road"
	vehicleBooked    = "booked"
	vehicleAvailable = "available"
	vehicleOut       = "out"
)

type vehicleRow struct {
	db.ListVehiclesWithDriversRow
	Group      string
	Insurance  paperView
	Inspection paperView
}

// vehiclePaper is one of a vehicle's papers that wants attention.
type vehiclePaper struct {
	Row   vehicleRow
	What  string
	Paper paperView
}

func (s *Server) vehicleList(w http.ResponseWriter, r *http.Request) {
	all, err := s.svc.Vehicles.List(r.Context(), principal(r).TenantID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	tab := r.URL.Query().Get("tab")
	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	counts := map[string]int{}
	var shown []vehicleRow
	var due []vehiclePaper
	var capacity float64
	for _, vh := range all {
		row := vehicleRow{ListVehiclesWithDriversRow: vh, Insurance: s.paper(vh.InsuranceExpiresOn), Inspection: s.paper(vh.InspectionDueOn)}
		switch {
		case !vh.IsActive:
			row.Group = vehicleOut
		case vh.OnRoad:
			row.Group = vehicleOnRoad
		case vh.ActiveShipments > 0:
			row.Group = vehicleBooked
		default:
			row.Group = vehicleAvailable
		}
		counts[row.Group]++
		counts["all"]++
		if vh.IsActive {
			capacity += toFloat(vh.CapacityKg)
			papersDue := false
			for _, pp := range []vehiclePaper{{row, "Insurance", row.Insurance}, {row, "Annual inspection (ቦሎ)", row.Inspection}} {
				if pp.Paper.urgent() {
					due, papersDue = append(due, pp), true
				}
			}
			if papersDue {
				counts["papers"]++
			}
		}
		if search != "" && !strings.Contains(strings.ToLower(vh.PlateNumber+" "+vh.DriverNames), search) {
			continue
		}
		if tab == "" || tab == row.Group || (tab == "papers" && (row.Insurance.urgent() || row.Inspection.urgent())) {
			shown = append(shown, row)
		}
	}
	sort.SliceStable(due, func(i, j int) bool { return due[i].Paper.Date.Time.Before(due[j].Paper.Date.Time) })
	s.render(w, r, http.StatusOK, "app/vehicles", s.view(w, r, "Vehicles", "vehicles", map[string]any{
		"Vehicles": shown, "Tab": tab, "Counts": counts, "Due": due, "CapacityKg": capacity,
		"Search": strings.TrimSpace(r.URL.Query().Get("q")),
	}))
}
