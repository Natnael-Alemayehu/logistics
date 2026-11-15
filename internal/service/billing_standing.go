package service

import (
	"context"
	"fmt"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
)

// Standing states.
const (
	StandingTrialing   = "trialing"
	StandingTrialEnded = "trial_ended"
	StandingActive     = "active"
	StandingDue        = "due"
	StandingPastDue    = "past_due"
	StandingReadOnly   = "read_only"
	StandingEnded      = "ended"
)

// Standing is where a company is with paying for the product. Three states
// make it read-only: a trial that has ended, an invoice still unpaid
// GraceDays after it was due, and a subscription whose renewal was turned
// off and whose last period is over.
type Standing struct {
	State string
	// TrialEnds is when a trial ends or ended.
	TrialEnds time.Time
	// Ends is when a subscription that won't renew ends or ended; zero
	// while it renews.
	Ends time.Time
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
	return s.State == StandingTrialEnded || s.State == StandingReadOnly || s.State == StandingEnded
}

// standingOf works out a company's standing. ends is when its subscription
// ends, if renewal was turned off.
func standingOf(plan string, trialEnds, ends *time.Time, debtNumber string, debtDue *time.Time, now time.Time) Standing {
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
	if ends != nil {
		st.Ends = *ends
		if !now.Before(*ends) {
			st.State = StandingEnded
			return st
		}
	}
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
	return standingOf(p.TenantPlan, p.TrialEnds, p.SubscriptionEnds, p.DebtNumber, p.DebtDue, now)
}

// subscriptionEnds is when t's subscription ends, if its renewal was
// turned off.
func subscriptionEnds(t db.Tenant) *time.Time {
	if !t.RenewalCancelledAt.Valid || !t.PeriodEnd.Valid {
		return nil
	}
	return &t.PeriodEnd.Time
}

// ended reports whether t's subscription was not renewed and is over.
func ended(t db.Tenant, now time.Time) bool {
	e := subscriptionEnds(t)
	return e != nil && !now.Before(*e)
}

var errTrialEnded = &Error{Kind: KindLimit, Message: "Your company's free trial has ended, so the account is read-only: " +
	"shipments on the road carry on, but booking or assigning shipments and adding drivers, vehicles or staff " +
	"wait until an administrator chooses a plan on the Billing page. Nothing has been deleted."}

var errSubscriptionEnded = &Error{Kind: KindLimit, Message: "Your company's subscription has ended, so the account is read-only: " +
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
	switch standingOf(t.Plan, timePtr(t.TrialEndsAt), subscriptionEnds(t), debtNumber, debtDue, c.now()).State {
	case StandingTrialEnded:
		return errTrialEnded
	case StandingEnded:
		return errSubscriptionEnded
	case StandingReadOnly:
		return limitReached(fmt.Sprintf("Invoice %s is unpaid, so your company's account is read-only: "+
			"shipments on the road carry on, but booking or assigning shipments and adding drivers, vehicles or staff "+
			"wait until it is paid on the Billing page.", debtNumber))
	}
	return nil
}
