package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetrics_RecordsRequestCount(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry

	r := chi.NewRouter()
	r.Use(Metrics())
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	count, err := testutil.GatherAndCount(prometheus.DefaultGatherer, "http_requests_total")
	if err != nil {
		t.Errorf("error gathering metrics: %v", err)
	}
	if count == 0 {
		t.Errorf("expected request count > 0, got %d", count)
	}
}

func TestMetrics_RecordsDuration(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry

	r := chi.NewRouter()
	r.Use(Metrics())
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	count, err := testutil.GatherAndCount(prometheus.DefaultGatherer, "http_request_duration_seconds")
	if err != nil {
		t.Errorf("error gathering metrics: %v", err)
	}
	if count == 0 {
		t.Errorf("expected histogram count > 0, got %d", count)
	}
}

func TestMetrics_TracksInFlightRequests(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry

	inFlightDuringRequest := 0.0

	r := chi.NewRouter()
	r.Use(Metrics())
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		mfs, err := prometheus.DefaultGatherer.Gather()
		if err == nil {
			for _, mf := range mfs {
				if mf.GetName() == "http_requests_in_flight" {
					for _, m := range mf.GetMetric() {
						inFlightDuringRequest = m.GetGauge().GetValue()
					}
				}
			}
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if inFlightDuringRequest != 1 {
		t.Errorf("expected in-flight requests 1 during request, got %v", inFlightDuringRequest)
	}

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Errorf("error gathering metrics: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() == "http_requests_in_flight" {
			for _, m := range mf.GetMetric() {
				if m.GetGauge().GetValue() != 0 {
					t.Errorf("expected in-flight requests 0 after request, got %v", m.GetGauge().GetValue())
				}
			}
		}
	}
}

func TestGetPathPattern_ExtractsPattern(t *testing.T) {
	r := chi.NewRouter()
	var capturedPattern string
	r.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		capturedPattern = getPathPattern(r)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if capturedPattern != "/users/{id}" {
		t.Errorf("expected pattern /users/{id}, got %s", capturedPattern)
	}
}

func TestGetPathPattern_FallbackToPath(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/fallback/path", nil)

	pattern := getPathPattern(req)
	if pattern != "/fallback/path" {
		t.Errorf("expected pattern /fallback/path, got %s", pattern)
	}
}

func TestMetrics_RecordsDifferentStatusCodes(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry

	r := chi.NewRouter()
	r.Use(Metrics())
	r.Get("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	r.Get("/notfound", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	tests := []struct {
		path       string
		statusCode int
	}{
		{"/ok", http.StatusOK},
		{"/error", http.StatusInternalServerError},
		{"/notfound", http.StatusNotFound},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != tt.statusCode {
			t.Errorf("expected status %d, got %d", tt.statusCode, rec.Code)
		}
	}

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Errorf("error gathering metrics: %v", err)
	}

	statuses := make(map[string]bool)
	for _, mf := range mfs {
		if mf.GetName() == "http_requests_total" {
			for _, m := range mf.GetMetric() {
				for _, l := range m.GetLabel() {
					if l.GetName() == "status" {
						statuses[l.GetValue()] = true
					}
				}
			}
		}
	}

	for _, s := range []string{"200", "500", "404"} {
		if !statuses[s] {
			t.Errorf("expected status %s in metrics", s)
		}
	}
}

func TestMetrics_RecordsDifferentMethods(t *testing.T) {
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = registry

	r := chi.NewRouter()
	r.Use(Metrics())
	r.Get("/resource", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Post("/resource", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	r.Put("/resource", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Delete("/resource", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	tests := []struct {
		method     string
		statusCode int
	}{
		{http.MethodGet, http.StatusOK},
		{http.MethodPost, http.StatusCreated},
		{http.MethodPut, http.StatusOK},
		{http.MethodDelete, http.StatusNoContent},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, "/resource", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != tt.statusCode {
			t.Errorf("expected status %d, got %d", tt.statusCode, rec.Code)
		}
	}

	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Errorf("error gathering metrics: %v", err)
	}

	methods := make(map[string]bool)
	for _, mf := range mfs {
		if mf.GetName() == "http_requests_total" {
			for _, m := range mf.GetMetric() {
				for _, l := range m.GetLabel() {
					if l.GetName() == "method" {
						methods[l.GetValue()] = true
					}
				}
			}
		}
	}

	for _, m := range []string{"GET", "POST", "PUT", "DELETE"} {
		if !methods[m] {
			t.Errorf("expected method %s in metrics", m)
		}
	}
}
