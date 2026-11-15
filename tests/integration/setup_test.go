//go:build integration

// Package integration drives the whole application — HTTP handlers, services
// and a real PostGIS database — the way browsers, the driver app and the SMS
// provider do.
//
// Set TEST_DATABASE_URL to use a running server (a scratch database is
// created on it and dropped afterwards); otherwise a PostGIS container is
// started with testcontainers.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/natnael-alemayehu/kiflet/internal/app"
	"github.com/natnael-alemayehu/kiflet/internal/config"
	"github.com/natnael-alemayehu/kiflet/internal/service"
	"github.com/natnael-alemayehu/kiflet/migrations"
	"github.com/natnael-alemayehu/kiflet/pkg/console"
)

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()
	dsn, cleanup, err := database(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration database:", err)
		os.Exit(1)
	}
	code := func() int {
		defer cleanup()
		if pool, err = pgxpool.New(ctx, dsn); err != nil {
			fmt.Fprintln(os.Stderr, "connect:", err)
			return 1
		}
		defer pool.Close()
		defer func() { stopObjectStore() }()
		if err := migrate(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "migrate:", err)
			return 1
		}
		return m.Run()
	}()
	os.Exit(code)
}

// database returns a DSN for an empty database and a function that removes it.
func database(ctx context.Context) (string, func(), error) {
	if base := os.Getenv("TEST_DATABASE_URL"); base != "" {
		admin, err := pgx.Connect(ctx, base)
		if err != nil {
			return "", nil, err
		}
		name := fmt.Sprintf("kiflet_it_%d", time.Now().UnixNano())
		if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
			admin.Close(ctx)
			return "", nil, err
		}
		u, err := url.Parse(base)
		if err != nil {
			return "", nil, err
		}
		u.Path = "/" + name
		return u.String(), func() {
			if pool != nil {
				pool.Close()
			}
			_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
			admin.Close(ctx)
		}, nil
	}

	c, err := postgres.Run(ctx, "postgis/postgis:15-3.3-alpine",
		postgres.WithDatabase("kiflet"), postgres.WithUsername("kiflet"), postgres.WithPassword("kiflet"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(2*time.Minute)),
	)
	if err != nil {
		return "", nil, err
	}
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return "", nil, err
	}
	return dsn, func() { _ = c.Terminate(ctx) }, nil
}

func migrate(ctx context.Context) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.UpContext(ctx, db, ".")
}

// The stand-in CBE Birr's merchant ID, and the login its servers use.
const (
	cbeMerchant = "100100"
	cbeUser     = "cbebirr"
	cbePassword = "integration-cbebirr-password"
)

// env is one running copy of the app.
type env struct {
	t     *testing.T
	srv   *httptest.Server
	app   *app.App
	svc   *service.Services
	stop  func() // stops the background workers, once they finish what they hold
	inbox *console.Console
	// texts, when set, are the texts a phone has received, for an app whose
	// texts go to phones other than the inbox's.
	texts func(phone string) []console.Item
	seen  map[string]time.Time // newest message received, by kind and recipient
	files string               // where uploaded images are stored
}

// setup is what a test can change before the app starts: its settings, and
// where its texts can be read. base is the app's address.
type setup struct {
	base  string
	cfg   *config.Config
	texts func(phone string) []console.Item
}

// newEnv starts the app as the server command does, with the stand-in
// providers. They print what they would send to the inbox, which the tests
// read as a developer reads the terminal; with SMS a stand-in, the SMS
// webhooks take no secret, as in development. The periodic jobs run only
// when a test calls them, as every app shares the database.
func newEnv(t *testing.T, options ...func(*setup)) *env {
	t.Helper()
	ts := httptest.NewUnstartedServer(nil)
	base := "http://" + ts.Listener.Addr().String()
	tz, _ := time.LoadLocation("Africa/Addis_Ababa")
	cfg := &config.Config{
		Port: "0", PublicURL: base, AppName: "Kiflet", Timezone: tz, Environment: "test",
		Secret: []byte("integration-test-secret-0123456789abcdef"), SMSProvider: "mock", SMSNumber: "6060", MailProvider: "mock",
		StorageType: "local", StoragePath: t.TempDir(), MetricsToken: metricsToken, PaymentProvider: "mock",
		CBEBirr:      config.CBEBirrSettings{MerchantID: cbeMerchant, CallbackUsername: cbeUser, CallbackPassword: cbePassword},
		SupportEmail: "support@kiflet.test", BankDetails: "Test Bank, account 123",
	}
	s := &setup{base: base, cfg: cfg}
	for _, o := range options {
		o(s)
	}
	log := zerolog.Nop()
	if os.Getenv("TEST_LOG") != "" { // the server's log, to see why a request failed
		log = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Logger()
	}
	inbox := console.New(nil)
	a, err := app.New(cfg, pool, log, app.Options{Console: inbox, NoJobs: true})
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, srv: ts, app: a, svc: a.Services, inbox: inbox, texts: s.texts, seen: map[string]time.Time{}, files: cfg.StoragePath}
	// Cleanups run last first: the server stops, then the workers finish
	// what they hold, before the next test starts.
	e.work()
	t.Cleanup(func() { e.stop() })
	ts.Config.Handler = a.Handler()
	ts.Start()
	t.Cleanup(ts.Close)
	return e
}

// work starts the app's background workers, as the server does on start.
func (e *env) work() {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		e.app.Run(ctx)
		close(done)
	}()
	e.stop = func() {
		cancel()
		<-done
	}
}

// received waits for an SMS or email to a recipient that mentions text and
// is newer than the last one this test received, and returns it.
func (e *env) received(kind, to, text string) console.Item {
	e.t.Helper()
	key := kind + " " + to
	deadline := time.Now().Add(5 * time.Second)
	for {
		items := e.inbox.Items()
		if kind == "sms" && e.texts != nil {
			items = e.texts(to)
		}
		for _, it := range items {
			if it.Kind == kind && it.To == to && it.At.After(e.seen[key]) && strings.Contains(it.Title+"\n"+it.Body, text) {
				e.seen[key] = it.At
				return it
			}
		}
		if time.Now().After(deadline) {
			e.t.Fatalf("no %s to %s mentioning %q", kind, to, text)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

var codeRe = regexp.MustCompile(`\b\d{6}\b`)

// code waits for an SMS with a one-time code and returns the code.
func (e *env) code(phone string) string {
	e.t.Helper()
	m := codeRe.FindString(e.received("sms", phone, "code").Body)
	if m == "" {
		e.t.Fatalf("no code in the SMS to %s", phone)
	}
	return m
}

// ---------------------------------------------------------------------------
// Browser sessions
// ---------------------------------------------------------------------------

type browser struct {
	e *env
	c *http.Client
	// email and phone are the signed-up admin's, for a company's browser.
	email, phone string
}

func (e *env) browser() *browser {
	jar, _ := cookiejar.New(nil)
	return &browser{e: e, c: &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

type page struct {
	Status   int
	Location string
	Body     string
}

func (b *browser) do(req *http.Request) page {
	b.e.t.Helper()
	res, err := b.c.Do(req)
	if err != nil {
		b.e.t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return page{Status: res.StatusCode, Location: res.Header.Get("Location"), Body: string(body)}
}

func (b *browser) get(path string) page {
	req, _ := http.NewRequest(http.MethodGet, b.e.srv.URL+path, nil)
	return b.do(req)
}

func (b *browser) post(path string, form url.Values) page {
	req, _ := http.NewRequest(http.MethodPost, b.e.srv.URL+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return b.do(req)
}

// mustPost posts a form and fails unless the app redirects (its success
// response for every action).
func (b *browser) mustPost(path string, form url.Values) page {
	b.e.t.Helper()
	p := b.post(path, form)
	if p.Status != http.StatusSeeOther {
		b.e.t.Fatalf("POST %s: status %d, want 303\n%s", path, p.Status, errorText(p.Body))
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

// company signs up a new company, confirming the phone with the code texted
// to it, and returns its administrator's session.
func (e *env) company(name string) *browser {
	e.t.Helper()
	b := e.browser()
	b.email = strings.ToLower(strings.ReplaceAll(name, " ", "")) + fmt.Sprintf("%d@example.et", rand.IntN(1e9))
	b.phone = phoneNumber()
	p := b.mustPost("/signup", url.Values{
		"company": {name}, "city": {"Addis Ababa"}, "name": {"Owner of " + name},
		"email": {b.email}, "phone": {b.phone}, "password": {"correct-horse-1"},
	})
	if p.Location != "/signup/verify" {
		e.t.Fatalf("signup redirected to %q", p.Location)
	}
	if p = b.mustPost("/signup/verify", url.Values{"code": {e.code(b.phone)}}); p.Location != "/app" {
		e.t.Fatalf("signup code redirected to %q", p.Location)
	}
	return b
}

var secretRe = regexp.MustCompile(`data-secret>([^<]+)<`)

// addDriver creates a driver and returns their id, phone and first PIN.
func (b *browser) addDriver(name string) (id, phone, pin string) {
	b.e.t.Helper()
	phone = phoneNumber()
	p := b.post("/app/drivers", url.Values{"full_name": {name}, "phone": {phone}})
	m := secretRe.FindStringSubmatch(p.Body)
	if p.Status != http.StatusOK || m == nil {
		b.e.t.Fatalf("add driver: status %d %s", p.Status, errorText(p.Body))
	}
	if err := pool.QueryRow(context.Background(), `SELECT id::text FROM users WHERE phone = $1`, phone).Scan(&id); err != nil {
		b.e.t.Fatal(err)
	}
	return id, phone, strings.TrimSpace(m[1])
}

// book creates a shipment and returns its id and tracking number.
func (b *browser) book(from, to, driverID string) (id, tracking string) {
	b.e.t.Helper()
	return b.bookFor(from, to, driverID, phoneNumber())
}

// bookFor books a shipment for the customer with this phone number.
func (b *browser) bookFor(from, to, driverID, customerPhone string) (id, tracking string) {
	b.e.t.Helper()
	p := b.mustPost("/app/shipments", url.Values{
		"priority": {"normal"}, "origin_city": {from}, "origin_address": {"Main depot"},
		"destination_city": {to}, "destination_address": {"Customer warehouse"},
		"customer_name": {"Abyssinia Traders"}, "customer_phone": {customerPhone},
		"cargo_description": {"Cement, 200 bags"}, "cargo_weight": {"10000"}, "driver_id": {driverID},
	})
	id = strings.TrimPrefix(p.Location, "/app/shipments/")
	if err := pool.QueryRow(context.Background(), `SELECT tracking_number FROM shipments WHERE id = $1`, id).Scan(&tracking); err != nil {
		b.e.t.Fatalf("booked shipment %q: %v", p.Location, err)
	}
	return id, tracking
}

// ---------------------------------------------------------------------------
// The driver app
// ---------------------------------------------------------------------------

type driverApp struct {
	e      *env
	token  string
	device string
}

func (e *env) api(method, path, token string, body, out any) int {
	e.t.Helper()
	var r io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		r = bytes.NewReader(buf)
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, r)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			e.t.Fatalf("%s %s: decode %q: %v", method, path, raw, err)
		}
	}
	return res.StatusCode
}

// signIn signs a driver in on a new phone and sets their own PIN.
func (e *env) signIn(phone, tempPIN string) *driverApp {
	e.t.Helper()
	d := &driverApp{e: e, device: fmt.Sprintf("device-%d", rand.IntN(1e9))}
	var login struct {
		Token         string `json:"token"`
		MustChangePIN bool   `json:"must_change_pin"`
	}
	if code := e.api("POST", "/api/v1/driver/login", "", map[string]string{"phone": phone, "pin": tempPIN, "device_id": d.device}, &login); code != http.StatusOK {
		e.t.Fatalf("driver login: status %d", code)
	}
	if !login.MustChangePIN {
		e.t.Error("a new driver should have to choose their own PIN")
	}
	d.token = login.Token
	if code := e.api("POST", "/api/v1/driver/pin", d.token, map[string]string{"current_pin": tempPIN, "new_pin": "4826"}, nil); code != http.StatusOK {
		e.t.Fatalf("change PIN: status %d", code)
	}
	return d
}

func (d *driverApp) sync(req service.SyncRequest) (service.SyncResponse, int) {
	d.e.t.Helper()
	req.DeviceID = d.device
	var resp service.SyncResponse
	code := d.e.api("POST", "/api/v1/driver/sync", d.token, req, &resp)
	return resp, code
}

// status reports one status change made now and returns the server's
// verdict on it.
func (d *driverApp) status(clientID, shipmentID, status string) service.SyncResult {
	d.e.t.Helper()
	return d.statusAt(clientID, shipmentID, status, time.Now())
}

// statusAt reports a status change the driver made at a given time, as a
// phone sending its outbox after being offline does.
func (d *driverApp) statusAt(clientID, shipmentID, status string, at time.Time) service.SyncResult {
	d.e.t.Helper()
	resp, code := d.sync(service.SyncRequest{Statuses: []service.SyncStatus{{
		ClientID: clientID, ShipmentID: shipmentID, Status: status, RecordedAt: at,
	}}})
	if code != http.StatusOK || len(resp.Results) != 1 {
		d.e.t.Fatalf("sync %s: status %d, results %+v", status, code, resp.Results)
	}
	return resp.Results[0]
}

// pod sends a proof of delivery and returns the server's verdict on it.
func (d *driverApp) pod(p service.SyncPOD) service.SyncResult {
	d.e.t.Helper()
	if p.ClientID == "" {
		p.ClientID = clientID()
	}
	if p.Signature == "" {
		p.Signature = signaturePNG
	}
	if p.RecordedAt.IsZero() {
		p.RecordedAt = time.Now()
	}
	resp, code := d.sync(service.SyncRequest{PODs: []service.SyncPOD{p}})
	if code != http.StatusOK || len(resp.Results) != 1 {
		d.e.t.Fatalf("sync POD: status %d, results %+v", code, resp.Results)
	}
	return resp.Results[0]
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func phoneNumber() string { return fmt.Sprintf("091%07d", rand.IntN(1e7)) }

func clientID() string { return fmt.Sprintf("c-%d", rand.Int64()) }

func scalar[T any](t *testing.T, sql string, args ...any) T {
	t.Helper()
	var v T
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

// signaturePNG is a valid 1×1 PNG as a data URL.
const signaturePNG = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
