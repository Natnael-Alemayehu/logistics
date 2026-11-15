package sandbox

// telebirr's side of Kiflet's adapter (pkg/payments/telebirr.go), after
// telebirr's published H5 web checkout integration:
//
//	POST /telebirr/payment/v1/token                X-APP-Key, {"appSecret"}
//	                                               → {"token", "effectiveDate", "expirationDate"}
//	POST /telebirr/payment/v1/merchant/preOrder    X-APP-Key, Authorization: <token>, a signed request
//	                                               → {"result": "SUCCESS", "code": "0", "biz_content": {"prepay_id", …}}
//	POST /telebirr/payment/v1/merchant/queryOrder  the same → {"biz_content": {"order_status", …}}
//	GET  /telebirr/checkout?<signed order>         the payer's page, asking for their number and PIN
//
// A request carries timestamp (Unix seconds), nonce_str, method, version
// 1.0, biz_content, sign and sign_type SHA256WithRSA. sign is RSA-PSS with
// SHA-256, in base64, by the merchant's key, over every field but sign and
// sign_type, biz_content's among them, as name=value sorted by name and
// joined with & (keys.go). The checkout link is signed the same way over
// appid, merch_code, nonce_str, prepay_id and timestamp. When the payer has
// paid or given up, telebirr posts the outcome to the order's notify_url,
// signed with telebirr's key, and sends the payer to its redirect_url.
//
// What the guide leaves open is the sandbox's choice, as it is the
// adapter's: order states (Pending, Completed, Canceled, Expired), error
// codes, and notifications tried 4 times.

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// order is a telebirr checkout order.
type order struct {
	PrepayID     string    `json:"prepay_id"`
	MerchOrderID string    `json:"merch_order_id"`
	AppID        string    `json:"appid"`
	MerchCode    string    `json:"merch_code"`
	Title        string    `json:"title"`
	Amount       int64     `json:"amount_santim"`
	NotifyURL    string    `json:"notify_url"`
	RedirectURL  string    `json:"redirect_url,omitempty"`
	Created      time.Time `json:"created"`
	Expires      time.Time `json:"expires"`
	Status       string    `json:"status"`
	Payer        string    `json:"payer,omitempty"`
	TransID      string    `json:"trans_id,omitempty"`
	PaymentID    string    `json:"payment_order_id,omitempty"`
	Finished     time.Time `json:"finished,omitzero"`
	// Notified is what became of the notification.
	Notified string `json:"notified,omitempty"`
	// request is the order's fields, to tell a repeat from a clash.
	request string
}

// telebirr's order states.
const (
	orderPending   = "Pending"
	orderCompleted = "Completed"
	orderCanceled  = "Canceled"
	orderExpired   = "Expired"
)

const telebirrTokenLife = time.Hour

// eat is Ethiopian time, telebirr's clock.
var eat = time.FixedZone("EAT", 3*60*60)

// telebirrError refuses a call as telebirr's gateway does: an HTTP error
// with errorCode and errorMsg. The codes are the sandbox's.
func telebirrError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"errorCode": code, "errorMsg": msg})
}

// telebirrFail answers a well-made call that can't be done.
func telebirrFail(w http.ResponseWriter, code, msg string) {
	writeJSON(w, http.StatusOK, map[string]string{"result": "FAIL", "code": code, "msg": msg})
}

// telebirrSuccess answers a call, signed with telebirr's key as its answers
// are.
func (s *Sandbox) telebirrSuccess(w http.ResponseWriter, biz map[string]string) {
	fields := maps.Clone(biz)
	fields["nonce_str"] = random(upperChars, 32)
	writeJSON(w, http.StatusOK, map[string]any{
		"result": "SUCCESS", "code": "0", "msg": "success", "nonce_str": fields["nonce_str"],
		"sign": sign(s.cfg.Telebirr.Key, fields), "sign_type": signType, "biz_content": biz,
	})
}

func (s *Sandbox) telebirrToken(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-APP-Key") != s.cfg.Telebirr.FabricAppID {
		note(r, "refused: X-APP-Key isn't the fabric app ID")
		telebirrError(w, http.StatusUnauthorized, "40100", "X-APP-Key is not a fabric app ID")
		return
	}
	var req struct {
		AppSecret string `json:"appSecret"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.AppSecret != s.cfg.Telebirr.AppSecret {
		note(r, "refused: wrong appSecret")
		telebirrError(w, http.StatusUnauthorized, "40100", "appSecret is wrong")
		return
	}
	token := s.issueToken(s.tbTokens, telebirrTokenLife)
	now := time.Now().In(eat)
	note(r, "signed in for %s", telebirrTokenLife)
	writeJSON(w, http.StatusOK, map[string]string{
		"token": token, "effectiveDate": now.Format("20060102150405"),
		"expirationDate": now.Add(telebirrTokenLife).Format("20060102150405"),
	})
}

// signedRequest reads one of Kiflet's API calls and checks it: the app key
// and token, then the method, the time, the nonce and the signature. It
// answers a call that fails a check, and returns biz_content otherwise.
func (s *Sandbox) signedRequest(w http.ResponseWriter, r *http.Request, method string) (map[string]string, bool) {
	refuse := func(status int, code, format string, args ...any) (map[string]string, bool) {
		msg := fmt.Sprintf(format, args...)
		note(r, "refused: %s", msg)
		telebirrError(w, status, code, msg)
		return nil, false
	}
	if r.Header.Get("X-APP-Key") != s.cfg.Telebirr.FabricAppID {
		return refuse(http.StatusUnauthorized, "40100", "X-APP-Key is not a fabric app ID")
	}
	if !s.validToken(s.tbTokens, r.Header.Get("Authorization")) {
		return refuse(http.StatusUnauthorized, "40101", "the token is unknown or has expired")
	}
	var req struct {
		Timestamp string         `json:"timestamp"`
		NonceStr  string         `json:"nonce_str"`
		Method    string         `json:"method"`
		Version   string         `json:"version"`
		Biz       map[string]any `json:"biz_content"`
		Sign      string         `json:"sign"`
		SignType  string         `json:"sign_type"`
	}
	if !readJSON(w, r, &req) {
		return nil, false
	}
	biz := map[string]string{}
	for k, v := range req.Biz {
		str, ok := v.(string)
		if !ok {
			return refuse(http.StatusBadRequest, "40001", "biz_content.%s is not a string", k)
		}
		biz[k] = str
	}
	ts, err := strconv.ParseInt(req.Timestamp, 10, 64)
	switch {
	case req.Method != method:
		return refuse(http.StatusBadRequest, "40001", "method is %q; this is %s", req.Method, method)
	case req.Version != "1.0":
		return refuse(http.StatusBadRequest, "40001", "version is %q, want 1.0", req.Version)
	case req.SignType != signType:
		return refuse(http.StatusBadRequest, "40001", "sign_type is %q, want %s", req.SignType, signType)
	case err != nil || time.Since(time.Unix(ts, 0)).Abs() > 5*time.Minute:
		return refuse(http.StatusBadRequest, "40001", "timestamp %q isn't the time now in Unix seconds", req.Timestamp)
	case req.NonceStr == "" || len(req.NonceStr) > 32:
		return refuse(http.StatusBadRequest, "40001", "nonce_str must be 1 to 32 characters")
	}
	fields := maps.Clone(biz)
	fields["timestamp"], fields["nonce_str"], fields["method"], fields["version"] = req.Timestamp, req.NonceStr, req.Method, req.Version
	if !verify(s.cfg.Telebirr.MerchantKey, fields, req.Sign) {
		return refuse(http.StatusBadRequest, "40002", "the signature doesn't match: sign every field but sign and sign_type, "+
			"biz_content's among them, as name=value sorted and joined with &, with the merchant's key (RSA-PSS, SHA-256)")
	}
	if biz["appid"] != s.cfg.Telebirr.MerchantAppID || biz["merch_code"] != s.cfg.Telebirr.MerchantCode {
		return refuse(http.StatusBadRequest, "40003", "appid %q and merch_code %q aren't a merchant of this sandbox", biz["appid"], biz["merch_code"])
	}
	if !s.fresh(req.NonceStr) {
		return refuse(http.StatusBadRequest, "40004", "nonce_str %q was used before", req.NonceStr)
	}
	return biz, true
}

// fresh records a nonce_str, reporting false when it was used in the last
// quarter of an hour.
func (s *Sandbox) fresh(nonce string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for n, at := range s.nonces {
		if now.Sub(at) > 15*time.Minute {
			delete(s.nonces, n)
		}
	}
	if _, used := s.nonces[nonce]; used {
		return false
	}
	s.nonces[nonce] = now
	return true
}

func (s *Sandbox) preOrder(w http.ResponseWriter, r *http.Request) {
	biz, ok := s.signedRequest(w, r, "payment.preorder")
	if !ok {
		return
	}
	if s.currentFaults().PreOrderFails {
		note(r, "refused: preOrder fails (fault)")
		telebirrError(w, http.StatusInternalServerError, "50000", "System busy, please try again later")
		return
	}
	o, problem := newOrder(biz)
	if problem != "" {
		note(r, "refused: %s", problem)
		telebirrError(w, http.StatusBadRequest, "40005", problem)
		return
	}
	s.mu.Lock()
	old := s.orderIDs[o.MerchOrderID]
	switch {
	case old != nil && old.request != o.request:
		s.mu.Unlock()
		note(r, "refused: order %s exists with other details", o.MerchOrderID)
		telebirrFail(w, "10011", "merch_order_id "+o.MerchOrderID+" is already used by another order")
		return
	case old != nil:
		o = old
	default:
		s.orders[o.PrepayID], s.orderIDs[o.MerchOrderID] = o, o
	}
	s.mu.Unlock()
	if old != nil {
		note(r, "order %s again: the same prepay_id", o.MerchOrderID)
	} else {
		note(r, "order %s for ETB %s", o.MerchOrderID, birr(o.Amount))
	}
	s.telebirrSuccess(w, map[string]string{"merch_order_id": o.MerchOrderID, "prepay_id": o.PrepayID})
}

// newOrder checks a preOrder's biz_content, returning the order or what is
// wrong with it.
func newOrder(biz map[string]string) (*order, string) {
	amount, amountErr := parseAmount(biz["total_amount"])
	minutes, minutesErr := strconv.Atoi(strings.TrimSuffix(biz["timeout_express"], "m"))
	switch {
	case biz["trade_type"] != "Checkout":
		return nil, fmt.Sprintf("trade_type is %q; the web checkout takes Checkout", biz["trade_type"])
	case biz["business_type"] != "BuyGoods":
		return nil, fmt.Sprintf("business_type is %q, want BuyGoods", biz["business_type"])
	case biz["trans_currency"] != "ETB":
		return nil, fmt.Sprintf("trans_currency is %q, want ETB", biz["trans_currency"])
	case !lettersAndDigits(biz["merch_order_id"]) || len(biz["merch_order_id"]) > 64:
		return nil, fmt.Sprintf("merch_order_id %q must be 1 to 64 letters and digits", biz["merch_order_id"])
	case !titleText(biz["title"]):
		return nil, fmt.Sprintf("title %q must be letters, digits and spaces, at most 512", biz["title"])
	case amountErr != nil || amount <= 0:
		return nil, fmt.Sprintf("total_amount %q is not an amount in birr with at most two decimals", biz["total_amount"])
	case !strings.HasSuffix(biz["timeout_express"], "m") || minutesErr != nil || minutes < 1 || minutes > 120:
		return nil, fmt.Sprintf("timeout_express %q is not minutes from 1m to 120m", biz["timeout_express"])
	case !webURL(biz["notify_url"]):
		return nil, fmt.Sprintf("notify_url %q is not an http or https URL", biz["notify_url"])
	case biz["redirect_url"] != "" && !webURL(biz["redirect_url"]):
		return nil, fmt.Sprintf("redirect_url %q is not an http or https URL", biz["redirect_url"])
	}
	now := time.Now()
	return &order{
		PrepayID: strings.ToLower(random(upperChars, 32)), MerchOrderID: biz["merch_order_id"],
		AppID: biz["appid"], MerchCode: biz["merch_code"], Title: biz["title"], Amount: amount,
		NotifyURL: biz["notify_url"], RedirectURL: biz["redirect_url"],
		Created: now, Expires: now.Add(time.Duration(minutes) * time.Minute), Status: orderPending,
		request: signedText(biz),
	}, ""
}

func (s *Sandbox) queryOrder(w http.ResponseWriter, r *http.Request) {
	biz, ok := s.signedRequest(w, r, "payment.queryorder")
	if !ok {
		return
	}
	s.mu.Lock()
	o, found := s.orderIDs[biz["merch_order_id"]]
	var snap order
	if found {
		s.expire(o)
		snap = *o
	}
	s.mu.Unlock()
	if !found {
		note(r, "no order %s", biz["merch_order_id"])
		telebirrFail(w, "10012", "no order has merch_order_id "+biz["merch_order_id"])
		return
	}
	out := map[string]string{
		"merch_order_id": snap.MerchOrderID, "order_status": snap.Status, "total_amount": decimal(snap.Amount),
		"trans_currency": "ETB", "create_time": strconv.FormatInt(snap.Created.Unix(), 10),
	}
	if snap.Status == orderCompleted {
		out["payment_order_id"], out["trans_id"] = snap.PaymentID, snap.TransID
		out["trans_time"] = strconv.FormatInt(snap.Finished.Unix(), 10)
	}
	note(r, "order %s is %s", snap.MerchOrderID, snap.Status)
	s.telebirrSuccess(w, out)
}

// expire closes an order whose time is up. s.mu must be held.
func (s *Sandbox) expire(o *order) {
	if o.Status == orderPending && time.Now().After(o.Expires) {
		o.Status, o.Finished = orderExpired, o.Expires
	}
}

// orderCopy is a copy of an order, by prepay_id.
func (s *Sandbox) orderCopy(prepayID string) (order, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := s.orders[prepayID]
	if o == nil {
		return order{}, false
	}
	s.expire(o)
	return *o, true
}

// recentOrders are the latest orders, newest first.
func (s *Sandbox) recentOrders(n int) []order {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]order, 0, len(s.orders))
	for _, o := range s.orders {
		s.expire(o)
		out = append(out, *o)
	}
	slices.SortFunc(out, func(a, b order) int { return b.Created.Compare(a.Created) })
	return out[:min(n, len(out))]
}

// checkoutFields are what the checkout link's signature covers.
var checkoutFields = []string{"appid", "merch_code", "nonce_str", "prepay_id", "timestamp"}

// checkout is the page Kiflet sends the payer to, with the signed order.
func (s *Sandbox) checkout(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	fields := map[string]string{}
	for _, k := range checkoutFields {
		fields[k] = q.Get(k)
	}
	var problem string
	switch {
	case q.Get("sign_type") != signType || q.Get("version") != "1.0" || q.Get("trade_type") != "Checkout":
		problem = "The link needs sign_type=" + signType + ", version=1.0 and trade_type=Checkout."
	case !verify(s.cfg.Telebirr.MerchantKey, fields, q.Get("sign")):
		problem = "The link's signature doesn't match it. It must be signed with the merchant's key, over " + strings.Join(checkoutFields, ", ") + "."
	case fields["appid"] != s.cfg.Telebirr.MerchantAppID || fields["merch_code"] != s.cfg.Telebirr.MerchantCode:
		problem = "The link is for another merchant."
	}
	o, found := s.orderCopy(fields["prepay_id"])
	if problem == "" && !found {
		problem = "telebirr has no order with this prepay_id."
	}
	if problem != "" {
		note(r, "checkout refused: %s", problem)
		s.renderCheckout(w, http.StatusBadRequest, checkoutView{Problem: problem})
		return
	}
	note(r, "checkout opened for order %s", o.MerchOrderID)
	s.renderCheckout(w, http.StatusOK, checkoutView{Order: o})
}

// checkoutAnswer is the payer paying or cancelling on the checkout page.
func (s *Sandbox) checkoutAnswer(w http.ResponseWriter, r *http.Request) {
	number := r.PostFormValue("phone")
	o, err := s.answerOrder(r.PathValue("prepay"), r.PostFormValue("action"), number, r.PostFormValue("pin"))
	var pe payerError
	switch {
	case errors.Is(err, errNoOrder):
		s.renderCheckout(w, http.StatusNotFound, checkoutView{Problem: "telebirr has no such order."})
	case errors.As(err, &pe):
		s.renderCheckout(w, http.StatusUnprocessableEntity, checkoutView{Order: o, Error: string(pe), Phone: number})
	case o.RedirectURL != "":
		// Paid, cancelled, or already closed: back to the merchant.
		http.Redirect(w, r, o.RedirectURL, http.StatusSeeOther)
	default:
		s.renderCheckout(w, http.StatusOK, checkoutView{Order: o})
	}
}

var (
	errNoOrder = errors.New("telebirr has no such order")
	errClosed  = errors.New("the order is no longer open")
)

// payerError is a problem the payer is shown, such as a wrong PIN.
type payerError string

func (e payerError) Error() string { return string(e) }

// answerOrder makes the payer's answer to an order: pay from the wallet of
// the number given, or cancel. Either way Kiflet is notified.
func (s *Sandbox) answerOrder(prepayID, action, number, pin string) (order, error) {
	o, err := s.closeOrder(prepayID, action, number, pin)
	if err == nil {
		s.notify(o)
	}
	return o, err
}

func (s *Sandbox) closeOrder(prepayID, action, number, pin string) (order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := s.orders[prepayID]
	if o == nil {
		return order{}, errNoOrder
	}
	s.expire(o)
	switch {
	case o.Status != orderPending:
		return *o, errClosed
	case action == "cancel":
		o.Status, o.Finished = orderCanceled, time.Now()
		return *o, nil
	case action != "pay":
		return *o, payerError("Choose Pay or Cancel.")
	}
	local, ok := localNumber(number)
	if !ok {
		return *o, payerError("Enter the telebirr number you pay from, such as 0911234567.")
	}
	if pin != s.cfg.PIN {
		return *o, payerError("Wrong PIN.")
	}
	p := s.phoneLocked(local)
	if p.Telebirr < o.Amount {
		return *o, payerError(fmt.Sprintf("Your telebirr balance, ETB %s, is less than ETB %s.", birr(p.Telebirr), birr(o.Amount)))
	}
	p.Telebirr -= o.Amount
	o.Status, o.Finished, o.Payer = orderCompleted, time.Now(), local
	o.TransID, o.PaymentID = "TB"+random(upperChars, 10), random(digitChars, 18)
	s.receive(p, "127", fmt.Sprintf("You have paid ETB %s to %s for %s. Transaction number %s. Your telebirr balance is ETB %s.",
		birr(o.Amount), s.cfg.AppName, o.Title, o.TransID, birr(p.Telebirr)))
	return *o, nil
}

// notify tells Kiflet how an order ended, as the faults say: held back,
// late, twice, signed with the wrong key, or just once.
func (s *Sandbox) notify(o order) {
	f := s.currentFaults()
	c := s.notification(o, f.WrongKey)
	calls := []*call{c}
	if f.DuplicateNotifications {
		calls = append(calls, s.again(c))
	}
	for _, c := range calls {
		switch {
		case f.DropNotifications:
			s.setNotified(o.PrepayID, "Held back (fault)")
			s.hold(c, "notifications are dropped (fault)")
		case f.DelayNotifications:
			s.setNotified(o.PrepayID, "Late (fault): due "+time.Now().Add(s.cfg.Timing.Delay).Format("15:04:05"))
			s.later(s.cfg.Timing.Delay, func() { s.deliver(c) })
		default:
			s.deliver(c)
		}
	}
}

// notification is the call telling Kiflet how an order ended.
func (s *Sandbox) notification(o order, forged bool) *call {
	fields := map[string]string{
		"notify_url": o.NotifyURL, "appid": o.AppID, "notify_time": strconv.FormatInt(time.Now().Unix(), 10),
		"merch_code": o.MerchCode, "merch_order_id": o.MerchOrderID, "payment_order_id": o.PaymentID,
		"total_amount": decimal(o.Amount), "trans_id": o.TransID, "trans_currency": "ETB",
		"trade_status": o.Status, "trans_end_time": strconv.FormatInt(o.Finished.Unix(), 10),
	}
	key, what := s.cfg.Telebirr.Key, "notification: order "+o.MerchOrderID+" "+o.Status
	if forged {
		key, what = s.forger, what+", signed with the wrong key (fault)"
	}
	fields["sign"], fields["sign_type"] = sign(key, fields), signType
	return s.newCall("telebirr", what, o.NotifyURL, jsonHeader(), mustJSON(fields), 4, func(a answer, last bool) {
		state := describeAnswer(a)
		if !a.ok() && !last {
			state = "Trying again: " + state
		}
		s.setNotified(o.PrepayID, state)
	})
}

func (s *Sandbox) setNotified(prepayID, state string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o := s.orders[prepayID]; o != nil {
		o.Notified = state
	}
}

func lettersAndDigits(s string) bool {
	for _, r := range s {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return s != ""
}

func titleText(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != ' ' {
			return false
		}
	}
	return strings.TrimSpace(s) != "" && len([]rune(s)) <= 512
}

func webURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
