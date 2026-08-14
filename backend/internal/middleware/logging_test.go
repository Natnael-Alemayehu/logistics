package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

func TestLogger_LogLevelBasedOnStatus(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		expectedLevel string
	}{
		{"200 OK", 200, `"level":"info"`},
		{"201 Created", 201, `"level":"info"`},
		{"301 Redirect", 301, `"level":"info"`},
		{"400 Bad Request", 400, `"level":"warn"`},
		{"401 Unauthorized", 401, `"level":"warn"`},
		{"404 Not Found", 404, `"level":"warn"`},
		{"500 Internal Error", 500, `"level":"error"`},
		{"502 Bad Gateway", 502, `"level":"error"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := zerolog.New(&buf).Level(zerolog.DebugLevel)

			handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))

			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			logOutput := strings.ToLower(buf.String())
			if !strings.Contains(logOutput, tt.expectedLevel) {
				t.Errorf("expected log level %s for status %d, got: %s", tt.expectedLevel, tt.status, logOutput)
			}
		})
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

func TestLogger_CapturesRequestBodyOnError(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))

	reqBody := `{"email":"test@example.com","password":"secret123"}`
	req := httptest.NewRequest("POST", "/login", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()

	if !strings.Contains(logOutput, `"request_body"`) {
		t.Error("log should contain request_body for error responses")
	}
	if !strings.Contains(logOutput, `\"password\":\"[REDACTED]\"`) {
		t.Error("log should redact password field")
	}
	if !strings.Contains(logOutput, `\"email\":\"test@example.com\"`) {
		t.Error("log should preserve non-sensitive fields")
	}
	if strings.Contains(logOutput, "secret123") {
		t.Error("log should not contain actual password value")
	}
}

func TestLogger_DoesNotCaptureBodyOnSuccess(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	reqBody := `{"email":"test@example.com"}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()

	if strings.Contains(logOutput, `"request_body"`) {
		t.Error("log should not contain request_body for successful responses")
	}
}

func TestLogger_RedactsMultipleSensitiveFields(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))

	reqBody := `{
		"email": "test@example.com",
		"password": "secret",
		"pin": "1234",
		"current_password": "oldpass",
		"new_password": "newpass",
		"token": "abc123",
		"refresh_token": "xyz789"
	}`
	req := httptest.NewRequest("POST", "/auth", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()

	if strings.Contains(logOutput, "secret") || strings.Contains(logOutput, "oldpass") || strings.Contains(logOutput, "newpass") || strings.Contains(logOutput, "abc123") || strings.Contains(logOutput, "xyz789") {
		t.Error("log should not contain any sensitive values")
	}

	if !strings.Contains(logOutput, `\"password\":\"[REDACTED]\"`) {
		t.Error("password should be redacted")
	}
	if !strings.Contains(logOutput, `\"pin\":\"[REDACTED]\"`) {
		t.Error("pin should be redacted")
	}
	if !strings.Contains(logOutput, `\"current_password\":\"[REDACTED]\"`) {
		t.Error("current_password should be redacted")
	}
	if !strings.Contains(logOutput, `\"new_password\":\"[REDACTED]\"`) {
		t.Error("new_password should be redacted")
	}
	if !strings.Contains(logOutput, `\"token\":\"[REDACTED]\"`) {
		t.Error("token should be redacted")
	}
	if !strings.Contains(logOutput, `\"refresh_token\":\"[REDACTED]\"`) {
		t.Error("refresh_token should be redacted")
	}
	if !strings.Contains(logOutput, `\"email\":\"test@example.com\"`) {
		t.Error("email should not be redacted")
	}
}

func TestLogger_HandlesMalformedJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))

	reqBody := `{invalid json`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()

	if !strings.Contains(logOutput, `"request_body":"[binary or malformed data]"`) {
		t.Error("log should indicate malformed data")
	}
}

func TestLogger_IncludesErrorType(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		expectedType string
	}{
		{"400", 400, "Bad Request"},
		{"401", 401, "Unauthorized"},
		{"403", 403, "Forbidden"},
		{"404", 404, "Not Found"},
		{"500", 500, "Internal Server Error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

			handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))

			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			logOutput := buf.String()
			if !strings.Contains(logOutput, `"error_type":"`+tt.expectedType+`"`) {
				t.Errorf("log should contain error_type %s", tt.expectedType)
			}
		})
	}
}

func TestLogger_HandlesEmptyBody(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))

	req := httptest.NewRequest("POST", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()

	if strings.Contains(logOutput, `"request_body"`) {
		t.Error("log should not contain request_body field when body is empty")
	}
}

func TestLogger_HandlesNestedJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))

	reqBody := `{
		"user": {
			"email": "test@example.com",
			"password": "secret",
			"profile": {
				"name": "John",
				"api_key": "key123"
			}
		}
	}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()

	if strings.Contains(logOutput, "secret") || strings.Contains(logOutput, "key123") {
		t.Error("log should not contain nested sensitive values")
	}
	if !strings.Contains(logOutput, `\"password\":\"[REDACTED]\"`) {
		t.Error("nested password should be redacted")
	}
	if !strings.Contains(logOutput, `\"api_key\":\"[REDACTED]\"`) {
		t.Error("nested api_key should be redacted")
	}
	if !strings.Contains(logOutput, `\"name\":\"John\"`) {
		t.Error("nested name should not be redacted")
	}
}

func TestLogger_HandlesArrayJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))

	reqBody := `{
		"users": [
			{"email": "a@example.com", "password": "pass1"},
			{"email": "b@example.com", "password": "pass2"}
		]
	}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()

	if strings.Contains(logOutput, "pass1") || strings.Contains(logOutput, "pass2") {
		t.Error("log should not contain passwords in arrays")
	}
}

func TestRedactSensitiveData(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
		excludes string
	}{
		{
			name:     "simple password",
			input:    `{"password": "secret123"}`,
			contains: `"password":"[REDACTED]"`,
			excludes: "secret123",
		},
		{
			name:     "api_key with underscore",
			input:    `{"api_key": "key123"}`,
			contains: `"api_key":"[REDACTED]"`,
			excludes: "key123",
		},
		{
			name:     "non-sensitive field",
			input:    `{"username": "john"}`,
			contains: `"username":"john"`,
			excludes: "[REDACTED]",
		},
		{
			name:     "empty object",
			input:    `{}`,
			contains: `{}`,
			excludes: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactSensitiveData([]byte(tt.input))
			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("expected result to contain %s, got %s", tt.contains, result)
			}
			if tt.excludes != "" && strings.Contains(result, tt.excludes) {
				t.Errorf("expected result to NOT contain %s, got %s", tt.excludes, result)
			}
		})
	}
}

func TestStatusLevel(t *testing.T) {
	tests := []struct {
		status    int
		wantLevel zerolog.Level
	}{
		{200, zerolog.InfoLevel},
		{201, zerolog.InfoLevel},
		{301, zerolog.InfoLevel},
		{304, zerolog.InfoLevel},
		{400, zerolog.WarnLevel},
		{401, zerolog.WarnLevel},
		{403, zerolog.WarnLevel},
		{404, zerolog.WarnLevel},
		{422, zerolog.WarnLevel},
		{500, zerolog.ErrorLevel},
		{502, zerolog.ErrorLevel},
		{503, zerolog.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			got := statusLevel(tt.status)
			if got != tt.wantLevel {
				t.Errorf("statusLevel(%d) = %v, want %v", tt.status, got, tt.wantLevel)
			}
		})
	}
}

func TestIsSensitiveField(t *testing.T) {
	sensitive := []string{
		"password",
		"PASSWORD",
		"Password",
		"pin",
		"token",
		"refresh_token",
		"access_token",
		"api_key",
		"apiKey",
		"API_KEY",
		"current_password",
		"new_password",
		"secret",
		"client_secret",
	}

	for _, field := range sensitive {
		if !isSensitiveField(strings.ToLower(field)) {
			t.Errorf("expected %s to be sensitive", field)
		}
	}

	nonSensitive := []string{
		"email",
		"username",
		"name",
		"phone",
		"id",
		"created_at",
	}

	for _, field := range nonSensitive {
		if isSensitiveField(field) {
			t.Errorf("expected %s to NOT be sensitive", field)
		}
	}
}

func TestLogger_PreserveBodyForHandler(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

	var receivedBody string
	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		receivedBody = string(bodyBytes)
		w.WriteHeader(http.StatusBadRequest)
	}))

	reqBody := `{"email":"test@example.com"}`
	req := httptest.NewRequest("POST", "/test", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if receivedBody != reqBody {
		t.Errorf("handler received different body: got %s, want %s", receivedBody, reqBody)
	}
}

func TestLogger_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.InfoLevel)

	handler := Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, "req-123"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()

	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(logOutput), &logEntry); err != nil {
		t.Fatalf("log output should be valid JSON: %v", err)
	}

	requiredFields := []string{"level", "request_id", "method", "path", "status", "duration", "message"}
	for _, field := range requiredFields {
		if _, exists := logEntry[field]; !exists {
			t.Errorf("log entry missing required field: %s", field)
		}
	}
}
