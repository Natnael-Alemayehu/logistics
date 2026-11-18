//go:build integration

package integration

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// TestHistoryIsAppendOnly: the audit log and the shipment timeline can't be
// changed or emptied, and signing out is recorded (AC-042.1, AC-042.5).
func TestHistoryIsAppendOnly(t *testing.T) {
	e := newEnv(t)
	office := e.company("Ledger Lines")
	id, _ := office.book("Addis Ababa", "Adama", "")
	for _, sql := range []string{
		`UPDATE audit_logs SET action = 'nothing' WHERE tenant_id = $1`,
		`DELETE FROM audit_logs WHERE tenant_id = $1`,
		`UPDATE shipment_events SET note = 'rewritten' WHERE tenant_id = $1`,
		`DELETE FROM shipment_events WHERE tenant_id = $1`,
	} {
		_, err := pool.Exec(context.Background(), sql, office.tenant())
		if err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("%s: %v, want it refused", sql, err)
		}
	}
	if _, err := pool.Exec(context.Background(), `TRUNCATE shipment_events`); err == nil {
		t.Error("the shipment timeline could be emptied")
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM shipment_events WHERE shipment_id = $1`, id); n == 0 {
		t.Error("the booking left no history")
	}

	office.mustPost("/logout", nil)
	ip := scalar[string](t, `SELECT COALESCE(a.ip_address, '') FROM audit_logs a JOIN users u ON u.id = a.user_id
		WHERE u.email = $1 AND a.action = 'logout'`, office.email)
	if ip == "" {
		t.Error("signing out was recorded without the address it came from")
	}
}

// TestExportHoldsEverything: an administrator downloads the company's data
// as one ZIP, with the delivery photos, and nothing of other companies'
// (AC-041.5).
func TestExportHoldsEverything(t *testing.T) {
	e := newEnv(t)
	office := e.company("Export Express")
	_, otherTracking := e.company("Someone Else Transport").book("Addis Ababa", "Hawassa", "")
	driverID, phone, pin := office.addDriver("Hirut Assefa")
	id, tracking := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)
	if r := d.status(clientID(), id, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	if r := d.pod(service.SyncPOD{ShipmentID: id, RecipientName: "Almaz Kebede", Photos: []string{photoJPEG(t)}}); !r.OK {
		t.Fatalf("POD: %+v", r)
	}

	res, body := office.download("/app/settings/export")
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "application/zip" {
		t.Fatalf("export: status %d, type %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	if cd := res.Header.Get("Content-Disposition"); !strings.Contains(cd, `filename="export-express-data-`) {
		t.Errorf("export file name: %q", cd)
	}
	z, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("the export isn't a ZIP: %v", err)
	}
	files := map[string][]byte{}
	for _, f := range z.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name], _ = io.ReadAll(rc)
		rc.Close()
	}
	for _, name := range []string{"README.txt", "shipments.csv", "events.csv", "deliveries.csv", "people.csv", "vehicles.csv",
		"deliveries/" + tracking + "/signature.png", "deliveries/" + tracking + "/photo-1.jpg"} {
		if files[name] == nil {
			t.Errorf("the export has no %s", name)
		}
	}
	key := scalar[string](t, `SELECT signature_url FROM proof_of_deliveries WHERE shipment_id = $1`, id)
	stored, err := os.ReadFile(filepath.Join(e.files, key))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, files["deliveries/"+tracking+"/signature.png"]) {
		t.Error("the exported signature isn't the stored one")
	}
	shipments := readCSV(t, files["shipments.csv"])
	if len(shipments) != 2 || shipments[1][0] != tracking || shipments[1][13] != "Abyssinia Traders" {
		t.Errorf("shipments.csv: %v", shipments)
	}
	if bytes.Contains(files["shipments.csv"], []byte(otherTracking)) {
		t.Error("the export holds another company's shipment")
	}
	deliveries := readCSV(t, files["deliveries.csv"])
	if len(deliveries) != 2 || deliveries[1][4] != "Almaz Kebede" || !strings.Contains(deliveries[1][13], "photo-1.jpg") {
		t.Errorf("deliveries.csv: %v", deliveries)
	}
	if !bytes.Contains(files["people.csv"], []byte("Hirut Assefa")) {
		t.Error("people.csv doesn't list the driver")
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM audit_logs WHERE tenant_id = $1 AND action = 'data_exported'`, office.tenant()); n != 1 {
		t.Errorf("%d audit entries for the download, want 1", n)
	}

	if res, _ := office.dispatcher().download("/app/settings/export"); res.StatusCode != http.StatusForbidden {
		t.Errorf("a dispatcher's export: status %d, want 403", res.StatusCode)
	}
}

// TestRetentionRemovesOldCustomerDetails: a year after a shipment finished,
// its customer's and recipient's details and the photos go; what reports
// count stays (AC-041.2 to AC-041.4).
func TestRetentionRemovesOldCustomerDetails(t *testing.T) {
	e := newEnv(t)
	office := e.company("Archive Haulage")
	driverID, phone, pin := office.addDriver("Bekele Girma")
	oldID, _ := office.book("Addis Ababa", "Adama", driverID)
	recentID, _ := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)
	for _, id := range []string{oldID, recentID} {
		if r := d.status(clientID(), id, "in_transit"); !r.OK {
			t.Fatalf("depart: %+v", r)
		}
		if r := d.pod(service.SyncPOD{ShipmentID: id, RecipientName: "Almaz Kebede", RecipientPhone: "0911223344",
			Notes: "Left with the guard", Photos: []string{photoJPEG(t)}}); !r.OK {
			t.Fatalf("POD: %+v", r)
		}
	}
	if n := len(storedFiles(t, e.files)); n != 4 {
		t.Fatalf("%d stored files, want a signature and a photo for each delivery", n)
	}
	exec(t, `UPDATE shipments SET actual_delivery = NOW() - INTERVAL '400 days', status_changed_at = NOW() - INTERVAL '400 days',
		updated_at = NOW() - INTERVAL '400 days' WHERE id = $1`, oldID)

	if err := e.svc.Jobs.Retention(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := scalar[string](t, `SELECT customer_name || '|' || customer_phone || '|' || destination_address || '|' || destination_city || '|' || status
		FROM shipments WHERE id = $1 AND anonymised_at IS NOT NULL`, oldID); got != "Former customer|||Adama|delivered" {
		t.Errorf("old shipment: %q, want the customer gone and the route kept", got)
	}
	if got := scalar[string](t, `SELECT recipient_name || '|' || COALESCE(recipient_phone, '') || '|' || COALESCE(signature_url, '') || '|' ||
		COALESCE(delivery_notes, '') || '|' || cardinality(photo_urls) FROM proof_of_deliveries WHERE shipment_id = $1`, oldID); got != "Removed||||0" {
		t.Errorf("old delivery: %q, want the recipient, signature, notes and photos gone", got)
	}
	if files := storedFiles(t, e.files); len(files) != 2 || !strings.Contains(strings.Join(files, " "), recentID) {
		t.Errorf("stored files %v, want only the recent delivery's", files)
	}
	if got := scalar[string](t, `SELECT customer_name FROM shipments WHERE id = $1 AND anonymised_at IS NULL`, recentID); got != "Abyssinia Traders" {
		t.Errorf("recent shipment's customer: %q", got)
	}
	// Reports still count it.
	if n := scalar[int64](t, `SELECT COUNT(*) FROM shipments WHERE tenant_id = $1 AND status = 'delivered'`, office.tenant()); n != 2 {
		t.Errorf("%d delivered shipments, want 2", n)
	}
	p := office.get("/app/shipments/" + oldID)
	if p.Status != http.StatusOK || !strings.Contains(p.Body, "end of the keeping period") || strings.Contains(p.Body, "Abyssinia Traders") {
		t.Errorf("old shipment's page: status %d, note shown %v", p.Status, strings.Contains(p.Body, "end of the keeping period"))
	}
	if n, err := e.svc.Data.AnonymiseFinished(context.Background(), time.Now().Add(-365*24*time.Hour)); err != nil || n != 0 {
		t.Errorf("second run: %d shipments, %v; want nothing left to do", n, err)
	}
}

// TestRemovePersonDetails: an administrator removes what identifies a
// driver who has left; their deliveries and history stay (AC-041.6).
func TestRemovePersonDetails(t *testing.T) {
	e := newEnv(t)
	office := e.company("Turnover Trucking")
	driverID, phone, pin := office.addDriver("Getachew Lemma")
	id, _ := office.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)
	if r := d.status(clientID(), id, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	if r := d.pod(service.SyncPOD{ShipmentID: id, RecipientName: "Almaz Kebede"}); !r.OK {
		t.Fatalf("POD: %+v", r)
	}
	if p := office.post("/app/drivers/"+driverID+"/reset", nil); p.Status != http.StatusOK { // texts a temporary PIN
		t.Fatalf("reset PIN: status %d %s", p.Status, errorText(p.Body))
	}

	if msg, failed := office.result("/app/drivers/"+driverID+"/anonymise", nil); !failed || !strings.Contains(msg, "Deactivate") {
		t.Errorf("removing an active driver: %q", msg)
	}
	if msg, failed := office.result("/app/drivers/"+driverID+"/active", url.Values{"active": {"false"}}); failed {
		t.Fatalf("deactivate: %s", msg)
	}
	if msg, failed := office.result("/app/drivers/"+driverID+"/anonymise", nil); failed || !strings.Contains(msg, "Getachew Lemma") {
		t.Fatalf("remove details: %q", msg)
	}

	if got := scalar[string](t, `SELECT full_name || '|' || COALESCE(phone, '') || '|' || (pin_hash IS NULL)::text FROM users WHERE id = $1`, driverID); got != "Former driver||true" {
		t.Errorf("the driver is now %q", got)
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM shipment_events WHERE actor_id = $1 AND actor_name <> 'Former driver'`, driverID); n != 0 {
		t.Errorf("%d history entries still name the driver", n)
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM sms_messages WHERE phone = $1`, phone); n != 0 {
		t.Errorf("%d texts to the driver's phone kept", n)
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM proof_of_deliveries WHERE shipment_id = $1 AND driver_id = $2`, id, driverID); n != 1 {
		t.Error("the delivery is no longer credited to the driver")
	}
	if p := office.get("/app/shipments/" + id); p.Status != http.StatusOK || !strings.Contains(p.Body, "Former driver") || strings.Contains(p.Body, "Getachew") {
		t.Errorf("shipment page after removal: status %d", p.Status)
	}
	if code := e.api("POST", "/api/v1/driver/login", "", map[string]string{"phone": phone, "pin": "4826", "device_id": "x"}, nil); code == http.StatusOK {
		t.Error("the driver can still sign in")
	}
	if msg, failed := office.result("/app/drivers/"+driverID+"/active", url.Values{"active": {"true"}}); !failed || !strings.Contains(msg, "removed") {
		t.Errorf("reactivating: %q", msg)
	}
	if msg, failed := office.result("/app/drivers/"+driverID+"/anonymise", nil); !failed || !strings.Contains(msg, "already") {
		t.Errorf("removing twice: %q", msg)
	}
	// The number is free for someone new.
	if p := office.post("/app/drivers", url.Values{"full_name": {"New Driver"}, "phone": {phone}}); p.Status != http.StatusOK {
		t.Errorf("adding a driver with the freed number: status %d %s", p.Status, errorText(p.Body))
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM audit_logs WHERE action = 'user_anonymised' AND entity_id = $1`, driverID); n != 1 {
		t.Errorf("%d audit entries for the removal, want 1", n)
	}
}

// TestDeleteCompany: the platform admin deletes a company and everything it
// holds; its invoices stay for the accounts (AC-049.3).
func TestDeleteCompany(t *testing.T) {
	e := newEnv(t)
	op := e.operator()
	gone := e.subscribed("Closing Freight", "starter", op)
	gone.confirmEmail()
	upgrade := gone.choose("business", "monthly")
	_, prompt := gone.startPayment(upgrade)
	driverID, phone, pin := gone.addDriver("Abebe Kebede")
	id, _ := gone.book("Addis Ababa", "Adama", driverID)
	d := e.signIn(phone, pin)
	if r := d.status(clientID(), id, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	if r := d.pod(service.SyncPOD{ShipmentID: id, RecipientName: "Almaz Kebede", Photos: []string{photoJPEG(t)}}); !r.OK {
		t.Fatalf("POD: %+v", r)
	}
	stays := e.company("Staying Logistics")
	_, staysTracking := stays.book("Addis Ababa", "Bahir Dar", "")
	goneID := gone.tenant()
	paid := scalar[string](t, `SELECT id::text FROM invoices WHERE tenant_id = $1 AND status = 'paid'`, goneID)

	page := op.get("/app/platform/tenants/" + goneID)
	number := scalar[string](t, `SELECT number FROM invoices WHERE id = $1`, paid)
	for _, text := range []string{"Closing Freight", number, "Delete for good"} {
		if !strings.Contains(page.Body, text) {
			t.Errorf("the company's page is missing %q", text)
		}
	}
	if p := op.get("/app/platform/tenants/" + op.tenant()); p.Status != http.StatusNotFound {
		t.Errorf("the platform's own account as a company: status %d, want 404", p.Status)
	}
	if p := stays.get("/app/platform/tenants/" + goneID); p.Status != http.StatusForbidden {
		t.Errorf("a company admin opening another company's page: status %d, want 403", p.Status)
	}

	back := url.Values{"confirm": {"closing freight"}, "back": {"/app/platform/tenants/" + goneID}}
	if msg, failed := op.result("/app/platform/tenants/"+goneID+"/delete", back); !failed || !strings.Contains(msg, "exactly") {
		t.Errorf("deleting with the name typed wrong: %q", msg)
	}
	if msg, failed := op.result("/app/platform/tenants/"+op.tenant()+"/delete", url.Values{"confirm": {"Platform Operations"}}); !failed {
		t.Errorf("deleting the platform's own account: %q", msg)
	}
	if p := stays.post("/app/platform/tenants/"+goneID+"/delete", url.Values{"confirm": {"Closing Freight"}}); p.Status != http.StatusForbidden {
		t.Errorf("a company admin deleting a company: status %d, want 403", p.Status)
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM tenants WHERE id = $1`, goneID); n != 1 {
		t.Fatal("the company went before it was confirmed")
	}

	back.Set("confirm", " Closing Freight ")
	if p := op.mustPost("/app/platform/tenants/"+goneID+"/delete", back); p.Location != "/app/platform" {
		t.Fatalf("after deleting, sent to %q, want the companies list", p.Location)
	}
	if m := anyFlashRe.FindStringSubmatch(op.get("/app/platform").Body); m == nil || m[1] != "success" || !strings.Contains(m[2], "deleted") {
		t.Fatalf("delete: %q", m)
	}
	for _, sql := range []string{
		`SELECT COUNT(*) FROM tenants WHERE id = $1`,
		`SELECT COUNT(*) FROM users WHERE tenant_id = $1`,
		`SELECT COUNT(*) FROM shipments WHERE tenant_id = $1`,
		`SELECT COUNT(*) FROM shipment_events WHERE tenant_id = $1`,
		`SELECT COUNT(*) FROM proof_of_deliveries WHERE tenant_id = $1`,
		`SELECT COUNT(*) FROM audit_logs WHERE tenant_id = $1`,
	} {
		if n := scalar[int64](t, sql, goneID); n != 0 {
			t.Errorf("%s: %d left", sql, n)
		}
	}
	if got := scalar[string](t, `SELECT status FROM invoices WHERE id = $1 AND tenant_id IS NULL`, paid); got != "paid" {
		t.Errorf("the paid invoice is %q", got)
	}
	if got := scalar[string](t, `SELECT status FROM invoices WHERE id = $1 AND tenant_id IS NULL`, upgrade); got != "void" {
		t.Errorf("the open invoice is %q, want cancelled", got)
	}
	if n := scalar[int64](t, `SELECT COUNT(*) FROM audit_logs WHERE action = 'tenant_deleted' AND entity_id = $1 AND tenant_id = $2
		AND new_value->>'name' = 'Closing Freight'`, goneID, op.tenant()); n != 1 {
		t.Errorf("%d records of the deletion, want 1", n)
	}
	if p := op.get("/app/platform/billing"); !strings.Contains(p.Body, "Closing Freight") {
		t.Error("the platform's billing page lost the deleted company's invoices")
	}
	if p := op.get("/app/platform/audit?action=tenant_deleted"); !strings.Contains(p.Body, "Closing Freight") {
		t.Error("the audit log doesn't show the deletion")
	}
	if p := gone.get("/app"); p.Status != http.StatusSeeOther || !strings.HasPrefix(p.Location, "/login") {
		t.Errorf("the deleted company's admin: GET /app = %d %s", p.Status, p.Location)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(storedFiles(t, e.files)) != 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if files := storedFiles(t, e.files); len(files) != 0 {
		t.Errorf("photos left behind: %v", files)
	}
	if p := stays.get("/app/shipments?q=" + staysTracking); !strings.Contains(p.Body, staysTracking) {
		t.Error("the other company lost its shipment")
	}

	// Money that was on its way is set aside for a refund.
	e.answer(prompt, "approve")
	if !scalar[bool](t, `SELECT status = 'paid' AND refund_due FROM payments WHERE reference = $1`, strings.TrimPrefix(prompt, "/dev/payments/")) {
		t.Error("a payment for the deleted company isn't marked for a refund")
	}
}

// TestSupportTickets: a company asks, the platform answers, and each side is
// emailed when the other writes (AC-049.5).
func TestSupportTickets(t *testing.T) {
	e := newEnv(t)
	op := e.operator()
	office := e.company("Question Cargo")
	help := office.get("/app/help")
	for _, want := range []string{`href="https://t.me/kiflet_support"`, `href="tel:&#43;251110000000"`, "mailto:support@kiflet.test", "How plans are priced"} {
		if !strings.Contains(help.Body, want) {
			t.Errorf("the help page lacks %s", want)
		}
	}
	if strings.Contains(help.Body, "Open</span>") {
		t.Error("the help page shows opening hours that weren't set")
	}
	if p := office.get("/app/help/new?subject=Enterprise+plan"); !strings.Contains(p.Body, `value="Enterprise plan"`) {
		t.Error("the Enterprise link doesn't fill in the subject")
	}
	if p := office.post("/app/help", url.Values{"subject": {"Hello"}, "body": {"  "}}); p.Status != http.StatusBadRequest {
		t.Errorf("an empty question: status %d, want 400", p.Status)
	}
	p := office.mustPost("/app/help", url.Values{"subject": {"Enterprise plan"}, "body": {"We run 60 trucks."}})
	ticket := strings.TrimPrefix(p.Location, "/app/help/")
	status := func() string { return scalar[string](t, `SELECT status FROM support_tickets WHERE id = $1`, ticket) }

	if body := e.received("email", op.email, "We run 60 trucks.").Body; !strings.Contains(body, "/app/platform/support/"+ticket) {
		t.Errorf("the platform's email has no link to the ticket:\n%s", body)
	}
	if p := op.get("/app/platform/support"); !strings.Contains(p.Body, "Question Cargo") || !strings.Contains(p.Body, "Enterprise plan") {
		t.Error("the ticket isn't in the platform's inbox")
	}
	if p := op.get("/app/platform/tenants/" + office.tenant()); !strings.Contains(p.Body, "/app/platform/support/"+ticket) {
		t.Error("the ticket isn't on the company's page")
	}
	if p := e.company("Nosy Neighbours").get("/app/help/" + ticket); p.Status != http.StatusNotFound {
		t.Errorf("another company opening the ticket: status %d, want 404", p.Status)
	}
	if p := office.get("/app/platform/support/" + ticket); p.Status != http.StatusForbidden {
		t.Errorf("a company opening the platform's view: status %d, want 403", p.Status)
	}

	op.mustPost("/app/platform/support/"+ticket+"/reply", url.Values{"body": {"Enterprise is 25,000 birr a month."}})
	if body := e.received("email", office.email, "25,000 birr").Body; !strings.Contains(body, "/app/help/"+ticket) {
		t.Errorf("the company's email has no link to the ticket:\n%s", body)
	}
	if s := status(); s != "answered" {
		t.Errorf("after the answer: %s", s)
	}
	office.mustPost("/app/help/"+ticket+"/reply", url.Values{"body": {"Thanks, we'll take it."}})
	e.received("email", op.email, "we'll take it")
	if s := status(); s != "open" {
		t.Errorf("after the company's reply: %s", s)
	}
	office.mustPost("/app/help/"+ticket+"/close", nil)
	if s := status(); s != "closed" {
		t.Errorf("after closing: %s", s)
	}
	office.mustPost("/app/help/"+ticket+"/reply", url.Values{"body": {"One more thing."}})
	if s := status(); s != "open" {
		t.Errorf("writing to a closed ticket: %s, want it reopened", s)
	}
	page := office.get("/app/help/" + ticket).Body
	for _, text := range []string{"We run 60 trucks.", "25,000 birr", "we&#39;ll take it", "One more thing."} {
		if !strings.Contains(page, text) {
			t.Errorf("the conversation is missing %q", text)
		}
	}
}

// TestPlatformAuditAndHealth: the platform admin searches every company's
// audit log and sees how the jobs are running (AC-042.7, AC-049.6).
func TestPlatformAuditAndHealth(t *testing.T) {
	e := newEnv(t)
	op := e.operator()
	office := e.company("Audited Movers")
	e.browser().post("/login", url.Values{"identifier": {office.email}, "password": {"not-the-password"}})
	tenant := office.tenant()

	// The entries, without the filter form's list of actions.
	entries := func(query string) string {
		t.Helper()
		p := op.get("/app/platform/audit?company=" + tenant + query)
		if i := strings.Index(p.Body, "<tbody>"); p.Status == http.StatusOK && i >= 0 {
			return p.Body[i:]
		}
		t.Fatalf("platform audit%s: status %d", query, p.Status)
		return ""
	}
	if rows := entries(""); !strings.Contains(rows, "Failed sign-in") || !strings.Contains(rows, "Created the company") {
		t.Error("the company's audit log is missing entries")
	}
	if rows := entries("&action=login_failed"); !strings.Contains(rows, "Failed sign-in") || strings.Contains(rows, "Created the company") {
		t.Error("filtering by action")
	}
	if rows := entries("&ip=127.0.0.1"); !strings.Contains(rows, "Failed sign-in") {
		t.Error("filtering by address")
	}
	if rows := entries("&ip=10.9.9.9"); !strings.Contains(rows, "Nothing matches") {
		t.Error("an address with no entries still lists some")
	}
	if rows := entries("&from=" + time.Now().AddDate(0, 0, 1).Format("2006-01-02")); !strings.Contains(rows, "Nothing matches") {
		t.Error("a date range with no entries still lists some")
	}
	if p := office.get("/app/platform/audit"); p.Status != http.StatusForbidden {
		t.Errorf("a company admin opening the platform audit: status %d, want 403", p.Status)
	}

	started := time.Now()
	ctx, stop := context.WithCancel(context.Background())
	go e.svc.Jobs.Run(ctx)
	// The first round runs every job; retention is the last of them.
	deadline := time.Now().Add(15 * time.Second)
	for scalar[int64](t, `SELECT COUNT(*) FROM job_runs WHERE started_at >= $1 AND name = 'retention'`, started) < 1 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	stop()
	p := op.get("/app/platform/health")
	if p.Status != http.StatusOK || !strings.Contains(p.Body, "Data retention") || !strings.Contains(p.Body, "Up to date") {
		t.Fatalf("health page: status %d\n%s", p.Status, errorText(p.Body))
	}
	if strings.Contains(p.Body, "Not running") || strings.Contains(p.Body, "Failing") {
		t.Errorf("a job isn't healthy:\n%s", p.Body[strings.Index(p.Body, "Background jobs"):])
	}
	if p := office.get("/app/platform/health"); p.Status != http.StatusForbidden {
		t.Errorf("a company admin opening the health page: status %d, want 403", p.Status)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// download posts to path and returns the raw response, for files.
func (b *browser) download(path string) (*http.Response, []byte) {
	b.e.t.Helper()
	req, _ := http.NewRequest(http.MethodPost, b.e.srv.URL+path, nil)
	res, err := b.c.Do(req)
	if err != nil {
		b.e.t.Fatalf("POST %s: %v", path, err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res, body
}

// dispatcher adds a dispatcher to the admin's company and signs them in.
func (b *browser) dispatcher() *browser {
	b.e.t.Helper()
	email := "dispatch" + phoneNumber() + "@example.et"
	p := b.post("/app/team", url.Values{"full_name": {"Dawit Bekele"}, "email": {email}, "phone": {phoneNumber()}, "role": {"dispatcher"}})
	m := secretRe.FindStringSubmatch(p.Body)
	if m == nil {
		b.e.t.Fatalf("add dispatcher: status %d %s", p.Status, errorText(p.Body))
	}
	d := b.e.browser()
	d.mustPost("/login", url.Values{"identifier": {email}, "password": {strings.TrimSpace(m[1])}})
	d.mustPost("/app/account/password", url.Values{
		"current_password": {strings.TrimSpace(m[1])}, "new_password": {"dispatch-pass-9"}, "confirm_password": {"dispatch-pass-9"},
	})
	return d
}

// photoJPEG is a small photo as a data URL, as the driver app sends it.
func photoJPEG(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func readCSV(t *testing.T, b []byte) [][]string {
	t.Helper()
	rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

// TestPlatformAdminStaysOnPlatform: the platform admin's own company is
// empty; its pages send them to the companies list.
func TestPlatformAdminStaysOnPlatform(t *testing.T) {
	e := newEnv(t)
	admin := e.company("Platform Operator")
	exec(t, `UPDATE users SET role = 'platform_admin' WHERE tenant_id = (SELECT tenant_id FROM users WHERE full_name = 'Owner of Platform Operator' LIMIT 1)`)

	for _, path := range []string{"/app", "/app/shipments", "/app/settings"} {
		if p := admin.get(path); p.Status != http.StatusSeeOther || p.Location != "/app/platform" {
			t.Errorf("GET %s: %d → %q, want a redirect to /app/platform", path, p.Status, p.Location)
		}
	}
	if p := admin.post("/app/shipments", url.Values{}); p.Status != http.StatusForbidden {
		t.Errorf("POST /app/shipments: status %d, want 403", p.Status)
	}
	if p := admin.get("/app/platform"); p.Status != http.StatusOK {
		t.Errorf("platform page: status %d", p.Status)
	}
	if p := admin.get("/app/account"); p.Status != http.StatusOK {
		t.Errorf("account page: status %d", p.Status)
	}
}
