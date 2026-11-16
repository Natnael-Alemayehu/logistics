// Package text holds the words customers read, in English and Amharic: the
// texts they are sent and the tracking and confirmation pages. Each language
// is one file, so a reviewer who reads Amharic needs only am.go beside en.go.
//
// The dashboard and the mail to the office are still English only, and the
// driver app keeps its own words in static/driver/text.js, because it works
// offline.
package text

import (
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/model"
)

// The languages a customer can be addressed in.
const (
	English = "en"
	Amharic = "am"
)

// Languages lists them with the name each goes by in its own script, for a
// language switch or a settings form.
var Languages = []Language{
	{Code: English, Name: "English"},
	{Code: Amharic, Name: "አማርኛ"},
}

// Language is one entry of Languages.
type Language struct {
	Code string
	Name string
}

var catalogues = map[string]map[string]string{English: en, Amharic: am}

// Valid reports whether lang is one of Languages.
func Valid(lang string) bool {
	_, ok := catalogues[lang]
	return ok
}

// Or returns lang if it is valid, else English.
func Or(lang string) string {
	if Valid(lang) {
		return lang
	}
	return English
}

// Has reports whether key is in the catalogue.
func Has(key string) bool {
	_, ok := en[key]
	return ok
}

// T is the text for key in lang, with args filled in as fmt.Sprintf does.
// A sentence whose words come in another order in Amharic names its
// arguments by position (%[2]s). A key missing from lang falls back to
// English, and one missing from both comes back as it is, which a test
// keeps from happening.
func T(lang, key string, args ...any) string {
	f, ok := catalogues[Or(lang)][key]
	if !ok {
		if f, ok = en[key]; !ok {
			return key
		}
	}
	if len(args) == 0 {
		return f
	}
	return fmt.Sprintf(f, args...)
}

// HTML is T for a key ending in _html: the catalogue's markup is kept and
// the arguments are escaped.
func HTML(lang, key string, args ...any) template.HTML {
	for i, a := range args {
		switch v := a.(type) {
		case string:
			args[i] = template.HTMLEscapeString(v)
		case template.URL:
			args[i] = template.HTMLEscapeString(string(v))
		}
	}
	return template.HTML(T(lang, key, args...)) //nolint:gosec // the markup is the catalogue's own, and string arguments are escaped above
}

// Status is a shipment's state as a customer is told it. It softens the
// office's words: a problem on the road is a delay to the customer while
// the company deals with it.
func Status(lang, status string) string {
	if !Has("status." + status) {
		return model.StatusLabel(status)
	}
	return T(lang, "status."+status)
}

// Reason is why a shipment is late. A reason that isn't one of the listed
// codes is prose from an older client or a text message, and passes through.
func Reason(lang, code string) string {
	if lang == Amharic {
		if l, ok := am["reason."+code]; ok {
			return l
		}
	}
	return model.ReasonLabel(code)
}

// Accept picks a language from an Accept-Language header: the first of
// ours it names, or "" if it names neither.
func Accept(header string) string {
	for part := range strings.SplitSeq(header, ",") {
		tag, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		tag, _, _ = strings.Cut(strings.ToLower(tag), "-")
		if Valid(tag) {
			return tag
		}
	}
	return ""
}

// Dates stay in the Gregorian calendar in both languages, with the months
// spelled out in Amharic so that nobody reads 16/11 as a day of Hamle.

var amDays = [...]string{"እሑድ", "ሰኞ", "ማክሰኞ", "ረቡዕ", "ሐሙስ", "ዓርብ", "ቅዳሜ"}

var amMonths = [...]string{
	"ጃንዩወሪ", "ፌብሩወሪ", "ማርች", "ኤፕሪል", "ሜይ", "ጁን",
	"ጁላይ", "ኦገስት", "ሴፕቴምበር", "ኦክቶበር", "ኖቬምበር", "ዲሴምበር",
}

// When writes a moment for a page: the day, date and time, with the year
// only when it isn't now's. t must already be in the zone to show.
func When(lang string, t, now time.Time) string {
	if lang != Amharic {
		if t.Year() == now.Year() {
			return t.Format("Mon 2 Jan, 15:04")
		}
		return t.Format("2 Jan 2006, 15:04")
	}
	s := fmt.Sprintf("%s %s %d", amDays[t.Weekday()], amMonths[t.Month()-1], t.Day())
	if t.Year() != now.Year() {
		s = fmt.Sprintf("%s %d %d", amMonths[t.Month()-1], t.Day(), t.Year())
	}
	return s + "፣ " + t.Format("15:04")
}

// ShortWhen writes a moment for a text message, where every character
// counts: no year.
func ShortWhen(lang string, t time.Time) string {
	if lang != Amharic {
		return t.Format("Mon 2 Jan 15:04")
	}
	return fmt.Sprintf("%s %s %d %s", amDays[t.Weekday()], amMonths[t.Month()-1], t.Day(), t.Format("15:04"))
}

// Date writes a day with its year.
func Date(lang string, t time.Time) string {
	if lang != Amharic {
		return t.Format("2 Jan 2006")
	}
	return fmt.Sprintf("%s %d %d", amMonths[t.Month()-1], t.Day(), t.Year())
}

// Ago writes how long ago something happened.
func Ago(lang string, d time.Duration) string {
	d = max(d, 0)
	switch {
	case d < time.Minute:
		return T(lang, "ago.now")
	case d < time.Hour:
		return T(lang, "ago.minutes", int(d/time.Minute))
	case d < 24*time.Hour:
		return T(lang, "ago.hours", int(d/time.Hour))
	case d < 48*time.Hour:
		return T(lang, "ago.yesterday")
	}
	return T(lang, "ago.days", int(d/(24*time.Hour)))
}
