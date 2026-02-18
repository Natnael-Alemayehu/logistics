//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/middleware"
	redismodule "github.com/natnael-alemayehu/logistics/pkg/redis"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestRateLimiterIntegration(t *testing.T) {
	ctx := context.Background()

	redisContainer, err := rediscontainer.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}
	defer redisContainer.Terminate(ctx)

	host, err := redisContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get redis host: %v", err)
	}
	port, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("failed to get redis port: %v", err)
	}

	client, err := redismodule.NewClient(redismodule.Config{
		Addr: fmt.Sprintf("%s:%s", host, port.Port()),
	})
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}
	defer client.Close()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	t.Run("AllowsRequestsUnderLimit", func(t *testing.T) {
		limiter := middleware.NewRateLimiter(client, 5, 10, 3)
		rateLimitedHandler := limiter.RateLimit()(handler)

		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "192.168.1.100:1234"
			rec := httptest.NewRecorder()

			rateLimitedHandler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("request %d: expected status 200, got %d", i+1, rec.Code)
			}
		}
	})

	t.Run("BlocksRequestsOverLimit", func(t *testing.T) {
		limiter := middleware.NewRateLimiter(client, 3, 10, 3)
		rateLimitedHandler := limiter.RateLimit()(handler)

		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "192.168.1.101:1234"
			rec := httptest.NewRecorder()
			rateLimitedHandler.ServeHTTP(rec, req)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.101:1234"
		rec := httptest.NewRecorder()
		rateLimitedHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("expected status 429, got %d", rec.Code)
		}
	})

	t.Run("SetsRateLimitHeaders", func(t *testing.T) {
		limiter := middleware.NewRateLimiter(client, 10, 20, 5)
		rateLimitedHandler := limiter.RateLimit()(handler)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.102:1234"
		rec := httptest.NewRecorder()

		rateLimitedHandler.ServeHTTP(rec, req)

		limit := rec.Header().Get("X-RateLimit-Limit")
		if limit != "10" {
			t.Errorf("expected X-RateLimit-Limit 10, got %s", limit)
		}

		remaining := rec.Header().Get("X-RateLimit-Remaining")
		if remaining == "" {
			t.Error("expected X-RateLimit-Remaining to be set")
		}

		reset := rec.Header().Get("X-RateLimit-Reset")
		if reset == "" {
			t.Error("expected X-RateLimit-Reset to be set")
		}
	})

	t.Run("RemainingDecreasesWithEachRequest", func(t *testing.T) {
		limiter := middleware.NewRateLimiter(client, 5, 10, 3)
		rateLimitedHandler := limiter.RateLimit()(handler)

		var previousRemaining = 5

		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "192.168.1.103:1234"
			rec := httptest.NewRecorder()

			rateLimitedHandler.ServeHTTP(rec, req)

			remainingStr := rec.Header().Get("X-RateLimit-Remaining")
			remaining, err := strconv.Atoi(remainingStr)
			if err != nil {
				t.Errorf("failed to parse remaining: %v", err)
				continue
			}

			if remaining >= previousRemaining {
				t.Errorf("expected remaining to decrease, previous: %d, current: %d", previousRemaining, remaining)
			}
			previousRemaining = remaining
		}
	})

	t.Run("DifferentIPsHaveSeparateLimits", func(t *testing.T) {
		limiter := middleware.NewRateLimiter(client, 2, 10, 3)
		rateLimitedHandler := limiter.RateLimit()(handler)

		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "192.168.1.200:1234"
			rec := httptest.NewRecorder()
			rateLimitedHandler.ServeHTTP(rec, req)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.200:1234"
		rec := httptest.NewRecorder()
		rateLimitedHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("first IP: expected status 429, got %d", rec.Code)
		}

		req = httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.201:1234"
		rec = httptest.NewRecorder()
		rateLimitedHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("second IP: expected status 200, got %d", rec.Code)
		}
	})

	t.Run("RespectsXForwardedForHeader", func(t *testing.T) {
		limiter := middleware.NewRateLimiter(client, 2, 10, 3)
		rateLimitedHandler := limiter.RateLimit()(handler)

		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("X-Forwarded-For", "10.0.0.1")
			rec := httptest.NewRecorder()
			rateLimitedHandler.ServeHTTP(rec, req)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		rec := httptest.NewRecorder()
		rateLimitedHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("expected status 429 for X-Forwarded-For IP, got %d", rec.Code)
		}

		req = httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.2")
		rec = httptest.NewRecorder()
		rateLimitedHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 for different X-Forwarded-For IP, got %d", rec.Code)
		}
	})

	t.Run("RespectsXRealIPHeader", func(t *testing.T) {
		limiter := middleware.NewRateLimiter(client, 2, 10, 3)
		rateLimitedHandler := limiter.RateLimit()(handler)

		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("X-Real-IP", "10.1.0.1")
			rec := httptest.NewRecorder()
			rateLimitedHandler.ServeHTTP(rec, req)
		}

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Real-IP", "10.1.0.1")
		rec := httptest.NewRecorder()
		rateLimitedHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("expected status 429 for X-Real-IP, got %d", rec.Code)
		}
	})

	t.Run("AuthRateLimitStricterThanGeneral", func(t *testing.T) {
		authLimiter := middleware.NewRateLimiter(client, 100, 100, 3)
		authHandler := authLimiter.RateLimitAuth()(handler)

		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
			req.RemoteAddr = "192.168.1.150:1234"
			rec := httptest.NewRecorder()
			authHandler.ServeHTTP(rec, req)
		}

		req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
		req.RemoteAddr = "192.168.1.150:1234"
		rec := httptest.NewRecorder()
		authHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("expected status 429 after auth limit, got %d", rec.Code)
		}
	})
}

func TestRateLimiterIntegration_ResetAfterWindow(t *testing.T) {
	ctx := context.Background()

	redisContainer, err := rediscontainer.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}
	defer redisContainer.Terminate(ctx)

	host, err := redisContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get redis host: %v", err)
	}
	port, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("failed to get redis port: %v", err)
	}

	client, err := redismodule.NewClient(redismodule.Config{
		Addr: fmt.Sprintf("%s:%s", host, port.Port()),
	})
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}
	defer client.Close()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	limiter := middleware.NewRateLimiter(client, 2, 10, 3)
	rateLimitedHandler := limiter.RateLimit()(handler)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.180:1234"
		rec := httptest.NewRecorder()
		rateLimitedHandler.ServeHTTP(rec, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.180:1234"
	rec := httptest.NewRecorder()
	rateLimitedHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429 before window reset, got %d", rec.Code)
	}

	time.Sleep(65 * time.Second)

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.180:1234"
	rec = httptest.NewRecorder()
	rateLimitedHandler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 after window reset, got %d", rec.Code)
	}
}

func TestRateLimiterIntegration_ConcurrentRequests(t *testing.T) {
	ctx := context.Background()

	redisContainer, err := rediscontainer.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}
	defer redisContainer.Terminate(ctx)

	host, err := redisContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get redis host: %v", err)
	}
	port, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("failed to get redis port: %v", err)
	}

	client, err := redismodule.NewClient(redismodule.Config{
		Addr: fmt.Sprintf("%s:%s", host, port.Port()),
	})
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}
	defer client.Close()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	limiter := middleware.NewRateLimiter(client, 100, 200, 50)
	rateLimitedHandler := limiter.RateLimit()(handler)

	done := make(chan bool)
	successCount := 0
	rateLimitedCount := 0

	for i := 0; i < 150; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "192.168.1.190:1234"
			rec := httptest.NewRecorder()
			rateLimitedHandler.ServeHTTP(rec, req)

			if rec.Code == http.StatusOK {
				successCount++
			} else if rec.Code == http.StatusTooManyRequests {
				rateLimitedCount++
			}
			done <- true
		}()
	}

	for i := 0; i < 150; i++ {
		<-done
	}

	if successCount > 100 {
		t.Errorf("expected at most 100 successful requests, got %d", successCount)
	}

	if rateLimitedCount < 50 {
		t.Errorf("expected at least 50 rate limited requests, got %d", rateLimitedCount)
	}
}
