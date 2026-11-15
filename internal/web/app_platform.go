package web

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
)

func (s *Server) platform(w http.ResponseWriter, r *http.Request) {
	tenants, err := s.svc.Tenants.All(r.Context(), principal(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var drivers, shipments int64
	for _, t := range tenants {
		drivers += t.Drivers
		shipments += t.Shipments30d
	}
	v := s.view(w, r, "Companies", "platform", map[string]any{
		"Tenants": tenants, "Plans": model.Plans, "Drivers": drivers, "Shipments": shipments,
	})
	s.render(w, r, http.StatusOK, "app/platform", v)
}

// platformTenant is one company as the platform admin manages it: its plan
// and invoices, questions, and the actions on it (AC-049.3).
func (s *Server) platformTenant(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	p := principal(r)
	tenants, err := s.svc.Tenants.All(r.Context(), p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	i := slices.IndexFunc(tenants, func(t db.ListTenantsWithStatsRow) bool { return t.ID == id })
	if i < 0 || id == p.TenantID {
		s.notFound(w, r)
		return
	}
	invoices, err := s.svc.Billing.CompanyInvoices(r.Context(), p, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	tickets, err := s.svc.Support.ForCompany(r.Context(), p, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	usage, err := s.svc.Tenants.Usage(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(w, r, tenants[i].Name, "platform", map[string]any{
		"T": tenants[i], "Invoices": invoices, "Tickets": tickets, "Usage": usage,
	})
	s.render(w, r, http.StatusOK, "app/platform_tenant", v)
}

func (s *Server) platformTenantActive(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	active, _ := strconv.ParseBool(r.PostFormValue("active"))
	err = s.svc.Tenants.SetActive(r.Context(), principal(r), id, active, clientInfo(r))
	msg := "Company suspended. Its users are signed out."
	if active {
		msg = "Company restored."
	}
	s.actionResult(w, r, "/app/platform", err, msg)
}

// tenantPath is a company's page for the platform admin.
func tenantPath(id pgtype.UUID) string { return "/app/platform/tenants/" + id.String() }
