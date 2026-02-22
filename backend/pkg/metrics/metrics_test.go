package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestHTTPMetricsRegistration(t *testing.T) {
	assert.NotNil(t, HTTPRequestsTotal)
	assert.NotNil(t, HTTPRequestDuration)
	assert.NotNil(t, HTTPRequestsInFlight)
}

func TestBusinessMetricsRegistration(t *testing.T) {
	assert.NotNil(t, ShipmentsCreated)
	assert.NotNil(t, DeliveriesCompleted)
	assert.NotNil(t, SyncEventsTotal)
}

func TestSMSMetricsRegistration(t *testing.T) {
	assert.NotNil(t, SMSSentTotal)
}

func TestWebSocketMetricsRegistration(t *testing.T) {
	assert.NotNil(t, WSConnectionsActive)
}

func TestQueueMetricsRegistration(t *testing.T) {
	assert.NotNil(t, QueueJobsPending)
	assert.NotNil(t, QueueJobsProcessed)
}

func TestDatabaseMetricsRegistration(t *testing.T) {
	assert.NotNil(t, DBQueryDuration)
}

func TestCounterIncrement(t *testing.T) {
	registry := prometheus.NewRegistry()
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_counter",
			Help: "Test counter",
		},
		[]string{"label"},
	)
	registry.MustRegister(counter)

	counter.WithLabelValues("test").Inc()

	count := testutil.ToFloat64(counter.WithLabelValues("test"))
	assert.Equal(t, float64(1), count)

	counter.WithLabelValues("test").Inc()
	count = testutil.ToFloat64(counter.WithLabelValues("test"))
	assert.Equal(t, float64(2), count)
}

func TestGaugeSet(t *testing.T) {
	registry := prometheus.NewRegistry()
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "test_gauge",
			Help: "Test gauge",
		},
		[]string{"label"},
	)
	registry.MustRegister(gauge)

	gauge.WithLabelValues("test").Set(42)

	value := testutil.ToFloat64(gauge.WithLabelValues("test"))
	assert.Equal(t, float64(42), value)

	gauge.WithLabelValues("test").Set(100)
	value = testutil.ToFloat64(gauge.WithLabelValues("test"))
	assert.Equal(t, float64(100), value)
}

func TestHistogramObserve(t *testing.T) {
	registry := prometheus.NewRegistry()
	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "test_histogram",
			Help:    "Test histogram",
			Buckets: []float64{0.1, 0.5, 1},
		},
		[]string{"label"},
	)
	registry.MustRegister(histogram)

	histogram.WithLabelValues("test").Observe(0.2)
	histogram.WithLabelValues("test").Observe(0.6)
	histogram.WithLabelValues("test").Observe(1.5)

	assert.NotNil(t, histogram)
}

func TestHTTPRequestsTotalWithLabels(t *testing.T) {
	HTTPRequestsTotal.WithLabelValues("GET", "/api/shipments", "200").Inc()
	HTTPRequestsTotal.WithLabelValues("POST", "/api/shipments", "201").Inc()

	assert.NotNil(t, HTTPRequestsTotal.WithLabelValues("GET", "/api/shipments", "200"))
	assert.NotNil(t, HTTPRequestsTotal.WithLabelValues("POST", "/api/shipments", "201"))
}

func TestShipmentsCreatedWithLabels(t *testing.T) {
	ShipmentsCreated.WithLabelValues("tenant-123").Inc()
	ShipmentsCreated.WithLabelValues("tenant-456").Add(5)

	assert.NotNil(t, ShipmentsCreated.WithLabelValues("tenant-123"))
	assert.NotNil(t, ShipmentsCreated.WithLabelValues("tenant-456"))
}

func TestWSConnectionsActiveGauge(t *testing.T) {
	WSConnectionsActive.WithLabelValues("tenant-123").Inc()
	WSConnectionsActive.WithLabelValues("tenant-123").Inc()
	WSConnectionsActive.WithLabelValues("tenant-456").Set(10)

	assert.NotNil(t, WSConnectionsActive.WithLabelValues("tenant-123"))
	assert.NotNil(t, WSConnectionsActive.WithLabelValues("tenant-456"))
}

func TestDBQueryDurationObserve(t *testing.T) {
	DBQueryDuration.WithLabelValues("SELECT_shipments").Observe(0.025)
	DBQueryDuration.WithLabelValues("INSERT_shipment").Observe(0.150)

	assert.NotNil(t, DBQueryDuration.WithLabelValues("SELECT_shipments"))
	assert.NotNil(t, DBQueryDuration.WithLabelValues("INSERT_shipment"))
}
