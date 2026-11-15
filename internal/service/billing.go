package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/pkg/payments"
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

// periodEnd is when a period starting at start ends, on the billing day
// (0 for start's own), counting days where the companies are rather than
// where the server is.
func (c *core) periodEnd(start time.Time, cycle string, anchorDay int32) time.Time {
	return model.PeriodEnd(start.In(c.cfg.Location), cycle, int(anchorDay))
}

// freshAnchor is the billing day of a period starting afresh at start.
func (c *core) freshAnchor(start time.Time) *int32 {
	d := int32(start.In(c.cfg.Location).Day())
	return &d
}

// anchorOf is a company's billing day, worked out for one whose day wasn't
// recorded.
func (c *core) anchorOf(t db.Tenant) int32 {
	if t.BillingAnchorDay != nil {
		return *t.BillingAnchorDay
	}
	return int32(model.AnchorDay(t.PeriodStart.Time.In(c.cfg.Location), t.SubscribedAt.Time.In(c.cfg.Location)))
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
