//go:build integration

package integration

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

// upload posts a file to the import page as a browser's file field does.
func (b *browser) upload(csv string) page {
	b.e.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "shipments.csv")
	if err != nil {
		b.e.t.Fatal(err)
	}
	_, _ = fw.Write([]byte(csv))
	_ = mw.Close()
	req, _ := http.NewRequest(http.MethodPost, b.e.srv.URL+"/app/shipments/import", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return b.do(req)
}

var carried = regexp.MustCompile(`(?s)<textarea name="csv" hidden>(.*?)</textarea>`)

// TestImportShipments (FR-013 AC-013.1): a CSV file is checked whole and
// shown before anything is booked, and booked only when every row is right.
func TestImportShipments(t *testing.T) {
	e := newEnv(t)
	office := e.company("Spreadsheet Freight")
	count := func() int64 {
		return scalar[int64](t, `SELECT count(*) FROM shipments WHERE tenant_id = (SELECT tenant_id FROM users WHERE email = $1)`, office.email)
	}

	if p := office.get("/app/shipments/import"); p.Status != http.StatusOK || !strings.Contains(p.Body, "Receiver phone") {
		t.Fatalf("the import page: status %d", p.Status)
	}
	// The template is itself a file the import accepts.
	template := office.get("/app/shipments/import/template.csv")
	if p := office.upload(template.Body); !strings.Contains(p.Body, "1 shipment is ready to book") {
		t.Errorf("the template should pass its own check: %s", errorText(p.Body))
	}

	for why, c := range map[string]struct{ file, want string }{
		"no file":               {"", "Choose a CSV file."},
		"headings only":         {"Origin,Destination,Customer,Customer phone\n", "no shipments under them"},
		"a column missing":      {"Origin,Destination,Customer\nAddis Ababa,Adama,Abyssinia Traders\n", "these are missing: Customer phone."},
		"more than the maximum": {"Origin,Destination,Customer,Customer phone\n" + strings.Repeat("Addis Ababa,Adama,Abyssinia Traders,0911223344\n", 201), "at most 200 shipments"},
	} {
		p := office.upload(c.file)
		if p.Status != http.StatusBadRequest || !strings.Contains(p.Body, c.want) {
			t.Errorf("%s: status %d, page should say %q, says %q", why, p.Status, c.want, errorText(p.Body))
		}
	}

	// Rows with mistakes: each is named, and nothing can be booked.
	customer, receiver := phoneNumber(), phoneNumber()
	bad := "Origin,Destination,Customer,Customer phone,Weight (kg),Pickup\n" +
		"Addis Ababa,Adama,Abyssinia Traders," + customer + ",1000,\n" +
		"Addis Ababa,Atlantis,Abyssinia Traders," + customer + ",,\n" +
		"Addis Ababa,Adama,Abyssinia Traders,12,,\n" +
		"Addis Ababa,Adama,Abyssinia Traders," + customer + ",heavy,\n" +
		"Addis Ababa,Adama,Abyssinia Traders," + customer + ",,next Tuesday\n"
	p := office.upload(bad)
	for _, want := range []string{"4 of 5 rows can't be booked", "Atlantis", "enter an Ethiopian mobile number", "The weight &#34;heavy&#34; is not a number.", "is not a date and time"} {
		if !strings.Contains(p.Body, want) {
			t.Errorf("the check should say %q", want)
		}
	}
	if strings.Contains(p.Body, `name="confirm"`) {
		t.Error("a file with bad rows must not offer to book")
	}
	// Confirming it by hand books nothing either.
	office.post("/app/shipments/import", url.Values{"confirm": {"1"}, "csv": {bad}})
	if n := count(); n != 0 {
		t.Fatalf("%d shipments booked from a file with bad rows", n)
	}

	// A good file, as a spreadsheet here saves it: a byte-order mark,
	// semicolons, headings spelled its own way, a column of its own, a
	// blank line, a quoted address with a line break's worth of commas.
	addis, _ := time.LoadLocation("Africa/Addis_Ababa")
	pickupAt := time.Now().In(addis).AddDate(0, 0, 3).Format("2006-01-02") + " 08:30"
	good := "\xEF\xBB\xBFfrom;TO;customer name;Customer Phone;consignee;receiver_phone;cargo;weight;priority;Pickup;Our ref\r\n" +
		"Addis Ababa;Hawassa;Abyssinia Traders;" + customer + ";Lakeside Hardware;" + receiver + ";\"Cement; 200 bags\";10,000;urgent;" + pickupAt + ";A-17\r\n" +
		";;;;;;;;;;\r\n" +
		"Adama;Addis Ababa;Sidama Coffee;" + phoneNumber() + ";;;Coffee;;;;A-18\r\n"
	p = office.upload(good)
	if !strings.Contains(p.Body, "2 shipments are ready to book") || !strings.Contains(p.Body, "Columns not read: Our ref.") {
		_, main, _ := strings.Cut(p.Body, `<div class="page-head">`)
		t.Fatalf("the good file's check: %.1500s", main)
	}
	if n := count(); n != 0 {
		t.Fatalf("%d shipments booked by the check alone", n)
	}
	m := carried.FindStringSubmatch(p.Body)
	if m == nil {
		t.Fatal("the check should carry the file to the confirmation")
	}
	// What the page carries is what gets booked.
	file := strings.NewReplacer("&#34;", `"`, "&amp;", "&", "&lt;", "<", "&gt;", ">", "&#39;", "'", "&#13;", "\r").Replace(m[1])
	done := office.mustPost("/app/shipments/import", url.Values{"confirm": {"1"}, "csv": {file}})
	if done.Location != "/app/shipments?tab=pending" {
		t.Errorf("after the import: %q", done.Location)
	}
	if p := office.get(done.Location); !strings.Contains(p.Body, "Booked 2 shipments.") {
		t.Error("the list should say how many were booked")
	}
	if n := count(); n != 2 {
		t.Fatalf("%d shipments booked, want 2", n)
	}
	var cargo, priority, consignee, pickup string
	var weight float64
	if err := pool.QueryRow(t.Context(), `SELECT cargo_description, priority, consignee_name, cargo_weight::float8,
			to_char(planned_pickup_at AT TIME ZONE 'Africa/Addis_Ababa', 'YYYY-MM-DD HH24:MI')
		FROM shipments WHERE customer_phone = $1`, customer).Scan(&cargo, &priority, &consignee, &weight, &pickup); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(cargo, "|", priority, "|", consignee, "|", weight, "|", pickup); got != "Cement; 200 bags|urgent|Lakeside Hardware|10000|"+pickupAt {
		t.Errorf("the booked shipment: %s", got)
	}
	// Each is booked as the form would book it: texts and all.
	e.received("sms", customer, "is booked")
	e.received("sms", receiver, "is booked")
	if n := scalar[int64](t, `SELECT count(*) FROM audit_logs WHERE action = 'shipments_imported' AND new_value->>'shipments' = '2'`); n != 1 {
		t.Errorf("%d audit entries for the import, want 1", n)
	}

	// The list's own export can be sent back.
	export := office.get("/app/shipments?format=csv")
	if p := office.upload(export.Body); !strings.Contains(p.Body, "2 shipments are ready to book") || !strings.Contains(p.Body, "Tracking number") {
		t.Errorf("the export should pass the check, with its other columns ignored: %s", errorText(p.Body))
	}
}
