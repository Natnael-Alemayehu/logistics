//go:build integration

package integration

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// uploadDocument posts a file to a shipment's documents, as the form does.
func (b *browser) uploadDocument(shipmentID, name, kind string, content []byte) page {
	b.e.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("kind", kind)
	fw, err := mw.CreateFormFile("file", name)
	if err != nil {
		b.e.t.Fatal(err)
	}
	_, _ = fw.Write(content)
	_ = mw.Close()
	req, _ := http.NewRequest(http.MethodPost, b.e.srv.URL+"/app/shipments/"+shipmentID+"/documents", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Origin", b.e.srv.URL)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	return b.do(req)
}

var pdf = []byte("%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\ntrailer << /Root 1 0 R >>\n%%EOF\n")

// TestShipmentDocuments: the office keeps files with a load. Only PDFs and
// photos are taken, by what they are rather than what they are called;
// only the company sees them; and the keeping period removes them with
// the customer's other details.
func TestShipmentDocuments(t *testing.T) {
	e := newEnv(t)
	office := e.company("Paperwork Freight")
	id, _ := office.book("Addis Ababa", "Adama", "")
	count := func() int64 {
		return scalar[int64](t, `SELECT count(*) FROM shipment_documents WHERE shipment_id = $1`, id)
	}
	before := len(storedFiles(t, e.files))

	office.uploadDocument(id, `C:\scans\Customs declaration.pdf`, "customs", pdf)
	if count() != 1 || len(storedFiles(t, e.files)) != before+1 {
		t.Fatalf("%d documents, want the PDF stored", count())
	}
	if got := scalar[string](t, `SELECT name || '|' || kind || '|' || content_type FROM shipment_documents WHERE shipment_id = $1`, id); got != "Customs declaration.pdf|customs|application/pdf" {
		t.Errorf("stored as %q", got)
	}
	for why, up := range map[string]func() page{
		"text called a PDF": func() page { return office.uploadDocument(id, "notes.pdf", "other", []byte("just some words")) },
		"an unknown kind":   func() page { return office.uploadDocument(id, "a.pdf", "secret", pdf) },
		"an empty file":     func() page { return office.uploadDocument(id, "a.pdf", "other", nil) },
	} {
		up()
		if count() != 1 {
			t.Errorf("%s was stored", why)
		}
	}

	docID := scalar[string](t, `SELECT id::text FROM shipment_documents WHERE shipment_id = $1`, id)
	if p := office.get("/app/shipments/" + id + "?tab=documents"); !strings.Contains(p.Body, "Customs declaration.pdf") || !strings.Contains(p.Body, "Customs papers") {
		t.Error("the Documents tab should list the file")
	}
	res, body := office.fetch("/app/shipments/" + id + "/documents/" + docID)
	if res.StatusCode != http.StatusOK || !bytes.Equal(body, pdf) || res.Header.Get("Content-Type") != "application/pdf" ||
		!strings.HasPrefix(res.Header.Get("Content-Disposition"), "attachment") || !strings.Contains(res.Header.Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("download: status %d, %q, %q, %d bytes", res.StatusCode, res.Header.Get("Content-Type"), res.Header.Get("Content-Disposition"), len(body))
	}

	// Another company can neither read nor remove it.
	other := e.company("Nosy Freight")
	if res, _ := other.fetch("/app/shipments/" + id + "/documents/" + docID); res.StatusCode != http.StatusNotFound {
		t.Errorf("another company's download: status %d, want 404", res.StatusCode)
	}
	other.post("/app/shipments/"+id+"/documents/"+docID+"/delete", nil)
	if count() != 1 {
		t.Error("another company removed the document")
	}

	// The company's export carries it, file and all, and the charges.
	office.post("/app/shipments/"+id+"/charges", url.Values{"description": {"Freight"}, "amount": {"9000"}})
	tracking := scalar[string](t, `SELECT tracking_number FROM shipments WHERE id = $1`, id)
	_, zipped := office.download("/app/settings/export")
	z, err := zip.NewReader(bytes.NewReader(zipped), int64(len(zipped)))
	if err != nil {
		t.Fatalf("the export isn't a ZIP: %v", err)
	}
	files := map[string][]byte{}
	for _, f := range z.File {
		rc, _ := f.Open()
		files[f.Name], _ = io.ReadAll(rc)
		rc.Close()
	}
	inZip := "documents/" + tracking + "/1-Customs-declaration.pdf"
	if !bytes.Equal(files[inZip], pdf) || !strings.Contains(string(files["documents.csv"]), "Customs papers,Customs declaration.pdf") ||
		!strings.Contains(string(files["documents.csv"]), inZip) || !strings.Contains(string(files["charges.csv"]), tracking+",Freight,9000.00") {
		t.Errorf("the export's documents and charges:\n%s\n%s", files["documents.csv"], files["charges.csv"])
	}

	office.mustPost("/app/shipments/"+id+"/documents/"+docID+"/delete", nil)
	if count() != 0 || len(storedFiles(t, e.files)) != before {
		t.Errorf("after removing: %d documents, %d files", count(), len(storedFiles(t, e.files)))
	}

	// The keeping period removes documents and their files.
	office.uploadDocument(id, "invoice.pdf", "invoice", pdf)
	exec(t, `UPDATE shipments SET status = 'cancelled', status_changed_at = NOW() - INTERVAL '400 days', updated_at = NOW() - INTERVAL '400 days' WHERE id = $1`, id)
	if err := e.svc.Jobs.Retention(context.Background()); err != nil {
		t.Fatal(err)
	}
	if count() != 0 || len(storedFiles(t, e.files)) != before {
		t.Errorf("after the keeping period: %d documents, %d files", count(), len(storedFiles(t, e.files))-before)
	}
	if office.uploadDocument(id, "late.pdf", "other", pdf); count() != 0 {
		t.Error("a shipment past its keeping period took a document")
	}
}

// TestShipmentCharges: what a customer is charged for a load, line by line,
// with discounts as negative lines.
func TestShipmentCharges(t *testing.T) {
	e := newEnv(t)
	office := e.company("Invoicing Freight")
	id, _ := office.book("Addis Ababa", "Adama", "")
	add := func(what, amount string) {
		office.post("/app/shipments/"+id+"/charges", url.Values{"description": {what}, "amount": {amount}})
	}
	add("Freight", "12,500")
	add("Loading", "750.25")
	add("Discount", "-500")
	for _, bad := range [][2]string{{"Nothing", "0"}, {"", "100"}, {"Words", "lots"}, {strings.Repeat("x", 121), "5"}} {
		add(bad[0], bad[1])
	}
	if n := scalar[int64](t, `SELECT count(*) FROM shipment_charges WHERE shipment_id = $1`, id); n != 3 {
		t.Fatalf("%d charges, want 3", n)
	}
	p := office.get("/app/shipments/" + id + "?tab=charges")
	if !strings.Contains(p.Body, "ETB\u00a012,750.25") || !strings.Contains(p.Body, "ETB\u00a0-500.00") {
		t.Error("the Charges tab should show each line and the total, 12,750.25")
	}

	other := e.company("Meddling Freight")
	charge := scalar[string](t, `SELECT id::text FROM shipment_charges WHERE shipment_id = $1 AND description = 'Discount'`, id)
	other.post("/app/shipments/"+id+"/charges/"+charge+"/delete", nil)
	other.post("/app/shipments/"+id+"/charges", url.Values{"description": {"Sneaky"}, "amount": {"1"}})
	if n := scalar[int64](t, `SELECT count(*) FROM shipment_charges WHERE shipment_id = $1`, id); n != 3 {
		t.Errorf("another company changed the charges: %d lines", n)
	}
	office.mustPost("/app/shipments/"+id+"/charges/"+charge+"/delete", nil)
	if n := scalar[int64](t, `SELECT count(*) FROM shipment_charges WHERE shipment_id = $1`, id); n != 2 {
		t.Errorf("%d charges after removing one, want 2", n)
	}
}

// fetch GETs a path and returns the response with its body read.
func (b *browser) fetch(path string) (*http.Response, []byte) {
	b.e.t.Helper()
	res, err := b.c.Get(b.e.srv.URL + path)
	if err != nil {
		b.e.t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res, body
}
