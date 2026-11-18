package web

import (
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// A shipment's documents and charges: its Documents and Charges tabs.

func (s *Server) shipmentDocumentAdd(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, service.MaxDocumentBytes+64<<10)
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	back := "/app/shipments/" + id.String() + "?tab=documents"
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		s.back(w, r, back, "error", "The file is larger than 10 MB. Scan it at a lower resolution, or split it.")
		return
	}
	var body []byte
	var name string
	if f, h, err := r.FormFile("file"); err == nil {
		body, err = io.ReadAll(f)
		_ = f.Close()
		if err != nil {
			s.fail(w, r, err)
			return
		}
		name = h.Filename
	}
	doc, err := s.svc.Shipments.AddDocument(r.Context(), actor(r), id, r.PostFormValue("kind"), name, body)
	s.actionResult(w, r, back, err, doc.Name+" added.")
}

func (s *Server) shipmentDocument(w http.ResponseWriter, r *http.Request) {
	id, err1 := pathID(r, "id")
	docID, err2 := pathID(r, "doc")
	if err1 != nil || err2 != nil {
		s.notFound(w, r)
		return
	}
	doc, rc, err := s.svc.Shipments.OpenDocument(r.Context(), principal(r).TenantID, id, docID)
	if err != nil {
		if service.IsNotFound(err) {
			s.notFound(w, r)
			return
		}
		s.fail(w, r, err)
		return
	}
	defer rc.Close()
	disposition := "attachment"
	if r.URL.Query().Get("view") != "" {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", doc.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(int(doc.SizeBytes)))
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": doc.Name}))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// A PDF shown in the browser runs no script of its own.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; object-src 'self'; sandbox")
	_, _ = io.Copy(w, rc)
}

func (s *Server) shipmentDocumentDelete(w http.ResponseWriter, r *http.Request) {
	s.shipmentFileAction(w, r, "doc", "documents", func(id, item pgtype.UUID) (string, error) {
		return "Document removed.", s.svc.Shipments.DeleteDocument(r.Context(), actor(r), id, item)
	})
}

func (s *Server) shipmentChargeAdd(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	amount, perr := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(r.PostFormValue("amount")), ",", ""), 64)
	if perr != nil {
		amount = 0
	}
	err = s.svc.Shipments.AddCharge(r.Context(), actor(r), id, r.PostFormValue("description"), amount)
	s.actionResult(w, r, "/app/shipments/"+id.String()+"?tab=charges", err, "Charge added.")
}

func (s *Server) shipmentChargeDelete(w http.ResponseWriter, r *http.Request) {
	s.shipmentFileAction(w, r, "charge", "charges", func(id, item pgtype.UUID) (string, error) {
		return "Charge removed.", s.svc.Shipments.DeleteCharge(r.Context(), actor(r), id, item)
	})
}

// shipmentFileAction runs an action on one item of a shipment's tab and
// returns to that tab.
func (s *Server) shipmentFileAction(w http.ResponseWriter, r *http.Request, param, tab string, do func(id, item pgtype.UUID) (string, error)) {
	id, err1 := pathID(r, "id")
	item, err2 := pathID(r, param)
	if err1 != nil || err2 != nil {
		s.notFound(w, r)
		return
	}
	msg, err := do(id, item)
	s.actionResult(w, r, "/app/shipments/"+id.String()+"?tab="+tab, err, msg)
}
