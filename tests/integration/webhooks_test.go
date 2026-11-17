//go:build integration

package integration

import (
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// hook is one message a receiver was sent.
type hook struct {
	ID, Timestamp, Signature string
	Body                     []byte
	Type                     string
	payload                  struct {
		Type string `json:"type"`
		Data struct {
			Shipment struct {
				ID             string `json:"id"`
				Status         string `json:"status"`
				TrackingNumber string `json:"tracking_number"`
			} `json:"shipment"`
			Event struct {
				Type   string `json:"type"`
				From   string `json:"from_status"`
				To     string `json:"to_status"`
				Source string `json:"source"`
				Actor  string `json:"actor"`
				Note   string `json:"note"`
			} `json:"event"`
			Message string `json:"message"`
		} `json:"data"`
	}
}

// receiver is a company's system: it records what it is sent and answers
// with the status set in answer.
type receiver struct {
	t      *testing.T
	srv    *httptest.Server
	got    chan hook
	answer atomic.Int32
}

func newReceiver(t *testing.T) *receiver {
	rc := &receiver{t: t, got: make(chan hook, 50)}
	rc.answer.Store(http.StatusOK)
	rc.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		h := hook{ID: r.Header.Get("webhook-id"), Timestamp: r.Header.Get("webhook-timestamp"), Signature: r.Header.Get("webhook-signature"), Body: body}
		if err := json.Unmarshal(body, &h.payload); err != nil {
			t.Errorf("a webhook that isn't JSON: %q", body)
		}
		h.Type = h.payload.Type
		rc.got <- h
		w.WriteHeader(int(rc.answer.Load()))
		_, _ = w.Write([]byte(`{"seen":true}`))
	}))
	t.Cleanup(rc.srv.Close)
	return rc
}

// next waits for the next message.
func (rc *receiver) next(want string) hook {
	rc.t.Helper()
	select {
	case h := <-rc.got:
		if h.Type != want {
			rc.t.Fatalf("got a %s webhook, want %s: %s", h.Type, want, h.Body)
		}
		return h
	case <-time.After(15 * time.Second):
		rc.t.Fatalf("no %s webhook", want)
	}
	return hook{}
}

// quiet checks that nothing more arrives for a moment.
func (rc *receiver) quiet() {
	rc.t.Helper()
	select {
	case h := <-rc.got:
		rc.t.Errorf("an unexpected %s webhook: %s", h.Type, h.Body)
	case <-time.After(700 * time.Millisecond):
	}
}

// verify checks a message's signature as a receiver would.
func (h hook) verify(t *testing.T, secret string) {
	t.Helper()
	sec, err := strconv.ParseInt(h.Timestamp, 10, 64)
	if err != nil || time.Since(time.Unix(sec, 0)) > time.Minute {
		t.Errorf("webhook-timestamp %q", h.Timestamp)
	}
	if want := service.SignWebhook(secret, h.ID, time.Unix(sec, 0), h.Body); h.Signature != want {
		t.Errorf("webhook-signature %q, want %q", h.Signature, want)
	}
}

func privateWebhooks(s *setup) { s.cfg.PrivateWebhooks = true }

var webhookSecretRe = regexp.MustCompile(`data-secret>(whsec_[^<]+)<`)

// webhookSecret reads a webhook's secret off its page.
func (b *browser) webhookSecret(page string) string {
	b.e.t.Helper()
	m := webhookSecretRe.FindStringSubmatch(b.get(page).Body)
	if m == nil {
		b.e.t.Fatal("the webhook's page should show its secret")
	}
	return html.UnescapeString(m[1]) // base64's + is written &#43;
}

// TestWebhooks (FR-047): a company's system is told as its shipments
// change, signed, and told again until it answers.
func TestWebhooks(t *testing.T) {
	e := newEnv(t, privateWebhooks)
	rc := newReceiver(t)
	office := e.company("Hooked Freight")
	driverID, driverPhone, pin := office.addDriver("Hana Girma")

	if p := office.get("/app/settings/webhooks"); p.Status != http.StatusOK || !strings.Contains(p.Body, "No webhooks yet") {
		t.Fatalf("the webhooks page: status %d", p.Status)
	}
	if p := office.post("/app/settings/webhooks", url.Values{"url": {rc.srv.URL}}); p.Status != http.StatusBadRequest || !strings.Contains(p.Body, "Choose at least one event") {
		t.Errorf("a webhook without events: status %d", p.Status)
	}
	p := office.mustPost("/app/settings/webhooks", url.Values{"url": {rc.srv.URL + "/hooks"}, "events": {
		service.WebhookShipmentCreated, service.WebhookStatusChanged, service.WebhookDelivered, service.WebhookIssueReported,
	}})
	page := p.Location
	secret := office.webhookSecret(page)

	// A test, on request.
	office.mustPost(page+"/test", nil)
	rc.next(service.WebhookTest).verify(t, secret)

	// Booked: created, with the shipment as the API sends it.
	id, tracking := office.book("Addis Ababa", "Hawassa", driverID)
	h := rc.next(service.WebhookShipmentCreated)
	h.verify(t, secret)
	if s := h.payload.Data.Shipment; s.ID != id || s.TrackingNumber != tracking || s.Status != "assigned" {
		t.Errorf("created: %s", h.Body)
	}
	if ev := h.payload.Data.Event; ev.Source != "dashboard" || ev.Actor != "Owner of Hooked Freight" {
		t.Errorf("created by %+v", ev)
	}

	// On the road: a status change. The driver's app is the source.
	d := e.signIn(driverPhone, pin)
	if r := d.status(clientID(), id, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	h = rc.next(service.WebhookStatusChanged)
	if ev := h.payload.Data.Event; ev.From != "assigned" || ev.To != "in_transit" || ev.Source != "app" || ev.Actor != "Hana Girma" {
		t.Errorf("departed: %s", h.Body)
	}

	// The receiver is down: the message waits to be tried again, and what
	// happened shows on the page.
	rc.answer.Store(http.StatusServiceUnavailable)
	if r := d.status(clientID(), id, "arrived"); !r.OK {
		t.Fatalf("arrive: %+v", r)
	}
	first := rc.next(service.WebhookStatusChanged)
	waitFor(t, "the failed attempt recorded", func() bool {
		return scalar[int64](t, `SELECT count(*) FROM webhook_deliveries WHERE event_id = $1 AND status = 'queued' AND attempts = 1 AND response_status = 503`, first.ID) == 1
	})
	if p := office.get(page); !strings.Contains(p.Body, "Trying again") || !strings.Contains(p.Body, "503 Service Unavailable") {
		t.Error("the page should show the failed attempt and the retry")
	}
	rc.answer.Store(http.StatusOK)
	exec(t, `UPDATE webhook_deliveries SET next_attempt_at = now() WHERE event_id = $1`, first.ID)

	// Delivered: a status change and a delivery. The retry goes out with
	// them, under the same webhook-id and a fresh signature.
	if r := d.pod(service.SyncPOD{ShipmentID: id, RecipientName: "Meron"}); !r.OK {
		t.Fatalf("proof: %+v", r)
	}
	seen := map[string]hook{}
	for range 3 {
		select {
		case h := <-rc.got:
			seen[h.Type+" "+h.payload.Data.Event.To] = h
			h.verify(t, secret)
		case <-time.After(15 * time.Second):
			t.Fatalf("only %d of 3 webhooks: %v", len(seen), seen)
		}
	}
	if seen["shipment.status_changed arrived"].ID != first.ID {
		t.Errorf("the retry should keep its webhook-id %s: %v", first.ID, seen)
	}
	if _, ok := seen["shipment.status_changed delivered"]; !ok {
		t.Errorf("no status change to delivered: %v", seen)
	}
	if h, ok := seen["shipment.delivered delivered"]; !ok || h.payload.Data.Shipment.Status != "delivered" {
		t.Errorf("no delivery: %v", seen)
	}

	// The receiver reports a problem on the confirmation page.
	customer := scalar[string](t, `SELECT customer_phone FROM shipments WHERE id = $1`, id)
	if rest, ok := strings.CutPrefix(customer, "+251"); ok {
		customer = "0" + rest // as the inbox shows it
	}
	asked := e.received("sms", customer, "Confirm receipt").Body
	e.browser().mustPost(asked[strings.Index(asked, "/c/"):], url.Values{"answer": {"problem"}, "feedback": {"Two bags torn"}})
	h = rc.next(service.WebhookIssueReported)
	if ev := h.payload.Data.Event; ev.Type != "customer_issue" || ev.Note != "Two bags torn" || ev.Source != "customer" {
		t.Errorf("issue: %s", h.Body)
	}

	// Paused, messages wait; resumed, they go.
	office.mustPost(page+"/pause", nil)
	_, other := office.book("Addis Ababa", "Adama", "")
	rc.quiet()
	office.mustPost(page+"/resume", nil)
	if h := rc.next(service.WebhookShipmentCreated); h.payload.Data.Shipment.TrackingNumber != other {
		t.Errorf("after resuming: %s", h.Body)
	}

	// Sent again on request, under the same webhook-id.
	delivered := scalar[string](t, `SELECT id::text FROM webhook_deliveries WHERE event_id = $1`, first.ID)
	office.mustPost(page+"/deliveries/"+delivered+"/resend", nil)
	if h := rc.next(service.WebhookStatusChanged); h.ID != first.ID {
		t.Errorf("resent with webhook-id %s, want %s", h.ID, first.ID)
	}

	// A new secret signs what follows.
	office.mustPost(page+"/secret", nil)
	newSecret := office.webhookSecret(page)
	if newSecret == secret {
		t.Fatal("the secret should change")
	}
	office.mustPost(page+"/test", nil)
	rc.next(service.WebhookTest).verify(t, newSecret)

	// Only the events asked for.
	office.mustPost(page, url.Values{"url": {rc.srv.URL + "/hooks"}, "events": {service.WebhookDelivered}})
	office.book("Addis Ababa", "Adama", "")
	rc.quiet()

	// Another company can neither see it nor use it.
	rival := e.company("Rival Freight")
	if p := rival.get(page); p.Status != http.StatusNotFound {
		t.Errorf("another company on the webhook's page: status %d", p.Status)
	}
	rival.post(page+"/test", nil)
	rival.post(page+"/delete", nil)
	rc.quiet()
	if n := scalar[int64](t, `SELECT count(*) FROM webhook_endpoints WHERE url = $1`, rc.srv.URL+"/hooks"); n != 1 {
		t.Error("another company removed the webhook")
	}

	// The audit log, and removing it.
	if p := office.get("/app/audit"); !strings.Contains(p.Body, "Added a webhook") || !strings.Contains(p.Body, "Gave a webhook a new secret") {
		t.Error("the audit log should record the webhook's changes")
	}
	office.mustPost(page+"/delete", nil)
	if n := scalar[int64](t, `SELECT count(*) FROM webhook_deliveries d JOIN webhook_endpoints w ON w.id = d.endpoint_id WHERE w.url = $1`, rc.srv.URL+"/hooks"); n != 0 {
		t.Errorf("%d messages left after removing the webhook", n)
	}
	office.book("Addis Ababa", "Adama", "")
	rc.quiet()
}

// TestWebhooksStayOnTheInternet: in production, a webhook can't point at
// the server's own network, by address or by a name that leads there.
func TestWebhooksStayOnTheInternet(t *testing.T) {
	e := newEnv(t)
	office := e.company("Curious Freight")
	for _, u := range []string{"http://example.com/hooks", "https://127.0.0.1/hooks", "https://localhost:5434/", "https://169.254.169.254/latest/meta-data"} {
		if p := office.post("/app/settings/webhooks", url.Values{"url": {u}, "events": {service.WebhookShipmentCreated}}); p.Status != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", u, p.Status)
		}
	}
}

// waitFor polls until ok, for work the app does in the background.
func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
