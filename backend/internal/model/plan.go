package model

import "time"

// Subscription plans. A plan sets how many drivers can be active and how
// many text messages a month are included; every plan has the whole product.
const (
	PlanTrial      = "trial"
	PlanStarter    = "starter"
	PlanBusiness   = "business"
	PlanEnterprise = "enterprise"
)

// Billing cycles. A year costs ten months.
const (
	CycleMonthly = "monthly"
	CycleAnnual  = "annual"
)

// Billing terms. Amounts are in santim, hundredths of a birr, and exclude
// VAT.
const (
	VATPercent   = 15
	AnnualMonths = 10
	// SMSOverageSantim is the price of a text beyond the month's allowance.
	// A paying company's texts are never blocked by the allowance: a
	// customer missing a delivery update costs the company more than the
	// text.
	SMSOverageSantim = 50
	// SMSPerHour and TrialSMSPerHour cap the texts (in SMS parts) a company
	// may send in an hour, far above what dispatching needs, so that an
	// account can't be used to send spam at the platform's expense. A
	// trial also stops at its plan's SMSIncluded for the whole trial.
	SMSPerHour      = 1000
	TrialSMSPerHour = 100
	// TrialDays is the length of the free trial.
	TrialDays = 14
	// InvoiceDueDays is how long an invoice waits for payment. GraceDays
	// later, an unpaid renewal or usage invoice makes the company read-only.
	InvoiceDueDays = 7
	GraceDays      = 7
)

// Plan describes a subscription tier.
type Plan struct {
	Code       string
	Name       string
	MaxDrivers int32
	// SMSIncluded is the number of texts a calendar month covered by the
	// price.
	SMSIncluded int32
	// MonthlyETB is the list price excluding VAT: zero for the trial, and
	// for Enterprise, which is priced per company.
	MonthlyETB int64
	// SelfServe plans can be chosen and paid for on the billing page.
	SelfServe bool
}

// Plans in ascending order. A trial has the Starter plan's limits.
var Plans = []Plan{
	{Code: PlanTrial, Name: "Free trial", MaxDrivers: 10, SMSIncluded: 500},
	{Code: PlanStarter, Name: "Starter", MaxDrivers: 10, SMSIncluded: 500, MonthlyETB: 2500, SelfServe: true},
	{Code: PlanBusiness, Name: "Business", MaxDrivers: 50, SMSIncluded: 3000, MonthlyETB: 9000, SelfServe: true},
	{Code: PlanEnterprise, Name: "Enterprise", MaxDrivers: 500, SMSIncluded: 10000},
}

// PlanByCode finds a plan, reporting whether it exists.
func PlanByCode(code string) (Plan, bool) {
	for _, p := range Plans {
		if p.Code == code {
			return p, true
		}
	}
	return Plan{}, false
}

// PlanName is the display name of a plan code.
func PlanName(code string) string {
	if p, ok := PlanByCode(code); ok {
		return p.Name
	}
	return code
}

// CycleName is the display name of a billing cycle.
func CycleName(cycle string) string {
	if cycle == CycleAnnual {
		return "Annual"
	}
	return "Monthly"
}

// ValidCycle reports whether c is a billing cycle.
func ValidCycle(c string) bool { return c == CycleMonthly || c == CycleAnnual }

// PeriodPrice is the price of one billing period, given the monthly price.
func PeriodPrice(monthly int64, cycle string) int64 {
	if cycle == CycleAnnual {
		return monthly * AnnualMonths
	}
	return monthly
}

// PeriodEnd is when a billing period starting at start ends: a month or a
// year later on the same day, or on the last day of a shorter month, so a
// month from 31 January ends on 28 February, not 3 March. Days are counted
// in start's location.
func PeriodEnd(start time.Time, cycle string) time.Time {
	months := 1
	if cycle == CycleAnnual {
		months = 12
	}
	end := start.AddDate(0, months, 0)
	if end.Day() != start.Day() {
		end = end.AddDate(0, 0, -end.Day())
	}
	return end
}

// VAT is the tax on a subtotal, rounded to the santim.
func VAT(subtotal int64) int64 { return (subtotal*VATPercent + 50) / 100 }

// Prorate is the share of amount for what remains of the period from start
// to end, at now, rounded to the santim.
func Prorate(amount int64, start, end, now time.Time) int64 {
	total := end.Sub(start)
	left := end.Sub(now)
	if total <= 0 || left <= 0 {
		return 0
	}
	if left > total {
		left = total
	}
	// Seconds keep the product well inside int64 for any real price.
	return (amount*int64(left/time.Second) + int64(total/time.Second)/2) / int64(total/time.Second)
}
