package service

import (
	"strings"
	"testing"

	"github.com/natnael-alemayehu/kiflet/internal/db"
	"github.com/natnael-alemayehu/kiflet/internal/model"
)

func TestParseSMSCommand(t *testing.T) {
	cases := []struct {
		in   string
		want SMSCommand
	}{
		{"ET 7K3M 1", SMSCommand{Ref: "7K3M", Status: model.StatusInTransit}},
		{"et 7k3m 2 flat tyre near Awash", SMSCommand{Ref: "7K3M", Status: model.StatusDelayed, Note: "flat tyre near Awash"}},
		{"  ET   9QX2  4 ", SMSCommand{Ref: "9QX2", Status: model.StatusDelivered}},
		{"ET-20261028-7K3M9QX2PA 3", SMSCommand{Ref: "ET202610287K3M9QX2PA", Status: model.StatusArrived}},
		{"5 accident", SMSCommand{Status: model.StatusIssue, Note: "accident"}},
		{"1", SMSCommand{Status: model.StatusInTransit}},
		{"STOP", SMSCommand{Keyword: "STOP"}},
		{"stop", SMSCommand{Keyword: "STOP"}},
		{"help", SMSCommand{Keyword: "HELP"}},
		{"START", SMSCommand{Keyword: "START"}},
	}
	for _, c := range cases {
		got, err := ParseSMSCommand(c.in)
		if err != nil {
			t.Errorf("ParseSMSCommand(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseSMSCommand(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
	for _, bad := range []string{"", "hello", "ET 7K3M", "ET 7K3M 9", "6 something"} {
		if got, err := ParseSMSCommand(bad); err == nil {
			t.Errorf("ParseSMSCommand(%q) = %+v, want error", bad, got)
		}
	}
}

func TestMatchShipment(t *testing.T) {
	a := db.Shipment{TrackingNumber: "ET-20261028-AAAA11K3M7"}
	b := db.Shipment{TrackingNumber: "ET-20261028-BBBB22Q9X2"}

	if got, _ := matchShipment([]db.Shipment{a}, ""); got == nil || got.TrackingNumber != a.TrackingNumber {
		t.Error("a sole active shipment should be matched without a reference")
	}
	if got, msg := matchShipment([]db.Shipment{a, b}, ""); got != nil || msg == "" {
		t.Error("several shipments without a reference should ask for one")
	}
	if got, _ := matchShipment([]db.Shipment{a, b}, "Q9X2"); got == nil || got.TrackingNumber != b.TrackingNumber {
		t.Error("suffix should select the matching shipment")
	}
	if got, msg := matchShipment([]db.Shipment{a, b}, "ZZZZ"); got != nil || msg == "" {
		t.Error("unknown suffix should be refused")
	}
	if got, msg := matchShipment(nil, "Q9X2"); got != nil || msg == "" {
		t.Error("no active shipments should be explained")
	}
	c := db.Shipment{TrackingNumber: "ET-20261029-CCCC33Q9X2"}
	if got, msg := matchShipment([]db.Shipment{b, c}, "Q9X2"); got != nil || msg == "" {
		t.Error("an ambiguous suffix should ask for more characters")
	}
}

func TestShortRef(t *testing.T) {
	if got := ShortRef("ET-20261028-7K3M9QX2PA"); got != "X2PA" {
		t.Errorf("ShortRef = %q", got)
	}
}

// TestSMSParts: texts count as the SMS parts the provider charges for (W8).
func TestSMSParts(t *testing.T) {
	cases := []struct {
		body string
		want int32
	}{
		{"", 1},
		{strings.Repeat("a", 160), 1},
		{strings.Repeat("a", 161), 2},
		{strings.Repeat("a", 306), 2},
		{strings.Repeat("a", 307), 3},
		{strings.Repeat("ሀ", 70), 1},
		{strings.Repeat("ሀ", 71), 2},
		{"Kiflet: ጭነትዎ ተልኳል " + strings.Repeat("x", 60), 2},
		{"line one\nline two", 1},
	}
	for _, c := range cases {
		if got := smsParts(c.body); got != c.want {
			t.Errorf("smsParts(%d runes starting %q) = %d, want %d", len([]rune(c.body)), firstRunes(c.body, 8), got, c.want)
		}
	}
}

func firstRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}
