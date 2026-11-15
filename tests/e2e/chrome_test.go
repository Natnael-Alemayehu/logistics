//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// chrome is a headless Chrome showing one page, told what to do over the
// DevTools protocol: the driver app is a program of its own that only a
// browser can run.
type chrome struct {
	t       *testing.T
	port    string            // where Chrome answers
	conn    *websocket.Conn   // the page
	workers []*websocket.Conn // its service workers, while they are held offline
	next    int
}

// findChrome is the Chrome or Chromium on this machine: E2E_CHROME, or the
// first of the usual names.
func findChrome() string {
	if bin := os.Getenv("E2E_CHROME"); bin != "" {
		return bin
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

// startChrome opens a browser the size of a phone, with nothing kept from
// earlier runs. Without a Chrome the test is skipped, except in CI, where
// its absence would otherwise pass for success.
func startChrome(t *testing.T) *chrome {
	t.Helper()
	bin := findChrome()
	if bin == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("no Chrome to run the driver app in")
		}
		t.Skip("no Chrome to run the driver app in: install one, or name it in E2E_CHROME")
	}
	// Not t.TempDir: Chrome's helper processes can still be writing to the
	// profile when the test's own cleanup would remove it.
	profile, err := os.MkdirTemp("", "kiflet-chrome-")
	if err != nil {
		t.Fatal(err)
	}
	args := []string{
		"--headless=new", "--remote-debugging-port=0", "--user-data-dir=" + profile,
		"--no-first-run", "--no-default-browser-check", "--disable-gpu", "--window-size=390,844",
		// Staging's certificates come from Caddy's own authority.
		"--ignore-certificate-errors",
	}
	if os.Getenv("CI") != "" {
		// The runner doesn't let Chrome set up its sandbox.
		args = append(args, "--no-sandbox")
	}
	cmd := exec.Command(bin, append(args, "about:blank")...)
	// Its own process group, so that its helpers can be stopped with it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting %s: %v", bin, err)
	}
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait() // how Chrome ends doesn't matter, only that it has
		close(exited)
	}()
	var port string
	t.Cleanup(func() {
		// Asked to close, Chrome stops its helpers and lets go of the
		// profile. One that doesn't is stopped, helpers and all.
		closeChrome(port)
		select {
		case <-exited:
		case <-time.After(10 * time.Second):
		}
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) // gone already, if it closed when asked
		<-exited
		for range 50 {
			if os.RemoveAll(profile) == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Logf("Chrome's profile %s is left behind", profile)
	})

	// Chrome writes the port it chose into its profile.
	within(t, 30*time.Second, "Chrome to start", func() bool {
		raw, err := os.ReadFile(filepath.Join(profile, "DevToolsActivePort"))
		if err != nil {
			return false
		}
		port = strings.SplitN(string(raw), "\n", 2)[0]
		return port != ""
	})
	var address string
	within(t, 10*time.Second, "Chrome to open a page", func() bool {
		for _, target := range chromeTargets(port) {
			if target.Type == "page" {
				address = target.Address
			}
		}
		return address != ""
	})
	conn, _, err := websocket.DefaultDialer.Dial(address, nil)
	if err != nil {
		t.Fatalf("connecting to Chrome: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	c := &chrome{t: t, port: port, conn: conn}
	c.call("Page.enable", nil)
	c.call("Runtime.enable", nil)
	c.call("Network.enable", nil)
	c.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 390, "height": 844, "deviceScaleFactor": 2, "mobile": true})
	return c
}

// chromeTarget is something Chrome can be told about: a page or a service
// worker, and the address to tell it at.
type chromeTarget struct {
	Type    string `json:"type"`
	URL     string `json:"url"`
	Address string `json:"webSocketDebuggerUrl"`
}

// closeChrome tells the browser at a port to close.
func closeChrome(port string) {
	if port == "" {
		return
	}
	res, err := http.Get("http://127.0.0.1:" + port + "/json/version")
	if err != nil {
		return
	}
	defer res.Body.Close()
	var version struct {
		Address string `json:"webSocketDebuggerUrl"`
	}
	if json.NewDecoder(res.Body).Decode(&version) != nil || version.Address == "" {
		return
	}
	conn, _, err := websocket.DefaultDialer.Dial(version.Address, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	if conn.WriteJSON(map[string]any{"id": 1, "method": "Browser.close"}) != nil || conn.SetReadDeadline(time.Now().Add(5*time.Second)) != nil {
		return
	}
	// An answer, or the connection closing: either way Chrome is closing.
	var answer json.RawMessage
	_ = conn.ReadJSON(&answer)
}

func chromeTargets(port string) []chromeTarget {
	res, err := http.Get("http://127.0.0.1:" + port + "/json/list")
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	var targets []chromeTarget
	if json.NewDecoder(res.Body).Decode(&targets) != nil {
		return nil
	}
	return targets
}

// call sends the page a command and waits for its answer.
func (c *chrome) call(method string, params any) json.RawMessage {
	c.t.Helper()
	return c.callOn(c.conn, method, params)
}

// callOn sends a command to the page or to a service worker. What Chrome
// reports in between is passed over: the tests look at the page instead.
func (c *chrome) callOn(conn *websocket.Conn, method string, params any) json.RawMessage {
	c.t.Helper()
	c.next++
	id := c.next
	if params == nil {
		params = map[string]any{}
	}
	if err := conn.WriteJSON(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		c.t.Fatalf("%s: %v", method, err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		c.t.Fatalf("%s: %v", method, err)
	}
	for {
		var m struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := conn.ReadJSON(&m); err != nil {
			c.t.Fatalf("%s: %v", method, err)
		}
		if m.ID != id {
			continue
		}
		if m.Error != nil {
			c.t.Fatalf("%s: %s", method, m.Error.Message)
		}
		return m.Result
	}
}

// open goes to a page of the app.
func (c *chrome) open(path string) {
	c.t.Helper()
	c.call("Page.navigate", map[string]any{"url": absolute(path)})
}

// reload loads the page again, as a phone short of memory does when the
// driver comes back from the camera.
func (c *chrome) reload() {
	c.t.Helper()
	c.call("Page.reload", nil)
}

// js runs an expression in the page and decodes its value into out. A
// promise is waited for.
func (c *chrome) js(expression string, out any) {
	c.t.Helper()
	raw := c.call("Runtime.evaluate", map[string]any{"expression": expression, "awaitPromise": true, "returnByValue": true})
	var r struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		Exception *struct {
			Text      string `json:"text"`
			Exception struct {
				Description string `json:"description"`
			} `json:"exception"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		c.t.Fatalf("%s: %v", expression, err)
	}
	if r.Exception != nil {
		c.t.Fatalf("%s: %s %s", expression, r.Exception.Text, r.Exception.Exception.Description)
	}
	if out != nil && len(r.Result.Value) > 0 {
		if err := json.Unmarshal(r.Result.Value, out); err != nil {
			c.t.Fatalf("%s: decoding %s: %v", expression, r.Result.Value, err)
		}
	}
}

// is reports whether a condition holds in the page. The condition may
// await, and one that is a promise is waited for. While the page is loading
// there may be nothing to ask about, which counts as no.
func (c *chrome) is(condition string) bool {
	c.t.Helper()
	raw := c.call("Runtime.evaluate", map[string]any{
		"expression": "(async () => (" + condition + "))().then(Boolean)", "awaitPromise": true, "returnByValue": true,
	})
	var r struct {
		Result struct {
			Value bool `json:"value"`
		} `json:"result"`
	}
	return json.Unmarshal(raw, &r) == nil && r.Result.Value
}

// waitFor waits until a condition holds in the page.
func (c *chrome) waitFor(what, condition string) {
	c.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for !c.is(condition) {
		if time.Now().After(deadline) {
			var page string
			c.js(`document.body ? document.body.innerText.slice(0, 600) : ""`, &page)
			c.t.Fatalf("waited 30s for %s. The page says:\n%s", what, page)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// fill types into the form fields a selector finds, one value for each in
// the order they are on the page, as the keyboard would: the app hears of
// it.
func (c *chrome) fill(selector string, values ...string) {
	c.t.Helper()
	list, _ := json.Marshal(values)
	c.js(fmt.Sprintf(`(() => {
		const fields = document.querySelectorAll(%q), values = %s;
		if (fields.length < values.length) throw new Error(fields.length + " fields for " + values.length + " values");
		values.forEach((v, i) => {
			fields[i].value = v;
			fields[i].dispatchEvent(new Event("input", { bubbles: true }));
		});
	})()`, selector, list), nil)
}

// click presses what a selector finds.
func (c *chrome) click(selector string) {
	c.t.Helper()
	c.js(fmt.Sprintf(`document.querySelector(%q).click()`, selector), nil)
}

// draw drags the pointer across what a selector finds, from a quarter of
// the way in to three quarters, as a finger signing does.
func (c *chrome) draw(selector string) {
	c.t.Helper()
	var box struct{ X, Y, W, H float64 }
	c.js(fmt.Sprintf(`(() => {
		const el = document.querySelector(%q);
		el.scrollIntoView({ block: "center" });
		const r = el.getBoundingClientRect();
		return { X: r.left, Y: r.top, W: r.width, H: r.height };
	})()`, selector), &box)
	if box.W == 0 || box.H == 0 {
		c.t.Fatalf("%s has no size to draw on", selector)
	}
	mouse := func(kind string, fx, fy float64) {
		c.call("Input.dispatchMouseEvent", map[string]any{
			"type": kind, "x": box.X + box.W*fx, "y": box.Y + box.H*fy, "button": "left", "buttons": 1, "clickCount": 1,
		})
	}
	mouse("mousePressed", 0.25, 0.3)
	mouse("mouseMoved", 0.4, 0.7)
	mouse("mouseMoved", 0.6, 0.3)
	mouse("mouseMoved", 0.75, 0.7)
	mouse("mouseReleased", 0.75, 0.7)
}

// offline takes the phone's signal away, or gives it back. The app's
// service worker loses it too: it makes its own requests, and would
// otherwise fetch the app from the network for a page that has none.
func (c *chrome) offline(off bool) {
	c.t.Helper()
	conditions := map[string]any{"offline": off, "latency": 0, "downloadThroughput": -1, "uploadThroughput": -1}
	c.call("Network.emulateNetworkConditions", conditions)
	for _, w := range c.workers {
		w.Close() // which gives a worker its signal back
	}
	c.workers = nil
	if !off {
		return
	}
	for _, target := range chromeTargets(c.port) {
		if target.Type != "service_worker" {
			continue
		}
		w, _, err := websocket.DefaultDialer.Dial(target.Address, nil)
		if err != nil {
			c.t.Fatalf("connecting to the service worker %s: %v", target.URL, err)
		}
		c.t.Cleanup(func() { w.Close() })
		c.callOn(w, "Network.enable", nil)
		c.callOn(w, "Network.emulateNetworkConditions", conditions)
		c.workers = append(c.workers, w)
		// Otherwise a test of working offline passes by working online.
		raw := c.callOn(w, "Runtime.evaluate", map[string]any{
			"expression":   `fetch("/healthz", { cache: "no-store" }).then(() => "reached", () => "unreachable")`,
			"awaitPromise": true, "returnByValue": true,
		})
		var r struct {
			Result struct {
				Value string `json:"value"`
			} `json:"result"`
		}
		if err := json.Unmarshal(raw, &r); err != nil {
			c.t.Fatalf("the service worker %s: %v", target.URL, err)
		}
		if r.Result.Value != "unreachable" {
			c.t.Fatalf("the service worker %s still reaches the server (%q)", target.URL, r.Result.Value)
		}
	}
	if len(c.workers) == 0 {
		c.t.Fatal("the page has no service worker to take offline")
	}
}

// at puts the phone at a place and lets the app ask where it is.
func (c *chrome) at(lat, lng float64) {
	c.t.Helper()
	c.call("Browser.grantPermissions", map[string]any{"origin": appURL, "permissions": []string{"geolocation"}})
	c.call("Emulation.setGeolocationOverride", map[string]any{"latitude": lat, "longitude": lng, "accuracy": 10})
}
