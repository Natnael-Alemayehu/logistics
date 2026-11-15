package sandbox

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// api serves the JSON API, for tests and scripts. Amounts are in santim.
//
//	GET   /api/faults                     the faults (faults.go)
//	PUT   /api/faults                     set them all
//	PATCH /api/faults                     set those given
//	POST  /api/tokens/expire              expire every token issued
//	GET   /api/phones/{number}            a phone: its texts and wallets
//	POST  /api/phones/{number}/texts      {"text"}: text Kiflet from the phone
//	PUT   /api/phones/{number}/balances   {"telebirr_santim", "cbebirr_santim"}
//	GET   /api/telebirr/orders            the orders, newest first
//	POST  /api/telebirr/orders/{prepay}   {"action": "pay" or "cancel", "phone", "pin"}
//	POST  /api/cbebirr/payments           {"phone", "merchant", "reference", "amount_santim", "pin"}
//	GET   /api/held                       the calls held back
//	POST  /api/held/{id}                  {"action": "send" or "discard"}
//	GET   /api/events?after=&service=     the log, oldest first
func (s *Sandbox) api(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/faults", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.currentFaults())
	})
	mux.HandleFunc("PUT /api/faults", s.apiFaults(false))
	mux.HandleFunc("PATCH /api/faults", s.apiFaults(true))
	mux.HandleFunc("POST /api/tokens/expire", func(w http.ResponseWriter, r *http.Request) {
		s.expireTokens()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/phones/{number}", s.apiPhone)
	mux.HandleFunc("POST /api/phones/{number}/texts", s.apiText)
	mux.HandleFunc("PUT /api/phones/{number}/balances", s.apiBalances)
	mux.HandleFunc("GET /api/telebirr/orders", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.recentOrders(100))
	})
	mux.HandleFunc("POST /api/telebirr/orders/{prepay}", s.apiAnswerOrder)
	mux.HandleFunc("POST /api/cbebirr/payments", s.apiCBEBirrPay)
	mux.HandleFunc("GET /api/held", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.heldCalls())
	})
	mux.HandleFunc("POST /api/held/{id}", s.apiHeld)
	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		after, _ := strconv.Atoi(r.URL.Query().Get("after"))
		writeJSON(w, http.StatusOK, s.eventsAfter(after, r.URL.Query().Get("service")))
	})
}

func apiError(w http.ResponseWriter, status int, msg string, extra map[string]any) {
	body := map[string]any{"error": msg}
	for k, v := range extra {
		body[k] = v
	}
	writeJSON(w, status, body)
}

func (s *Sandbox) apiFaults(merge bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var f Faults
		if merge {
			f = s.currentFaults()
		}
		if !readJSON(w, r, &f) {
			return
		}
		if err := s.setFaults(f); err != nil {
			apiError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		writeJSON(w, http.StatusOK, s.currentFaults())
	}
}

// apiNumber reads the phone a call is for, answering one that isn't.
func apiNumber(w http.ResponseWriter, raw string) (string, bool) {
	number, ok := localNumber(raw)
	if !ok {
		apiError(w, http.StatusNotFound, "not an Ethiopian mobile number: "+raw, nil)
	}
	return number, ok
}

func (s *Sandbox) apiPhone(w http.ResponseWriter, r *http.Request) {
	if number, ok := apiNumber(w, r.PathValue("number")); ok {
		writeJSON(w, http.StatusOK, s.phoneCopy(number))
	}
}

func (s *Sandbox) apiText(w http.ResponseWriter, r *http.Request) {
	number, ok := apiNumber(w, r.PathValue("number"))
	if !ok {
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		apiError(w, http.StatusBadRequest, "text is empty", nil)
		return
	}
	t, a := s.textKiflet(number, strings.TrimSpace(req.Text))
	out := map[string]any{"text": t, "kiflet_status": a.Status}
	if json.Valid(a.Body) {
		out["kiflet_answer"] = json.RawMessage(a.Body)
	}
	if a.Err != nil {
		apiError(w, http.StatusBadGateway, "Kiflet didn't answer: "+a.Err.Error(), out)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Sandbox) apiBalances(w http.ResponseWriter, r *http.Request) {
	number, ok := apiNumber(w, r.PathValue("number"))
	if !ok {
		return
	}
	var req struct {
		Telebirr *int64 `json:"telebirr_santim"`
		CBEBirr  *int64 `json:"cbebirr_santim"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if (req.Telebirr != nil && *req.Telebirr < 0) || (req.CBEBirr != nil && *req.CBEBirr < 0) {
		apiError(w, http.StatusBadRequest, "a balance can't be negative", nil)
		return
	}
	s.setBalances(number, req.Telebirr, req.CBEBirr)
	writeJSON(w, http.StatusOK, s.phoneCopy(number))
}

func (s *Sandbox) apiAnswerOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"`
		Phone  string `json:"phone"`
		PIN    string `json:"pin"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	o, err := s.answerOrder(r.PathValue("prepay"), req.Action, req.Phone, req.PIN)
	var pe payerError
	switch {
	case errors.Is(err, errNoOrder):
		apiError(w, http.StatusNotFound, err.Error(), nil)
	case errors.Is(err, errClosed):
		apiError(w, http.StatusConflict, err.Error(), map[string]any{"order": o})
	case errors.As(err, &pe):
		apiError(w, http.StatusUnprocessableEntity, pe.Error(), map[string]any{"order": o})
	default:
		writeJSON(w, http.StatusOK, map[string]any{"order": o, "redirect": o.RedirectURL})
	}
}

// apiCBEBirrPay pays a reference from a phone, as the phone page does: the
// query, then the payment. With no amount, it pays what the bill says.
func (s *Sandbox) apiCBEBirrPay(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone     string `json:"phone"`
		Merchant  string `json:"merchant"`
		Reference string `json:"reference"`
		Amount    int64  `json:"amount_santim"`
		PIN       string `json:"pin"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	number, ok := apiNumber(w, req.Phone)
	if !ok {
		return
	}
	if req.Merchant == "" {
		req.Merchant = s.cfg.CBEBirr.MerchantID
	}
	bill, err := s.cbeQuery(number, req.Merchant, strings.TrimSpace(req.Reference))
	if err != nil {
		apiError(w, http.StatusUnprocessableEntity, err.Error(), nil)
		return
	}
	if bill.ResultCode != "0" {
		apiError(w, http.StatusUnprocessableEntity, bill.ResultDesc, map[string]any{"bill": bill})
		return
	}
	if req.Amount == 0 {
		if req.Amount, err = parseAmount(bill.Amount); err != nil {
			apiError(w, http.StatusBadGateway, "the bill's amount can't be read: "+bill.Amount, map[string]any{"bill": bill})
			return
		}
	}
	p, err := s.cbePay(number, req.Merchant, strings.TrimSpace(req.Reference), req.Amount, req.PIN)
	if err != nil {
		apiError(w, http.StatusUnprocessableEntity, err.Error(), map[string]any{"bill": bill})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"payment": p, "bill": bill})
}

func (s *Sandbox) apiHeld(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	var req struct {
		Action string `json:"action"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if err != nil || (req.Action != "send" && req.Action != "discard") {
		apiError(w, http.StatusBadRequest, `action is "send" or "discard"`, nil)
		return
	}
	if !s.release(id, req.Action == "send") {
		apiError(w, http.StatusNotFound, "no held call "+r.PathValue("id"), nil)
		return
	}
	writeJSON(w, http.StatusOK, s.heldCalls())
}
