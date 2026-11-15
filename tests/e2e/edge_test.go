//go:build e2e

package e2e

import (
	"crypto/tls"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestEdge: what anyone reaching the site meets first. Plain HTTP is sent to
// HTTPS, which is TLS 1.2 or newer over HTTP/2, with the security headers on
// every page, and the metrics are not reachable from outside.
func TestEdge(t *testing.T) {
	visitor := newBrowser(t)

	p := visitor.get(plainURL + "/track?number=ET-1")
	if p.Status != http.StatusPermanentRedirect || p.Location != appURL+"/track?number=ET-1" {
		t.Errorf("plain HTTP: status %d to %q, want a permanent redirect to the same page over HTTPS", p.Status, p.Location)
	}

	p = visitor.get("/login")
	if p.Status != http.StatusOK {
		t.Fatalf("the sign-in page: status %d", p.Status)
	}
	if p.TLS == nil || p.TLS.Version < tls.VersionTLS12 || p.Proto != 2 {
		t.Errorf("connection: TLS %v, HTTP/%d, want TLS 1.2 or newer and HTTP/2", p.TLS, p.Proto)
	}
	hsts := p.Header.Get("Strict-Transport-Security")
	if age, _ := strconv.Atoi(strings.TrimPrefix(strings.Split(hsts, ";")[0], "max-age=")); age < 180*24*3600 {
		t.Errorf("Strict-Transport-Security %q, want at least half a year", hsts)
	}
	for header, want := range map[string]string{
		"Content-Security-Policy": "script-src 'self'",
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
	} {
		if got := p.Header.Get(header); !strings.Contains(got, want) {
			t.Errorf("%s %q, want %q", header, got, want)
		}
	}
	if csp := p.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("Content-Security-Policy %q lets other sites frame the pages", csp)
	}
	if alt := p.Header.Get("Alt-Svc"); !strings.Contains(alt, "h3=") {
		t.Errorf("Alt-Svc %q, want HTTP/3 offered", alt)
	}

	for path, want := range map[string]int{
		"/healthz": http.StatusOK,
		"/readyz":  http.StatusOK,
		// Prometheus scrapes it on the internal network; the edge refuses it
		// even with the token.
		"/metrics": http.StatusNotFound,
	} {
		if got := visitor.get(path).Status; got != want {
			t.Errorf("GET %s: status %d, want %d", path, got, want)
		}
	}

	// Static files are compressed, and kept by browsers for good under the
	// versioned names the pages use.
	css := cssRe.FindStringSubmatch(p.Body)
	if css == nil {
		t.Fatal("the sign-in page links no stylesheet")
	}
	req, _ := http.NewRequest(http.MethodGet, appURL+css[1], nil)
	req.Header.Set("Accept-Encoding", "gzip")
	res, err := newClient(nil).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Encoding") != "gzip" {
		t.Errorf("app.css: status %d, encoding %q, want gzip", res.StatusCode, res.Header.Get("Content-Encoding"))
	}
	if cc := res.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("app.css: Cache-Control %q, want it kept", cc)
	}
}

var cssRe = regexp.MustCompile(`href="(/static/app\.css\?v=[^"]+)"`)
