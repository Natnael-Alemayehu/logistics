package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_Health(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var response HealthResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response.Status)
	assert.Equal(t, "1.0.0", response.Version)
	assert.Equal(t, "connected", response.Database)
	assert.NotEmpty(t, response.Timestamp)
}

func TestHealthResponse_Fields(t *testing.T) {
	response := HealthResponse{
		Status:    "healthy",
		Version:   "1.0.0",
		Database:  "connected",
		Timestamp: "2024-01-01T00:00:00Z",
	}

	assert.Equal(t, "healthy", response.Status)
	assert.Equal(t, "1.0.0", response.Version)
	assert.Equal(t, "connected", response.Database)
	assert.NotEmpty(t, response.Timestamp)
}

func TestGetIPAddress(t *testing.T) {
	t.Run("returns X-Forwarded-For when present", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.1")
		req.RemoteAddr = "10.0.0.1:12345"

		ip := getIPAddress(req)
		assert.Equal(t, "192.168.1.1", ip)
	})

	t.Run("returns RemoteAddr when X-Forwarded-For not present", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:12345"

		ip := getIPAddress(req)
		assert.Equal(t, "10.0.0.1:12345", ip)
	})

	t.Run("handles multiple X-Forwarded-For values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")
		req.RemoteAddr = "172.16.0.1:12345"

		ip := getIPAddress(req)
		assert.Contains(t, ip, "192.168.1.1")
	})
}

func TestHandler_New(t *testing.T) {
	h := New(nil, nil, nil, nil, nil, nil, nil, nil)
	require.NotNil(t, h)
}

func TestHandler_Health_JSONFormat(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.Health(rec, req)

	body := rec.Body.String()
	assert.True(t, strings.Contains(body, `"status"`))
	assert.True(t, strings.Contains(body, `"version"`))
	assert.True(t, strings.Contains(body, `"database"`))
	assert.True(t, strings.Contains(body, `"timestamp"`))
}

func TestHandler_Health_MethodNotAllowed(t *testing.T) {
	h := &Handler{}

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/health", nil)
			rec := httptest.NewRecorder()

			h.Health(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func BenchmarkHandler_Health(b *testing.B) {
	h := &Handler{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		h.Health(rec, req)
	}
}
