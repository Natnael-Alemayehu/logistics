package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/natnael-alemayehu/logistics/internal/config"
)

// A client may send any forwarding header it likes. Behind a proxy, only
// the address the proxy added counts; without one, none do.
func TestClientIPBelievesOnlyTheProxy(t *testing.T) {
	for _, c := range []struct {
		name    string
		proxy   bool
		headers [][2]string
		want    string
	}{
		{"no proxy", false, [][2]string{{"X-Forwarded-For", "198.51.100.7"}, {"X-Real-IP", "203.0.113.9"}}, "192.0.2.1"},
		{"the proxy's entry", true, [][2]string{{"X-Forwarded-For", "203.0.113.9, 198.51.100.7"}}, "198.51.100.7"},
		{"a header of the client's before the proxy's", true, [][2]string{{"X-Forwarded-For", "203.0.113.9"}, {"X-Forwarded-For", "198.51.100.7"}}, "198.51.100.7"},
		// Caddy passes a client's X-Real-IP through untouched.
		{"X-Real-IP", true, [][2]string{{"X-Real-IP", "203.0.113.9"}, {"X-Forwarded-For", "198.51.100.7"}}, "198.51.100.7"},
		{"X-Real-IP alone", true, [][2]string{{"X-Real-IP", "203.0.113.9"}}, "192.0.2.1"},
		{"not an address", true, [][2]string{{"X-Forwarded-For", "198.51.100.7, unknown"}}, "192.0.2.1"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := &Server{cfg: &config.Config{TrustProxy: c.proxy}}
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "192.0.2.1:51234"
			for _, h := range c.headers {
				req.Header.Add(h[0], h[1])
			}
			var got string
			s.clientIP(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = remoteIP(r)
			})).ServeHTTP(httptest.NewRecorder(), req)
			if got != c.want {
				t.Errorf("client IP %q, want %q", got, c.want)
			}
		})
	}
}
