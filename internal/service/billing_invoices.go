package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
)

// InvoiceLine is one charge on an invoice. Amounts are in santim.
type InvoiceLine struct {
	Description string `json:"description"`
	Quantity    int64  `json:"quantity"`
	Unit        int64  `json:"unit_santim"`
	Amount      int64  `json:"amount_santim"`
}

func line(desc string, qty, unit int64) InvoiceLine {
	return InvoiceLine{Description: desc, Quantity: qty, Unit: unit, Amount: qty * unit}
}

// SMSUsage is what an overage invoice charged for. It is kept with the
// invoice because the text log it was counted from is deleted after 180
// days (AC-045.5).
type SMSUsage struct {
	Month    string `json:"month"`
	Sent     int64  `json:"sent"`
	Included int32  `json:"included"`
	Extra    int64  `json:"extra"`
}

type invoiceDraft struct {
	Kind, Plan, Cycle      string
	PeriodStart, PeriodEnd time.Time
	Lines                  []InvoiceLine
	Usage                  *SMSUsage
	Due                    time.Time
	By                     pgtype.UUID
	// Intro opens the email that sends the invoice.
	Intro string
}

// issueInvoice numbers and stores an invoice and emails it to the company.
// One for nothing is paid at once.
func (s *BillingService) issueInvoice(ctx context.Context, q *db.Queries, after *afterCommit, t db.Tenant, d invoiceDraft) (db.Invoice, error) {
	now := s.now()
	var subtotal int64
	for _, l := range d.Lines {
		subtotal += l.Amount
	}
	vat := model.VAT(subtotal)
	year := now.In(s.cfg.Location).Year()
	n, err := q.NextInvoiceNumber(ctx, int32(year))
	if err != nil {
		return db.Invoice{}, err
	}
	var usage []byte
	if d.Usage != nil {
		usage = mustJSON(d.Usage)
	}
	inv, err := q.CreateInvoice(ctx, db.CreateInvoiceParams{
		TenantID: t.ID, Number: fmt.Sprintf("INV-%d-%05d", year, n), Kind: d.Kind, Plan: d.Plan, BillingCycle: d.Cycle,
		PeriodStart: toTimestamp(d.PeriodStart), PeriodEnd: toTimestamp(d.PeriodEnd),
		Lines: mustJSON(d.Lines), Usage: usage,
		SubtotalSantim: subtotal, VatSantim: vat, TotalSantim: subtotal + vat,
		IssuedAt: toTimestamp(now), DueAt: toTimestamp(d.Due),
		CustomerName: t.Name, CustomerTin: t.Tin, CustomerCity: &t.RegionCity, CreatedBy: d.By,
	})
	if err != nil {
		return inv, err
	}
	if inv.TotalSantim == 0 {
		return inv, s.settle(ctx, q, after, t, inv, db.Payment{}, now, SystemActor(t.ID))
	}
	paragraphs := []string{}
	if d.Intro != "" {
		paragraphs = append(paragraphs, d.Intro)
	}
	paragraphs = append(paragraphs,
		fmt.Sprintf("Invoice %s is for %s including VAT, due on %s. Pay it by telebirr or CBE Birr, or see the bank transfer details, here:",
			inv.Number, money(inv.TotalSantim), s.date(inv.DueAt.Time)),
		s.invoiceURL(inv))
	if s.cfg.BankDetails != "" {
		paragraphs = append(paragraphs, "To pay by bank transfer: "+s.cfg.BankDetails+". Quote "+inv.Number+" as the reference.")
	}
	return inv, s.emailBilling(ctx, q, after, t.ID, EmailInvoice, fmt.Sprintf("Invoice %s from %s", inv.Number, s.cfg.AppName), paragraphs...)
}

func (c *core) invoiceURL(inv db.Invoice) string {
	return c.cfg.BaseURL + "/app/billing/invoices/" + inv.ID.String()
}

// InvoiceLines reads an invoice's lines.
func InvoiceLines(raw []byte) []InvoiceLine {
	var out []InvoiceLine
	_ = json.Unmarshal(raw, &out)
	return out
}

// MonthName is the month the texts were sent in, such as "October 2026".
func (u SMSUsage) MonthName() string {
	m, err := time.Parse("2006-01", u.Month)
	if err != nil {
		return u.Month
	}
	return m.Format("January 2006")
}

// InvoiceUsage reads what an overage invoice charged for, if it is one.
func InvoiceUsage(raw []byte) *SMSUsage {
	if len(raw) == 0 {
		return nil
	}
	var u SMSUsage
	if json.Unmarshal(raw, &u) != nil {
		return nil
	}
	return &u
}

// InvoiceDetail is an invoice with what has been tried to pay it.
type InvoiceDetail struct {
	Invoice    db.Invoice
	TenantName string
	Lines      []InvoiceLine
	Usage      *SMSUsage
	Payments   []db.Payment
	// Waiting is a payment the payer started that the provider hasn't
	// reported on yet.
	Waiting *db.Payment
}

// PayReference is what a payer quotes to pay this invoice by reference.
func (d *InvoiceDetail) PayReference() string { return PayReference(d.Invoice.Number) }

// PayReference is the reference that pays an invoice from a phone: its
// number's digits (INV-2026-00012 is 202600012), easy to key in and unique,
// since invoice numbers are the platform's.
func PayReference(number string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, number)
}

// invoiceNumberOf reads the invoice number in a reference a payer quoted:
// the number (INV-2026-00012), its digits (202600012), or the digits without
// the zeros in the middle (202612). It is "" for anything else.
func invoiceNumberOf(ref string) string {
	ref = strings.TrimPrefix(strings.ToUpper(strings.Join(strings.Fields(ref), "")), "INV")
	var digits strings.Builder
	for _, r := range ref {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
		case r != '-':
			return ""
		}
	}
	d := digits.String()
	if len(d) < 5 {
		return ""
	}
	year, n := d[:4], strings.TrimLeft(d[4:], "0")
	if n == "" {
		return ""
	}
	if len(n) < 5 {
		n = strings.Repeat("0", 5-len(n)) + n
	}
	return "INV-" + year + "-" + n
}

// Invoice loads one of the caller's company's invoices.
func (s *BillingService) Invoice(ctx context.Context, p *Principal, id pgtype.UUID) (*InvoiceDetail, error) {
	if !p.CanManageCompany() {
		return nil, ErrUnauthorized
	}
	inv, err := s.q.GetInvoice(ctx, db.GetInvoiceParams{ID: id, TenantID: p.TenantID})
	if isNoRows(err) {
		return nil, notFound("Invoice not found.")
	}
	if err != nil {
		return nil, err
	}
	return s.invoiceDetail(ctx, inv, p.TenantName)
}

// PlatformInvoice loads any company's invoice, for the platform operator.
func (s *BillingService) PlatformInvoice(ctx context.Context, p *Principal, id pgtype.UUID) (*InvoiceDetail, error) {
	if !p.IsPlatformAdmin() {
		return nil, ErrUnauthorized
	}
	row, err := s.q.GetInvoiceByID(ctx, id)
	if isNoRows(err) {
		return nil, notFound("Invoice not found.")
	}
	if err != nil {
		return nil, err
	}
	return s.invoiceDetail(ctx, db.Invoice{
		ID: row.ID, TenantID: row.TenantID, Number: row.Number, Kind: row.Kind, Plan: row.Plan, BillingCycle: row.BillingCycle,
		PeriodStart: row.PeriodStart, PeriodEnd: row.PeriodEnd, Lines: row.Lines, Usage: row.Usage,
		SubtotalSantim: row.SubtotalSantim, VatSantim: row.VatSantim, TotalSantim: row.TotalSantim, Status: row.Status,
		IssuedAt: row.IssuedAt, DueAt: row.DueAt, PaidAt: row.PaidAt, VoidedAt: row.VoidedAt,
		CustomerName: row.CustomerName, CustomerTin: row.CustomerTin, CustomerCity: row.CustomerCity, CreatedBy: row.CreatedBy,
	}, row.TenantName)
}

// CompanyInvoices is one company's latest invoices, for the platform admin.
func (s *BillingService) CompanyInvoices(ctx context.Context, p *Principal, tenantID pgtype.UUID) ([]db.Invoice, error) {
	if !p.IsPlatformAdmin() {
		return nil, ErrUnauthorized
	}
	return s.q.ListInvoices(ctx, db.ListInvoicesParams{TenantID: tenantID, Limit: 24})
}

func (s *BillingService) invoiceDetail(ctx context.Context, inv db.Invoice, tenantName string) (*InvoiceDetail, error) {
	d := &InvoiceDetail{Invoice: inv, TenantName: tenantName, Lines: InvoiceLines(inv.Lines), Usage: InvoiceUsage(inv.Usage)}
	var err error
	if d.Payments, err = s.q.ListPaymentsByInvoice(ctx, db.ListPaymentsByInvoiceParams{InvoiceID: inv.ID, TenantID: inv.TenantID}); err != nil {
		return nil, err
	}
	for i := range d.Payments {
		if d.Payments[i].Status == paymentPending {
			d.Waiting = &d.Payments[i]
			break
		}
	}
	return d, nil
}
