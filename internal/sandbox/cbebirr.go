package sandbox

// CBE Birr's side of Kiflet's adapter (pkg/payments/cbebirr.go). CBE Birr
// publishes no merchant API, so this is the contract as Kiflet defines it.
// The payer pays Kiflet's merchant ID from their phone, quoting a
// reference, and CBE Birr calls Kiflet with the merchant's login in Basic
// authentication:
//
//	POST {Kiflet}/api/v1/payments/cbebirr/query         {"BillRefNumber", "BusinessShortCode", "MSISDN"}
//	     → {"ResultCode", "ResultDesc", "Amount", "Currency", "CustomerName", "Description"}
//	POST {Kiflet}/api/v1/payments/cbebirr/validation    a transaction → {"ResultCode", "ResultDesc"}
//	POST {Kiflet}/api/v1/payments/cbebirr/confirmation  the transaction, once the money is taken
//
// A transaction is TransType, TransID, TransTime (yyyyMMddHHmmss, Ethiopian
// time), TransAmount ("2875.00"), BusinessShortCode, BillRefNumber and
// MSISDN (251…). ResultCode 0 accepts; any other refuses, and ResultDesc is
// shown to the payer. The confirmation is sent until Kiflet answers 200;
// one Kiflet refuses is reversed, the money going back to the payer.

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
)

type cbeBill struct {
	ResultCode    string `json:"ResultCode"`
	ResultDesc    string `json:"ResultDesc"`
	BillRefNumber string `json:"BillRefNumber,omitempty"`
	Amount        string `json:"Amount,omitempty"`
	Currency      string `json:"Currency,omitempty"`
	CustomerName  string `json:"CustomerName,omitempty"`
	Description   string `json:"Description,omitempty"`
}

type cbeTransaction struct {
	TransType         string `json:"TransType"`
	TransID           string `json:"TransID"`
	TransTime         string `json:"TransTime"`
	TransAmount       string `json:"TransAmount"`
	BusinessShortCode string `json:"BusinessShortCode"`
	BillRefNumber     string `json:"BillRefNumber"`
	MSISDN            string `json:"MSISDN"`
}

type cbeResult struct {
	ResultCode        string `json:"ResultCode"`
	ResultDesc        string `json:"ResultDesc"`
	ThirdPartyTransID string `json:"ThirdPartyTransID,omitempty"`
}

// cbePayment is a payment made from a phone's CBE Birr wallet.
type cbePayment struct {
	TransID   string    `json:"trans_id"`
	Phone     string    `json:"phone"`
	Merchant  string    `json:"merchant"`
	Reference string    `json:"reference"`
	Amount    int64     `json:"amount_santim"`
	At        time.Time `json:"at"`
	// Confirmation is what became of the confirmation. Confirmed is set when
	// Kiflet accepted one, and Reversed when it refused it and the money
	// went back.
	Confirmation string `json:"confirmation"`
	Confirmed    bool   `json:"confirmed"`
	Reversed     bool   `json:"reversed"`
}

const keepPayments = 100

// cbeQuery asks Kiflet what a reference owes, as CBE Birr does when the
// payer keys it in.
func (s *Sandbox) cbeQuery(number, merchant, ref string) (cbeBill, error) {
	if merchant != s.cfg.CBEBirr.MerchantID {
		return cbeBill{}, payerError("CBE Birr has no merchant with the ID " + merchant + ".")
	}
	if ref == "" {
		return cbeBill{}, payerError("Enter the reference the merchant gave you.")
	}
	body := mustJSON(map[string]string{"BillRefNumber": ref, "BusinessShortCode": merchant, "MSISDN": msisdn(number)})
	a := s.attempt(s.ctx, s.newCall("CBE Birr", "query "+ref, s.cbeURL("query"), s.cbeHeader(), body, 1, nil))
	var bill cbeBill
	return bill, cbeAnswer(a, &bill)
}

// cbePay pays a reference from a phone's CBE Birr wallet, as CBE Birr does
// once the payer confirms: it checks the PIN and the balance, asks Kiflet
// to validate the payment, takes the money and confirms it.
func (s *Sandbox) cbePay(number, merchant, ref string, amount int64, pin string) (cbePayment, error) {
	switch {
	case merchant != s.cfg.CBEBirr.MerchantID:
		return cbePayment{}, payerError("CBE Birr has no merchant with the ID " + merchant + ".")
	case amount <= 0:
		return cbePayment{}, payerError("Enter the amount to pay.")
	case pin != s.cfg.PIN:
		return cbePayment{}, payerError("Wrong PIN.")
	}
	if err := s.cbeFunds(number, amount); err != nil {
		return cbePayment{}, err
	}
	now := time.Now()
	tx := cbeTransaction{
		TransType: "Pay Bill", TransID: random(upperChars, 10), TransTime: now.In(eat).Format("20060102150405"),
		TransAmount: decimal(amount), BusinessShortCode: merchant, BillRefNumber: ref, MSISDN: msisdn(number),
	}
	a := s.attempt(s.ctx, s.newCall("CBE Birr", "validation "+tx.TransID+" for "+ref, s.cbeURL("validation"), s.cbeHeader(), mustJSON(tx), 1, nil))
	var res cbeResult
	if err := cbeAnswer(a, &res); err != nil {
		return cbePayment{}, err
	}
	if res.ResultCode != "0" {
		return cbePayment{}, payerError(res.ResultDesc + " CBE Birr took no money.")
	}

	s.mu.Lock()
	p := s.phoneLocked(number)
	if p.CBEBirr < amount {
		s.mu.Unlock()
		return cbePayment{}, payerError(fmt.Sprintf("Your CBE Birr balance, ETB %s, is less than ETB %s.", birr(p.CBEBirr), birr(amount)))
	}
	p.CBEBirr -= amount
	pay := &cbePayment{TransID: tx.TransID, Phone: number, Merchant: merchant, Reference: ref, Amount: amount, At: now, Confirmation: "Sending"}
	s.payments = append(s.payments, pay)
	if len(s.payments) > keepPayments {
		s.payments = s.payments[len(s.payments)-keepPayments:]
	}
	s.receive(p, "CBE Birr", fmt.Sprintf("You have paid ETB %s to %s (merchant %s) for reference %s. Transaction ID %s. Your CBE Birr balance is ETB %s.",
		birr(amount), s.cfg.AppName, merchant, ref, tx.TransID, birr(p.CBEBirr)))
	snap := *pay
	s.mu.Unlock()

	s.confirm(tx)
	snap, _ = s.paymentCopy(tx.TransID)
	return snap, nil
}

// cbeFunds checks a wallet holds an amount, before Kiflet is asked to
// validate it.
func (s *Sandbox) cbeFunds(number string, amount int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.phoneLocked(number); p.CBEBirr < amount {
		return payerError(fmt.Sprintf("Your CBE Birr balance, ETB %s, is less than ETB %s.", birr(p.CBEBirr), birr(amount)))
	}
	return nil
}

// confirm sends a transaction's confirmation, as the faults say: held back,
// twice, or once, tried until Kiflet takes it.
func (s *Sandbox) confirm(tx cbeTransaction) {
	f := s.currentFaults()
	c := s.newCall("CBE Birr", "confirmation "+tx.TransID+" for "+tx.BillRefNumber, s.cbeURL("confirmation"), s.cbeHeader(), mustJSON(tx), 12,
		func(a answer, last bool) { s.confirmed(tx.TransID, a, last) })
	calls := []*call{c}
	if f.DuplicateConfirmations {
		calls = append(calls, s.again(c))
	}
	for _, c := range calls {
		if f.DropConfirmations {
			s.setConfirmation(tx.TransID, "Held back (fault)")
			s.hold(c, "confirmations are dropped (fault)")
			continue
		}
		s.deliver(c)
	}
}

// confirmed records what Kiflet made of a confirmation. A refused one, say
// for an invoice cancelled since it was validated, is reversed: the money
// goes back to the payer, as a wallet does when a merchant turns a payment
// away. Once one confirmation is accepted or reversed, repeats change
// nothing.
func (s *Sandbox) confirmed(transID string, a answer, last bool) {
	var res cbeResult
	refused := a.ok() && json.Unmarshal(a.Body, &res) == nil && res.ResultCode != "0"
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.payments, func(p *cbePayment) bool { return p.TransID == transID })
	if i < 0 {
		return
	}
	pay := s.payments[i]
	switch {
	case pay.Confirmed || pay.Reversed:
	case refused:
		pay.Reversed, pay.Confirmation = true, "Refused by Kiflet: "+res.ResultDesc+" The money went back."
		p := s.phoneLocked(pay.Phone)
		p.CBEBirr += pay.Amount
		s.receive(p, "CBE Birr", fmt.Sprintf("ETB %s for reference %s was returned: %s Your CBE Birr balance is ETB %s.",
			birr(pay.Amount), pay.Reference, res.ResultDesc, birr(p.CBEBirr)))
	case a.ok():
		pay.Confirmed, pay.Confirmation = true, "Taken by Kiflet"
	case last:
		pay.Confirmation = describeAnswer(a)
	default:
		pay.Confirmation = "Trying again: " + describeAnswer(a)
	}
}

func (s *Sandbox) setConfirmation(transID, state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.payments {
		if p.TransID == transID && !p.Confirmed && !p.Reversed {
			p.Confirmation = state
		}
	}
}

// paymentCopy is a copy of a CBE Birr payment, by transaction ID.
func (s *Sandbox) paymentCopy(transID string) (cbePayment, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.payments {
		if p.TransID == transID {
			return *p, true
		}
	}
	return cbePayment{}, false
}

// recentPayments are the latest CBE Birr payments, newest first.
func (s *Sandbox) recentPayments(n int) []cbePayment {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]cbePayment, 0, min(n, len(s.payments)))
	for _, p := range slices.Backward(s.payments) {
		if len(out) == n {
			break
		}
		out = append(out, *p)
	}
	return out
}

func (s *Sandbox) cbeURL(step string) string {
	return s.cfg.AppURL + "/api/v1/payments/cbebirr/" + step
}

// cbeHeader is the header of CBE Birr's calls: JSON, and the merchant's
// callback login.
func (s *Sandbox) cbeHeader() http.Header {
	h := jsonHeader()
	login := base64.StdEncoding.EncodeToString([]byte(s.cfg.CBEBirr.Username + ":" + s.cfg.CBEBirr.Password))
	h.Set("Authorization", "Basic "+login)
	return h
}

// cbeAnswer reads Kiflet's answer to a call, or says, for the payer, why
// there is none.
func cbeAnswer(a answer, out any) error {
	switch {
	case a.Err != nil:
		return payerError("CBE Birr couldn't reach the merchant: " + a.Err.Error())
	case a.Status != http.StatusOK:
		return payerError(fmt.Sprintf("The merchant's system answered %d: %s", a.Status, clip(strings.TrimSpace(string(a.Body)), 160)))
	}
	if err := json.Unmarshal(a.Body, out); err != nil {
		return payerError("The merchant's answer couldn't be read: " + err.Error())
	}
	return nil
}
