// Package metrics holds what the server exports at /metrics for Prometheus.
// Labels stay bounded: route patterns rather than paths, and no company or
// user IDs, which belong in the product's own reports.
package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kiflet_http_requests_total",
		Help: "HTTP requests by route pattern, method and status code.",
	}, []string{"route", "method", "status"})

	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "kiflet_http_request_duration_seconds",
		Help:    "Time to answer an HTTP request, by route pattern and method.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"route", "method"})

	HTTPInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "kiflet_http_requests_in_flight",
		Help: "HTTP requests being answered now.",
	})
)

var (
	// SyncChanges counts what driver apps send: kind is status, pod or
	// message; result is applied, refused (the office's decision stands)
	// or retry (a server error the app will send again).
	SyncChanges = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kiflet_sync_changes_total",
		Help: "Changes sent by the driver app, by kind and result.",
	}, []string{"kind", "result"})

	SyncLocations = promauto.NewCounter(prometheus.CounterOpts{
		Name: "kiflet_sync_locations_total",
		Help: "GPS points stored from the driver app.",
	})
)

var (
	// Messages counts attempts to hand a text or email to its provider:
	// result is sent, retry (it will be tried again), failed (given up) or
	// skipped (opted out, or no provider).
	Messages = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kiflet_messages_total",
		Help: "Texts and emails handed to a provider, by channel, provider and result.",
	}, []string{"channel", "provider", "result"})

	// SMSDeliveries counts the provider's delivery reports for texts it
	// accepted: result is delivered or undelivered.
	SMSDeliveries = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kiflet_sms_delivery_reports_total",
		Help: "Delivery reports from the SMS provider, by provider and result.",
	}, []string{"provider", "result"})

	OutboxWaiting = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kiflet_outbox_waiting",
		Help: "Texts and emails waiting to be sent, by channel. Set by the server running the jobs.",
	}, []string{"channel"})

	OutboxOldest = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kiflet_outbox_oldest_seconds",
		Help: "Age of the oldest text or email waiting to be sent, by channel.",
	}, []string{"channel"})
)

// VerificationBudgetUsed is how much of the day's budget of verification
// codes has been texted. Sign-up and lookup codes go to any number, so the
// budget is what stops a script running up the SMS bill.
var VerificationBudgetUsed = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "kiflet_verification_budget_used_ratio",
	Help: "Share of the daily budget of verification codes texted in the last 24 hours.",
})

var diskOnce sync.Once

// WatchDisk exports kiflet_disk_free_ratio, read from free() at each
// scrape: the share of the disk holding uploads that is free. Only the
// first call counts; a server with no disk to watch (S3 storage) doesn't
// call it, and the metric is then absent.
func WatchDisk(free func() float64) {
	diskOnce.Do(func() { prometheus.MustRegister(diskFreeRatio(free)) })
}

func diskFreeRatio(free func() float64) prometheus.GaugeFunc {
	return prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "kiflet_disk_free_ratio",
		Help: "Share of the disk holding uploads that is free.",
	}, free)
}

var WebSocketConnections = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "kiflet_websocket_connections",
	Help: "Dashboard live-update connections open on this server.",
})

// The background jobs are labelled name, not job: Prometheus keeps job for
// the scrape target.
var (
	JobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "kiflet_job_duration_seconds",
		Help:    "Time a background job took, by job.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 300},
	}, []string{"name"})

	JobFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kiflet_job_failures_total",
		Help: "Background job runs that ended in an error, by job.",
	}, []string{"name"})

	JobLastSuccess = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kiflet_job_last_success_timestamp_seconds",
		Help: "When each background job last finished without an error.",
	}, []string{"name"})
)
