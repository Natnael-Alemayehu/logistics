//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestDriverAppInABrowser: the driver app itself, in a browser the size of
// a phone. The driver signs in with the PIN the office gave them and
// chooses their own, finds the load, and starts the trip. Then the signal
// goes. The delivery form half filled in survives the app being loaded
// again, with no network to load it from, the delivery is recorded with a
// signature, and the office has it once the signal is back.
func TestDriverAppInABrowser(t *testing.T) {
	office := signUp(t, "Lakeside Haulage")
	driverPhone := phoneNumber()
	p := office.post("/app/drivers", url.Values{"full_name": {"Tigist Haile"}, "phone": {driverPhone}})
	pin := secretRe.FindStringSubmatch(p.Body)
	if p.Status != http.StatusOK || pin == nil {
		t.Fatalf("adding the driver: status %d %s", p.Status, errorText(p.Body))
	}
	driverID := optionFor(t, office.get("/app/shipments/new").Body, "Tigist Haile")
	p = office.mustPost("/app/shipments", url.Values{
		"priority": {"normal"}, "origin_city": {"Addis Ababa"}, "origin_address": {"Main depot"},
		"destination_city": {"Adama"}, "destination_address": {"Customer warehouse"},
		"customer_name": {"Rift Valley Traders"}, "customer_phone": {phoneNumber()},
		"cargo_description": {"Flour, 300 sacks"}, "cargo_weight": {"15000"}, "driver_id": {driverID},
	})
	shipmentID := strings.TrimPrefix(p.Location, "/app/shipments/")
	live := listen(t, office)

	c := startChrome(t)
	c.at(8.5400, 39.2700) // Adama, where the app places the drop-off
	c.open("/driver/")

	// Signing in, and choosing a PIN.
	c.waitFor("the sign-in form", `document.querySelector('input[name="pin"]')`)
	c.fill(`input[name="phone"]`, driverPhone)
	c.fill(`input[name="pin"]`, pin[1])
	c.click(`form button[type="submit"]`)
	c.waitFor("being asked to choose a PIN", `document.querySelectorAll('form input[type="password"]').length === 3`)
	c.fill(`form input[type="password"]`, pin[1], "4826", "4826")
	c.click(`form button[type="submit"]`)

	// The load arrives with the first sync, and the trip starts.
	card := fmt.Sprintf(`a.ship[href="#/s/%s"]`, shipmentID)
	c.waitFor("the load on the driver's list", fmt.Sprintf(`document.querySelector(%q)`, card))
	c.click(card)
	c.waitFor("the load's page", `document.querySelector(".detail-head .st-assigned")`)
	c.click(".actions button.primary")
	c.waitFor("the trip to show as started", `document.querySelector(".detail-head .st-in_transit")`)
	live.waitFor(t, "shipment", shipmentID, "in_transit")

	// No signal from here. The service worker holds the app by now.
	c.waitFor("the service worker to take the page", `navigator.serviceWorker.controller`)
	c.waitFor("the app to be installed whole, in the version staging serves", installed)
	c.waitFor("the trip to be sent", outbox+` === 0`)
	c.offline(true)
	if c.is(`fetch("/healthz", { cache: "no-store" }).then(() => true, () => false)`) {
		t.Fatal("the page still reaches the server")
	}
	c.waitFor("the app to say there is no signal", `document.querySelector("[data-pill].off")`)

	// The delivery form, half filled in when the phone loads the app again.
	const recipient = `form input[maxlength="120"]`
	c.click(`.actions a[href$="/deliver"]`)
	c.waitFor("the delivery form", `document.querySelector("canvas.sig")`)
	// The form saves what is typed once it has looked for a draft to bring
	// back. No driver types faster than that look, but a test does.
	c.waitFor("the form to look for a draft", settled)
	c.fill(recipient, "Meseret Tadesse")
	c.draw("canvas.sig")
	c.waitFor("the form to be saved as a draft", draft(shipmentID)+`.then((d) => d && d.name === "Meseret Tadesse" && d.strokes.length === 1)`)
	c.reload()
	c.waitFor("the app to start without signal, with the form as it was left",
		fmt.Sprintf(`document.querySelector(%q).value === "Meseret Tadesse"`, recipient))

	// Recorded with the signature that came back with the draft: without
	// one the form refuses.
	c.click(`form button[type="submit"]`)
	c.waitFor("the delivery to be recorded on the phone", `document.querySelector(".detail-head .st-delivered") && document.querySelector(".pending")`)
	c.waitFor("the delivery to wait in the outbox", outbox+` === 1`)
	c.waitFor("the draft to go once the delivery is recorded", draft(shipmentID)+`.then((d) => !d)`)
	if strings.Contains(office.get("/app/shipments/"+shipmentID).Body, "Meseret Tadesse") {
		t.Fatal("the office has the delivery, though the phone has no signal")
	}

	// The signal is back: the delivery goes by itself.
	c.offline(false)
	live.waitFor(t, "shipment", shipmentID, "delivered")
	c.waitFor("the outbox to empty", outbox+` === 0`)
	c.waitFor("the app to say all is sent", `document.querySelector("[data-pill].ok")`)
	if !strings.Contains(office.get("/app/shipments/"+shipmentID).Body, "Meseret Tadesse") {
		t.Error("the shipment page doesn't show who received the load")
	}
	if p := office.get("/app/shipments/" + shipmentID + "/media/0"); p.Status != http.StatusOK || !strings.HasPrefix(p.Body, "\x89PNG") {
		t.Errorf("the signature drawn on the phone: status %d", p.Status)
	}

	// An update. Staging's files don't change, so the phone is made to hold
	// an older version: a copy of its files, marked, in use. Started again,
	// the app runs it and the worker downloads the version staging serves.
	c.js(older, nil)
	c.reload()
	c.waitFor("the app to run the older version", `document.body && document.body.hasAttribute("data-older")`)
	c.waitFor("the version staging serves to be downloaded and ready", `caches.open("kiflet-driver-state").then((s) => s.match("ready")).then((r) => !!r)`)
	// It doesn't take over while a form is open, the driver coming back to
	// the app or not.
	c.js(`location.hash = "#/pin"`, nil)
	c.waitFor("the form to change the PIN", `document.querySelectorAll('form input[type="password"]').length === 3`)
	c.js(`document.dispatchEvent(new Event("visibilitychange"))`, nil)
	time.Sleep(time.Second)
	if !c.is(`document.body.hasAttribute("data-older")`) {
		t.Fatal("the new version took over while a form was open")
	}
	// With none open, it does, and the app loads again in it.
	c.js(`location.hash = "#/menu"`, nil)
	c.waitFor("the menu", `!document.querySelector("form")`)
	c.js(`document.dispatchEvent(new Event("visibilitychange"))`, nil)
	c.waitFor("the new version to take over", `document.body && !document.body.hasAttribute("data-older") && document.readyState === "complete"`)
	c.waitFor("only the new version to be kept", installed)
}

// What the app keeps on the phone, read from its database as promises: how
// many changes wait to be sent, and the delivery form saved for a shipment.
const outbox = `(await new Promise((done, fail) => {
	const open = indexedDB.open("kiflet-driver");
	open.onerror = () => fail(open.error);
	open.onsuccess = () => {
		const count = open.result.transaction("outbox").objectStore("outbox").count();
		count.onerror = () => fail(count.error);
		count.onsuccess = () => { open.result.close(); done(count.result); };
	};
}))`

// installed holds when the service worker keeps one version of the app,
// the one the server serves now, with every file of it, and has it in use.
const installed = `(async () => {
	const build = await (await fetch("/driver/build.json", { cache: "no-store" })).json();
	const state = await caches.open("kiflet-driver-state");
	const current = await state.match("current").then((r) => r && r.text());
	const versions = (await caches.keys()).filter((k) => k.startsWith("kiflet-driver-") && k !== "kiflet-driver-state");
	if (current !== build.version || (await state.match("ready")) || versions.join() !== "kiflet-driver-" + build.version) return false;
	const cache = await caches.open(versions[0]);
	for (const path of Object.keys(build.files)) if (!(await cache.match(path))) return false;
	return true;
})()`

// older makes the phone hold an older version of the app, in use in place
// of the one it has: the same files, with the page marked data-older.
const older = `(async () => {
	const state = await caches.open("kiflet-driver-state");
	const current = await state.match("current").then((r) => r.text());
	const from = await caches.open("kiflet-driver-" + current);
	const to = await caches.open("kiflet-driver-older");
	for (const req of await from.keys()) {
		const res = await from.match(req);
		let body = await res.arrayBuffer();
		if (new URL(req.url).pathname === "/driver/") {
			body = new TextEncoder().encode(new TextDecoder().decode(body).replace("<body>", "<body data-older>"));
		}
		await to.put(req, new Response(body, { headers: res.headers }));
	}
	await state.put("current", new Response("older"));
	await caches.delete("kiflet-driver-" + current);
})()`

// settled waits for what the app has already asked of its key-value store:
// a transaction that writes to the store starts only once those before it
// have finished.
const settled = `new Promise((done, fail) => {
	const open = indexedDB.open("kiflet-driver");
	open.onerror = () => fail(open.error);
	open.onsuccess = () => {
		const tx = open.result.transaction("kv", "readwrite");
		tx.objectStore("kv").get("deviceId");
		tx.oncomplete = () => { open.result.close(); done(true); };
		tx.onerror = () => fail(tx.error);
	};
})`

func draft(shipmentID string) string {
	return fmt.Sprintf(`new Promise((done, fail) => {
		const open = indexedDB.open("kiflet-driver");
		open.onerror = () => fail(open.error);
		open.onsuccess = () => {
			const get = open.result.transaction("kv").objectStore("kv").get("draft:%s");
			get.onerror = () => fail(get.error);
			get.onsuccess = () => { open.result.close(); done(get.result); };
		};
	})`, shipmentID)
}
