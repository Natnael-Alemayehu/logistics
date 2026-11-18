//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"
)

// TestMonitoring: Prometheus scrapes the app over the internal network with
// its token, and Grafana shows it. Each figure is read through Grafana's
// Prometheus data source, as its dashboard reads them.
func TestMonitoring(t *testing.T) {
	var health struct {
		Database string `json:"database"`
	}
	if code := grafana(t, "/api/health", &health); code != http.StatusOK || health.Database != "ok" {
		t.Fatalf("Grafana's health: status %d, database %q", code, health.Database)
	}

	// The app is up as Prometheus sees it, and the figures are the app's:
	// the requests it served, the texts it sent and the jobs it ran. Each
	// query comes to one number: staging runs two instances, and up has a
	// series for each, so every one of them must be up.
	for _, q := range []string{
		`min(up{job="kiflet"})`,
		`sum(kiflet_http_requests_total{job="kiflet"})`,
		`sum(kiflet_messages_total{job="kiflet",channel="sms",result="sent"})`,
		`count(kiflet_job_last_success_timestamp_seconds{job="kiflet"})`,
	} {
		within(t, 90*time.Second, "Prometheus to have "+q, func() bool { return query(t, q) >= 1 })
	}

	var dash struct {
		Dashboard struct {
			Title  string            `json:"title"`
			Panels []json.RawMessage `json:"panels"`
		} `json:"dashboard"`
	}
	if code := grafana(t, "/api/dashboards/uid/kiflet-overview", &dash); code != http.StatusOK || dash.Dashboard.Title != "Kiflet" || len(dash.Dashboard.Panels) == 0 {
		t.Errorf("the Kiflet dashboard: status %d, %q with %d panels", code, dash.Dashboard.Title, len(dash.Dashboard.Panels))
	}
}

// grafana calls Grafana's API as its administrator.
func grafana(t *testing.T, path string, out any) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, grafanaURL+path, nil)
	req.SetBasicAuth(grafanaUser, grafanaPass)
	res := send(t, newClient(nil), req)
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatalf("Grafana %s: %v", path, err)
		}
	}
	return res.StatusCode
}

// query asks Prometheus, through Grafana, for one number; -1 if it has none,
// or more than one (a query over several instances must aggregate them).
func query(t *testing.T, q string) float64 {
	t.Helper()
	var res struct {
		Data struct {
			Result []struct {
				Value [2]any `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if code := grafana(t, "/api/datasources/proxy/uid/prometheus/api/v1/query?query="+url.QueryEscape(q), &res); code != http.StatusOK || len(res.Data.Result) != 1 {
		return -1
	}
	s, _ := res.Data.Result[0].Value[1].(string)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return -1
	}
	return v
}
