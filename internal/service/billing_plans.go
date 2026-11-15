package service

import (
	"context"
	"fmt"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
)

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
		PendingPlan: t.PendingPlan, PendingCycle: t.PendingCycle, BillingAnchorDay: t.BillingAnchorDay,
		RenewalCancelledAt: t.RenewalCancelledAt,
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
	Tenant   db.Tenant
	Standing Standing
	Current  PlanOption
	// Subscribed is a paid period running now, renewing or not.
	Subscribed bool
	// Ending is a subscription whose renewal was turned off, still running.
	Ending        bool
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
	now := s.now()
	o := &BillingOverview{Tenant: t, Subscribed: subscribed(t) && !ended(t, now)}
	o.Ending = o.Subscribed && t.RenewalCancelledAt.Valid
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
	o.Standing = standingOf(t.Plan, timePtr(t.TrialEndsAt), subscriptionEnds(t), debtNumber, debtDue, now)

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
		case o.Ending:
			opt.Blocked = "Turn renewal back on first."
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
	if !o.Subscribed && t.Plan == model.PlanTrial {
		trial, _ := model.PlanByCode(model.PlanTrial)
		o.Current = PlanOption{Code: trial.Code, Name: trial.Name, MaxDrivers: t.MaxDrivers, SMSIncluded: t.SmsQuota, Current: true}
	}
	return o, nil
}
