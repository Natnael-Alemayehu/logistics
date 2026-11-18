package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/service"
)

func personInput(r *http.Request) service.PersonInput {
	return service.PersonInput{
		FullName:  r.PostFormValue("full_name"),
		Phone:     r.PostFormValue("phone"),
		Email:     r.PostFormValue("email"),
		Role:      r.PostFormValue("role"),
		VehicleID: r.PostFormValue("vehicle_id"),

		LicenceExpires: r.PostFormValue("licence_expires_on"),
	}
}

// personPath is where a person's page lives: drivers have their own pages,
// staff are managed from the team list.
func personPath(u db.User) string {
	if u.Role == model.RoleDriver {
		return "/app/drivers/" + u.ID.String()
	}
	return "/app/team"
}

// ---------------------------------------------------------------------------
// Drivers (FR-004)
// ---------------------------------------------------------------------------

func (s *Server) driverDetail(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	d, err := s.svc.Users.Driver(r.Context(), p.TenantID, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var last *mapPoint
	if d.Location != nil {
		last = &mapPoint{d.Location.Latitude, d.Location.Longitude, d.Driver.FullName}
	}
	v := s.view(w, r, d.Driver.FullName, "drivers", map[string]any{
		"D": d, "Last": last, "OnTime": service.OnTimeRate(d.Performance.OnTime, d.Performance.WithEta),
	})
	s.render(w, r, http.StatusOK, "app/driver", v)
}

func (s *Server) driverFormData(r *http.Request, u *db.User) (map[string]any, error) {
	vehicles, err := s.svc.Vehicles.Active(r.Context(), principal(r).TenantID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"Vehicles": vehicles, "Person": u}, nil
}

func (s *Server) driverNew(w http.ResponseWriter, r *http.Request) {
	data, err := s.driverFormData(r, nil)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "app/driver_form", s.view(w, r, "Add a driver", "drivers", data))
}

func (s *Server) driverCreate(w http.ResponseWriter, r *http.Request) {
	acct, err := s.svc.Users.CreateDriver(r.Context(), principal(r), personInput(r), clientInfo(r))
	if err != nil {
		data, derr := s.driverFormData(r, nil)
		if derr != nil {
			s.fail(w, r, derr)
			return
		}
		if !s.formError(w, r, "app/driver_form", s.view(w, r, "Add a driver", "drivers", data), err) {
			s.fail(w, r, err)
		}
		return
	}
	s.showCredential(w, r, acct.User, acct.Credential, acct.Texted, true)
}

func (s *Server) driverEdit(w http.ResponseWriter, r *http.Request) {
	s.personEdit(w, r, true)
}

func (s *Server) driverUpdate(w http.ResponseWriter, r *http.Request) {
	s.personUpdate(w, r, true)
}

// showCredential is the one time a PIN or password is visible. The page is
// rendered rather than redirected to so the secret never sits in a cookie or
// URL, and it is marked no-store like every page.
func (s *Server) showCredential(w http.ResponseWriter, r *http.Request, u db.User, credential string, texted, created bool) {
	v := s.view(w, r, u.FullName, navFor(u), map[string]any{
		"Person": u, "Credential": credential, "Texted": texted, "Created": created,
		"IsDriver": u.Role == model.RoleDriver, "Next": personPath(u),
		"AppURL": s.cfg.PublicURL + "/driver/",
	})
	s.render(w, r, http.StatusOK, "app/credential", v)
}

func navFor(u db.User) string {
	if u.Role == model.RoleDriver {
		return "drivers"
	}
	return "team"
}

// ---------------------------------------------------------------------------
// Office staff
// ---------------------------------------------------------------------------

// teamList is the office staff in three groups: members, those who were
// invited and haven't signed in yet, and those who left.
func (s *Server) teamList(w http.ResponseWriter, r *http.Request) {
	staff, err := s.svc.Users.Staff(r.Context(), principal(r).TenantID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	var members, waiting, former []db.User
	for _, u := range staff {
		if u.Role == model.RoleDriver {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(u.FullName+" "+deref(u.Email)+" "+deref(u.Phone)), q) {
			continue
		}
		switch {
		case !u.IsActive || u.AnonymisedAt.Valid:
			former = append(former, u)
		case u.PasswordResetRequired && !u.LastLoginAt.Valid:
			waiting = append(waiting, u)
		default:
			members = append(members, u)
		}
	}
	v := s.view(w, r, "Team", "team", map[string]any{
		"Members": members, "Waiting": waiting, "Former": former, "Search": r.URL.Query().Get("q"),
		"Roles": model.StaffRoles,
	})
	v.Form.Set("role", model.RoleDispatcher)
	s.render(w, r, http.StatusOK, "app/team", v)
}

func (s *Server) teamNew(w http.ResponseWriter, r *http.Request) {
	v := s.view(w, r, "Invite a team member", "team", map[string]any{"Roles": model.StaffRoles})
	v.Form.Set("role", model.RoleDispatcher)
	s.render(w, r, http.StatusOK, "app/team_form", v)
}

func (s *Server) teamCreate(w http.ResponseWriter, r *http.Request) {
	acct, err := s.svc.Users.CreateStaff(r.Context(), principal(r), personInput(r), clientInfo(r))
	if err != nil {
		v := s.view(w, r, "Invite a team member", "team", map[string]any{"Roles": model.StaffRoles})
		if !s.formError(w, r, "app/team_form", v, err) {
			s.fail(w, r, err)
		}
		return
	}
	s.showCredential(w, r, acct.User, acct.Credential, acct.Texted, true)
}

func (s *Server) teamEdit(w http.ResponseWriter, r *http.Request) {
	s.personEdit(w, r, false)
}

func (s *Server) teamUpdate(w http.ResponseWriter, r *http.Request) {
	s.personUpdate(w, r, false)
}

func personFormValues(u db.User) url.Values {
	return url.Values{
		"full_name":  {u.FullName},
		"phone":      {deref(u.Phone)},
		"email":      {deref(u.Email)},
		"role":       {u.Role},
		"vehicle_id": {uuidOrEmpty(u.VehicleID)},

		"licence_expires_on": {formDay(u.LicenceExpiresOn)},
	}
}

// formDay is a date as a date input holds it.
func formDay(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("2006-01-02")
}

func uuidOrEmpty(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return u.String()
}

// loadPerson fetches the person in the URL, checking it is the right kind
// for the page (a driver under /drivers, staff under /team).
func (s *Server) loadPerson(w http.ResponseWriter, r *http.Request, driver bool) (db.User, bool) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return db.User{}, false
	}
	u, err := s.svc.Users.Get(r.Context(), principal(r).TenantID, id)
	if err != nil {
		s.fail(w, r, err)
		return db.User{}, false
	}
	if (u.Role == model.RoleDriver) != driver {
		s.notFound(w, r)
		return db.User{}, false
	}
	return u, true
}

func (s *Server) personEdit(w http.ResponseWriter, r *http.Request, driver bool) {
	u, ok := s.loadPerson(w, r, driver)
	if !ok {
		return
	}
	page, data := "app/team_form", map[string]any{"Roles": model.StaffRoles, "Person": &u}
	if driver {
		var err error
		if data, err = s.driverFormData(r, &u); err != nil {
			s.fail(w, r, err)
			return
		}
		page = "app/driver_form"
	}
	v := s.view(w, r, "Edit "+u.FullName, navFor(u), data)
	v.Form = personFormValues(u)
	s.render(w, r, http.StatusOK, page, v)
}

func (s *Server) personUpdate(w http.ResponseWriter, r *http.Request, driver bool) {
	u, ok := s.loadPerson(w, r, driver)
	if !ok {
		return
	}
	p := principal(r)
	var err error
	if driver {
		_, err = s.svc.Users.UpdateDriver(r.Context(), p, u.ID, personInput(r), clientInfo(r))
	} else {
		_, err = s.svc.Users.UpdateStaff(r.Context(), p, u.ID, personInput(r), clientInfo(r))
	}
	if err != nil {
		page, data := "app/team_form", map[string]any{"Roles": model.StaffRoles, "Person": &u}
		if driver {
			var derr error
			if data, derr = s.driverFormData(r, &u); derr != nil {
				s.fail(w, r, derr)
				return
			}
			page = "app/driver_form"
		}
		if !s.formError(w, r, page, s.view(w, r, "Edit "+u.FullName, navFor(u), data), err) {
			s.fail(w, r, err)
		}
		return
	}
	s.setFlash(w, "success", "Changes saved.")
	http.Redirect(w, r, personPath(u), http.StatusSeeOther)
}

// personActive deactivates or reactivates a driver or staff member.
func (s *Server) personActive(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	p := principal(r)
	u, err := s.svc.Users.Get(r.Context(), p.TenantID, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	active := r.PostFormValue("active") == "true"
	err = s.svc.Users.SetActive(r.Context(), p, id, active, clientInfo(r))
	msg := u.FullName + " can no longer sign in."
	if active {
		msg = u.FullName + " can sign in again."
	}
	s.actionResult(w, r, personPath(u), err, msg)
}

// personReset issues a temporary PIN or password.
func (s *Server) personReset(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	p := principal(r)
	u, err := s.svc.Users.Get(r.Context(), p.TenantID, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	temp, texted, err := s.svc.Auth.ResetCredential(r.Context(), p, id, clientInfo(r))
	if err != nil {
		s.actionResult(w, r, personPath(u), err, "")
		return
	}
	s.showCredential(w, r, u, temp, texted, false)
}

// ---------------------------------------------------------------------------
// Vehicles (FR-005)
// ---------------------------------------------------------------------------

func (s *Server) vehicleDetail(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	vh, err := s.svc.Vehicles.Get(r.Context(), p.TenantID, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	recent, err := s.svc.Vehicles.Recent(r.Context(), p.TenantID, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	all, err := s.svc.Users.Drivers(r.Context(), p.TenantID, false, "")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var drivers []db.ListDriversWithStatusRow
	for _, d := range all {
		if d.VehicleID == id {
			drivers = append(drivers, d)
		}
	}
	v := s.view(w, r, vh.PlateNumber, "vehicles", map[string]any{"V": vh, "Recent": recent, "Drivers": drivers})
	s.render(w, r, http.StatusOK, "app/vehicle", v)
}

func vehicleInput(r *http.Request) service.VehicleInput {
	capacity, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(r.PostFormValue("capacity_kg")), ",", ""), 64)
	return service.VehicleInput{
		PlateNumber: r.PostFormValue("plate_number"),
		VehicleType: r.PostFormValue("vehicle_type"),
		CapacityKg:  capacity,
		Active:      r.PostFormValue("active") != "",

		InsuranceExpires: r.PostFormValue("insurance_expires_on"),
		InspectionDue:    r.PostFormValue("inspection_due_on"),
	}
}

func (s *Server) vehicleNew(w http.ResponseWriter, r *http.Request) {
	v := s.view(w, r, "Add a vehicle", "vehicles", map[string]any{"Types": service.VehicleTypes})
	v.Form.Set("active", "on")
	s.render(w, r, http.StatusOK, "app/vehicle_form", v)
}

func (s *Server) vehicleCreate(w http.ResponseWriter, r *http.Request) {
	vh, err := s.svc.Vehicles.Create(r.Context(), principal(r), vehicleInput(r), clientInfo(r))
	if err != nil {
		v := s.view(w, r, "Add a vehicle", "vehicles", map[string]any{"Types": service.VehicleTypes})
		if !s.formError(w, r, "app/vehicle_form", v, err) {
			s.fail(w, r, err)
		}
		return
	}
	s.setFlash(w, "success", "Vehicle "+vh.PlateNumber+" added.")
	http.Redirect(w, r, "/app/vehicles", http.StatusSeeOther)
}

func (s *Server) vehicleEdit(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	vh, err := s.svc.Vehicles.Get(r.Context(), principal(r).TenantID, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(w, r, "Edit "+vh.PlateNumber, "vehicles", map[string]any{"Types": service.VehicleTypes, "V": &vh})
	v.Form = url.Values{
		"plate_number": {vh.PlateNumber},
		"vehicle_type": {deref(vh.VehicleType)},
		"capacity_kg":  {numStr(toFloat(vh.CapacityKg))},

		"insurance_expires_on": {formDay(vh.InsuranceExpiresOn)},
		"inspection_due_on":    {formDay(vh.InspectionDueOn)},
	}
	if vh.IsActive {
		v.Form.Set("active", "on")
	}
	s.render(w, r, http.StatusOK, "app/vehicle_form", v)
}

func (s *Server) vehicleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	p := principal(r)
	vh, err := s.svc.Vehicles.Update(r.Context(), p, id, vehicleInput(r), clientInfo(r))
	if err != nil {
		cur, gerr := s.svc.Vehicles.Get(r.Context(), p.TenantID, id)
		if gerr != nil {
			s.fail(w, r, gerr)
			return
		}
		v := s.view(w, r, "Edit "+cur.PlateNumber, "vehicles", map[string]any{"Types": service.VehicleTypes, "V": &cur})
		if !s.formError(w, r, "app/vehicle_form", v, err) {
			s.fail(w, r, err)
		}
		return
	}
	s.setFlash(w, "success", "Changes saved.")
	http.Redirect(w, r, "/app/vehicles/"+vh.ID.String(), http.StatusSeeOther)
}

// broadcastPage writes to every driver with a shipment assigned or on the
// road (FR-035 AC-035.4).
func (s *Server) broadcastPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.broadcastData(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "app/broadcast", s.view(w, r, "Message drivers", "drivers", data))
}

func (s *Server) broadcastData(r *http.Request) (map[string]any, error) {
	p := principal(r)
	drivers, err := s.svc.Shipments.DriversOnTheRoad(r.Context(), p.TenantID)
	if err != nil {
		return nil, err
	}
	sent, err := s.svc.Shipments.Broadcasts(r.Context(), p.TenantID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"Drivers": drivers, "Sent": sent, "Quick": service.QuickMessages}, nil
}

func (s *Server) broadcastSend(w http.ResponseWriter, r *http.Request) {
	res, err := s.svc.Shipments.Broadcast(r.Context(), actor(r), r.PostFormValue("body"))
	if err != nil {
		data, derr := s.broadcastData(r)
		if derr != nil {
			s.fail(w, r, derr)
			return
		}
		if !s.formError(w, r, "app/broadcast", s.view(w, r, "Message drivers", "drivers", data), err) {
			s.fail(w, r, err)
		}
		return
	}
	msg := "Sent to 1 driver."
	if res.Drivers != 1 {
		msg = fmt.Sprintf("Sent to %d drivers.", res.Drivers)
	}
	switch {
	case res.BySMS == 0:
	case res.Drivers == 1:
		msg += " Their app hasn't synced lately, so it went by SMS as well."
	case res.BySMS == res.Drivers:
		msg += " None of them has opened the app lately, so it went to all of them by SMS as well."
	case res.BySMS == 1:
		msg += " One of them hasn't opened the app lately, so it went to them by SMS as well."
	default:
		msg += fmt.Sprintf(" %d of them haven't opened the app lately, so it went to them by SMS as well.", res.BySMS)
	}
	s.setFlash(w, "success", msg)
	http.Redirect(w, r, "/app/drivers/message", http.StatusSeeOther)
}
