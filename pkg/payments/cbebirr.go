package payments

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CBEBirr takes payments the way CBE Birr merchants are paid. The payer
// opens CBE Birr, pays Kiflet's merchant ID and quotes the invoice's
// reference. CBE Birr then calls Kiflet three times:
//
//   - query, to ask what the reference owes;
//   - validation, to check the payment before taking the money;
//   - confirmation, once the money is taken.
//
// Kiflet never calls CBE Birr.
//
// CBE Birr does not publish its merchant API. The three steps are those of
// the only public integration package. The field names, the result codes
// and the Basic authentication of CBE Birr's calls are Kiflet's own; they
// are kept in this file, to be matched to CBE's documents at onboarding.
type CBEBirr struct{ cfg CBEBirrConfig }

// CBEBirrConfig is Kiflet's CBE Birr merchant account.
type CBEBirrConfig struct {
	// MerchantID is the short code payers enter.
	MerchantID string
	// Username and Password are what CBE Birr's calls must carry.
	Username string
	Password string
	// StandIn is set when CBE Birr's side is played by a page of this app.
	StandIn bool
}

// NewCBEBirr checks the configuration.
func NewCBEBirr(c CBEBirrConfig) (*CBEBirr, error) {
	if c.MerchantID == "" || c.Username == "" || c.Password == "" {
		return nil, errors.New("CBE Birr: the merchant ID and the callback username and password are all needed")
	}
	return &CBEBirr{cfg: c}, nil
}

func (c *CBEBirr) Code() string       { return "cbebirr" }
func (c *CBEBirr) Label() string      { return "CBE Birr" }
func (c *CBEBirr) Mock() bool         { return c.cfg.StandIn }
func (*CBEBirr) Flow() Flow           { return FlowReference }
func (c *CBEBirr) MerchantID() string { return c.cfg.MerchantID }

// CBEBirrQuery is CBE Birr asking what a reference owes.
type CBEBirrQuery struct {
	BillRefNumber     string `json:"BillRefNumber"`
	BusinessShortCode string `json:"BusinessShortCode"`
	MSISDN            string `json:"MSISDN"`
}

// CBEBirrBill answers a query.
type CBEBirrBill struct {
	ResultCode    string `json:"ResultCode"`
	ResultDesc    string `json:"ResultDesc"`
	BillRefNumber string `json:"BillRefNumber,omitempty"`
	Amount        string `json:"Amount,omitempty"`
	Currency      string `json:"Currency,omitempty"`
	CustomerName  string `json:"CustomerName,omitempty"`
	Description   string `json:"Description,omitempty"`
}

// CBEBirrTransaction is a payment CBE Birr is about to make (validation) or
// has made (confirmation).
type CBEBirrTransaction struct {
	TransType   string `json:"TransType"`
	TransID     string `json:"TransID"`
	TransTime   string `json:"TransTime"` // yyyyMMddHHmmss, Ethiopian time
	TransAmount string `json:"TransAmount"`
	// BusinessShortCode is the merchant ID the payer entered.
	BusinessShortCode string `json:"BusinessShortCode"`
	BillRefNumber     string `json:"BillRefNumber"`
	MSISDN            string `json:"MSISDN"`
}

// CBEBirrResult answers a validation or a confirmation.
type CBEBirrResult struct {
	ResultCode        string `json:"ResultCode"`
	ResultDesc        string `json:"ResultDesc"`
	ThirdPartyTransID string `json:"ThirdPartyTransID,omitempty"`
}

// Result codes. 0 accepts; any other refuses, and CBE Birr shows the
// description to the payer.
const (
	CBEBirrOK            = "0"
	CBEBirrUnknownBill   = "1"
	CBEBirrBillPaid      = "2"
	CBEBirrBillClosed    = "3"
	CBEBirrWrongAmount   = "4"
	CBEBirrWrongMerchant = "5"
	CBEBirrInvalid       = "6"
)

// ethiopia is Ethiopian time, which has no daylight saving.
var ethiopia = time.FixedZone("EAT", 3*60*60)

// Handler serves CBE Birr's calls: POST /query, /validation and
// /confirmation, each with the account's Basic credentials. Refusals are
// answered 200 with a result code; only a bad login or an unreadable call
// gets an error status.
func (c *CBEBirr) Handler(l Ledger, fail ErrorWriter) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /query", func(w http.ResponseWriter, r *http.Request) {
		var q CBEBirrQuery
		if !read(w, r, fail, &q) {
			return
		}
		if q.BusinessShortCode != c.cfg.MerchantID {
			writeJSON(w, CBEBirrBill{ResultCode: CBEBirrWrongMerchant, ResultDesc: "This is not the merchant's ID."})
			return
		}
		bill, err := l.Bill(r.Context(), c.Code(), q.BillRefNumber)
		if code, desc, ok := refusal(err); ok {
			writeJSON(w, CBEBirrBill{ResultCode: code, ResultDesc: desc})
			return
		}
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, CBEBirrBill{
			ResultCode: CBEBirrOK, ResultDesc: "Success", BillRefNumber: q.BillRefNumber,
			Amount: Decimal(bill.Amount), Currency: "ETB", CustomerName: bill.Customer, Description: bill.Description,
		})
	})
	mux.HandleFunc("POST /validation", func(w http.ResponseWriter, r *http.Request) {
		t, amount, ok := c.transaction(w, r, fail)
		if !ok {
			return
		}
		bill, err := l.Bill(r.Context(), c.Code(), t.BillRefNumber)
		if err == nil && amount != bill.Amount {
			err = ErrWrongAmount
		}
		if code, desc, ok := refusal(err); ok {
			writeJSON(w, CBEBirrResult{ResultCode: code, ResultDesc: desc})
			return
		}
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, CBEBirrResult{ResultCode: CBEBirrOK, ResultDesc: "Accepted", ThirdPartyTransID: bill.InvoiceNumber})
	})
	mux.HandleFunc("POST /confirmation", func(w http.ResponseWriter, r *http.Request) {
		t, amount, ok := c.transaction(w, r, fail)
		if !ok {
			return
		}
		paidAt, err := time.ParseInLocation("20060102150405", t.TransTime, ethiopia)
		if err != nil {
			paidAt = time.Time{}
		}
		raw, _ := json.Marshal(t)
		err = l.BillPaid(r.Context(), c.Code(), BillPayment{
			Reference: t.BillRefNumber, TransID: t.TransID, Amount: amount, Payer: t.MSISDN, PaidAt: paidAt, Raw: raw,
		})
		if code, desc, ok := refusal(err); ok {
			writeJSON(w, CBEBirrResult{ResultCode: code, ResultDesc: desc})
			return
		}
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, CBEBirrResult{ResultCode: CBEBirrOK, ResultDesc: "Success"})
	})
	return c.authenticated(mux, fail)
}

// transaction reads a validation or confirmation and its amount, answering
// it when it is unusable.
func (c *CBEBirr) transaction(w http.ResponseWriter, r *http.Request, fail ErrorWriter) (CBEBirrTransaction, int64, bool) {
	var t CBEBirrTransaction
	if !read(w, r, fail, &t) {
		return t, 0, false
	}
	if t.BusinessShortCode != c.cfg.MerchantID {
		writeJSON(w, CBEBirrResult{ResultCode: CBEBirrWrongMerchant, ResultDesc: "This is not the merchant's ID."})
		return t, 0, false
	}
	amount, err := ParseDecimal(t.TransAmount)
	if err != nil || t.TransID == "" {
		writeJSON(w, CBEBirrResult{ResultCode: CBEBirrInvalid, ResultDesc: "The transaction has no ID or no amount."})
		return t, 0, false
	}
	return t, amount, true
}

func (c *CBEBirr) authenticated(next http.Handler, fail ErrorWriter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		userOK := subtle.ConstantTimeCompare([]byte(user), []byte(c.cfg.Username)) == 1
		passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(c.cfg.Password)) == 1
		if !ok || !userOK || !passOK {
			w.Header().Set("WWW-Authenticate", `Basic realm="cbebirr"`)
			fail(w, r, ErrUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// refusal is the result code for an answer about a reference that refuses
// the payment.
func refusal(err error) (code, desc string, ok bool) {
	switch {
	case errors.Is(err, ErrUnknownBill):
		return CBEBirrUnknownBill, "No invoice has this reference.", true
	case errors.Is(err, ErrBillPaid):
		return CBEBirrBillPaid, "This invoice is already paid.", true
	case errors.Is(err, ErrBillClosed):
		return CBEBirrBillClosed, "This invoice was cancelled.", true
	case errors.Is(err, ErrWrongAmount):
		return CBEBirrWrongAmount, "The amount is not what the invoice asks for.", true
	}
	return "", "", false
}

func read(w http.ResponseWriter, r *http.Request, fail ErrorWriter, v any) bool {
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(v); err != nil {
		fail(w, r, fmt.Errorf("%w: %v", ErrMalformed, err))
		return false
	}
	return true
}
