package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// Every metric follows Prometheus naming: units in the name, _total on
// counters, help text.
func TestMetricsFollowConventions(t *testing.T) {
	// A vector reports nothing until it has a child.
	HTTPRequests.WithLabelValues("/app", "GET", "200").Inc()
	HTTPDuration.WithLabelValues("/app", "GET").Observe(0.01)
	SyncChanges.WithLabelValues("status", "applied").Inc()
	Messages.WithLabelValues("sms", "mock", "sent").Inc()
	OutboxWaiting.WithLabelValues("sms").Set(0)
	OutboxOldest.WithLabelValues("sms").Set(0)
	JobDuration.WithLabelValues("billing").Observe(0.1)
	JobFailures.WithLabelValues("billing").Add(0)
	JobLastSuccess.WithLabelValues("billing").SetToCurrentTime()

	for _, c := range []prometheus.Collector{
		HTTPRequests, HTTPDuration, HTTPInFlight, SyncChanges, SyncLocations,
		Messages, OutboxWaiting, OutboxOldest, VerificationBudgetUsed, WebSocketConnections,
		JobDuration, JobFailures, JobLastSuccess,
	} {
		problems, err := testutil.CollectAndLint(c)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range problems {
			t.Errorf("%s: %s", p.Metric, p.Text)
		}
	}
}
