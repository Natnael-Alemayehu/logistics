// Package metrics holds what the server exports at /metrics for Prometheus.
// Labels stay bounded: route patterns rather than paths, and no company or
// user IDs, which belong in the product's own reports.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "menged_http_requests_total",
		Help: "HTTP requests by route pattern, method and status code.",
	}, []string{"route", "method", "status"})

	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "menged_http_request_duration_seconds",
		Help:    "Time to answer an HTTP request, by route pattern and method.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"route", "method"})

	HTTPInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "menged_http_requests_in_flight",
		Help: "HTTP requests being answered now.",
	})
)

var (
	// SyncChanges counts what driver apps send: kind is status, pod or
	// message; result is applied, refused (the office's decision stands)
	// or retry (a server error the app will send again).
	SyncChanges = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "menged_sync_changes_total",
		Help: "Changes sent by the driver app, by kind and result.",
	}, []string{"kind", "result"})

	SyncLocations = promauto.NewCounter(prometheus.CounterOpts{
		Name: "menged_sync_locations_total",
		Help: "GPS points stored from the driver app.",
	})
)

var (
	// Messages counts attempts to hand a text or email to its provider:
	// result is sent, retry (it will be tried again), failed (given up) or
	// skipped (opted out, or no provider).
	Messages = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "menged_messages_total",
		Help: "Texts and emails handed to a provider, by channel, provider and result.",
	}, []string{"channel", "provider", "result"})

	OutboxWaiting = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "menged_outbox_waiting",
		Help: "Texts and emails waiting to be sent, by channel. Set by the server running the jobs.",
	}, []string{"channel"})

	OutboxOldest = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "menged_outbox_oldest_seconds",
		Help: "Age of the oldest text or email waiting to be sent, by channel.",
	}, []string{"channel"})
)

var WebSocketConnections = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "menged_websocket_connections",
	Help: "Dashboard live-update connections open on this server.",
})

// The background jobs are labelled name, not job: Prometheus keeps job for
// the scrape target.
var (
	JobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "menged_job_duration_seconds",
		Help:    "Time a background job took, by job.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 300},
	}, []string{"name"})

	JobFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "menged_job_failures_total",
		Help: "Background job runs that ended in an error, by job.",
	}, []string{"name"})

	JobLastSuccess = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "menged_job_last_success_timestamp_seconds",
		Help: "When each background job last finished without an error.",
	}, []string{"name"})
)
