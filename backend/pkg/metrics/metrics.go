package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	HTTPRequestsInFlight = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
		[]string{"method"},
	)
)

var (
	ShipmentsCreated = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "shipments_created_total",
			Help: "Total number of shipments created",
		},
		[]string{"tenant_id"},
	)

	DeliveriesCompleted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "deliveries_completed_total",
			Help: "Total number of deliveries completed",
		},
		[]string{"tenant_id"},
	)

	SyncEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sync_events_total",
			Help: "Total number of sync events received",
		},
		[]string{"event_type"},
	)
)

var (
	SMSSentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sms_sent_total",
			Help: "Total number of SMS messages sent",
		},
		[]string{"provider", "status"},
	)
)

var (
	WSConnectionsActive = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "websocket_connections_active",
			Help: "Number of active WebSocket connections",
		},
		[]string{"tenant_id"},
	)
)

var (
	QueueJobsPending = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "queue_jobs_pending",
			Help: "Number of pending jobs in the queue",
		},
		[]string{"job_type"},
	)

	QueueJobsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "queue_jobs_processed_total",
			Help: "Total number of jobs processed",
		},
		[]string{"job_type", "status"},
	)
)

var (
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"query"},
	)
)
