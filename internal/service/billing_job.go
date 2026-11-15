package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
)

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
		if !subscribed(t) || t.PeriodEnd.Time.After(now) || t.RenewalCancelledAt.Valid {
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
		anchor := s.anchorOf(t)
		if now.Sub(start) > lateRenewal {
			start = now
			anchor = *s.freshAnchor(now)
		}
		end := s.periodEnd(start, cycle, anchor)
		sub := withTerms(subscriptionOf(t), tm)
		sub.BillingAnchorDay = &anchor
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
							Lines: []InvoiceLine{line(fmt.Sprintf("Text messages beyond the %s included in %s, counted in SMS parts (a long text is several)", grouped(int64(quota)), month), extra, model.SMSOverageSantim)},
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
