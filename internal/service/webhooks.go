package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/pkg/metrics"
)

// Webhooks (FR-047) tell a company's own systems when its shipments change.
// A message is written to an outbox with the change that causes it, as
// texts and emails are, and sent after commit by a loop that retries.
// Messages follow the Standard Webhooks specification
// (https://www.standardwebhooks.com): the webhook-id, webhook-timestamp
// and webhook-signature headers, signed with the endpoint's whsec_ secret,
// so a receiver can check them with any of that specification's libraries.

// The events an endpoint can ask for (AC-047.1).
const (
	WebhookShipmentCreated = "shipment.created"
	WebhookStatusChanged   = "shipment.status_changed"
	WebhookDelivered       = "shipment.delivered"
	WebhookIssueReported   = "shipment.issue_reported"
	// WebhookTest is the message the endpoint's page sends on request.
	WebhookTest = "test"
)

// WebhookEvents are the events an endpoint can ask for, in the order the
// page lists them.
var WebhookEvents = []string{WebhookShipmentCreated, WebhookStatusChanged, WebhookDelivered, WebhookIssueReported}

const (
	maxWebhookEndpoints = 5
	// webhookTimeout is how long a receiver has to answer.
	webhookTimeout = 10 * time.Second
	// retainWebhooks is how long sent messages are kept for the endpoint's
	// page. They carry customers' names and phones.
	retainWebhooks = 30 * 24 * time.Hour
)

// webhookBackoff spaces the retries of a message whose receiver didn't
// answer 2xx: seven attempts over about a day.
var webhookBackoff = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 6 * time.Hour, 12 * time.Hour}

// WebhookService manages a company's endpoints and sends their messages.
type WebhookService struct {
	*core
	client *http.Client
	wake   chan struct{}
}

func newWebhookService(c *core) *WebhookService {
	return &WebhookService{core: c, client: webhookClient(c.cfg.PrivateWebhooks), wake: make(chan struct{}, 1)}
}

// webhookClient is the client messages go out with. It follows no
// redirects and uses no proxy, and unless private is set it connects only
// to public addresses, checked as it connects so that a name can't point
// at the server's own network after it was checked.
func webhookClient(private bool) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	if !private {
		dialer.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip, err := netip.ParseAddr(host)
			if err != nil || !publicAddr(ip) {
				return errPrivateAddress
			}
			return nil
		}
	}
	return &http.Client{
		Timeout: webhookTimeout,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: webhookTimeout,
			MaxIdleConnsPerHost:   2,
			IdleConnTimeout:       time.Minute,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

var errPrivateAddress = errors.New("the address is not on the public internet")

// Ranges that are neither private nor loopback by Go's definitions but
// still don't belong to the public internet.
var notPublic = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), // benchmarking
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"), // NAT64, which could reach any of the above
	netip.MustParsePrefix("2001:db8::/32"),
}

// publicAddr reports whether ip is on the public internet.
func publicAddr(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return false
	}
	for _, p := range notPublic {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

// checkWebhookURL checks an endpoint's address as it is entered.
func (s *WebhookService) checkWebhookURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Opaque != "" || (u.Scheme != "https" && u.Scheme != "http") || len(raw) > 500 {
		return "", invalid("url", "Enter the full address, starting with https://.")
	}
	if u.User != nil {
		return "", invalid("url", "Leave the user name and password out of the address: check the signature instead.")
	}
	u.Fragment = ""
	if s.cfg.PrivateWebhooks {
		return u.String(), nil
	}
	if u.Scheme != "https" {
		return "", invalid("url", "The address must start with https://, so that what is sent can't be read on the way.")
	}
	host := strings.ToLower(u.Hostname())
	if ip, err := netip.ParseAddr(host); err == nil {
		if !publicAddr(ip) {
			return "", invalid("url", "The address must be on the public internet.")
		}
	} else if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || !strings.Contains(host, ".") {
		return "", invalid("url", "The address must be on the public internet.")
	}
	return u.String(), nil
}

func checkWebhookEvents(events []string) ([]string, error) {
	var out []string
	for _, e := range WebhookEvents {
		if slices.Contains(events, e) {
			out = append(out, e)
		}
	}
	if len(out) == 0 || len(out) != len(slices.Compact(slices.Sorted(slices.Values(events)))) {
		return nil, invalid("events", "Choose at least one event.")
	}
	return out, nil
}

// newWebhookSecret is a secret in the Standard Webhooks form: whsec_ and
// 24 random bytes in base64.
func newWebhookSecret() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	return "whsec_" + base64.StdEncoding.EncodeToString(b)
}

// SignWebhook is the webhook-signature header for a message: HMAC-SHA256,
// keyed by the secret's bytes, of its id, timestamp and body joined by
// dots.
func SignWebhook(secret, id string, at time.Time, body []byte) string {
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil {
		key = []byte(secret)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + strconv.FormatInt(at.Unix(), 10) + "."))
	mac.Write(body)
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// ---------------------------------------------------------------------------
// Endpoints
// ---------------------------------------------------------------------------

// Endpoints lists a company's endpoints with how each is doing.
func (s *WebhookService) Endpoints(ctx context.Context, tenantID pgtype.UUID) ([]db.ListWebhookEndpointsRow, error) {
	return s.q.ListWebhookEndpoints(ctx, tenantID)
}

// Endpoint is one of a company's endpoints.
func (s *WebhookService) Endpoint(ctx context.Context, tenantID, id pgtype.UUID) (db.WebhookEndpoint, error) {
	e, err := s.q.GetWebhookEndpoint(ctx, db.GetWebhookEndpointParams{ID: id, TenantID: tenantID})
	if isNoRows(err) {
		return e, notFound("That webhook doesn't exist.")
	}
	return e, err
}

// Deliveries is a page of the messages sent to an endpoint, newest first.
func (s *WebhookService) Deliveries(ctx context.Context, tenantID, endpointID pgtype.UUID, page, perPage int) ([]db.ListWebhookDeliveriesRow, error) {
	limit, offset := pageBounds(page, perPage)
	return s.q.ListWebhookDeliveries(ctx, db.ListWebhookDeliveriesParams{EndpointID: endpointID, TenantID: tenantID, Limit: limit, Offset: offset})
}

// CreateEndpoint adds an endpoint (AC-047.2).
func (s *WebhookService) CreateEndpoint(ctx context.Context, p *Principal, rawURL string, events []string, ci ClientInfo) (db.WebhookEndpoint, error) {
	if !p.CanManageCompany() {
		return db.WebhookEndpoint{}, ErrUnauthorized
	}
	u, err := s.checkWebhookURL(rawURL)
	if err != nil {
		return db.WebhookEndpoint{}, err
	}
	ev, err := checkWebhookEvents(events)
	if err != nil {
		return db.WebhookEndpoint{}, err
	}
	var out db.WebhookEndpoint
	err = s.tx(ctx, func(q *db.Queries, _ *afterCommit) error {
		if _, err := q.GetTenantForUpdate(ctx, p.TenantID); err != nil {
			return err
		}
		n, err := q.CountWebhookEndpoints(ctx, p.TenantID)
		if err != nil {
			return err
		}
		if n >= maxWebhookEndpoints {
			return limitReached(fmt.Sprintf("A company can have %d webhooks. Remove one that is no longer used.", maxWebhookEndpoints))
		}
		out, err = q.CreateWebhookEndpoint(ctx, db.CreateWebhookEndpointParams{
			TenantID: p.TenantID, Url: u, Events: ev, Secret: newWebhookSecret(), CreatedBy: p.UserID,
		})
		if err != nil {
			return err
		}
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "webhook_created", "webhook", out.ID, nil, map[string]any{"url": u, "events": ev})
		return nil
	})
	return out, err
}

// UpdateEndpoint changes an endpoint's address or events. Messages still
// waiting go to the new address.
func (s *WebhookService) UpdateEndpoint(ctx context.Context, p *Principal, id pgtype.UUID, rawURL string, events []string, ci ClientInfo) (db.WebhookEndpoint, error) {
	if !p.CanManageCompany() {
		return db.WebhookEndpoint{}, ErrUnauthorized
	}
	cur, err := s.Endpoint(ctx, p.TenantID, id)
	if err != nil {
		return cur, err
	}
	u, err := s.checkWebhookURL(rawURL)
	if err != nil {
		return cur, err
	}
	ev, err := checkWebhookEvents(events)
	if err != nil {
		return cur, err
	}
	out, err := s.q.UpdateWebhookEndpoint(ctx, db.UpdateWebhookEndpointParams{ID: id, TenantID: p.TenantID, Url: u, Events: ev})
	if err != nil {
		return cur, err
	}
	s.audit(ctx, s.q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "webhook_updated", "webhook", id,
		map[string]any{"url": cur.Url, "events": cur.Events}, map[string]any{"url": out.Url, "events": out.Events})
	return out, nil
}

// SetActive pauses an endpoint or sends to it again. A paused endpoint's
// messages are still written, and wait for it.
func (s *WebhookService) SetActive(ctx context.Context, p *Principal, id pgtype.UUID, active bool, ci ClientInfo) error {
	if !p.CanManageCompany() {
		return ErrUnauthorized
	}
	e, err := s.q.SetWebhookEndpointActive(ctx, db.SetWebhookEndpointActiveParams{ID: id, TenantID: p.TenantID, Active: active})
	if isNoRows(err) {
		return notFound("That webhook doesn't exist.")
	}
	if err != nil {
		return err
	}
	action := "webhook_paused"
	if active {
		action = "webhook_resumed"
		s.kick()
	}
	s.audit(ctx, s.q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), action, "webhook", e.ID, nil, map[string]any{"url": e.Url})
	return nil
}

// RollSecret gives an endpoint a new secret. Messages are signed with it
// from now on, retries of older ones included.
func (s *WebhookService) RollSecret(ctx context.Context, p *Principal, id pgtype.UUID, ci ClientInfo) error {
	if !p.CanManageCompany() {
		return ErrUnauthorized
	}
	e, err := s.q.SetWebhookSecret(ctx, db.SetWebhookSecretParams{ID: id, TenantID: p.TenantID, Secret: newWebhookSecret()})
	if isNoRows(err) {
		return notFound("That webhook doesn't exist.")
	}
	if err != nil {
		return err
	}
	s.audit(ctx, s.q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "webhook_secret_rolled", "webhook", e.ID, nil, map[string]any{"url": e.Url})
	return nil
}

// DeleteEndpoint removes an endpoint and its messages.
func (s *WebhookService) DeleteEndpoint(ctx context.Context, p *Principal, id pgtype.UUID, ci ClientInfo) error {
	if !p.CanManageCompany() {
		return ErrUnauthorized
	}
	e, err := s.q.DeleteWebhookEndpoint(ctx, db.DeleteWebhookEndpointParams{ID: id, TenantID: p.TenantID})
	if isNoRows(err) {
		return notFound("That webhook doesn't exist.")
	}
	if err != nil {
		return err
	}
	s.audit(ctx, s.q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "webhook_deleted", "webhook", e.ID, map[string]any{"url": e.Url, "events": e.Events}, nil)
	return nil
}

// SendTest sends an endpoint a test message, whatever events it asked for.
func (s *WebhookService) SendTest(ctx context.Context, p *Principal, id pgtype.UUID) error {
	if !p.CanManageCompany() {
		return ErrUnauthorized
	}
	e, err := s.Endpoint(ctx, p.TenantID, id)
	if err != nil {
		return err
	}
	eventID := newUUID()
	body := webhookBody(WebhookTest, s.now(), map[string]any{
		"message": "A test from " + s.cfg.AppName + ", sent from the webhook's page by " + p.Name + ".",
	})
	if _, err := s.q.EnqueueWebhookTo(ctx, db.EnqueueWebhookToParams{
		TenantID: p.TenantID, EndpointID: e.ID, EventID: eventID, EventType: WebhookTest, Payload: body,
	}); err != nil {
		return err
	}
	s.kick()
	return nil
}

// Resend sends a delivered or failed message again, from the start of its
// retries (AC-047.4).
func (s *WebhookService) Resend(ctx context.Context, p *Principal, deliveryID pgtype.UUID) error {
	if !p.CanManageCompany() {
		return ErrUnauthorized
	}
	_, err := s.q.ResendWebhook(ctx, db.ResendWebhookParams{ID: deliveryID, TenantID: p.TenantID})
	if isNoRows(err) {
		return notFound("That message doesn't exist or is still being sent.")
	}
	if err != nil {
		return err
	}
	s.kick()
	return nil
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

func webhookBody(eventType string, at time.Time, data any) []byte {
	return mustJSON(map[string]any{"type": eventType, "timestamp": at.UTC().Format(time.RFC3339), "data": data})
}

// webhookTypes are the webhook events a shipment event is: none for most,
// and two for a delivery or a problem, which are status changes as well.
func webhookTypes(e shipmentEvent) []string {
	switch e.Type {
	case "created":
		return []string{WebhookShipmentCreated}
	case "customer_issue":
		return []string{WebhookIssueReported}
	case "status_changed":
		if e.From == e.To {
			return nil
		}
		out := []string{WebhookStatusChanged}
		switch e.To {
		case model.StatusDelivered:
			out = append(out, WebhookDelivered)
		case model.StatusIssue:
			out = append(out, WebhookIssueReported)
		}
		return out
	}
	return nil
}

// enqueueWebhooks writes a message for each endpoint that asked for what
// happened, within the caller's transaction, and wakes the sender after
// commit. A failure is logged: the change must not fail because of it.
func (c *core) enqueueWebhooks(ctx context.Context, q *db.Queries, after *afterCommit, a Actor, sh db.Shipment, e shipmentEvent, at time.Time) {
	types := webhookTypes(e)
	if len(types) == 0 || c.webhooks == nil {
		return
	}
	data := map[string]any{
		"shipment": AsJSON(sh, c.cfg.BaseURL),
		"event": EventJSON{
			Type: e.Type, FromStatus: e.From, ToStatus: e.To, Reason: e.Reason, Note: e.Note,
			Source: a.Source, Actor: a.Name, Lat: e.Lat, Lng: e.Lng, At: at.UTC(),
		},
	}
	var sent int64
	for _, t := range types {
		n, err := q.EnqueueWebhooks(ctx, db.EnqueueWebhooksParams{
			EventID: newUUID(), EventType: t, ShipmentID: sh.ID, Payload: webhookBody(t, at, data), TenantID: sh.TenantID,
		})
		if err != nil {
			c.log.Error().Err(err).Str("event", t).Msg("enqueue webhook")
			return
		}
		sent += n
	}
	if sent > 0 && after != nil {
		after.do(c.webhooks.kick)
	}
}

func newUUID() pgtype.UUID {
	var u pgtype.UUID
	if _, err := rand.Read(u.Bytes[:]); err != nil {
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	u.Bytes[6] = (u.Bytes[6] & 0x0f) | 0x40 // version 4
	u.Bytes[8] = (u.Bytes[8] & 0x3f) | 0x80 // RFC 4122 variant
	u.Valid = true
	return u
}

func (s *WebhookService) kick() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Run sends queued messages until ctx ends.
func (s *WebhookService) Run(ctx context.Context) {
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

// webhookSenders is how many messages go out at once, so that one slow
// receiver doesn't hold up every other company's.
const webhookSenders = 8

// drain sends what is due, stopping as MailService.drain does.
func (s *WebhookService) drain(ctx context.Context) {
	for ctx.Err() == nil {
		batch, err := s.q.ClaimQueuedWebhooks(context.WithoutCancel(ctx), 20)
		if err != nil {
			s.log.Error().Err(err).Msg("claim queued webhooks")
			return
		}
		if len(batch) == 0 {
			return
		}
		var wg sync.WaitGroup
		slots := make(chan struct{}, webhookSenders)
		for i, m := range batch {
			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
			}
			if ctx.Err() != nil {
				s.release(batch[i:])
				break
			}
			wg.Go(func() {
				defer func() { <-slots }()
				s.deliver(ctx, m)
			})
		}
		wg.Wait()
	}
}

func (s *WebhookService) release(batch []db.WebhookDelivery) {
	ids := make([]pgtype.UUID, len(batch))
	for i, m := range batch {
		ids[i] = m.ID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.q.ReleaseWebhooks(ctx, ids); err != nil {
		s.log.Warn().Err(err).Int("webhooks", len(ids)).Msg("hand back claimed webhooks")
	}
}

// deliver sends one message and records how it went (AC-047.4).
func (s *WebhookService) deliver(ctx context.Context, m db.WebhookDelivery) {
	ctx = context.WithoutCancel(ctx)
	target, err := s.q.GetWebhookTarget(ctx, m.EndpointID)
	if err != nil {
		s.log.Error().Err(err).Msg("read webhook endpoint")
		s.release([]db.WebhookDelivery{m})
		return
	}
	if !target.Active { // paused between the claim and now
		s.release([]db.WebhookDelivery{m})
		return
	}
	status, body, took, err := s.post(ctx, target.Url, target.Secret, m)
	ms := int32(min(took.Milliseconds(), 1<<30))
	var code *int32
	if status != 0 {
		code = ptr(int32(status))
	}
	var respBody *string
	if body != "" {
		respBody = &body
	}
	if err == nil && status >= 200 && status < 300 {
		metrics.Messages.WithLabelValues("webhook", "http", "sent").Inc()
		if err := s.q.MarkWebhookDelivered(ctx, db.MarkWebhookDeliveredParams{ID: m.ID, ResponseStatus: code, ResponseBody: respBody, DurationMs: &ms}); err != nil {
			s.log.Error().Err(err).Msg("mark webhook delivered")
		}
		return
	}
	msg := fmt.Sprintf("The receiver answered %d %s.", status, http.StatusText(status))
	if err != nil {
		msg = webhookError(err)
	}
	if int(m.Attempts) > len(webhookBackoff) {
		metrics.Messages.WithLabelValues("webhook", "http", "failed").Inc()
		_ = s.q.MarkWebhookFailed(ctx, db.MarkWebhookFailedParams{ID: m.ID, ResponseStatus: code, ResponseBody: respBody, DurationMs: &ms, LastError: &msg})
		return
	}
	metrics.Messages.WithLabelValues("webhook", "http", "retry").Inc()
	next := s.now().Add(webhookBackoff[m.Attempts-1])
	_ = s.q.MarkWebhookRetry(ctx, db.MarkWebhookRetryParams{
		ID: m.ID, ResponseStatus: code, ResponseBody: respBody, DurationMs: &ms, LastError: &msg, NextAttemptAt: toTimestamp(next),
	})
}

// post sends a message, returning the answer's status and the start of
// its body.
func (s *WebhookService) post(ctx context.Context, target, secret string, m db.WebhookDelivery) (int, string, time.Duration, error) {
	id := m.EventID.String()
	at := s.now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(m.Payload))
	if err != nil {
		return 0, "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", s.cfg.AppName+"-Webhooks/1")
	req.Header.Set("webhook-id", id)
	req.Header.Set("webhook-timestamp", strconv.FormatInt(at.Unix(), 10))
	req.Header.Set("webhook-signature", SignWebhook(secret, id, at, m.Payload))
	start := time.Now()
	res, err := s.client.Do(req)
	if err != nil {
		return 0, "", time.Since(start), err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
	took := time.Since(start)
	body := strings.ToValidUTF8(string(raw), "")
	if utf8.RuneCountInString(body) > 500 {
		body = string([]rune(body)[:500]) + "…"
	}
	return res.StatusCode, body, took, nil
}

// webhookError says in a sentence why a message couldn't be sent.
func webhookError(err error) string {
	var dns *net.DNSError
	var ne net.Error
	switch {
	case errors.Is(err, errPrivateAddress):
		return "The address leads to a private network, which webhooks don't reach."
	case errors.As(err, &dns):
		return "The address's name couldn't be found (" + dns.Name + ")."
	case errors.As(err, &ne) && ne.Timeout():
		return fmt.Sprintf("The receiver didn't answer within %d seconds.", int(webhookTimeout.Seconds()))
	case errors.Is(err, syscall.ECONNREFUSED):
		return "The receiver refused the connection."
	}
	return truncate(err.Error(), 300)
}

// WebhookPayload pretty-prints a message's body for its page.
func WebhookPayload(raw []byte) string {
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return string(raw)
	}
	return out.String()
}
