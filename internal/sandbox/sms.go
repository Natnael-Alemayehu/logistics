package sandbox

// Ethio telecom's side of Kiflet's SMS adapter (pkg/sms/ethiotelecom.go).
// Ethio telecom publishes no HTTP API, so this is the contract as Kiflet
// defines it:
//
//	POST /ethiotelecom/auth/token  {"api_key"} → {"token", "expires_in"}
//	POST /ethiotelecom/sms/send    Authorization: Bearer <token>
//	                               {"to": "+2519…", "from": <sender ID>, "message"}
//	                               → {"message_id", "status": "accepted"}
//
// A refused token is answered 401; a text the gateway will never take, 400
// with status "rejected"; a gateway in trouble, 503. The gateway calls
// Kiflet back, with the shared secret in X-Webhook-Secret:
//
//	POST {Kiflet}/api/v1/sms/status   {"message_id", "status": "delivered" or "undelivered", "error", "done_at"}
//	POST {Kiflet}/api/v1/sms/inbound  {"message_id", "from": "+2519…", "to": <short code>, "message", "received_at"}

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/natnael-alemayehu/kiflet/pkg/console"
)

// phone is a phone the sandbox plays: its texts and its two wallets.
type phone struct {
	Number   string `json:"number"`
	Texts    []text `json:"texts"`
	Telebirr int64  `json:"telebirr_santim"`
	CBEBirr  int64  `json:"cbebirr_santim"`
}

// text is a text message on a phone.
type text struct {
	ID string    `json:"id"`
	At time.Time `json:"at"`
	// From is who sent it: Kiflet's sender ID, telebirr (127), CBE Birr, or
	// this phone. To is the short code a text from this phone went to.
	From string `json:"from"`
	To   string `json:"to,omitempty"`
	Mine bool   `json:"mine"`
	Body string `json:"body"`
	// State is what became of it: for a text to the phone, whether it
	// arrived; for one from it, what Kiflet answered. Report is what Kiflet
	// made of the delivery report.
	State  string `json:"state,omitempty"`
	Report string `json:"report,omitempty"`
	Failed bool   `json:"failed,omitempty"`
}

const (
	keepTexts = 200
	// startingBalance is what a new phone's wallets hold: 200,000 birr.
	startingBalance = 200_000_00
	smsTokenLife    = 10 * time.Minute
)

// phoneLocked returns a phone, adding it with full wallets the first time.
// s.mu must be held.
func (s *Sandbox) phoneLocked(number string) *phone {
	p := s.phones[number]
	if p == nil {
		p = &phone{Number: number, Telebirr: startingBalance, CBEBirr: startingBalance}
		s.phones[number] = p
	}
	return p
}

// phoneCopy is a copy of a phone, or a new one with full wallets that is
// kept only once something happens to it.
func (s *Sandbox) phoneCopy(number string) phone {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.phones[number]; p != nil {
		c := *p
		c.Texts = slices.Clone(p.Texts)
		return c
	}
	return phone{Number: number, Telebirr: startingBalance, CBEBirr: startingBalance}
}

func (p *phone) add(t text) {
	p.Texts = append(p.Texts, t)
	if len(p.Texts) > keepTexts {
		p.Texts = p.Texts[len(p.Texts)-keepTexts:]
	}
}

// updateText changes one of a phone's texts, if it is still kept.
func (s *Sandbox) updateText(number, id string, change func(*text)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.phones[number]; p != nil {
		if i := slices.IndexFunc(p.Texts, func(t text) bool { return t.ID == id }); i >= 0 {
			change(&p.Texts[i])
		}
	}
}

// receive puts a text from a provider, such as a receipt, on a phone.
// s.mu must be held.
func (s *Sandbox) receive(p *phone, from, body string) {
	p.add(text{ID: "R" + random(upperChars, 12), At: time.Now(), From: from, Body: body, State: "Delivered"})
}

// parts is roughly how many messages a text goes as: 160 characters in one,
// or 153 to a part when longer, in the GSM alphabet; 70 and 67 in Unicode,
// which Amharic needs.
func parts(s string) int {
	single, multi := 160, 153
	for _, r := range s {
		if r > '~' || (r < ' ' && r != '\n' && r != '\r') {
			single, multi = 70, 67
			break
		}
	}
	if n := utf8.RuneCountInString(s); n > single {
		return (n + multi - 1) / multi
	}
	return 1
}

// gatewayError refuses a call as the gateway does.
func gatewayError(w http.ResponseWriter, status int, kind, msg string) {
	writeJSON(w, status, map[string]string{"status": kind, "error": msg})
}

func (s *Sandbox) smsToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		APIKey string `json:"api_key"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.APIKey != s.cfg.SMS.APIKey {
		note(r, "login refused: not the account's API key")
		gatewayError(w, http.StatusUnauthorized, "error", "invalid API key")
		return
	}
	token := s.issueToken(s.smsTokens, smsTokenLife)
	note(r, "signed in for %s", smsTokenLife)
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "expires_in": int(smsTokenLife / time.Second)})
}

func (s *Sandbox) smsSend(w http.ResponseWriter, r *http.Request) {
	f := s.currentFaults()
	if f.SMSDown {
		note(r, "refused: the gateway is down (fault)")
		gatewayError(w, http.StatusServiceUnavailable, "error", "service temporarily unavailable")
		return
	}
	if f.SMSSlow {
		t := time.NewTimer(s.cfg.Timing.Slow)
		defer t.Stop()
		select {
		case <-t.C:
		case <-r.Context().Done():
			note(r, "Kiflet stopped waiting for the slow gateway (fault): nothing was sent")
			return
		case <-s.ctx.Done():
			return
		}
	}
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if f.SMSRefuseTokens || !s.validToken(s.smsTokens, token) {
		why := "the token is unknown or has expired"
		if f.SMSRefuseTokens {
			why = "tokens are refused (fault)"
		}
		note(r, "refused: %s", why)
		gatewayError(w, http.StatusUnauthorized, "error", "invalid or expired token")
		return
	}
	var req struct {
		To      string `json:"to"`
		From    string `json:"from"`
		Message string `json:"message"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	number, ok := "", strings.HasPrefix(req.To, "+251") && len(req.To) == 13
	if ok {
		number, ok = localNumber(req.To)
	}
	var problem string
	switch {
	case req.From != s.cfg.SMS.SenderID:
		problem = fmt.Sprintf("sender ID %q is not registered for this account", req.From)
	case !ok:
		problem = fmt.Sprintf("invalid destination %q: want +251 and a nine-digit mobile number", req.To)
	case strings.TrimSpace(req.Message) == "":
		problem = "empty message"
	case parts(req.Message) > 10:
		problem = "message longer than 10 parts"
	case f.isBarred(number):
		problem = "the number is barred (fault)"
	}
	if problem != "" {
		note(r, "rejected: %s", problem)
		gatewayError(w, http.StatusBadRequest, "rejected", problem)
		return
	}

	id := "ET" + random(upperChars, 14)
	s.mu.Lock()
	s.phoneLocked(number).add(text{ID: id, At: time.Now(), From: req.From, Body: req.Message, State: "Arriving"})
	s.mu.Unlock()
	s.terminal.Print(console.Item{Kind: "sms", To: number, Body: req.Message})
	note(r, "text to %s accepted as %s", number, id)
	writeJSON(w, http.StatusOK, map[string]string{"message_id": id, "status": "accepted"})
	s.later(s.cfg.Timing.Report, func() { s.deliveryReport(number, id) })
}

// deliveryReport tells Kiflet a text arrived, or, when the fault says the
// phone can't be reached, that it didn't.
func (s *Sandbox) deliveryReport(number, id string) {
	f := s.currentFaults()
	status, why, state := "delivered", "", "Delivered"
	if f.isUnreachable(number) {
		status, why = "undelivered", "absent subscriber: the phone is off or out of coverage"
		state = "Not delivered: the phone is off or out of coverage (fault)"
	}
	s.updateText(number, id, func(t *text) { t.State, t.Failed = state, status != "delivered" })
	body := mustJSON(map[string]string{"message_id": id, "status": status, "error": why, "done_at": time.Now().Format(time.RFC3339)})
	c := s.newCall("SMS", "delivery report for "+id+": "+status, s.cfg.AppURL+"/api/v1/sms/status", s.webhookHeader(), body, 4,
		func(a answer, last bool) {
			report := describeAnswer(a)
			if !a.ok() && !last {
				report = "Trying again: " + report
			}
			s.updateText(number, id, func(t *text) { t.Report = "Report: " + report })
		})
	if f.DropReports {
		s.updateText(number, id, func(t *text) { t.Report = "Report held back (fault)" })
		s.hold(c, "delivery reports are dropped (fault)")
		return
	}
	s.deliver(c)
}

// textKiflet sends a text from a phone to Kiflet's short code, as the
// gateway forwards it. It is sent once: Kiflet acts on every text it is
// given, so a gateway resending one would repeat what it says.
func (s *Sandbox) textKiflet(number, body string) (text, answer) {
	id := "MO" + random(upperChars, 14)
	payload := mustJSON(map[string]string{
		"message_id": id, "from": e164(number), "to": s.cfg.SMS.Number, "message": body,
		"received_at": time.Now().Format(time.RFC3339),
	})
	c := s.newCall("SMS", fmt.Sprintf("text from %s: %q", number, clip(body, 40)), s.cfg.AppURL+"/api/v1/sms/inbound", s.webhookHeader(), payload, 1, nil)
	a := s.attempt(s.ctx, c)
	t := text{ID: id, At: time.Now(), From: number, To: s.cfg.SMS.Number, Mine: true, Body: body, State: describeAnswer(a), Failed: !a.ok()}
	if a.ok() {
		var res struct {
			Detail string `json:"detail"`
		}
		if json.Unmarshal(a.Body, &res) == nil && res.Detail != "" {
			t.State += ": " + res.Detail
		}
	}
	s.mu.Lock()
	s.phoneLocked(number).add(t)
	s.mu.Unlock()
	return t, a
}

// webhookHeader is the header of the gateway's calls to Kiflet.
func (s *Sandbox) webhookHeader() http.Header {
	h := jsonHeader()
	h.Set("X-Webhook-Secret", s.cfg.SMS.Secret)
	return h
}

// issueToken makes a token good for life, forgetting those expired.
func (s *Sandbox) issueToken(tokens map[string]time.Time, life time.Duration) string {
	token := random(upperChars+"abcdefghijklmnopqrstuvwxyz", 40)
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for t, until := range tokens {
		if now.After(until) {
			delete(tokens, t)
		}
	}
	tokens[token] = now.Add(life)
	return token
}

// validToken reports whether a token was issued and hasn't expired.
func (s *Sandbox) validToken(tokens map[string]time.Time, token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	until, ok := tokens[token]
	return ok && time.Now().Before(until)
}

// expireTokens ends every token issued, as a gateway may at any time, so
// that Kiflet's next call is refused and it must sign in again.
func (s *Sandbox) expireTokens() {
	s.mu.Lock()
	clear(s.smsTokens)
	clear(s.tbTokens)
	s.mu.Unlock()
}
