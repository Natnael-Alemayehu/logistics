package service

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/phone"
	"github.com/natnael-alemayehu/kiflet/pkg/payments"
)

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

// Settle applies a provider's report on a payment Kiflet started. Reports
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
		sub.BillingAnchorDay = s.freshAnchor(start)
		end = s.periodEnd(start, inv.BillingCycle, *sub.BillingAnchorDay)
		sub = withTerms(sub, s.termsOrDefault(t, inv.Plan))
		sub.BillingCycle = inv.BillingCycle
		sub.PeriodStart, sub.PeriodEnd, sub.SubscribedAt = toTimestamp(start), toTimestamp(end), toTimestamp(start)
		sub.PendingPlan, sub.PendingCycle = nil, nil
		sub.RenewalCancelledAt = pgtype.Timestamptz{}
	case invoiceUpgrade:
		sub = withTerms(sub, s.termsOrDefault(t, inv.Plan))
		sub.PendingPlan = nil
	case invoiceRenewal:
		if !paidAt.Before(inv.DueAt.Time.AddDate(0, 0, model.GraceDays)) {
			sub.BillingAnchorDay = s.freshAnchor(paidAt)
			start, end = paidAt, s.periodEnd(paidAt, t.BillingCycle, *sub.BillingAnchorDay)
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
