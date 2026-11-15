package console

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestPrintFramesTheWholeBody(t *testing.T) {
	var out bytes.Buffer
	c := New(&out)
	link := "http://localhost:8090/reset/" + strings.Repeat("x", 90)
	c.Print(Item{Kind: "email", To: "almaz@example.et", Title: "Reset your password",
		Body: "Hello Almaz,\n\nOpen this link to choose a new password:\n" + link,
		At:   time.Date(2026, 10, 29, 14, 3, 12, 0, time.UTC)})

	got := out.String()
	for _, want := range []string{"┌─ EMAIL to almaz@example.et ", " 14:03:12\n", "│ Reset your password\n", "│ Hello Almaz,\n", "│\n", "│ " + link + "\n", "└───"} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
}

func TestWrapKeepsLongWordsWhole(t *testing.T) {
	long := strings.Repeat("a", width+10)
	lines := wrap("short words then " + long + " end")
	if len(lines) != 3 || lines[1] != long {
		t.Fatalf("wrap = %q", lines)
	}
	for _, l := range wrap(strings.Repeat("word ", 40)) {
		if len(l) > width {
			t.Errorf("line of %d runes exceeds %d", len(l), width)
		}
	}
}

func TestLastFindsTheNewestForARecipient(t *testing.T) {
	c := New(nil)
	c.Print(Item{Kind: "sms", To: "0911000001", Body: "first"})
	c.Print(Item{Kind: "sms", To: "0911000002", Body: "other"})
	c.Print(Item{Kind: "sms", To: "0911000001", Body: "second"})
	c.Print(Item{Kind: "email", To: "0911000001", Body: "not an sms"})

	if it, ok := c.Last("sms", "0911000001"); !ok || it.Body != "second" {
		t.Fatalf("Last = %+v, %v", it, ok)
	}
	if _, ok := c.Last("sms", "0911000009"); ok {
		t.Fatal("found a message for a number that received none")
	}
}

func TestKeepsOnlyRecentItems(t *testing.T) {
	c := New(nil)
	for range keep + 20 {
		c.Print(Item{Kind: "sms", To: "x"})
	}
	if n := len(c.Items()); n != keep {
		t.Fatalf("kept %d items, want %d", n, keep)
	}
}
