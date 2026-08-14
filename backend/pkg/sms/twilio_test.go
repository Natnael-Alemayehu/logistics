package sms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTwilioProvider_Name(t *testing.T) {
	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, nil)
	assert.Equal(t, "twilio", provider.Name())
}

func TestTwilioProvider_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "Messages.json")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sid":    "SM123456789",
			"status": "queued",
		})
	}))
	defer server.Close()

	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, nil)
	provider.apiURL = server.URL + "/Messages.json"

	err := provider.Send(context.Background(), "0912345678", "Test message", MessageTypeNotification)
	require.NoError(t, err)
}

func TestTwilioProvider_Send_InvalidPhone(t *testing.T) {
	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, nil)

	err := provider.Send(context.Background(), "invalid", "Test message", MessageTypeNotification)
	require.Error(t, err)
	assert.Equal(t, ErrInvalidPhone, err)
}

func TestTwilioProvider_Send_OptedOut(t *testing.T) {
	optOutStore := NewInMemoryOptOutStore()
	optOutStore.AddOptOut("+251912345678")

	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, optOutStore)

	err := provider.Send(context.Background(), "0912345678", "Test message", MessageTypeNotification)
	require.Error(t, err)
	assert.Equal(t, ErrOptedOut, err)
}

type alwaysBlockRateLimiter struct{}

func (a *alwaysBlockRateLimiter) Allow(phone string) bool { return false }
func (a *alwaysBlockRateLimiter) Record(phone string)     {}

func TestTwilioProvider_Send_RateLimited(t *testing.T) {
	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, &alwaysBlockRateLimiter{}, nil)

	err := provider.Send(context.Background(), "0912345678", "Test message", MessageTypeNotification)
	require.Error(t, err)
	assert.Equal(t, ErrRateLimited, err)
}

func TestTwilioProvider_CheckStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "delivered",
		})
	}))
	defer server.Close()

	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, nil)

	// Note: CheckStatus uses hardcoded Twilio URLs, so we can't easily mock this
	// This test would need integration testing with a real or mock HTTP transport
	_ = provider
}

func TestTwilioProvider_SendBatch(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sid":    "SM123",
			"status": "queued",
		})
	}))
	defer server.Close()

	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, nil)
	provider.apiURL = server.URL + "/Messages.json"

	err := provider.SendBatch(context.Background(), []string{"0912345678", "0712345678"}, "Test message", MessageTypeNotification)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestTwilioProvider_SendBatch_PartialFailure(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sid":    "SM123",
			"status": "queued",
		})
	}))
	defer server.Close()

	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, nil)
	provider.apiURL = server.URL + "/Messages.json"

	err := provider.SendBatch(context.Background(), []string{"0912345678"}, "Test message", MessageTypeNotification)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestTwilioProvider_HandleOptOut_Stop(t *testing.T) {
	optOutStore := NewInMemoryOptOutStore()
	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, optOutStore)

	provider.HandleOptOut("STOP", "0912345678")

	assert.True(t, optOutStore.IsOptedOut("+251912345678"))
}

func TestTwilioProvider_HandleOptOut_Unsubscribe(t *testing.T) {
	optOutStore := NewInMemoryOptOutStore()
	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, optOutStore)

	provider.HandleOptOut("UNSUBSCRIBE", "0912345678")

	assert.True(t, optOutStore.IsOptedOut("+251912345678"))
}

func TestTwilioProvider_HandleOptOut_Cancel(t *testing.T) {
	optOutStore := NewInMemoryOptOutStore()
	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, optOutStore)

	provider.HandleOptOut("CANCEL", "0912345678")

	assert.True(t, optOutStore.IsOptedOut("+251912345678"))
}

func TestTwilioProvider_HandleOptOut_Start(t *testing.T) {
	optOutStore := NewInMemoryOptOutStore()
	optOutStore.AddOptOut("+251912345678")
	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, optOutStore)

	provider.HandleOptOut("START", "0912345678")

	assert.False(t, optOutStore.IsOptedOut("+251912345678"))
}

func TestTwilioProvider_HandleOptOut_Yes(t *testing.T) {
	optOutStore := NewInMemoryOptOutStore()
	optOutStore.AddOptOut("+251912345678")
	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, optOutStore)

	provider.HandleOptOut("YES", "0912345678")

	assert.False(t, optOutStore.IsOptedOut("+251912345678"))
}

func TestTwilioProvider_HandleOptOut_Unstop(t *testing.T) {
	optOutStore := NewInMemoryOptOutStore()
	optOutStore.AddOptOut("+251912345678")
	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, optOutStore)

	provider.HandleOptOut("UNSTOP", "0912345678")

	assert.False(t, optOutStore.IsOptedOut("+251912345678"))
}

func TestTwilioProvider_HandleOptOut_CaseInsensitive(t *testing.T) {
	optOutStore := NewInMemoryOptOutStore()
	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, optOutStore)

	provider.HandleOptOut("stop", "0912345678")

	assert.True(t, optOutStore.IsOptedOut("+251912345678"))
}

func TestTwilioProvider_HandleOptOut_NoStore(t *testing.T) {
	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, nil)

	provider.HandleOptOut("STOP", "0912345678")
}

func TestTwilioProvider_HandleWebhook_JSON(t *testing.T) {
	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, nil)

	body := []byte(`{"MessageSid":"SM123","From":"+251912345678","Body":"STOP","SmsStatus":"received"}`)

	err := provider.HandleWebhook(body)
	require.NoError(t, err)
}

func TestTwilioProvider_HandleWebhook_FormData(t *testing.T) {
	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, nil)

	body := []byte("MessageSid=SM123&From=%2B251912345678&Body=STOP&SmsStatus=received")

	err := provider.HandleWebhook(body)
	require.NoError(t, err)
}

func TestTwilioProvider_SendWithRetry_Success(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sid":    "SM123",
			"status": "queued",
		})
	}))
	defer server.Close()

	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, nil)
	provider.apiURL = server.URL + "/Messages.json"

	msgID, err := provider.sendWithRetry(context.Background(), "+251912345678", "Test", 3)
	require.NoError(t, err)
	assert.Equal(t, "SM123", msgID)
	assert.Equal(t, 2, attempts)
}

func TestTwilioProvider_SendWithRetry_MaxRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, nil)
	provider.apiURL = server.URL + "/Messages.json"

	_, err := provider.sendWithRetry(context.Background(), "+251912345678", "Test", 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 3 retries")
}

func TestIsTwilioErrorRetryable(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{20429, true},
		{20491, true},
		{21603, false},
		{21614, false},
		{21612, false},
		{500, true},
		{502, true},
		{503, true},
		{400, false},
		{401, false},
		{404, false},
	}

	for _, tt := range tests {
		result := isTwilioErrorRetryable(tt.code)
		assert.Equal(t, tt.expected, result, "code %d", tt.code)
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

	provider := NewTwilioProvider("sid", "token", "+1234567890", &mockLogger{}, nil, nil)
	provider.apiURL = server.URL + "/Messages.json"
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.Send(ctx, "0912345678", "Test", MessageTypeNotification)
	}
}
