package sms

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTwilioProvider_Name(t *testing.T) {
	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{})
	assert.Equal(t, "twilio", provider.Name())
}

func TestTwilioProvider_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "Messages.json")
		user, pass, _ := r.BasicAuth()
		assert.Equal(t, "sid", user)
		assert.Equal(t, "token", pass)
		assert.Equal(t, "+251912345678", r.FormValue("To"))
		assert.Equal(t, "+1234567890", r.FormValue("From"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sid":    "SM123456789",
			"status": "queued",
		})
	}))
	defer server.Close()

	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{})
	provider.apiURL = server.URL + "/Messages.json"

	id, err := provider.Send(context.Background(), "0912345678", "Test message", MessageTypeNotification)
	require.NoError(t, err)
	assert.Equal(t, "SM123456789", id)
}

func TestTwilioProvider_Send_InvalidPhone(t *testing.T) {
	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{})

	_, err := provider.Send(context.Background(), "invalid", "Test message", MessageTypeNotification)
	assert.ErrorIs(t, err, ErrInvalidPhone)
	assert.ErrorIs(t, err, ErrRejected)
}

func TestTwilioProvider_Send_ErrorClasses(t *testing.T) {
	for status, rejected := range map[int]bool{
		http.StatusBadRequest:          true,
		http.StatusNotFound:            true,
		http.StatusUnauthorized:        false,
		http.StatusTooManyRequests:     false,
		http.StatusInternalServerError: false,
		http.StatusServiceUnavailable:  false,
	} {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]interface{}{"code": 21211, "message": "Invalid 'To' Phone Number"})
		}))
		provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{})
		provider.apiURL = server.URL + "/Messages.json"

		_, err := provider.Send(context.Background(), "0912345678", "Test", MessageTypeNotification)
		server.Close()
		require.Error(t, err, "status %d", status)
		assert.Equal(t, rejected, errors.Is(err, ErrRejected), "status %d: %v", status, err)
		assert.Equal(t, 1, attempts, "status %d: one attempt per send", status)
	}
}

func BenchmarkTwilioProvider_Send(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sid":    "SM123",
			"status": "queued",
		})
	}))
	defer server.Close()

	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{})
	provider.apiURL = server.URL + "/Messages.json"
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = provider.Send(ctx, "0912345678", "Test", MessageTypeNotification)
	}
}
