package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The signature is Standard Webhooks', checked against the example in its
// specification, so a receiver can use any of its libraries.
func TestSignWebhook(t *testing.T) {
	got := SignWebhook("whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw", "msg_p5jXN8AQM9LWM0D4loKWxJek",
		time.Unix(1614265330, 0), []byte(`{"test": 2432232314}`))
	if want := "v1,g0hM9SsE+OTPJTGt/tmIKtSyZlE3uFJELVlNIOLJ1OE="; got != want {
		t.Errorf("signature %s, want %s", got, want)
	}
	if s := newWebhookSecret(); len(s) != len("whsec_")+32 || s[:6] != "whsec_" {
		t.Errorf("secret %q", s)
	}
}

func TestPublicAddr(t *testing.T) {
	for addr, want := range map[string]bool{
		"196.188.0.1":           true, // Ethio telecom
		"8.8.8.8":               true,
		"2a00:1450:4001:80b::1": true,
		"127.0.0.1":             false,
		"10.0.0.5":              false,
		"172.17.0.1":            false, // Docker's bridge
		"192.168.1.10":          false,
		"169.254.169.254":       false, // cloud metadata
		"100.64.0.1":            false,
		"0.0.0.0":               false,
		"::1":                   false,
		"fd00::1":               false,
		"fe80::1":               false,
		"::ffff:127.0.0.1":      false,
		"64:ff9b::a00:1":        false,
		"255.255.255.255":       false,
		"224.0.0.1":             false,
	} {
		if got := publicAddr(netip.MustParseAddr(addr)); got != want {
			t.Errorf("%s: public %v, want %v", addr, got, want)
		}
	}
}

func TestCheckWebhookURL(t *testing.T) {
	strict := &WebhookService{core: &core{}}
	for raw, ok := range map[string]bool{
		"https://erp.example.com/hooks/kiflet": true,
		"https://erp.example.com:8443/x?y=1":   true,
		"http://erp.example.com/hooks":         false, // not encrypted
		"https://localhost/hooks":              false,
		"https://127.0.0.1/hooks":              false,
		"https://[::1]/hooks":                  false,
		"https://10.1.2.3/hooks":               false,
		"https://169.254.169.254/latest":       false,
		"https://intranet/hooks":               false,
		"https://user:pw@erp.example.com/":     false,
		"ftp://erp.example.com/":               false,
		"erp.example.com/hooks":                false,
		"":                                     false,
	} {
		if _, err := strict.checkWebhookURL(raw); (err == nil) != ok {
			t.Errorf("%q: accepted %v, want %v (%v)", raw, err == nil, ok, err)
		}
	}
	dev := &WebhookService{core: &core{cfg: Settings{PrivateWebhooks: true}}}
	if _, err := dev.checkWebhookURL("http://127.0.0.1:9000/hooks"); err != nil {
		t.Errorf("development should reach a receiver on the same machine: %v", err)
	}
	if u, _ := strict.checkWebhookURL("  https://erp.example.com/hooks#top "); u != "https://erp.example.com/hooks" {
		t.Errorf("address kept as %q", u)
	}
}

func TestCheckWebhookEvents(t *testing.T) {
	got, err := checkWebhookEvents([]string{WebhookDelivered, WebhookShipmentCreated, WebhookDelivered})
	if err != nil || !slices.Equal(got, []string{WebhookShipmentCreated, WebhookDelivered}) {
		t.Errorf("events %v, %v", got, err)
	}
	for _, bad := range [][]string{nil, {}, {"shipment.lost"}, {WebhookDelivered, "shipment.lost"}, {WebhookTest}} {
		if _, err := checkWebhookEvents(bad); err == nil {
			t.Errorf("%v should be refused", bad)
		}
	}
}

func TestWebhookTypes(t *testing.T) {
	for _, c := range []struct {
		e    shipmentEvent
		want []string
	}{
		{shipmentEvent{Type: "created", To: "pending"}, []string{WebhookShipmentCreated}},
		{shipmentEvent{Type: "status_changed", From: "assigned", To: "in_transit"}, []string{WebhookStatusChanged}},
		{shipmentEvent{Type: "status_changed", From: "arrived", To: "delivered"}, []string{WebhookStatusChanged, WebhookDelivered}},
		{shipmentEvent{Type: "status_changed", From: "in_transit", To: "issue"}, []string{WebhookStatusChanged, WebhookIssueReported}},
		{shipmentEvent{Type: "customer_issue"}, []string{WebhookIssueReported}},
		{shipmentEvent{Type: "status_updated", From: "delayed", To: "delayed"}, nil},
		{shipmentEvent{Type: "conflict", From: "delivered", To: "issue"}, nil},
		{shipmentEvent{Type: "note"}, nil},
		{shipmentEvent{Type: "assigned", To: "assigned"}, nil},
	} {
		if got := webhookTypes(c.e); !slices.Equal(got, c.want) {
			t.Errorf("%+v: %v, want %v", c.e, got, c.want)
		}
	}
}

// The address is checked again as the message goes out, so a name that
// leads to this machine is stopped there too.
func TestWebhookClientStaysOnTheInternet(t *testing.T) {
	var reached atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached.Add(1) }))
	defer srv.Close()
	viaName := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
	_, err := webhookClient(false).Post(viaName, "application/json", nil)
	if !errors.Is(err, errPrivateAddress) {
		t.Errorf("posting to %s: %v, want errPrivateAddress", viaName, err)
	}
	if msg := webhookError(err); !strings.Contains(msg, "private network") {
		t.Errorf("explained as %q", msg)
	}
	if reached.Load() != 0 {
		t.Fatal("the receiver on this machine was reached")
	}
	res, err := webhookClient(true).Post(srv.URL, "application/json", nil)
	if err != nil {
		t.Fatalf("with private targets allowed: %v", err)
	}
	res.Body.Close()
	if reached.Load() != 1 {
		t.Error("with private targets allowed, the receiver should be reached")
	}
}
