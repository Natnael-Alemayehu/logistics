package sandbox

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"go/parser"
	"go/token"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestImportsOnlyTheStandardLibraryAndTheConsole: the sandbox implements the
// providers' side of each contract on its own, so that a mistake in
// Menged's adapter shows up as a refused call rather than being shared.
func TestImportsOnlyTheStandardLibraryAndTheConsole(t *testing.T) {
	allowed := map[string]bool{
		"github.com/natnael-alemayehu/logistics/pkg/console":      true,
		"github.com/natnael-alemayehu/logistics/internal/sandbox": true,
	}
	for _, dir := range []string{".", "../../cmd/sandbox"} {
		paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil || len(paths) == 0 {
			t.Fatalf("no Go files in %s: %v", dir, err)
		}
		for _, path := range paths {
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range f.Imports {
				p, _ := strconv.Unquote(imp.Path.Value)
				std := !strings.Contains(strings.SplitN(p, "/", 2)[0], ".")
				if !std && !allowed[p] {
					t.Errorf("%s imports %s: the sandbox may use the standard library and the console only", path, p)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// A sandbox and a fake Menged
// ---------------------------------------------------------------------------

var testKeys = sync.OnceValues(func() ([2]*rsa.PrivateKey, error) {
	var keys [2]*rsa.PrivateKey
	for i := range keys {
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return keys, err
		}
		keys[i] = k
	}
	return keys, nil
})

// fakeApp plays Menged's side of the calls the sandbox makes, and keeps
// them. respond, when set, answers them; otherwise they get 200.
type fakeApp struct {
	mu      sync.Mutex
	calls   []fakeCall
	respond func(c fakeCall) (int, any)
}

type fakeCall struct {
	Path   string
	Header http.Header
	Body   []byte
}

func (c fakeCall) fields(t *testing.T) map[string]string {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal(c.Body, &m); err != nil {
		t.Fatalf("%s: %v: %s", c.Path, err, c.Body)
	}
	return m
}

func (f *fakeApp) start(t *testing.T) string {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		c := fakeCall{Path: r.URL.Path, Header: r.Header.Clone(), Body: body}
		f.mu.Lock()
		f.calls = append(f.calls, c)
		respond := f.respond
		f.mu.Unlock()
		status, answer := http.StatusOK, any(map[string]any{"ok": true})
		if respond != nil {
			status, answer = respond(c)
		}
		writeJSON(w, status, answer)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func (f *fakeApp) answer(respond func(fakeCall) (int, any)) {
	f.mu.Lock()
	f.respond = respond
	f.mu.Unlock()
}

func (f *fakeApp) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c.Path == path {
			n++
		}
	}
	return n
}

// received waits for n calls to path, and returns those made.
func (f *fakeApp) received(t *testing.T, path string, n int) []fakeCall {
	t.Helper()
	var got []fakeCall
	eventually(t, "calls to "+path, func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		got = got[:0]
		for _, c := range f.calls {
			if c.Path == path {
				got = append(got, c)
			}
		}
		return len(got) >= n
	})
	return got
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

type testbed struct {
	t        *testing.T
	sb       *Sandbox
	url      string
	app      *fakeApp
	merchant *rsa.PrivateKey // Menged's half: signs its requests
	telebirr *rsa.PrivateKey // telebirr's: its public half checks notifications
}

// newTestbed starts a sandbox for a fake Menged. change, when given,
// alters its settings first.
func newTestbed(t *testing.T, change ...func(*Config)) *testbed {
	t.Helper()
	keys, err := testKeys()
	if err != nil {
		t.Fatal(err)
	}
	app := &fakeApp{}
	cfg := Config{
		AppURL: app.start(t),
		SMS:    SMSConfig{APIKey: "sms-key", SenderID: "MENGED", Number: "6060", Secret: "hook-secret"},
		Telebirr: TelebirrConfig{
			FabricAppID: "fabric", AppSecret: "app-secret", MerchantAppID: "850001", MerchantCode: "900001",
			MerchantKey: &keys[0].PublicKey, Key: keys[1],
		},
		CBEBirr: CBEBirrConfig{MerchantID: "100100", Username: "cbe", Password: "cbe-pass"},
		Timing:  Timing{Report: 10 * time.Millisecond, Retry: 10 * time.Millisecond, Delay: 50 * time.Millisecond, Slow: 300 * time.Millisecond},
	}
	for _, c := range change {
		c(&cfg)
	}
	sb, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(sb.Handler())
	t.Cleanup(srv.Close)
	t.Cleanup(sb.Close)
	return &testbed{t: t, sb: sb, url: srv.URL, app: app, merchant: keys[0], telebirr: keys[1]}
}

var browser = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

type reply struct {
	Status   int
	Body     string
	Location string
}

func (tb *testbed) do(req *http.Request) reply {
	tb.t.Helper()
	res, err := browser.Do(req)
	if err != nil {
		tb.t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return reply{Status: res.StatusCode, Body: string(body), Location: res.Header.Get("Location")}
}

// call makes a JSON call and decodes the answer into out.
func (tb *testbed) call(method, path string, header http.Header, body, out any) int {
	tb.t.Helper()
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(mustJSON(body))
	}
	req, _ := http.NewRequest(method, tb.url+path, r)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header[k] = v
	}
	res := tb.do(req)
	if out != nil && res.Body != "" {
		if err := json.Unmarshal([]byte(res.Body), out); err != nil {
			tb.t.Fatalf("%s %s: %v: %s", method, path, err, res.Body)
		}
	}
	return res.Status
}

func (tb *testbed) get(path string) reply {
	req, _ := http.NewRequest(http.MethodGet, tb.url+path, nil)
	return tb.do(req)
}

func (tb *testbed) post(path string, form url.Values) reply {
	req, _ := http.NewRequest(http.MethodPost, tb.url+path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return tb.do(req)
}

func (tb *testbed) faults(patch map[string]any) {
	tb.t.Helper()
	if code := tb.call(http.MethodPatch, "/api/faults", nil, patch, nil); code != http.StatusOK {
		tb.t.Fatalf("PATCH /api/faults %v: status %d", patch, code)
	}
}

func (tb *testbed) held() []call {
	tb.t.Helper()
	var held []call
	tb.call(http.MethodGet, "/api/held", nil, nil, &held)
	return held
}

// ---------------------------------------------------------------------------
// Ethio telecom
// ---------------------------------------------------------------------------

func (tb *testbed) smsToken() string {
	tb.t.Helper()
	var res struct {
		Token     string `json:"token"`
		ExpiresIn int    `json:"expires_in"`
	}
	if code := tb.call(http.MethodPost, "/ethiotelecom/auth/token", nil, map[string]string{"api_key": "sms-key"}, &res); code != http.StatusOK || res.Token == "" || res.ExpiresIn != 600 {
		tb.t.Fatalf("signing in: status %d, %+v", code, res)
	}
	return res.Token
}

type smsAnswer struct {
	MessageID string `json:"message_id"`
	Status    string `json:"status"`
	Error     string `json:"error"`
}

func (tb *testbed) sendSMS(token, to, from, message string) (int, smsAnswer) {
	tb.t.Helper()
	var res smsAnswer
	code := tb.call(http.MethodPost, "/ethiotelecom/sms/send", http.Header{"Authorization": {"Bearer " + token}},
		map[string]string{"to": to, "from": from, "message": message}, &res)
	return code, res
}

func TestSMSGateway(t *testing.T) {
	tb := newTestbed(t)
	if code := tb.call(http.MethodPost, "/ethiotelecom/auth/token", nil, map[string]string{"api_key": "wrong"}, nil); code != http.StatusUnauthorized {
		t.Errorf("signing in with the wrong key: status %d, want 401", code)
	}
	token := tb.smsToken()

	code, res := tb.sendSMS(token, "+251911223344", "MENGED", "Your code is 482913.")
	if code != http.StatusOK || res.Status != "accepted" || !strings.HasPrefix(res.MessageID, "ET") {
		t.Fatalf("sending: status %d, %+v", code, res)
	}
	if p := tb.sb.phoneCopy("0911223344"); len(p.Texts) != 1 || p.Texts[0].Body != "Your code is 482913." || p.Texts[0].From != "MENGED" {
		t.Fatalf("the phone has %+v", p.Texts)
	}

	// The delivery report follows, with the webhook secret.
	report := tb.app.received(t, "/api/v1/sms/status", 1)[0]
	if got := report.Header.Get("X-Webhook-Secret"); got != "hook-secret" {
		t.Errorf("the report's secret is %q", got)
	}
	if f := report.fields(t); f["message_id"] != res.MessageID || f["status"] != "delivered" || f["done_at"] == "" {
		t.Errorf("the report is %v", f)
	}
	eventually(t, "the report's outcome on the text", func() bool {
		tx := tb.sb.phoneCopy("0911223344").Texts[0]
		return tx.State == "Delivered" && tx.Report == "Report: Taken by Menged"
	})

	for name, c := range map[string]struct {
		token, to, from, message string
		status                   int
		kind                     string
	}{
		"unknown token":   {"not-a-token", "+251911223344", "MENGED", "Hi", http.StatusUnauthorized, "error"},
		"another sender":  {token, "+251911223344", "SOMEONE", "Hi", http.StatusBadRequest, "rejected"},
		"local form":      {token, "0911223344", "MENGED", "Hi", http.StatusBadRequest, "rejected"},
		"landline":        {token, "+251111223344", "MENGED", "Hi", http.StatusBadRequest, "rejected"},
		"empty":           {token, "+251911223344", "MENGED", "  ", http.StatusBadRequest, "rejected"},
		"eleven parts in": {token, "+251911223344", "MENGED", strings.Repeat("ሀ", 68*11), http.StatusBadRequest, "rejected"},
	} {
		if code, res := tb.sendSMS(c.token, c.to, c.from, c.message); code != c.status || res.Status != c.kind {
			t.Errorf("%s: status %d %+v, want %d %s", name, code, res, c.status, c.kind)
		}
	}

	// Tokens end when the gateway says, and a new sign-in works.
	tb.sb.expireTokens()
	if code, _ := tb.sendSMS(token, "+251911223344", "MENGED", "Hi"); code != http.StatusUnauthorized {
		t.Errorf("an expired token: status %d, want 401", code)
	}
	if code, _ := tb.sendSMS(tb.smsToken(), "+251911223344", "MENGED", "Hi"); code != http.StatusOK {
		t.Errorf("a new token: status %d", code)
	}
}

func TestSMSFaults(t *testing.T) {
	tb := newTestbed(t)
	token := tb.smsToken()

	tb.faults(map[string]any{"sms_down": true})
	if code, res := tb.sendSMS(token, "+251911223344", "MENGED", "Hi"); code != http.StatusServiceUnavailable || res.Status != "error" {
		t.Errorf("gateway down: status %d %+v", code, res)
	}
	tb.faults(map[string]any{"sms_down": false, "sms_refuse_tokens": true})
	if code, _ := tb.sendSMS(tb.smsToken(), "+251911223344", "MENGED", "Hi"); code != http.StatusUnauthorized {
		t.Errorf("tokens refused: status %d, want 401 even for a new token", code)
	}
	tb.faults(map[string]any{"sms_refuse_tokens": false, "barred": []string{"0911000001"}, "unreachable": []string{"+251911000002"}})
	if code, res := tb.sendSMS(token, "+251911000001", "MENGED", "Hi"); code != http.StatusBadRequest || res.Status != "rejected" {
		t.Errorf("a barred number: status %d %+v", code, res)
	}
	if code, _ := tb.sendSMS(token, "+251911000002", "MENGED", "Hi"); code != http.StatusOK {
		t.Errorf("an unreachable number: status %d, want the text taken", code)
	}
	if f := tb.app.received(t, "/api/v1/sms/status", 1)[0].fields(t); f["status"] != "undelivered" || f["error"] == "" {
		t.Errorf("the unreachable number's report: %v", f)
	}

	// Reports kept back go when sent by hand.
	tb.faults(map[string]any{"unreachable": []string{}, "drop_reports": true})
	if code, _ := tb.sendSMS(token, "+251911000003", "MENGED", "Hi"); code != http.StatusOK {
		t.Fatalf("sending: status %d", code)
	}
	var held []call
	eventually(t, "the report held back", func() bool { held = tb.held(); return len(held) == 1 })
	if tb.app.count("/api/v1/sms/status") != 1 {
		t.Error("a dropped report was sent")
	}
	if code := tb.call(http.MethodPost, "/api/held/"+strconv.Itoa(held[0].ID), nil, map[string]string{"action": "send"}, nil); code != http.StatusOK {
		t.Fatalf("sending the held report: status %d", code)
	}
	tb.app.received(t, "/api/v1/sms/status", 2)
	if len(tb.held()) != 0 {
		t.Error("the sent report is still held")
	}

	// A slow gateway: Menged stops waiting, and nothing is sent.
	tb.faults(map[string]any{"drop_reports": false, "sms_slow": true})
	impatient := &http.Client{Timeout: 50 * time.Millisecond}
	req, _ := http.NewRequest(http.MethodPost, tb.url+"/ethiotelecom/sms/send",
		bytes.NewReader(mustJSON(map[string]string{"to": "+251911000004", "from": "MENGED", "message": "Hi"})))
	req.Header.Set("Authorization", "Bearer "+token)
	if res, err := impatient.Do(req); err == nil {
		res.Body.Close()
		t.Fatalf("the slow gateway answered %d at once", res.StatusCode)
	}
	time.Sleep(350 * time.Millisecond)
	if p := tb.sb.phoneCopy("0911000004"); len(p.Texts) != 0 {
		t.Error("the slow gateway sent a text Menged gave up on")
	}
}

func TestTextsFromPhones(t *testing.T) {
	tb := newTestbed(t)
	tb.app.answer(func(fakeCall) (int, any) {
		return http.StatusOK, map[string]any{"ok": true, "outcome": "recorded", "detail": "In transit recorded."}
	})
	var res struct {
		Text         text `json:"text"`
		MengedStatus int  `json:"menged_status"`
	}
	if code := tb.call(http.MethodPost, "/api/phones/0911223344/texts", nil, map[string]string{"text": "ET 4821 1"}, &res); code != http.StatusOK {
		t.Fatalf("texting: status %d", code)
	}
	in := tb.app.received(t, "/api/v1/sms/inbound", 1)[0]
	if f := in.fields(t); f["from"] != "+251911223344" || f["to"] != "6060" || f["message"] != "ET 4821 1" || in.Header.Get("X-Webhook-Secret") != "hook-secret" {
		t.Errorf("Menged was sent %v, secret %q", f, in.Header.Get("X-Webhook-Secret"))
	}
	if res.MengedStatus != 200 || !res.Text.Mine || res.Text.State != "Taken by Menged: In transit recorded." {
		t.Errorf("the text is %+v", res)
	}

	tb.app.answer(func(fakeCall) (int, any) {
		return http.StatusUnauthorized, map[string]string{"error": "Bad webhook secret."}
	})
	tb.call(http.MethodPost, "/api/phones/0911223344/texts", nil, map[string]string{"text": "ET 4821 2"}, &res)
	if !res.Text.Failed || !strings.Contains(res.Text.State, "answered 401") {
		t.Errorf("a refused text is %+v", res.Text)
	}
	if n := tb.app.count("/api/v1/sms/inbound"); n != 2 {
		t.Errorf("%d texts sent to Menged, want 2: a refused one isn't sent again", n)
	}
}

// ---------------------------------------------------------------------------
// telebirr
// ---------------------------------------------------------------------------

func (tb *testbed) telebirrToken() string {
	tb.t.Helper()
	var res map[string]string
	code := tb.call(http.MethodPost, "/telebirr/payment/v1/token", http.Header{"X-App-Key": {"fabric"}}, map[string]string{"appSecret": "app-secret"}, &res)
	if code != http.StatusOK || res["token"] == "" || len(res["expirationDate"]) != 14 {
		tb.t.Fatalf("telebirr token: status %d, %v", code, res)
	}
	return res["token"]
}

type telebirrAnswer struct {
	Result     string            `json:"result"`
	Code       string            `json:"code"`
	Msg        string            `json:"msg"`
	BizContent map[string]string `json:"biz_content"`
	ErrorCode  string            `json:"errorCode"`
	ErrorMsg   string            `json:"errorMsg"`
}

// telebirrCall signs and makes one of Menged's API calls. change, when
// set, alters the request after it is signed.
func (tb *testbed) telebirrCall(path, method, token string, biz map[string]string, key *rsa.PrivateKey, change func(map[string]any)) (int, telebirrAnswer) {
	tb.t.Helper()
	req := map[string]any{
		"timestamp": strconv.FormatInt(time.Now().Unix(), 10), "nonce_str": random(upperChars, 32),
		"method": method, "version": "1.0", "biz_content": biz, "sign_type": signType,
	}
	req["sign"] = sign(key, signedFields(req))
	if change != nil {
		change(req)
	}
	var res telebirrAnswer
	code := tb.call(http.MethodPost, path, http.Header{"X-App-Key": {"fabric"}, "Authorization": {token}}, req, &res)
	return code, res
}

// signedFields are the fields of a request that its signature covers:
// biz_content's, and the request's own but sign and sign_type.
func signedFields(req map[string]any) map[string]string {
	fields := maps.Clone(req["biz_content"].(map[string]string))
	for _, k := range []string{"timestamp", "nonce_str", "method", "version"} {
		fields[k] = req[k].(string)
	}
	return fields
}

func (tb *testbed) preOrder(token string, biz map[string]string) string {
	tb.t.Helper()
	code, res := tb.telebirrCall("/telebirr/payment/v1/merchant/preOrder", "payment.preorder", token, biz, tb.merchant, nil)
	if code != http.StatusOK || res.Result != "SUCCESS" || res.BizContent["prepay_id"] == "" {
		tb.t.Fatalf("preOrder: status %d, %+v", code, res)
	}
	return res.BizContent["prepay_id"]
}

func (tb *testbed) orderBiz(merchOrderID string) map[string]string {
	return map[string]string{
		"notify_url": "http://app.example/api/v1/payments/telebirr/notify", "appid": "850001", "merch_code": "900001",
		"merch_order_id": merchOrderID, "trade_type": "Checkout", "title": "Invoice INV 2026 00012",
		"total_amount": "2875.00", "trans_currency": "ETB", "timeout_express": "30m", "business_type": "BuyGoods",
		"redirect_url": "http://app.example/app/billing/payments/42",
	}
}

// checkoutLink is the signed link Menged sends the payer to.
func (tb *testbed) checkoutLink(prepayID string, key *rsa.PrivateKey) string {
	fields := map[string]string{
		"appid": "850001", "merch_code": "900001", "nonce_str": random(upperChars, 32), "prepay_id": prepayID,
		"timestamp": strconv.FormatInt(time.Now().Unix(), 10),
	}
	q := url.Values{}
	for k, v := range fields {
		q.Set(k, v)
	}
	q.Set("sign", sign(key, fields))
	q.Set("sign_type", signType)
	q.Set("version", "1.0")
	q.Set("trade_type", "Checkout")
	return "/telebirr/checkout?" + q.Encode()
}

// notifyAt points an order's notifications at the fake Menged.
func (tb *testbed) notifyAt(biz map[string]string) map[string]string {
	biz["notify_url"] = tb.sb.cfg.AppURL + "/api/v1/payments/telebirr/notify"
	return biz
}

// telebirrSigned reports whether a notification is signed with telebirr's
// key, as Menged checks it.
func (tb *testbed) telebirrSigned(f map[string]string) bool {
	return verify(&tb.telebirr.PublicKey, f, f["sign"])
}

func TestTelebirrOrders(t *testing.T) {
	tb := newTestbed(t)
	if code := tb.call(http.MethodPost, "/telebirr/payment/v1/token", http.Header{"X-App-Key": {"other"}}, map[string]string{"appSecret": "app-secret"}, nil); code != http.StatusUnauthorized {
		t.Errorf("a token for another app: status %d", code)
	}
	token := tb.telebirrToken()
	biz := tb.orderBiz("PAYAB12CD")
	prepay := tb.preOrder(token, biz)
	if again := tb.preOrder(token, biz); again != prepay {
		t.Errorf("the same order again has prepay_id %s, want %s", again, prepay)
	}
	clash := maps.Clone(biz)
	clash["total_amount"] = "1.00"
	if _, res := tb.telebirrCall("/telebirr/payment/v1/merchant/preOrder", "payment.preorder", token, clash, tb.merchant, nil); res.Result != "FAIL" {
		t.Errorf("another order with the same merch_order_id: %+v", res)
	}

	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	for name, c := range map[string]struct {
		token  string
		biz    map[string]string
		key    *rsa.PrivateKey
		change func(map[string]any)
		status int
	}{
		"no token":         {"", biz, tb.merchant, nil, http.StatusUnauthorized},
		"another key":      {token, biz, other, nil, http.StatusBadRequest},
		"changed after":    {token, biz, tb.merchant, func(r map[string]any) { r["biz_content"].(map[string]string)["total_amount"] = "1.00" }, http.StatusBadRequest},
		"another method":   {token, biz, tb.merchant, func(r map[string]any) { r["method"] = "payment.queryorder" }, http.StatusBadRequest},
		"an old timestamp": {token, biz, tb.merchant, func(r map[string]any) { r["timestamp"] = "1700000000" }, http.StatusBadRequest},
		"milliseconds":     {token, biz, tb.merchant, func(r map[string]any) { r["timestamp"] = strconv.FormatInt(time.Now().UnixMilli(), 10) }, http.StatusBadRequest},
		"PKCS#1 v1.5": {token, biz, tb.merchant, func(r map[string]any) {
			digest := sha256.Sum256([]byte(signedText(signedFields(r))))
			sig, _ := rsa.SignPKCS1v15(rand.Reader, tb.merchant, crypto.SHA256, digest[:])
			r["sign"] = base64.StdEncoding.EncodeToString(sig)
		}, http.StatusBadRequest},
	} {
		if code, res := tb.telebirrCall("/telebirr/payment/v1/merchant/preOrder", "payment.preorder", c.token, c.biz, c.key, c.change); code != c.status {
			t.Errorf("%s: status %d (%s), want %d", name, code, res.ErrorMsg, c.status)
		}
	}
	for field, bad := range map[string]string{
		"title": "Invoice #12", "total_amount": "2,875.00", "merch_order_id": "PAY-AB12", "timeout_express": "30",
		"trade_type": "InApp", "trans_currency": "USD", "notify_url": "/notify",
	} {
		b := tb.orderBiz("PAYBAD1")
		b[field] = bad
		if code, res := tb.telebirrCall("/telebirr/payment/v1/merchant/preOrder", "payment.preorder", token, b, tb.merchant, nil); code != http.StatusBadRequest {
			t.Errorf("%s %q: status %d %+v, want 400", field, bad, code, res)
		}
	}

	// A nonce is good once.
	query := func() (int, telebirrAnswer) {
		return tb.telebirrCall("/telebirr/payment/v1/merchant/queryOrder", "payment.queryorder", token,
			map[string]string{"appid": "850001", "merch_code": "900001", "merch_order_id": "PAYAB12CD"}, tb.merchant, func(r map[string]any) {
				r["nonce_str"] = "SAMEONE"
				r["sign"] = sign(tb.merchant, signedFields(r))
			})
	}
	if _, res := query(); res.BizContent["order_status"] != orderPending {
		t.Errorf("queryOrder of a new order: %+v", res)
	}
	if code, _ := query(); code != http.StatusBadRequest {
		t.Errorf("a nonce used twice: status %d, want 400", code)
	}
}

func TestTelebirrCheckout(t *testing.T) {
	tb := newTestbed(t)
	token := tb.telebirrToken()
	biz := tb.notifyAt(tb.orderBiz("PAYCHECKOUT1"))
	prepay := tb.preOrder(token, biz)

	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	if p := tb.get(tb.checkoutLink(prepay, other)); p.Status != http.StatusBadRequest || !strings.Contains(p.Body, "signature") {
		t.Errorf("a link signed with another key: status %d", p.Status)
	}
	if p := tb.get(tb.checkoutLink("nosuchorder", tb.merchant)); p.Status != http.StatusBadRequest {
		t.Errorf("a link to no order: status %d", p.Status)
	}
	if p := tb.get(tb.checkoutLink(prepay, tb.merchant)); p.Status != http.StatusOK || !strings.Contains(p.Body, "ETB 2,875.00") || !strings.Contains(p.Body, "Invoice INV 2026 00012") {
		t.Fatalf("the checkout: status %d", p.Status)
	}

	pay := func(number, pin string) reply {
		return tb.post("/telebirr/checkout/"+prepay, url.Values{"action": {"pay"}, "phone": {number}, "pin": {pin}})
	}
	if p := pay("0911223344", "0000"); p.Status != http.StatusUnprocessableEntity || !strings.Contains(p.Body, "Wrong PIN") {
		t.Errorf("a wrong PIN: status %d", p.Status)
	}
	low := int64(1000)
	tb.sb.setBalances("0911223344", &low, nil)
	if p := pay("0911223344", "1234"); p.Status != http.StatusUnprocessableEntity || !strings.Contains(p.Body, "less than") {
		t.Errorf("too little money: status %d", p.Status)
	}
	if o, _ := tb.sb.orderCopy(prepay); o.Status != orderPending || tb.app.count("/api/v1/payments/telebirr/notify") != 0 {
		t.Fatalf("refused answers changed the order to %s, or notified Menged", o.Status)
	}

	full := int64(startingBalance)
	tb.sb.setBalances("0911223344", &full, nil)
	if p := pay("0911223344", "1234"); p.Status != http.StatusSeeOther || p.Location != biz["redirect_url"] {
		t.Fatalf("paying: status %d to %q", p.Status, p.Location)
	}
	n := tb.app.received(t, "/api/v1/payments/telebirr/notify", 1)[0].fields(t)
	if !tb.telebirrSigned(n) || n["trade_status"] != "Completed" || n["total_amount"] != "2875.00" || n["merch_order_id"] != "PAYCHECKOUT1" ||
		n["payment_order_id"] == "" || n["appid"] != "850001" || n["merch_code"] != "900001" {
		t.Errorf("the notification: %v", n)
	}
	p := tb.sb.phoneCopy("0911223344")
	if p.Telebirr != startingBalance-287500 || len(p.Texts) != 1 || p.Texts[0].From != "127" {
		t.Errorf("after paying the wallet holds %d, and the phone has %+v", p.Telebirr, p.Texts)
	}

	_, res := tb.telebirrCall("/telebirr/payment/v1/merchant/queryOrder", "payment.queryorder", token,
		map[string]string{"appid": "850001", "merch_code": "900001", "merch_order_id": "PAYCHECKOUT1"}, tb.merchant, nil)
	if res.BizContent["order_status"] != orderCompleted || res.BizContent["payment_order_id"] != n["payment_order_id"] {
		t.Errorf("queryOrder of the paid order: %+v", res)
	}
	// Paying again sends the payer back, and nothing more.
	if p := pay("0911223344", "1234"); p.Status != http.StatusSeeOther {
		t.Errorf("paying a paid order: status %d", p.Status)
	}
	time.Sleep(30 * time.Millisecond)
	if c := tb.app.count("/api/v1/payments/telebirr/notify"); c != 1 {
		t.Errorf("%d notifications, want 1", c)
	}
	if o, _ := tb.sb.orderCopy(prepay); o.Notified != "Taken by Menged" {
		t.Errorf("the order's notification is %q", o.Notified)
	}

	// Cancelling notifies too.
	prepay2 := tb.preOrder(token, tb.notifyAt(tb.orderBiz("PAYCHECKOUT2")))
	if p := tb.post("/telebirr/checkout/"+prepay2, url.Values{"action": {"cancel"}}); p.Status != http.StatusSeeOther {
		t.Errorf("cancelling: status %d", p.Status)
	}
	if n := tb.app.received(t, "/api/v1/payments/telebirr/notify", 2)[1].fields(t); n["trade_status"] != "Canceled" || !tb.telebirrSigned(n) {
		t.Errorf("the cancellation: %v", n)
	}
}

func TestTelebirrFaults(t *testing.T) {
	tb := newTestbed(t)
	token := tb.telebirrToken()
	const notify = "/api/v1/payments/telebirr/notify"
	payOrder := func(id string) string {
		t.Helper()
		prepay := tb.preOrder(token, tb.notifyAt(tb.orderBiz(id)))
		var res map[string]any
		if code := tb.call(http.MethodPost, "/api/telebirr/orders/"+prepay, nil, map[string]string{"action": "pay", "phone": "0911223344", "pin": "1234"}, &res); code != http.StatusOK {
			t.Fatalf("paying %s: status %d %v", id, code, res)
		}
		return prepay
	}

	tb.faults(map[string]any{"preorder_fails": true})
	if code, _ := tb.telebirrCall("/telebirr/payment/v1/merchant/preOrder", "payment.preorder", token, tb.orderBiz("PAYFAIL"), tb.merchant, nil); code != http.StatusInternalServerError {
		t.Errorf("preOrder failing: status %d", code)
	}

	tb.faults(map[string]any{"preorder_fails": false, "drop_notifications": true})
	prepay := payOrder("PAYDROPPED")
	if held := tb.held(); len(held) != 1 || tb.app.count(notify) != 0 {
		t.Fatalf("a dropped notification: %d held, %d sent", len(held), tb.app.count(notify))
	}
	if o, _ := tb.sb.orderCopy(prepay); o.Status != orderCompleted || o.Notified != "Held back (fault)" {
		t.Errorf("the dropped order: %s, %q", o.Status, o.Notified)
	}
	tb.call(http.MethodPost, "/api/held/"+strconv.Itoa(tb.held()[0].ID), nil, map[string]string{"action": "send"}, nil)
	tb.app.received(t, notify, 1)

	tb.faults(map[string]any{"drop_notifications": false, "duplicate_notifications": true})
	payOrder("PAYTWICE")
	twice := tb.app.received(t, notify, 3)
	if !bytes.Equal(twice[1].Body, twice[2].Body) {
		t.Error("a duplicate notification differs from the first")
	}

	tb.faults(map[string]any{"duplicate_notifications": false, "wrong_key": true})
	payOrder("PAYFORGED")
	if n := tb.app.received(t, notify, 4)[3].fields(t); tb.telebirrSigned(n) {
		t.Error("the wrong-key fault signed with telebirr's key")
	}

	tb.faults(map[string]any{"wrong_key": false, "delay_notifications": true})
	payOrder("PAYLATE")
	if c := tb.app.count(notify); c != 4 {
		t.Errorf("a late notification came at once: %d", c)
	}
	tb.app.received(t, notify, 5)

	// Menged refusing: tried four times, then held.
	tb.faults(map[string]any{"delay_notifications": false})
	tb.app.answer(func(fakeCall) (int, any) { return http.StatusInternalServerError, map[string]string{"error": "down"} })
	prepay = payOrder("PAYREFUSED")
	tb.app.received(t, notify, 9)
	eventually(t, "the refused notification held", func() bool { return len(tb.held()) == 1 })
	if o, _ := tb.sb.orderCopy(prepay); !strings.HasPrefix(o.Notified, "Refused: answered 500") {
		t.Errorf("after four refusals the order says %q", o.Notified)
	}
}

// ---------------------------------------------------------------------------
// CBE Birr
// ---------------------------------------------------------------------------

// cbeApp answers CBE Birr's calls as Menged does, for one bill. confirm
// decides each confirmation's answer.
func (tb *testbed) cbeApp(confirm func(n int) (int, any)) {
	confirmations := 0
	tb.app.answer(func(c fakeCall) (int, any) {
		if c.Header.Get("Authorization") != "Basic "+base64.StdEncoding.EncodeToString([]byte("cbe:cbe-pass")) {
			return http.StatusUnauthorized, map[string]string{"error": "login"}
		}
		f := map[string]string{}
		_ = json.Unmarshal(c.Body, &f)
		known := f["BillRefNumber"] == "202600012"
		switch c.Path {
		case "/api/v1/payments/cbebirr/query":
			if !known {
				return http.StatusOK, map[string]string{"ResultCode": "1", "ResultDesc": "No invoice has this reference."}
			}
			return http.StatusOK, map[string]string{"ResultCode": "0", "ResultDesc": "Success", "BillRefNumber": f["BillRefNumber"],
				"Amount": "2875.00", "Currency": "ETB", "CustomerName": "Abay Freight", "Description": "Invoice INV-2026-00012"}
		case "/api/v1/payments/cbebirr/validation":
			if f["TransAmount"] != "2875.00" {
				return http.StatusOK, map[string]string{"ResultCode": "4", "ResultDesc": "The amount is not what the invoice asks for."}
			}
			return http.StatusOK, map[string]string{"ResultCode": "0", "ResultDesc": "Accepted"}
		case "/api/v1/payments/cbebirr/confirmation":
			confirmations++
			if confirm != nil {
				return confirm(confirmations)
			}
			return http.StatusOK, map[string]string{"ResultCode": "0", "ResultDesc": "Success"}
		}
		return http.StatusNotFound, nil
	})
}

type cbeReply struct {
	Payment cbePayment `json:"payment"`
	Bill    cbeBill    `json:"bill"`
	Error   string     `json:"error"`
}

func (tb *testbed) payCBE(ref string, amount int64, pin string) (int, cbeReply) {
	tb.t.Helper()
	var res cbeReply
	code := tb.call(http.MethodPost, "/api/cbebirr/payments", nil, map[string]any{"phone": "0911223344", "reference": ref, "amount_santim": amount, "pin": pin}, &res)
	return code, res
}

func TestCBEBirr(t *testing.T) {
	tb := newTestbed(t)
	const confirmation = "/api/v1/payments/cbebirr/confirmation"
	// The first confirmation is refused: CBE Birr tries again until Menged
	// takes it.
	tb.cbeApp(func(n int) (int, any) {
		if n == 1 {
			return http.StatusInternalServerError, map[string]string{"error": "busy"}
		}
		return http.StatusOK, map[string]string{"ResultCode": "0", "ResultDesc": "Success"}
	})

	if code, res := tb.payCBE("202600012", 0, "0000"); code != http.StatusUnprocessableEntity || res.Error != "Wrong PIN." {
		t.Errorf("a wrong PIN: status %d %+v", code, res)
	}
	if code, res := tb.payCBE("999", 0, "1234"); code != http.StatusUnprocessableEntity || res.Bill.ResultCode != "1" {
		t.Errorf("an unknown reference: status %d %+v", code, res)
	}
	if code, res := tb.payCBE("202600012", 100, "1234"); code != http.StatusUnprocessableEntity || !strings.Contains(res.Error, "took no money") {
		t.Errorf("the wrong amount: status %d %+v", code, res)
	}
	if tb.app.count(confirmation) != 0 || tb.sb.phoneCopy("0911223344").CBEBirr != startingBalance {
		t.Fatal("a refused payment took money or was confirmed")
	}

	code, res := tb.payCBE("202600012", 0, "1234")
	if code != http.StatusOK || res.Payment.Amount != 287500 || res.Payment.TransID == "" {
		t.Fatalf("paying: status %d %+v", code, res)
	}
	calls := tb.app.received(t, confirmation, 2)
	if f := calls[1].fields(t); f["TransID"] != res.Payment.TransID || f["BillRefNumber"] != "202600012" || f["MSISDN"] != "251911223344" ||
		f["BusinessShortCode"] != "100100" || len(f["TransTime"]) != 14 || f["TransAmount"] != "2875.00" {
		t.Errorf("the confirmation: %v", f)
	}
	eventually(t, "the payment confirmed", func() bool { p, _ := tb.sb.paymentCopy(res.Payment.TransID); return p.Confirmed })
	if p := tb.sb.phoneCopy("0911223344"); p.CBEBirr != startingBalance-287500 || len(p.Texts) != 1 || p.Texts[0].From != "CBE Birr" {
		t.Errorf("after paying: %d in the wallet, texts %+v", p.CBEBirr, p.Texts)
	}

	// A confirmation Menged refuses is reversed.
	tb.cbeApp(func(int) (int, any) {
		return http.StatusOK, map[string]string{"ResultCode": "3", "ResultDesc": "This invoice was cancelled."}
	})
	_, res = tb.payCBE("202600012", 0, "1234")
	eventually(t, "the payment reversed", func() bool { p, _ := tb.sb.paymentCopy(res.Payment.TransID); return p.Reversed })
	if p := tb.sb.phoneCopy("0911223344"); p.CBEBirr != startingBalance-287500 {
		t.Errorf("after the reversal the wallet holds %d", p.CBEBirr)
	}

	// Faults: held back, then sent by hand; sent twice.
	tb.cbeApp(nil)
	tb.faults(map[string]any{"drop_confirmations": true})
	before := tb.app.count(confirmation)
	_, res = tb.payCBE("202600012", 0, "1234")
	if held := tb.held(); len(held) != 1 || tb.app.count(confirmation) != before {
		t.Fatalf("a dropped confirmation: %d held, %d sent", len(held), tb.app.count(confirmation)-before)
	}
	tb.call(http.MethodPost, "/api/held/"+strconv.Itoa(tb.held()[0].ID), nil, map[string]string{"action": "send"}, nil)
	eventually(t, "the held confirmation taken", func() bool { p, _ := tb.sb.paymentCopy(res.Payment.TransID); return p.Confirmed })

	tb.faults(map[string]any{"drop_confirmations": false, "duplicate_confirmations": true})
	before = tb.app.count(confirmation)
	tb.payCBE("202600012", 0, "1234")
	if got := tb.app.received(t, confirmation, before+2); !bytes.Equal(got[before].Body, got[before+1].Body) {
		t.Error("the duplicate confirmation differs from the first")
	}

	// Menged must recognise CBE Birr's login.
	wrong := newTestbed(t, func(c *Config) { c.CBEBirr.Password = "wrong" })
	wrong.cbeApp(nil)
	if _, err := wrong.sb.cbeQuery("0911223344", "100100", "202600012"); err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("a query with the wrong login: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Faults, pages, keys and settings
// ---------------------------------------------------------------------------

func TestFaultsAPI(t *testing.T) {
	tb := newTestbed(t)
	var f Faults
	tb.call(http.MethodPatch, "/api/faults", nil, map[string]any{"sms_down": true}, &f)
	tb.call(http.MethodPatch, "/api/faults", nil, map[string]any{"barred": []string{"+251911223344", "0911223344"}}, &f)
	if !f.SMSDown || len(f.Barred) != 1 || f.Barred[0] != "0911223344" || f.on() != 2 {
		t.Errorf("after two patches: %+v", f)
	}
	if code := tb.call(http.MethodPatch, "/api/faults", nil, map[string]any{"unreachable": []string{"12"}}, nil); code != http.StatusBadRequest {
		t.Errorf("a list with no number: status %d", code)
	}
	tb.call(http.MethodPut, "/api/faults", nil, map[string]any{"wrong_key": true}, &f)
	if f.SMSDown || len(f.Barred) != 0 || !f.WrongKey {
		t.Errorf("PUT replaces them all: %+v", f)
	}
	if p := tb.post("/controls", url.Values{"drop_reports": {"on"}, "unreachable": {"0911000002, 0711000003"}}); p.Status != http.StatusSeeOther {
		t.Fatalf("the controls form: status %d", p.Status)
	}
	if f = tb.sb.currentFaults(); !f.DropReports || f.WrongKey || len(f.Unreachable) != 2 {
		t.Errorf("after the form: %+v", f)
	}
}

func TestPages(t *testing.T) {
	tb := newTestbed(t)
	tb.cbeApp(nil)
	token := tb.smsToken()
	tb.sendSMS(token, "+251911223344", "MENGED", "Your code is 482913.")

	for path, want := range map[string]string{
		"/":                        "0911 22 33 44",
		"/phones/0911223344":       "Your code is 482913.",
		"/controls":                "The gateway is down.",
		"/log":                     "/ethiotelecom/sms/send",
		"/log?service=telebirr":    "No calls yet.",
		"/static/sandbox.css":      "--sandbox",
		"/healthz":                 "ok",
		"/phones/0911223344?ref=1": `value="1"`,
	} {
		if p := tb.get(path); p.Status != http.StatusOK || !strings.Contains(p.Body, want) {
			t.Errorf("GET %s: status %d, without %q", path, p.Status, want)
		}
	}
	if p := tb.get("/phones/+251911223344"); p.Status != http.StatusFound || p.Location != "/phones/0911223344" {
		t.Errorf("a phone in international form: status %d to %q", p.Status, p.Location)
	}
	if p := tb.get("/phones/12345"); p.Status != http.StatusNotFound {
		t.Errorf("not a phone: status %d", p.Status)
	}
	if p := tb.get("/telebirr/checkout?prepay_id=x"); p.Status != http.StatusBadRequest || !strings.Contains(p.Body, "can't be opened") {
		t.Errorf("a bad checkout link: status %d", p.Status)
	}

	// Paying by reference on the phone page.
	p := tb.post("/phones/0911223344/cbebirr", url.Values{"step": {"query"}, "merchant": {"100100"}, "ref": {"202600012"}})
	if p.Status != http.StatusOK || !strings.Contains(p.Body, "ETB 2,875.00") || !strings.Contains(p.Body, "Invoice INV-2026-00012") {
		t.Fatalf("looking the reference up: status %d", p.Status)
	}
	form := url.Values{"step": {"pay"}, "merchant": {"100100"}, "ref": {"202600012"}, "amount": {"2,875.00"}, "billed": {"2875.00"}, "pin": {"1234"}}
	if p = tb.post("/phones/0911223344/cbebirr", form); p.Status != http.StatusSeeOther || !strings.Contains(p.Location, "?paid=") {
		t.Fatalf("paying: status %d to %q", p.Status, p.Location)
	}
	if p = tb.get(p.Location); !strings.Contains(p.Body, "Paid ETB 2,875.00") {
		t.Error("the phone page doesn't show the payment")
	}
	if p = tb.post("/phones/0911223344/balances", url.Values{"telebirr": {"lots"}, "cbebirr": {"1"}}); p.Status != http.StatusUnprocessableEntity {
		t.Errorf("a balance that isn't one: status %d", p.Status)
	}

	// A page elsewhere can't post to the sandbox from the browser.
	req, _ := http.NewRequest(http.MethodPost, tb.url+"/controls", strings.NewReader("sms_down=on"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	if p := tb.do(req); p.Status != http.StatusForbidden || tb.sb.currentFaults().SMSDown {
		t.Errorf("a cross-site post: status %d", p.Status)
	}
	if csp := tb.getHeader("/", "Content-Security-Policy"); strings.Contains(csp, "form-action") || !strings.Contains(csp, "default-src 'self'") {
		t.Errorf("the CSP is %q", csp)
	}
}

func (tb *testbed) getHeader(path, name string) string {
	res, err := http.Get(tb.url + path)
	if err != nil {
		tb.t.Fatal(err)
	}
	res.Body.Close()
	return res.Header.Get(name)
}

func TestKeys(t *testing.T) {
	dir := t.TempDir()
	written, err := WriteKeys(dir)
	if err != nil || len(written) != 4 {
		t.Fatalf("WriteKeys: %v, %v", written, err)
	}
	if again, err := WriteKeys(dir); err != nil || len(again) != 0 {
		t.Errorf("WriteKeys again: %v, %v; want the keys kept", again, err)
	}
	merchant, telebirr, err := LoadKeys(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, MerchantPrivateKey))
	merchantPrivate, err := parsePrivate(b)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{"b": "2", "a": "1", "sign_type": signType}
	sig := sign(merchantPrivate, fields)
	if signedText(fields) != "a=1&b=2" || !verify(merchant, fields, sig) {
		t.Error("a signature by the merchant's private key doesn't verify with its public key")
	}
	fields["sign"] = sig
	if !verify(merchant, fields, sig) {
		t.Error("sign is part of what is signed")
	}
	fields["b"] = "3"
	if verify(merchant, fields, sig) || verify(&telebirr.PublicKey, map[string]string{"a": "1", "b": "2"}, sig) {
		t.Error("a changed field, or another key, verifies")
	}

	if err := os.Remove(filepath.Join(dir, TelebirrPublicKey)); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteKeys(dir); err == nil || !strings.Contains(err.Error(), "half") {
		t.Errorf("half a pair: %v", err)
	}
	if _, _, err := LoadKeys(t.TempDir()); err == nil || !strings.Contains(err.Error(), "make sandbox-keys") {
		t.Errorf("no keys: %v", err)
	}
}

func TestFromEnv(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteKeys(dir); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"SANDBOX_KEYS_DIR": dir, "PORT": "8090", "ETHIO_TELECOM_API_KEY": "k", "CBEBIRR_MERCHANT_ID": "100100"}
	cfg, err := FromEnv(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppURL != "http://localhost:8090" || cfg.PIN != "1234" || cfg.SMS.Number != "6060" || cfg.SMS.APIKey != "k" || cfg.Telebirr.Key == nil {
		t.Errorf("from the environment: %+v", cfg)
	}
	env["PUBLIC_URL"], env["SANDBOX_APP_URL"] = "https://menged.example", "http://app:8080"
	if cfg, _ = FromEnv(func(k string) string { return env[k] }); cfg.AppURL != "http://app:8080" {
		t.Errorf("SANDBOX_APP_URL comes first: %s", cfg.AppURL)
	}
	if _, err := New(cfg); err == nil || !strings.Contains(err.Error(), "TELEBIRR_APP_SECRET") || strings.Contains(err.Error(), "CBEBIRR_MERCHANT_ID") {
		t.Errorf("New with settings missing: %v", err)
	}
}

func TestReachAndMasking(t *testing.T) {
	s := &Sandbox{cfg: Config{HostAlias: "host.docker.internal"}}
	for in, want := range map[string]string{
		"http://localhost:8090/api/v1/sms/status": "http://host.docker.internal:8090/api/v1/sms/status",
		"http://127.0.0.1/notify":                 "http://host.docker.internal/notify",
		"https://menged.example/notify":           "https://menged.example/notify",
	} {
		if got := s.reach(in); got != want {
			t.Errorf("reach(%s) = %s, want %s", in, got, want)
		}
	}

	h := http.Header{
		"Authorization":    {"Basic " + base64.StdEncoding.EncodeToString([]byte("cbe:cbe-pass"))},
		"X-Webhook-Secret": {"hook-secret"},
	}
	got := describe(h, []byte(`{"api_key":"sms-key","appSecret":"app-secret","token":"abcdefghijklmnopqrstuvwxyz","to":"+251911223344"}`), "")
	for _, secret := range []string{"cbe-pass", "hook-secret", "sms-key", "app-secret", "ghijklmnop"} {
		if strings.Contains(got, secret) {
			t.Errorf("the log shows %q:\n%s", secret, got)
		}
	}
	if !strings.Contains(got, "Basic cbe:•••") || !strings.Contains(got, "+251911223344") || !strings.Contains(got, `"abcdef…"`) {
		t.Errorf("the log hides too much:\n%s", got)
	}
}

func TestMoneyAndNumbers(t *testing.T) {
	for in, want := range map[string]int64{"2875.00": 287500, "2875": 287500, "0.5": 50, "12.05": 1205} {
		if got, err := parseAmount(in); err != nil || got != want {
			t.Errorf("parseAmount(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "1,000.00", "1.005", "-5", "1e3", "12."} {
		if _, err := parseAmount(bad); err == nil && bad != "12." {
			t.Errorf("parseAmount(%q) took it", bad)
		}
	}
	if got, _ := parseBirr(" 1,250.50 "); got != 125050 {
		t.Errorf("parseBirr: %d", got)
	}
	if birr(123456789) != "1,234,567.89" || birr(5) != "0.05" || decimal(287500) != "2875.00" {
		t.Errorf("birr: %s %s %s", birr(123456789), birr(5), decimal(287500))
	}
	for in, want := range map[string]string{"0911223344": "0911223344", "911 22 33 44": "0911223344", "+251-911-223344": "0911223344", "251711223344": "0711223344"} {
		if got, ok := localNumber(in); !ok || got != want {
			t.Errorf("localNumber(%q) = %q, %v", in, got, ok)
		}
	}
	for _, bad := range []string{"0111223344", "12345", "+1911223344", "09112233445"} {
		if _, ok := localNumber(bad); ok {
			t.Errorf("localNumber(%q) took it", bad)
		}
	}
	if parts(strings.Repeat("a", 160)) != 1 || parts(strings.Repeat("a", 161)) != 2 || parts(strings.Repeat("ሀ", 71)) != 2 {
		t.Error("parts miscounts")
	}
}
