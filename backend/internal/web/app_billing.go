package web

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/internal/service"
	"github.com/natnael-alemayehu/logistics/pkg/payments"
)

// ---------------------------------------------------------------------------
// A company's billing (FR-043 to FR-045)
// ---------------------------------------------------------------------------

func (s *Server) billingPage(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	o, err := s.svc.Billing.Overview(r.Context(), p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	usage, err := s.svc.Tenants.Usage(r.Context(), p.TenantID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(w, r, "Billing", "billing", map[string]any{"O": o, "Usage": usage, "VAT": model.VATPercent})
	v.Form = url.Values{"cycle": {o.Tenant.BillingCycle}}
	if o.Tenant.PendingCycle != nil {
		v.Form.Set("cycle", *o.Tenant.PendingCycle)
	}
	s.render(w, r, http.StatusOK, "app/billing", v)
}

func (s *Server) billingChoose(w http.ResponseWriter, r *http.Request) {
	change, err := s.svc.Billing.ChoosePlan(r.Context(), principal(r), r.PostFormValue("plan"), r.PostFormValue("cycle"), clientInfo(r))
	if err != nil {
		s.actionResult(w, r, "/app/billing", err, "")
		return
	}
	if change.Invoice != nil {
		s.setFlash(w, "success", "Invoice "+change.Invoice.Number+" is ready. The new plan applies as soon as it is paid.")
		http.Redirect(w, r, "/app/billing/invoices/"+change.Invoice.ID.String(), http.StatusSeeOther)
		return
	}
	s.back(w, r, "/app/billing", "success", change.Message)
}

func (s *Server) invoicePage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	d, err := s.svc.Billing.Invoice(r.Context(), principal(r), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderInvoice(w, r, d, false)
}

func (s *Server) renderInvoice(w http.ResponseWriter, r *http.Request, d *service.InvoiceDetail, platform bool) {
	nav := "billing"
	if platform {
		nav = "platform-billing"
	}
	v := s.view(w, r, "Invoice "+d.Invoice.Number, nav, map[string]any{
		"D": d, "Platform": platform, "Providers": s.svc.Billing.Providers(), "VAT": model.VATPercent,
	})
	if p := principal(r); p != nil {
		v.Form = url.Values{"phone": {p.Phone}}
	}
	if len(s.svc.Billing.Providers()) > 0 {
		v.Form.Set("provider", s.svc.Billing.Providers()[0].Code())
	}
	s.render(w, r, http.StatusOK, "app/invoice", v)
}

func (s *Server) invoicePay(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	pay, err := s.svc.Billing.Pay(r.Context(), principal(r), id, r.PostFormValue("provider"), r.PostFormValue("phone"), clientInfo(r))
	if err != nil {
		s.actionResult(w, r, "/app/billing/invoices/"+id.String(), err, "")
		return
	}
	http.Redirect(w, r, "/app/billing/payments/"+pay.ID.String(), http.StatusSeeOther)
}

// paymentPage waits for the payer to approve the prompt on their phone. It
// reloads itself until the provider reports back.
func (s *Server) paymentPage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	pay, err := s.svc.Billing.Payment(r.Context(), principal(r), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	label := pay.Provider
	if prov, ok := s.svc.Billing.Provider(pay.Provider); ok {
		label = prov.Label()
	}
	v := s.view(w, r, "Payment", "billing", map[string]any{"Pay": pay, "Label": label})
	s.render(w, r, http.StatusOK, "app/payment", v)
}

// ---------------------------------------------------------------------------
// Payment providers
// ---------------------------------------------------------------------------

// apiPaymentNotify receives a provider's report on a payment. The provider
// signs it; anything unsigned or unknown is refused, and repeats are
// harmless.
func (s *Server) apiPaymentNotify(w http.ResponseWriter, r *http.Request) {
	prov, ok := s.svc.Billing.Provider(chi.URLParam(r, "provider"))
	if !ok {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "Unknown payment provider.")
		return
	}
	n, err := prov.Verify(r)
	if err != nil {
		s.log.Warn().Err(err).Str("provider", prov.Code()).Msg("payment notification refused")
		writeJSONError(w, http.StatusUnauthorized, "BAD_SIGNATURE", "The notification could not be verified.")
		return
	}
	if err := s.svc.Billing.PaymentNotified(r.Context(), prov.Code(), n); err != nil {
		s.apiError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// devPaymentPage stands in for the payment prompt on the payer's phone
// while payments are simulated. The terminal prints its link.
func (s *Server) devPaymentPage(w http.ResponseWriter, r *http.Request) {
	pay, prov, err := s.svc.Billing.PaymentPrompt(r.Context(), chi.URLParam(r, "reference"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(w, r, prov.Label()+" payment", "", map[string]any{"Pay": pay, "Label": prov.Label()})
	s.render(w, r, http.StatusOK, "public/pay_prompt", v)
}

// devPaymentAnswer approves or declines a simulated payment the way the
// provider would: a signed notification, checked and applied by the same
// code that handles a real provider's.
func (s *Server) devPaymentAnswer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pay, prov, err := s.svc.Billing.PaymentPrompt(ctx, chi.URLParam(r, "reference"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	signer, ok := prov.(interface {
		Sign(payments.Notification) string
	})
	if !ok {
		s.notFound(w, r)
		return
	}
	n := payments.Notification{Reference: pay.Reference, ProviderRef: deref(pay.ProviderRef), Status: payments.StatusFailed, Amount: pay.AmountSantim}
	if r.PostFormValue("answer") == "approve" {
		n.Status = payments.StatusPaid
	}
	form := url.Values{
		"reference": {n.Reference}, "provider_ref": {n.ProviderRef}, "status": {n.Status},
		"amount": {strconv.FormatInt(n.Amount, 10)}, "signature": {signer.Sign(n)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/payments/"+prov.Code()+"/notify", strings.NewReader(form.Encode()))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	verified, err := prov.Verify(req)
	if err == nil {
		err = s.svc.Billing.PaymentNotified(ctx, prov.Code(), verified)
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	pay, _, err = s.svc.Billing.PaymentPrompt(ctx, pay.Reference)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(w, r, prov.Label()+" payment", "", map[string]any{"Pay": pay, "Label": prov.Label(), "Answered": true})
	s.render(w, r, http.StatusOK, "public/pay_prompt", v)
}

// ---------------------------------------------------------------------------
// The platform operator
// ---------------------------------------------------------------------------

func (s *Server) platformBilling(w http.ResponseWriter, r *http.Request) {
	b, err := s.svc.Billing.Platform(r.Context(), principal(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "app/platform_billing", s.view(w, r, "Billing", "platform-billing", b))
}

func (s *Server) platformInvoice(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	d, err := s.svc.Billing.PlatformInvoice(r.Context(), principal(r), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderInvoice(w, r, d, true)
}

func (s *Server) platformMarkPaid(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	err = s.svc.Billing.MarkPaid(r.Context(), principal(r), id, r.PostFormValue("reference"), r.PostFormValue("note"), clientInfo(r))
	s.actionResult(w, r, "/app/platform/billing", err, "Payment recorded. The company has its receipt.")
}

func (s *Server) platformVoid(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	err = s.svc.Billing.VoidInvoice(r.Context(), principal(r), id, r.PostFormValue("reason"), clientInfo(r))
	s.actionResult(w, r, "/app/platform/billing", err, "Invoice cancelled.")
}

func (s *Server) platformRefunded(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	err = s.svc.Billing.MarkRefunded(r.Context(), principal(r), id, r.PostFormValue("reference"), clientInfo(r))
	s.actionResult(w, r, "/app/platform/billing", err, "Refund recorded. The company has been told.")
}

func (s *Server) platformExtendTrial(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	days, _ := strconv.Atoi(r.PostFormValue("days"))
	err = s.svc.Billing.ExtendTrial(r.Context(), principal(r), id, days, clientInfo(r))
	s.actionResult(w, r, "/app/platform", err, "Trial extended. The company has been told.")
}

func (s *Server) platformOffer(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	// An empty price withdraws the offer.
	raw := strings.ReplaceAll(strings.TrimSpace(r.PostFormValue("price")), ",", "")
	var price int64
	if raw != "" {
		if price, err = strconv.ParseInt(raw, 10, 64); err != nil {
			s.actionResult(w, r, "/app/platform", &service.Error{Kind: service.KindInvalid, Message: "Enter the monthly price in whole birr."}, "")
			return
		}
	}
	drivers, _ := strconv.Atoi(r.PostFormValue("max_drivers"))
	texts, _ := strconv.Atoi(r.PostFormValue("sms_quota"))
	err = s.svc.Billing.SetEnterpriseOffer(r.Context(), principal(r), id, price, drivers, texts, clientInfo(r))
	msg := "Enterprise terms saved and sent to the company."
	if price == 0 {
		msg = "Enterprise offer withdrawn."
	}
	s.actionResult(w, r, "/app/platform", err, msg)
}
