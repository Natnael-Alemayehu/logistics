package web

import (
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/service"
	"github.com/natnael-alemayehu/kiflet/internal/text"
)

// The pages customers see (tracking, and confirming a delivery) are in
// English and Amharic, from the catalogue in internal/text. The rest of the
// site is English, so its views keep Lang at "en".

const langCookie = "lang"

// customerPage reports whether the request is for one of those pages.
func customerPage(r *http.Request) bool {
	p := r.URL.Path
	return p == "/track" || strings.HasPrefix(p, "/track/") || strings.HasPrefix(p, "/c/")
}

// lang is the language to show a customer's page in: the one they chose
// with the switch (?lang=, remembered in a cookie), else company, the
// language the shipment's company writes to its customers in, else what
// the browser asks for, else English.
//
// Most phones here are set to English whatever their owner reads, which is
// why the company's choice comes before the browser's. A page shown in
// Amharic for that reason remembers it, so the pages it links to, which
// know no company, are in Amharic too.
func (s *Server) lang(w http.ResponseWriter, r *http.Request, company string) string {
	remember := func(l string) {
		http.SetCookie(w, &http.Cookie{
			Name: langCookie, Value: l, Path: "/", MaxAge: 365 * 24 * 3600,
			HttpOnly: true, Secure: s.cfg.SecureCookies(), SameSite: http.SameSiteLaxMode,
		})
	}
	if l := r.URL.Query().Get("lang"); text.Valid(l) {
		remember(l)
		return l
	}
	if c, err := r.Cookie(langCookie); err == nil && text.Valid(c.Value) {
		return c.Value
	}
	if text.Valid(company) {
		if company != text.English {
			remember(company)
		}
		return company
	}
	if l := text.Accept(r.Header.Get("Accept-Language")); l != "" {
		return l
	}
	return text.English
}

// chosenLang is the language the visitor chose with the switch, or "".
func chosenLang(r *http.Request) string {
	if l := r.URL.Query().Get("lang"); text.Valid(l) {
		return l
	}
	if c, err := r.Cookie(langCookie); err == nil && text.Valid(c.Value) {
		return c.Value
	}
	return ""
}

// customerView starts a View for a customer's page in lang, which gets the
// language switch.
func (s *Server) customerView(w http.ResponseWriter, r *http.Request, lang, title string, data any) *View {
	v := s.view(w, r, title, "", data)
	v.Lang = lang
	v.Switch = true
	return v
}

// message is a domain error's message in lang, for the errors that have
// one there.
func message(lang string, e *service.Error) string {
	if e.Key != "" && lang != text.English {
		return text.T(lang, e.Key)
	}
	return e.Message
}

// T is the catalogue's text for key in the page's language.
func (v *View) T(key string, args ...any) string { return text.T(v.Lang, key, args...) }

// TH is T for a key ending in _html.
func (v *View) TH(key string, args ...any) template.HTML { return text.HTML(v.Lang, key, args...) }

// Status is a shipment's state as a customer is told it.
func (v *View) Status(status string) string { return text.Status(v.Lang, status) }

// When, Date and Ago write a moment in the page's language.
func (v *View) When(x any) string {
	t, ok := toTime(x)
	if !ok {
		return "—"
	}
	return text.When(v.Lang, t.In(v.loc), v.Now.In(v.loc))
}

func (v *View) Date(x any) string {
	t, ok := toTime(x)
	if !ok {
		return "—"
	}
	return text.Date(v.Lang, t.In(v.loc))
}

func (v *View) Ago(x any) string {
	t, ok := toTime(x)
	if !ok {
		return "—"
	}
	return text.Ago(v.Lang, time.Since(t))
}

// OtherLangs are the languages the switch offers: every one but the page's.
func (v *View) OtherLangs() []text.Language {
	var out []text.Language
	for _, l := range text.Languages {
		if l.Code != v.Lang {
			out = append(out, l)
		}
	}
	return out
}

// SwitchPath is where the switch's links go: this page, or, when this page
// answered a form, the page that form is on.
func (v *View) SwitchPath() string {
	return strings.TrimSuffix(v.Path, "/verify")
}
