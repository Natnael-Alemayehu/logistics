//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"
)

// TestDriverPINReset: a driver who forgot their PIN gets a code by SMS and
// signs in with a new PIN; the code works once.
func TestDriverPINReset(t *testing.T) {
	e := newEnv(t)
	office := e.company("Reset Haulage")
	_, phone, pin := office.addDriver("Forgetful Driver")
	e.signIn(phone, pin)

	if code := e.api("POST", "/api/v1/driver/pin-reset/request", "", map[string]string{"phone": phone}, nil); code != http.StatusOK {
		t.Fatalf("request: status %d", code)
	}
	code := e.code(phone)

	confirm := map[string]string{"phone": phone, "code": code, "pin": "7391", "device_id": "new-phone"}
	var login struct {
		Token string `json:"token"`
	}
	if status := e.api("POST", "/api/v1/driver/pin-reset/confirm", "", confirm, &login); status != http.StatusOK || login.Token == "" {
		t.Fatalf("confirm: status %d", status)
	}
	if status := e.api("POST", "/api/v1/driver/login", "", map[string]string{"phone": phone, "pin": "7391", "device_id": "new-phone"}, nil); status != http.StatusOK {
		t.Errorf("sign in with the new PIN: status %d", status)
	}
	if status := e.api("POST", "/api/v1/driver/pin-reset/confirm", "", confirm, nil); status == http.StatusOK {
		t.Error("a used code worked a second time")
	}

	// The SMS log keeps the message but not the code.
	if body := scalar[string](t, `SELECT body FROM sms_messages WHERE phone = $1 AND sensitive ORDER BY created_at DESC LIMIT 1`, phone); strings.Contains(body, code) {
		t.Error("the code is still readable in the SMS log after sending")
	}
}
