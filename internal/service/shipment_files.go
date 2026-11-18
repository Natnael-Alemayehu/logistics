package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
)

// A shipment's documents and charges: the files the office keeps with a load
// and what the customer is charged for it.

// DocumentKinds are what a document can be, in the order the form offers
// them, with their labels.
var DocumentKinds = []string{"waybill", "invoice", "customs", "delivery_note", "photo", "other"}

var documentKindLabels = map[string]string{
	"waybill": "Waybill", "invoice": "Invoice", "customs": "Customs papers",
	"delivery_note": "Delivery note", "photo": "Photo", "other": "Other",
}

// DocumentKindLabel names a kind of document for people.
func DocumentKindLabel(kind string) string {
	if l, ok := documentKindLabels[kind]; ok {
		return l
	}
	return "Document"
}

// Limits on a shipment's files and lines.
const (
	MaxDocumentBytes      = 10 << 20
	maxDocumentsPerLoad   = 30
	maxChargesPerShipment = 50
)

// documentTypes are the files accepted, by the type their first bytes show,
// and the extension they are stored with. What a browser claims is not
// trusted.
var documentTypes = map[string]string{
	"application/pdf": ".pdf",
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
}

// Documents lists a shipment's documents, newest first.
func (s *ShipmentService) Documents(ctx context.Context, tenantID, shipmentID pgtype.UUID) ([]db.ShipmentDocument, error) {
	return s.q.ListShipmentDocuments(ctx, db.ListShipmentDocumentsParams{TenantID: tenantID, ShipmentID: shipmentID})
}

// AddDocument stores a file with a shipment.
func (s *ShipmentService) AddDocument(ctx context.Context, a Actor, shipmentID pgtype.UUID, kind, name string, body []byte) (db.ShipmentDocument, error) {
	var doc db.ShipmentDocument
	if _, ok := documentKindLabels[kind]; !ok {
		return doc, invalid("kind", "Choose what the document is.")
	}
	if len(body) == 0 {
		return doc, invalid("file", "Choose a file to upload.")
	}
	if len(body) > MaxDocumentBytes {
		return doc, invalid("file", "The file is larger than 10 MB. Scan it at a lower resolution, or split it.")
	}
	ct := http.DetectContentType(body)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	ext, ok := documentTypes[ct]
	if !ok {
		return doc, invalid("file", "Upload a PDF, or a photo as JPEG or PNG.")
	}
	name = documentName(name, ext)
	if name == "" {
		name = DocumentKindLabel(kind) + ext
	}
	if s.media == nil || s.media.store == nil {
		return doc, errors.New("file storage is not configured")
	}

	sh, err := s.Get(ctx, a.TenantID, shipmentID)
	if err != nil {
		return doc, err
	}
	if sh.AnonymisedAt.Valid {
		return doc, invalid("file", "This shipment's details were removed at the end of the keeping period, so it takes no new documents.")
	}
	n, err := s.q.CountShipmentDocuments(ctx, db.CountShipmentDocumentsParams{TenantID: a.TenantID, ShipmentID: shipmentID})
	if err != nil {
		return doc, err
	}
	if n >= maxDocumentsPerLoad {
		return doc, invalid("file", fmt.Sprintf("A shipment holds at most %d documents. Remove one to add another.", maxDocumentsPerLoad))
	}

	key := fmt.Sprintf("documents/%s/%s/%s%s", uuidString(a.TenantID), uuidString(shipmentID), randomToken(16), ext)
	if _, err := s.media.store.Upload(ctx, key, bytes.NewReader(body), ct); err != nil {
		return doc, fmt.Errorf("store document: %w", err)
	}
	err = s.tx(ctx, func(q *db.Queries, _ *afterCommit) error {
		tenant, err := q.GetTenantByID(ctx, a.TenantID)
		if err != nil {
			return err
		}
		if err := s.requireWritable(ctx, q, tenant); err != nil {
			return err
		}
		doc, err = q.CreateShipmentDocument(ctx, db.CreateShipmentDocumentParams{
			TenantID: a.TenantID, ShipmentID: shipmentID, Kind: kind, Name: name, StorageKey: key,
			ContentType: ct, SizeBytes: int32(len(body)), UploadedBy: a.UserID, UploadedByName: a.Name,
		})
		if err != nil {
			return err
		}
		s.audit(ctx, q, a, "document_added", "shipment", shipmentID, nil, map[string]any{"document": name, "kind": kind, "bytes": len(body)})
		return nil
	})
	if err != nil {
		_ = s.media.store.Delete(context.WithoutCancel(ctx), key)
		return db.ShipmentDocument{}, err
	}
	return doc, nil
}

// documentName keeps a file's own name, tidied: no folders, no control
// characters, at most 120 letters, ending in the extension its content
// shows.
func documentName(name, ext string) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(strings.TrimSuffix(name, path.Ext(name)))
	if name == "." || name == "/" || name == "" {
		return ""
	}
	for utf8.RuneCountInString(name)+len(ext) > 120 {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return name + ext
}

// OpenDocument streams one of a shipment's documents.
func (s *ShipmentService) OpenDocument(ctx context.Context, tenantID, shipmentID, id pgtype.UUID) (db.ShipmentDocument, io.ReadCloser, error) {
	doc, err := s.q.GetShipmentDocument(ctx, db.GetShipmentDocumentParams{TenantID: tenantID, ShipmentID: shipmentID, ID: id})
	if isNoRows(err) {
		return doc, nil, notFound("Document not found.")
	}
	if err != nil {
		return doc, nil, err
	}
	rc, _, err := s.media.Open(ctx, doc.StorageKey)
	return doc, rc, err
}

// DeleteDocument removes a document and its file.
func (s *ShipmentService) DeleteDocument(ctx context.Context, a Actor, shipmentID, id pgtype.UUID) error {
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		key, err := q.DeleteShipmentDocument(ctx, db.DeleteShipmentDocumentParams{TenantID: a.TenantID, ShipmentID: shipmentID, ID: id})
		if isNoRows(err) {
			return notFound("Document not found.")
		}
		if err != nil {
			return err
		}
		s.audit(ctx, q, a, "document_removed", "shipment", shipmentID, map[string]any{"document": uuidString(id)}, nil)
		after.do(func() { s.media.Delete(context.Background(), key) })
		return nil
	})
}

// Charges lists a shipment's charges in the order they were added, and
// their total in birr.
func (s *ShipmentService) Charges(ctx context.Context, tenantID, shipmentID pgtype.UUID) ([]db.ShipmentCharge, float64, error) {
	rows, err := s.q.ListShipmentCharges(ctx, db.ListShipmentChargesParams{TenantID: tenantID, ShipmentID: shipmentID})
	total := 0.0
	for _, c := range rows {
		total += numericOr(c.Amount)
	}
	return rows, math.Round(total*100) / 100, err
}

// AddCharge adds a line to what the customer is charged for a load. A
// negative amount is a discount.
func (s *ShipmentService) AddCharge(ctx context.Context, a Actor, shipmentID pgtype.UUID, description string, amount float64) error {
	description = strings.TrimSpace(description)
	if description == "" {
		return invalid("description", "Say what the charge is for.")
	}
	if utf8.RuneCountInString(description) > 120 {
		return invalid("description", "Describe the charge in 120 letters or fewer.")
	}
	amount = math.Round(amount*100) / 100
	if amount == 0 || math.IsNaN(amount) || math.Abs(amount) >= 1e12 {
		return invalid("amount", "Enter an amount in birr; a discount is negative.")
	}
	if _, err := s.Get(ctx, a.TenantID, shipmentID); err != nil {
		return err
	}
	return s.tx(ctx, func(q *db.Queries, _ *afterCommit) error {
		tenant, err := q.GetTenantByID(ctx, a.TenantID)
		if err != nil {
			return err
		}
		if err := s.requireWritable(ctx, q, tenant); err != nil {
			return err
		}
		n, err := q.CountShipmentCharges(ctx, db.CountShipmentChargesParams{TenantID: a.TenantID, ShipmentID: shipmentID})
		if err != nil {
			return err
		}
		if n >= maxChargesPerShipment {
			return invalid("description", fmt.Sprintf("A shipment holds at most %d charges.", maxChargesPerShipment))
		}
		if _, err := q.CreateShipmentCharge(ctx, db.CreateShipmentChargeParams{
			TenantID: a.TenantID, ShipmentID: shipmentID, Description: description, Amount: toNumeric(amount), CreatedBy: a.UserID,
		}); err != nil {
			return err
		}
		s.audit(ctx, q, a, "charge_added", "shipment", shipmentID, nil, map[string]any{"description": description, "amount": amount})
		return nil
	})
}

// DeleteCharge removes a line from a shipment's charges.
func (s *ShipmentService) DeleteCharge(ctx context.Context, a Actor, shipmentID, id pgtype.UUID) error {
	return s.tx(ctx, func(q *db.Queries, _ *afterCommit) error {
		n, err := q.DeleteShipmentCharge(ctx, db.DeleteShipmentChargeParams{TenantID: a.TenantID, ShipmentID: shipmentID, ID: id})
		if err != nil {
			return err
		}
		if n == 0 {
			return notFound("Charge not found.")
		}
		s.audit(ctx, q, a, "charge_removed", "shipment", shipmentID, map[string]any{"charge": uuidString(id)}, nil)
		return nil
	})
}
