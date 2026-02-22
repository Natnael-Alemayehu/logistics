package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_SetsAllowOriginHeader(t *testing.T) {
	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected Access-Control-Allow-Origin to be '*', got %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}

	if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials to be 'true', got %s", rec.Header().Get("Access-Control-Allow-Credentials"))
	}

	if rec.Header().Get("Access-Control-Expose-Headers") != "Link" {
		t.Errorf("expected Access-Control-Expose-Headers to be 'Link', got %s", rec.Header().Get("Access-Control-Expose-Headers"))
	}
}

func TestCORS_OptionsPreflight(t *testing.T) {
	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type,Authorization")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected Access-Control-Allow-Origin to be '*', got %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if rec.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected Access-Control-Allow-Methods header to be set for preflight")
	}
}

func TestCORS_AllowsAllOrigins(t *testing.T) {
	origins := []string{
		"http://localhost:3000",
		"http://example.com",
		"https://app.example.com",
		"http://192.168.1.1:8080",
	}

	for _, origin := range origins {
		handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Errorf("for origin %s: expected Access-Control-Allow-Origin to be '*', got %s", origin, rec.Header().Get("Access-Control-Allow-Origin"))
		}
	}
}

func TestCORS_NextHandlerCalled(t *testing.T) {
	called := false
	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("response body"))
	}))

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("next handler should have been called")
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
	if rec.Body.String() != "response body" {
		t.Errorf("expected body 'response body', got %s", rec.Body.String())
	}
}

func TestCORS_PreflightWithRequestHeaders(t *testing.T) {
	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type,Authorization")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	allowedHeaders := rec.Header().Get("Access-Control-Allow-Headers")
	if allowedHeaders == "" {
		t.Error("expected Access-Control-Allow-Headers to be set for preflight with request headers")
	}
}
