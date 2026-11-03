package service

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/internal/phone"
	"github.com/natnael-alemayehu/logistics/pkg/payments"
)

// BillingService runs subscriptions (FR-043 to FR-045): choosing and changing
// plans, invoices, payments, renewals, text overage, and the read-only mode
// that follows an ended trial or an unpaid invoice.
//
// A company pays for a period in advance. An upgrade applies once its
// prorated invoice is paid; a downgrade or a change of billing cycle waits
// for the end of the period. Renewal and usage invoices are debts: unpaid
// GraceDays after they were due, they make the company read-only. Plan
// invoices (subscription and upgrade) are offers: unpaid, they lapse.
type BillingService struct {
	*core
	providers []payments.Provider
}

// Invoice kinds.
const (
	invoiceSubscription = "subscription"
	invoiceUpgrade      = "upgrade"
	invoiceRenewal      = "renewal"
	invoiceUsage        = "usage"
)

// Payment states, and the provider code of a bank transfer recorded by the
// platform operator.
const (
	paymentPending  = "pending"
	paymentPaid     = "paid"
	paymentFailed   = "failed"
	paymentExpired  = "expired"
	paymentRefunded = "refunded"
	providerBank    = "bank"

	// paymentPromptTTL is how long a payment waits for the payer to finish
	// at the provider's checkout. A late payment still counts.
	paymentPromptTTL = 30 * time.Minute

	// A payment the provider hasn't reported on is asked about every
	// paymentCheckEvery until paymentCheckWindow after it started, in case
	// the report was lost.
	paymentCheckEvery  = 2 * time.Minute
	paymentCheckWindow = 2 * time.Hour

	// lateRenewal is how long after a period ended the next one still
	// follows on from it rather than starting when it is renewed.
	lateRenewal = 24 * time.Hour
)

// Billing emails sent once per situation.
const (
	noticeTrial3Days  = "trial_3d"
	noticeTrial1Day   = "trial_1d"
	noticeTrialEnded  = "trial_ended"
	noticeOverdue     = "overdue"
	noticeReadOnly    = "read_only"
	trialNoticeWindow = 3 * 24 * time.Hour
)

// Providers are the mobile-money methods payments can be made with.
func (s *BillingService) Providers() []payments.Provider { return s.providers }

// Provider finds a payment provider by its code.
func (s *BillingService) Provider(code string) (payments.Provider, bool) {
	for _, p := range s.providers {
		if p.Code() == code {
			return p, true
		}
	}
	return nil, false
}

// ---------------------------------------------------------------------------
// Standing
// ---------------------------------------------------------------------------

// Standing states.
const (
	StandingTrialing   = "trialing"
	StandingTrialEnded = "trial_ended"
	StandingActive     = "active"
	StandingDue        = "due"
	StandingPastDue    = "past_due"
	StandingReadOnly   = "read_only"
)

// Standing is where a company is with paying for the product. Two states
// make it read-only: a trial that has ended, and an invoice still unpaid
// GraceDays after it was due.
type Standing struct {
	State string
	// TrialEnds is when a trial ends or ended.
	TrialEnds time.Time
	// DebtNumber and DebtDue are the oldest unpaid renewal or usage
	// invoice, and ReadOnlyAt when it makes the company read-only.
	DebtNumber string
	DebtDue    time.Time
	ReadOnlyAt time.Time
	// DaysLeft counts whole days, rounded up, until the trial ends or, for
	// an overdue invoice, until the company becomes read-only.
	DaysLeft int
}

// ReadOnly reports whether booking and adding to the fleet are blocked.
func (s Standing) ReadOnly() bool {
	return s.State == StandingTrialEnded || s.State == StandingReadOnly
}

func standingOf(plan string, trialEnds *time.Time, debtNumber string, debtDue *time.Time, now time.Time) Standing {
	if plan == model.PlanTrial {
		st := Standing{State: StandingTrialing}
		if trialEnds != nil {
			st.TrialEnds = *trialEnds
			if now.Before(*trialEnds) {
				st.DaysLeft = daysUntil(now, *trialEnds)
			} else {
				st.State = StandingTrialEnded
			}
		}
		return st
	}
	st := Standing{State: StandingActive}
	if debtNumber == "" || debtDue == nil {
		return st
	}
	st.DebtNumber, st.DebtDue = debtNumber, *debtDue
	st.ReadOnlyAt = debtDue.AddDate(0, 0, model.GraceDays)
	switch {
	case now.Before(st.DebtDue):
		st.State = StandingDue
	case now.Before(st.ReadOnlyAt):
		st.State = StandingPastDue
		st.DaysLeft = daysUntil(now, st.ReadOnlyAt)
	default:
		st.State = StandingReadOnly
	}
	return st
}

func daysUntil(now, t time.Time) int {
	const day = 24 * time.Hour
	return int((t.Sub(now) + day - 1) / day)
}

// Standing is where the caller's company is with paying.
func (p *Principal) Standing(now time.Time) Standing {
	return standingOf(p.TenantPlan, p.TrialEnds, p.DebtNumber, p.DebtDue, now)
}

var errTrialEnded = &Error{Kind: KindLimit, Message: "Your company's free trial has ended, so the account is read-only: " +
	"shipments on the road carry on, but booking or assigning shipments and adding drivers, vehicles or staff " +
	"wait until an administrator chooses a plan on the Billing page. Nothing has been deleted."}

// requireWritable refuses what read-only mode blocks: booking and assigning
// shipments, and adding drivers, vehicles and staff. Work already on the road
// is never blocked, and neither is paying.
func (c *core) requireWritable(ctx context.Context, q *db.Queries, t db.Tenant) error {
	var debtNumber string
	var debtDue *time.Time
	if t.Plan != model.PlanTrial {
		debt, err := q.GetOldestDebt(ctx, t.ID)
		switch {
		case err == nil:
			debtNumber, debtDue = debt.Number, &debt.DueAt.Time
		case !isNoRows(err):
			return err
		}
	}
	switch standingOf(t.Plan, timePtr(t.TrialEndsAt), debtNumber, debtDue, c.now()).State {
	case StandingTrialEnded:
		return errTrialEnded
	case StandingReadOnly:
		return limitReached(fmt.Sprintf("Invoice %s is unpaid, so your company's account is read-only: "+
			"shipments on the road carry on, but booking or assigning shipments and adding drivers, vehicles or staff "+
			"wait until it is paid on the Billing page.", debtNumber))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Plans and prices
// ---------------------------------------------------------------------------

// terms are what a plan costs a company and what it allows.
type terms struct {
	plan     model.Plan
	monthly  int64 // santim, excluding VAT
	drivers  int32
	smsQuota int32
}

// planTerms is what plan code would cost t. Enterprise has the price and
// limits the platform operator offered the company, and can't be chosen
// without an offer.
func (c *core) planTerms(t db.Tenant, code string) (terms, error) {
	p, ok := model.PlanByCode(code)
	if !ok || code == model.PlanTrial {
		return terms{}, invalid("plan", "Choose a plan.")
	}
	tm := terms{plan: p, monthly: p.MonthlyETB * 100, drivers: p.MaxDrivers, smsQuota: p.SMSIncluded}
	if code == model.PlanEnterprise {
		if t.EnterprisePriceSantim == nil {
			return terms{}, invalid("plan", "Enterprise is priced for each company. Ask for an offer on the Help page.")
		}
		tm.monthly = *t.EnterprisePriceSantim
		if t.EnterpriseMaxDrivers != nil {
			tm.drivers = *t.EnterpriseMaxDrivers
		}
		if t.EnterpriseSmsQuota != nil {
			tm.smsQuota = *t.EnterpriseSmsQuota
		}
	}
	return tm, nil
}

// monthlyPrice is what a company on plan pays a month, in santim, excluding
// VAT; offer is its Enterprise price.
func monthlyPrice(plan string, offer *int64) int64 {
	if plan == model.PlanEnterprise {
		return deref(offer)
	}
	p, _ := model.PlanByCode(plan)
	return p.MonthlyETB * 100
}

// subscriptionOf is t's subscription as stored, for changing part of it.
func subscriptionOf(t db.Tenant) db.SaveSubscriptionParams {
	return db.SaveSubscriptionParams{
		ID: t.ID, Plan: t.Plan, BillingCycle: t.BillingCycle, MaxDrivers: t.MaxDrivers, SmsQuota: t.SmsQuota,
		PeriodStart: t.PeriodStart, PeriodEnd: t.PeriodEnd, SubscribedAt: t.SubscribedAt,
		PendingPlan: t.PendingPlan, PendingCycle: t.PendingCycle,
	}
}

func withTerms(sub db.SaveSubscriptionParams, tm terms) db.SaveSubscriptionParams {
	sub.Plan, sub.MaxDrivers, sub.SmsQuota = tm.plan.Code, tm.drivers, tm.smsQuota
	return sub
}

// subscribed reports whether t has a paid period, as opposed to a trial (or
// the platform's own account, which never pays).
func subscribed(t db.Tenant) bool { return t.Plan != model.PlanTrial && t.PeriodEnd.Valid }

// PlanOption is a plan as the billing page offers it.
type PlanOption struct {
	Code        string
	Name        string
	Monthly     int64 // santim a month, excluding VAT
	Annual      int64 // santim a year, excluding VAT
	MaxDrivers  int32
	SMSIncluded int32
	// Offered is false for Enterprise until the platform operator makes the
	// company an offer.
	Offered bool
	Current bool
	// Pending marks the plan the company moves to at the end of the period.
	Pending bool
	// Change is what choosing it does: subscribe, upgrade, downgrade, or
	// switch (another plan at the same price).
	Change string
	// Blocked says why it can't be chosen now.
	Blocked string
}

// BillingOverview is the billing page.
type BillingOverview struct {
	Tenant        db.Tenant
	Standing      Standing
	Current       PlanOption
	Subscribed    bool
	Options       []PlanOption
	Invoices      []db.Invoice
	Open          []db.Invoice
	ActiveDrivers int64
}

// Overview gathers what the billing page shows.
func (s *BillingService) Overview(ctx context.Context, p *Principal) (*BillingOverview, error) {
	if !p.CanManageCompany() {
		return nil, ErrUnauthorized
	}
	t, err := s.q.GetTenantByID(ctx, p.TenantID)
	if err != nil {
		return nil, err
	}
	o := &BillingOverview{Tenant: t, Subscribed: subscribed(t)}
	if o.ActiveDrivers, err = s.q.CountActiveDriversByTenant(ctx, t.ID); err != nil {
		return nil, err
	}
	if o.Invoices, err = s.q.ListInvoices(ctx, db.ListInvoicesParams{TenantID: t.ID, Limit: 50}); err != nil {
		return nil, err
	}
	var debtNumber string
	var debtDue *time.Time
	for i := len(o.Invoices) - 1; i >= 0; i-- { // oldest first
		inv := o.Invoices[i]
		if inv.Status != "open" {
			continue
		}
		o.Open = append(o.Open, inv)
		if (inv.Kind == invoiceRenewal || inv.Kind == invoiceUsage) && (debtDue == nil || inv.DueAt.Time.Before(*debtDue)) {
			debtNumber, debtDue = inv.Number, &inv.DueAt.Time
		}
	}
	o.Standing = standingOf(t.Plan, timePtr(t.TrialEndsAt), debtNumber, debtDue, s.now())

	cur, curErr := s.planTerms(t, t.Plan)
	for _, plan := range model.Plans {
		if plan.Code == model.PlanTrial {
			continue
		}
		opt := PlanOption{Code: plan.Code, Name: plan.Name, MaxDrivers: plan.MaxDrivers, SMSIncluded: plan.SMSIncluded}
		tm, err := s.planTerms(t, plan.Code)
		if err == nil {
			opt.Offered = true
			opt.Monthly, opt.Annual = tm.monthly, model.PeriodPrice(tm.monthly, model.CycleAnnual)
			opt.MaxDrivers, opt.SMSIncluded = tm.drivers, tm.smsQuota
		}
		opt.Current = plan.Code == t.Plan && o.Subscribed
		opt.Pending = t.PendingPlan != nil && *t.PendingPlan == plan.Code
		switch {
		case !opt.Offered || opt.Current:
		case !o.Subscribed:
			opt.Change = "subscribe"
		case curErr != nil || tm.monthly > cur.monthly:
			opt.Change = "upgrade"
		case tm.monthly < cur.monthly:
			opt.Change = "downgrade"
		default:
			opt.Change = "switch"
		}
		switch {
		case !opt.Offered || opt.Current:
		case o.Subscribed && debtNumber != "" && !opt.Pending:
			opt.Blocked = "Pay invoice " + debtNumber + " first."
		case o.ActiveDrivers > int64(opt.MaxDrivers):
			opt.Blocked = fmt.Sprintf("You have %d active drivers; this plan allows %d. Deactivate some first.", o.ActiveDrivers, opt.MaxDrivers)
		}
		if opt.Current {
			o.Current = opt
		}
		o.Options = append(o.Options, opt)
	}
	if !o.Subscribed {
		trial, _ := model.PlanByCode(model.PlanTrial)
		o.Current = PlanOption{Code: trial.Code, Name: trial.Name, MaxDrivers: t.MaxDrivers, SMSIncluded: t.SmsQuota, Current: true}
	}
	return o, nil
}

// ---------------------------------------------------------------------------
// Choosing a plan
// ---------------------------------------------------------------------------

// PlanChange is the outcome of choosing a plan: an invoice to pay, or a
// change that waits for the end of the period.
type PlanChange struct {
	Invoice *db.Invoice
	Message string
}

// ChoosePlan starts a change of plan or billing cycle. A company on a trial
// gets an invoice for a first period, which starts when it is paid (or when
// the trial ends, if that is later: paying early doesn't cut the trial
// short). A paying company that upgrades gets an invoice for the difference
// over the rest of the period; a downgrade or a change of cycle waits for
// the end of the period.
func (s *BillingService) ChoosePlan(ctx context.Context, p *Principal, code, cycle string, ci ClientInfo) (*PlanChange, error) {
	if !p.CanManageCompany() {
		return nil, ErrUnauthorized
	}
	if !model.ValidCycle(cycle) {
		return nil, invalid("cycle", "Choose monthly or annual billing.")
	}
	out := &PlanChange{}
	err := s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		t, err := q.GetTenantForUpdate(ctx, p.TenantID)
		if err != nil {
			return err
		}
		next, err := s.planTerms(t, code)
		if err != nil {
			return err
		}
		drivers, err := q.CountActiveDriversByTenant(ctx, t.ID)
		if err != nil {
			return err
		}
		if drivers > int64(next.drivers) && code != t.Plan {
			return limitReached(fmt.Sprintf("You have %d active drivers and the %s plan allows %d. Deactivate %d first.",
				drivers, next.plan.Name, next.drivers, drivers-int64(next.drivers)))
		}
		now := s.now()
		actor := p.Actor(SourceDashboard, ci.IP, ci.UserAgent)
		audit := func(change string) {
			s.audit(ctx, q, actor, "plan_chosen", "tenant", t.ID, map[string]any{"plan": t.Plan, "cycle": t.BillingCycle},
				map[string]any{"plan": code, "cycle": cycle, "change": change})
		}
		open, err := q.GetOpenPlanInvoice(ctx, t.ID)
		hasOpen := err == nil
		if err != nil && !isNoRows(err) {
			return err
		}

		if !subscribed(t) {
			if hasOpen && open.Kind == invoiceSubscription && open.Plan == code && open.BillingCycle == cycle {
				out.Invoice = &open // the same choice, made again
				return nil
			}
			if err := q.VoidOpenPlanInvoices(ctx, t.ID); err != nil {
				return err
			}
			amount := model.PeriodPrice(next.monthly, cycle)
			inv, err := s.issueInvoice(ctx, q, after, t, invoiceDraft{
				Kind: invoiceSubscription, Plan: code, Cycle: cycle, By: p.UserID,
				Due:   now.AddDate(0, 0, model.InvoiceDueDays),
				Lines: []InvoiceLine{line(firstPeriod(next.plan.Name, cycle), 1, amount)},
				Intro: fmt.Sprintf("Thank you for choosing the %s plan. It starts as soon as this invoice is paid%s.", next.plan.Name, s.trialNote(t, now)),
			})
			if err != nil {
				return err
			}
			out.Invoice = &inv
			audit("subscribe")
			return nil
		}

		// A new plan waits until what is owed is paid. Keeping the plan, or
		// changing only how often it is billed, doesn't.
		if code != t.Plan {
			if debt, err := q.GetOldestDebt(ctx, t.ID); err == nil {
				return conflict(fmt.Sprintf("Pay invoice %s first, then change your plan.", debt.Number))
			} else if !isNoRows(err) {
				return err
			}
		}
		cur, err := s.planTerms(t, t.Plan)
		if err != nil {
			return err
		}
		endsOn := s.date(t.PeriodEnd.Time)
		sub := subscriptionOf(t)
		var pendingCycle *string
		if cycle != t.BillingCycle {
			pendingCycle = &cycle
		}

		switch {
		case code == t.Plan:
			if hasOpen {
				if err := q.VoidOpenPlanInvoices(ctx, t.ID); err != nil {
					return err
				}
			}
			switch {
			case pendingCycle != nil:
				sub.PendingPlan, sub.PendingCycle = nil, pendingCycle
				out.Message = fmt.Sprintf("From %s you'll be billed %s.", endsOn, cycleAdverb(cycle))
			case t.PendingPlan != nil || t.PendingCycle != nil || hasOpen:
				sub.PendingPlan, sub.PendingCycle = nil, nil
				out.Message = fmt.Sprintf("You'll stay on the %s plan, billed %s.", cur.plan.Name, cycleAdverb(t.BillingCycle))
			default:
				return invalid("plan", "You're already on this plan.")
			}
			if err := q.SaveSubscription(ctx, sub); err != nil {
				return err
			}
			audit("keep")

		case next.monthly > cur.monthly:
			sub.PendingPlan, sub.PendingCycle = nil, pendingCycle
			if hasOpen && open.Kind == invoiceUpgrade && open.Plan == code {
				if err := q.SaveSubscription(ctx, sub); err != nil {
					return err
				}
				out.Invoice = &open
				return nil
			}
			if err := q.VoidOpenPlanInvoices(ctx, t.ID); err != nil {
				return err
			}
			diff := model.PeriodPrice(next.monthly-cur.monthly, t.BillingCycle)
			amount := model.Prorate(diff, t.PeriodStart.Time, t.PeriodEnd.Time, now)
			if amount == 0 {
				sub = withTerms(sub, next)
				out.Message = fmt.Sprintf("You're now on the %s plan.", next.plan.Name)
			} else {
				// The difference was worked out for the period in progress,
				// so the invoice lapses when the period ends.
				due := now.AddDate(0, 0, model.InvoiceDueDays)
				if t.PeriodEnd.Time.Before(due) {
					due = t.PeriodEnd.Time
				}
				start := now
				if t.PeriodStart.Time.After(start) {
					start = t.PeriodStart.Time
				}
				inv, err := s.issueInvoice(ctx, q, after, t, invoiceDraft{
					Kind: invoiceUpgrade, Plan: code, Cycle: t.BillingCycle, By: p.UserID,
					PeriodStart: start, PeriodEnd: t.PeriodEnd.Time, Due: due,
					Lines: []InvoiceLine{line(fmt.Sprintf("Upgrade from %s to %s for the rest of the period, %s to %s",
						cur.plan.Name, next.plan.Name, s.date(start), endsOn), 1, amount)},
					Intro: fmt.Sprintf("Your upgrade to the %s plan applies as soon as this invoice is paid.", next.plan.Name),
				})
				if err != nil {
					return err
				}
				out.Invoice = &inv
			}
			if err := q.SaveSubscription(ctx, sub); err != nil {
				return err
			}
			audit("upgrade")

		case next.monthly == cur.monthly:
			if err := q.VoidOpenPlanInvoices(ctx, t.ID); err != nil {
				return err
			}
			sub = withTerms(sub, next)
			sub.PendingPlan, sub.PendingCycle = nil, pendingCycle
			if err := q.SaveSubscription(ctx, sub); err != nil {
				return err
			}
			out.Message = fmt.Sprintf("You're now on the %s plan.", next.plan.Name)
			audit("switch")

		default:
			if err := q.VoidOpenPlanInvoices(ctx, t.ID); err != nil {
				return err
			}
			sub.PendingPlan, sub.PendingCycle = &code, pendingCycle
			if err := q.SaveSubscription(ctx, sub); err != nil {
				return err
			}
			out.Message = fmt.Sprintf("Your plan changes to %s on %s. Until then, nothing changes.", next.plan.Name, endsOn)
			if err := s.emailBilling(ctx, q, after, t.ID, EmailBilling, "Your plan changes on "+endsOn,
				fmt.Sprintf("%s asked to move %s from the %s plan to the %s plan. The change happens on %s, when the current period ends: the invoice for the next period will be for %s.",
					p.Name, t.Name, cur.plan.Name, next.plan.Name, endsOn, money(model.PeriodPrice(next.monthly, cycle))+" plus VAT"),
				"To cancel it, choose "+cur.plan.Name+" again on the Billing page: "+s.cfg.BaseURL+"/app/billing"); err != nil {
				return err
			}
			audit("downgrade")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// trialNote explains when a first period starts for a company still on its
// trial.
func (s *BillingService) trialNote(t db.Tenant, now time.Time) string {
	if t.Plan == model.PlanTrial && t.TrialEndsAt.Valid && t.TrialEndsAt.Time.After(now) {
		return ", and not before your free trial ends on " + s.date(t.TrialEndsAt.Time)
	}
	return ""
}

// ---------------------------------------------------------------------------
// Invoices
// ---------------------------------------------------------------------------

// InvoiceLine is one charge on an invoice. Amounts are in santim.
type InvoiceLine struct {
	Description string `json:"description"`
	Quantity    int64  `json:"quantity"`
	Unit        int64  `json:"unit_santim"`
	Amount      int64  `json:"amount_santim"`
}

func line(desc string, qty, unit int64) InvoiceLine {
	return InvoiceLine{Description: desc, Quantity: qty, Unit: unit, Amount: qty * unit}
}

// SMSUsage is what an overage invoice charged for. It is kept with the
// invoice because the text log it was counted from is deleted after 180
// days (AC-045.5).
type SMSUsage struct {
	Month    string `json:"month"`
	Sent     int64  `json:"sent"`
	Included int32  `json:"included"`
	Extra    int64  `json:"extra"`
}

type invoiceDraft struct {
	Kind, Plan, Cycle      string
	PeriodStart, PeriodEnd time.Time
	Lines                  []InvoiceLine
	Usage                  *SMSUsage
	Due                    time.Time
	By                     pgtype.UUID
	// Intro opens the email that sends the invoice.
	Intro string
}

// issueInvoice numbers and stores an invoice and emails it to the company.
// One for nothing is paid at once.
func (s *BillingService) issueInvoice(ctx context.Context, q *db.Queries, after *afterCommit, t db.Tenant, d invoiceDraft) (db.Invoice, error) {
	now := s.now()
	var subtotal int64
	for _, l := range d.Lines {
		subtotal += l.Amount
	}
	vat := model.VAT(subtotal)
	year := now.In(s.cfg.Location).Year()
	n, err := q.NextInvoiceNumber(ctx, int32(year))
	if err != nil {
		return db.Invoice{}, err
	}
	var usage []byte
	if d.Usage != nil {
		usage = mustJSON(d.Usage)
	}
	inv, err := q.CreateInvoice(ctx, db.CreateInvoiceParams{
		TenantID: t.ID, Number: fmt.Sprintf("INV-%d-%05d", year, n), Kind: d.Kind, Plan: d.Plan, BillingCycle: d.Cycle,
		PeriodStart: toTimestamp(d.PeriodStart), PeriodEnd: toTimestamp(d.PeriodEnd),
		Lines: mustJSON(d.Lines), Usage: usage,
		SubtotalSantim: subtotal, VatSantim: vat, TotalSantim: subtotal + vat,
		IssuedAt: toTimestamp(now), DueAt: toTimestamp(d.Due),
		CustomerName: t.Name, CustomerTin: t.Tin, CustomerCity: &t.RegionCity, CreatedBy: d.By,
	})
	if err != nil {
		return inv, err
	}
	if inv.TotalSantim == 0 {
		return inv, s.settle(ctx, q, after, t, inv, db.Payment{}, now, SystemActor(t.ID))
	}
	paragraphs := []string{}
	if d.Intro != "" {
		paragraphs = append(paragraphs, d.Intro)
	}
	paragraphs = append(paragraphs,
		fmt.Sprintf("Invoice %s is for %s including VAT, due on %s. Pay it by telebirr or CBE Birr, or see the bank transfer details, here:",
			inv.Number, money(inv.TotalSantim), s.date(inv.DueAt.Time)),
		s.invoiceURL(inv))
	if s.cfg.BankDetails != "" {
		paragraphs = append(paragraphs, "To pay by bank transfer: "+s.cfg.BankDetails+". Quote "+inv.Number+" as the reference.")
	}
	return inv, s.emailBilling(ctx, q, after, t.ID, EmailInvoice, fmt.Sprintf("Invoice %s from %s", inv.Number, s.cfg.AppName), paragraphs...)
}

func (c *core) invoiceURL(inv db.Invoice) string {
	return c.cfg.BaseURL + "/app/billing/invoices/" + inv.ID.String()
}

// InvoiceLines reads an invoice's lines.
func InvoiceLines(raw []byte) []InvoiceLine {
	var out []InvoiceLine
	_ = json.Unmarshal(raw, &out)
	return out
}

// MonthName is the month the texts were sent in, such as "October 2026".
func (u SMSUsage) MonthName() string {
	m, err := time.Parse("2006-01", u.Month)
	if err != nil {
		return u.Month
	}
	return m.Format("January 2006")
}

// InvoiceUsage reads what an overage invoice charged for, if it is one.
func InvoiceUsage(raw []byte) *SMSUsage {
	if len(raw) == 0 {
		return nil
	}
	var u SMSUsage
	if json.Unmarshal(raw, &u) != nil {
		return nil
	}
	return &u
}

// InvoiceDetail is an invoice with what has been tried to pay it.
type InvoiceDetail struct {
	Invoice    db.Invoice
	TenantName string
	Lines      []InvoiceLine
	Usage      *SMSUsage
	Payments   []db.Payment
	// Waiting is a payment the payer started that the provider hasn't
	// reported on yet.
	Waiting *db.Payment
}

// PayReference is what a payer quotes to pay this invoice by reference.
func (d *InvoiceDetail) PayReference() string { return PayReference(d.Invoice.Number) }

// PayReference is the reference that pays an invoice from a phone: its
// number's digits (INV-2026-00012 is 202600012), easy to key in and unique,
// since invoice numbers are the platform's.
func PayReference(number string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, number)
}

// invoiceNumberOf reads the invoice number in a reference a payer quoted:
// the number (INV-2026-00012), its digits (202600012), or the digits without
// the zeros in the middle (202612). It is "" for anything else.
func invoiceNumberOf(ref string) string {
	ref = strings.TrimPrefix(strings.ToUpper(strings.Join(strings.Fields(ref), "")), "INV")
	var digits strings.Builder
	for _, r := range ref {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
		case r != '-':
			return ""
		}
	}
	d := digits.String()
	if len(d) < 5 {
		return ""
	}
	year, n := d[:4], strings.TrimLeft(d[4:], "0")
	if n == "" {
		return ""
	}
	if len(n) < 5 {
		n = strings.Repeat("0", 5-len(n)) + n
	}
	return "INV-" + year + "-" + n
}

// Invoice loads one of the caller's company's invoices.
func (s *BillingService) Invoice(ctx context.Context, p *Principal, id pgtype.UUID) (*InvoiceDetail, error) {
	if !p.CanManageCompany() {
		return nil, ErrUnauthorized
	}
	inv, err := s.q.GetInvoice(ctx, db.GetInvoiceParams{ID: id, TenantID: p.TenantID})
	if isNoRows(err) {
		return nil, notFound("Invoice not found.")
	}
	if err != nil {
		return nil, err
	}
	return s.invoiceDetail(ctx, inv, p.TenantName)
}

// PlatformInvoice loads any company's invoice, for the platform operator.
func (s *BillingService) PlatformInvoice(ctx context.Context, p *Principal, id pgtype.UUID) (*InvoiceDetail, error) {
	if !p.IsPlatformAdmin() {
		return nil, ErrUnauthorized
	}
	row, err := s.q.GetInvoiceByID(ctx, id)
	if isNoRows(err) {
		return nil, notFound("Invoice not found.")
	}
	if err != nil {
		return nil, err
	}
	return s.invoiceDetail(ctx, db.Invoice{
		ID: row.ID, TenantID: row.TenantID, Number: row.Number, Kind: row.Kind, Plan: row.Plan, BillingCycle: row.BillingCycle,
		PeriodStart: row.PeriodStart, PeriodEnd: row.PeriodEnd, Lines: row.Lines, Usage: row.Usage,
		SubtotalSantim: row.SubtotalSantim, VatSantim: row.VatSantim, TotalSantim: row.TotalSantim, Status: row.Status,
		IssuedAt: row.IssuedAt, DueAt: row.DueAt, PaidAt: row.PaidAt, VoidedAt: row.VoidedAt,
		CustomerName: row.CustomerName, CustomerTin: row.CustomerTin, CustomerCity: row.CustomerCity, CreatedBy: row.CreatedBy,
	}, row.TenantName)
}

// CompanyInvoices is one company's latest invoices, for the platform admin.
func (s *BillingService) CompanyInvoices(ctx context.Context, p *Principal, tenantID pgtype.UUID) ([]db.Invoice, error) {
	if !p.IsPlatformAdmin() {
		return nil, ErrUnauthorized
	}
	return s.q.ListInvoices(ctx, db.ListInvoicesParams{TenantID: tenantID, Limit: 24})
}

func (s *BillingService) invoiceDetail(ctx context.Context, inv db.Invoice, tenantName string) (*InvoiceDetail, error) {
	d := &InvoiceDetail{Invoice: inv, TenantName: tenantName, Lines: InvoiceLines(inv.Lines), Usage: InvoiceUsage(inv.Usage)}
	var err error
	if d.Payments, err = s.q.ListPaymentsByInvoice(ctx, db.ListPaymentsByInvoiceParams{InvoiceID: inv.ID, TenantID: inv.TenantID}); err != nil {
		return nil, err
	}
	for i := range d.Payments {
		if d.Payments[i].Status == paymentPending {
			d.Waiting = &d.Payments[i]
			break
		}
	}
	return d, nil
}

// ---------------------------------------------------------------------------
// Payments
// ---------------------------------------------------------------------------

// Pay starts paying an invoice with a provider that has a checkout: the
// payment waits while the payer is at the provider's page, whose address is
// returned, and ends when the provider reports how it went. The payer's
// email must be confirmed, since the receipt and later invoices go there.
func (s *BillingService) Pay(ctx context.Context, p *Principal, invoiceID pgtype.UUID, providerCode string, ci ClientInfo) (db.Payment, string, error) {
	if !p.CanManageCompany() {
		return db.Payment{}, "", ErrUnauthorized
	}
	if !p.EmailVerified {
		return db.Payment{}, "", forbidden("Confirm your email address before paying: receipts and invoices are sent there. " +
			"Open the link we emailed you, or send it again from Your account.")
	}
	prov, ok := s.Provider(providerCode)
	if !ok {
		return db.Payment{}, "", invalid("provider", "Choose how to pay.")
	}
	starter, ok := prov.(payments.Starter)
	if !ok {
		return db.Payment{}, "", invalid("provider", "Pay with "+prov.Label()+" from your phone, quoting the invoice's reference.")
	}
	var pay db.Payment
	var inv db.Invoice
	err := s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		var err error
		inv, err = q.LockInvoice(ctx, db.LockInvoiceParams{ID: invoiceID, TenantID: p.TenantID})
		if isNoRows(err) {
			return notFound("Invoice not found.")
		}
		if err != nil {
			return err
		}
		if err := payable(inv); err != nil {
			return err
		}
		pay, err = q.CreatePayment(ctx, db.CreatePaymentParams{
			TenantID: p.TenantID, InvoiceID: inv.ID, Provider: prov.Code(), Reference: paymentReference(),
			AmountSantim: inv.TotalSantim, Status: paymentPending,
		})
		if err != nil {
			return err
		}
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "payment_started", "invoice", inv.ID, nil,
			map[string]any{"invoice": inv.Number, "provider": prov.Code(), "amount": payments.FormatETB(inv.TotalSantim)})
		return nil
	})
	if err != nil {
		return pay, "", err
	}
	checkout, err := starter.Start(ctx, payments.Charge{
		Reference: pay.Reference, Amount: pay.AmountSantim,
		Description: fmt.Sprintf("%s invoice %s for %s", s.cfg.AppName, inv.Number, p.TenantName),
		NotifyURL:   s.cfg.BaseURL + "/api/v1/payments/" + prov.Code() + "/notify",
		ReturnURL:   s.cfg.BaseURL + "/app/billing/payments/" + pay.ID.String(),
		Timeout:     paymentPromptTTL,
	})
	if err != nil {
		s.log.Error().Err(err).Str("provider", prov.Code()).Msg("start payment")
		_ = s.q.FinishPayment(ctx, db.FinishPaymentParams{ID: pay.ID, Status: paymentFailed, Note: ptr("The provider could not be reached.")})
		return pay, "", conflict(prov.Label() + " could not be reached. Try again in a moment, or pay by bank transfer.")
	}
	if checkout.ProviderRef != "" {
		_ = s.q.SetPaymentProviderRef(ctx, db.SetPaymentProviderRefParams{ID: pay.ID, ProviderRef: &checkout.ProviderRef})
		pay.ProviderRef = &checkout.ProviderRef
	}
	return pay, checkout.RedirectURL, nil
}

func payable(inv db.Invoice) error {
	switch inv.Status {
	case "paid":
		return conflict("Invoice " + inv.Number + " is already paid.")
	case "void":
		return conflict("Invoice " + inv.Number + " was cancelled, so it can't be paid.")
	}
	return nil
}

// paymentReference is our reference for a payment attempt, which the
// provider echoes back.
func paymentReference() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	return "PAY-" + base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}

// Payment loads one of the caller's company's payments.
func (s *BillingService) Payment(ctx context.Context, p *Principal, id pgtype.UUID) (db.GetPaymentRow, error) {
	if !p.CanManageCompany() {
		return db.GetPaymentRow{}, ErrUnauthorized
	}
	pay, err := s.q.GetPayment(ctx, db.GetPaymentParams{ID: id, TenantID: p.TenantID})
	if isNoRows(err) {
		return pay, notFound("Payment not found.")
	}
	return pay, err
}

// PaymentPrompt is a payment waiting on a stand-in provider, for the page
// that plays its checkout. Knowing the reference is the permission.
func (s *BillingService) PaymentPrompt(ctx context.Context, reference string) (db.GetPaymentByReferenceRow, payments.Provider, error) {
	pay, err := s.q.GetPaymentByReference(ctx, reference)
	if isNoRows(err) {
		return pay, nil, notFound("There is no payment with this reference.")
	}
	if err != nil {
		return pay, nil, err
	}
	prov, ok := s.Provider(pay.Provider)
	if !ok || !prov.Mock() {
		return pay, nil, notFound("There is no payment with this reference.")
	}
	return pay, prov, nil
}

// Settle applies a provider's report on a payment Menged started. Reports
// repeat and arrive late, so this is idempotent: a payment that ended stays
// as it is, except that money arriving after the checkout expired still
// counts. Money for an invoice already paid, or cancelled meanwhile, is
// recorded and flagged for a refund.
func (s *BillingService) Settle(ctx context.Context, providerCode string, n payments.Notification) error {
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		pay, err := q.LockPaymentByReference(ctx, n.Reference)
		if isNoRows(err) {
			return notFound("Unknown payment reference.")
		}
		if err != nil {
			return err
		}
		if pay.Provider != providerCode {
			return invalid("reference", "This payment was not made with this provider.")
		}
		if pay.Status != paymentPending && pay.Status != paymentExpired {
			// Paid, failed or refunded: it has ended.
			return nil
		}
		published := func() {
			s.publish(pay.TenantID, "payment", map[string]any{"id": pay.ID.String(), "status": pay.Status})
		}
		raw := mustJSON(n)
		if n.Status != payments.StatusPaid {
			if err := q.FinishPayment(ctx, db.FinishPaymentParams{ID: pay.ID, Status: paymentFailed, ProviderRef: toText(n.ProviderRef), Raw: raw}); err != nil {
				return err
			}
			pay.Status = paymentFailed
			after.do(published)
			return nil
		}
		if n.Amount != pay.AmountSantim {
			s.log.Warn().Str("reference", pay.Reference).Int64("expected", pay.AmountSantim).Int64("reported", n.Amount).
				Msg("payment notification for the wrong amount")
			return invalid("amount", "The amount does not match the payment.")
		}
		if err := q.FinishPayment(ctx, db.FinishPaymentParams{ID: pay.ID, Status: paymentPaid, ProviderRef: toText(n.ProviderRef), Raw: raw}); err != nil {
			return err
		}
		pay.Status = paymentPaid
		pay.ProviderRef = toText(n.ProviderRef)
		after.do(published)
		return s.applyPayment(ctx, q, after, pay, s.now(), SystemActor(pay.TenantID))
	})
}

// Bill says what an invoice quoted by reference owes, for a provider whose
// payers pay from their phone.
func (s *BillingService) Bill(ctx context.Context, providerCode, reference string) (payments.Bill, error) {
	number := invoiceNumberOf(reference)
	if number == "" {
		return payments.Bill{}, payments.ErrUnknownBill
	}
	inv, err := s.q.GetInvoiceByNumber(ctx, number)
	if isNoRows(err) {
		return payments.Bill{}, payments.ErrUnknownBill
	}
	if err != nil {
		return payments.Bill{}, err
	}
	switch {
	case inv.Status == "paid":
		return payments.Bill{}, payments.ErrBillPaid
	case inv.Status == "void", !inv.TenantID.Valid:
		return payments.Bill{}, payments.ErrBillClosed
	}
	return payments.Bill{
		InvoiceNumber: inv.Number, Amount: inv.TotalSantim, Customer: inv.CustomerName,
		Description: fmt.Sprintf("%s invoice %s", s.cfg.AppName, inv.Number),
	}, nil
}

// BillPaid records money a provider took against an invoice's reference.
// The provider has already moved it, so it is never turned away once the
// invoice is found: paid twice, for the wrong amount, or for an invoice
// cancelled meanwhile, it is recorded and flagged for a refund. A repeated
// confirmation of the same transaction changes nothing.
func (s *BillingService) BillPaid(ctx context.Context, providerCode string, bp payments.BillPayment) error {
	if bp.TransID == "" || len(bp.TransID) > 64 {
		return invalid("TransID", "The transaction needs an ID of at most 64 characters.")
	}
	number := invoiceNumberOf(bp.Reference)
	if number == "" {
		return payments.ErrUnknownBill
	}
	reference := strings.ToUpper(providerCode) + "-" + bp.TransID
	now := s.now()
	paidAt := bp.PaidAt
	if paidAt.IsZero() || paidAt.After(now) {
		paidAt = now
	}
	var payer *string
	if local, err := phone.Normalize(bp.Payer); err == nil {
		payer = &local
	}
	err := s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		found, err := q.GetInvoiceByNumber(ctx, number)
		if isNoRows(err) {
			return payments.ErrUnknownBill
		}
		if err != nil {
			return err
		}
		// Company, then invoice, then payment: the order applyPayment locks
		// them in.
		if found.TenantID.Valid {
			if _, err := q.GetTenantForUpdate(ctx, found.TenantID); err != nil {
				return err
			}
		}
		inv, err := q.LockInvoiceByNumber(ctx, number)
		if err != nil {
			return err
		}
		if _, err := q.LockPaymentByReference(ctx, reference); err == nil {
			return nil
		} else if !isNoRows(err) {
			return err
		}
		pay, err := q.CreatePayment(ctx, db.CreatePaymentParams{
			TenantID: inv.TenantID, InvoiceID: inv.ID, Provider: providerCode, Reference: reference,
			ProviderRef: &bp.TransID, PayerPhone: payer, AmountSantim: bp.Amount, Status: paymentPaid,
			Raw: bp.Raw, CompletedAt: toTimestamp(paidAt),
		})
		if err != nil {
			return err
		}
		after.do(func() {
			s.publish(pay.TenantID, "payment", map[string]any{"id": pay.ID.String(), "status": pay.Status})
		})
		actor := SystemActor(pay.TenantID)
		if bp.Amount != inv.TotalSantim {
			note := fmt.Sprintf("%s was paid, but invoice %s asks for %s, so it will be refunded.", money(bp.Amount), inv.Number, money(inv.TotalSantim))
			if err := q.FlagRefundDue(ctx, db.FlagRefundDueParams{ID: pay.ID, Note: &note}); err != nil {
				return err
			}
			s.log.Warn().Str("reference", pay.Reference).Str("invoice", inv.Number).Msg("payment for the wrong amount needs a refund")
			s.audit(ctx, q, actor, "payment_refund_due", "invoice", inv.ID, nil,
				map[string]any{"invoice": inv.Number, "reference": pay.Reference, "amount": payments.FormatETB(pay.AmountSantim)})
			return nil
		}
		return s.applyPayment(ctx, q, after, pay, paidAt, actor)
	})
	if name, dup := isUniqueViolation(err); dup && name == "payments_reference_key" {
		// A confirmation of the same transaction recorded it meanwhile.
		return nil
	}
	return err
}

// CheckPayment asks the provider how one of the caller's company's waiting
// payments went, at most every few seconds, so that coming back from the
// provider's checkout shows the outcome even when its report is late or lost.
func (s *BillingService) CheckPayment(ctx context.Context, p *Principal, id pgtype.UUID) {
	if !p.CanManageCompany() {
		return
	}
	pay, err := s.q.GetPayment(ctx, db.GetPaymentParams{ID: id, TenantID: p.TenantID})
	if err != nil || pay.Status != paymentPending {
		return
	}
	if err := s.check(ctx, id, 10*time.Second); err != nil {
		s.log.Warn().Err(err).Str("reference", pay.Reference).Msg("check payment")
	}
}

// reconcile asks the providers that can be asked how the payments they
// haven't reported on went. It stops after a while, so a provider that
// doesn't answer can't hold up the other jobs; the rest wait for the next run.
func (s *BillingService) reconcile(ctx context.Context) error {
	var codes []string
	for _, p := range s.providers {
		if _, ok := p.(payments.Querier); ok {
			codes = append(codes, p.Code())
		}
	}
	if len(codes) == 0 {
		return nil
	}
	now := s.now()
	due, err := s.q.ListPaymentsToCheck(ctx, db.ListPaymentsToCheckParams{
		Providers: codes, StartedBefore: toTimestamp(now.Add(-time.Minute)),
		StartedAfter: toTimestamp(now.Add(-paymentCheckWindow)), CheckedBefore: toTimestamp(now.Add(-paymentCheckEvery)),
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for _, pay := range due {
		if ctx.Err() != nil {
			break
		}
		if err := s.check(ctx, pay.ID, paymentCheckEvery); err != nil {
			s.log.Warn().Err(err).Str("reference", pay.Reference).Msg("check payment")
		}
	}
	return nil
}

// check asks a payment's provider how it went, unless it was asked within
// the interval, and applies a final answer.
func (s *BillingService) check(ctx context.Context, id pgtype.UUID, interval time.Duration) error {
	now := s.now()
	pay, err := s.q.ClaimPaymentCheck(ctx, db.ClaimPaymentCheckParams{
		ID: id, Now: toTimestamp(now), CheckedBefore: toTimestamp(now.Add(-interval)),
	})
	if isNoRows(err) {
		return nil
	}
	if err != nil {
		return err
	}
	prov, _ := s.Provider(pay.Provider)
	querier, ok := prov.(payments.Querier)
	if !ok {
		return nil
	}
	qctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	n, done, err := querier.Query(qctx, pay.Reference)
	cancel()
	if err != nil || !done {
		return err
	}
	return s.Settle(ctx, pay.Provider, n)
}

// applyPayment settles the invoice a payment that arrived was for.
func (s *BillingService) applyPayment(ctx context.Context, q *db.Queries, after *afterCommit, pay db.Payment, paidAt time.Time, a Actor) error {
	if !pay.TenantID.Valid {
		// The company was deleted while the money was on its way.
		note := "The company was deleted before this payment arrived, so it will be refunded."
		if err := q.FlagRefundDue(ctx, db.FlagRefundDueParams{ID: pay.ID, Note: &note}); err != nil {
			return err
		}
		s.log.Warn().Str("reference", pay.Reference).Msg("payment for a deleted company needs a refund")
		s.audit(ctx, q, a, "payment_refund_due", "payment", pay.ID, nil,
			map[string]any{"reference": pay.Reference, "amount": payments.FormatETB(pay.AmountSantim)})
		return nil
	}
	t, err := q.GetTenantForUpdate(ctx, pay.TenantID)
	if err != nil {
		return err
	}
	inv, err := q.LockInvoice(ctx, db.LockInvoiceParams{ID: pay.InvoiceID, TenantID: pay.TenantID})
	if err != nil {
		return err
	}
	if inv.Status == "open" {
		return s.settle(ctx, q, after, t, inv, pay, paidAt, a)
	}
	note := "Invoice " + inv.Number + " was already paid, so this payment will be refunded."
	if inv.Status == "void" {
		note = "Invoice " + inv.Number + " was cancelled before this payment arrived, so it will be refunded."
	}
	if err := q.FlagRefundDue(ctx, db.FlagRefundDueParams{ID: pay.ID, Note: &note}); err != nil {
		return err
	}
	s.log.Warn().Str("reference", pay.Reference).Str("invoice", inv.Number).Msg("payment needs a refund")
	s.audit(ctx, q, a, "payment_refund_due", "invoice", inv.ID, nil,
		map[string]any{"invoice": inv.Number, "reference": pay.Reference, "amount": payments.FormatETB(pay.AmountSantim)})
	return nil
}

// settle records an invoice as paid and applies what it paid for: a first
// period, an upgrade, or a renewal paid only after read-only mode began,
// whose period then restarts from the payment.
func (s *BillingService) settle(ctx context.Context, q *db.Queries, after *afterCommit, t db.Tenant, inv db.Invoice, pay db.Payment, paidAt time.Time, a Actor) error {
	var start, end time.Time
	sub := subscriptionOf(t)
	switch inv.Kind {
	case invoiceSubscription:
		start = paidAt
		if t.Plan == model.PlanTrial && t.TrialEndsAt.Valid && t.TrialEndsAt.Time.After(start) {
			start = t.TrialEndsAt.Time
		}
		end = s.periodEnd(start, inv.BillingCycle)
		sub = withTerms(sub, s.termsOrDefault(t, inv.Plan))
		sub.BillingCycle = inv.BillingCycle
		sub.PeriodStart, sub.PeriodEnd, sub.SubscribedAt = toTimestamp(start), toTimestamp(end), toTimestamp(start)
		sub.PendingPlan, sub.PendingCycle = nil, nil
	case invoiceUpgrade:
		sub = withTerms(sub, s.termsOrDefault(t, inv.Plan))
		sub.PendingPlan = nil
	case invoiceRenewal:
		if !paidAt.Before(inv.DueAt.Time.AddDate(0, 0, model.GraceDays)) {
			start, end = paidAt, s.periodEnd(paidAt, t.BillingCycle)
			sub.PeriodStart, sub.PeriodEnd = toTimestamp(start), toTimestamp(end)
		}
	}
	if inv.Kind != invoiceUsage {
		if err := q.SaveSubscription(ctx, sub); err != nil {
			return err
		}
	}
	if err := q.MarkInvoicePaid(ctx, db.MarkInvoicePaidParams{
		ID: inv.ID, TenantID: t.ID, PaidAt: toTimestamp(paidAt), PeriodStart: toTimestamp(start), PeriodEnd: toTimestamp(end),
	}); err != nil {
		return err
	}
	s.audit(ctx, q, a, "invoice_paid", "invoice", inv.ID, nil, map[string]any{
		"invoice": inv.Number, "amount": payments.FormatETB(inv.TotalSantim), "provider": pay.Provider, "reference": pay.Reference,
	})
	after.do(func() { s.publish(t.ID, "invoice", map[string]any{"id": inv.ID.String(), "status": "paid"}) })
	if inv.TotalSantim == 0 {
		return nil
	}

	how := ""
	if pay.Provider != "" {
		label := "bank transfer"
		if prov, ok := s.Provider(pay.Provider); ok {
			label = prov.Label()
		}
		how = fmt.Sprintf(", by %s (reference %s)", label, pay.Reference)
	}
	paragraphs := []string{fmt.Sprintf("We received %s for invoice %s on %s%s. Thank you.", money(inv.TotalSantim), inv.Number, s.date(paidAt), how)}
	switch inv.Kind {
	case invoiceSubscription, invoiceRenewal:
		paragraphs = append(paragraphs, fmt.Sprintf("Your %s plan is paid for from %s to %s.", model.PlanName(sub.Plan), s.date(sub.PeriodStart.Time), s.date(sub.PeriodEnd.Time)))
	case invoiceUpgrade:
		paragraphs = append(paragraphs, fmt.Sprintf("You're now on the %s plan, with up to %d drivers.", model.PlanName(sub.Plan), sub.MaxDrivers))
	}
	paragraphs = append(paragraphs, "The invoice, marked paid, is here: "+s.invoiceURL(inv))
	return s.emailBilling(ctx, q, after, t.ID, EmailReceipt, "Receipt for invoice "+inv.Number, paragraphs...)
}

// termsOrDefault is planTerms for a plan already paid for. Should the
// Enterprise offer have been withdrawn meanwhile, the plan's own limits
// apply.
func (s *BillingService) termsOrDefault(t db.Tenant, code string) terms {
	if tm, err := s.planTerms(t, code); err == nil {
		return tm
	}
	p, _ := model.PlanByCode(code)
	return terms{plan: p, monthly: p.MonthlyETB * 100, drivers: p.MaxDrivers, smsQuota: p.SMSIncluded}
}

// ---------------------------------------------------------------------------
// The platform operator
// ---------------------------------------------------------------------------

// MarkPaid records a bank transfer the platform operator has seen arrive,
// and settles the invoice it paid.
func (s *BillingService) MarkPaid(ctx context.Context, p *Principal, invoiceID pgtype.UUID, reference, note string, ci ClientInfo) error {
	if !p.IsPlatformAdmin() {
		return ErrUnauthorized
	}
	reference = strings.ToUpper(strings.Join(strings.Fields(reference), ""))
	if reference == "" {
		return invalid("reference", "Enter the bank's reference for the transfer.")
	}
	if len(reference) > 60 {
		return invalid("reference", "That reference is too long.")
	}
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		row, err := q.GetInvoiceByID(ctx, invoiceID)
		if isNoRows(err) {
			return notFound("Invoice not found.")
		}
		if err != nil {
			return err
		}
		if !row.TenantID.Valid {
			return conflict("This invoice's company has been deleted.")
		}
		t, err := q.GetTenantForUpdate(ctx, row.TenantID)
		if err != nil {
			return err
		}
		inv, err := q.LockInvoice(ctx, db.LockInvoiceParams{ID: invoiceID, TenantID: t.ID})
		if err != nil {
			return err
		}
		if err := payable(inv); err != nil {
			return err
		}
		now := s.now()
		pay, err := q.CreatePayment(ctx, db.CreatePaymentParams{
			TenantID: t.ID, InvoiceID: inv.ID, Provider: providerBank, Reference: "BANK-" + reference,
			AmountSantim: inv.TotalSantim, Status: paymentPaid, Note: toText(strings.TrimSpace(note)),
			RecordedBy: p.UserID, CompletedAt: toTimestamp(now),
		})
		if _, dup := isUniqueViolation(err); dup {
			return invalid("reference", "A transfer with this reference is already recorded.")
		}
		if err != nil {
			return err
		}
		operator := p.Actor(SourceDashboard, ci.IP, ci.UserAgent)
		s.audit(ctx, q, operator, "invoice_marked_paid", "invoice", inv.ID, nil,
			map[string]any{"invoice": inv.Number, "company": t.Name, "reference": pay.Reference})
		operator.TenantID = t.ID
		return s.settle(ctx, q, after, t, inv, pay, now, operator)
	})
}

// MarkRefunded records that the platform operator sent back a payment that
// arrived for an invoice already paid or cancelled.
func (s *BillingService) MarkRefunded(ctx context.Context, p *Principal, paymentID pgtype.UUID, reference string, ci ClientInfo) error {
	if !p.IsPlatformAdmin() {
		return ErrUnauthorized
	}
	reference = strings.ToUpper(strings.Join(strings.Fields(reference), ""))
	if reference == "" {
		return invalid("reference", "Enter the reference of the refund transfer.")
	}
	if len(reference) > 60 {
		return invalid("reference", "That reference is too long.")
	}
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		pay, err := q.LockPayment(ctx, paymentID)
		if isNoRows(err) {
			return notFound("Payment not found.")
		}
		if err != nil {
			return err
		}
		if !pay.RefundDue {
			return conflict("This payment isn't waiting for a refund.")
		}
		now := s.now()
		note := fmt.Sprintf("Refunded on %s, reference %s.", s.date(now), reference)
		if err := q.MarkPaymentRefunded(ctx, db.MarkPaymentRefundedParams{ID: pay.ID, RefundReference: &reference, Note: &note}); err != nil {
			return err
		}
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "payment_refunded", "invoice", pay.InvoiceID, nil,
			map[string]any{"invoice": pay.InvoiceNumber, "payment": pay.Reference, "refund": reference, "amount": payments.FormatETB(pay.AmountSantim)})
		if !pay.TenantID.Valid {
			return nil
		}
		why := fmt.Sprintf("invoice %s had already been paid or cancelled when it arrived", pay.InvoiceNumber)
		if pay.AmountSantim != pay.InvoiceTotal {
			why = fmt.Sprintf("it wasn't the %s invoice %s asks for", money(pay.InvoiceTotal), pay.InvoiceNumber)
		}
		return s.emailBilling(ctx, q, after, pay.TenantID, EmailBilling, "Refund of "+money(pay.AmountSantim),
			fmt.Sprintf("We've sent back the %s paid on %s (reference %s): %s. The refund's reference is %s.",
				money(pay.AmountSantim), s.date(pay.CompletedAt.Time), pay.Reference, why, reference))
	})
}

// VoidInvoice cancels an unpaid invoice, for the platform operator: a
// mistake, or a period given for free.
func (s *BillingService) VoidInvoice(ctx context.Context, p *Principal, invoiceID pgtype.UUID, reason string, ci ClientInfo) error {
	if !p.IsPlatformAdmin() {
		return ErrUnauthorized
	}
	reason = strings.TrimSpace(reason)
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		row, err := q.GetInvoiceByID(ctx, invoiceID)
		if isNoRows(err) {
			return notFound("Invoice not found.")
		}
		if err != nil {
			return err
		}
		n, err := q.VoidInvoice(ctx, db.VoidInvoiceParams{ID: invoiceID, TenantID: row.TenantID})
		if err != nil {
			return err
		}
		if n == 0 {
			return conflict("Only an unpaid invoice can be cancelled.")
		}
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "invoice_voided", "invoice", invoiceID, nil,
			map[string]any{"invoice": row.Number, "company": row.TenantName, "reason": reason})
		if !row.TenantID.Valid {
			return nil
		}
		paragraphs := []string{fmt.Sprintf("Invoice %s for %s has been cancelled: there is nothing to pay on it.", row.Number, money(row.TotalSantim))}
		if reason != "" {
			paragraphs = append(paragraphs, "Reason: "+reason)
		}
		return s.emailBilling(ctx, q, after, row.TenantID, EmailBilling, "Invoice "+row.Number+" cancelled", paragraphs...)
	})
}

// ExtendTrial gives a company on a free trial more days.
func (s *BillingService) ExtendTrial(ctx context.Context, p *Principal, tenantID pgtype.UUID, days int, ci ClientInfo) error {
	if !p.IsPlatformAdmin() {
		return ErrUnauthorized
	}
	if days < 1 || days > 90 {
		return invalid("days", "Extend by 1 to 90 days.")
	}
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		t, err := q.GetTenantForUpdate(ctx, tenantID)
		if isNoRows(err) {
			return notFound("Company not found.")
		}
		if err != nil {
			return err
		}
		if t.Plan != model.PlanTrial {
			return conflict(t.Name + " is not on a free trial.")
		}
		from := s.now()
		if t.TrialEndsAt.Valid && t.TrialEndsAt.Time.After(from) {
			from = t.TrialEndsAt.Time
		}
		end := from.AddDate(0, 0, days)
		if err := q.SetTrialEnd(ctx, db.SetTrialEndParams{ID: t.ID, TrialEndsAt: toTimestamp(end)}); err != nil {
			return err
		}
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "trial_extended", "tenant", t.ID,
			map[string]any{"trial_ends": timePtr(t.TrialEndsAt)}, map[string]any{"trial_ends": end, "company": t.Name})
		return s.emailBilling(ctx, q, after, t.ID, EmailBilling, "Your free trial is extended",
			fmt.Sprintf("Your free trial of %s now ends on %s. Everything works as before until then.", s.cfg.AppName, s.when(end)),
			"Plans and prices are on the Billing page: "+s.cfg.BaseURL+"/app/billing")
	})
}

// SetEnterpriseOffer sets the Enterprise price and limits for a company,
// which it can then choose on its billing page. A price of zero withdraws
// the offer. A company already on Enterprise gets the new limits at once
// and the new price from its next period.
func (s *BillingService) SetEnterpriseOffer(ctx context.Context, p *Principal, tenantID pgtype.UUID, priceETB int64, maxDrivers, smsQuota int, ci ClientInfo) error {
	if !p.IsPlatformAdmin() {
		return ErrUnauthorized
	}
	if tenantID == p.TenantID {
		return invalid("", "The platform's own account doesn't pay.")
	}
	withdraw := priceETB == 0
	if !withdraw {
		if priceETB < 0 || priceETB > 10_000_000 {
			return invalid("price", "Enter a monthly price in birr.")
		}
		if maxDrivers < 1 || maxDrivers > 10_000 {
			return invalid("max_drivers", "Allow between 1 and 10,000 drivers.")
		}
		if smsQuota < 0 || smsQuota > 1_000_000 {
			return invalid("sms_quota", "Include between 0 and 1,000,000 texts a month.")
		}
	}
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		t, err := q.GetTenantForUpdate(ctx, tenantID)
		if isNoRows(err) {
			return notFound("Company not found.")
		}
		if err != nil {
			return err
		}
		operator := p.Actor(SourceDashboard, ci.IP, ci.UserAgent)
		if withdraw {
			if t.Plan == model.PlanEnterprise {
				return conflict(t.Name + " is on Enterprise: change its terms rather than withdrawing them.")
			}
			if open, err := q.GetOpenPlanInvoice(ctx, t.ID); err == nil && open.Plan == model.PlanEnterprise {
				if err := q.VoidOpenPlanInvoices(ctx, t.ID); err != nil {
					return err
				}
			}
			if err := q.SetEnterpriseOffer(ctx, db.SetEnterpriseOfferParams{ID: t.ID}); err != nil {
				return err
			}
			s.audit(ctx, q, operator, "enterprise_offer_withdrawn", "tenant", t.ID, nil, map[string]any{"company": t.Name})
			return nil
		}
		price, drivers, texts := priceETB*100, int32(maxDrivers), int32(smsQuota)
		if err := q.SetEnterpriseOffer(ctx, db.SetEnterpriseOfferParams{ID: t.ID, PriceSantim: &price, MaxDrivers: &drivers, SmsQuota: &texts}); err != nil {
			return err
		}
		if t.Plan == model.PlanEnterprise {
			sub := subscriptionOf(t)
			sub.MaxDrivers, sub.SmsQuota = drivers, texts
			if err := q.SaveSubscription(ctx, sub); err != nil {
				return err
			}
		}
		s.audit(ctx, q, operator, "enterprise_offer_set", "tenant", t.ID, nil, map[string]any{
			"company": t.Name, "monthly": payments.FormatETB(price), "max_drivers": drivers, "sms_quota": texts,
		})
		intro := fmt.Sprintf("We've set up Enterprise terms for %s: %s a month plus VAT, for up to %d drivers and %s texts a month (%s a year if billed annually).",
			t.Name, money(price), drivers, strconv.Itoa(int(texts)), money(model.PeriodPrice(price, model.CycleAnnual)))
		next := "To take them up, choose Enterprise on the Billing page: " + s.cfg.BaseURL + "/app/billing"
		if t.Plan == model.PlanEnterprise {
			next = "The new limits apply now, and the new price from your next billing period."
		}
		return s.emailBilling(ctx, q, after, t.ID, EmailBilling, "Your Enterprise terms", intro, next)
	})
}

// PlatformBilling is the platform operator's view of the money.
type PlatformBilling struct {
	// MRR is monthly recurring revenue, in santim excluding VAT: an annual
	// plan counts as a twelfth of its price.
	MRR       int64
	Paying    int
	Collected int64 // this calendar month, including VAT
	Open      []db.ListAllInvoicesRow
	OpenTotal int64
	Overdue   int
	Payments  []db.ListRecentPaymentsRow
	// Refunds are payments for invoices already paid or cancelled, still
	// to be sent back.
	Refunds []db.ListRefundsDueRow
}

// Platform reports revenue, unpaid invoices and recent payments.
func (s *BillingService) Platform(ctx context.Context, p *Principal) (*PlatformBilling, error) {
	if !p.IsPlatformAdmin() {
		return nil, ErrUnauthorized
	}
	tenants, err := s.q.ListTenantsWithStats(ctx)
	if err != nil {
		return nil, err
	}
	out := &PlatformBilling{}
	for _, t := range tenants {
		if !t.IsActive || t.Plan == model.PlanTrial || !t.PeriodEnd.Valid {
			continue
		}
		price := monthlyPrice(t.Plan, t.EnterprisePriceSantim)
		if t.BillingCycle == model.CycleAnnual {
			price = price * model.AnnualMonths / 12
		}
		out.MRR += price
		out.Paying++
	}
	now := s.now()
	if out.Collected, err = s.q.SumPaidSince(ctx, toTimestamp(monthStart(now, s.cfg.Location))); err != nil {
		return nil, err
	}
	if out.Open, err = s.q.ListAllInvoices(ctx, db.ListAllInvoicesParams{Status: "open", MaxRows: 200}); err != nil {
		return nil, err
	}
	for _, inv := range out.Open {
		out.OpenTotal += inv.TotalSantim
		if inv.DueAt.Time.Before(now) {
			out.Overdue++
		}
	}
	if out.Payments, err = s.q.ListRecentPayments(ctx, 50); err != nil {
		return nil, err
	}
	out.Refunds, err = s.q.ListRefundsDue(ctx)
	return out, err
}

// ---------------------------------------------------------------------------
// The billing job
// ---------------------------------------------------------------------------

// Run is the billing part of the periodic jobs: payments the provider
// hasn't reported on, checkouts nobody finished, plan invoices nobody paid,
// renewals, text overage and the billing emails.
func (s *BillingService) Run(ctx context.Context) error {
	if err := s.reconcile(ctx); err != nil {
		return err
	}
	now := s.now()
	if err := s.q.ExpirePendingPayments(ctx, toTimestamp(now.Add(-paymentPromptTTL))); err != nil {
		return err
	}
	if _, err := s.q.VoidLapsedPlanInvoices(ctx, toTimestamp(now)); err != nil {
		return err
	}
	if err := s.Renew(ctx); err != nil {
		return fmt.Errorf("renew: %w", err)
	}
	if err := s.BillUsage(ctx); err != nil {
		return fmt.Errorf("usage: %w", err)
	}
	return s.Remind(ctx)
}

// Renew starts the next period for every company whose period has ended:
// a pending change applies (a downgrade only if the drivers fit), and the
// period's invoice is issued.
func (s *BillingService) Renew(ctx context.Context) error {
	due, err := s.q.ListTenantsToRenew(ctx, toTimestamp(s.now()))
	if err != nil {
		return err
	}
	for _, t := range due {
		if err := s.renew(ctx, t.ID); err != nil {
			s.log.Error().Err(err).Str("tenant", t.ID.String()).Msg("renew subscription")
		}
	}
	return nil
}

func (s *BillingService) renew(ctx context.Context, id pgtype.UUID) error {
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		t, err := q.GetTenantForUpdate(ctx, id)
		if err != nil {
			return err
		}
		now := s.now()
		if !subscribed(t) || t.PeriodEnd.Time.After(now) {
			return nil
		}
		if n, err := q.CountOpenRenewals(ctx, t.ID); err != nil || n > 0 {
			return err
		}
		// An unpaid upgrade was priced for the period that has ended.
		if err := q.VoidOpenPlanInvoices(ctx, t.ID); err != nil {
			return err
		}
		plan, cycle := t.Plan, t.BillingCycle
		if t.PendingCycle != nil {
			cycle = *t.PendingCycle
		}
		var notes []string
		if t.PendingPlan != nil {
			next, err := s.planTerms(t, *t.PendingPlan)
			drivers, cerr := q.CountActiveDriversByTenant(ctx, t.ID)
			if cerr != nil {
				return cerr
			}
			switch {
			case err != nil:
				notes = append(notes, fmt.Sprintf("Your change to the %s plan was cancelled: it is no longer available.", model.PlanName(*t.PendingPlan)))
			case drivers > int64(next.drivers):
				notes = append(notes, fmt.Sprintf("Your change to the %s plan was cancelled: you have %d active drivers and it allows %d.",
					next.plan.Name, drivers, next.drivers))
			default:
				plan = next.plan.Code
			}
		}
		tm, err := s.planTerms(t, plan)
		if err != nil {
			return err
		}
		// The next period follows on from the last, unless that ended more
		// than a day ago: the company was suspended (or the job wasn't
		// running), and time already gone by isn't billed.
		start := t.PeriodEnd.Time
		if now.Sub(start) > lateRenewal {
			start = now
		}
		end := s.periodEnd(start, cycle)
		sub := withTerms(subscriptionOf(t), tm)
		sub.BillingCycle = cycle
		sub.PeriodStart, sub.PeriodEnd = toTimestamp(start), toTimestamp(end)
		sub.PendingPlan, sub.PendingCycle = nil, nil
		if err := q.SaveSubscription(ctx, sub); err != nil {
			return err
		}
		t.Plan, t.BillingCycle, t.MaxDrivers, t.SmsQuota = sub.Plan, sub.BillingCycle, sub.MaxDrivers, sub.SmsQuota
		t.PeriodStart, t.PeriodEnd, t.PendingPlan, t.PendingCycle = sub.PeriodStart, sub.PeriodEnd, nil, nil
		amount := model.PeriodPrice(tm.monthly, cycle)
		intro := strings.Join(append(notes, fmt.Sprintf("Your %s plan renews for %s to %s.", tm.plan.Name, s.date(start), s.date(end))), " ")
		inv, err := s.issueInvoice(ctx, q, after, t, invoiceDraft{
			Kind: invoiceRenewal, Plan: plan, Cycle: cycle, PeriodStart: start, PeriodEnd: end,
			Due:   now.AddDate(0, 0, model.InvoiceDueDays),
			Lines: []InvoiceLine{line(fmt.Sprintf("%s plan, billed %s: %s to %s", tm.plan.Name, cycleAdverb(cycle), s.date(start), s.date(end)), 1, amount)},
			Intro: intro,
		})
		if err != nil {
			return err
		}
		s.audit(ctx, q, SystemActor(t.ID), "subscription_renewed", "invoice", inv.ID, nil,
			map[string]any{"invoice": inv.Number, "plan": plan, "cycle": cycle})
		return nil
	})
}

// BillUsage invoices, once a calendar month is over, the texts a paying
// company sent beyond the month's allowance. Texts are never blocked: a
// customer missing a delivery update costs the company more than the text.
// Trials aren't billed for texts.
func (s *BillingService) BillUsage(ctx context.Context) error {
	current := monthStart(s.now(), s.cfg.Location)
	tenants, err := s.q.ListTenantsForUsageBilling(ctx, toDate(current))
	if err != nil {
		return err
	}
	for _, t := range tenants {
		if err := s.billUsage(ctx, t.ID, current); err != nil {
			s.log.Error().Err(err).Str("tenant", t.ID.String()).Msg("bill text usage")
		}
	}
	return nil
}

func (s *BillingService) billUsage(ctx context.Context, id pgtype.UUID, current time.Time) error {
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		t, err := q.GetTenantForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if t.SmsBilledMonth.Valid && !fromDate(t.SmsBilledMonth, s.cfg.Location).Before(current) {
			return nil
		}
		if t.SmsBilledMonth.Valid && t.Plan != model.PlanTrial && t.SubscribedAt.Valid {
			quota := t.SmsMonthQuota
			for m := fromDate(t.SmsBilledMonth, s.cfg.Location); m.Before(current); m = m.AddDate(0, 1, 0) {
				since, until := m, m.AddDate(0, 1, 0)
				if t.SubscribedAt.Time.After(since) {
					since = t.SubscribedAt.Time
				}
				if since.Before(until) {
					sent, err := q.CountSMSSentBetween(ctx, db.CountSMSSentBetweenParams{TenantID: t.ID, Since: toTimestamp(since), Until: toTimestamp(until)})
					if err != nil {
						return err
					}
					if extra := sent - int64(quota); extra > 0 {
						month := m.Format("January 2006")
						if _, err := s.issueInvoice(ctx, q, after, t, invoiceDraft{
							Kind: invoiceUsage, Plan: t.Plan, Cycle: t.BillingCycle, PeriodStart: since, PeriodEnd: until,
							Due:   s.now().AddDate(0, 0, model.InvoiceDueDays),
							Usage: &SMSUsage{Month: m.Format("2006-01"), Sent: sent, Included: quota, Extra: extra},
							Lines: []InvoiceLine{line(fmt.Sprintf("Text messages beyond the %s included in %s", grouped(int64(quota)), month), extra, model.SMSOverageSantim)},
							Intro: fmt.Sprintf("In %s, %s sent %s text messages, %s more than the %s your plan includes.",
								month, t.Name, grouped(sent), grouped(extra), grouped(int64(quota))),
						}); err != nil {
							return err
						}
					}
				}
				quota = t.SmsQuota
			}
		}
		return q.SetSMSBilledMonth(ctx, db.SetSMSBilledMonthParams{ID: t.ID, SmsBilledMonth: toDate(current)})
	})
}

// Remind sends the billing emails that are due: a trial ending in three
// days, in one, and ended; an invoice overdue; a company gone read-only.
// Each is sent once per trial end or invoice.
func (s *BillingService) Remind(ctx context.Context) error {
	now := s.now()
	trials, err := s.q.ListTrialsEnding(ctx, toTimestamp(now.Add(trialNoticeWindow)))
	if err != nil {
		return err
	}
	for _, t := range trials {
		if err := s.trialNotice(ctx, t, now); err != nil {
			s.log.Error().Err(err).Str("tenant", t.ID.String()).Msg("trial notice")
		}
	}
	debts, err := s.q.ListOverdueDebts(ctx, toTimestamp(now))
	if err != nil {
		return err
	}
	for _, inv := range debts {
		if err := s.debtNotice(ctx, inv, now); err != nil {
			s.log.Error().Err(err).Str("invoice", inv.Number).Msg("overdue notice")
		}
	}
	return nil
}

func (s *BillingService) trialNotice(ctx context.Context, t db.Tenant, now time.Time) error {
	end := t.TrialEndsAt.Time
	billing := s.cfg.BaseURL + "/app/billing"
	from := money(model.Plans[1].MonthlyETB * 100)
	kind, subject := noticeTrial3Days, "Your free trial ends on "+s.date(end)
	paragraphs := []string{
		fmt.Sprintf("Your free trial of %s ends on %s. To keep booking shipments after that, choose a plan: from %s a month plus VAT.", s.cfg.AppName, s.when(end), from),
		billing,
	}
	switch {
	case !now.Before(end):
		kind, subject = noticeTrialEnded, "Your free trial has ended"
		paragraphs = []string{
			fmt.Sprintf("Your free trial of %s ended on %s, so %s's account is read-only: shipments already on the road carry on and nothing is deleted, but booking new shipments and adding drivers wait until you choose a plan.", s.cfg.AppName, s.when(end), t.Name),
			"Plans start at " + from + " a month plus VAT: " + billing,
		}
	case end.Sub(now) <= 24*time.Hour:
		kind, subject = noticeTrial1Day, "Your free trial ends tomorrow"
		if end.In(s.cfg.Location).YearDay() == now.In(s.cfg.Location).YearDay() {
			subject = "Your free trial ends today"
		}
	}
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		n, err := q.AddBillingNotice(ctx, db.AddBillingNoticeParams{TenantID: t.ID, Kind: kind, Episode: strconv.FormatInt(end.Unix(), 10)})
		if err != nil || n == 0 {
			return err
		}
		return s.emailBilling(ctx, q, after, t.ID, EmailBilling, subject, paragraphs...)
	})
}

func (s *BillingService) debtNotice(ctx context.Context, inv db.Invoice, now time.Time) error {
	readOnlyAt := inv.DueAt.Time.AddDate(0, 0, model.GraceDays)
	kind, subject := noticeOverdue, "Invoice "+inv.Number+" is overdue"
	paragraphs := []string{
		fmt.Sprintf("Invoice %s for %s was due on %s and is still unpaid. If it is not paid by %s, your account becomes read-only: shipments on the road carry on, but booking new ones waits until it is paid.",
			inv.Number, money(inv.TotalSantim), s.date(inv.DueAt.Time), s.when(readOnlyAt)),
		"Pay it here: " + s.invoiceURL(inv),
	}
	if !now.Before(readOnlyAt) {
		kind, subject = noticeReadOnly, "Your account is read-only"
		paragraphs = []string{
			fmt.Sprintf("Invoice %s for %s is still unpaid, so your account is now read-only: shipments already on the road carry on and nothing is deleted, but booking new shipments and adding drivers wait until it is paid.",
				inv.Number, money(inv.TotalSantim)),
			"Pay it here: " + s.invoiceURL(inv),
		}
	}
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		n, err := q.AddBillingNotice(ctx, db.AddBillingNoticeParams{TenantID: inv.TenantID, Kind: kind, Episode: inv.Number})
		if err != nil || n == 0 {
			return err
		}
		return s.emailBilling(ctx, q, after, inv.TenantID, EmailBilling, subject, paragraphs...)
	})
}

// CheckUsage tells dispatchers when the company uses 80% of its driver
// places, once per billing period, or of the month's texts, once a month.
func (s *BillingService) CheckUsage(ctx context.Context) error {
	month := monthStart(s.now(), s.cfg.Location)
	near, err := s.q.ListTenantsNearLimits(ctx, toTimestamp(month))
	if err != nil || len(near) == 0 {
		return err
	}
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		for _, t := range near {
			plan := model.PlanName(t.Plan)
			if t.MaxDrivers > 0 && t.ActiveDrivers*5 >= int64(t.MaxDrivers)*4 {
				period := "trial"
				if t.PeriodStart.Valid {
					period = strconv.FormatInt(t.PeriodStart.Time.Unix(), 10)
				}
				s.raise(ctx, q, after, newAlert{
					TenantID: t.ID, Type: AlertDriverLimitNear, Severity: SeverityInfo,
					Message: fmt.Sprintf("%d of the %d driver places on your %s plan are in use. An administrator can move to a bigger plan on the Billing page.",
						t.ActiveDrivers, t.MaxDrivers, plan),
					Episode: fmt.Sprintf("%d/%s", t.MaxDrivers, period),
				})
			}
			if t.SmsMonthQuota > 0 && t.SmsSent*5 >= int64(t.SmsMonthQuota)*4 {
				msg := fmt.Sprintf("%s of the %s text messages included this month have been sent.", grouped(t.SmsSent), grouped(int64(t.SmsMonthQuota)))
				if t.Plan != model.PlanTrial {
					msg += fmt.Sprintf(" Texts keep going out beyond that, at %s each on next month's invoice.", money(model.SMSOverageSantim))
				}
				s.raise(ctx, q, after, newAlert{
					TenantID: t.ID, Type: AlertSMSQuotaNear, Severity: SeverityInfo, Message: msg,
					Episode: fmt.Sprintf("%s/%d", month.Format("2006-01"), t.SmsMonthQuota),
				})
			}
		}
		return nil
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// emailBilling sends a billing email to the company's admins: those who
// have confirmed their address or, while none has, every admin with one.
func (c *core) emailBilling(ctx context.Context, q *db.Queries, after *afterCommit, tenantID pgtype.UUID, purpose, subject string, paragraphs ...string) error {
	contacts, err := q.ListBillingContacts(ctx, tenantID)
	if err != nil {
		return err
	}
	var verified []db.ListBillingContactsRow
	for _, ct := range contacts {
		if ct.Verified {
			verified = append(verified, ct)
		}
	}
	if len(verified) > 0 {
		contacts = verified
	}
	for _, ct := range contacts {
		c.enqueueEmail(ctx, q, after, outboundEmail{
			TenantID: tenantID, To: ct.Email, Purpose: purpose, Subject: subject, Body: c.emailBody(ct.FullName, paragraphs...),
		})
	}
	return nil
}

// periodEnd is when a period starting at start ends, counting days where
// the companies are rather than where the server is.
func (c *core) periodEnd(start time.Time, cycle string) time.Time {
	return model.PeriodEnd(start.In(c.cfg.Location), cycle)
}

func (c *core) date(t time.Time) string { return t.In(c.cfg.Location).Format("2 Jan 2006") }

func (c *core) when(t time.Time) string { return t.In(c.cfg.Location).Format("2 Jan 2006 at 15:04") }

// money writes santim as birr: "ETB 2,875.00".
func money(santim int64) string { return "ETB " + payments.FormatETB(santim) }

func grouped(n int64) string {
	sign, digits := "", strconv.FormatInt(n, 10)
	if n < 0 {
		sign, digits = "-", digits[1:]
	}
	var b strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return sign + b.String()
}

func cycleAdverb(cycle string) string {
	if cycle == model.CycleAnnual {
		return "annually"
	}
	return "monthly"
}

// firstPeriod describes a first period, which starts when it is paid for.
func firstPeriod(plan, cycle string) string {
	if cycle == model.CycleAnnual {
		return plan + " plan, billed annually: one year from payment (12 months for the price of 10)"
	}
	return plan + " plan, billed monthly: one month from payment"
}

// monthStart is the first moment of t's calendar month in loc.
func monthStart(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
}

func toDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
}

// fromDate is the start of a calendar date in loc.
func fromDate(d pgtype.Date, loc *time.Location) time.Time {
	return time.Date(d.Time.Year(), d.Time.Month(), d.Time.Day(), 0, 0, 0, 0, loc)
}
