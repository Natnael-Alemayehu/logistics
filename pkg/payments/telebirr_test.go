package payments

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// Keys are slow to make, so the tests share two pairs: Kiflet's and
// telebirr's.
var (
	keysOnce           sync.Once
	merchantKey, tbKey *rsa.PrivateKey
)

func testKeys(t *testing.T) (merchant, telebirr *rsa.PrivateKey) {
	t.Helper()
	keysOnce.Do(func() {
		var err error
		if merchantKey, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
			panic(err)
		}
		if tbKey, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
			panic(err)
		}
	})
	return merchantKey, tbKey
}

func pemPrivate(k *rsa.PrivateKey) []byte {
	der, _ := x509.MarshalPKCS8PrivateKey(k)
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func pemPublic(k *rsa.PublicKey) []byte {
	der, _ := x509.MarshalPKIXPublicKey(k)
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
}

// fakeTelebirr answers token, preOrder and queryOrder the way the guide
// describes, and checks what Kiflet sends.
type fakeTelebirr struct {
	t        *testing.T
	merchant *rsa.PublicKey
	mu       sync.Mutex
	tokens   int
	reject   int // how many calls to refuse with 401, as for an expired token
	orders   []map[string]string
	status   string
}

func (f *fakeTelebirr) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("X-APP-Key") != "fabric-app" {
		http.Error(w, "unknown app", http.StatusUnauthorized)
		return
	}
	if r.URL.Path == "/gw/payment/v1/token" {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["appSecret"] != "app-secret" {
			http.Error(w, "bad secret", http.StatusUnauthorized)
			return
		}
		f.tokens++
		writeJSON(w, map[string]string{
			"token": "token-" + string(rune('0'+f.tokens)), "effectiveDate": time.Now().UTC().Format("20060102150405"),
			"expirationDate": time.Now().UTC().Add(time.Hour).Format("20060102150405"),
		})
		return
	}
	if f.reject > 0 {
		f.reject--
		http.Error(w, "token expired", http.StatusUnauthorized)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Authorization"), "token-") {
		http.Error(w, "no token", http.StatusUnauthorized)
		return
	}
	var req telebirrRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		f.t.Errorf("request body: %v", err)
		return
	}
	fields := map[string]string{"timestamp": req.Timestamp, "nonce_str": req.NonceStr, "method": req.Method, "version": req.Version}
	for k, v := range req.BizContent {
		fields[k] = v
	}
	if err := verifyFields(f.merchant, fields, req.Sign); err != nil || req.SignType != "SHA256WithRSA" {
		writeJSON(w, map[string]string{"result": "FAIL", "code": "40001", "msg": "verify sign failed"})
		return
	}
	switch r.URL.Path {
	case "/gw/payment/v1/merchant/preOrder":
		f.orders = append(f.orders, req.BizContent)
		writeJSON(w, map[string]any{"result": "SUCCESS", "code": "0", "msg": "Success",
			"biz_content": map[string]string{"merch_order_id": req.BizContent["merch_order_id"], "prepay_id": "prepay-1"}})
	case "/gw/payment/v1/merchant/queryOrder":
		writeJSON(w, map[string]any{"result": "SUCCESS", "code": "0",
			"biz_content": map[string]string{"merch_order_id": req.BizContent["merch_order_id"], "order_status": f.status,
				"payment_order_id": "PO-9", "total_amount": "2875.00", "trans_currency": "ETB"}})
	default:
		http.NotFound(w, r)
	}
}

func newTelebirr(t *testing.T, apiURL string) *Telebirr {
	t.Helper()
	merchant, telebirr := testKeys(t)
	tb, err := NewTelebirr(TelebirrConfig{
		APIURL: apiURL + "/gw", CheckoutURL: "https://checkout.telebirr.test/payment/web/paygate?",
		FabricAppID: "fabric-app", AppSecret: "app-secret", MerchantAppID: "merchant-app", MerchantCode: "100200",
		PrivateKey: pemPrivate(merchant), PublicKey: pemPublic(&telebirr.PublicKey),
	})
	if err != nil {
		t.Fatal(err)
	}
	return tb
}

func TestTelebirrSignString(t *testing.T) {
	got := signString(map[string]string{
		"timestamp": "1718000000", "nonce_str": "N1", "method": "payment.preorder", "version": "1.0",
		"title": "Kiflet invoice", "appid": "A1", "sign": "x", "sign_type": "SHA256WithRSA",
	})
	want := "appid=A1&method=payment.preorder&nonce_str=N1&timestamp=1718000000&title=Kiflet invoice&version=1.0"
	if got != want {
		t.Errorf("signString =\n %s\nwant\n %s", got, want)
	}
}

func TestTelebirrSignatures(t *testing.T) {
	merchant, telebirr := testKeys(t)
	fields := map[string]string{"appid": "A1", "total_amount": "10.00"}
	sig, err := signFields(merchant, fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyFields(&merchant.PublicKey, fields, sig); err != nil {
		t.Errorf("own signature: %v", err)
	}
	fields["total_amount"] = "1.00"
	if err := verifyFields(&merchant.PublicKey, fields, sig); !errors.Is(err, ErrBadSignature) {
		t.Errorf("a changed amount verified: %v", err)
	}
	fields["total_amount"] = "10.00"
	if err := verifyFields(&telebirr.PublicKey, fields, sig); !errors.Is(err, ErrBadSignature) {
		t.Errorf("another key verified: %v", err)
	}

	// The portal shows keys as bare base64; PKCS#1 works as well as PKCS#8.
	pkcs1 := x509.MarshalPKCS1PrivateKey(merchant)
	for name, b := range map[string][]byte{
		"pem pkcs8": pemPrivate(merchant), "base64 pkcs1": []byte(base64.StdEncoding.EncodeToString(pkcs1)),
		"pem pkcs1": pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: pkcs1}),
	} {
		if k, err := parsePrivateKey(b); err != nil || !k.Equal(merchant) {
			t.Errorf("%s: %v", name, err)
		}
	}
	pub, _ := x509.MarshalPKIXPublicKey(&telebirr.PublicKey)
	if k, err := parsePublicKey([]byte(base64.StdEncoding.EncodeToString(pub))); err != nil || !k.Equal(&telebirr.PublicKey) {
		t.Errorf("base64 public key: %v", err)
	}
	if _, err := parsePrivateKey([]byte("not a key")); err == nil {
		t.Error("read a key from rubbish")
	}
}

func TestTelebirrStartsAnOrder(t *testing.T) {
	merchant, _ := testKeys(t)
	fake := &fakeTelebirr{t: t, merchant: &merchant.PublicKey}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	tb := newTelebirr(t, srv.URL)
	charge := Charge{
		Reference: "PAY-ABCD2345", Amount: 287500, Description: "Kiflet invoice INV-2026-00012 for Abay Freight & Co.",
		NotifyURL: "https://kiflet.test/api/v1/payments/telebirr/notify", ReturnURL: "https://kiflet.test/app/billing/payments/1",
		Timeout: 30 * time.Minute,
	}
	co, err := tb.Start(context.Background(), charge)
	if err != nil {
		t.Fatal(err)
	}
	order := fake.orders[0]
	for k, want := range map[string]string{
		"merch_order_id": "PAYABCD2345", "total_amount": "2875.00", "trans_currency": "ETB", "timeout_express": "30m",
		"title": "Kiflet invoice INV 2026 00012 for Abay Freight Co", "trade_type": "Checkout", "business_type": "BuyGoods",
		"appid": "merchant-app", "merch_code": "100200", "notify_url": charge.NotifyURL, "redirect_url": charge.ReturnURL,
	} {
		if order[k] != want {
			t.Errorf("preOrder %s = %q, want %q", k, order[k], want)
		}
	}

	// The checkout page opens with the signed order; version and trade_type
	// follow, unsigned.
	if co.ProviderRef != "prepay-1" || !strings.HasPrefix(co.RedirectURL, "https://checkout.telebirr.test/payment/web/paygate?appid=") ||
		!strings.HasSuffix(co.RedirectURL, "&version=1.0&trade_type=Checkout") {
		t.Fatalf("checkout = %+v", co)
	}
	u, _ := url.Parse(co.RedirectURL)
	q := u.Query()
	raw := map[string]string{}
	for _, k := range []string{"appid", "merch_code", "nonce_str", "prepay_id", "timestamp"} {
		raw[k] = q.Get(k)
	}
	if err := verifyFields(&merchant.PublicKey, raw, q.Get("sign")); err != nil || q.Get("prepay_id") != "prepay-1" {
		t.Errorf("the checkout link's signature doesn't verify: %v", err)
	}
	if tb.CheckoutOrigin() != "https://checkout.telebirr.test" {
		t.Errorf("checkout origin = %q", tb.CheckoutOrigin())
	}

	// The token is reused, and replaced once when telebirr refuses it.
	if _, err := tb.Start(context.Background(), charge); err != nil || fake.tokens != 1 {
		t.Errorf("second order: %v, %d tokens", err, fake.tokens)
	}
	fake.reject = 1
	if _, err := tb.Start(context.Background(), charge); err != nil || fake.tokens != 2 {
		t.Errorf("after a 401: %v, %d tokens", err, fake.tokens)
	}
	fake.reject = 2
	if _, err := tb.Start(context.Background(), charge); err == nil {
		t.Error("a second 401 in a row should fail the call")
	}
}

func TestTelebirrAnswersAreChecked(t *testing.T) {
	_, other := testKeys(t) // signs with the wrong key, so telebirr refuses
	fake := &fakeTelebirr{t: t, merchant: &other.PublicKey}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	if _, err := newTelebirr(t, srv.URL).Start(context.Background(), Charge{Reference: "PAY-X", Amount: 100}); err == nil ||
		!strings.Contains(err.Error(), "verify sign failed") {
		t.Errorf("a refused order: %v", err)
	}
}

func TestTelebirrQuery(t *testing.T) {
	merchant, _ := testKeys(t)
	fake := &fakeTelebirr{t: t, merchant: &merchant.PublicKey}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	tb := newTelebirr(t, srv.URL)
	for status, want := range map[string]Notification{
		"Completed": {Reference: "PAY-AB", ProviderRef: "PO-9", Status: StatusPaid, Amount: 287500},
		"Expired":   {Reference: "PAY-AB", ProviderRef: "PO-9", Status: StatusFailed},
		"Paying":    {},
	} {
		fake.status = status
		n, done, err := tb.Query(context.Background(), "PAY-AB")
		if err != nil || n != want || done != (status != "Paying") {
			t.Errorf("%s: %+v, %v, %v", status, n, done, err)
		}
	}
}

func TestTelebirrNotifications(t *testing.T) {
	_, telebirr := testKeys(t)
	tb := newTelebirr(t, "http://unused")
	notification := func(change map[string]string, key *rsa.PrivateKey) *http.Request {
		fields := map[string]string{
			"notify_url": "https://kiflet.test/api/v1/payments/telebirr/notify", "appid": "merchant-app",
			"notify_time": "1670575472482", "merch_code": "100200", "merch_order_id": "PAYABCD2345",
			"payment_order_id": "00801104C911443200001002", "total_amount": "2875.00", "trans_id": "49485948475845",
			"trans_currency": "ETB", "trade_status": "Completed", "trans_end_time": "1670575472000",
		}
		for k, v := range change {
			fields[k] = v
		}
		sig, _ := signFields(key, fields)
		fields["sign"], fields["sign_type"] = sig, "SHA256WithRSA"
		b, _ := json.Marshal(fields)
		r := httptest.NewRequest(http.MethodPost, "/notify", strings.NewReader(string(b)))
		r.Header.Set("Content-Type", "application/json")
		return r
	}
	serve := func(r *http.Request) (*ledger, int) {
		l := &ledger{}
		res := httptest.NewRecorder()
		tb.Handler(l, l.fail).ServeHTTP(res, r)
		return l, res.Code
	}

	l, code := serve(notification(nil, telebirr))
	want := Notification{Reference: "PAY-ABCD2345", ProviderRef: "00801104C911443200001002", Status: StatusPaid, Amount: 287500}
	if code != http.StatusOK || len(l.settled) != 1 || l.settled[0] != want {
		t.Errorf("a completed payment: status %d, settled %+v", code, l.settled)
	}
	if l, code := serve(notification(map[string]string{"trade_status": "Failure"}, telebirr)); code != http.StatusOK || l.settled[0].Status != StatusFailed {
		t.Errorf("a failed payment: status %d, settled %+v", code, l.settled)
	}
	if l, code := serve(notification(map[string]string{"trade_status": "Paying"}, telebirr)); code != http.StatusOK || len(l.settled) != 0 {
		t.Errorf("a payment under way: status %d, settled %+v", code, l.settled)
	}

	merchant, _ := testKeys(t)
	for name, r := range map[string]*http.Request{
		"signed with another key": notification(nil, merchant),
		"for another merchant":    notification(map[string]string{"merch_code": "999999"}, telebirr),
	} {
		if l, code := serve(r); code != http.StatusUnauthorized || len(l.settled) != 0 {
			t.Errorf("%s: status %d, settled %+v", name, code, l.settled)
		}
	}
	tampered := notification(nil, telebirr)
	body := strings.Replace(readAll(tampered), `"total_amount":"2875.00"`, `"total_amount":"1.00"`, 1)
	tampered = httptest.NewRequest(http.MethodPost, "/notify", strings.NewReader(body))
	if _, code := serve(tampered); code != http.StatusUnauthorized {
		t.Errorf("a changed amount: status %d", code)
	}
	if _, code := serve(httptest.NewRequest(http.MethodPost, "/notify", strings.NewReader("{"))); code != http.StatusBadRequest {
		t.Errorf("unreadable: status %d", code)
	}
}

func TestTelebirrFieldRules(t *testing.T) {
	if got := referenceOf(orderID("PAY-ABCD2345")); got != "PAY-ABCD2345" {
		t.Errorf("a reference doesn't survive telebirr: %q", got)
	}
	for d, want := range map[time.Duration]string{30 * time.Minute: "30m", 0: "120m", 5 * time.Hour: "120m", 90 * time.Second: "1m"} {
		if got := timeoutExpress(d); got != want {
			t.Errorf("timeoutExpress(%v) = %q, want %q", d, got, want)
		}
	}
	if n := nonce(); len(n) != 32 || strings.Trim(n, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") != "" {
		t.Errorf("nonce %q", n)
	}
	if _, err := NewTelebirr(TelebirrConfig{APIURL: "x"}); err == nil || !strings.Contains(err.Error(), "app secret") {
		t.Errorf("missing settings: %v", err)
	}
}

func readAll(r *http.Request) string {
	var b strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := r.Body.Read(buf)
		b.Write(buf[:n])
		if err != nil {
			return b.String()
		}
	}
}
