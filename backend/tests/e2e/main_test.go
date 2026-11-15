//go:build e2e

// Package e2e tests a running deployment from the outside, as the people and
// providers using it reach it: staging (deployments/staging), over HTTPS
// through Caddy, with the provider sandbox playing the phones, telebirr and
// CBE Birr, and Mailpit receiving the email.
//
//	make staging-up && make test-e2e
//
// Nothing here reads the database or calls the app's code: each test checks
// what a person, a phone or a provider would see.
package e2e

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Where staging is, as deployments/staging/compose.yml serves it.
var (
	appURL      = setting("E2E_APP_URL", "https://kiflet.localhost:8443")
	plainURL    = setting("E2E_HTTP_URL", "http://kiflet.localhost:8088")
	sandboxURL  = setting("E2E_SANDBOX_URL", "https://sandbox.kiflet.localhost:8443")
	mailURL     = setting("E2E_MAIL_URL", "https://mail.kiflet.localhost:8443")
	grafanaURL  = setting("E2E_GRAFANA_URL", "https://grafana.kiflet.localhost:8443")
	grafanaUser = setting("E2E_GRAFANA_USER", "admin")
	grafanaPass = setting("E2E_GRAFANA_PASSWORD", "staging-grafana-password")
)

const (
	// smsSender is the sender ID the app's texts arrive from.
	smsSender = "KIFLET"
	walletPIN = "1234"
	password  = "correct-horse-1"
)

func setting(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// transport trusts staging's certificates, which Caddy's own authority
// signs, and reaches *.localhost on this machine.
var transport *http.Transport

func TestMain(m *testing.M) {
	file := os.Getenv("E2E_CA_FILE")
	pem, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "E2E_CA_FILE (%q): %v\nStart staging with `make staging-up`, which writes Caddy's root certificate, and run `make test-e2e`.\n", file, err)
		os.Exit(1)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		fmt.Fprintf(os.Stderr, "E2E_CA_FILE: no certificate in %s\n", file)
		os.Exit(1)
	}
	transport = &http.Transport{
		DialContext:         dial,
		TLSClientConfig:     &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2:   true,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	if res, err := newClient(nil).Get(appURL + "/readyz"); err != nil || res.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "staging isn't ready at %s (%v): start it with `make staging-up`\n", appURL, err)
		os.Exit(1)
	} else {
		res.Body.Close()
	}
	os.Exit(m.Run())
}

// dial reaches *.localhost on this machine, as browsers do, whatever the
// resolver says.
func dial(ctx context.Context, network, addr string) (net.Conn, error) {
	if host, port, err := net.SplitHostPort(addr); err == nil && (host == "localhost" || strings.HasSuffix(host, ".localhost")) {
		addr = net.JoinHostPort("127.0.0.1", port)
	}
	return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, addr)
}

// newClient is a client that doesn't follow redirects, so a test sees each.
func newClient(jar http.CookieJar) *http.Client {
	return &http.Client{
		Jar: jar, Transport: transport, Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// within waits up to d for cond, failing the test with what it waited for.
func within(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(d); !cond(); time.Sleep(200 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("waited %s for %s", d, what)
		}
	}
}

func phoneNumber() string { return fmt.Sprintf("091%07d", rand.IntN(1e7)) }

func clientID() string { return fmt.Sprintf("e2e-%d", rand.Int64()) }

// ---------------------------------------------------------------------------
// A browser
// ---------------------------------------------------------------------------

type browser struct {
	t *testing.T
	c *http.Client
	// header goes with every request, as headers a client chose to send.
	header http.Header
	// email, phone and company are the signed-up administrator's.
	email, phone, company string
}

func newBrowser(t *testing.T) *browser {
	jar, _ := cookiejar.New(nil)
	return &browser{t: t, c: newClient(jar)}
}

type page struct {
	Status   int
	Location string
	Header   http.Header
	Body     string
	TLS      *tls.ConnectionState
	Proto    int
}

// send makes a request, waiting out the app's rate limits as a person
// would: every test reaches staging from the same address.
func send(t *testing.T, c *http.Client, req *http.Request) *http.Response {
	t.Helper()
	for try := 1; ; try++ {
		res, err := c.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", req.Method, req.URL, err)
		}
		if res.StatusCode != http.StatusTooManyRequests || try == 4 {
			return res
		}
		res.Body.Close()
		wait, err := strconv.Atoi(res.Header.Get("Retry-After"))
		if err != nil || wait < 1 || wait > 60 {
			wait = 20
		}
		t.Logf("%s %s: too many requests from this address, waiting %ds", req.Method, req.URL.Path, wait)
		time.Sleep(time.Duration(wait) * time.Second)
		if req.GetBody != nil {
			req.Body, _ = req.GetBody()
		}
	}
}

func (b *browser) do(req *http.Request) page {
	b.t.Helper()
	for k, vs := range b.header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	res := send(b.t, b.c, req)
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return page{Status: res.StatusCode, Location: res.Header.Get("Location"), Header: res.Header, Body: string(body), TLS: res.TLS, Proto: res.ProtoMajor}
}

// absolute makes a path of the app's a URL.
func absolute(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return appURL + path
}

func (b *browser) get(path string) page {
	req, _ := http.NewRequest(http.MethodGet, absolute(path), nil)
	return b.do(req)
}

// post sends a form as the browser does, from a page of the same site.
func (b *browser) post(path string, form url.Values) page {
	req, _ := http.NewRequest(http.MethodPost, absolute(path), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if u, err := url.Parse(absolute(path)); err == nil {
		req.Header.Set("Origin", u.Scheme+"://"+u.Host)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
	}
	return b.do(req)
}

// mustPost posts a form and fails unless the app redirects, its answer to
// every action that worked.
func (b *browser) mustPost(path string, form url.Values) page {
	b.t.Helper()
	p := b.post(path, form)
	if p.Status != http.StatusSeeOther {
		b.t.Fatalf("POST %s: status %d, want 303\n%s", path, p.Status, errorText(p.Body))
	}
	return p
}

var flashRe = regexp.MustCompile(`(?s)class="(?:flash flash-error|field-error)"[^>]*>(.*?)<`)

// errorText pulls the error messages out of a page, for failure output.
func errorText(body string) string {
	var out []string
	for _, m := range flashRe.FindAllStringSubmatch(body, -1) {
		out = append(out, strings.TrimSpace(m[1]))
	}
	if len(out) == 0 && len(body) > 400 {
		return body[:400]
	}
	return strings.Join(out, "; ")
}

var codeRe = regexp.MustCompile(`\b\d{6}\b`)

// signUp signs up a new company, confirming the phone with the code texted
// to it and the email address with the link emailed to it.
func signUp(t *testing.T, company string) *browser {
	t.Helper()
	b := newBrowser(t)
	b.company = company
	b.phone = phoneNumber()
	b.email = fmt.Sprintf("e2e%d@example.et", rand.IntN(1e9))
	p := b.mustPost("/signup", url.Values{
		"company": {company}, "city": {"Addis Ababa"}, "name": {"Owner of " + company},
		"email": {b.email}, "phone": {b.phone}, "password": {password},
	})
	if p.Location != "/signup/verify" {
		t.Fatalf("signing up went to %q", p.Location)
	}
	text := waitForText(t, b.phone, "code")
	if p = b.mustPost("/signup/verify", url.Values{"code": {codeRe.FindString(text)}}); p.Location != "/app" {
		t.Fatalf("the code went to %q", p.Location)
	}
	link := verifyLinkRe.FindString(waitForEmail(t, b.email, "Confirm").Text)
	if link == "" {
		t.Fatal("no confirmation link in the email")
	}
	if p = b.get(link); p.Status != http.StatusSeeOther || p.Location != "/app/account" {
		t.Fatalf("confirming the address: status %d to %q", p.Status, p.Location)
	}
	return b
}

var verifyLinkRe = regexp.MustCompile(`https://\S+/verify-email/[A-Za-z0-9_-]+`)

// ---------------------------------------------------------------------------
// The provider sandbox: phones and wallets
// ---------------------------------------------------------------------------

// sandbox calls the sandbox's API, decodes the answer into out and returns
// the status.
func sandbox(t *testing.T, method, path string, body, out any) int {
	t.Helper()
	var r io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		r = bytes.NewReader(buf)
	}
	req, _ := http.NewRequest(method, sandboxURL+path, r)
	req.Header.Set("Content-Type", "application/json")
	res, err := newClient(nil).Do(req)
	if err != nil {
		t.Fatalf("sandbox %s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("sandbox %s %s: decode %q: %v", method, path, raw, err)
		}
	}
	return res.StatusCode
}

type phone struct {
	Texts []struct {
		At     time.Time `json:"at"`
		From   string    `json:"from"`
		Mine   bool      `json:"mine"`
		Body   string    `json:"body"`
		State  string    `json:"state"`
		Report string    `json:"report"`
	} `json:"texts"`
	Telebirr int64 `json:"telebirr_santim"`
	CBEBirr  int64 `json:"cbebirr_santim"`
}

func phoneOf(t *testing.T, number string) phone {
	t.Helper()
	var ph phone
	if code := sandbox(t, http.MethodGet, "/api/phones/"+number, nil, &ph); code != http.StatusOK {
		t.Fatalf("sandbox phone %s: status %d", number, code)
	}
	return ph
}

// waitForText waits for a text from the app to a phone that mentions what,
// and returns it.
func waitForText(t *testing.T, number, what string) string {
	t.Helper()
	var text string
	within(t, 20*time.Second, "a text to "+number+" mentioning "+what, func() bool {
		for _, tx := range phoneOf(t, number).Texts {
			if !tx.Mine && tx.From == smsSender && strings.Contains(tx.Body, what) {
				text = tx.Body
				return true
			}
		}
		return false
	})
	return text
}

// text sends a text from a phone to the app's number, and returns the
// status the app answered the gateway with.
func text(t *testing.T, from, body string) int {
	t.Helper()
	var res struct {
		KifletStatus int `json:"kiflet_status"`
	}
	if code := sandbox(t, http.MethodPost, "/api/phones/"+from+"/texts", map[string]string{"text": body}, &res); code != http.StatusOK {
		t.Fatalf("texting from %s: status %d", from, code)
	}
	return res.KifletStatus
}

// ---------------------------------------------------------------------------
// Mailpit: the inbox of every address
// ---------------------------------------------------------------------------

type email struct {
	ID      string `json:"ID"`
	Subject string `json:"Subject"`
	Text    string `json:"Text"`
}

func mailpit(t *testing.T, path string, out any) {
	t.Helper()
	res, err := newClient(nil).Get(mailURL + path)
	if err != nil {
		t.Fatalf("mailpit %s: %v", path, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("mailpit %s: status %d", path, res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		t.Fatalf("mailpit %s: %v", path, err)
	}
}

// waitForEmail waits for an email to an address whose subject mentions
// what, and returns it with its text.
func waitForEmail(t *testing.T, to, what string) email {
	t.Helper()
	var found email
	within(t, 20*time.Second, "an email to "+to+" about "+what, func() bool {
		var list struct {
			Messages []email `json:"messages"`
		}
		mailpit(t, "/api/v1/search?query="+url.QueryEscape(`to:"`+to+`"`), &list)
		for _, m := range list.Messages {
			if strings.Contains(m.Subject, what) {
				found = m
				return true
			}
		}
		return false
	})
	mailpit(t, "/api/v1/message/"+found.ID, &found)
	return found
}
