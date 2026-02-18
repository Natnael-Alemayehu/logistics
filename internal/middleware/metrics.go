package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/natnael-alemayehu/logistics/pkg/metrics"
)

func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			method := r.Method

			metrics.HTTPRequestsInFlight.WithLabelValues(method).Inc()
			defer metrics.HTTPRequestsInFlight.WithLabelValues(method).Dec()

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				path := getPathPattern(r)
				status := strconv.Itoa(ww.Status())
				duration := time.Since(start).Seconds()

				metrics.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
				metrics.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

func getPathPattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx != nil && rctx.RoutePattern() != "" {
		return rctx.RoutePattern()
	}
	return r.URL.Path
}
