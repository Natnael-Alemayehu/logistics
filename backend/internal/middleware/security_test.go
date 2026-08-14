package middleware

import (
	"bytes"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	tests := []struct {
		name           string
		tls            bool
		forwardedProto string
		wantHSTS       bool
	}{
		{
			name:     "sets all security headers without HSTS for HTTP",
			tls:      false,
			wantHSTS: false,
		},
		{
			name:     "sets HSTS for HTTPS via TLS",
			tls:      true,
			wantHSTS: true,
		},
		{
			name:           "sets HSTS for HTTPS via X-Forwarded-Proto",
			tls:            false,
			forwardedProto: "https",
			wantHSTS:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/", nil)
			if tt.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
			}
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}

			rec := httptest.NewRecorder()
			SecurityHeaders()(handler).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
			}

			headers := rec.Header()
			if got := headers.Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options: expected nosniff, got %q", got)
			}
			if got := headers.Get("X-Frame-Options"); got != "DENY" {
				t.Errorf("X-Frame-Options: expected DENY, got %q", got)
			}
			if got := headers.Get("X-XSS-Protection"); got != "1; mode=block" {
				t.Errorf("X-XSS-Protection: expected '1; mode=block', got %q", got)
			}
			if got := headers.Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
				t.Errorf("Referrer-Policy: expected strict-origin-when-cross-origin, got %q", got)
			}
			if got := headers.Get("Content-Security-Policy"); got != "default-src 'self'" {
				t.Errorf("Content-Security-Policy: expected default-src 'self', got %q", got)
			}

			hsts := headers.Get("Strict-Transport-Security")
			if tt.wantHSTS {
				if hsts != "max-age=31536000; includeSubDomains" {
					t.Errorf("HSTS: expected 'max-age=31536000; includeSubDomains', got %q", hsts)
				}
			} else {
				if hsts != "" {
					t.Errorf("HSTS: expected empty, got %q", hsts)
				}
			}
		})
	}
}

func TestContentType(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		contentType string
		wantStatus  int
	}{
		{
			name:       "GET request passes without Content-Type",
			method:     "GET",
			wantStatus: http.StatusOK,
		},
		{
			name:       "DELETE request passes without Content-Type",
			method:     "DELETE",
			wantStatus: http.StatusOK,
		},
		{
			name:       "HEAD request passes without Content-Type",
			method:     "HEAD",
			wantStatus: http.StatusOK,
		},
		{
			name:       "OPTIONS request passes without Content-Type",
			method:     "OPTIONS",
			wantStatus: http.StatusOK,
		},
		{
			name:        "POST with application/json passes",
			method:      "POST",
			contentType: "application/json",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "POST with application/json; charset=utf-8 passes",
			method:      "POST",
			contentType: "application/json; charset=utf-8",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "POST without Content-Type fails",
			method:      "POST",
			contentType: "",
			wantStatus:  http.StatusUnsupportedMediaType,
		},
		{
			name:        "POST with text/plain fails",
			method:      "POST",
			contentType: "text/plain",
			wantStatus:  http.StatusUnsupportedMediaType,
		},
		{
			name:        "POST with application/xml fails",
			method:      "POST",
			contentType: "application/xml",
			wantStatus:  http.StatusUnsupportedMediaType,
		},
		{
			name:        "PUT with application/json passes",
			method:      "PUT",
			contentType: "application/json",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "PUT without Content-Type fails",
			method:      "PUT",
			contentType: "",
			wantStatus:  http.StatusUnsupportedMediaType,
		},
		{
			name:        "PATCH with application/json passes",
			method:      "PATCH",
			contentType: "application/json",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "PATCH with text/html fails",
			method:      "PATCH",
			contentType: "text/html",
			wantStatus:  http.StatusUnsupportedMediaType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			var body bytes.Buffer
			if tt.method == "POST" || tt.method == "PUT" || tt.method == "PATCH" {
				body.WriteString(`{"test":"data"}`)
			}

			req := httptest.NewRequest(tt.method, "/", &body)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			rec := httptest.NewRecorder()
			ContentType()(handler).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestRequestSizeLimit(t *testing.T) {
	tests := []struct {
		name       string
		maxSize    int64
		bodySize   int
		wantStatus int
	}{
		{
			name:       "small request body passes",
			maxSize:    1024,
			bodySize:   100,
			wantStatus: http.StatusOK,
		},
		{
			name:       "exact size request body passes",
			maxSize:    1024,
			bodySize:   1024,
			wantStatus: http.StatusOK,
		},
		{
			name:       "large request body fails",
			maxSize:    1024,
			bodySize:   2048,
			wantStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				buf := make([]byte, tt.bodySize+1)
				_, err := r.Body.Read(buf)
				if err != nil && int64(tt.bodySize) > tt.maxSize {
					http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
					return
				}
				w.WriteHeader(http.StatusOK)
			})

			body := make([]byte, tt.bodySize)
			for i := range body {
				body[i] = 'a'
			}

			req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()
			RequestSizeLimit(tt.maxSize)(handler).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestRequestSizeLimitWithNilBody(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)

	rec := httptest.NewRecorder()
	RequestSizeLimit(1024)(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}
