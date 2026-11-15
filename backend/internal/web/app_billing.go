package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/service"
	"github.com/natnael-alemayehu/kiflet/pkg/payments"
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

func (s *Server) billingCancel(w http.ResponseWriter, r *http.Request) {
	msg, err := s.svc.Billing.CancelRenewal(r.Context(), principal(r), clientInfo(r))
	s.actionResult(w, r, "/app/billing", err, msg)
}

func (s *Server) billingResume(w http.ResponseWriter, r *http.Request) {
	msg, err := s.svc.Billing.ResumeRenewal(r.Context(), principal(r), clientInfo(r))
	s.actionResult(w, r, "/app/billing", err, msg)
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

// payOption is a way to pay an invoice, as its page offers it.
type payOption struct {
	Code, Label string
	// Checkout providers take the payer to their page.
	Checkout bool
	// Otherwise the payer pays MerchantID from their phone, quoting the
	// invoice's reference.
	MerchantID string
	// Simulated is the page that plays the payer's side, when it is played.
	Simulated string
}

func (s *Server) payOptions(r *http.Request, d *service.InvoiceDetail) []payOption {
	var out []payOption
	for _, p := range s.svc.Billing.Providers() {
		o := payOption{Code: p.Code(), Label: p.Label()}
		switch prov := p.(type) {
		case payments.Starter:
			o.Checkout = true
		case payments.Referenced:
			o.MerchantID = prov.MerchantID()
			ref := url.Values{"merchant": {o.MerchantID}, "ref": {d.PayReference()}}
			switch {
			case p.Mock():
				o.Simulated = "/dev/" + p.Code() + "?" + ref.Encode()
			case s.cfg.SandboxURL != "":
				msisdn := "0911000000"
				if me := principal(r); me != nil && me.Phone != "" {
					msisdn = me.Phone
				}
				o.Simulated = s.cfg.SandboxURL + "/phones/" + url.PathEscape(msisdn) + "?" + ref.Encode()
			}
		default:
			continue
		}
		out = append(out, o)
	}
	return out
}

func (s *Server) renderInvoice(w http.ResponseWriter, r *http.Request, d *service.InvoiceDetail, platform bool) {
	nav := "billing"
	if platform {
		nav = "platform-billing"
	}
	labels := map[string]string{"bank": "Bank transfer"}
	for _, p := range s.svc.Billing.Providers() {
		labels[p.Code()] = p.Label()
	}
	v := s.view(w, r, "Invoice "+d.Invoice.Number, nav, map[string]any{
		"D": d, "Platform": platform, "Options": s.payOptions(r, d), "Labels": labels,
		"Sandbox": s.cfg.SandboxURL, "VAT": model.VATPercent,
	})
	s.render(w, r, http.StatusOK, "app/invoice", v)
}

func (s *Server) invoicePay(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	_, checkout, err := s.svc.Billing.Pay(r.Context(), principal(r), id, r.PostFormValue("provider"), clientInfo(r))
	if err != nil {
		s.actionResult(w, r, "/app/billing/invoices/"+id.String(), err, "")
		return
	}
	// To the provider's checkout, which sends the payer back to the payment
	// page. The CSP's form-action lets this redirect through.
	http.Redirect(w, r, checkout, http.StatusSeeOther)
}

// paymentPage is where the provider's checkout sends the payer back to. It
// reloads itself until the provider reports how the payment went, and asks
// the provider in case its report is lost.
func (s *Server) paymentPage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		s.notFound(w, r)
		return
	}
	s.svc.Billing.CheckPayment(r.Context(), principal(r), id)
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

// paymentFail answers a payment provider's call that failed: one that isn't
// signed or logged in gets 401, one that can't be read 400, and billing's
// refusals their usual status.
func (s *Server) paymentFail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, payments.ErrBadSignature), errors.Is(err, payments.ErrUnauthorized):
		s.log.Warn().Err(err).Str("path", r.RequestURI).Msg("payment provider call refused")
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "The call could not be verified.")
	case errors.Is(err, payments.ErrMalformed):
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "The call could not be read.")
	default:
		s.apiError(w, r, err)
	}
}

// devPaymentPage stands in for a checkout provider's payment page while
// payments are simulated. Pay sends Kiflet its signed notification.
func (s *Server) devPaymentPage(w http.ResponseWriter, r *http.Request) {
	pay, prov, err := s.svc.Billing.PaymentPrompt(r.Context(), chi.URLParam(r, "reference"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	v := s.view(w, r, prov.Label()+" payment", "", map[string]any{"Pay": pay, "Label": prov.Label()})
	s.render(w, r, http.StatusOK, "public/pay_prompt", v)
}

// devPaymentAnswer pays or cancels a simulated payment the way the provider
// would: a signed notification to the handler a real provider's reaches,
// then back to the payment page.
func (s *Server) devPaymentAnswer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pay, prov, err := s.svc.Billing.PaymentPrompt(ctx, chi.URLParam(r, "reference"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	mock, ok := prov.(*payments.Mock)
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
		"amount": {strconv.FormatInt(n.Amount, 10)}, "signature": {mock.Sign(n)},
	}
	res, err := call(ctx, mock.Handler(s.svc.Billing, s.paymentFail), "/notify", "application/x-www-form-urlencoded",
		[]byte(form.Encode()), nil)
	if err == nil && res.code != http.StatusOK {
		err = fmt.Errorf("the notification was answered %d: %s", res.code, res.body.String())
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/app/billing/payments/"+pay.ID.String(), http.StatusSeeOther)
}

// devCBEBirr plays CBE Birr while it is a stand-in: the payer keying in the
// merchant ID and the invoice's reference, and CBE Birr's servers making
// their calls to the real handler, logged in as CBE Birr would be.
func (s *Server) devCBEBirr(w http.ResponseWriter, r *http.Request) {
	prov, ok := s.svc.Billing.Provider("cbebirr")
	cbe, isCBE := prov.(*payments.CBEBirr)
	if !ok || !isCBE || !prov.Mock() {
		s.notFound(w, r)
		return
	}
	in := url.Values{"merchant": {r.FormValue("merchant")}, "ref": {r.FormValue("ref")}, "phone": {r.FormValue("phone")}}
	if in.Get("merchant") == "" {
		in.Set("merchant", cbe.MerchantID())
	}
	if in.Get("phone") == "" {
		in.Set("phone", "0911000000")
		if me := principal(r); me != nil && me.Phone != "" {
			in.Set("phone", me.Phone)
		}
	}
	data := map[string]any{"Label": prov.Label()}
	if r.Method == http.MethodPost {
		step, bill, result, problem := s.cbeStep(r.Context(), cbe, r.PostFormValue("step"), in)
		data["Step"], data["Result"], data["Error"] = step, result, problem
		if bill != nil {
			data["Bill"] = bill
			data["Amount"], _ = payments.ParseDecimal(bill.Amount)
		}
	}
	v := s.view(w, r, prov.Label()+" payment", "", data)
	v.Form = in
	s.render(w, r, http.StatusOK, "public/cbebirr_dev", v)
}

// cbeStep makes the calls of one step on the CBE Birr page: look the
// reference up, or pay it (validation, then confirmation).
func (s *Server) cbeStep(ctx context.Context, cbe *payments.CBEBirr, step string, in url.Values) (string, *payments.CBEBirrBill, *payments.CBEBirrResult, string) {
	h := cbe.Handler(s.svc.Billing, s.paymentFail)
	msisdn := in.Get("phone")
	if len(msisdn) == 10 && msisdn[0] == '0' {
		msisdn = "251" + msisdn[1:]
	}
	var bill payments.CBEBirrBill
	q := payments.CBEBirrQuery{BillRefNumber: strings.TrimSpace(in.Get("ref")), BusinessShortCode: strings.TrimSpace(in.Get("merchant")), MSISDN: msisdn}
	if err := s.cbeCall(ctx, h, "/query", q, &bill); err != nil {
		return "query", nil, nil, err.Error()
	}
	if bill.ResultCode != payments.CBEBirrOK || step != "pay" {
		return "query", &bill, nil, ""
	}
	id := make([]byte, 5)
	_, _ = rand.Read(id)
	tx := payments.CBEBirrTransaction{
		TransType: "Pay Bill", TransID: "CB" + strings.ToUpper(hex.EncodeToString(id)),
		TransTime:   time.Now().In(time.FixedZone("EAT", 3*60*60)).Format("20060102150405"),
		TransAmount: bill.Amount, BusinessShortCode: q.BusinessShortCode, BillRefNumber: q.BillRefNumber, MSISDN: msisdn,
	}
	var res payments.CBEBirrResult
	if err := s.cbeCall(ctx, h, "/validation", tx, &res); err != nil {
		return "pay", &bill, nil, err.Error()
	}
	if res.ResultCode == payments.CBEBirrOK {
		if err := s.cbeCall(ctx, h, "/confirmation", tx, &res); err != nil {
			return "pay", &bill, nil, err.Error()
		}
		res.ThirdPartyTransID = tx.TransID
	}
	return "pay", &bill, &res, ""
}

// cbeCall makes one of CBE Birr's calls to its handler, in-process.
func (s *Server) cbeCall(ctx context.Context, h http.Handler, path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	login := func(req *http.Request) {
		req.SetBasicAuth(s.cfg.CBEBirr.CallbackUsername, s.cfg.CBEBirr.CallbackPassword)
	}
	res, err := call(ctx, h, path, "application/json", b, login)
	if err != nil {
		return err
	}
	if res.code != http.StatusOK {
		return fmt.Errorf("%s was answered %d: %s", path, res.code, strings.TrimSpace(res.body.String()))
	}
	return json.Unmarshal(res.body.Bytes(), out)
}

// call posts to a provider's handler in-process, for the pages that play
// the provider, and keeps its answer.
func call(ctx context.Context, h http.Handler, path, contentType string, body []byte, prepare func(*http.Request)) (*recorder, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	if prepare != nil {
		prepare(req)
	}
	res := &recorder{header: http.Header{}}
	h.ServeHTTP(res, req)
	if res.code == 0 {
		res.code = http.StatusOK
	}
	return res, nil
}

// recorder is an http.ResponseWriter that keeps the answer.
type recorder struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) Write(b []byte) (int, error) {
	if r.code == 0 {
		r.code = http.StatusOK
	}
	return r.body.Write(b)
}

func (r *recorder) WriteHeader(code int) {
	if r.code == 0 {
		r.code = code
	}
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
