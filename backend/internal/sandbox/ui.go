package sandbox

import (
	"bytes"
	"cmp"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

//go:embed templates static
var files embed.FS

var funcs = template.FuncMap{
	"birr":  birr,
	"clip":  clip,
	"phone": spaced,
	"clock": func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.Format("15:04:05")
	},
	// amount writes an amount as the providers carry it for people.
	"amount": func(s string) string {
		if v, err := parseAmount(s); err == nil {
			return birr(v)
		}
		return s
	},
	"lines":      func(l []string) string { return strings.Join(l, "\n") },
	"orderState": orderState,
	"statusClass": func(code int) string {
		switch {
		case code == 0:
			return "none"
		case code < 300:
			return "ok"
		case code < 500:
			return "warn"
		}
		return "bad"
	},
}

// spaced writes a local number as people read it: 0911 22 33 44.
func spaced(n string) string {
	if len(n) != 10 {
		return n
	}
	return n[:4] + " " + n[4:6] + " " + n[6:8] + " " + n[8:]
}

func orderState(status string) string {
	switch status {
	case orderPending:
		return "Waiting for the payer"
	case orderCompleted:
		return "Paid"
	case orderCanceled:
		return "Cancelled by the payer"
	case orderExpired:
		return "Expired"
	}
	return status
}

// parsePages parses each page with the layouts and parts they share.
func parsePages() (map[string]*template.Template, error) {
	base, err := template.New("").Funcs(funcs).ParseFS(files, "templates/layout.html")
	if err != nil {
		return nil, err
	}
	pages := map[string]*template.Template{}
	for _, name := range []string{"home", "phone", "checkout", "controls", "log"} {
		t, err := template.Must(base.Clone()).ParseFS(files, "templates/"+name+".html")
		if err != nil {
			return nil, fmt.Errorf("sandbox page %s: %w", name, err)
		}
		pages[name] = t
	}
	return pages, nil
}

func staticFiles() http.Handler {
	sub, err := fs.Sub(files, "static")
	if err != nil {
		panic(err)
	}
	return http.StripPrefix("/static/", http.FileServerFS(sub))
}

// view is what every page is given.
type view struct {
	Title    string
	Nav      string
	AppName  string
	AppURL   string
	PIN      string
	FaultsOn int
	Data     any
}

// render writes a page in a layout: "layout" with the navigation, "bare"
// for the checkout, which plays telebirr's own page.
func (s *Sandbox) render(w http.ResponseWriter, status int, page, layout string, v view) {
	v.AppName, v.AppURL, v.PIN, v.FaultsOn = s.cfg.AppName, s.cfg.AppURL, s.cfg.PIN, s.currentFaults().on()
	var b bytes.Buffer
	if err := s.pages[page].ExecuteTemplate(&b, layout, v); err != nil {
		http.Error(w, "sandbox page "+page+": "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(b.Bytes())
}

// ---------------------------------------------------------------------------
// Home
// ---------------------------------------------------------------------------

type homeView struct {
	Phones   []phoneRow
	Held     []call
	Orders   []order
	Payments []cbePayment
	Events   []Event
	Env      string
	Error    string
}

type phoneRow struct {
	Number   string
	Telebirr int64
	CBEBirr  int64
	Last     *text
}

func (s *Sandbox) homePage(w http.ResponseWriter, r *http.Request) {
	s.renderHome(w, r, http.StatusOK, "")
}

func (s *Sandbox) renderHome(w http.ResponseWriter, r *http.Request, status int, problem string) {
	v := homeView{Held: s.heldCalls(), Orders: s.recentOrders(15), Payments: s.recentPayments(15), Env: s.envFor(r), Error: problem}
	s.mu.Lock()
	for _, p := range s.phones {
		row := phoneRow{Number: p.Number, Telebirr: p.Telebirr, CBEBirr: p.CBEBirr}
		if n := len(p.Texts); n > 0 {
			last := p.Texts[n-1]
			row.Last = &last
		}
		v.Phones = append(v.Phones, row)
	}
	v.Events = slices.Clone(s.events[max(0, len(s.events)-8):])
	s.mu.Unlock()
	slices.Reverse(v.Events)
	// The latest texted first, and those with no texts after them.
	lastAt := func(p phoneRow) time.Time {
		if p.Last == nil {
			return time.Time{}
		}
		return p.Last.At
	}
	slices.SortFunc(v.Phones, func(a, b phoneRow) int {
		return cmp.Or(lastAt(b).Compare(lastAt(a)), strings.Compare(a.Number, b.Number))
	})
	s.render(w, status, "home", "layout", view{Title: "Phones", Nav: "home", Data: v})
}

// envFor is the settings Kiflet needs to reach this sandbox, at the
// address the browser reached it by.
func (s *Sandbox) envFor(r *http.Request) string {
	base := "http://" + r.Host
	if r.TLS != nil {
		base = "https://" + r.Host
	}
	c := s.cfg
	return strings.Join([]string{
		"SMS_PROVIDER=ethiotelecom",
		"ETHIO_TELECOM_API_URL=" + base + "/ethiotelecom",
		"ETHIO_TELECOM_API_KEY=" + c.SMS.APIKey,
		"ETHIO_TELECOM_SENDER_ID=" + c.SMS.SenderID,
		"SMS_NUMBER=" + c.SMS.Number,
		"SMS_INBOUND_SECRET=" + c.SMS.Secret,
		"PAYMENT_PROVIDER=telebirr,cbebirr",
		"TELEBIRR_API_URL=" + base + "/telebirr",
		"TELEBIRR_CHECKOUT_URL=" + base + "/telebirr/checkout?",
		"TELEBIRR_FABRIC_APP_ID=" + c.Telebirr.FabricAppID,
		"TELEBIRR_APP_SECRET=" + c.Telebirr.AppSecret,
		"TELEBIRR_MERCHANT_APP_ID=" + c.Telebirr.MerchantAppID,
		"TELEBIRR_MERCHANT_CODE=" + c.Telebirr.MerchantCode,
		"TELEBIRR_PRIVATE_KEY_FILE=sandbox-keys/" + MerchantPrivateKey,
		"TELEBIRR_PUBLIC_KEY_FILE=sandbox-keys/" + TelebirrPublicKey,
		"CBEBIRR_MERCHANT_ID=" + c.CBEBirr.MerchantID,
		"CBEBIRR_CALLBACK_USERNAME=" + c.CBEBirr.Username,
		"CBEBIRR_CALLBACK_PASSWORD=" + c.CBEBirr.Password,
		"SANDBOX_URL=" + base,
	}, "\n")
}

func (s *Sandbox) openPhone(w http.ResponseWriter, r *http.Request) {
	number, ok := localNumber(r.URL.Query().Get("number"))
	if !ok {
		s.renderHome(w, r, http.StatusUnprocessableEntity, "That isn't an Ethiopian mobile number, such as 0911234567.")
		return
	}
	http.Redirect(w, r, "/phones/"+number, http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// A phone
// ---------------------------------------------------------------------------

type phoneView struct {
	Phone     phone
	ShortCode string
	SenderID  string
	Pay       cbeStep
	// Paid is the payment just made, shown until the page is left.
	Paid        *cbePayment
	WalletError string
}

// cbeStep is where the payer is in paying a merchant with CBE Birr: keying
// in the merchant and the reference (start), or confirming what the
// merchant's bill says (confirm).
type cbeStep struct {
	Step     string
	Merchant string
	Ref      string
	Amount   string
	Bill     cbeBill
	Error    string
}

// number reads the phone a request is for, answering one that isn't.
func (s *Sandbox) number(w http.ResponseWriter, r *http.Request) (string, bool) {
	number, ok := localNumber(r.PathValue("number"))
	if !ok {
		s.renderHome(w, r, http.StatusNotFound, fmt.Sprintf("%q isn't an Ethiopian mobile number, such as 0911234567.", r.PathValue("number")))
	}
	return number, ok
}

func (s *Sandbox) phonePage(w http.ResponseWriter, r *http.Request) {
	number, ok := s.number(w, r)
	if !ok {
		return
	}
	if number != r.PathValue("number") {
		target := "/phones/" + number
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusFound)
		return
	}
	q := r.URL.Query()
	v := phoneView{Pay: cbeStep{Step: "start", Merchant: q.Get("merchant"), Ref: q.Get("ref")}}
	if v.Pay.Merchant == "" {
		v.Pay.Merchant = s.cfg.CBEBirr.MerchantID
	}
	if p, ok := s.paymentCopy(q.Get("paid")); ok && p.Phone == number {
		v.Paid = &p
	}
	s.renderPhone(w, http.StatusOK, number, v)
}

func (s *Sandbox) renderPhone(w http.ResponseWriter, status int, number string, v phoneView) {
	v.Phone, v.ShortCode, v.SenderID = s.phoneCopy(number), s.cfg.SMS.Number, s.cfg.SMS.SenderID
	s.render(w, status, "phone", "layout", view{Title: spaced(number), Nav: "home", Data: v})
}

func (s *Sandbox) phoneText(w http.ResponseWriter, r *http.Request) {
	number, ok := s.number(w, r)
	if !ok {
		return
	}
	if body := strings.TrimSpace(r.PostFormValue("text")); body != "" {
		s.textKiflet(number, body)
	}
	http.Redirect(w, r, "/phones/"+number+"#thread", http.StatusSeeOther)
}

func (s *Sandbox) phoneBalances(w http.ResponseWriter, r *http.Request) {
	number, ok := s.number(w, r)
	if !ok {
		return
	}
	telebirr, err1 := parseBirr(r.PostFormValue("telebirr"))
	cbe, err2 := parseBirr(r.PostFormValue("cbebirr"))
	if err1 != nil || err2 != nil {
		s.renderPhone(w, http.StatusUnprocessableEntity, number, phoneView{
			Pay:         cbeStep{Step: "start", Merchant: s.cfg.CBEBirr.MerchantID},
			WalletError: "Enter each balance in birr, such as 50000 or 1,250.50.",
		})
		return
	}
	s.setBalances(number, &telebirr, &cbe)
	http.Redirect(w, r, "/phones/"+number+"#wallets", http.StatusSeeOther)
}

// setBalances changes what a phone's wallets hold; nil leaves one as it is.
func (s *Sandbox) setBalances(number string, telebirr, cbe *int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.phoneLocked(number)
	if telebirr != nil {
		p.Telebirr = *telebirr
	}
	if cbe != nil {
		p.CBEBirr = *cbe
	}
}

func (s *Sandbox) phoneCBEBirr(w http.ResponseWriter, r *http.Request) {
	number, ok := s.number(w, r)
	if !ok {
		return
	}
	st := cbeStep{Merchant: strings.TrimSpace(r.PostFormValue("merchant")), Ref: strings.TrimSpace(r.PostFormValue("ref"))}
	switch r.PostFormValue("step") {
	case "query":
		bill, err := s.cbeQuery(number, st.Merchant, st.Ref)
		switch {
		case err != nil:
			st.Step, st.Error = "start", err.Error()
		case bill.ResultCode != "0":
			st.Step, st.Error = "start", bill.ResultDesc
		default:
			st.Step, st.Bill, st.Amount = "confirm", bill, bill.Amount
		}
	case "pay":
		st.Step, st.Amount = "confirm", strings.TrimSpace(r.PostFormValue("amount"))
		st.Bill = cbeBill{Amount: r.PostFormValue("billed"), CustomerName: r.PostFormValue("customer"), Description: r.PostFormValue("description")}
		amount, err := parseBirr(st.Amount)
		if err != nil {
			st.Error = "Enter the amount in birr, such as 2875.00."
			break
		}
		p, err := s.cbePay(number, st.Merchant, st.Ref, amount, r.PostFormValue("pin"))
		if err != nil {
			st.Error = err.Error()
			break
		}
		http.Redirect(w, r, "/phones/"+number+"?paid="+p.TransID+"#pay", http.StatusSeeOther)
		return
	default:
		st.Step = "start"
	}
	status := http.StatusOK
	if st.Error != "" {
		status = http.StatusUnprocessableEntity
	}
	s.renderPhone(w, status, number, phoneView{Pay: st})
}

// ---------------------------------------------------------------------------
// The checkout, faults and the log
// ---------------------------------------------------------------------------

type checkoutView struct {
	Order order
	// Problem is why the link can't be used, and Error why the payer's
	// answer wasn't taken.
	Problem string
	Error   string
	Phone   string
}

func (s *Sandbox) renderCheckout(w http.ResponseWriter, status int, v checkoutView) {
	s.render(w, status, "checkout", "bare", view{Title: "telebirr checkout", Data: v})
}

type controlsView struct {
	Faults Faults
	Error  string
	Notice string
}

func (s *Sandbox) controlsPage(w http.ResponseWriter, r *http.Request) {
	v := controlsView{Faults: s.currentFaults()}
	switch {
	case r.URL.Query().Has("saved"):
		v.Notice = "Saved."
	case r.URL.Query().Has("expired"):
		v.Notice = "Every token is expired: Kiflet's next call is refused, and it must sign in again."
	}
	s.render(w, http.StatusOK, "controls", "layout", view{Title: "Faults", Nav: "controls", Data: v})
}

func (s *Sandbox) setControls(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f := faultsFromForm(r.PostForm)
	if err := s.setFaults(f); err != nil {
		s.render(w, http.StatusUnprocessableEntity, "controls", "layout", view{Title: "Faults", Nav: "controls", Data: controlsView{Faults: f, Error: err.Error()}})
		return
	}
	http.Redirect(w, r, "/controls?saved", http.StatusSeeOther)
}

func (s *Sandbox) expireTokensPage(w http.ResponseWriter, r *http.Request) {
	s.expireTokens()
	http.Redirect(w, r, "/controls?expired", http.StatusSeeOther)
}

func (s *Sandbox) heldAction(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	action := r.PostFormValue("action")
	if err != nil || (action != "send" && action != "discard") {
		http.Error(w, "send or discard a held call", http.StatusBadRequest)
		return
	}
	s.release(id, action == "send")
	back := r.PostFormValue("back")
	if !strings.HasPrefix(back, "/") || strings.HasPrefix(back, "//") {
		back = "/"
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

type logView struct {
	Events   []Event
	Service  string
	Services []string
}

func (s *Sandbox) logPage(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	events := s.eventsAfter(0, service)
	slices.Reverse(events)
	v := logView{Events: events[:min(len(events), 200)], Service: service, Services: []string{"SMS", "telebirr", "CBE Birr"}}
	s.render(w, http.StatusOK, "log", "layout", view{Title: "Log", Nav: "log", Data: v})
}
