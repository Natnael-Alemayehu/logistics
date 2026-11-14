package payments

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func cbeHandler(t *testing.T, l *ledger) http.Handler {
	t.Helper()
	c, err := NewCBEBirr(CBEBirrConfig{MerchantID: "100100", Username: "cbe", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	return c.Handler(l, l.fail)
}

// cbeCall posts one of CBE Birr's calls and decodes the answer into out.
func cbeCall(t *testing.T, h http.Handler, path string, body any, out any, user, pass string) int {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(b)))
	r.Header.Set("Content-Type", "application/json")
	if user != "" {
		r.SetBasicAuth(user, pass)
	}
	res := httptest.NewRecorder()
	h.ServeHTTP(res, r)
	if out != nil && res.Code == http.StatusOK {
		if err := json.Unmarshal(res.Body.Bytes(), out); err != nil {
			t.Fatalf("%s answered %q: %v", path, res.Body.String(), err)
		}
	}
	return res.Code
}

func TestCBEBirrCallsMustLogIn(t *testing.T) {
	l := &ledger{bills: map[string]Bill{"202600012": {Amount: 287500}}}
	h := cbeHandler(t, l)
	q := CBEBirrQuery{BillRefNumber: "202600012", BusinessShortCode: "100100"}
	for _, creds := range [][2]string{{"", ""}, {"cbe", "wrong"}, {"other", "secret"}} {
		if code := cbeCall(t, h, "/query", q, nil, creds[0], creds[1]); code != http.StatusUnauthorized {
			t.Errorf("login %v: status %d, want 401", creds, code)
		}
	}
	if len(l.failed) != 3 || !errors.Is(l.failed[0], ErrUnauthorized) {
		t.Errorf("failures: %v", l.failed)
	}
}

func TestCBEBirrQuery(t *testing.T) {
	l := &ledger{bills: map[string]Bill{"202600012": {InvoiceNumber: "INV-2026-00012", Amount: 287500, Customer: "Abay Freight", Description: "Menged invoice INV-2026-00012"}}}
	h := cbeHandler(t, l)
	var bill CBEBirrBill
	if cbeCall(t, h, "/query", CBEBirrQuery{BillRefNumber: "202600012", BusinessShortCode: "100100", MSISDN: "251911223344"}, &bill, "cbe", "secret"); bill !=
		(CBEBirrBill{ResultCode: "0", ResultDesc: "Success", BillRefNumber: "202600012", Amount: "2875.00", Currency: "ETB",
			CustomerName: "Abay Freight", Description: "Menged invoice INV-2026-00012"}) {
		t.Errorf("bill = %+v", bill)
	}
	for name, c := range map[string]struct {
		ref, merchant string
		err           error
		want          string
	}{
		"unknown":        {"999", "100100", nil, CBEBirrUnknownBill},
		"paid":           {"202600012", "100100", ErrBillPaid, CBEBirrBillPaid},
		"cancelled":      {"202600012", "100100", ErrBillClosed, CBEBirrBillClosed},
		"wrong merchant": {"202600012", "555555", nil, CBEBirrWrongMerchant},
	} {
		l.billErr = c.err
		bill = CBEBirrBill{}
		cbeCall(t, h, "/query", CBEBirrQuery{BillRefNumber: c.ref, BusinessShortCode: c.merchant}, &bill, "cbe", "secret")
		if bill.ResultCode != c.want || bill.Amount != "" {
			t.Errorf("%s: %+v, want code %s", name, bill, c.want)
		}
	}
}

func TestCBEBirrValidationAndConfirmation(t *testing.T) {
	l := &ledger{bills: map[string]Bill{"202600012": {InvoiceNumber: "INV-2026-00012", Amount: 287500}}}
	h := cbeHandler(t, l)
	tx := CBEBirrTransaction{
		TransType: "Pay Bill", TransID: "CB7Q2K9", TransTime: "20261103142501", TransAmount: "2875.00",
		BusinessShortCode: "100100", BillRefNumber: "202600012", MSISDN: "251911223344",
	}
	var res CBEBirrResult
	if cbeCall(t, h, "/validation", tx, &res, "cbe", "secret"); res.ResultCode != CBEBirrOK || res.ThirdPartyTransID != "INV-2026-00012" {
		t.Errorf("validation = %+v", res)
	}
	short := tx
	short.TransAmount = "2000.00"
	if cbeCall(t, h, "/validation", short, &res, "cbe", "secret"); res.ResultCode != CBEBirrWrongAmount {
		t.Errorf("validation of the wrong amount = %+v", res)
	}
	noID := tx
	noID.TransID = ""
	if cbeCall(t, h, "/validation", noID, &res, "cbe", "secret"); res.ResultCode != CBEBirrInvalid {
		t.Errorf("validation without an ID = %+v", res)
	}

	if cbeCall(t, h, "/confirmation", tx, &res, "cbe", "secret"); res.ResultCode != CBEBirrOK || len(l.paid) != 1 {
		t.Fatalf("confirmation = %+v, paid %+v", res, l.paid)
	}
	p := l.paid[0]
	if p.Reference != "202600012" || p.TransID != "CB7Q2K9" || p.Amount != 287500 || p.Payer != "251911223344" ||
		!p.PaidAt.Equal(time.Date(2026, 11, 3, 11, 25, 1, 0, time.UTC)) || !strings.Contains(string(p.Raw), "CB7Q2K9") {
		t.Errorf("recorded %+v", p)
	}
	unknown := tx
	unknown.BillRefNumber = "404"
	if cbeCall(t, h, "/confirmation", unknown, &res, "cbe", "secret"); res.ResultCode != CBEBirrUnknownBill {
		t.Errorf("confirmation for an unknown reference = %+v", res)
	}
	if code := cbeCall(t, h, "/confirmation", "not an object", nil, "cbe", "secret"); code != http.StatusBadRequest {
		t.Errorf("an unreadable confirmation: status %d", code)
	}
}
