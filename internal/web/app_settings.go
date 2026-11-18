package web

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/geo"
	"github.com/natnael-alemayehu/kiflet/internal/service"
	"github.com/natnael-alemayehu/kiflet/internal/text"
)

func (s *Server) settingsData(r *http.Request) (map[string]any, error) {
	p := principal(r)
	tenant, err := s.svc.Tenants.Get(r.Context(), p.TenantID)
	if err != nil {
		return nil, err
	}
	usage, err := s.svc.Tenants.Usage(r.Context(), p.TenantID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"Tenant": tenant, "Usage": usage, "Towns": geo.Places(),
		"RetainDays": int(s.svc.Data.RetainShipments().Hours() / 24),
		"Languages":  text.Languages,
		"Previews":   s.svc.Tenants.SMSPreviews(tenant),
	}, nil
}

func (s *Server) settingsPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.settingsData(r)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	tenant := data["Tenant"].(db.Tenant)
	v := s.view(w, r, "Settings", "settings", data)
	v.Form = url.Values{
		"name":                  {tenant.Name},
		"contact_phone":         {deref(tenant.ContactPhone)},
		"contact_email":         {deref(tenant.ContactEmail)},
		"region_city":           {tenant.RegionCity},
		"max_active_per_driver": {strconv.Itoa(int(tenant.MaxActivePerDriver))},
		"tin":                   {deref(tenant.Tin)},
		"customer_language":     {tenant.CustomerLanguage},
	}
	for name, on := range map[string]bool{
		"sms_on_created": tenant.SmsOnCreated, "sms_on_dispatched": tenant.SmsOnDispatched,
		"sms_on_arrived": tenant.SmsOnArrived, "sms_on_delivered": tenant.SmsOnDelivered,
		"sms_on_cancelled": tenant.SmsOnCancelled, "sms_on_delayed": tenant.SmsOnDelayed,
		"sms_driver_on_assign": tenant.SmsDriverOnAssign, "alert_on_delivery": tenant.AlertOnDelivery,
	} {
		if on {
			v.Form.Set(name, "on")
		}
	}
	s.render(w, r, http.StatusOK, "app/settings", v)
}

func (s *Server) settingsUpdate(w http.ResponseWriter, r *http.Request) {
	maxActive, _ := strconv.Atoi(r.PostFormValue("max_active_per_driver"))
	on := func(name string) bool { return r.PostFormValue(name) != "" }
	_, err := s.svc.Tenants.UpdateSettings(r.Context(), principal(r), service.SettingsInput{
		Name: r.PostFormValue("name"), TIN: r.PostFormValue("tin"), ContactPhone: r.PostFormValue("contact_phone"),
		ContactEmail: r.PostFormValue("contact_email"), RegionCity: r.PostFormValue("region_city"),
		MaxActivePerDriver: maxActive,
		SMSOnCreated:       on("sms_on_created"), SMSOnDispatched: on("sms_on_dispatched"),
		SMSOnArrived: on("sms_on_arrived"), SMSOnDelivered: on("sms_on_delivered"),
		SMSOnCancelled: on("sms_on_cancelled"), SMSOnDelayed: on("sms_on_delayed"),
		SMSDriverOnAssign: on("sms_driver_on_assign"), AlertOnDelivery: on("alert_on_delivery"),
		CustomerLanguage: r.PostFormValue("customer_language"),
	}, clientInfo(r))
	if err != nil {
		data, derr := s.settingsData(r)
		if derr != nil {
			s.fail(w, r, derr)
			return
		}
		if !s.formError(w, r, "app/settings", s.view(w, r, "Settings", "settings", data), err) {
			s.fail(w, r, err)
		}
		return
	}
	s.setFlash(w, "success", "Settings saved.")
	http.Redirect(w, r, "/app/settings", http.StatusSeeOther)
}

// smsLog lists the company's text messages (FR-030).
func (s *Server) smsLog(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	page = max(page, 1)
	const per = 50
	rows, total, err := s.svc.SMS.Log(r.Context(), p.TenantID, page, per)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var drivers any
	if s.mockSMS {
		if drivers, err = s.svc.Users.Drivers(r.Context(), p.TenantID, true, ""); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	v := s.view(w, r, "Text messages", "settings", map[string]any{
		"Messages": rows, "Total": total, "Page": page, "Pages": int((total + per - 1) / per),
		"Provider": s.svc.SMS.ProviderName(), "Drivers": drivers,
	})
	s.render(w, r, http.StatusOK, "app/sms", v)
}

// smsSimulate plays an inbound driver SMS through the real handler. It only
// exists with the mock provider, so a demo or test can exercise SMS reporting
// without a phone network.
func (s *Server) smsSimulate(w http.ResponseWriter, r *http.Request) {
	if !s.mockSMS {
		s.notFound(w, r)
		return
	}
	res, err := s.svc.SMS.Inbound(r.Context(), service.InboundSMS{From: r.PostFormValue("from"), Body: r.PostFormValue("body")})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	kind, msg := "success", res.Detail
	switch res.Outcome {
	case "bad_number", "unknown_sender", "refused":
		kind = "error"
	}
	if res.Reply != "" {
		msg += " Reply: “" + res.Reply + "”"
	}
	s.back(w, r, "/app/settings/sms", kind, msg)
}
