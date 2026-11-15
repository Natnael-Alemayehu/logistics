package web

import (
	"errors"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
	"github.com/natnael-alemayehu/kiflet/internal/phone"
	"github.com/natnael-alemayehu/kiflet/internal/service"
	"github.com/natnael-alemayehu/kiflet/pkg/payments"
)

// invoiceKind names what an invoice is for.
func invoiceKind(kind string) string {
	switch kind {
	case "subscription":
		return "First period"
	case "upgrade":
		return "Upgrade"
	case "renewal":
		return "Renewal"
	case "usage":
		return "Text messages"
	}
	return kind
}

// mark escapes s and wraps each occurrence of term in it, ignoring case,
// in <mark>, to show what a search matched (FR-009 AC-009.5).
func mark(term, s string) template.HTML {
	t := []rune(strings.TrimSpace(term))
	if len(t) == 0 {
		return template.HTML(template.HTMLEscapeString(s))
	}
	r := []rune(s)
	var b strings.Builder
	for i := 0; i < len(r); {
		if i+len(t) <= len(r) && strings.EqualFold(string(r[i:i+len(t)]), string(t)) {
			b.WriteString("<mark>" + template.HTMLEscapeString(string(r[i:i+len(t)])) + "</mark>")
			i += len(t)
			continue
		}
		b.WriteString(template.HTMLEscapeString(string(r[i])))
		i++
	}
	return template.HTML(b.String())
}

// truncateText shortens s to at most n characters, ending with an ellipsis
// when it cuts.
func truncateText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

var errBadID = errors.New("malformed id")

// pathID parses a UUID route parameter.
func pathID(r *http.Request, name string) (pgtype.UUID, error) {
	return parseID(chi.URLParam(r, name))
}

func parseID(s string) (pgtype.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, errBadID
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}

// toTime unwraps the time representations templates receive.
func toTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, !t.IsZero()
	case *time.Time:
		if t == nil {
			return time.Time{}, false
		}
		return *t, !t.IsZero()
	case pgtype.Timestamptz:
		return t.Time, t.Valid
	case pgtype.Date:
		return t.Time, t.Valid
	}
	return time.Time{}, false
}

func (s *Server) funcs() template.FuncMap {
	loc := s.cfg.Timezone
	return template.FuncMap{
		"asset": func(p string) string { return "/static/" + p + "?v=" + s.version },

		"statusLabel": model.StatusLabel,
		"reasonLabel": model.ReasonLabel,
		"roleLabel":   model.RoleLabel,
		"planName":    model.PlanName,
		"alertLabel":  service.AlertLabel,
		"heldReason":  service.HeldReasonLabel,
		"vehicleType": service.VehicleTypeLabel,
		"reasonsFor":  model.ReasonsFor,
		"isFinal":     model.IsFinalStatus,
		"isOnRoad":    model.IsOnRoad,
		"shortRef":    service.ShortRef,

		// when formats a moment in the operating time zone; "—" for none.
		"when": func(v any) string {
			t, ok := toTime(v)
			if !ok {
				return "—"
			}
			t = t.In(loc)
			now := time.Now().In(loc)
			if t.Year() == now.Year() {
				return t.Format("Mon 2 Jan, 15:04")
			}
			return t.Format("2 Jan 2006, 15:04")
		},
		"date": func(v any) string {
			t, ok := toTime(v)
			if !ok {
				return "—"
			}
			return t.In(loc).Format("2 Jan 2006")
		},
		"isoDate": func(v any) string {
			t, ok := toTime(v)
			if !ok {
				return ""
			}
			return t.In(loc).Format("2006-01-02")
		},
		// localInput formats for <input type="datetime-local">.
		"localInput": func(v any) string {
			t, ok := toTime(v)
			if !ok {
				return ""
			}
			return t.In(loc).Format("2006-01-02T15:04")
		},
		"rfc3339": func(v any) string {
			t, ok := toTime(v)
			if !ok {
				return ""
			}
			return t.UTC().Format(time.RFC3339)
		},
		"ago": func(v any) string {
			t, ok := toTime(v)
			if !ok {
				return "never"
			}
			return relative(time.Since(t))
		},
		"until": func(v any) string {
			t, ok := toTime(v)
			if !ok {
				return ""
			}
			d := time.Until(t)
			switch {
			case d > -time.Minute && d < time.Minute:
				return "now"
			case d < 0:
				return span(-d) + " late"
			}
			return "in " + span(d)
		},
		"isActive": model.IsActiveStatus,
		// newestFirst lists up to n of a trip's positions, newest first.
		"newestFirst": func(n int, pts []db.ListShipmentRouteRow) []db.ListShipmentRouteRow {
			out := make([]db.ListShipmentRouteRow, 0, min(n, len(pts)))
			for i := len(pts) - 1; i >= 0 && len(out) < n; i-- {
				out = append(out, pts[i])
			}
			return out
		},
		"device": device,
		// storage writes a size in kilobytes the way a phone's settings do.
		"storage": func(kb *int32) string {
			if kb == nil {
				return ""
			}
			switch k := float64(*kb); {
			case k >= 1024*1024:
				return fmt.Sprintf("%.1f GB", k/1024/1024)
			case k >= 1024:
				return fmt.Sprintf("%.0f MB", k/1024)
			default:
				return fmt.Sprintf("%.0f KB", k)
			}
		},
		"late": func(eta any, status string) bool {
			t, ok := toTime(eta)
			return ok && !model.IsFinalStatus(status) && time.Now().After(t)
		},
		"connState": func(v any) string {
			t, _ := toTime(v)
			return service.ConnectionState(t, time.Now())
		},

		"str": func(p *string) string {
			if p == nil {
				return ""
			}
			return *p
		},
		"uuid": func(u pgtype.UUID) string {
			if !u.Valid {
				return ""
			}
			return u.String()
		},
		"num": func(n pgtype.Numeric) float64 {
			f, err := n.Float64Value()
			if err != nil || !f.Valid {
				return 0
			}
			return f.Float64
		},
		"hasNum": func(n pgtype.Numeric) bool { return n.Valid },
		"int32": func(p *int32) string {
			if p == nil {
				return ""
			}
			return strconv.Itoa(int(*p))
		},
		"bool": func(p *bool) bool { return p != nil && *p },
		"i64": func(p *int64) int64 {
			if p == nil {
				return 0
			}
			return *p
		},
		// wholeBirr is an optional santim amount in birr, for a form field.
		"wholeBirr": func(p *int64) string {
			if p == nil {
				return ""
			}
			return strconv.FormatInt(*p/100, 10)
		},
		"i32": func(p *int32) int32 {
			if p == nil {
				return 0
			}
			return *p
		},
		// birr writes santim, hundredths of a birr, as "ETB 2,875.00".
		// birr and etb join the currency to the number with a no-break
		// space, so an amount never wraps between the two.
		"birr": func(santim int64) string { return "ETB\u00a0" + payments.FormatETB(santim) },
		// etb writes santim as whole birr, for figures read at a glance.
		"etb":       func(santim int64) string { return "ETB\u00a0" + grouped((santim+50)/100) },
		"cycleName": model.CycleName,
		// share is used as a percentage of limit, from 0 to 100.
		"share": func(used, limit any) int {
			l := toFloat(limit)
			if l <= 0 {
				return 0
			}
			return int(math.Min(100, math.Round(toFloat(used)*100/l)))
		},
		// past reports whether a moment has gone by.
		"past": func(v any) bool {
			t, ok := toTime(v)
			return ok && time.Now().After(t)
		},
		"invoiceKind":  invoiceKind,
		"invoiceLines": service.InvoiceLines,
		"money": func(v any) string {
			f := toFloat(v)
			return "ETB " + grouped(int64(math.Round(f)))
		},
		"grouped": func(v any) string { return grouped(int64(math.Round(toFloat(v)))) },
		"kg": func(v any) string {
			f := toFloat(v)
			if f >= 1000 {
				return strconv.FormatFloat(f/1000, 'f', 1, 64) + " t"
			}
			return strconv.FormatFloat(f, 'f', 0, 64) + " kg"
		},
		"pct": func(v float64) string {
			if v < 0 {
				return "—"
			}
			return strconv.Itoa(int(math.Round(v*100))) + "%"
		},
		"rate": func(a, b int64) float64 { return service.OnTimeRate(a, b) },
		"hours": func(h float64) string {
			switch {
			case h <= 0:
				return "—"
			case h < 48:
				return strconv.FormatFloat(h, 'f', 1, 64) + " h"
			default:
				return strconv.FormatFloat(h/24, 'f', 1, 64) + " days"
			}
		},
		"distance": func(v any) string {
			f := toFloat(v)
			if f < 1000 {
				return strconv.Itoa(int(f)) + " m"
			}
			return strconv.FormatFloat(f/1000, 'f', 1, 64) + " km"
		},
		"maskPhone": phone.Mask,
		"telURL": func(p string) template.URL {
			return template.URL("tel:" + strings.ReplaceAll(p, " ", ""))
		},
		"local": func(t time.Time) time.Time { return t.In(loc) },
		"firstName": func(name string) string {
			if f := strings.Fields(name); len(f) > 0 {
				return f[0]
			}
			return name
		},
		"initials": func(name string) string {
			var out []rune
			for _, f := range strings.Fields(name) {
				out = append(out, []rune(f)[0])
				if len(out) == 2 {
					break
				}
			}
			return strings.ToUpper(string(out))
		},

		"in": func(v string, list []string) bool {
			for _, x := range list {
				if x == v {
					return true
				}
			}
			return false
		},
		"field": func(form url.Values, name, def string) string {
			if form != nil {
				if v, ok := form[name]; ok && len(v) > 0 {
					return v[0]
				}
			}
			return def
		},
		"checked": func(form url.Values, name string, def bool) bool {
			if form != nil && len(form) > 0 {
				return form.Get(name) != ""
			}
			return def
		},
		"bytes": func(v any) string { return humanBytes(toFloat(v)) },
		"millis": func(d time.Duration) string {
			if d < 10*time.Millisecond {
				return fmt.Sprintf("%.1f ms", float64(d.Microseconds())/1000)
			}
			return fmt.Sprintf("%d ms", d.Milliseconds())
		},
		"span": span,
		"add":  func(a, b int) int { return a + b },
		"sub":  func(a, b int) int { return a - b },
		"mul":  func(a, b float64) float64 { return a * b },
		"div": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"float": toFloat,
		"seq": func(n int) []int {
			out := make([]int, n)
			for i := range out {
				out[i] = i
			}
			return out
		},
		"dict": func(kv ...any) map[string]any {
			m := make(map[string]any, len(kv)/2)
			for i := 0; i+1 < len(kv); i += 2 {
				m[fmt.Sprint(kv[i])] = kv[i+1]
			}
			return m
		},
		"list": func(v ...string) []string { return v },
		// withQuery returns the current query with one key replaced, for
		// pagination and sort links.
		"withQuery": func(q url.Values, key, value string) string {
			c := url.Values{}
			for k, v := range q {
				c[k] = v
			}
			if value == "" {
				c.Del(key)
			} else {
				c.Set(key, value)
			}
			if key != "page" {
				c.Del("page")
			}
			return "?" + c.Encode()
		},
		"safeHTML": func(s string) template.HTML { return template.HTML(s) }, //nolint:gosec // only for trusted config strings
		"lower":    strings.ToLower,
		"upper":    strings.ToUpper,
		"title": func(s string) string {
			if s == "" {
				return s
			}
			return strings.ToUpper(s[:1]) + s[1:]
		},
		"join": strings.Join,
		// bareURL drops the scheme, for addresses people read and type.
		"bareURL": func(u string) string {
			return strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
		},
		"truncate": func(n int, s string) string { return truncateText(s, n) },
		"mark":     mark,
	}
}

// device names a browser the way people know it, "Chrome on Android", from
// its user agent, or shortens the user agent when it is not recognised.
func device(ua *string) string {
	if ua == nil {
		return ""
	}
	s := *ua
	var browser, system string
	switch {
	case strings.Contains(s, "Edg/"), strings.Contains(s, "EdgA/"), strings.Contains(s, "EdgiOS/"):
		browser = "Edge"
	case strings.Contains(s, "OPR/"), strings.Contains(s, "Opera"):
		browser = "Opera"
	case strings.Contains(s, "SamsungBrowser/"):
		browser = "Samsung Internet"
	case strings.Contains(s, "Firefox/"), strings.Contains(s, "FxiOS/"):
		browser = "Firefox"
	case strings.Contains(s, "Chrome/"), strings.Contains(s, "CriOS/"):
		browser = "Chrome"
	case strings.Contains(s, "Safari/"):
		browser = "Safari"
	}
	switch {
	case strings.Contains(s, "Android"):
		system = "Android"
	case strings.Contains(s, "iPhone"):
		system = "iPhone"
	case strings.Contains(s, "iPad"):
		system = "iPad"
	case strings.Contains(s, "Windows"):
		system = "Windows"
	case strings.Contains(s, "CrOS"):
		system = "ChromeOS"
	case strings.Contains(s, "Macintosh"):
		system = "Mac"
	case strings.Contains(s, "Linux"):
		system = "Linux"
	}
	switch {
	case browser != "" && system != "":
		return browser + " on " + system
	case browser != "":
		return browser
	case system != "":
		return system
	}
	return truncateText(s, 60)
}

func relative(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d >= 24*time.Hour && d < 48*time.Hour:
		return "yesterday"
	}
	return span(d) + " ago"
}

// span is a rough length of time: "5 min", "3 h", "2 days".
func span(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%d min", max(1, int(d.Minutes())))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h", int(d.Hours()))
	case d < 48*time.Hour:
		return "1 day"
	case d < 60*24*time.Hour:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%d months", int(d.Hours()/24/30))
	}
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int32:
		return float64(n)
	case int64:
		return float64(n)
	case uint64:
		return float64(n)
	case *int32:
		if n == nil {
			return 0
		}
		return float64(*n)
	case *float64:
		if n == nil {
			return 0
		}
		return *n
	case pgtype.Numeric:
		f, err := n.Float64Value()
		if err != nil || !f.Valid {
			return 0
		}
		return f.Float64
	}
	return 0
}

// humanBytes is a size for people: "820 MB", "12.4 GB".
func humanBytes(n float64) string {
	units := []string{"bytes", "KB", "MB", "GB", "TB"}
	i := 0
	for n >= 1024 && i < len(units)-1 {
		n /= 1024
		i++
	}
	if i == 0 || n >= 100 {
		return fmt.Sprintf("%.0f %s", n, units[i])
	}
	return fmt.Sprintf("%.1f %s", n, units[i])
}

func grouped(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}
