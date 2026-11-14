// Package console shows, on the server's terminal, what the stand-in
// providers would have delivered: text messages, emails and payment
// requests. In development it is the inbox for every code and link the app
// sends. It also keeps the most recent items, for tests to read.
package console

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Item is one delivery.
type Item struct {
	// Kind is what was delivered: "sms", "email" or "payment".
	Kind string
	// To is the recipient: a phone number or an email address.
	To string
	// Title is an email's subject or a payment's summary; empty for SMS.
	Title string
	Body  string
	At    time.Time
}

// Console prints deliveries and remembers the latest ones.
type Console struct {
	mu    sync.Mutex
	out   io.Writer
	items []Item
}

const (
	keep  = 100
	width = 76
)

// New returns a console printing to out; nil prints nothing and only keeps
// the items.
func New(out io.Writer) *Console { return &Console{out: out} }

// Print records an item and writes it as a framed block:
//
//	┌─ SMS to 0912345601 ──────────────────────────── 14:03:12
//	│ Your code is 482913. It expires in 10 minutes.
//	└──────────────────────────────────────────────────────────
func (c *Console) Print(it Item) {
	if it.At.IsZero() {
		it.At = time.Now()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = append(c.items, it)
	if len(c.items) > keep {
		c.items = c.items[len(c.items)-keep:]
	}
	if c.out != nil {
		_, _ = io.WriteString(c.out, frame(it))
	}
}

// Last returns the most recent item of a kind sent to a recipient.
func (c *Console) Last(kind, to string) (Item, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, it := range slices.Backward(c.items) {
		if it.Kind == kind && it.To == to {
			return it, true
		}
	}
	return Item{}, false
}

// Items returns the remembered items, oldest first.
func (c *Console) Items() []Item {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Item(nil), c.items...)
}

func frame(it Item) string {
	var b strings.Builder
	head := fmt.Sprintf("┌─ %s to %s ", strings.ToUpper(it.Kind), it.To)
	clock := " " + it.At.Format("15:04:05")
	fill := width + 2 - utf8.RuneCountInString(head) - utf8.RuneCountInString(clock)
	fmt.Fprintf(&b, "\n%s%s%s\n", head, strings.Repeat("─", max(fill, 3)), clock)
	var lines []string
	if it.Title != "" {
		lines = append(lines, wrap(it.Title)...)
		lines = append(lines, "")
	}
	for para := range strings.SplitSeq(strings.TrimRight(it.Body, "\n"), "\n") {
		lines = append(lines, wrap(para)...)
	}
	for _, l := range lines {
		fmt.Fprintln(&b, strings.TrimRight("│ "+l, " "))
	}
	fmt.Fprintf(&b, "└%s\n\n", strings.Repeat("─", width+1))
	return b.String()
}

// wrap breaks a paragraph at spaces to fit the frame. A word longer than a
// line, such as a link, stays whole so it can be copied.
func wrap(s string) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	line := words[0]
	for _, w := range words[1:] {
		if utf8.RuneCountInString(line)+1+utf8.RuneCountInString(w) > width {
			lines = append(lines, line)
			line = w
			continue
		}
		line += " " + w
	}
	return append(lines, line)
}
