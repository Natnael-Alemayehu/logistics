package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/db"
	"github.com/natnael-alemayehu/logistics/internal/model"
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

func TestCheckPIN(t *testing.T) {
	for _, ok := range []string{"2580", "13579", "904172"} {
		if err := checkPIN(ok); err != nil {
			t.Errorf("checkPIN(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "123", "1234567", "1111", "1234", "4321", "123456", "12a4"} {
		if err := checkPIN(bad); err == nil {
			t.Errorf("checkPIN(%q) accepted", bad)
		}
	}
}

func TestTemporaryCredentials(t *testing.T) {
	for i := 0; i < 50; i++ {
		pw := temporaryPassword()
		if len(pw) != 10 {
			t.Fatalf("temporary password %q has length %d", pw, len(pw))
		}
		if checkPassword(pw) != nil {
			t.Fatalf("temporary password %q fails policy", pw)
		}
	}
	if d := randomDigits(6); len(d) != 6 {
		t.Errorf("randomDigits = %q", d)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Abay Logistics PLC":   "abay-logistics-plc",
		"  Selam  & Sons  ":    "selam-sons",
		"ሰላም ትራንስፖርት":          "company",
		"Tana Freight (Bahir)": "tana-freight-bahir",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTrackingNumberFormat(t *testing.T) {
	day := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	n := newTrackingNumber(day)
	if len(n) != len("ET-20261028-")+10 || n[:12] != "ET-20261028-" {
		t.Fatalf("tracking number %q", n)
	}
	for _, c := range n[12:] {
		found := false
		for _, a := range trackingAlphabet {
			found = found || a == c
		}
		if !found {
			t.Fatalf("tracking number %q uses %q outside the alphabet", n, c)
		}
	}
	if newTrackingNumber(day) == n {
		t.Error("two tracking numbers should differ")
	}
}

func TestSyncResultClassification(t *testing.T) {
	base := SyncResult{ClientID: "c1", Kind: "status", OK: true}
	if r := resultFor(base, nil); !r.OK || r.Retry {
		t.Errorf("success: %+v", r)
	}
	if r := resultFor(base, ErrStaleUpdate); r.OK || r.Retry || r.Code != "STALE" {
		t.Errorf("stale: %+v", r)
	}
	if r := resultFor(base, conflict("no")); r.OK || r.Retry || r.Code != "CONFLICT" {
		t.Errorf("domain error should be final: %+v", r)
	}
	if r := resultFor(base, errors.New("connection reset")); r.OK || !r.Retry {
		t.Errorf("internal error should be retried: %+v", r)
	}
}

func TestTrackingSteps(t *testing.T) {
	steps := trackingSteps(model.StatusArrived)
	if !steps[3].Current || !steps[2].Done || steps[4].Done {
		t.Errorf("arrived steps = %+v", steps)
	}
	for _, s := range trackingSteps(model.StatusCancelled) {
		if s.Done {
			t.Error("a cancelled shipment has no progress")
		}
	}
}

func TestCoarsePositionHidesExactLocation(t *testing.T) {
	p := coarsePosition(8.5432, 39.2701, time.Now())
	if p.Near != "near Adama" {
		t.Errorf("near = %q, want %q", p.Near, "near Adama")
	}
	if p.Lat == 8.5432 || p.Lng == 39.2701 {
		t.Error("public position should be rounded")
	}
}

func TestResolvePlace(t *testing.T) {
	p, err := resolvePlace("origin", "nazret", "", nil, nil)
	if err != nil || p.City == nil || *p.City != "Adama" || p.Precise || p.Address != "Adama" {
		t.Fatalf("town only: %+v, %v", p, err)
	}
	lat, lng := 9.0105, 38.7613
	p, err = resolvePlace("destination", "", "Merkato, block 12", &lat, &lng)
	if err != nil || !p.Precise || p.City == nil || *p.City != "Addis Ababa" {
		t.Fatalf("pin: %+v, %v", p, err)
	}
	if _, err := resolvePlace("origin", "Atlantis", "", nil, nil); err == nil {
		t.Error("unknown town without a pin should be refused")
	}
	far, farLng := 40.0, -3.0
	if _, err := resolvePlace("origin", "", "", &far, &farLng); err == nil {
		t.Error("pin outside the service area should be refused")
	}
}

func TestHumanDuration(t *testing.T) {
	cases := map[time.Duration]string{
		30 * time.Minute: "30 minutes",
		time.Hour:        "an hour",
		5 * time.Hour:    "5 hours",
		72 * time.Hour:   "3 days",
	}
	for d, want := range cases {
		if got := humanDuration(d); got != want {
			t.Errorf("humanDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestPODLocationReasonIsKnownOrOther(t *testing.T) {
	for in, want := range map[string]string{"met_elsewhere": "met_elsewhere", " gps_inaccurate ": "gps_inaccurate", "teleported": "other", "": ""} {
		p := PODInput{RecipientName: "Almaz", LocationReason: in}
		if err := p.Check(); err != nil {
			t.Fatal(err)
		}
		if p.LocationReason != want {
			t.Errorf("reason %q became %q, want %q", in, p.LocationReason, want)
		}
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
		{"Menged: ጭነትዎ ተልኳል " + strings.Repeat("x", 60), 2},
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
