package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/natnael-alemayehu/logistics/pkg/redis"
)

func setupTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	client, err := redis.NewClient(redis.Config{
		Addr: mr.Addr(),
	})
	if err != nil {
		mr.Close()
		t.Fatalf("failed to connect to redis: %v", err)
	}

	return client, mr
}

func TestRateLimit_AllowsUnderLimit(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	limiter := NewRateLimiter(client, 5, 10, 3)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()

		limiter.RateLimit()(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected status %d, got %d", i+1, http.StatusOK, rec.Code)
		}
	}
}

func TestRateLimit_BlocksOverLimit(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	limit := 3
	limiter := NewRateLimiter(client, limit, 10, 2)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < limit; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.2:12345"
		rec := httptest.NewRecorder()
		limiter.RateLimit()(handler).ServeHTTP(rec, req)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.2:12345"
	rec := httptest.NewRecorder()
	limiter.RateLimit()(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
}

func TestRateLimit_SetsCorrectHeaders(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	limit := 5
	limiter := NewRateLimiter(client, limit, 10, 2)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.3:12345"
	rec := httptest.NewRecorder()
	limiter.RateLimit()(handler).ServeHTTP(rec, req)

	limitHeader := rec.Header().Get("X-RateLimit-Limit")
	if limitHeader != strconv.Itoa(limit) {
		t.Errorf("X-RateLimit-Limit: expected %d, got %s", limit, limitHeader)
	}

	remainingHeader := rec.Header().Get("X-RateLimit-Remaining")
	remaining, err := strconv.Atoi(remainingHeader)
	if err != nil {
		t.Errorf("X-RateLimit-Remaining: failed to parse: %v", err)
	}
	if remaining != limit-1 {
		t.Errorf("X-RateLimit-Remaining: expected %d, got %d", limit-1, remaining)
	}

	resetHeader := rec.Header().Get("X-RateLimit-Reset")
	if resetHeader == "" {
		t.Error("X-RateLimit-Reset: expected non-empty value")
	}
}

func TestRateLimit_DifferentIPsHaveSeparateLimits(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	limit := 2
	limiter := NewRateLimiter(client, limit, 10, 2)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < limit; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.10:12345"
		rec := httptest.NewRecorder()
		limiter.RateLimit()(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("IP1 request %d: expected status %d, got %d", i+1, http.StatusOK, rec.Code)
		}
	}

	for i := 0; i < limit; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.20:12345"
		rec := httptest.NewRecorder()
		limiter.RateLimit()(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("IP2 request %d: expected status %d, got %d", i+1, http.StatusOK, rec.Code)
		}
	}
}

func TestRateLimitUser_UsesUserContext(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	userLimit := 5
	limiter := NewRateLimiter(client, 10, userLimit, 2)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < userLimit; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		ctx := context.WithValue(req.Context(), UserIDKey, "user-123")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		limiter.RateLimitUser()(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected status %d, got %d", i+1, http.StatusOK, rec.Code)
		}
	}

	req := httptest.NewRequest("GET", "/", nil)
	ctx := context.WithValue(req.Context(), UserIDKey, "user-123")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	limiter.RateLimitUser()(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
}

func TestRateLimitUser_PassesThroughWhenNoUser(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	limiter := NewRateLimiter(client, 10, 1, 2)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	limiter.RateLimitUser()(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestRateLimitAuth_UsesStricterLimit(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	authLimit := 2
	limiter := NewRateLimiter(client, 10, 10, authLimit)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < authLimit; i++ {
		req := httptest.NewRequest("POST", "/auth/login", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		rec := httptest.NewRecorder()
		limiter.RateLimitAuth()(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected status %d, got %d", i+1, http.StatusOK, rec.Code)
		}
	}

	req := httptest.NewRequest("POST", "/auth/login", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()
	limiter.RateLimitAuth()(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
}

func TestRateLimit_FailOpenWhenRedisDown(t *testing.T) {
	client, mr := setupTestRedis(t)
	mr.Close()

	limiter := NewRateLimiter(client, 1, 10, 2)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.200:12345"
		rec := httptest.NewRecorder()
		limiter.RateLimit()(handler).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d with Redis down: expected status %d, got %d", i+1, http.StatusOK, rec.Code)
		}
	}
}

func TestRateLimit_WindowExpires(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	limit := 2
	limiter := NewRateLimiter(client, limit, 10, 2)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < limit; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.50:12345"
		rec := httptest.NewRecorder()
		limiter.RateLimit()(handler).ServeHTTP(rec, req)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.50:12345"
	rec := httptest.NewRecorder()
	limiter.RateLimit()(handler).ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}

	mr.FastForward(time.Minute)

	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.50:12345"
	rec = httptest.NewRecorder()
	limiter.RateLimit()(handler).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("after window expiry: expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestGetIP_XForwardedFor(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		xri        string
		remoteAddr string
		expectedIP string
	}{
		{
			name:       "X-Forwarded-For single IP",
			xff:        "192.168.1.1",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For multiple IPs",
			xff:        "192.168.1.1, 10.0.0.1, 172.16.0.1",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.1",
		},
		{
			name:       "X-Real-IP when no X-Forwarded-For",
			xri:        "192.168.1.2",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.2",
		},
		{
			name:       "X-Forwarded-For takes precedence over X-Real-IP",
			xff:        "192.168.1.3",
			xri:        "192.168.1.4",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.3",
		},
		{
			name:       "RemoteAddr when no headers",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "10.0.0.1",
		},
		{
			name:       "RemoteAddr without port",
			remoteAddr: "10.0.0.1",
			expectedIP: "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}
			req.RemoteAddr = tt.remoteAddr

			ip := getIP(req)
			if ip != tt.expectedIP {
				t.Errorf("expected IP %q, got %q", tt.expectedIP, ip)
			}
		})
	}
}
