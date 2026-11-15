package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
)

// SupportService carries questions from companies to the people running
// Kiflet, and their answers (FR-049 AC-049.5). Both sides are emailed when
// the other writes.
type SupportService struct{ *core }

// Ticket statuses: open waits for the platform, answered for the company.
const (
	TicketOpen     = "open"
	TicketAnswered = "answered"
	TicketClosed   = "closed"
)

// Ticket is one conversation, as either side sees it.
type Ticket struct {
	ID, TenantID  pgtype.UUID
	Number        int64
	Subject       string
	Status        string
	Company       string
	OpenedBy      string
	OpenedByEmail string
	CreatedAt     pgtype.Timestamptz
	Messages      []db.SupportMessage
}

func supportText(subject, body string) (string, string, error) {
	subject, body = strings.TrimSpace(subject), strings.TrimSpace(body)
	if subject == "" {
		return "", "", invalid("subject", "Say in a few words what it's about.")
	}
	if len([]rune(subject)) > 200 {
		return "", "", invalid("subject", "Keep the subject under 200 characters.")
	}
	if err := supportBody(body); err != nil {
		return "", "", err
	}
	return subject, body, nil
}

func supportBody(body string) error {
	if body == "" {
		return invalid("body", "Write your message.")
	}
	if len([]rune(body)) > 5000 {
		return invalid("body", "Keep the message under 5,000 characters.")
	}
	return nil
}

// Open starts a ticket for the caller's company.
func (s *SupportService) Open(ctx context.Context, p *Principal, subject, body string) (pgtype.UUID, error) {
	if p.IsPlatformAdmin() {
		return pgtype.UUID{}, ErrUnauthorized
	}
	subject, body, err := supportText(subject, body)
	if err != nil {
		return pgtype.UUID{}, err
	}
	var id pgtype.UUID
	err = s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		t, err := q.CreateSupportTicket(ctx, db.CreateSupportTicketParams{TenantID: p.TenantID, Subject: subject, OpenedBy: p.UserID})
		if err != nil {
			return err
		}
		id = t.ID
		if _, err := q.AddSupportMessage(ctx, db.AddSupportMessageParams{
			TicketID: t.ID, TenantID: p.TenantID, AuthorID: p.UserID, AuthorName: p.Name, Body: body,
		}); err != nil {
			return err
		}
		return s.tellPlatform(ctx, q, after, t.ID, t.Number, subject, p, body, true)
	})
	return id, err
}

// Reply adds to a ticket. A company's reply reopens it; the platform's
// marks it answered.
func (s *SupportService) Reply(ctx context.Context, p *Principal, id pgtype.UUID, body string) error {
	body = strings.TrimSpace(body)
	if err := supportBody(body); err != nil {
		return err
	}
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		t, err := s.load(ctx, q, p, id)
		if err != nil {
			return err
		}
		platform := p.IsPlatformAdmin()
		if _, err := q.AddSupportMessage(ctx, db.AddSupportMessageParams{
			TicketID: t.ID, TenantID: t.TenantID, AuthorID: p.UserID, AuthorName: p.Name, FromPlatform: platform, Body: body,
		}); err != nil {
			return err
		}
		status := TicketOpen
		if platform {
			status = TicketAnswered
		}
		if err := q.SetSupportTicketStatus(ctx, db.SetSupportTicketStatusParams{Status: status, ID: t.ID, TenantID: t.TenantID}); err != nil {
			return err
		}
		if !platform {
			return s.tellPlatform(ctx, q, after, t.ID, t.Number, t.Subject, p, body, false)
		}
		return s.tellCompany(ctx, q, after, t, body)
	})
}

// Close ends a ticket; writing to it again reopens it.
func (s *SupportService) Close(ctx context.Context, p *Principal, id pgtype.UUID) error {
	return s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
		t, err := s.load(ctx, q, p, id)
		if err != nil {
			return err
		}
		return q.SetSupportTicketStatus(ctx, db.SetSupportTicketStatusParams{Status: TicketClosed, ID: t.ID, TenantID: t.TenantID})
	})
}

// Get returns a ticket with its messages: the company's own, or any for the
// platform admin.
func (s *SupportService) Get(ctx context.Context, p *Principal, id pgtype.UUID) (*Ticket, error) {
	t, err := s.load(ctx, s.q, p, id)
	if err != nil {
		return nil, err
	}
	t.Messages, err = s.q.ListTicketMessages(ctx, db.ListTicketMessagesParams{TicketID: t.ID, TenantID: t.TenantID})
	return t, err
}

func (s *SupportService) load(ctx context.Context, q *db.Queries, p *Principal, id pgtype.UUID) (*Ticket, error) {
	if p.IsPlatformAdmin() {
		r, err := q.GetTicketForPlatform(ctx, id)
		if isNoRows(err) {
			return nil, notFound("Ticket not found.")
		}
		if err != nil {
			return nil, err
		}
		return &Ticket{ID: r.ID, TenantID: r.TenantID, Number: r.Number, Subject: r.Subject, Status: r.Status, Company: r.TenantName,
			OpenedBy: r.OpenedByName, OpenedByEmail: r.OpenedByEmail, CreatedAt: r.CreatedAt}, nil
	}
	r, err := q.GetCompanyTicket(ctx, db.GetCompanyTicketParams{ID: id, TenantID: p.TenantID})
	if isNoRows(err) {
		return nil, notFound("Ticket not found.")
	}
	if err != nil {
		return nil, err
	}
	return &Ticket{ID: r.ID, TenantID: r.TenantID, Number: r.Number, Subject: r.Subject, Status: r.Status, Company: p.TenantName,
		OpenedBy: r.OpenedByName, OpenedByEmail: r.OpenedByEmail, CreatedAt: r.CreatedAt}, nil
}

// List is the company's tickets, open ones first.
func (s *SupportService) List(ctx context.Context, p *Principal) ([]db.ListCompanyTicketsRow, error) {
	if p.IsPlatformAdmin() {
		return nil, ErrUnauthorized
	}
	return s.q.ListCompanyTickets(ctx, p.TenantID)
}

// ForCompany is one company's tickets, for the platform admin.
func (s *SupportService) ForCompany(ctx context.Context, p *Principal, tenantID pgtype.UUID) ([]db.ListCompanyTicketsRow, error) {
	if !p.IsPlatformAdmin() {
		return nil, ErrUnauthorized
	}
	return s.q.ListCompanyTickets(ctx, tenantID)
}

// Inbox is every company's tickets for the platform admin, those waiting
// for an answer first. status narrows it to open, answered or closed.
func (s *SupportService) Inbox(ctx context.Context, p *Principal, status string) ([]db.ListSupportInboxRow, error) {
	if !p.IsPlatformAdmin() {
		return nil, ErrUnauthorized
	}
	var st *string
	if status == TicketOpen || status == TicketAnswered || status == TicketClosed {
		st = &status
	}
	return s.q.ListSupportInbox(ctx, st)
}

// Waiting counts tickets waiting for the platform.
func (s *SupportService) Waiting(ctx context.Context) (int64, error) {
	return s.q.CountWaitingTickets(ctx)
}

func (s *SupportService) tellPlatform(ctx context.Context, q *db.Queries, after *afterCommit, id pgtype.UUID, number int64, subject string, p *Principal, body string, opened bool) error {
	admins, err := q.ListPlatformAdmins(ctx)
	if err != nil {
		return err
	}
	what := "replied to"
	if opened {
		what = "opened"
	}
	for _, a := range admins {
		s.enqueueEmail(ctx, q, after, outboundEmail{
			To:      a.Email,
			Subject: fmt.Sprintf("[#%d] %s — %s", number, subject, p.TenantName),
			Body: s.emailBody(a.FullName,
				fmt.Sprintf("%s of %s %s ticket #%d:", p.Name, p.TenantName, what, number),
				quote(body),
				"Answer it at "+s.cfg.BaseURL+"/app/platform/support/"+id.String()),
			Purpose: EmailSupport,
		})
	}
	return nil
}

// tellCompany emails the platform's answer to whoever opened the ticket, or
// to the company's admins when that person has gone.
func (s *SupportService) tellCompany(ctx context.Context, q *db.Queries, after *afterCommit, t *Ticket, body string) error {
	type to struct{ name, email string }
	var list []to
	if t.OpenedByEmail != "" {
		list = append(list, to{t.OpenedBy, t.OpenedByEmail})
	} else {
		admins, err := q.ListBillingContacts(ctx, t.TenantID)
		if err != nil {
			return err
		}
		for _, a := range admins {
			list = append(list, to{a.FullName, a.Email})
		}
	}
	for _, r := range list {
		s.enqueueEmail(ctx, q, after, outboundEmail{
			TenantID: t.TenantID,
			To:       r.email,
			Subject:  fmt.Sprintf("Re: [#%d] %s", t.Number, t.Subject),
			Body: s.emailBody(r.name,
				fmt.Sprintf("%s support answered ticket #%d:", s.cfg.AppName, t.Number),
				quote(body),
				"Reply at "+s.cfg.BaseURL+"/app/help/"+t.ID.String()),
			Purpose: EmailSupport,
		})
	}
	return nil
}

// quote indents a message inside an email.
func quote(body string) string {
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		lines[i] = "> " + l
	}
	return strings.Join(lines, "\n")
}
