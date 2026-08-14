package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

var sensitiveFields = []string{
	"password",
	"pin",
	"token",
	"secret",
	"credential",
	"refresh_token",
	"access_token",
	"api_key",
	"apikey",
	"authorization",
	"current_password",
	"new_password",
}

type responseCapture struct {
	http.ResponseWriter
	statusCode int
	bytes      int
}

func (rc *responseCapture) WriteHeader(code int) {
	rc.statusCode = code
	rc.ResponseWriter.WriteHeader(code)
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	if rc.statusCode == 0 {
		rc.statusCode = http.StatusOK
	}
	n, err := rc.ResponseWriter.Write(b)
	rc.bytes += n
	return n, err
}

func redactSensitiveData(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return "[binary or malformed data]"
	}

	redacted := redactValue(v)
	result, err := json.Marshal(redacted)
	if err != nil {
		return "[redaction error]"
	}
	return string(result)
}

func redactValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, v := range val {
			lowerKey := strings.ToLower(k)
			if isSensitiveField(lowerKey) {
				result[k] = "[REDACTED]"
			} else {
				result[k] = redactValue(v)
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = redactValue(item)
		}
		return result
	default:
		return v
	}
}

func isSensitiveField(key string) bool {
	for _, field := range sensitiveFields {
		if strings.Contains(key, field) {
			return true
		}
	}
	return false
}

func statusLevel(status int) zerolog.Level {
	switch {
	case status >= 500:
		return zerolog.ErrorLevel
	case status >= 400:
		return zerolog.WarnLevel
	default:
		return zerolog.InfoLevel
	}
}

func Logger(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			var requestBody []byte

			if r.Body != nil && r.Body != http.NoBody {
				var readErr error
				requestBody, readErr = io.ReadAll(r.Body)
				if readErr != nil {
					logger.Warn().Err(readErr).Msg("failed to read request body for logging")
				}
				r.Body = io.NopCloser(bytes.NewReader(requestBody))
			}

			rc := &responseCapture{ResponseWriter: w}

			var finalCtx context.Context

			// Wrap the handler to capture context after middleware runs
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				finalCtx = r.Context()
				next.ServeHTTP(w, r)
			})

			defer func() {
				duration := time.Since(start)
				status := rc.statusCode
				if status == 0 {
					status = http.StatusOK
				}

				level := statusLevel(status)
				event := logger.WithLevel(level)

				ctx := finalCtx
				if ctx == nil {
					ctx = r.Context()
				}

				event.
					Str("request_id", middleware.GetReqID(ctx)).
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Str("query", r.URL.RawQuery).
					Int("status", status).
					Int("bytes", rc.bytes).
					Dur("duration", duration).
					Str("remote_addr", r.RemoteAddr).
					Str("tenant_id", GetTenantID(ctx)).
					Str("user_id", GetUserID(ctx))

				if status >= 400 && len(requestBody) > 0 {
					event.Str("request_body", redactSensitiveData(requestBody))
				}

				if status >= 400 {
					event.Str("error_type", http.StatusText(status))
				}

				event.Msg("request completed")
			}()

			handler.ServeHTTP(rc, r)
		})
	}
}
