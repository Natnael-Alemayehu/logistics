package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/natnael-alemayehu/logistics/pkg/redis"
	"github.com/natnael-alemayehu/logistics/pkg/response"
)

type RateLimiter struct {
	redis     *redis.Client
	limitIP   int
	limitUser int
	limitAuth int
	window    time.Duration
}

func NewRateLimiter(redisClient *redis.Client, limitIP, limitUser, limitAuth int) *RateLimiter {
	return &RateLimiter{
		redis:     redisClient,
		limitIP:   limitIP,
		limitUser: limitUser,
		limitAuth: limitAuth,
		window:    time.Minute,
	}
}

func (rl *RateLimiter) RateLimit() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getIP(r)
			key := "ratelimit:ip:" + ip

			allowed, remaining, resetAfter, err := rl.checkRateLimit(r.Context(), key, rl.limitIP)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.limitIP))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAfter, 10))

			if !allowed {
				response.ErrorJSON(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests. Please try again later.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (rl *RateLimiter) RateLimitUser() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r.Context())
			if userID == "" {
				next.ServeHTTP(w, r)
				return
			}

			key := "ratelimit:user:" + userID

			allowed, remaining, resetAfter, err := rl.checkRateLimit(r.Context(), key, rl.limitUser)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.limitUser))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAfter, 10))

			if !allowed {
				response.ErrorJSON(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests. Please try again later.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (rl *RateLimiter) RateLimitAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getIP(r)
			key := "ratelimit:auth:" + ip

			allowed, remaining, resetAfter, err := rl.checkRateLimit(r.Context(), key, rl.limitAuth)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.limitAuth))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAfter, 10))

			if !allowed {
				response.ErrorJSON(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "Too many authentication attempts. Please try again later.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (rl *RateLimiter) checkRateLimit(ctx context.Context, key string, limit int) (allowed bool, remaining int, resetAfter int64, err error) {
	count, err := rl.redis.Incr(ctx, key)
	if err != nil {
		return false, 0, 0, err
	}

	if count == 1 {
		if err := rl.redis.Expire(ctx, key, rl.window); err != nil {
			return false, 0, 0, err
		}
	}

	ttl, err := rl.redis.TTL(ctx, key)
	if err != nil {
		return false, 0, 0, err
	}

	resetAfter = int64(ttl.Seconds())
	remaining = int(int64(limit) - count)
	if remaining < 0 {
		remaining = 0
	}

	allowed = count <= int64(limit)
	return allowed, remaining, resetAfter, nil
}

func getIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
