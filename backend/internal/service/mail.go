package service

import (
	"context"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/pkg/metrics"
)

// MailSender is an email provider.
type MailSender interface {
	Name() string
	Send(ctx context.Context, to, subject, body string) error
}

// Email purposes. Sensitive ones carry a link that resets a password or
// proves ownership of an address: their body is redacted once sent.
const (
	EmailPasswordReset = "password_reset"
	EmailVerification  = "email_verification"
	EmailInvoice       = "invoice"
	EmailReceipt       = "receipt"
	EmailBilling       = "billing_notice"
	EmailAlert         = "alert"
	EmailSupport       = "support"
)

// MailService is the email outbox, delivered like SMS: written with the
// change that causes it, sent after commit by a loop that retries.
type MailService struct {
	*core
	sender MailSender
	wake   chan struct{}
}

func newMailService(c *core, sender MailSender) *MailService {
	return &MailService{core: c, sender: sender, wake: make(chan struct{}, 1)}
}

// ProviderName is the configured provider.
func (s *MailService) ProviderName() string {
	if s.sender == nil {
		return "none"
	}
	return s.sender.Name()
}

type outboundEmail struct {
	TenantID  pgtype.UUID
	To        string
	Subject   string
	Body      string
	Purpose   string
	Sensitive bool
}

// enqueueEmail adds a message to the outbox within the caller's transaction
// and wakes the sender after commit. A malformed address is logged and
// dropped: the change that caused the email must not fail because of it.
func (c *core) enqueueEmail(ctx context.Context, q *db.Queries, after *afterCommit, m outboundEmail) {
	addr, err := mail.ParseAddress(m.To)
	if err != nil {
		c.log.Warn().Str("purpose", m.Purpose).Msg("email not sent: invalid address")
		return
	}
	if _, err := q.EnqueueEmail(ctx, db.EnqueueEmailParams{
		TenantID: m.TenantID, ToAddress: addr.Address, Subject: m.Subject,
		Body: m.Body, Purpose: m.Purpose, Sensitive: m.Sensitive,
	}); err != nil {
		c.log.Error().Err(err).Str("purpose", m.Purpose).Msg("enqueue email")
		return
	}
	after.do(c.mail.kick)
}

// emailBody lays out a message: greeting, paragraphs, sign-off.
func (c *core) emailBody(name string, paragraphs ...string) string {
	var b strings.Builder
	if name = strings.TrimSpace(name); name != "" {
		b.WriteString("Hello " + firstName(name) + ",\n\n")
	} else {
		b.WriteString("Hello,\n\n")
	}
	for _, p := range paragraphs {
		b.WriteString(p + "\n\n")
	}
	b.WriteString("— " + c.cfg.AppName + "\n")
	return b.String()
}

func firstName(full string) string {
	if f := strings.Fields(full); len(f) > 0 {
		return f[0]
	}
	return full
}

func (s *MailService) kick() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Run delivers queued email until ctx ends.
func (s *MailService) Run(ctx context.Context) {
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		s.drain(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-s.wake:
		}
	}
}

// drain sends what is due, stopping as SMSService.drain does.
func (s *MailService) drain(ctx context.Context) {
	for ctx.Err() == nil {
		batch, err := s.q.ClaimQueuedEmails(context.WithoutCancel(ctx), 20)
		if err != nil {
			s.log.Error().Err(err).Msg("claim queued email")
			return
		}
		if len(batch) == 0 {
			return
		}
		for i, m := range batch {
			if ctx.Err() != nil {
				s.release(batch[i:])
				return
			}
			s.deliver(ctx, m)
		}
	}
}

// release hands back email claimed but not attempted.
func (s *MailService) release(batch []db.Email) {
	ids := make([]pgtype.UUID, len(batch))
	for i, m := range batch {
		ids[i] = m.ID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.q.ReleaseEmails(ctx, ids); err != nil {
		s.log.Warn().Err(err).Int("emails", len(ids)).Msg("hand back claimed email")
	}
}

// Email retries back off more gently than SMS: a mail server that is down
// is often down for a while, and nothing is lost by waiting.
var emailBackoff = []time.Duration{time.Minute, 10 * time.Minute, time.Hour, 6 * time.Hour}

func (s *MailService) deliver(ctx context.Context, m db.Email) {
	ctx = context.WithoutCancel(ctx) // seen through, as SMSService.deliver
	if s.sender == nil {
		metrics.Messages.WithLabelValues("email", "none", "failed").Inc()
		_ = s.q.MarkEmailFailed(ctx, db.MarkEmailFailedParams{ID: m.ID, LastError: ptr("no email provider configured")})
		return
	}
	provider := s.sender.Name()
	sendCtx, cancel := context.WithTimeout(ctx, time.Minute)
	err := s.sender.Send(sendCtx, m.ToAddress, m.Subject, m.Body)
	cancel()
	if err == nil {
		metrics.Messages.WithLabelValues("email", provider, "sent").Inc()
		if err := s.q.MarkEmailSent(ctx, db.MarkEmailSentParams{ID: m.ID, Provider: &provider}); err != nil {
			s.log.Error().Err(err).Msg("mark email sent")
		}
		return
	}
	msg := truncate(err.Error(), 300)
	if int(m.Attempts) > len(emailBackoff) {
		metrics.Messages.WithLabelValues("email", provider, "failed").Inc()
		s.log.Warn().Err(err).Str("purpose", m.Purpose).Msg("email failed permanently")
		_ = s.q.MarkEmailFailed(ctx, db.MarkEmailFailedParams{ID: m.ID, LastError: &msg})
		return
	}
	metrics.Messages.WithLabelValues("email", provider, "retry").Inc()
	next := s.now().Add(emailBackoff[m.Attempts-1])
	_ = s.q.MarkEmailRetry(ctx, db.MarkEmailRetryParams{ID: m.ID, LastError: &msg, NextAttemptAt: toTimestamp(next)})
}
