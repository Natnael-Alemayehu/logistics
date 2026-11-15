package sandbox

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Event is one call between Kiflet and the sandbox, as the log shows it.
type Event struct {
	ID      int       `json:"id"`
	At      time.Time `json:"at"`
	Service string    `json:"service"`
	// Dir is "in" for a call Kiflet made to a provider, "out" for one a
	// provider made to Kiflet, and "held" for one kept back.
	Dir    string `json:"dir"`
	Method string `json:"method"`
	URL    string `json:"url"`
	// Status is the answer's, or 0 when none came.
	Status  int    `json:"status"`
	TookMS  int64  `json:"took_ms"`
	Summary string `json:"summary"`
	Error   string `json:"error,omitempty"`
	// Request and Response are the headers and bodies, secrets masked.
	Request  string `json:"request"`
	Response string `json:"response"`
}

const keepEvents = 500

// record adds an event to the log and lists it on the terminal.
func (s *Sandbox) record(e Event) {
	if e.At.IsZero() {
		e.At = time.Now()
	}
	s.mu.Lock()
	s.eventSeq++
	e.ID = s.eventSeq
	s.events = append(s.events, e)
	if len(s.events) > keepEvents {
		s.events = s.events[len(s.events)-keepEvents:]
	}
	s.mu.Unlock()
	if s.out == nil {
		return
	}
	status := "---"
	if e.Status > 0 {
		status = strconv.Itoa(e.Status)
	}
	line := fmt.Sprintf("%s %-8s %-4s %s %s %s", e.At.Format("15:04:05"), e.Service, e.Dir, status, e.Method, e.URL)
	if e.Dir != "held" {
		line += fmt.Sprintf(" %dms", e.TookMS)
	}
	if e.Summary != "" {
		line += "  " + e.Summary
	}
	if e.Error != "" {
		line += " (" + e.Error + ")"
	}
	fmt.Fprintln(s.out, line)
}

// eventsAfter returns the logged events after an ID, oldest first, of one
// service or of all.
func (s *Sandbox) eventsAfter(after int, service string) []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Event
	for _, e := range s.events {
		if e.ID > after && (service == "" || e.Service == service) {
			out = append(out, e)
		}
	}
	return out
}

type noteKey struct{}

// note gives the log's summary of a call to a provider.
func note(r *http.Request, format string, args ...any) {
	if p, ok := r.Context().Value(noteKey{}).(*string); ok {
		*p = fmt.Sprintf(format, args...)
	}
}

// provider logs the calls made to a provider: what was asked and what was
// answered, with the summary the handler gives.
func (s *Sandbox) provider(service string, h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		r.Body = io.NopCloser(bytes.NewReader(body))
		var summary string
		rec := &recorder{ResponseWriter: w}
		h(rec, r.WithContext(context.WithValue(r.Context(), noteKey{}, &summary)))
		s.record(Event{
			At: start, Service: service, Dir: "in", Method: r.Method, URL: r.URL.Path, Status: rec.status,
			TookMS: time.Since(start).Milliseconds(), Summary: summary,
			Request: describe(r.Header, body, r.URL.RawQuery), Response: describe(rec.Header(), rec.body.Bytes(), ""),
		})
	})
}

// recorder passes an answer on and keeps the start of it for the log.
type recorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (r *recorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	if room := 64<<10 - r.body.Len(); room > 0 {
		r.body.Write(b[:min(len(b), room)])
	}
	return r.ResponseWriter.Write(b)
}

// ---------------------------------------------------------------------------
// Calls to Kiflet
// ---------------------------------------------------------------------------

// call is a request a provider makes to Kiflet: a payment notification, a
// delivery report, one of CBE Birr's calls, or a text from a phone.
type call struct {
	ID      int    `json:"id"`
	Service string `json:"service"`
	What    string `json:"what"`
	URL     string `json:"url"`
	// Held is when the call was kept back, and Why says why: a fault, or
	// Kiflet not taking it after every try.
	Held time.Time `json:"held,omitzero"`
	Why  string    `json:"why,omitempty"`

	header http.Header
	body   []byte
	tries  int
	// done learns the answer to each attempt, and whether it is the last.
	done func(a answer, last bool)
}

// answer is how Kiflet answered a call.
type answer struct {
	Status int
	Body   []byte
	Err    error
}

func (a answer) ok() bool { return a.Err == nil && a.Status/100 == 2 }

var errDiscarded = errors.New("discarded")

// describeAnswer says, for the pages, what Kiflet made of a call.
func describeAnswer(a answer) string {
	switch {
	case errors.Is(a.Err, errDiscarded):
		return "Discarded"
	case a.Err != nil:
		return "No answer: " + a.Err.Error()
	case a.ok():
		return "Taken by Kiflet"
	}
	return fmt.Sprintf("Refused: answered %d %s", a.Status, clip(string(a.Body), 120))
}

// newCall prepares a call to Kiflet, made up to tries times.
func (s *Sandbox) newCall(service, what, target string, header http.Header, body []byte, tries int, done func(answer, bool)) *call {
	s.mu.Lock()
	s.callSeq++
	id := s.callSeq
	s.mu.Unlock()
	return &call{ID: id, Service: service, What: what, URL: target, header: header, body: body, tries: max(tries, 1), done: done}
}

// again is the same call once more, as a provider sends a duplicate.
func (s *Sandbox) again(c *call) *call {
	return s.newCall(c.Service, c.What+" (again)", c.URL, c.header, c.body, c.tries, c.done)
}

func (c *call) report(a answer, last bool) {
	if c.done != nil {
		c.done(a, last)
	}
}

// attempt makes a call once and logs it.
func (s *Sandbox) attempt(ctx context.Context, c *call) answer {
	start := time.Now()
	target := s.reach(c.URL)
	var a answer
	var resHeader http.Header
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(c.body))
	if err == nil {
		req.Header = c.header.Clone()
		var res *http.Response
		if res, err = s.client.Do(req); err == nil {
			a.Status, resHeader = res.StatusCode, res.Header
			a.Body, _ = io.ReadAll(io.LimitReader(res.Body, 64<<10))
			res.Body.Close()
		}
	}
	var uerr *url.Error
	if errors.As(err, &uerr) {
		err = uerr.Err
	}
	a.Err = err
	e := Event{
		At: start, Service: c.Service, Dir: "out", Method: http.MethodPost, URL: target, Status: a.Status,
		TookMS: time.Since(start).Milliseconds(), Summary: c.What,
		Request: describe(c.header, c.body, ""), Response: describe(resHeader, a.Body, ""),
	}
	if err != nil {
		e.Error = err.Error()
	}
	s.record(e)
	return a
}

// deliver makes a call until Kiflet takes it, with any 2xx answer, waiting
// Timing.Retry before the second attempt and twice as long before each one
// after. The first attempt is made before deliver returns, so a payer sent
// back to Kiflet right after finds the payment settled; the others are
// made in the background. A call Kiflet never takes is kept with the held
// ones, to send again by hand.
func (s *Sandbox) deliver(c *call) {
	a := s.attempt(s.ctx, c)
	if a.ok() || c.tries == 1 {
		s.finish(c, a)
		return
	}
	c.report(a, false)
	s.background(func() {
		wait := s.cfg.Timing.Retry
		for try := 2; try <= c.tries; try++ {
			if !s.sleep(wait) {
				return
			}
			a = s.attempt(s.ctx, c)
			if a.ok() || try == c.tries {
				break
			}
			c.report(a, false)
			wait = min(wait*2, 5*time.Minute)
		}
		s.finish(c, a)
	})
}

// finish reports a call's last answer, keeping it to send by hand when it
// was never taken.
func (s *Sandbox) finish(c *call, a answer) {
	c.report(a, true)
	if !a.ok() && c.tries > 1 && s.ctx.Err() == nil {
		s.hold(c, fmt.Sprintf("Kiflet didn't take it in %d tries", c.tries))
	}
}

// hold keeps a call back, until someone sends it or discards it.
func (s *Sandbox) hold(c *call, why string) {
	s.mu.Lock()
	c.Held, c.Why = time.Now(), why
	s.held = append(s.held, c)
	s.mu.Unlock()
	s.record(Event{
		Service: c.Service, Dir: "held", Method: http.MethodPost, URL: s.reach(c.URL),
		Summary: c.What + ": held, " + why, Request: describe(c.header, c.body, ""),
	})
}

// release sends a held call now, or discards it. It reports whether the
// call was held.
func (s *Sandbox) release(id int, send bool) bool {
	s.mu.Lock()
	i := slices.IndexFunc(s.held, func(c *call) bool { return c.ID == id })
	var c *call
	if i >= 0 {
		c = s.held[i]
		s.held = slices.Delete(s.held, i, i+1)
	}
	s.mu.Unlock()
	switch {
	case c == nil:
		return false
	case send:
		c.Held, c.Why = time.Time{}, ""
		s.deliver(c)
	default:
		c.report(answer{Err: errDiscarded}, true)
	}
	return true
}

// heldCalls is a copy of the calls held back, oldest first.
func (s *Sandbox) heldCalls() []call {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]call, len(s.held))
	for i, c := range s.held {
		out[i] = *c
	}
	return out
}

// later runs f after d, in the background, unless the sandbox closes first.
func (s *Sandbox) later(d time.Duration, f func()) {
	s.background(func() {
		if s.sleep(d) {
			f()
		}
	})
}

// sleep waits d, reporting false when the sandbox closed first.
func (s *Sandbox) sleep(d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-s.ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// background runs f in a goroutine Close waits for.
func (s *Sandbox) background(f func()) {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	if s.closed {
		return
	}
	s.jobs.Add(1)
	go func() {
		defer s.jobs.Done()
		f()
	}()
}

// reach is the address to call for one given: localhost becomes HostAlias,
// when there is one.
func (s *Sandbox) reach(target string) string {
	if s.cfg.HostAlias == "" {
		return target
	}
	u, err := url.Parse(target)
	if err != nil {
		return target
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		host := s.cfg.HostAlias
		if port := u.Port(); port != "" {
			host = net.JoinHostPort(host, port)
		}
		u.Host = host
		return u.String()
	}
	return target
}

// jsonHeader is the header of a JSON call.
func jsonHeader() http.Header { return http.Header{"Content-Type": {"application/json"}} }

// ---------------------------------------------------------------------------
// What the log shows
// ---------------------------------------------------------------------------

// describe writes a request's or an answer's headers, query and body for
// the log, secrets masked.
func describe(h http.Header, body []byte, query string) string {
	var b strings.Builder
	for _, k := range []string{"Content-Type", "Authorization", "X-App-Key", "X-Webhook-Secret", "Location"} {
		if v := h.Get(k); v != "" {
			fmt.Fprintf(&b, "%s: %s\n", k, maskHeader(k, v))
		}
	}
	if q, err := url.ParseQuery(query); err == nil && len(q) > 0 {
		b.WriteByte('\n')
		keys := make([]string, 0, len(q))
		for k := range q {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "%s=%s\n", k, maskField(k, q.Get(k)))
		}
	}
	if len(body) > 0 {
		b.WriteByte('\n')
		b.WriteString(maskBody(h.Get("Content-Type"), body))
	}
	return strings.TrimRight(b.String(), "\n")
}

func maskHeader(name, v string) string {
	switch name {
	case "X-Webhook-Secret":
		return "•••"
	case "Authorization":
		scheme, cred, ok := strings.Cut(v, " ")
		if !ok {
			return clipSecret(v)
		}
		if strings.EqualFold(scheme, "Basic") {
			if raw, err := base64.StdEncoding.DecodeString(cred); err == nil {
				user, _, _ := strings.Cut(string(raw), ":")
				return "Basic " + user + ":•••"
			}
		}
		return scheme + " " + clipSecret(cred)
	}
	return v
}

// maskField hides what the log never shows, and shortens what is only long.
func maskField(name, v string) string {
	switch strings.ToLower(name) {
	case "appsecret", "api_key", "password", "secret", "pin":
		return "•••"
	case "token":
		return clipSecret(v)
	case "sign":
		return clip(v, 24)
	}
	return v
}

// clipSecret keeps enough of a token to tell it from another.
func clipSecret(v string) string {
	if len(v) <= 12 {
		return "•••"
	}
	return v[:6] + "…"
}

func maskBody(contentType string, body []byte) string {
	if strings.HasPrefix(contentType, "text/html") {
		return "(a page)"
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if dec.Decode(&v) == nil {
		out, err := json.MarshalIndent(maskJSON(v), "", "  ")
		if err == nil {
			return string(out)
		}
	}
	return clip(string(body), 4000)
}

func maskJSON(v any) any {
	switch v := v.(type) {
	case map[string]any:
		for k, x := range v {
			if str, ok := x.(string); ok {
				v[k] = maskField(k, str)
			} else {
				v[k] = maskJSON(x)
			}
		}
	case []any:
		for i := range v {
			v[i] = maskJSON(v[i])
		}
	}
	return v
}
