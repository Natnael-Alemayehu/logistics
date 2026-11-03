// Package sandbox plays the services Menged connects to: Ethio telecom's
// SMS gateway, telebirr and CBE Birr. Menged reaches it over HTTP with its
// real adapters, as it would reach the providers; only the addresses,
// credentials and keys differ. The sandbox also plays the people on the
// other side, the phones that receive texts and reply and the payers who
// pay or give up, and it can be told to misbehave (faults.go).
//
// Each provider's side is written from the contract, not from Menged's
// adapter: the package imports nothing of Menged's but the console, which a
// test enforces. A mistake on either side then shows up as a refused call
// instead of two copies agreeing. The contracts are at the top of sms.go,
// telebirr.go and cbebirr.go.
package sandbox

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/natnael-alemayehu/logistics/pkg/console"
)

// Config is what the sandbox needs to know. The providers' credentials are
// the ones Menged is given, under the same names (FromEnv), so the two can
// share a file and can't disagree.
type Config struct {
	// AppURL is Menged's address, which the providers call.
	AppURL string
	// AppName is the merchant's name, as payers see it.
	AppName string
	// HostAlias replaces localhost in the addresses the sandbox calls, for
	// a sandbox in a container calling Menged on the host. The addresses
	// browsers are sent to keep localhost.
	HostAlias string
	// PIN unlocks every wallet.
	PIN string

	SMS      SMSConfig
	Telebirr TelebirrConfig
	CBEBirr  CBEBirrConfig

	// Timing paces what the sandbox does by itself; zero values are the
	// defaults.
	Timing Timing
	// Out is the terminal, which shows the texts and lists the calls. nil
	// shows nothing.
	Out io.Writer
	// Client makes the calls to Menged; nil is a client with a 15-second
	// timeout.
	Client *http.Client
}

// SMSConfig is Menged's account with the SMS gateway.
type SMSConfig struct {
	// APIKey signs Menged in, and SenderID is the name its texts come from.
	APIKey   string
	SenderID string
	// Number is the short code phones text Menged on.
	Number string
	// Secret goes with every call to Menged's SMS webhooks.
	Secret string
}

// TelebirrConfig is Menged's telebirr merchant account.
type TelebirrConfig struct {
	FabricAppID   string
	AppSecret     string
	MerchantAppID string
	MerchantCode  string
	// MerchantKey checks Menged's signatures, and Key makes telebirr's.
	MerchantKey *rsa.PublicKey
	Key         *rsa.PrivateKey
}

// CBEBirrConfig is Menged's CBE Birr merchant account.
type CBEBirrConfig struct {
	MerchantID string
	// Username and Password are the login CBE Birr's calls carry.
	Username string
	Password string
}

// Timing is how long things take, shortened by tests.
type Timing struct {
	// Report is how long a text takes to reach its phone, and so when its
	// delivery report is sent: 2 seconds.
	Report time.Duration
	// Retry is the first wait before calling Menged again: 10 seconds,
	// doubled after each failure up to 5 minutes.
	Retry time.Duration
	// Delay is how late the late-notification fault is: a minute.
	Delay time.Duration
	// Slow is how long the slow gateway takes to answer: 35 seconds, longer
	// than Menged waits.
	Slow time.Duration
}

func (t Timing) orDefaults() Timing {
	for _, d := range []struct {
		v   *time.Duration
		def time.Duration
	}{{&t.Report, 2 * time.Second}, {&t.Retry, 10 * time.Second}, {&t.Delay, time.Minute}, {&t.Slow, 35 * time.Second}} {
		if *d.v <= 0 {
			*d.v = d.def
		}
	}
	return t
}

// FromEnv reads the configuration from the environment: the providers'
// credentials from the variables Menged reads, the sandbox's own settings
// from SANDBOX_ ones, and the keys from SANDBOX_KEYS_DIR.
func FromEnv(getenv func(string) string) (Config, error) {
	get := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}
	cfg := Config{
		AppURL:    get("SANDBOX_APP_URL", get("PUBLIC_URL", "http://localhost:"+get("PORT", "8080"))),
		AppName:   get("APP_NAME", "Menged"),
		HostAlias: get("SANDBOX_HOST_ALIAS", ""),
		PIN:       get("SANDBOX_PIN", "1234"),
		SMS: SMSConfig{
			APIKey: get("ETHIO_TELECOM_API_KEY", ""), SenderID: get("ETHIO_TELECOM_SENDER_ID", ""),
			Number: get("SMS_NUMBER", "6060"), Secret: get("SMS_INBOUND_SECRET", ""),
		},
		Telebirr: TelebirrConfig{
			FabricAppID: get("TELEBIRR_FABRIC_APP_ID", ""), AppSecret: get("TELEBIRR_APP_SECRET", ""),
			MerchantAppID: get("TELEBIRR_MERCHANT_APP_ID", ""), MerchantCode: get("TELEBIRR_MERCHANT_CODE", ""),
		},
		CBEBirr: CBEBirrConfig{
			MerchantID: get("CBEBIRR_MERCHANT_ID", ""),
			Username:   get("CBEBIRR_CALLBACK_USERNAME", ""), Password: get("CBEBIRR_CALLBACK_PASSWORD", ""),
		},
	}
	var err error
	cfg.Telebirr.MerchantKey, cfg.Telebirr.Key, err = LoadKeys(get("SANDBOX_KEYS_DIR", "sandbox-keys"))
	return cfg, err
}

// missing names the settings the configuration lacks.
func (c Config) missing() []string {
	var out []string
	for name, set := range map[string]bool{
		"SANDBOX_APP_URL":           c.AppURL != "",
		"ETHIO_TELECOM_API_KEY":     c.SMS.APIKey != "",
		"ETHIO_TELECOM_SENDER_ID":   c.SMS.SenderID != "",
		"SMS_INBOUND_SECRET":        c.SMS.Secret != "",
		"TELEBIRR_FABRIC_APP_ID":    c.Telebirr.FabricAppID != "",
		"TELEBIRR_APP_SECRET":       c.Telebirr.AppSecret != "",
		"TELEBIRR_MERCHANT_APP_ID":  c.Telebirr.MerchantAppID != "",
		"TELEBIRR_MERCHANT_CODE":    c.Telebirr.MerchantCode != "",
		"the merchant's public key": c.Telebirr.MerchantKey != nil,
		"telebirr's private key":    c.Telebirr.Key != nil,
		"CBEBIRR_MERCHANT_ID":       c.CBEBirr.MerchantID != "",
		"CBEBIRR_CALLBACK_USERNAME": c.CBEBirr.Username != "",
		"CBEBIRR_CALLBACK_PASSWORD": c.CBEBirr.Password != "",
	} {
		if !set {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// Sandbox is the providers' side of Menged's connections, and the people's.
// Everything it knows is kept in memory, and forgotten when it stops.
type Sandbox struct {
	cfg      Config
	client   *http.Client
	out      io.Writer
	terminal *console.Console
	pages    map[string]*template.Template
	// forger signs the notifications of the wrong-key fault.
	forger *rsa.PrivateKey

	// ctx ends the calls waiting to be made when the sandbox closes.
	ctx    context.Context
	cancel context.CancelFunc
	jobs   sync.WaitGroup
	jobsMu sync.Mutex
	closed bool

	mu        sync.Mutex
	faults    Faults
	phones    map[string]*phone // by local number
	orders    map[string]*order // telebirr's, by prepay_id
	orderIDs  map[string]*order // the same, by merch_order_id
	payments  []*cbePayment     // CBE Birr's, oldest first
	smsTokens map[string]time.Time
	tbTokens  map[string]time.Time
	nonces    map[string]time.Time // telebirr's nonce_str values lately used
	held      []*call
	events    []Event
	eventSeq  int
	callSeq   int
}

// New checks the configuration and returns a sandbox with no phones, no
// orders and no faults.
func New(cfg Config) (*Sandbox, error) {
	if missing := cfg.missing(); len(missing) > 0 {
		return nil, fmt.Errorf("sandbox: missing %s", strings.Join(missing, ", "))
	}
	cfg.AppURL = strings.TrimRight(cfg.AppURL, "/")
	if u, err := url.Parse(cfg.AppURL); err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("sandbox: Menged's address %q is not an http or https URL", cfg.AppURL)
	}
	if cfg.AppName == "" {
		cfg.AppName = "Menged"
	}
	if cfg.PIN == "" {
		cfg.PIN = "1234"
	}
	if cfg.SMS.Number == "" {
		cfg.SMS.Number = "6060"
	}
	cfg.Timing = cfg.Timing.orDefaults()
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	forger, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	pages, err := parsePages()
	if err != nil {
		return nil, err
	}
	var out io.Writer
	if cfg.Out != nil {
		out = &lockedWriter{w: cfg.Out}
	}
	s := &Sandbox{
		cfg: cfg, client: client, out: out, terminal: console.New(out), pages: pages, forger: forger,
		phones: map[string]*phone{}, orders: map[string]*order{}, orderIDs: map[string]*order{},
		smsTokens: map[string]time.Time{}, tbTokens: map[string]time.Time{}, nonces: map[string]time.Time{},
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	return s, nil
}

// Close stops the calls waiting to be made and waits for those under way.
func (s *Sandbox) Close() {
	s.jobsMu.Lock()
	s.closed = true
	s.jobsMu.Unlock()
	s.cancel()
	s.jobs.Wait()
}

// Handler serves the providers' APIs, the pages people use (ui.go) and a
// JSON API for tests and scripts (api.go).
func (s *Sandbox) Handler() http.Handler {
	mux := http.NewServeMux()

	// The providers, as Menged calls them.
	mux.Handle("POST /ethiotelecom/auth/token", s.provider("SMS", s.smsToken))
	mux.Handle("POST /ethiotelecom/sms/send", s.provider("SMS", s.smsSend))
	mux.Handle("POST /telebirr/payment/v1/token", s.provider("telebirr", s.telebirrToken))
	mux.Handle("POST /telebirr/payment/v1/merchant/preOrder", s.provider("telebirr", s.preOrder))
	mux.Handle("POST /telebirr/payment/v1/merchant/queryOrder", s.provider("telebirr", s.queryOrder))
	mux.Handle("GET /telebirr/checkout", s.provider("telebirr", s.checkout))
	mux.HandleFunc("POST /telebirr/checkout/{prepay}", s.checkoutAnswer)

	// The people.
	mux.HandleFunc("GET /{$}", s.homePage)
	mux.HandleFunc("GET /phones", s.openPhone)
	mux.HandleFunc("GET /phones/{number}", s.phonePage)
	mux.HandleFunc("POST /phones/{number}/texts", s.phoneText)
	mux.HandleFunc("POST /phones/{number}/balances", s.phoneBalances)
	mux.HandleFunc("POST /phones/{number}/cbebirr", s.phoneCBEBirr)
	mux.HandleFunc("GET /controls", s.controlsPage)
	mux.HandleFunc("POST /controls", s.setControls)
	mux.HandleFunc("POST /controls/tokens", s.expireTokensPage)
	mux.HandleFunc("POST /held/{id}", s.heldAction)
	mux.HandleFunc("GET /log", s.logPage)
	mux.Handle("GET /static/", staticFiles())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "ok\n")
	})

	s.api(mux)
	// The pages set no cookies, but a page elsewhere could still post to
	// them from the developer's browser: cross-origin posts are refused.
	return secured(http.NewCrossOriginProtection().Handler(mux))
}

// secured sets the headers of every answer. The pages load nothing from
// elsewhere and run no inline script. form-action is left open: the
// checkout's form ends at whatever page the merchant asked for.
func secured(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; frame-ancestors 'none'; base-uri 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// currentFaults is a copy of the faults now set.
func (s *Sandbox) currentFaults() Faults {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.faults.copy()
}

// lockedWriter lets the console and the call log share the terminal.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// ---------------------------------------------------------------------------
// Money, numbers and names
// ---------------------------------------------------------------------------

// birr writes santim as birr for people: 287500 is 2,875.00.
func birr(santim int64) string {
	sign := ""
	if santim < 0 {
		sign, santim = "-", -santim
	}
	whole := strconv.FormatInt(santim/100, 10)
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "," + whole[i:]
	}
	return fmt.Sprintf("%s%s.%02d", sign, whole, santim%100)
}

// decimal writes santim as the providers' calls carry amounts: 287500 is
// 2875.00.
func decimal(santim int64) string { return fmt.Sprintf("%d.%02d", santim/100, santim%100) }

var errNotAmount = errors.New("not an amount in birr with at most two decimals")

// parseAmount reads an amount as the providers' calls carry it, such as
// 2875.00 or 2875, as santim.
func parseAmount(s string) (int64, error) {
	whole, frac, dot := strings.Cut(s, ".")
	if !allDigits(whole) || len(whole) > 12 || len(frac) > 2 || (dot && !allDigits(frac)) {
		return 0, errNotAmount
	}
	frac += strings.Repeat("0", 2-len(frac))
	w, _ := strconv.ParseInt(whole, 10, 64)
	f, _ := strconv.ParseInt(frac, 10, 64)
	return w*100 + f, nil
}

// parseBirr reads an amount people typed, which may group the thousands.
func parseBirr(s string) (int64, error) {
	return parseAmount(strings.ReplaceAll(strings.TrimSpace(s), ",", ""))
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// localNumber reads an Ethiopian mobile number in any of the usual forms,
// 0911223344, 911223344, +251911223344 or 251911223344, as the local one.
func localNumber(s string) (string, bool) {
	d := strings.TrimPrefix(strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' {
			return -1
		}
		return r
	}, strings.TrimSpace(s)), "+")
	switch {
	case len(d) == 12 && strings.HasPrefix(d, "251"):
		d = "0" + d[3:]
	case len(d) == 9:
		d = "0" + d
	}
	if len(d) != 10 || !allDigits(d) || d[0] != '0' || (d[1] != '9' && d[1] != '7') {
		return "", false
	}
	return d, true
}

// e164 is a local number in international form: 0911223344 is
// +251911223344.
func e164(local string) string { return "+251" + local[1:] }

// msisdn is a local number as CBE Birr writes it: 251911223344.
func msisdn(local string) string { return "251" + local[1:] }

const (
	digitChars = "0123456789"
	upperChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" + digitChars
)

// random is n characters drawn from chars. The sandbox's IDs are random, not
// counted, so that one restarted doesn't hand Menged an ID it has seen.
func random(chars string, n int) string {
	b := make([]byte, n)
	for i := range b {
		k, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			panic(fmt.Sprintf("crypto/rand: %v", err))
		}
		b[i] = chars[k.Int64()]
	}
	return string(b)
}

// clip shortens text for a summary.
func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

// ---------------------------------------------------------------------------
// JSON
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// readJSON reads a JSON body into v, answering 400 when it can't.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the body is not the JSON expected: " + err.Error()})
		return false
	}
	return true
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
