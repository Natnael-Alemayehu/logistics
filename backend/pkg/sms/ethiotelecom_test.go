package sms

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEthioTelecomProvider_Name(t *testing.T) {
	provider := NewEthioTelecomProvider("http://api.example.com", "api-key", "SENDER", &mockLogger{})
	assert.Equal(t, "ethiotelecom", provider.Name())
}

func TestEthioTelecomProvider_Send_Success(t *testing.T) {
	authCalled := false
	sendCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "auth/token") {
			authCalled = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":      "test-token",
				"expires_in": 3600,
			})
			return
		}

		if strings.Contains(r.URL.Path, "sms/send") {
			sendCalled = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message_id": "MSG123",
				"status":     "success",
			})
		}
	}))
	defer server.Close()

	provider := NewEthioTelecomProvider(server.URL, "api-key", "SENDER", &mockLogger{})

	err := provider.Send(context.Background(), "0912345678", "Test message", MessageTypeNotification)
	require.NoError(t, err)
	assert.True(t, authCalled)
	assert.True(t, sendCalled)
}

func TestEthioTelecomProvider_Send_InvalidPhone(t *testing.T) {
	provider := NewEthioTelecomProvider("http://api.example.com", "api-key", "SENDER", &mockLogger{})

	err := provider.Send(context.Background(), "invalid", "Test message", MessageTypeNotification)
	require.Error(t, err)
	assert.Equal(t, ErrInvalidPhone, err)
}

func TestEthioTelecomProvider_Send_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "auth/token") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":      "test-token",
				"expires_in": 3600,
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  "Invalid request",
		})
	}))
	defer server.Close()

	provider := NewEthioTelecomProvider(server.URL, "api-key", "SENDER", &mockLogger{})

	err := provider.Send(context.Background(), "0912345678", "Test message", MessageTypeNotification)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error")
}

func TestEthioTelecomProvider_SendBatch_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "auth/token") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":      "test-token",
				"expires_in": 3600,
			})
			return
		}

		if strings.Contains(r.URL.Path, "sms/bulk") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{"to": "+251912345678", "message_id": "MSG1", "status": "success"},
					{"to": "+251712345678", "message_id": "MSG2", "status": "success"},
				},
			})
		}
	}))
	defer server.Close()

	provider := NewEthioTelecomProvider(server.URL, "api-key", "SENDER", &mockLogger{})

	err := provider.SendBatch(context.Background(), []string{"0912345678", "0712345678"}, "Test message", MessageTypeNotification)
	require.NoError(t, err)
}

func TestEthioTelecomProvider_SendBatch_EmptyRecipients(t *testing.T) {
	provider := NewEthioTelecomProvider("http://api.example.com", "api-key", "SENDER", &mockLogger{})

	err := provider.SendBatch(context.Background(), []string{}, "Test message", MessageTypeNotification)
	require.NoError(t, err)
}

func TestEthioTelecomProvider_SendBatch_PartialFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "auth/token") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":      "test-token",
				"expires_in": 3600,
			})
			return
		}

		if strings.Contains(r.URL.Path, "sms/bulk") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{"to": "+251912345678", "message_id": "MSG1", "status": "success"},
					{"to": "+251712345678", "status": "failed", "error": "Network error"},
				},
			})
		}
	}))
	defer server.Close()

	provider := NewEthioTelecomProvider(server.URL, "api-key", "SENDER", &mockLogger{})

	err := provider.SendBatch(context.Background(), []string{"0912345678", "0712345678"}, "Test message", MessageTypeNotification)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "errors")
}

func TestEthioTelecomProvider_CheckStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "auth/token") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":      "test-token",
				"expires_in": 3600,
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "delivered",
		})
	}))
	defer server.Close()

	provider := NewEthioTelecomProvider(server.URL, "api-key", "SENDER", &mockLogger{})

	status, err := provider.CheckStatus(context.Background(), "MSG123")
	require.NoError(t, err)
	assert.Equal(t, "delivered", status)
}

func TestEthioTelecomProvider_TokenManagement(t *testing.T) {
	authCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "auth/token") {
			authCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":      "test-token",
				"expires_in": 3600,
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message_id": "MSG123",
			"status":     "success",
		})
	}))
	defer server.Close()

	provider := NewEthioTelecomProvider(server.URL, "api-key", "SENDER", &mockLogger{})

	_ = provider.Send(context.Background(), "0912345678", "Test 1", MessageTypeNotification)
	_ = provider.Send(context.Background(), "0912345678", "Test 2", MessageTypeNotification)

	assert.Equal(t, 1, authCount)
}

func TestEthioTelecomProvider_TokenExpiry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "auth/token") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":      "test-token",
				"expires_in": -1,
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message_id": "MSG123",
			"status":     "success",
		})
	}))
	defer server.Close()

	provider := NewEthioTelecomProvider(server.URL, "api-key", "SENDER", &mockLogger{})

	err := provider.Send(context.Background(), "0912345678", "Test message", MessageTypeNotification)
	require.NoError(t, err)
}

func TestEthioTelecomProvider_InvalidateToken(t *testing.T) {
	provider := NewEthioTelecomProvider("http://api.example.com", "api-key", "SENDER", &mockLogger{})

	provider.token = "existing-token"
	provider.tokenExpiry = provider.tokenExpiry.Add(1 * time.Hour)

	provider.invalidateToken()

	assert.Empty(t, provider.token)
	assert.True(t, provider.tokenExpiry.IsZero())
}

func TestEthioTelecomProvider_SendWithRetry_Success(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "auth/token") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":      "test-token",
				"expires_in": 3600,
			})
			return
		}

		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message_id": "MSG123",
			"status":     "success",
		})
	}))
	defer server.Close()

	provider := NewEthioTelecomProvider(server.URL, "api-key", "SENDER", &mockLogger{})

	msgID, err := provider.sendWithRetry(context.Background(), "+251912345678", "Test", 3)
	require.NoError(t, err)
	assert.Equal(t, "MSG123", msgID)
	assert.Equal(t, 2, attempts)
}

func TestEthioTelecomProvider_SendWithRetry_MaxRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "auth/token") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":      "test-token",
				"expires_in": 3600,
			})
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	provider := NewEthioTelecomProvider(server.URL, "api-key", "SENDER", &mockLogger{})

	_, err := provider.sendWithRetry(context.Background(), "+251912345678", "Test", 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 3 retries")
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		err      error
		expected bool
	}{
		{nil, false},
		{errors.New("authentication failed"), true},
		{errors.New("token expired"), true},
		{errors.New("other error"), false},
	}

	for _, tt := range tests {
		result := isAuthError(tt.err)
		assert.Equal(t, tt.expected, result)
	}
}

func BenchmarkEthioTelecomProvider_Send(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "auth/token") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"token":      "test-token",
				"expires_in": 3600,
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message_id": "MSG123",
			"status":     "success",
		})
	}))
	defer server.Close()

	provider := NewEthioTelecomProvider(server.URL, "api-key", "SENDER", &mockLogger{})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = provider.Send(ctx, "0912345678", "Test", MessageTypeNotification)
	}
}
