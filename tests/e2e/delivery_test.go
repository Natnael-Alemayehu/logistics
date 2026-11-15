//go:build e2e

package e2e

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/jpeg"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestDelivery: a load from booking to its proof, through every interface
// it touches. The office books it on the dashboard, the driver is texted
// it and texts back from the road, the driver app delivers it with a
// signature and a photo, which the app keeps in S3, the dashboard is told
// as it happens, and the customer's tracking page says it's delivered.
func TestDelivery(t *testing.T) {
	office := signUp(t, "Highland Freight")

	// Signing in: the cookies go over HTTPS only, out of scripts' reach. The
	// browser claims other addresses, as anyone can, and the app records the
	// one Caddy saw, so no one can take a new address for each try.
	again := newBrowser(t)
	claimed := []string{"203.0.113.7", "198.51.100.7"}
	again.header = http.Header{"X-Real-IP": {claimed[0]}, "X-Forwarded-For": {claimed[1]}}
	p := again.mustPost("/login", url.Values{"identifier": {office.email}, "password": {password}})
	if p.Location != "/app" {
		t.Fatalf("signing in went to %q", p.Location)
	}
	cookies := p.Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		t.Fatal("signing in set no cookie")
	}
	for _, c := range cookies {
		for _, flag := range []string{"Secure", "HttpOnly", "SameSite=Lax"} {
			if !strings.Contains(c, flag) {
				t.Errorf("cookie %s without %s", strings.SplitN(c, "=", 2)[0], flag)
			}
		}
	}
	if from := lastSignIn(t, office); net.ParseIP(from) == nil || slices.Contains(claimed, from) {
		t.Errorf("the sign-in is recorded from %q, want the address Caddy saw", from)
	}

	// A driver, and a load for them.
	driverPhone := phoneNumber()
	p = office.post("/app/drivers", url.Values{"full_name": {"Almaz Bekele"}, "phone": {driverPhone}})
	pin := secretRe.FindStringSubmatch(p.Body)
	if p.Status != http.StatusOK || pin == nil {
		t.Fatalf("adding the driver: status %d %s", p.Status, errorText(p.Body))
	}
	driverID := optionFor(t, office.get("/app/shipments/new").Body, "Almaz Bekele")
	p = office.mustPost("/app/shipments", url.Values{
		"priority": {"normal"}, "origin_city": {"Addis Ababa"}, "origin_address": {"Main depot"},
		"destination_city": {"Adama"}, "destination_address": {"Customer warehouse"},
		"customer_name": {"Abyssinia Traders"}, "customer_phone": {phoneNumber()},
		"cargo_description": {"Cement, 200 bags"}, "cargo_weight": {"10000"}, "driver_id": {driverID},
	})
	shipmentID := strings.TrimPrefix(p.Location, "/app/shipments/")
	tracking := trackingRe.FindString(office.get(p.Location).Body)
	if tracking == "" {
		t.Fatal("the shipment page shows no tracking number")
	}

	live := listen(t, office)

	// The driver is texted the load, and texts back from the road.
	assigned := waitForText(t, driverPhone, "new shipment")
	ref := reportRe.FindStringSubmatch(assigned)
	if ref == nil {
		t.Fatalf("the assignment text says nothing about texting back: %q", assigned)
	}
	if code := text(t, driverPhone, "ET "+ref[1]+" 1"); code != http.StatusOK {
		t.Fatalf("the driver's text: the app answered the gateway %d", code)
	}
	waitForText(t, driverPhone, "recorded")
	live.waitFor(t, "shipment", shipmentID, "in_transit")

	// The driver app: signing in on the phone, choosing a PIN, arriving,
	// and the proof of delivery.
	d := driverSignIn(t, driverPhone, pin[1])
	d.sync(t, map[string]any{"statuses": []map[string]any{{
		"client_id": clientID(), "shipment_id": shipmentID, "status": "arrived", "recorded_at": time.Now(),
	}}})
	live.waitFor(t, "shipment", shipmentID, "arrived")
	lat, lng := 8.5400, 39.2700 // Adama, where the app places it
	d.sync(t, map[string]any{"pods": []map[string]any{{
		"client_id": clientID(), "shipment_id": shipmentID, "recipient_name": "Meseret Tadesse",
		"signature": signaturePNG, "photos": []string{photoJPEG(t)}, "lat": lat, "lng": lng, "recorded_at": time.Now(),
	}}})
	live.waitFor(t, "shipment", shipmentID, "delivered")

	// The office sees the proof. The signature and the photo come from the
	// bucket through the app.
	p = office.get("/app/shipments/" + shipmentID)
	if !strings.Contains(p.Body, "Meseret Tadesse") {
		t.Error("the shipment page doesn't show who received the load")
	}
	if p := office.get("/app/shipments/" + shipmentID + "/media/0"); p.Status != http.StatusOK || !strings.HasPrefix(p.Body, "\x89PNG") {
		t.Errorf("the signature: status %d", p.Status)
	}
	if p := office.get("/app/shipments/" + shipmentID + "/media/1"); p.Status != http.StatusOK || !strings.HasPrefix(p.Body, "\xff\xd8") {
		t.Errorf("the photo: status %d", p.Status)
	}
	if p := newBrowser(t).get("/app/shipments/" + shipmentID + "/media/1"); p.Status != http.StatusSeeOther {
		t.Errorf("the photo without signing in: status %d, want a redirect to sign in", p.Status)
	}

	// Anyone with the number sees the outcome.
	if p := newBrowser(t).get("/track/" + tracking); p.Status != http.StatusOK || !strings.Contains(p.Body, "Delivered") {
		t.Errorf("the tracking page: status %d, not delivered", p.Status)
	}
}

// TestDriverApp: the driver app is an installable app with a service worker,
// so it works offline: its shell, manifest and worker are served as such.
func TestDriverApp(t *testing.T) {
	b := newBrowser(t)
	p := b.get("/driver/")
	if p.Status != http.StatusOK || !strings.Contains(p.Body, `rel="manifest"`) {
		t.Fatalf("the driver app: status %d, without a manifest", p.Status)
	}
	if p := b.get("/driver/sw.js"); p.Status != http.StatusOK || !strings.Contains(p.Header.Get("Content-Type"), "javascript") {
		t.Errorf("the service worker: status %d, type %q", p.Status, p.Header.Get("Content-Type"))
	}
	var config map[string]any
	if p := b.get("/api/v1/driver/config"); p.Status != http.StatusOK || json.Unmarshal([]byte(p.Body), &config) != nil {
		t.Errorf("the driver app's configuration: status %d", p.Status)
	}
	// The map library is in the image. An ignore rule once kept it out of
	// git, and every image built from a clone served a 404 for it.
	for path, kind := range map[string]string{
		"/static/vendor/leaflet/leaflet.js":             "javascript",
		"/static/vendor/leaflet/leaflet.css":            "css",
		"/static/vendor/leaflet/images/marker-icon.png": "png",
	} {
		if p := b.get(path); p.Status != http.StatusOK || !strings.Contains(p.Header.Get("Content-Type"), kind) {
			t.Errorf("%s: status %d, type %q", path, p.Status, p.Header.Get("Content-Type"))
		}
	}
}

var (
	secretRe   = regexp.MustCompile(`data-secret>([^<]+)<`)
	trackingRe = regexp.MustCompile(`ET-\d{8}-[0-9A-Z]{10}`)
	reportRe   = regexp.MustCompile(`text ET (\w+) 1 to \d+`)
	auditIPRe  = regexp.MustCompile(`<td class="small muted mono">([^<]*)</td>`)
)

// lastSignIn is the address the company's audit log, newest first, records
// its latest sign-in from.
func lastSignIn(t *testing.T, admin *browser) string {
	t.Helper()
	for _, row := range strings.Split(admin.get("/app/audit").Body, "<tr>") {
		if strings.Contains(row, "Signed in") {
			if m := auditIPRe.FindStringSubmatch(row); m != nil {
				return m[1]
			}
			return ""
		}
	}
	t.Fatal("the audit log shows no sign-in")
	return ""
}

// optionFor finds a driver's ID among the choices on the booking form.
func optionFor(t *testing.T, form, name string) string {
	t.Helper()
	m := regexp.MustCompile(`<option value="([0-9a-f-]{36})"[^>]*>` + regexp.QuoteMeta(name) + ` —`).FindStringSubmatch(form)
	if m == nil {
		t.Fatalf("%s isn't on the booking form", name)
	}
	return m[1]
}

// ---------------------------------------------------------------------------
// The dashboard's live updates
// ---------------------------------------------------------------------------

type liveUpdates struct {
	conn     *websocket.Conn
	messages chan liveMessage
}

type liveMessage struct {
	Type string `json:"type"`
	Data struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"data"`
}

// listen opens the dashboard's WebSocket, as app.js does, over wss.
func listen(t *testing.T, b *browser) *liveUpdates {
	t.Helper()
	// The HTTP client's TLS settings offer HTTP/2, which WebSockets don't
	// speak.
	tlsConfig := transport.TLSClientConfig.Clone()
	tlsConfig.NextProtos = nil
	dialer := websocket.Dialer{NetDialContext: dial, TLSClientConfig: tlsConfig, Jar: b.c.Jar, HandshakeTimeout: 10 * time.Second}
	u, _ := url.Parse(appURL)
	conn, res, err := dialer.Dial("wss://"+u.Host+"/app/live", http.Header{"Origin": {appURL}})
	if err != nil {
		status := 0
		if res != nil {
			status = res.StatusCode
		}
		t.Fatalf("live updates: %v (status %d)", err, status)
	}
	l := &liveUpdates{conn: conn, messages: make(chan liveMessage, 64)}
	t.Cleanup(func() { conn.Close() })
	go func() {
		defer close(l.messages)
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var m liveMessage
			if json.Unmarshal(raw, &m) == nil {
				l.messages <- m
			}
		}
	}()
	return l
}

// waitFor waits for a live update about a record, in a status.
func (l *liveUpdates) waitFor(t *testing.T, kind, id, status string) {
	t.Helper()
	timeout := time.After(20 * time.Second)
	for {
		select {
		case m, ok := <-l.messages:
			if !ok {
				t.Fatalf("the live updates closed before a %s update for %s", kind, id)
			}
			if m.Type == kind && m.Data.ID == id && m.Data.Status == status {
				return
			}
		case <-timeout:
			t.Fatalf("no live %s update for %s, %s", kind, id, status)
		}
	}
}

// ---------------------------------------------------------------------------
// The driver app's API
// ---------------------------------------------------------------------------

type driverApp struct {
	token, device string
}

func driverAPI(t *testing.T, path, token string, body, out any) int {
	t.Helper()
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, appURL+path, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res := send(t, newClient(nil), req)
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("POST %s: decode %q: %v", path, raw, err)
		}
	}
	return res.StatusCode
}

// driverSignIn signs a driver in on a new phone, with the PIN the office
// gave them, and sets their own.
func driverSignIn(t *testing.T, phone, firstPIN string) *driverApp {
	t.Helper()
	d := &driverApp{device: clientID()}
	var login struct {
		Token         string `json:"token"`
		MustChangePIN bool   `json:"must_change_pin"`
	}
	if code := driverAPI(t, "/api/v1/driver/login", "", map[string]string{"phone": phone, "pin": firstPIN, "device_id": d.device}, &login); code != http.StatusOK || login.Token == "" {
		t.Fatalf("the driver signing in: status %d", code)
	}
	if !login.MustChangePIN {
		t.Error("a new driver isn't asked to choose their own PIN")
	}
	d.token = login.Token
	if code := driverAPI(t, "/api/v1/driver/pin", d.token, map[string]string{"current_pin": firstPIN, "new_pin": "4826"}, nil); code != http.StatusOK {
		t.Fatalf("choosing a PIN: status %d", code)
	}
	return d
}

// sync sends what the phone holds and fails unless the app took all of it.
func (d *driverApp) sync(t *testing.T, req map[string]any) {
	t.Helper()
	req["device_id"] = d.device
	var res struct {
		Results []struct {
			OK      bool   `json:"ok"`
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"results"`
	}
	if code := driverAPI(t, "/api/v1/driver/sync", d.token, req, &res); code != http.StatusOK || len(res.Results) == 0 {
		t.Fatalf("sync: status %d, results %+v", code, res.Results)
	}
	for _, r := range res.Results {
		if !r.OK {
			t.Fatalf("sync: refused, %s: %s", r.Code, r.Message)
		}
	}
}

// signaturePNG is a valid 1×1 PNG as a data URL.
const signaturePNG = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

func photoJPEG(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 64, 48)), nil); err != nil {
		t.Fatal(err)
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}
