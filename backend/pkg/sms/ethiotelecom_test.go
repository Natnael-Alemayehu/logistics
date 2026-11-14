package sms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGateway answers the Ethio telecom adapter's calls. send decides each
// /sms/send answer; by default the message is accepted as MSG<n>.
type fakeGateway struct {
	mu        sync.Mutex
	expiresIn int
	logins    int
	sends     int
	tokens    []string
	requests  []ethioTelecomSMSRequest
	send      func(n int) (int, any)
}

func (g *fakeGateway) serve(tb testing.TB) *httptest.Server {
	tb.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		defer g.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/token":
			var req ethioTelecomAuthRequest
			json.NewDecoder(r.Body).Decode(&req)
			if req.APIKey != "api-key" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			g.logins++
			json.NewEncoder(w).Encode(map[string]any{"token": fmt.Sprintf("token-%d", g.logins), "expires_in": g.expiresIn})
		case "/sms/send":
			g.sends++
			g.tokens = append(g.tokens, r.Header.Get("Authorization"))
			var req ethioTelecomSMSRequest
			json.NewDecoder(r.Body).Decode(&req)
			g.requests = append(g.requests, req)
			status, body := http.StatusOK, any(map[string]any{"message_id": fmt.Sprintf("MSG%d", g.sends), "status": "accepted"})
			if g.send != nil {
				status, body = g.send(g.sends)
			}
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(body)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	tb.Cleanup(srv.Close)
	return srv
}

func TestEthioTelecomProvider_Name(t *testing.T) {
	provider := NewEthioTelecomProvider("http://api.example.com", "api-key", "SENDER", &mockLogger{})
	assert.Equal(t, "ethiotelecom", provider.Name())
}

func TestEthioTelecomProvider_Send_Success(t *testing.T) {
	g := &fakeGateway{expiresIn: 3600}
	provider := NewEthioTelecomProvider(g.serve(t).URL+"/", "api-key", "SENDER", &mockLogger{})

	id, err := provider.Send(context.Background(), "0912345678", "Test message", MessageTypeNotification)
	require.NoError(t, err)
	assert.Equal(t, "MSG1", id)
	assert.Equal(t, []ethioTelecomSMSRequest{{To: "+251912345678", From: "SENDER", Message: "Test message"}}, g.requests)
	assert.Equal(t, []string{"Bearer token-1"}, g.tokens)
}

func TestEthioTelecomProvider_Send_InvalidPhone(t *testing.T) {
	g := &fakeGateway{expiresIn: 3600}
	provider := NewEthioTelecomProvider(g.serve(t).URL, "api-key", "SENDER", &mockLogger{})

	_, err := provider.Send(context.Background(), "invalid", "Test message", MessageTypeNotification)
	assert.ErrorIs(t, err, ErrInvalidPhone)
	assert.ErrorIs(t, err, ErrRejected)
	assert.Zero(t, g.logins+g.sends, "nothing is sent for a number that can't be one")
}

func TestEthioTelecomProvider_ReusesItsToken(t *testing.T) {
	g := &fakeGateway{expiresIn: 3600}
	provider := NewEthioTelecomProvider(g.serve(t).URL, "api-key", "SENDER", &mockLogger{})

	for _, body := range []string{"Test 1", "Test 2"} {
		_, err := provider.Send(context.Background(), "0912345678", body, MessageTypeNotification)
		require.NoError(t, err)
	}
	assert.Equal(t, 1, g.logins)

	// A token about to expire is replaced before it is used.
	g.expiresIn = 30
	provider.token = ""
	for _, body := range []string{"Test 3", "Test 4"} {
		_, err := provider.Send(context.Background(), "0912345678", body, MessageTypeNotification)
		require.NoError(t, err)
	}
	assert.Equal(t, 3, g.logins)
}

func TestEthioTelecomProvider_SignsInAgainWhenTheTokenIsRefused(t *testing.T) {
	g := &fakeGateway{expiresIn: 3600}
	g.send = func(n int) (int, any) {
		if n == 1 {
			return http.StatusUnauthorized, map[string]any{"error": "token expired"}
		}
		return http.StatusOK, map[string]any{"message_id": "MSG-RETRY", "status": "queued"}
	}
	provider := NewEthioTelecomProvider(g.serve(t).URL, "api-key", "SENDER", &mockLogger{})

	id, err := provider.Send(context.Background(), "0912345678", "Test", MessageTypeNotification)
	require.NoError(t, err)
	assert.Equal(t, "MSG-RETRY", id)
	assert.Equal(t, []string{"Bearer token-1", "Bearer token-2"}, g.tokens)

	// Refused again, the message waits for the outbox to try later.
	g.send = func(int) (int, any) { return http.StatusUnauthorized, map[string]any{} }
	_, err = provider.Send(context.Background(), "0912345678", "Test", MessageTypeNotification)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrRejected)
	assert.Equal(t, 4, g.sends, "one retry, not more")
}

func TestEthioTelecomProvider_RefusedKey(t *testing.T) {
	g := &fakeGateway{expiresIn: 3600}
	provider := NewEthioTelecomProvider(g.serve(t).URL, "wrong-key", "SENDER", &mockLogger{})

	_, err := provider.Send(context.Background(), "0912345678", "Test", MessageTypeNotification)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrRejected, "a deployment's wrong key mustn't fail the messages for good")
	assert.Zero(t, g.sends)
}

func TestEthioTelecomProvider_ErrorClasses(t *testing.T) {
	for name, c := range map[string]struct {
		status   int
		body     any
		rejected bool
	}{
		"bad request":     {http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid destination"}, true},
		"forbidden":       {http.StatusForbidden, map[string]any{"error": "number barred"}, true},
		"rejected answer": {http.StatusOK, map[string]any{"status": "rejected", "error": "not a mobile number"}, true},
		"timeout":         {http.StatusRequestTimeout, map[string]any{}, false},
		"too many":        {http.StatusTooManyRequests, map[string]any{}, false},
		"server error":    {http.StatusInternalServerError, map[string]any{}, false},
		"unavailable":     {http.StatusServiceUnavailable, map[string]any{}, false},
		"unknown status":  {http.StatusOK, map[string]any{"status": "failed", "error": "try later"}, false},
		"unreadable":      {http.StatusOK, "<html>", false},
	} {
		t.Run(name, func(t *testing.T) {
			g := &fakeGateway{expiresIn: 3600, send: func(int) (int, any) { return c.status, c.body }}
			provider := NewEthioTelecomProvider(g.serve(t).URL, "api-key", "SENDER", &mockLogger{})

			_, err := provider.Send(context.Background(), "0912345678", "Test", MessageTypeNotification)
			require.Error(t, err)
			assert.Equal(t, c.rejected, errors.Is(err, ErrRejected), "%v", err)
			assert.Equal(t, 1, g.sends, "one attempt per send; retries are the outbox's")
		})
	}
}

func BenchmarkEthioTelecomProvider_Send(b *testing.B) {
	g := &fakeGateway{expiresIn: 3600}
	provider := NewEthioTelecomProvider(g.serve(b).URL, "api-key", "SENDER", &mockLogger{})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = provider.Send(ctx, "0912345678", "Test", MessageTypeNotification)
	}
}
