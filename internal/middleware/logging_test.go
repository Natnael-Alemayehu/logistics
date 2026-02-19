package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

func TestLogger_LogsRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))

	req := httptest.NewRequest("GET", "/test/path?query=value", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, "test-request-id"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()

	if !strings.Contains(logOutput, `"method":"GET"`) {
		t.Error("log should contain method")
	}
	if !strings.Contains(logOutput, `"path":"/test/path"`) {
		t.Error("log should contain path")
	}
	if !strings.Contains(logOutput, `"query":"query=value"`) {
		t.Error("log should contain query")
	}
	if !strings.Contains(logOutput, `"request_id":"test-request-id"`) {
		t.Error("log should contain request_id")
	}
	if !strings.Contains(logOutput, `"status":200`) {
		t.Error("log should contain status")
	}
	if !strings.Contains(logOutput, `"message":"request completed"`) {
		t.Error("log should contain message")
	}
}

func TestLogger_CapturesStatusAndDuration(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

	statuses := []int{200, 201, 400, 404, 500}
	for _, status := range statuses {
		buf.Reset()

		handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		logOutput := buf.String()
		statusStr := string(rune('0'+status/100)) + string(rune('0'+(status/10)%10)) + string(rune('0'+status%10))
		if !strings.Contains(logOutput, `"status":`+statusStr) {
			t.Errorf("log should contain status %d", status)
		}
		if !strings.Contains(logOutput, `"duration"`) {
			t.Error("log should contain duration")
		}
	}
}

func TestLogger_CapturesBytesWritten(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

	responseBody := "hello world test response"
	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseBody))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, `"bytes"`) {
		t.Error("log should contain bytes written")
	}
}

func TestLogger_WithContextValues(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = context.WithValue(ctx, UserIDKey, "user-123")
		ctx = context.WithValue(ctx, TenantIDKey, "tenant-456")
		r = r.WithContext(ctx)

		Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})).ServeHTTP(w, r)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, `"user_id":"user-123"`) {
		t.Error("log should contain user_id")
	}
	if !strings.Contains(logOutput, `"tenant_id":"tenant-456"`) {
		t.Error("log should contain tenant_id")
	}
}

func TestLogger_CapturesRemoteAddr(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, `"remote_addr":"192.168.1.1:12345"`) {
		t.Error("log should contain remote_addr")
	}
}

func TestLogger_DifferentMethods(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		buf.Reset()

		handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(method, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		logOutput := buf.String()
		if !strings.Contains(logOutput, `"method":"`+method+`"`) {
			t.Errorf("log should contain method %s", method)
		}
	}
}
