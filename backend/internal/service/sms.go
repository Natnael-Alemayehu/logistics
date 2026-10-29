package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/internal/phone"
)

// SMSSender is an SMS provider. It receives numbers in local form and is
// responsible for any conversion its API needs.
type SMSSender interface {
	Name() string
	Send(ctx context.Context, to, body string) error
}

// Message purposes. Sensitive ones carry credentials or codes: their body is
// redacted once sent and they bypass opt-out, since the recipient asked for
// them.
const (
	PurposeCustomerUpdate = "customer_update"
	PurposeDriverAssigned = "driver_assigned"
	PurposeDriverMessage  = "driver_message"
	PurposeDriverReply    = "driver_reply"
	PurposeCredential     = "credential"
	PurposeOTP            = "otp"
	PurposeInbound        = "inbound"
)

// Retry schedule for a failed send: after the first failure wait a minute,
// then five, then fifteen, then give up.
var smsBackoff = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}

// SMSService is the outbox: messages are written in the same transaction as
// the change that caused them, then delivered by a background loop with
// retries. It also interprets the status codes drivers text in.
type SMSService struct {
	*core
	sender    SMSSender
	wake      chan struct{}
	shipments *ShipmentService
}

func newSMSService(c *core, sender SMSSender) *SMSService {
	return &SMSService{core: c, sender: sender, wake: make(chan struct{}, 1)}
}

// ProviderName is the configured provider, for the settings page.
func (s *SMSService) ProviderName() string {
	if s.sender == nil {
		return "none"
	}
	return s.sender.Name()
}

type outboundSMS struct {
	TenantID   pgtype.UUID
	ShipmentID pgtype.UUID
	Phone      string
	Body       string
	Purpose    string
	Sensitive  bool
}

// enqueue adds a message to the outbox within the caller's transaction and
// wakes the sender after commit.
func (c *core) enqueueSMS(ctx context.Context, q *db.Queries, after *afterCommit, m outboundSMS) {
	local, err := phone.Normalize(m.Phone)
	if err != nil {
		_, _ = q.RecordSMS(ctx, db.RecordSMSParams{
			TenantID: m.TenantID, ShipmentID: m.ShipmentID, Direction: "outbound",
			Phone: m.Phone, Body: redactIf(m.Sensitive, m.Body), Purpose: m.Purpose,
			Status: "skipped", LastError: ptr("invalid phone number"),
		})
		return
	}
	if _, err := q.EnqueueSMS(ctx, db.EnqueueSMSParams{
		TenantID: m.TenantID, ShipmentID: m.ShipmentID, Phone: local,
		Body: m.Body, Purpose: m.Purpose, Sensitive: m.Sensitive,
	}); err != nil {
		c.log.Error().Err(err).Str("purpose", m.Purpose).Msg("enqueue sms")
		return
	}
	after.do(c.sms.kick)
}

func redactIf(sensitive bool, body string) string {
	if sensitive {
		return "[redacted]"
	}
	return body
}

func (s *SMSService) kick() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Run delivers queued messages until ctx ends.
func (s *SMSService) Run(ctx context.Context) {
	tick := time.NewTicker(5 * time.Second)
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

func (s *SMSService) drain(ctx context.Context) {
	for ctx.Err() == nil {
		batch, err := s.q.ClaimQueuedSMS(ctx, 20)
		if err != nil {
			if ctx.Err() == nil {
				s.log.Error().Err(err).Msg("claim queued sms")
			}
			return
		}
		if len(batch) == 0 {
			return
		}
		for _, m := range batch {
			s.deliver(ctx, m)
		}
	}
}

func (s *SMSService) deliver(ctx context.Context, m db.SmsMessage) {
	if s.sender == nil {
		_ = s.q.MarkSMSSkipped(ctx, db.MarkSMSSkippedParams{ID: m.ID, LastError: ptr("no SMS provider configured")})
		return
	}
	if !m.Sensitive {
		if out, err := s.q.IsPhoneOptedOut(ctx, m.Phone); err == nil && out {
			_ = s.q.MarkSMSSkipped(ctx, db.MarkSMSSkippedParams{ID: m.ID, LastError: ptr("recipient opted out")})
			return
		}
	}

	sendCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	err := s.sender.Send(sendCtx, m.Phone, m.Body)
	cancel()
	if err == nil {
		if err := s.q.MarkSMSSent(ctx, db.MarkSMSSentParams{ID: m.ID, Provider: ptr(s.sender.Name())}); err != nil {
			s.log.Error().Err(err).Msg("mark sms sent")
		}
		return
	}

	msg := truncate(err.Error(), 300)
	if int(m.Attempts) > len(smsBackoff) {
		s.log.Warn().Err(err).Str("purpose", m.Purpose).Msg("sms failed permanently")
		_ = s.q.MarkSMSFailed(ctx, db.MarkSMSFailedParams{ID: m.ID, LastError: &msg})
		return
	}
	next := s.now().Add(smsBackoff[m.Attempts-1])
	_ = s.q.MarkSMSRetry(ctx, db.MarkSMSRetryParams{ID: m.ID, LastError: &msg, NextAttemptAt: toTimestamp(next)})
}

// Log returns a page of a tenant's SMS history.
func (s *SMSService) Log(ctx context.Context, tenantID pgtype.UUID, page, perPage int) ([]db.ListSMSByTenantRow, int64, error) {
	limit, offset := pageBounds(page, perPage)
	rows, err := s.q.ListSMSByTenant(ctx, db.ListSMSByTenantParams{TenantID: tenantID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountSMSByTenant(ctx, tenantID)
	return rows, total, err
}

// ---------------------------------------------------------------------------
// Inbound: drivers on feature phones report status by SMS (FR-019).
//
//	ET <ref> <code> [note]     ref is the end of the tracking number
//	<code> [note]              when the driver has exactly one active shipment
//	STOP / START               opt out of, or back into, notifications
//	HELP                       the code list
//
// Codes: 1 departed, 2 delayed, 3 arrived, 4 delivered, 5 problem.
// ---------------------------------------------------------------------------

var smsCodes = map[string]string{
	"1": model.StatusInTransit,
	"2": model.StatusDelayed,
	"3": model.StatusArrived,
	"4": model.StatusDelivered,
	"5": model.StatusIssue,
}

// SMSCommand is a parsed inbound message.
type SMSCommand struct {
	Keyword string // STOP, START, HELP, or "" for a status report
	Ref     string // tracking-number suffix, upper case; may be empty
	Status  string
	Note    string
}

var errUnrecognisedSMS = errors.New("unrecognised message")

// ParseSMSCommand interprets an inbound message body.
func ParseSMSCommand(body string) (SMSCommand, error) {
	fields := strings.Fields(strings.TrimSpace(body))
	if len(fields) == 0 {
		return SMSCommand{}, errUnrecognisedSMS
	}
	first := strings.ToUpper(fields[0])
	switch first {
	case "STOP", "UNSUBSCRIBE", "ቁም":
		return SMSCommand{Keyword: "STOP"}, nil
	case "START", "UNSTOP":
		return SMSCommand{Keyword: "START"}, nil
	case "HELP", "?", "እርዳታ":
		return SMSCommand{Keyword: "HELP"}, nil
	}

	var cmd SMSCommand
	if first == "ET" && len(fields) >= 3 {
		cmd.Ref = normaliseRef(fields[1])
		fields = fields[2:]
	} else if strings.HasPrefix(first, "ET-") && len(fields) >= 2 {
		// The full tracking number pasted in.
		cmd.Ref = normaliseRef(first)
		fields = fields[1:]
	}
	status, ok := smsCodes[fields[0]]
	if !ok {
		return SMSCommand{}, errUnrecognisedSMS
	}
	cmd.Status = status
	cmd.Note = strings.TrimSpace(strings.Join(fields[1:], " "))
	return cmd, nil
}

func normaliseRef(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToUpper(r)
		}
		return -1
	}, s)
}

// smsHelp is the reply to HELP and to anything unrecognised.
func smsHelp() string {
	return "Reply: ET <last 4 of tracking no.> <code> [note]. Codes: 1 Departed, 2 Delayed, 3 Arrived, 4 Delivered, 5 Problem. Example: ET 7K3M 1"
}

// InboundResult says what an inbound message did. The dashboard's simulator
// shows it and the webhook returns it, so a test can tell a recorded update
// from a message that was ignored.
type InboundResult struct {
	// Outcome is recorded, unchanged, refused, replied, opted_out, opted_in,
	// unknown_sender or bad_number.
	Outcome string `json:"outcome"`
	// Detail says the same for people.
	Detail string `json:"detail"`
	// Reply is the text sent back, if any.
	Reply string `json:"reply,omitempty"`
}

// Inbound handles a message received from the provider. Replies go only to
// registered drivers, so a stranger cannot make the platform spend money.
func (s *SMSService) Inbound(ctx context.Context, from, body string) (InboundResult, error) {
	local, err := phone.Normalize(from)
	if err != nil {
		// Not a number we could ever reply to.
		return InboundResult{Outcome: "bad_number", Detail: fmt.Sprintf("%q is not a phone number we can read.", from)}, nil
	}
	cmd, parseErr := ParseSMSCommand(body)

	if parseErr == nil && cmd.Keyword == "STOP" {
		s.logInbound(ctx, pgtype.UUID{}, pgtype.UUID{}, local, body)
		return InboundResult{Outcome: "opted_out", Detail: local + " gets no more text messages until it sends START."},
			s.q.AddSMSOptOut(ctx, local)
	}
	if parseErr == nil && cmd.Keyword == "START" {
		s.logInbound(ctx, pgtype.UUID{}, pgtype.UUID{}, local, body)
		return InboundResult{Outcome: "opted_in", Detail: local + " gets text messages again."},
			s.q.RemoveSMSOptOut(ctx, local)
	}

	user, err := s.q.GetUserByPhone(ctx, &local)
	if err != nil || user.Role != model.RoleDriver || !user.IsActive {
		s.logInbound(ctx, pgtype.UUID{}, pgtype.UUID{}, local, body)
		return InboundResult{Outcome: "unknown_sender",
			Detail: local + " is not an active driver's number, so the message was logged and not answered."}, nil
	}

	reply := func(shipmentID pgtype.UUID, outcome, detail, text string) (InboundResult, error) {
		res := InboundResult{Outcome: outcome, Detail: detail, Reply: text}
		return res, s.tx(ctx, func(q *db.Queries, after *afterCommit) error {
			s.enqueueSMS(ctx, q, after, outboundSMS{
				TenantID: user.TenantID, ShipmentID: shipmentID, Phone: local,
				Body: text, Purpose: PurposeDriverReply, Sensitive: false,
			})
			return nil
		})
	}

	if parseErr != nil || cmd.Keyword == "HELP" {
		s.logInbound(ctx, user.TenantID, pgtype.UUID{}, local, body)
		return reply(pgtype.UUID{}, "replied", "Not a status report, so "+user.FullName+" was sent the list of codes.", smsHelp())
	}

	active, err := s.q.ListActiveShipmentsByDriver(ctx, db.ListActiveShipmentsByDriverParams{DriverID: user.ID, TenantID: user.TenantID})
	if err != nil {
		return InboundResult{}, err
	}
	target, msg := matchShipment(active, cmd.Ref)
	if target == nil {
		s.logInbound(ctx, user.TenantID, pgtype.UUID{}, local, body)
		return reply(pgtype.UUID{}, "replied", "No shipment matched, so "+user.FullName+" was told why.", msg)
	}
	s.logInbound(ctx, user.TenantID, target.ID, local, body)

	actor := Actor{UserID: user.ID, TenantID: user.TenantID, Name: user.FullName, Role: user.Role, Source: SourceSMS}
	reason := ""
	if cmd.Status == model.StatusDelayed || cmd.Status == model.StatusIssue {
		reason = "other"
	}
	_, err = s.shipments.ChangeStatus(ctx, actor, target.ID, StatusChange{
		Status: cmd.Status, Reason: reason, Note: cmd.Note, By: model.ActorDriverSMS,
	})
	short := ShortRef(target.TrackingNumber)
	label := model.StatusLabel(cmd.Status)
	switch {
	case err == nil:
		return reply(target.ID, "recorded", fmt.Sprintf("%s recorded on %s.", label, target.TrackingNumber),
			fmt.Sprintf("%s: %s recorded. Thank you.", short, label))
	case errors.Is(err, model.ErrSameStatus):
		return reply(target.ID, "unchanged", fmt.Sprintf("%s was already %s.", target.TrackingNumber, strings.ToLower(label)),
			fmt.Sprintf("%s is already %s.", short, label))
	default:
		if de, ok := AsError(err); ok {
			return reply(target.ID, "refused", fmt.Sprintf("Not applied to %s: %s", target.TrackingNumber, de.Message),
				fmt.Sprintf("%s: %s", short, de.Message))
		}
		return InboundResult{}, err
	}
}

// matchShipment picks the shipment an SMS refers to among a driver's active
// work, or explains why it cannot.
func matchShipment(active []db.Shipment, ref string) (*db.Shipment, string) {
	if len(active) == 0 {
		return nil, "You have no active shipments."
	}
	if ref == "" {
		if len(active) == 1 {
			return &active[0], ""
		}
		refs := make([]string, 0, len(active))
		for _, s := range active {
			refs = append(refs, ShortRef(s.TrackingNumber))
		}
		return nil, "You have several shipments. Start with ET and the code, e.g. ET " + refs[0] + " 1. Yours: " + strings.Join(refs, ", ")
	}
	var found *db.Shipment
	for i := range active {
		if strings.HasSuffix(normaliseRef(active[i].TrackingNumber), ref) {
			if found != nil {
				return nil, "More than one shipment ends in " + ref + ". Send more characters."
			}
			found = &active[i]
		}
	}
	if found == nil {
		return nil, "No active shipment of yours ends in " + ref + "."
	}
	return found, ""
}

func (s *SMSService) logInbound(ctx context.Context, tenantID, shipmentID pgtype.UUID, from, body string) {
	_, err := s.q.RecordSMS(ctx, db.RecordSMSParams{
		TenantID: tenantID, ShipmentID: shipmentID, Direction: "inbound",
		Phone: from, Body: truncate(body, 480), Purpose: PurposeInbound, Status: "received",
	})
	if err != nil {
		s.log.Error().Err(err).Msg("record inbound sms")
	}
}

// ShortRef is the part of a tracking number a driver texts: its last four
// characters, unique among one driver's active shipments in practice.
func ShortRef(tracking string) string {
	n := normaliseRef(tracking)
	if len(n) <= 4 {
		return n
	}
	return n[len(n)-4:]
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
