package web

import (
	"crypto/subtle"
	"encoding/json"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/model"
	"github.com/natnael-alemayehu/logistics/internal/service"
)

type codeLabel struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

func reasonList(codes []string) []codeLabel {
	out := make([]codeLabel, 0, len(codes))
	for _, c := range codes {
		out = append(out, codeLabel{c, model.ReasonLabel(c)})
	}
	return out
}

// apiDriverConfig is what the app needs to work offline: the status rules,
// reason lists and labels. It mirrors the server's rules so the app never
// offers a move the server would refuse.
func (s *Server) apiDriverConfig(w http.ResponseWriter, r *http.Request) {
	transitions := map[string][]string{}
	statuses := make([]codeLabel, 0, len(model.AllStatuses))
	for _, st := range model.AllStatuses {
		statuses = append(statuses, codeLabel{st, model.StatusLabel(st)})
		next := model.AllowedTransitions(st, model.ActorDriver)
		if next == nil {
			next = []string{}
		}
		transitions[st] = next
	}
	w.Header().Set("Cache-Control", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{
		"app_name":      s.cfg.AppName,
		"version":       s.version,
		"sms_number":    s.cfg.SMSNumber,
		"statuses":      statuses,
		"transitions":   transitions,
		"pod_from":      append(append([]string{}, model.OnRoadStatuses...), model.StatusAssigned),
		"delay_reasons": reasonList(model.DelayReasons),
		"issue_reasons": reasonList(model.IssueReasons),
		// Asked for when a delivery is recorded away from the destination.
		"location_reasons": reasonList(model.LocationReasons),
		"max_photos":       3,
		"tile_url":         s.cfg.MapTileURL,
		"tile_credit":      s.cfg.MapTileAttribution,
	})
}

type driverLoginRequest struct {
	Phone    string `json:"phone"`
	PIN      string `json:"pin"`
	DeviceID string `json:"device_id"`
}

func (s *Server) loginResponse(w http.ResponseWriter, res *service.LoginResult) {
	writeJSON(w, http.StatusOK, map[string]any{
		"token":           res.Token,
		"expires_at":      res.ExpiresAt,
		"must_change_pin": res.Principal.MustChangeCredential,
		"driver": map[string]string{
			"id": res.Principal.UserID.String(), "name": res.Principal.Name,
			"phone": res.Principal.Phone, "company": res.Principal.TenantName,
		},
	})
}

func (s *Server) apiDriverLogin(w http.ResponseWriter, r *http.Request) {
	var req driverLoginRequest
	if err := decodeJSON(r, &req); err != nil {
		s.apiError(w, r, err)
		return
	}
	ci := clientInfo(r)
	ci.DeviceID = req.DeviceID
	res, err := s.svc.Auth.DriverLogin(r.Context(), req.Phone, req.PIN, ci)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	s.loginResponse(w, res)
}

func (s *Server) apiDriverLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.Auth.Logout(r.Context(), principal(r), clientInfo(r)); err != nil {
		s.apiError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) apiPINResetRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.apiError(w, r, err)
		return
	}
	if err := s.svc.Auth.RequestPINReset(r.Context(), req.Phone); err != nil {
		s.apiError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "If this number belongs to a driver, a code is on its way by SMS.",
	})
}

func (s *Server) apiPINResetConfirm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone    string `json:"phone"`
		Code     string `json:"code"`
		PIN      string `json:"pin"`
		DeviceID string `json:"device_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.apiError(w, r, err)
		return
	}
	ci := clientInfo(r)
	ci.DeviceID = req.DeviceID
	res, err := s.svc.Auth.ConfirmPINReset(r.Context(), req.Phone, req.Code, req.PIN, ci)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	s.loginResponse(w, res)
}

func (s *Server) apiChangePIN(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Current string `json:"current_pin"`
		New     string `json:"new_pin"`
	}
	if err := decodeJSON(r, &req); err != nil {
		s.apiError(w, r, err)
		return
	}
	if err := s.svc.Auth.ChangePIN(r.Context(), principal(r), req.Current, req.New, clientInfo(r)); err != nil {
		s.apiError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) apiSync(w http.ResponseWriter, r *http.Request) {
	// Before the body is read: a body with photos can take minutes to
	// upload, which mustn't count as the phone's clock being slow.
	req := service.SyncRequest{ReceivedAt: time.Now()}
	if err := decodeJSON(r, &req); err != nil {
		s.apiError(w, r, err)
		return
	}
	ci := clientInfo(r)
	ci.DeviceID = req.DeviceID
	resp, err := s.svc.Sync.Sync(r.Context(), principal(r), req, ci)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// apiSMSInbound receives messages from the SMS provider. Providers differ in
// field names, so the common ones are all accepted. The shared secret in the
// URL or a header is the authentication; without one configured the endpoint
// only works with the mock provider.
func (s *Server) apiSMSInbound(w http.ResponseWriter, r *http.Request) {
	if !s.smsWebhook(w, r) {
		return
	}
	fields, ok := webhookFields(w, r)
	if !ok {
		return
	}
	from := pick(fields, "from", "phone", "msisdn", "sender")
	body := pick(fields, "body", "message", "text", "content")
	if from == "" || body == "" {
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "from and body are required.")
		return
	}
	res, err := s.svc.SMS.Inbound(r.Context(), service.InboundSMS{
		From: from, Body: body, ProviderMessageID: pick(fields, "message_id", "messageId", "MessageSid", "id"),
	})
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	// Replies go out through the outbox, so the provider gets an empty answer.
	if strings.Contains(r.Header.Get("Accept"), "xml") || r.PostForm.Get("AccountSid") != "" {
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Response></Response>`))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "outcome": res.Outcome, "detail": res.Detail, "reply": res.Reply})
}

// apiSMSStatus receives the SMS provider's delivery reports: message_id (as
// Send returned it), status (delivered, undelivered, failed, rejected or
// expired), error, and done_at in RFC 3339, as JSON or a form.
func (s *Server) apiSMSStatus(w http.ResponseWriter, r *http.Request) {
	if !s.smsWebhook(w, r) {
		return
	}
	fields, ok := webhookFields(w, r)
	if !ok {
		return
	}
	id := pick(fields, "message_id", "messageid", "messagesid", "id")
	status := pick(fields, "status", "messagestatus")
	if id == "" || status == "" {
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "message_id and status are required.")
		return
	}
	at, _ := time.Parse(time.RFC3339, pick(fields, "done_at", "delivered_at", "timestamp"))
	updated, err := s.svc.SMS.DeliveryReport(r.Context(), id, status, pick(fields, "error", "error_message", "reason"), at)
	if err != nil {
		s.apiError(w, r, err)
		return
	}
	// A report on a text this platform didn't send is answered 200 too, or
	// the provider would keep sending it.
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updated": updated})
}

// smsWebhook checks a call from the SMS provider: the shared secret, in the
// X-Webhook-Secret header or a secret parameter. With no secret set, the
// webhooks are open only while SMS is a stand-in. A refused call is
// answered.
func (s *Server) smsWebhook(w http.ResponseWriter, r *http.Request) bool {
	secret := s.cfg.SMSInboundSecret
	if secret == "" && !s.mockSMS {
		http.NotFound(w, r)
		return false
	}
	if secret != "" {
		got := r.Header.Get("X-Webhook-Secret")
		if got == "" {
			got = r.URL.Query().Get("secret")
		}
		if subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
			writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Bad webhook secret.")
			return false
		}
	}
	return true
}

// webhookFields reads a provider's JSON or form body as lower-case names
// and their text, answering the call when it can't be read.
func webhookFields(w http.ResponseWriter, r *http.Request) (map[string]string, bool) {
	fields := map[string]string{}
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if ct == "application/json" {
		var m map[string]any
		dec := json.NewDecoder(r.Body)
		dec.UseNumber()
		if err := dec.Decode(&m); err != nil {
			writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Bad JSON.")
			return nil, false
		}
		for k, v := range m {
			switch v := v.(type) {
			case string:
				fields[strings.ToLower(k)] = v
			case json.Number:
				fields[strings.ToLower(k)] = v.String()
			}
		}
		return fields, true
	}
	if err := r.ParseForm(); err != nil {
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Bad form body.")
		return nil, false
	}
	for k := range r.PostForm {
		fields[strings.ToLower(k)] = r.PostForm.Get(k)
	}
	return fields, true
}

// pick is the first of the named fields that is set.
func pick(fields map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := fields[k]; v != "" {
			return v
		}
	}
	return ""
}
