// Package payments takes subscription payments from mobile-money providers.
//
// Providers work one of two ways:
//
//   - Checkout (telebirr): Kiflet starts a payment with the provider and
//     sends the payer to the provider's page to approve it. The provider
//     reports the outcome to a notification URL, and can be asked about it
//     when that report never arrives.
//   - Reference (CBE Birr): the payer pays from the provider's own app,
//     quoting the invoice's reference. The provider asks Kiflet what the
//     reference owes before taking the money and tells it once taken.
//
// The stand-in (Mock) imitates the checkout flow with a page of this app.
package payments

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Outcomes a provider reports.
const (
	StatusPaid   = "paid"
	StatusFailed = "failed"
)

var (
	// ErrBadSignature means a notification did not come from the provider.
	ErrBadSignature = errors.New("payment notification signature is invalid")
	// ErrUnauthorized means a provider's call carried the wrong credentials.
	ErrUnauthorized = errors.New("payment provider credentials are invalid")
)

// Flow is how payments are made with a provider.
type Flow int

const (
	// FlowCheckout providers are started by Kiflet, which sends the payer to
	// the provider's page.
	FlowCheckout Flow = iota
	// FlowReference providers are paid from the provider's app, quoting a
	// reference.
	FlowReference
)

// Charge is a request for money.
type Charge struct {
	// Reference is ours and unique per attempt; the provider echoes it back.
	Reference string
	// Amount is in santim, hundredths of a birr.
	Amount      int64
	Description string
	// NotifyURL is where the provider reports the outcome, and ReturnURL is
	// where it sends the payer back to afterwards.
	NotifyURL string
	ReturnURL string
	// Timeout is how long the payer has to approve.
	Timeout time.Duration
}

// Checkout is the provider's answer to a charge.
type Checkout struct {
	ProviderRef string
	// RedirectURL is the provider's page where the payer approves.
	RedirectURL string
}

// Notification is a provider's report of how a charge ended.
type Notification struct {
	Reference   string
	ProviderRef string
	Status      string
	Amount      int64
}

// Provider is a payment method.
type Provider interface {
	// Code names the provider in URLs and records, such as "telebirr".
	Code() string
	// Label is its name for people.
	Label() string
	// Mock reports whether it only pretends to take money.
	Mock() bool
	Flow() Flow
}

// Starter is a provider whose payments Kiflet starts (FlowCheckout).
type Starter interface {
	Provider
	Start(ctx context.Context, c Charge) (Checkout, error)
	// CheckoutOrigin is the scheme and host payers are sent to, which the
	// pay form must be allowed to redirect to. Empty when it is this app.
	CheckoutOrigin() string
}

// Querier can say how a payment ended, for when its notification is lost.
type Querier interface {
	// Query reports the outcome of the payment with our reference; done is
	// false while the payer may still approve it.
	Query(ctx context.Context, reference string) (n Notification, done bool, err error)
}

// Referenced is a provider paid from its own app (FlowReference).
type Referenced interface {
	Provider
	// MerchantID is what the payer enters to pay Kiflet.
	MerchantID() string
}

// Bill is what a reference quoted to a provider owes.
type Bill struct {
	InvoiceNumber string
	Amount        int64
	// Customer is the company the invoice is for.
	Customer    string
	Description string
}

// BillPayment is money a provider took against a reference.
type BillPayment struct {
	// Reference is what the payer quoted.
	Reference string
	// TransID is the provider's transaction; a repeat of one is ignored.
	TransID string
	Amount  int64
	// Payer is the phone that paid.
	Payer  string
	PaidAt time.Time
	// Raw is the provider's message, kept with the payment.
	Raw []byte
}

// Answers about a reference.
var (
	ErrUnknownBill = errors.New("there is no invoice with this reference")
	ErrBillPaid    = errors.New("the invoice is already paid")
	ErrBillClosed  = errors.New("the invoice was cancelled")
	ErrWrongAmount = errors.New("the amount is not what the invoice asks for")
)

// Ledger is what providers' calls look up and record: Kiflet's billing.
type Ledger interface {
	// Settle applies the outcome of a payment Kiflet started.
	Settle(ctx context.Context, provider string, n Notification) error
	// Bill says what a reference owes: ErrUnknownBill, ErrBillPaid or
	// ErrBillClosed when it can't be paid.
	Bill(ctx context.Context, provider, reference string) (Bill, error)
	// BillPaid records money a provider took against a reference.
	BillPaid(ctx context.Context, provider string, p BillPayment) error
}

// ErrorWriter answers a provider's call that failed.
type ErrorWriter func(w http.ResponseWriter, r *http.Request, err error)

// Callbacks is a provider whose servers call Kiflet. Its handler is served
// under /api/v1/payments/{code}, with that prefix removed.
type Callbacks interface {
	Handler(l Ledger, fail ErrorWriter) http.Handler
}

// FormatETB writes santim as birr with thousands separators: 287500 is
// "2,875.00".
func FormatETB(santim int64) string {
	sign := ""
	if santim < 0 {
		sign, santim = "-", -santim
	}
	birr := strconv.FormatInt(santim/100, 10)
	var b strings.Builder
	for i, r := range birr {
		if i > 0 && (len(birr)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return fmt.Sprintf("%s%s.%02d", sign, b.String(), santim%100)
}

// Decimal writes santim the way providers' APIs take amounts: 287500 is
// "2875.00".
func Decimal(santim int64) string {
	return fmt.Sprintf("%d.%02d", santim/100, santim%100)
}

// ParseDecimal reads an amount in birr, with at most two decimals, as
// santim: "2875" is 287500, and "2875.5" and "2875.50" are both 287550.
func ParseDecimal(s string) (int64, error) {
	s = strings.TrimSpace(s)
	whole, frac, _ := strings.Cut(s, ".")
	if whole == "" || len(frac) > 2 || strings.Trim(whole, "0123456789") != "" || strings.Trim(frac, "0123456789") != "" {
		return 0, fmt.Errorf("amount %q is not a number of birr", s)
	}
	birr, err := strconv.ParseInt(whole, 10, 64)
	if err != nil || birr > 1e12 {
		return 0, fmt.Errorf("amount %q is out of range", s)
	}
	frac += strings.Repeat("0", 2-len(frac))
	santim, _ := strconv.ParseInt(frac, 10, 64)
	return birr*100 + santim, nil
}
