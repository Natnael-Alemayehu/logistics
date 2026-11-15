package service

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
)

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
		now := s.now()
		fresh := !subscribed(t) || ended(t, now)
		if !fresh && t.RenewalCancelledAt.Valid {
			return conflict(fmt.Sprintf("Your plan ends on %s and won't renew. Turn renewal back on first, then change your plan.", s.date(t.PeriodEnd.Time)))
		}
		drivers, err := q.CountActiveDriversByTenant(ctx, t.ID)
		if err != nil {
			return err
		}
		if drivers > int64(next.drivers) && (code != t.Plan || fresh) {
			return limitReached(fmt.Sprintf("You have %d active drivers and the %s plan allows %d. Deactivate %d first.",
				drivers, next.plan.Name, next.drivers, drivers-int64(next.drivers)))
		}
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

		// A trial, or a subscription that ended without renewing, starts
		// a fresh one.
		if fresh {
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
// CancelRenewal turns off a company's renewal, for its admin. The plan runs
// to the end of the period already paid for, then the account becomes
// read-only until a plan is chosen again. A renewal invoice not yet paid is
// cancelled with the period it was for, so nothing is owed for it and the
// plan ends at once.
func (s *BillingService) CancelRenewal(ctx context.Context, p *Principal, ci ClientInfo) (string, error) {
	if !p.CanManageCompany() {
		return "", ErrUnauthorized
	}
	var msg string
	err := s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		t, err := q.GetTenantForUpdate(ctx, p.TenantID)
		if err != nil {
			return err
		}
		now := s.now()
		if !subscribed(t) || ended(t, now) {
			return conflict("There's no subscription renewing to cancel.")
		}
		if t.RenewalCancelledAt.Valid {
			return conflict(fmt.Sprintf("Renewal is already off: your plan ends on %s.", s.date(t.PeriodEnd.Time)))
		}
		sub := subscriptionOf(t)
		sub.PendingPlan, sub.PendingCycle = nil, nil
		sub.RenewalCancelledAt = toTimestamp(now)
		var voided *db.Invoice
		switch inv, err := q.GetOpenRenewal(ctx, t.ID); {
		case err == nil:
			if _, err := q.VoidInvoice(ctx, db.VoidInvoiceParams{ID: inv.ID, TenantID: t.ID}); err != nil {
				return err
			}
			if inv.PeriodStart.Valid {
				sub.PeriodStart, sub.PeriodEnd = inv.PeriodStart, inv.PeriodStart
			}
			voided = &inv
		case !isNoRows(err):
			return err
		}
		if err := q.SaveSubscription(ctx, sub); err != nil {
			return err
		}
		change := map[string]any{"ends": sub.PeriodEnd.Time}
		if voided != nil {
			change["invoice_voided"] = voided.Number
		}
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "renewal_cancelled", "tenant", t.ID,
			map[string]any{"plan": t.Plan, "period_end": t.PeriodEnd.Time}, change)

		plan, ends := model.PlanName(t.Plan), s.date(sub.PeriodEnd.Time)
		var paragraphs []string
		if voided != nil {
			msg = fmt.Sprintf("Your %s plan has ended and invoice %s is cancelled. The account is read-only until you choose a plan again.", plan, voided.Number)
			paragraphs = []string{
				fmt.Sprintf("%s turned off renewal of %s's %s plan. Invoice %s, for the period from %s, is cancelled: there is nothing to pay on it.",
					p.Name, t.Name, plan, voided.Number, ends),
				"The plan ended on " + ends + ", the end of the last period paid for, so the account is now read-only: shipments on the road carry on and nothing is deleted, but booking shipments and adding drivers wait until a plan is chosen again.",
			}
		} else {
			msg = fmt.Sprintf("Your %s plan won't renew. It runs until %s; after that the account is read-only until you choose a plan again.", plan, ends)
			paragraphs = []string{
				fmt.Sprintf("%s turned off renewal of %s's %s plan. It runs until %s and no invoice will be issued for after that.", p.Name, t.Name, plan, ends),
				"From " + ends + " the account is read-only: shipments on the road carry on and nothing is deleted, but booking shipments and adding drivers wait until a plan is chosen again. Until then, renewal can be turned back on on the Billing page.",
			}
		}
		paragraphs = append(paragraphs, s.cfg.BaseURL+"/app/billing")
		return s.emailBilling(ctx, q, after, t.ID, EmailBilling, "Your plan won't renew", paragraphs...)
	})
	return msg, err
}

// ResumeRenewal turns renewal back on while the period is still running.
// Once it has ended, the company chooses a plan instead.
func (s *BillingService) ResumeRenewal(ctx context.Context, p *Principal, ci ClientInfo) (string, error) {
	if !p.CanManageCompany() {
		return "", ErrUnauthorized
	}
	var msg string
	err := s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		t, err := q.GetTenantForUpdate(ctx, p.TenantID)
		if err != nil {
			return err
		}
		switch {
		case !subscribed(t) || !t.RenewalCancelledAt.Valid:
			return conflict("Your plan already renews.")
		case ended(t, s.now()):
			return conflict("Your plan has ended. Choose a plan below to start again.")
		}
		sub := subscriptionOf(t)
		sub.RenewalCancelledAt = pgtype.Timestamptz{}
		if err := q.SaveSubscription(ctx, sub); err != nil {
			return err
		}
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "renewal_resumed", "tenant", t.ID, nil,
			map[string]any{"plan": t.Plan, "renews": t.PeriodEnd.Time})
		msg = fmt.Sprintf("Your %s plan renews on %s as before.", model.PlanName(t.Plan), s.date(t.PeriodEnd.Time))
		return nil
	})
	return msg, err
}

func (s *BillingService) trialNote(t db.Tenant, now time.Time) string {
	if t.Plan == model.PlanTrial && t.TrialEndsAt.Valid && t.TrialEndsAt.Time.After(now) {
		return ", and not before your free trial ends on " + s.date(t.TrialEndsAt.Time)
	}
	return ""
}
