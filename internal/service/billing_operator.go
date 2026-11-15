package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/pkg/payments"
)

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
