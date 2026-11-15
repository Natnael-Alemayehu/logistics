package payments

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// ErrMalformed means a provider's call could not be read.
var ErrMalformed = errors.New("the provider's message could not be read")

// Telebirr takes payments through telebirr's web checkout, its H5 C2B
// integration. Kiflet creates an order with telebirr's payment server and
// sends the payer to telebirr's checkout page, where they pay with their
// phone number and PIN. telebirr then reports the result to the
// notification URL. A lost report is made up for by asking about the order.
//
// The calls follow telebirr's published integration guide. Two things it
// leaves open each live in one place, to check against the merchant
// portal: the signature padding (pss) and the order states
// (telebirrOutcome). Answers to Kiflet's own calls arrive over the HTTPS
// connection it opened, so their signatures aren't checked; notifications,
// which anyone could send, are.
type Telebirr struct {
	cfg      TelebirrConfig
	key      *rsa.PrivateKey // ours: signs requests
	pub      *rsa.PublicKey  // telebirr's: checks notifications
	checkout string
	origin   string
	client   *http.Client

	mu         sync.Mutex
	token      string
	tokenUntil time.Time
}

// TelebirrConfig is what telebirr's merchant portal issues.
type TelebirrConfig struct {
	// APIURL is the payment gateway, which /payment/v1/… follows.
	APIURL string
	// CheckoutURL is the web checkout page; the signed order is appended to
	// it as a query.
	CheckoutURL string
	// FabricAppID and AppSecret obtain the API token.
	FabricAppID string
	AppSecret   string
	// MerchantAppID and MerchantCode identify Kiflet in orders.
	MerchantAppID string
	MerchantCode  string
	// PrivateKey is ours and PublicKey telebirr's: PEM, or the bare base64
	// the portal shows.
	PrivateKey []byte
	PublicKey  []byte
	// Client makes the API calls; nil is a client with a 20-second timeout.
	Client *http.Client
}

// NewTelebirr checks the configuration and reads the keys.
func NewTelebirr(c TelebirrConfig) (*Telebirr, error) {
	var missing []string
	for name, v := range map[string]string{
		"API URL": c.APIURL, "checkout URL": c.CheckoutURL, "fabric app ID": c.FabricAppID, "app secret": c.AppSecret,
		"merchant app ID": c.MerchantAppID, "merchant code": c.MerchantCode,
	} {
		if v == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("telebirr: missing %s", strings.Join(missing, ", "))
	}
	key, err := parsePrivateKey(c.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("telebirr private key: %w", err)
	}
	pub, err := parsePublicKey(c.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("telebirr public key: %w", err)
	}
	u, err := url.Parse(c.CheckoutURL)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("telebirr checkout URL %q is not a URL", c.CheckoutURL)
	}
	t := &Telebirr{cfg: c, key: key, pub: pub, client: c.Client, origin: u.Scheme + "://" + u.Host}
	t.cfg.APIURL = strings.TrimRight(c.APIURL, "/")
	switch {
	case !strings.Contains(c.CheckoutURL, "?"):
		t.checkout = c.CheckoutURL + "?"
	case strings.HasSuffix(c.CheckoutURL, "?"), strings.HasSuffix(c.CheckoutURL, "&"):
		t.checkout = c.CheckoutURL
	default:
		t.checkout = c.CheckoutURL + "&"
	}
	if t.client == nil {
		t.client = &http.Client{Timeout: 20 * time.Second}
	}
	return t, nil
}

func (t *Telebirr) Code() string           { return "telebirr" }
func (t *Telebirr) Label() string          { return "telebirr" }
func (*Telebirr) Mock() bool               { return false }
func (*Telebirr) Flow() Flow               { return FlowCheckout }
func (t *Telebirr) CheckoutOrigin() string { return t.origin }

// Start creates the order (preOrder) and returns the checkout page for it.
func (t *Telebirr) Start(ctx context.Context, c Charge) (Checkout, error) {
	biz := map[string]string{
		"notify_url":      c.NotifyURL,
		"appid":           t.cfg.MerchantAppID,
		"merch_code":      t.cfg.MerchantCode,
		"merch_order_id":  orderID(c.Reference),
		"trade_type":      "Checkout",
		"title":           orderTitle(c.Description),
		"total_amount":    Decimal(c.Amount),
		"trans_currency":  "ETB",
		"timeout_express": timeoutExpress(c.Timeout),
		"business_type":   "BuyGoods",
	}
	if c.ReturnURL != "" {
		biz["redirect_url"] = c.ReturnURL
	}
	res, err := t.call(ctx, "/payment/v1/merchant/preOrder", "payment.preorder", biz)
	if err != nil {
		return Checkout{}, err
	}
	prepay := string(res["prepay_id"])
	if prepay == "" {
		return Checkout{}, errors.New("telebirr preOrder: no prepay_id in the answer")
	}
	raw, err := t.rawRequest(prepay)
	if err != nil {
		return Checkout{}, err
	}
	return Checkout{ProviderRef: prepay, RedirectURL: t.checkout + raw + "&version=1.0&trade_type=Checkout"}, nil
}

// rawRequest is the signed order the checkout page opens with. As in
// telebirr's guide, version and trade_type follow it unsigned.
func (t *Telebirr) rawRequest(prepayID string) (string, error) {
	fields := map[string]string{
		"appid": t.cfg.MerchantAppID, "merch_code": t.cfg.MerchantCode, "nonce_str": nonce(),
		"prepay_id": prepayID, "timestamp": strconv.FormatInt(time.Now().Unix(), 10),
	}
	sig, err := signFields(t.key, fields)
	if err != nil {
		return "", err
	}
	var q []string
	for _, k := range []string{"appid", "merch_code", "nonce_str", "prepay_id", "timestamp"} {
		q = append(q, k+"="+url.QueryEscape(fields[k]))
	}
	q = append(q, "sign="+url.QueryEscape(sig), "sign_type="+signType)
	return strings.Join(q, "&"), nil
}

// Query asks how the order for one of our references ended (queryOrder).
func (t *Telebirr) Query(ctx context.Context, reference string) (Notification, bool, error) {
	res, err := t.call(ctx, "/payment/v1/merchant/queryOrder", "payment.queryorder", map[string]string{
		"appid": t.cfg.MerchantAppID, "merch_code": t.cfg.MerchantCode, "merch_order_id": orderID(reference),
	})
	if err != nil {
		return Notification{}, false, err
	}
	status, done := telebirrOutcome[string(res["order_status"])]
	if !done {
		return Notification{}, false, nil
	}
	n := Notification{Reference: reference, ProviderRef: firstOf(string(res["payment_order_id"]), string(res["trans_id"])), Status: status}
	if status == StatusPaid {
		if n.Amount, err = ParseDecimal(string(res["total_amount"])); err != nil {
			return Notification{}, false, fmt.Errorf("telebirr queryOrder: %w", err)
		}
	}
	return n, true, nil
}

// Handler serves POST /notify, where telebirr reports how an order ended.
func (t *Telebirr) Handler(l Ledger, fail ErrorWriter) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /notify", func(w http.ResponseWriter, r *http.Request) {
		n, done, err := t.notification(r)
		if err == nil && done {
			err = l.Settle(r.Context(), t.Code(), n)
		}
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, map[string]string{"code": "0", "msg": "success"})
	})
	return mux
}

// notification reads telebirr's report and checks that it is signed with
// telebirr's key and is about one of our orders. done is false for a state
// that isn't final.
func (t *Telebirr) notification(r *http.Request) (Notification, bool, error) {
	fields, err := readFields(r)
	if err != nil {
		return Notification{}, false, err
	}
	if err := verifyFields(t.pub, fields, fields["sign"]); err != nil {
		return Notification{}, false, err
	}
	if fields["appid"] != t.cfg.MerchantAppID || fields["merch_code"] != t.cfg.MerchantCode {
		return Notification{}, false, fmt.Errorf("%w: it is for another merchant", ErrBadSignature)
	}
	status, done := telebirrOutcome[fields["trade_status"]]
	if !done {
		return Notification{}, false, nil
	}
	n := Notification{
		Reference:   referenceOf(fields["merch_order_id"]),
		ProviderRef: firstOf(fields["payment_order_id"], fields["trans_id"]),
		Status:      status,
	}
	if status == StatusPaid {
		if n.Amount, err = ParseDecimal(fields["total_amount"]); err != nil {
			return Notification{}, false, fmt.Errorf("%w: %v", ErrMalformed, err)
		}
	}
	return n, true, nil
}

// telebirrOutcome maps telebirr's trade and order states to ours. The guide
// names only Completed. The others are the usual names for an order that
// failed, expired or was cancelled, still to be checked against the
// merchant portal. Any other state, such as Paying, leaves the payment open.
var telebirrOutcome = map[string]string{
	"Completed": StatusPaid, "PAY_SUCCESS": StatusPaid, "SUCCESS": StatusPaid,
	"Failure": StatusFailed, "FAILED": StatusFailed, "PAY_FAILED": StatusFailed,
	"Expired": StatusFailed, "ORDER_CLOSED": StatusFailed, "CLOSED": StatusFailed,
	"Canceled": StatusFailed, "Cancelled": StatusFailed,
}

// ---------------------------------------------------------------------------
// The API
// ---------------------------------------------------------------------------

type telebirrRequest struct {
	Timestamp  string            `json:"timestamp"`
	NonceStr   string            `json:"nonce_str"`
	Method     string            `json:"method"`
	Version    string            `json:"version"`
	BizContent map[string]string `json:"biz_content"`
	Sign       string            `json:"sign"`
	SignType   string            `json:"sign_type"`
}

type telebirrResponse struct {
	Result     string          `json:"result"`
	Code       text            `json:"code"`
	Msg        string          `json:"msg"`
	BizContent map[string]text `json:"biz_content"`
	ErrorCode  text            `json:"errorCode"`
	ErrorMsg   string          `json:"errorMsg"`
}

// call signs and sends a request, getting a new token once if telebirr
// says the current one is no longer valid.
func (t *Telebirr) call(ctx context.Context, path, method string, biz map[string]string) (map[string]text, error) {
	for attempt := 0; ; attempt++ {
		token, err := t.apiToken(ctx, attempt > 0)
		if err != nil {
			return nil, err
		}
		req := telebirrRequest{
			Timestamp: strconv.FormatInt(time.Now().Unix(), 10), NonceStr: nonce(),
			Method: method, Version: "1.0", BizContent: biz, SignType: signType,
		}
		fields := map[string]string{"timestamp": req.Timestamp, "nonce_str": req.NonceStr, "method": method, "version": req.Version}
		maps.Copy(fields, biz)
		if req.Sign, err = signFields(t.key, fields); err != nil {
			return nil, err
		}
		var res telebirrResponse
		status, err := t.post(ctx, path, token, req, &res)
		if status == http.StatusUnauthorized && attempt == 0 {
			continue
		}
		if err != nil {
			return nil, err
		}
		if res.Result != "SUCCESS" || res.Code != "0" {
			return nil, fmt.Errorf("telebirr %s: %s %s", method, firstOf(string(res.Code), string(res.ErrorCode)), firstOf(res.Msg, res.ErrorMsg))
		}
		return res.BizContent, nil
	}
}

// apiToken returns the API token, asking for a new one when it is about to
// expire or fresh is set.
func (t *Telebirr) apiToken(ctx context.Context, fresh bool) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !fresh && t.token != "" && time.Now().Before(t.tokenUntil) {
		return t.token, nil
	}
	var res struct {
		Token          string `json:"token"`
		ExpirationDate string `json:"expirationDate"`
	}
	if _, err := t.post(ctx, "/payment/v1/token", "", map[string]string{"appSecret": t.cfg.AppSecret}, &res); err != nil {
		return "", err
	}
	if res.Token == "" {
		return "", errors.New("telebirr token: no token in the answer")
	}
	// The guide doesn't say which clock expirationDate is on, so a token is
	// kept for an hour at most; a 401 replaces it sooner.
	until := time.Now().Add(time.Hour)
	if exp, err := time.Parse("20060102150405", res.ExpirationDate); err == nil && exp.Add(-5*time.Minute).Before(until) {
		until = exp.Add(-5 * time.Minute)
	}
	t.token, t.tokenUntil = res.Token, until
	return t.token, nil
}

func (t *Telebirr) post(ctx context.Context, path, token string, body, out any) (int, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.cfg.APIURL+path, bytes.NewReader(b))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-APP-Key", t.cfg.FabricAppID)
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	res, err := t.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("telebirr %s: %w", path, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return res.StatusCode, fmt.Errorf("telebirr %s: %w", path, err)
	}
	if res.StatusCode >= 300 {
		return res.StatusCode, fmt.Errorf("telebirr %s: status %d: %s", path, res.StatusCode, excerpt(raw))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return res.StatusCode, fmt.Errorf("telebirr %s: %w", path, err)
	}
	return res.StatusCode, nil
}

// ---------------------------------------------------------------------------
// Signatures
// ---------------------------------------------------------------------------

const signType = "SHA256WithRSA"

// pss is the padding of telebirr's SHA256WithRSA signatures. Its production
// gateway is reported to require RSA-PSS rather than PKCS#1 v1.5.
var pss = &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256}

// signString is what a signature covers: every field except sign and
// sign_type, those of biz_content among them, as name=value sorted by name
// and joined with &.
func signString(fields map[string]string) string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		if k != "sign" && k != "sign_type" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(fields[k])
	}
	return b.String()
}

func signFields(key *rsa.PrivateKey, fields map[string]string) (string, error) {
	sum := sha256.Sum256([]byte(signString(fields)))
	sig, err := rsa.SignPSS(rand.Reader, key, crypto.SHA256, sum[:], pss)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func verifyFields(pub *rsa.PublicKey, fields map[string]string, sig string) error {
	raw, err := base64.StdEncoding.DecodeString(sig)
	if err != nil || sig == "" {
		return ErrBadSignature
	}
	sum := sha256.Sum256([]byte(signString(fields)))
	if rsa.VerifyPSS(pub, crypto.SHA256, sum[:], raw, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthAuto, Hash: crypto.SHA256}) != nil {
		return ErrBadSignature
	}
	return nil
}

// parsePrivateKey reads an RSA private key: PEM or bare base64, PKCS#8 or
// PKCS#1.
func parsePrivateKey(b []byte) (*rsa.PrivateKey, error) {
	der := keyDER(b)
	if k, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		if rk, ok := k.(*rsa.PrivateKey); ok {
			return rk, nil
		}
		return nil, errors.New("not an RSA key")
	}
	if k, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return k, nil
	}
	return nil, errors.New("not an RSA private key in PEM or base64")
}

// parsePublicKey reads an RSA public key: PEM or bare base64, PKIX or
// PKCS#1.
func parsePublicKey(b []byte) (*rsa.PublicKey, error) {
	der := keyDER(b)
	if k, err := x509.ParsePKIXPublicKey(der); err == nil {
		if rk, ok := k.(*rsa.PublicKey); ok {
			return rk, nil
		}
		return nil, errors.New("not an RSA key")
	}
	if k, err := x509.ParsePKCS1PublicKey(der); err == nil {
		return k, nil
	}
	return nil, errors.New("not an RSA public key in PEM or base64")
}

func keyDER(b []byte) []byte {
	if blk, _ := pem.Decode(b); blk != nil {
		return blk.Bytes
	}
	der, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(string(b)), ""))
	if err != nil {
		return nil
	}
	return der
}

// ---------------------------------------------------------------------------
// Field rules
// ---------------------------------------------------------------------------

// orderID is a reference as telebirr's merch_order_id, which takes letters
// and digits only: PAY-AB12 becomes PAYAB12.
func orderID(reference string) string { return strings.ReplaceAll(reference, "-", "") }

// referenceOf puts back the dash orderID removed from Kiflet's PAY-
// references.
func referenceOf(orderID string) string {
	if strings.HasPrefix(orderID, "PAY") && !strings.Contains(orderID, "-") {
		return "PAY-" + orderID[3:]
	}
	return orderID
}

// orderTitle keeps a description to the letters, digits and spaces that
// telebirr's titles take.
func orderTitle(s string) string {
	title := strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, s)), " ")
	if r := []rune(title); len(r) > 512 {
		title = string(r[:512])
	}
	return title
}

// timeoutExpress is a timeout as telebirr takes it: whole minutes from 1 to
// 120, such as "30m".
func timeoutExpress(d time.Duration) string {
	m := int(d / time.Minute)
	if m < 1 || m > 120 {
		m = 120
	}
	return strconv.Itoa(m) + "m"
}

// nonce is a random string of 32 letters and digits.
func nonce() string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

// readFields reads a provider's message, JSON or a form, as text fields.
func readFields(r *http.Request) (map[string]string, error) {
	out := map[string]string{}
	if ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); ct == "application/x-www-form-urlencoded" {
		if err := r.ParseForm(); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
		}
		for k := range r.PostForm {
			out[k] = r.PostForm.Get(k)
		}
		return out, nil
	}
	var m map[string]text
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&m); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	for k, v := range m {
		out[k] = string(v)
	}
	return out, nil
}

// text reads a JSON string, or a number, as text: telebirr's examples
// quote their numbers, but its guide doesn't promise that they always will.
type text string

func (t *text) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*t = text(s)
		return nil
	}
	if string(b) == "null" {
		*t = ""
		return nil
	}
	*t = text(b)
	return nil
}

func firstOf(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func excerpt(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
