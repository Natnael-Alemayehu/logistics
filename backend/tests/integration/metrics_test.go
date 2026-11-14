//go:build integration

package integration

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

const metricsToken = "integration-metrics-token"

// TestMetrics: /metrics answers only with its token, and counts requests by
// route pattern, driver app changes and texts, with no IDs in the labels.
func TestMetrics(t *testing.T) {
	e := newEnv(t)
	office := e.company("Metric Movers")
	driverID, phone, pin := office.addDriver("Abebe Kebede")
	id, _ := office.book("Addis Ababa", "Adama", driverID)
	office.get("/app/shipments/" + id)
	d := e.signIn(phone, pin)
	if r := d.status(clientID(), id, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	e.received("sms", phone, "")

	scrape := func(auth string) (int, string) {
		req, _ := http.NewRequest(http.MethodGet, e.srv.URL+"/metrics", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(body)
	}
	for _, auth := range []string{"", "Bearer wrong"} {
		if status, _ := scrape(auth); status != http.StatusNotFound {
			t.Errorf("/metrics with %q: status %d, want 404", auth, status)
		}
	}
	status, body := scrape("Bearer " + metricsToken)
	if status != http.StatusOK {
		t.Fatalf("/metrics: status %d", status)
	}
	for _, want := range []string{
		`menged_http_requests_total{method="GET",route="/app/shipments/{id}",status="200"}`,
		`menged_http_request_duration_seconds_bucket{method="POST",route="/api/v1/driver/sync"`,
		`menged_sync_changes_total{kind="status",result="applied"}`,
		`menged_messages_total{channel="sms",provider="mock",result="sent"}`,
		`menged_websocket_connections`,
		`go_goroutines`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/metrics is missing %s", want)
		}
	}
	if strings.Contains(body, id) {
		t.Error("a shipment ID made its way into the metrics")
	}
}
