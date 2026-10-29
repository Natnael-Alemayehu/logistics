package web

import (
	"crypto/subtle"
	"encoding/json"
	"mime"
	"net/http"
	"strings"

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
		"max_photos":    3,
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
	if err := s.svc.Auth.Logout(r.Context(), principal(r)); err != nil {
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
	var req service.SyncRequest
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
	secret := s.cfg.SMSInboundSecret
	if secret == "" && !s.mockSMS {
		http.NotFound(w, r)
		return
	}
	if secret != "" {
		got := r.Header.Get("X-Webhook-Secret")
		if got == "" {
			got = r.URL.Query().Get("secret")
		}
		if subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
			writeJSONError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Bad webhook secret.")
			return
		}
	}

	fields := map[string]string{}
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if ct == "application/json" {
		var m map[string]any
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Bad JSON.")
			return
		}
		for k, v := range m {
			if str, ok := v.(string); ok {
				fields[strings.ToLower(k)] = str
			}
		}
	} else {
		if err := r.ParseForm(); err != nil {
			writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Bad form body.")
			return
		}
		for k := range r.PostForm {
			fields[strings.ToLower(k)] = r.PostForm.Get(k)
		}
	}
	pick := func(keys ...string) string {
		for _, k := range keys {
			if v := fields[k]; v != "" {
				return v
			}
		}
		return ""
	}
	from := pick("from", "phone", "msisdn", "sender")
	body := pick("body", "message", "text", "content")
	if from == "" || body == "" {
		writeJSONError(w, http.StatusBadRequest, "VALIDATION_ERROR", "from and body are required.")
		return
	}
	if err := s.svc.SMS.Inbound(r.Context(), from, body); err != nil {
		s.apiError(w, r, err)
		return
	}
	// Replies go out through the outbox, so the provider gets an empty answer.
	if strings.Contains(r.Header.Get("Accept"), "xml") || r.PostForm.Get("AccountSid") != "" {
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><Response></Response>`))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
