package web

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/geo"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/phone"
	"github.com/natnael-alemayehu/kiflet/internal/service"
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

// listParams are the query parameters that make up a view of the shipment
// list: what a saved view keeps.
var listParams = []string{"tab", "q", "status", "driver", "priority", "origin", "destination", "from", "to", "sort"}

// viewQuery reduces a query string to the list's own parameters, without
// empty ones, the page or the export format.
func viewQuery(q url.Values) string {
	out := url.Values{}
	for _, k := range listParams {
		for _, v := range q[k] {
			if v = strings.TrimSpace(v); v != "" {
				out.Add(k, v)
			}
		}
	}
	return out.Encode()
}

// sortColumn is a sortable header of the shipment list (FR-029 AC-029.2).
type sortColumn struct {
	Key, Label string
	// Href sorts by the column, or reverses the order when it is already
	// sorted by it.
	Href string
	// Dir is "ascending" or "descending" for the column sorted by.
	Dir string
	Num bool
}

func sortColumns(q url.Values) []sortColumn {
	cols := []struct {
		key, label, asc, desc string
		num                   bool
	}{
		{"shipment", "Shipment", "tracking", "tracking_desc", false},
		{"route", "Route", "route", "route_desc", false},
		{"customer", "Customer", "customer", "customer_desc", false},
		{"driver", "Driver", "driver", "driver_desc", false},
		{"status", "Status", "status", "status_desc", false},
		{"due", "Due", "eta_asc", "eta_desc", true},
	}
	cur := q.Get("sort")
	out := make([]sortColumn, 0, len(cols))
	for _, c := range cols {
		sc := sortColumn{Key: c.key, Label: c.label, Num: c.num}
		next := c.asc
		switch cur {
		case c.asc:
			sc.Dir, next = "ascending", c.desc
		case c.desc:
			sc.Dir = "descending"
		}
		v := url.Values{}
		for k, vs := range q {
			if k != "page" && k != "sort" && k != "highlight" {
				v[k] = vs
			}
		}
		v.Set("sort", next)
		sc.Href = "/app/shipments?" + v.Encode()
		out = append(out, sc)
	}
	return out
}

// searchTerm is what the list highlights for a search: a phone number
// typed in any form as it is stored.
func searchTerm(q string) string {
	q = strings.TrimSpace(q)
	if local, err := phone.Normalize(q); err == nil {
		return local
	}
	return q
}

// savedView is a saved filter as the list shows it.
type savedView struct {
	ID, Name, Href string
	Current        bool
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
	saved, err := s.svc.Shipments.SavedFilters(r.Context(), p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	current := viewQuery(q)
	views := make([]savedView, 0, len(saved))
	for _, sv := range saved {
		views = append(views, savedView{ID: sv.ID.String(), Name: sv.Name, Href: "/app/shipments?" + sv.Query, Current: sv.Query == current})
	}
	// Pending rows can be ticked and assigned together (FR-007 AC-007.1).
	var pending bool
	for _, row := range rows {
		pending = pending || row.Shipment.Status == model.StatusPending
	}
	var vehicles []db.Vehicle
	var maxActive int32
	if pending {
		if vehicles, err = s.svc.Vehicles.Active(r.Context(), p.TenantID); err != nil {
			s.fail(w, r, err)
			return
		}
		tenant, err := s.svc.Tenants.Get(r.Context(), p.TenantID)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		maxActive = tenant.MaxActivePerDriver
	}
	page := max(f.Page, 1)
	pages := int((total + int64(f.PerPage) - 1) / int64(f.PerPage))
	filtered := q.Get("q") != "" || q.Get("driver") != "" || q.Get("priority") != "" || q.Get("from") != "" ||
		q.Get("to") != "" || q.Get("origin") != "" || q.Get("destination") != "" || len(q["status"]) > 0
	v := s.view(w, r, "Shipments", "shipments", map[string]any{
		"Rows": rows, "Total": total, "Page": page, "Pages": pages, "Tabs": shipmentTabs, "Tab": q.Get("tab"),
		"Drivers": drivers, "Statuses": model.AllStatuses, "Priorities": model.Priorities, "Filtered": filtered,
		"Towns": geo.Places(), "Highlight": q.Get("highlight"), "Search": searchTerm(q.Get("q")),
		"Columns": sortColumns(q), "Views": views, "ViewQuery": current, "Pending": pending,
		"Vehicles": vehicles, "MaxActive": maxActive, "Back": r.URL.RequestURI(),
	})
	s.render(w, r, http.StatusOK, "app/shipments", v)
}

// shipmentBulkAssign assigns the ticked shipments to one driver and says
// what became of each.
func (s *Server) shipmentBulkAssign(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	var ids []pgtype.UUID
	for _, raw := range r.PostForm["id"] {
		id, err := parseID(raw)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	res, err := s.svc.Shipments.AssignMany(r.Context(), actor(r), ids, r.PostFormValue("driver_id"), r.PostFormValue("vehicle_id"))
	if err != nil {
		s.actionResult(w, r, "/app/shipments?tab=pending", err, "")
		return
	}
	var done, failed []string
	for _, o := range res {
		if o.Err == nil {
			done = append(done, o.TrackingNumber)
		} else {
			name := o.TrackingNumber
			if name == "" {
				name = "One shipment" // not this company's, or gone
			}
			failed = append(failed, name+": "+truncateText(o.Err.Message, 140))
		}
	}
	var msg string
	switch len(done) {
	case 0:
	case 1:
		msg = "Assigned " + done[0] + "."
	default:
		msg = fmt.Sprintf("Assigned %d shipments.", len(done))
	}
	if len(failed) > 0 {
		shown := failed
		if len(shown) > 3 {
			shown = shown[:3]
		}
		more := ""
		if len(failed) > len(shown) {
			more = fmt.Sprintf(" And %d more.", len(failed)-len(shown))
		}
		msg = strings.TrimSpace(msg + " Not assigned: " + strings.Join(shown, " · ") + more)
		s.back(w, r, "/app/shipments?tab=pending", "error", msg)
		return
	}
	s.back(w, r, "/app/shipments?tab=pending", "success", msg)
}

// shipmentViewSave saves the list's current filter under a name.
func (s *Server) shipmentViewSave(w http.ResponseWriter, r *http.Request) {
	query, _ := url.ParseQuery(r.PostFormValue("query"))
	vq := viewQuery(query)
	back := "/app/shipments"
	if vq != "" {
		back += "?" + vq
	}
	_, err := s.svc.Shipments.SaveFilter(r.Context(), principal(r), r.PostFormValue("name"), vq)
	s.actionResult(w, r, back, err, "View saved. It's above the list from now on.")
}

func (s *Server) shipmentViewDelete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	err = s.svc.Shipments.DeleteFilter(r.Context(), principal(r), id)
	s.actionResult(w, r, "/app/shipments", err, "View removed.")
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
