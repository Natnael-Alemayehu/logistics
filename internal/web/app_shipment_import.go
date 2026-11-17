package web

import (
	"io"
	"net/http"

	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// Booking shipments from a CSV file (FR-013 AC-013.1): the file is chosen,
// checked and shown, and booked only once every row is right and the
// dispatcher confirms.

func (s *Server) shipmentImportPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "app/shipment_import", s.importView(w, r, nil, ""))
}

func (s *Server) importView(w http.ResponseWriter, r *http.Request, im *service.Import, file string) *View {
	texts := false
	if t, err := s.svc.Tenants.Get(r.Context(), principal(r).TenantID); err == nil {
		texts = t.SmsOnCreated
	}
	return s.view(w, r, "Import shipments", "shipments", map[string]any{
		"Columns": service.ImportColumns, "Max": service.MaxImportRows, "Import": im, "File": file, "Texts": texts,
	})
}

func (s *Server) shipmentImportTemplate(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="shipments-import.csv"`)
	_, _ = w.Write(service.ImportTemplate())
}

// shipmentImport checks an uploaded file and shows what it would book.
// The page it renders carries the file back, so that confirming books
// exactly what was shown.
func (s *Server) shipmentImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, service.MaxImportBytes+64<<10)
	var file []byte
	if r.PostFormValue("confirm") != "" {
		file = []byte(r.PostFormValue("csv"))
	} else if f, _, err := r.FormFile("file"); err == nil {
		file, _ = io.ReadAll(f)
		_ = f.Close()
	}
	fail := func(im *service.Import, err error) {
		v := s.importView(w, r, im, string(file))
		if !s.formError(w, r, "app/shipment_import", v, err) {
			s.fail(w, r, err)
		}
	}
	if len(file) == 0 {
		fail(nil, &service.Error{Kind: service.KindInvalid, Field: "file", Message: "Choose a CSV file."})
		return
	}
	im, err := s.svc.Shipments.ReadImport(file)
	if err != nil {
		file = nil
		fail(nil, err)
		return
	}
	if r.PostFormValue("confirm") == "" || im.Bad > 0 {
		s.render(w, r, http.StatusOK, "app/shipment_import", s.importView(w, r, im, string(file)))
		return
	}
	booked, err := s.svc.Shipments.BookImport(r.Context(), actor(r), im)
	if err != nil {
		if _, ok := service.AsError(err); ok && len(booked) > 0 {
			// Part of the file is booked: say so on the list, where it shows.
			s.setFlash(w, "error", err.Error())
			http.Redirect(w, r, "/app/shipments", http.StatusSeeOther)
			return
		}
		fail(im, err)
		return
	}
	msg := "Booked 1 shipment."
	if len(booked) != 1 {
		msg = "Booked " + grouped(int64(len(booked))) + " shipments."
	}
	s.setFlash(w, "success", msg+" Select them below to assign a driver.")
	http.Redirect(w, r, "/app/shipments?tab=pending", http.StatusSeeOther)
}
