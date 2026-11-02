//go:build integration

package integration

import (
	"context"
	"fmt"
	"html"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/pkg/payments"
)

// TestTrialEndMakesTheAccountReadOnly: once the free trial is over nothing
// new starts until a plan is chosen, but what is on the road finishes and
// everything can still be seen.
func TestTrialEndMakesTheAccountReadOnly(t *testing.T) {
	e := newEnv(t)
	office := e.company("Lapsed Trial Transport")
	driverID, phone, pin := office.addDriver("Abebe Kebede")
	onRoad, _ := office.book("Addis Ababa", "Adama", driverID)
	waiting, _ := office.book("Addis Ababa", "Dire Dawa", "")
	d := e.signIn(phone, pin)
	if r := d.status(clientID(), onRoad, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}

	exec(t, `UPDATE tenants SET trial_ends_at = NOW() - INTERVAL '1 minute' WHERE id = $1`, office.tenant())
	if !strings.Contains(office.get("/app").Body, "Your free trial has ended") {
		t.Error("the dashboard doesn't say the trial has ended")
	}

	refused := func(what string, p page) {
		t.Helper()
		if p.Status != http.StatusPaymentRequired || !strings.Contains(p.Body, "read-only") {
			t.Errorf("%s after the trial: status %d %s", what, p.Status, errorText(p.Body))
		}
	}
	refused("booking", office.post("/app/shipments", url.Values{
		"priority": {"normal"}, "origin_city": {"Addis Ababa"}, "origin_address": {"Main depot"},
		"destination_city": {"Hawassa"}, "destination_address": {"Customer warehouse"},
		"customer_name": {"Sidama Coffee"}, "customer_phone": {phoneNumber()},
	}))
	refused("adding a driver", office.post("/app/drivers", url.Values{"full_name": {"Late Driver"}, "phone": {phoneNumber()}}))
	refused("adding a vehicle", office.post("/app/vehicles", url.Values{"plate_number": {"3-A12345"}, "vehicle_type": {"heavy_truck"}, "active": {"on"}}))
	refused("inviting staff", office.post("/app/team", url.Values{
		"full_name": {"Late Dispatcher"}, "phone": {phoneNumber()}, "email": {fmt.Sprintf("late%d@example.et", rand.IntN(1e9))}, "role": {"dispatcher"},
	}))
	if msg, failed := office.result("/app/shipments/"+waiting+"/assign", url.Values{"driver_id": {driverID}}); !failed || !strings.Contains(msg, "read-only") {
		t.Errorf("assigning a waiting shipment after the trial: %q", msg)
	}

	// The load on the road is delivered as usual.
	if r := d.status(clientID(), onRoad, "arrived"); !r.OK {
		t.Fatalf("arrive: %+v", r)
	}
	if r := d.pod(service.SyncPOD{ShipmentID: onRoad, RecipientName: "Meseret Tadesse"}); !r.OK {
		t.Fatalf("proof of delivery after the trial: %+v", r)
	}
	if st := scalar[string](t, `SELECT status FROM shipments WHERE id = $1`, onRoad); st != "delivered" {
		t.Errorf("the load on the road is %s, want delivered", st)
	}
	for _, path := range []string{"/app/shipments", "/app/reports", "/app/billing"} {
		if p := office.get(path); p.Status != http.StatusOK {
			t.Errorf("GET %s while read-only: status %d", path, p.Status)
		}
	}
}

// TestPayingForAPlan: a company on its trial chooses a plan, pays by mobile
// money with the prompt printed on the terminal, and its plan starts when
// the trial ends. The provider's notifications are checked and repeats
// change nothing.
func TestPayingForAPlan(t *testing.T) {
	e := newEnv(t)
	office := e.company("Paying Freight")
	tenant := office.tenant()
	if p := office.get("/app/billing"); p.Status != http.StatusOK || !strings.Contains(p.Body, "Business") {
		t.Fatalf("billing page: status %d", p.Status)
	}

	invoiceID := office.choose("business", "monthly")
	if invoiceID == "" {
		t.Fatal("choosing a plan on a trial should issue an invoice")
	}
	number := scalar[string](t, `SELECT number FROM invoices WHERE id = $1`, invoiceID)
	if sub, vat, total := scalar[int64](t, `SELECT subtotal_santim FROM invoices WHERE id = $1`, invoiceID),
		scalar[int64](t, `SELECT vat_santim FROM invoices WHERE id = $1`, invoiceID),
		scalar[int64](t, `SELECT total_santim FROM invoices WHERE id = $1`, invoiceID); sub != 900000 || vat != 135000 || total != 1035000 {
		t.Errorf("invoice is %d + %d VAT = %d santim, want 900000 + 135000 = 1035000", sub, vat, total)
	}
	e.received("email", office.email, number)
	// Choosing the same again is the same invoice.
	if again := office.choose("business", "monthly"); again != invoiceID {
		t.Errorf("choosing the same plan again issued another invoice")
	}

	// Receipts go by email, so the address must be confirmed first.
	if msg, failed := office.result("/app/billing/invoices/"+invoiceID+"/pay", url.Values{"provider": {"telebirr"}, "phone": {office.phone}}); !failed || !strings.Contains(msg, "Confirm your email") {
		t.Errorf("paying before confirming the email: %q", msg)
	}
	office.confirmEmail()

	// The payer declines the first prompt.
	status, prompt := office.startPayment(invoiceID)
	if p := office.get(status); !strings.Contains(p.Body, "Approve the payment on your phone") {
		t.Errorf("the payment page doesn't wait for the phone: status %d", p.Status)
	}
	if p := e.browser().get(prompt); p.Status != http.StatusOK || !strings.Contains(p.Body, "10,350.00") || !strings.Contains(p.Body, number) {
		t.Errorf("the stand-in prompt: status %d", p.Status)
	}
	e.answer(prompt, "decline")
	if p := office.get(status); !strings.Contains(p.Body, "didn't go through") {
		t.Error("a declined payment isn't reported as such")
	}
	if st := scalar[string](t, `SELECT status FROM invoices WHERE id = $1`, invoiceID); st != "open" {
		t.Fatalf("after a declined payment the invoice is %s", st)
	}

	// The second is answered by the provider's notification, checked first.
	status, prompt = office.startPayment(invoiceID)
	reference := strings.TrimPrefix(prompt, "/dev/payments/")
	n := payments.Notification{Reference: reference, Status: payments.StatusPaid, Amount: 1035000}
	if code := e.notify("telebirr", n, "forged"); code != http.StatusUnauthorized {
		t.Errorf("an unsigned notification: status %d, want 401", code)
	}
	short := n
	short.Amount = 1000
	if code := e.notify("telebirr", short, e.sign("telebirr", short)); code != http.StatusBadRequest {
		t.Errorf("a notification for the wrong amount: status %d, want 400", code)
	}
	unknown := payments.Notification{Reference: "PAY-NOSUCHPAYMENT", Status: payments.StatusPaid, Amount: 1035000}
	if code := e.notify("telebirr", unknown, e.sign("telebirr", unknown)); code != http.StatusNotFound {
		t.Errorf("a notification for an unknown payment: status %d, want 404", code)
	}
	if code := e.notify("cbebirr", n, e.sign("cbebirr", n)); code != http.StatusBadRequest {
		t.Errorf("another provider's notification: status %d, want 400", code)
	}
	if st := scalar[string](t, `SELECT status FROM payments WHERE reference = $1`, reference); st != "pending" {
		t.Fatalf("refused notifications changed the payment to %s", st)
	}
	for range 2 {
		if code := e.notify("telebirr", n, e.sign("telebirr", n)); code != http.StatusOK {
			t.Fatalf("the provider's notification: status %d", code)
		}
	}
	if p := office.get(status); !strings.Contains(p.Body, "Paid. Thank you") {
		t.Error("the payment page doesn't say it's paid")
	}
	e.received("email", office.email, "Receipt for invoice "+number)

	// The plan starts when the trial ends, with the plan's limits from now.
	if plan := scalar[string](t, `SELECT plan FROM tenants WHERE id = $1`, tenant); plan != "business" {
		t.Errorf("plan is %s, want business", plan)
	}
	if n := scalar[int32](t, `SELECT max_drivers FROM tenants WHERE id = $1`, tenant); n != 50 {
		t.Errorf("%d driver places, want 50", n)
	}
	if !scalar[bool](t, `SELECT period_start = trial_ends_at AND period_end > period_start + INTERVAL '27 days' FROM tenants WHERE id = $1`, tenant) {
		t.Error("the first period should start when the trial ends and last a month")
	}
	if !scalar[bool](t, `SELECT status = 'paid' AND period_start IS NOT NULL FROM invoices WHERE id = $1`, invoiceID) {
		t.Error("the invoice isn't paid, with its period")
	}
	// Repeats and the payer answering again change nothing.
	e.answer(prompt, "approve")
	if c := scalar[int64](t, `SELECT COUNT(*) FROM payments WHERE invoice_id = $1 AND status = 'paid'`, invoiceID); c != 1 {
		t.Errorf("%d paid payments, want 1", c)
	}
	if c := scalar[int64](t, `SELECT COUNT(*) FROM emails WHERE to_address = $1 AND purpose = 'receipt'`, office.email); c != 1 {
		t.Errorf("%d receipts, want 1", c)
	}
	if msg, failed := office.result("/app/billing/invoices/"+invoiceID+"/pay", url.Values{"provider": {"telebirr"}, "phone": {office.phone}}); !failed || !strings.Contains(msg, "already paid") {
		t.Errorf("paying a paid invoice: %q", msg)
	}
}

// TestRenewalOverdueAndBankTransfer: each period's invoice is issued when
// the last one ends. Unpaid a week after its due date, it makes the company
// read-only; a bank transfer recorded by the operator restores it, and the
// period starts again from the payment.
func TestRenewalOverdueAndBankTransfer(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	operator := e.operator()
	office := e.subscribed("Renewing Haulage", "starter", operator)
	tenant := office.tenant()

	exec(t, `UPDATE tenants SET period_start = NOW() - INTERVAL '1 month' - INTERVAL '1 minute', period_end = NOW() - INTERVAL '1 minute' WHERE id = $1`, tenant)
	for range 2 {
		if err := e.svc.Billing.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if c := scalar[int64](t, `SELECT COUNT(*) FROM invoices WHERE tenant_id = $1 AND kind = 'renewal'`, tenant); c != 1 {
		t.Fatalf("%d renewal invoices, want 1", c)
	}
	renewal := scalar[string](t, `SELECT id::text FROM invoices WHERE tenant_id = $1 AND kind = 'renewal'`, tenant)
	number := scalar[string](t, `SELECT number FROM invoices WHERE id = $1`, renewal)
	if !scalar[bool](t, `SELECT i.period_start = t.period_start AND i.period_end = t.period_end AND t.period_end > NOW() + INTERVAL '27 days'
		FROM invoices i JOIN tenants t ON t.id = i.tenant_id WHERE i.id = $1`, renewal) {
		t.Error("the renewal should cover the next month, which has begun")
	}
	if total := scalar[int64](t, `SELECT total_santim FROM invoices WHERE id = $1`, renewal); total != 287500 {
		t.Errorf("the renewal is %d santim, want 287500", total)
	}
	e.received("email", office.email, "renews")
	if !strings.Contains(office.get("/app").Body, "Invoice "+number+" is due on") {
		t.Error("the dashboard doesn't mention the invoice that is due")
	}

	// Overdue: a reminder, and a warning, but nothing is blocked yet.
	exec(t, `UPDATE invoices SET due_at = NOW() - INTERVAL '1 day' WHERE id = $1`, renewal)
	if err := e.svc.Billing.Run(ctx); err != nil {
		t.Fatal(err)
	}
	e.received("email", office.email, "Invoice "+number+" is overdue")
	if !strings.Contains(office.get("/app/shipments").Body, "is overdue") {
		t.Error("no warning about the overdue invoice")
	}
	driverID, _, _ := office.addDriver("Grace Driver")
	office.book("Addis Ababa", "Adama", driverID)

	// A week later the account is read-only.
	exec(t, `UPDATE invoices SET due_at = NOW() - INTERVAL '8 days' WHERE id = $1`, renewal)
	for range 2 {
		if err := e.svc.Billing.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}
	e.received("email", office.email, "Your account is read-only")
	if c := scalar[int64](t, `SELECT COUNT(*) FROM billing_notices WHERE tenant_id = $1`, tenant); c != 2 {
		t.Errorf("%d billing notices, want 2 (overdue, read-only)", c)
	}
	if p := office.post("/app/drivers", url.Values{"full_name": {"Blocked Driver"}, "phone": {phoneNumber()}}); p.Status != http.StatusPaymentRequired || !strings.Contains(p.Body, number) {
		t.Errorf("adding a driver while an invoice is long overdue: status %d %s", p.Status, errorText(p.Body))
	}
	if p := operator.get("/app/platform/billing"); p.Status != http.StatusOK || !strings.Contains(p.Body, number) {
		t.Errorf("the operator's billing page doesn't list the unpaid invoice: status %d", p.Status)
	}

	// A new plan waits for the debt; the billing cycle can still change.
	if !strings.Contains(office.get("/app/billing").Body, "Pay invoice "+number+" first.") {
		t.Error("the billing page offers plan changes the server would refuse")
	}
	if msg, failed := office.result("/app/billing/plan", url.Values{"plan": {"business"}, "cycle": {"monthly"}}); !failed || !strings.Contains(msg, "Pay invoice "+number+" first") {
		t.Errorf("upgrading with an unpaid invoice: %q", msg)
	}
	if msg, failed := office.result("/app/billing/plan", url.Values{"plan": {"starter"}, "cycle": {"annual"}}); failed || !strings.Contains(msg, "billed annually") {
		t.Errorf("switching to annual billing with an unpaid invoice: %q", msg)
	}
	if cycle := scalar[string](t, `SELECT COALESCE(pending_cycle, '') FROM tenants WHERE id = $1`, tenant); cycle != "annual" {
		t.Errorf("the switch to annual billing wasn't kept: pending %q", cycle)
	}
	if msg, failed := office.result("/app/billing/plan", url.Values{"plan": {"starter"}, "cycle": {"monthly"}}); failed || !strings.Contains(msg, "stay on the Starter plan") {
		t.Errorf("keeping things as they are: %q", msg)
	}

	// The transfer arrives; the operator records it.
	if msg, failed := operator.result("/app/platform/invoices/"+renewal+"/paid", url.Values{"reference": {""}}); !failed {
		t.Errorf("a transfer without a reference was recorded: %q", msg)
	}
	ref := fmt.Sprintf("ft %d", rand.IntN(1e9))
	operator.markPaid(renewal, ref)
	if got := scalar[string](t, `SELECT reference FROM payments WHERE invoice_id = $1`, renewal); got != "BANK-"+strings.ToUpper(strings.ReplaceAll(ref, " ", "")) {
		t.Errorf("the transfer is recorded as %q", got)
	}
	if !scalar[bool](t, `SELECT period_start > NOW() - INTERVAL '1 minute' FROM tenants WHERE id = $1`, tenant) {
		t.Error("a renewal paid after read-only began should start the period again from the payment")
	}
	e.received("email", office.email, "Receipt for invoice "+number)
	office.addDriver("Welcome Back Driver")
	if strings.Contains(office.get("/app").Body, "read-only") {
		t.Error("still read-only after paying")
	}
}

// TestRenewalAfterSuspension: a company suspended past the end of its
// period isn't billed for the time it was suspended.
func TestRenewalAfterSuspension(t *testing.T) {
	e := newEnv(t)
	operator := e.operator()
	office := e.subscribed("Suspended Lines", "starter", operator)
	tenant := office.tenant()
	exec(t, `UPDATE tenants SET is_active = false, period_start = NOW() - INTERVAL '3 months', period_end = NOW() - INTERVAL '2 months' WHERE id = $1`, tenant)
	if err := e.svc.Billing.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c := scalar[int64](t, `SELECT COUNT(*) FROM invoices WHERE tenant_id = $1 AND kind = 'renewal'`, tenant); c != 0 {
		t.Fatalf("a suspended company was invoiced")
	}
	operator.mustPost("/app/platform/tenants/"+tenant+"/active", url.Values{"active": {"true"}})
	if err := e.svc.Billing.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !scalar[bool](t, `SELECT period_start > NOW() - INTERVAL '1 minute' AND period_end > NOW() + INTERVAL '27 days' FROM invoices WHERE tenant_id = $1 AND kind = 'renewal'`, tenant) {
		t.Error("the period after a suspension should start when the company is back")
	}
}

// TestUpgradeIsProrated: moving up a plan costs the difference for what is
// left of the period, and applies once paid.
func TestUpgradeIsProrated(t *testing.T) {
	e := newEnv(t)
	operator := e.operator()
	office := e.subscribed("Growing Freight", "starter", operator)
	tenant := office.tenant()
	exec(t, `UPDATE tenants SET period_start = NOW() - INTERVAL '15 days', period_end = NOW() + INTERVAL '15 days' WHERE id = $1`, tenant)

	invoiceID := office.choose("business", "monthly")
	if invoiceID == "" {
		t.Fatal("an upgrade should issue an invoice")
	}
	// Half of (9,000 − 2,500) birr, give or take the seconds the test takes.
	if sub := scalar[int64](t, `SELECT subtotal_santim FROM invoices WHERE id = $1 AND kind = 'upgrade'`, invoiceID); sub < 324900 || sub > 325000 {
		t.Errorf("the upgrade costs %d santim, want about 325000", sub)
	}
	if n := scalar[int32](t, `SELECT max_drivers FROM tenants WHERE id = $1`, tenant); n != 10 {
		t.Errorf("the upgrade applied before it was paid: %d driver places", n)
	}
	periodEnd := scalar[time.Time](t, `SELECT period_end FROM tenants WHERE id = $1`, tenant)

	operator.markPaid(invoiceID, fmt.Sprintf("UP%d", rand.IntN(1e9)))
	if plan, n := scalar[string](t, `SELECT plan FROM tenants WHERE id = $1`, tenant), scalar[int32](t, `SELECT max_drivers FROM tenants WHERE id = $1`, tenant); plan != "business" || n != 50 {
		t.Errorf("after paying the upgrade: plan %s with %d drivers", plan, n)
	}
	if got := scalar[time.Time](t, `SELECT period_end FROM tenants WHERE id = $1`, tenant); !got.Equal(periodEnd) {
		t.Errorf("an upgrade moved the end of the period from %s to %s", periodEnd, got)
	}
	if msg, failed := office.result("/app/billing/plan", url.Values{"plan": {"business"}, "cycle": {"monthly"}}); !failed || !strings.Contains(msg, "already on this plan") {
		t.Errorf("choosing the current plan: %q", msg)
	}
}

// TestDowngradeWaitsForTheEndOfThePeriod: a smaller plan starts when the
// paid period ends, and only if the active drivers fit in it.
func TestDowngradeWaitsForTheEndOfThePeriod(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	operator := e.operator()
	office := e.subscribed("Shrinking Fleet", "business", operator)
	tenant := office.tenant()
	var last string
	for i := range 11 {
		last, _, _ = office.addDriver(fmt.Sprintf("Driver %d", i+1))
	}

	if msg, failed := office.result("/app/billing/plan", url.Values{"plan": {"starter"}, "cycle": {"monthly"}}); !failed || !strings.Contains(msg, "Deactivate 1") {
		t.Errorf("downgrading with too many drivers: %q", msg)
	}
	office.mustPost("/app/drivers/"+last+"/active", url.Values{"active": {"false"}})
	if msg, failed := office.result("/app/billing/plan", url.Values{"plan": {"starter"}, "cycle": {"monthly"}}); failed || !strings.Contains(msg, "changes to Starter") {
		t.Fatalf("downgrading: %q", msg)
	}
	e.received("email", office.email, "Your plan changes on")
	if plan, pending := scalar[string](t, `SELECT plan FROM tenants WHERE id = $1`, tenant), scalar[string](t, `SELECT pending_plan FROM tenants WHERE id = $1`, tenant); plan != "business" || pending != "starter" {
		t.Errorf("after asking to downgrade: plan %s, pending %s", plan, pending)
	}

	// The driver comes back before the period ends: the change can't happen.
	office.mustPost("/app/drivers/"+last+"/active", url.Values{"active": {"true"}})
	endPeriod := func() {
		t.Helper()
		exec(t, `UPDATE tenants SET period_start = period_start - (period_end - NOW()) - INTERVAL '1 minute', period_end = NOW() - INTERVAL '1 minute' WHERE id = $1`, tenant)
		if err := e.svc.Billing.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}
	endPeriod()
	e.received("email", office.email, "change to the Starter plan was cancelled")
	if plan := scalar[string](t, `SELECT plan || '/' || COALESCE(pending_plan, '') FROM tenants WHERE id = $1`, tenant); plan != "business/" {
		t.Errorf("after a downgrade that didn't fit: %s", plan)
	}
	renewal := scalar[string](t, `SELECT id::text FROM invoices WHERE tenant_id = $1 AND kind = 'renewal' ORDER BY issued_at DESC LIMIT 1`, tenant)
	if amount := scalar[int64](t, `SELECT subtotal_santim FROM invoices WHERE id = $1`, renewal); amount != 900000 {
		t.Errorf("the renewal is for %d santim, want Business's 900000", amount)
	}
	operator.markPaid(renewal, fmt.Sprintf("RN%d", rand.IntN(1e9)))

	office.mustPost("/app/drivers/"+last+"/active", url.Values{"active": {"false"}})
	if _, failed := office.result("/app/billing/plan", url.Values{"plan": {"starter"}, "cycle": {"annual"}}); failed {
		t.Fatal("downgrading again failed")
	}
	endPeriod()
	if plan := scalar[string](t, `SELECT plan || '/' || billing_cycle || '/' || max_drivers FROM tenants WHERE id = $1`, tenant); plan != "starter/annual/10" {
		t.Errorf("after the period ended: %s, want starter/annual/10", plan)
	}
	if amount := scalar[int64](t, `SELECT subtotal_santim FROM invoices WHERE tenant_id = $1 AND kind = 'renewal' ORDER BY issued_at DESC LIMIT 1`, tenant); amount != 2500000 {
		t.Errorf("the renewal is for %d santim, want a Starter year's 2500000", amount)
	}
}

// TestTextOverageIsInvoiced: texts beyond the month's allowance are billed
// once the month is over, and the invoice keeps the counts.
func TestTextOverageIsInvoiced(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	office := e.subscribed("Chatty Couriers", "starter", e.operator())
	tenant := office.tenant()

	tz, _ := time.LoadLocation("Africa/Addis_Ababa")
	now := time.Now().In(tz)
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, tz)
	lastMonth := thisMonth.AddDate(0, -1, 0)
	exec(t, `UPDATE tenants SET subscribed_at = $2, sms_billed_month = $3 WHERE id = $1`, tenant, lastMonth.AddDate(0, -1, 0), lastMonth.Format("2006-01-02"))
	exec(t, `INSERT INTO sms_messages (tenant_id, direction, phone, body, purpose, status, sent_at)
		SELECT $1, 'outbound', '0911000000', 'Your shipment is on its way', 'customer_status', 'sent', $2::timestamptz + (n * INTERVAL '1 minute')
		FROM generate_series(1, 520) n`, tenant, lastMonth.Add(9*24*time.Hour))
	// Texts that failed, and this month's, aren't counted.
	exec(t, `INSERT INTO sms_messages (tenant_id, direction, phone, body, purpose, status, sent_at)
		SELECT $1, 'outbound', '0911000000', 'x', 'customer_status', s, $2::timestamptz FROM unnest(ARRAY['failed', 'sent']) s`, tenant, thisMonth.Add(time.Minute))

	for range 2 {
		if err := e.svc.Billing.BillUsage(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if c := scalar[int64](t, `SELECT COUNT(*) FROM invoices WHERE tenant_id = $1 AND kind = 'usage'`, tenant); c != 1 {
		t.Fatalf("%d usage invoices, want 1", c)
	}
	inv := scalar[string](t, `SELECT id::text FROM invoices WHERE tenant_id = $1 AND kind = 'usage'`, tenant)
	if sub, total := scalar[int64](t, `SELECT subtotal_santim FROM invoices WHERE id = $1`, inv), scalar[int64](t, `SELECT total_santim FROM invoices WHERE id = $1`, inv); sub != 1000 || total != 1150 {
		t.Errorf("20 extra texts cost %d santim, %d with VAT; want 1000 and 1150", sub, total)
	}
	if usage := scalar[string](t, `SELECT usage::text FROM invoices WHERE id = $1`, inv); !strings.Contains(usage, `"sent": 520`) || !strings.Contains(usage, `"extra": 20`) {
		t.Errorf("the invoice keeps %s", usage)
	}
	if got := scalar[time.Time](t, `SELECT sms_billed_month::timestamp FROM tenants WHERE id = $1`, tenant); got.Format("2006-01") != thisMonth.Format("2006-01") {
		t.Errorf("metering %s, want %s", got.Format("2006-01"), thisMonth.Format("2006-01"))
	}
	if p := office.get("/app/billing/invoices/" + inv); !strings.Contains(p.Body, "Text messages sent in "+lastMonth.Format("January 2006")+": 520") {
		t.Errorf("the invoice page doesn't show the counts: status %d", p.Status)
	}
}

// TestUsageAlerts: the office hears once when 80% of its driver places or
// of the month's texts are used.
func TestUsageAlerts(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	office := e.company("Busy Trial Logistics")
	tenant := office.tenant()
	for i := range 8 {
		office.addDriver(fmt.Sprintf("Driver %d", i+1))
	}
	exec(t, `INSERT INTO sms_messages (tenant_id, direction, phone, body, purpose, status, sent_at)
		SELECT $1, 'outbound', '0911000000', 'x', 'customer_status', 'sent', NOW() - INTERVAL '1 minute' FROM generate_series(1, 400)`, tenant)

	count := func(kind string) int64 {
		return scalar[int64](t, `SELECT COUNT(*) FROM alerts WHERE tenant_id = $1 AND alert_type = $2`, tenant, kind)
	}
	for range 2 {
		if err := e.svc.Billing.CheckUsage(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if count("driver_limit_near") != 1 || count("sms_quota_near") != 1 {
		t.Fatalf("%d driver and %d text alerts, want one each", count("driver_limit_near"), count("sms_quota_near"))
	}
	exec(t, `UPDATE alerts SET acknowledged_at = NOW() WHERE tenant_id = $1`, tenant)
	if err := e.svc.Billing.CheckUsage(ctx); err != nil {
		t.Fatal(err)
	}
	if count("driver_limit_near") != 1 || count("sms_quota_near") != 1 {
		t.Errorf("dismissed usage alerts came back")
	}
	msg := scalar[string](t, `SELECT message FROM alerts WHERE tenant_id = $1 AND alert_type = 'driver_limit_near'`, tenant)
	if !strings.Contains(msg, "8 of the 10 driver places") {
		t.Errorf("driver alert says %q", msg)
	}
}

// TestTrialNotices: a company hears three days and one day before its trial
// ends, and when it has, once each; the operator can extend it.
func TestTrialNotices(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	operator := e.operator()
	office := e.company("Deciding Transport")
	tenant := office.tenant()
	remind := func(trialEnds string) {
		t.Helper()
		exec(t, `UPDATE tenants SET trial_ends_at = NOW() + $2::interval WHERE id = $1`, tenant, trialEnds)
		for range 2 {
			if err := e.svc.Billing.Remind(ctx); err != nil {
				t.Fatal(err)
			}
		}
	}
	notices := func() int64 {
		return scalar[int64](t, `SELECT COUNT(*) FROM emails WHERE to_address = $1 AND subject LIKE 'Your free trial%'`, office.email)
	}

	remind("2 days 20 hours")
	e.received("email", office.email, "Your free trial ends on")
	remind("20 hours")
	e.received("email", office.email, "Your free trial ends to")
	if !strings.Contains(office.get("/app").Body, "Your free trial ends") {
		t.Error("no banner on the trial's last day")
	}
	remind("-1 minute")
	e.received("email", office.email, "Your free trial has ended")
	if c := notices(); c != 3 {
		t.Errorf("%d trial emails, want 3", c)
	}

	operator.mustPost("/app/platform/tenants/"+tenant+"/trial", url.Values{"days": {"7"}})
	e.received("email", office.email, "Your free trial is extended")
	if !scalar[bool](t, `SELECT trial_ends_at BETWEEN NOW() + INTERVAL '6 days 23 hours' AND NOW() + INTERVAL '7 days 1 hour' FROM tenants WHERE id = $1`, tenant) {
		t.Error("an ended trial extended by 7 days should end 7 days from now")
	}
	office.book("Addis Ababa", "Adama", "")
}

// TestEnterpriseTerms: Enterprise can be chosen once the operator has made
// the company an offer, at its price and limits.
func TestEnterpriseTerms(t *testing.T) {
	e := newEnv(t)
	operator := e.operator()
	office := e.company("Big Fleet PLC")
	tenant := office.tenant()
	exec(t, `UPDATE tenants SET trial_ends_at = NOW() - INTERVAL '1 minute' WHERE id = $1`, tenant)

	if msg, failed := office.result("/app/billing/plan", url.Values{"plan": {"enterprise"}, "cycle": {"monthly"}}); !failed || !strings.Contains(msg, "support@menged.test") {
		t.Errorf("Enterprise without an offer: %q", msg)
	}
	if msg, failed := operator.result("/app/platform/tenants/"+tenant+"/offer", url.Values{"price": {"30,000"}, "max_drivers": {"120"}, "sms_quota": {"20000"}}); failed {
		t.Fatalf("offering Enterprise: %q", msg)
	}
	e.received("email", office.email, "ETB 30,000.00 a month")

	invoiceID := office.choose("enterprise", "monthly")
	if sub := scalar[int64](t, `SELECT subtotal_santim FROM invoices WHERE id = $1`, invoiceID); sub != 3000000 {
		t.Errorf("the Enterprise invoice is for %d santim, want 3000000", sub)
	}
	operator.markPaid(invoiceID, fmt.Sprintf("ENT%d", rand.IntN(1e9)))
	if got := scalar[string](t, `SELECT plan || '/' || max_drivers || '/' || sms_quota FROM tenants WHERE id = $1`, tenant); got != "enterprise/120/20000" {
		t.Errorf("on Enterprise: %s", got)
	}
	if msg, failed := operator.result("/app/platform/tenants/"+tenant+"/offer", url.Values{"price": {""}}); !failed || !strings.Contains(msg, "is on Enterprise") {
		t.Errorf("withdrawing the terms of a company on them: %q", msg)
	}
}

// TestSignupWithAPaidPlan: a plan chosen at sign-up is invoiced at once,
// and the new admin lands on the invoice.
func TestSignupWithAPaidPlan(t *testing.T) {
	e := newEnv(t)
	b := e.browser()
	ph, email := phoneNumber(), fmt.Sprintf("annual%d@example.et", rand.IntN(1e9))
	b.mustPost("/signup", url.Values{
		"company": {"Annual Express"}, "city": {"Mekelle"}, "name": {"Yared Haile"},
		"email": {email}, "phone": {ph}, "password": {"correct-horse-1"}, "plan": {"business:annual"},
	})
	p := b.mustPost("/signup/verify", url.Values{"code": {e.code(ph)}})
	m := invoicePathRe.FindStringSubmatch(p.Location)
	if m == nil {
		t.Fatalf("after sign-up with a plan, redirected to %q", p.Location)
	}
	if got := scalar[string](t, `SELECT kind || '/' || billing_cycle || '/' || subtotal_santim FROM invoices WHERE id = $1`, m[1]); got != "subscription/annual/9000000" {
		t.Errorf("the sign-up invoice is %s", got)
	}
	if page := b.get(p.Location); !strings.Contains(page.Body, "First confirm your email address") {
		t.Error("the invoice page should ask for the email to be confirmed before paying")
	}
}

// TestPaymentForAPaidInvoiceIsRefunded: money that arrives for an invoice
// already paid is kept on record and flagged until the operator has sent it
// back.
func TestPaymentForAPaidInvoiceIsRefunded(t *testing.T) {
	e := newEnv(t)
	operator := e.operator()
	office := e.company("Double Pay Transport")
	office.confirmEmail()
	invoiceID := office.choose("starter", "monthly")
	_, first := office.startPayment(invoiceID)
	second, again := office.startPayment(invoiceID)
	e.answer(first, "approve")
	e.answer(again, "approve")

	reference := strings.TrimPrefix(again, "/dev/payments/")
	if !scalar[bool](t, `SELECT status = 'paid' AND refund_due FROM payments WHERE reference = $1`, reference) {
		t.Fatal("the second payment isn't flagged for a refund")
	}
	if p := office.get(second); !strings.Contains(p.Body, "will be refunded") {
		t.Error("the payer isn't told the second payment will be refunded")
	}
	if p := operator.get("/app/platform/billing"); !strings.Contains(p.Body, "Refunds to send") || !strings.Contains(p.Body, reference) {
		t.Error("the operator's billing page doesn't list the refund")
	}
	payment := scalar[string](t, `SELECT id::text FROM payments WHERE reference = $1`, reference)
	if msg, failed := operator.result("/app/platform/payments/"+payment+"/refunded", url.Values{"reference": {"TB-REFUND-1"}}); failed {
		t.Fatalf("recording the refund: %q", msg)
	}
	e.received("email", office.email, "Refund of ETB 2,875.00")
	if !scalar[bool](t, `SELECT status = 'refunded' AND NOT refund_due AND refund_reference = 'TB-REFUND-1' FROM payments WHERE id = $1`, payment) {
		t.Error("the payment isn't recorded as refunded")
	}
	if strings.Contains(operator.get("/app/platform/billing").Body, "Refunds to send") {
		t.Error("the refund is still listed")
	}
	if msg, failed := operator.result("/app/platform/payments/"+payment+"/refunded", url.Values{"reference": {"TB-REFUND-2"}}); !failed {
		t.Errorf("refunding twice: %q", msg)
	}
	if p := office.get(second); !strings.Contains(p.Body, "was refunded") {
		t.Error("the payment page doesn't say it was refunded")
	}
}

// ---------------------------------------------------------------------------
// Billing helpers
// ---------------------------------------------------------------------------

var (
	invoicePathRe = regexp.MustCompile(`^/app/billing/invoices/([0-9a-f-]{36})$`)
	promptLinkRe  = regexp.MustCompile(`/dev/payments/PAY-[A-Z2-7]+`)
	anyFlashRe    = regexp.MustCompile(`(?s)class="flash flash-(\w+)"[^>]*>(.*?)<`)
)

// operator signs in someone who runs the service.
func (e *env) operator() *browser {
	e.t.Helper()
	b := e.company("Platform Operations")
	exec(e.t, `UPDATE users SET role = 'platform_admin' WHERE email = $1`, b.email)
	return b
}

// subscribed signs up a company whose trial is over and which has paid for
// plan by bank transfer, so its first month has begun.
func (e *env) subscribed(name, plan string, operator *browser) *browser {
	e.t.Helper()
	b := e.company(name)
	exec(e.t, `UPDATE tenants SET trial_ends_at = NOW() - INTERVAL '1 minute' WHERE id = $1`, b.tenant())
	operator.markPaid(b.choose(plan, "monthly"), fmt.Sprintf("SUB%d", rand.IntN(1e9)))
	return b
}

// tenant is the id of the company the browser's admin works for.
func (b *browser) tenant() string {
	b.e.t.Helper()
	return scalar[string](b.e.t, `SELECT tenant_id::text FROM users WHERE email = $1`, b.email)
}

// confirmEmail opens the link emailed to the admin at sign-up, whichever
// emails the test has read since.
func (b *browser) confirmEmail() {
	b.e.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		for _, it := range b.e.inbox.Items() {
			if link := verifyLinkRe.FindString(it.Body); it.Kind == "email" && it.To == b.email && link != "" {
				if p := b.get(link); p.Location != "/app/account" {
					b.e.t.Fatalf("confirm the email address: status %d, location %q", p.Status, p.Location)
				}
				return
			}
		}
		if time.Now().After(deadline) {
			b.e.t.Fatalf("no confirmation link emailed to %s", b.email)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// result posts a form whose outcome is a message on the page it redirects
// to, and returns the message and whether it is an error.
func (b *browser) result(path string, form url.Values) (msg string, failed bool) {
	b.e.t.Helper()
	p := b.mustPost(path, form)
	next := b.get(p.Location)
	m := anyFlashRe.FindStringSubmatch(next.Body)
	if m == nil {
		b.e.t.Fatalf("POST %s: no message on %s (status %d)", path, p.Location, next.Status)
	}
	return html.UnescapeString(strings.TrimSpace(m[2])), m[1] == "error"
}

// choose picks a plan on the billing page and returns the invoice to pay,
// or "" when the change needs no payment.
func (b *browser) choose(plan, cycle string) string {
	b.e.t.Helper()
	p := b.mustPost("/app/billing/plan", url.Values{"plan": {plan}, "cycle": {cycle}})
	if m := invoicePathRe.FindStringSubmatch(p.Location); m != nil {
		return m[1]
	}
	return ""
}

// startPayment asks for an invoice to be paid by telebirr. It returns the
// page that follows the payment and the link of the prompt printed on the
// terminal, which stands in for the payer's phone.
func (b *browser) startPayment(invoiceID string) (status, prompt string) {
	b.e.t.Helper()
	p := b.mustPost("/app/billing/invoices/"+invoiceID+"/pay", url.Values{"provider": {"telebirr"}, "phone": {b.phone}})
	if !strings.HasPrefix(p.Location, "/app/billing/payments/") {
		b.e.t.Fatalf("paying invoice %s redirected to %q", invoiceID, p.Location)
	}
	prompt = promptLinkRe.FindString(b.e.received("payment", b.phone, "/dev/payments/").Body)
	if prompt == "" {
		b.e.t.Fatal("no payment prompt on the terminal")
	}
	return p.Location, prompt
}

// answer approves or declines a payment prompt as the payer would.
func (e *env) answer(prompt, answer string) {
	e.t.Helper()
	if p := e.browser().post(prompt, url.Values{"answer": {answer}}); p.Status != http.StatusOK {
		e.t.Fatalf("answering %s: status %d", prompt, p.Status)
	}
}

// markPaid records a bank transfer for an invoice, as the operator does.
func (b *browser) markPaid(invoiceID, reference string) {
	b.e.t.Helper()
	if msg, failed := b.result("/app/platform/invoices/"+invoiceID+"/paid", url.Values{"reference": {reference}}); failed {
		b.e.t.Fatalf("recording the transfer for %s: %s", invoiceID, msg)
	}
}

// sign signs a notification as the (simulated) provider does.
func (e *env) sign(provider string, n payments.Notification) string {
	e.t.Helper()
	p, ok := e.svc.Billing.Provider(provider)
	if !ok {
		e.t.Fatalf("no provider %s", provider)
	}
	return p.(*payments.Mock).Sign(n)
}

// notify posts a provider's notification and returns the response status.
func (e *env) notify(provider string, n payments.Notification, signature string) int {
	e.t.Helper()
	res, err := http.PostForm(e.srv.URL+"/api/v1/payments/"+provider+"/notify", url.Values{
		"reference": {n.Reference}, "provider_ref": {n.ProviderRef}, "status": {n.Status},
		"amount": {strconv.FormatInt(n.Amount, 10)}, "signature": {signature},
	})
	if err != nil {
		e.t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}
