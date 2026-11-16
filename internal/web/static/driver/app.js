// Kiflet driver app. Offline-first: everything the driver does is written to
// IndexedDB first and sent to the server by a background sync loop, so the
// app behaves the same with or without signal.
//
// This module starts it. The rest, by what each does: state.js, text.js
// (English and Amharic), store.js (IndexedDB), api.js, sync.js (the outbox
// and the sync), settings.js, gps.js, router.js, dom.js, signature.js, and
// one module per screen in screens/. index.html names every module in a
// modulepreload link, so they load together rather than one import at a
// time, and the service worker installs them as one version (sw.js).

import { api } from "./api.js";
import { h, root, toast } from "./dom.js";
import { updateTracking, updateWakeLock } from "./gps.js";
import { render, renderPill, rerender } from "./router.js";
import { DEFAULT_SETTINGS, S, VERSION, uuid } from "./state.js";
import { all, kvGet, kvSet, protectStorage, req2p, storageEstimate, tx } from "./store.js";
import { scheduleSync, sync } from "./sync.js";
import { t } from "./text.js";

// The config the phone has is used at once, so the app opens without
// waiting on a weak signal, and the server's is fetched behind it. Only the
// very first start waits for the network.
async function loadConfig() {
  const cached = await kvGet("config");
  const fresh = api("/driver/config", { auth: false, timeout: cached ? 30000 : 15000 }).then(async (cfg) => {
    await kvSet("config", cfg);
    return cfg;
  });
  if (!cached) return fresh.catch(() => null);
  fresh.then((cfg) => {
    if (JSON.stringify(cfg) === JSON.stringify(S.config)) return;
    S.config = cfg;
    rerender();
  }).catch(() => {});
  return cached;
}
// A first start without signal has nothing to draw: try again when the
// signal or the driver comes back.
async function retryConfig() {
  if (S.config) return;
  const cfg = await loadConfig();
  if (cfg && !S.config) {
    S.config = cfg;
    render();
  }
}

// Updates. The service worker downloads a new version whole and keeps it
// ready (sw.js); it takes over when the app next starts. The app hands
// over sooner when that interrupts nothing: when it comes back to the
// screen with no form open. It then loads again, in the new version, with
// drafts and the outbox where they were.
function watchUpdates() {
  if (!("serviceWorker" in navigator)) return;
  const sw = navigator.serviceWorker;
  let ready = false;
  sw.addEventListener("message", (event) => {
    if (event.data === "ready") ready = true;
    else if (event.data === "switched") location.reload();
  });
  sw.register("/driver/sw.js", { scope: "/driver/" }).catch(() => {});
  // The first worker downloads the app as it installs.
  const ask = (message) => sw.controller && sw.controller.postMessage(message);
  ask("check");
  setInterval(() => {
    if (navigator.onLine) ask("check");
  }, 60 * 60 * 1000);
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible" && ready && !S.formOpen) ask("take-over");
  });
}

async function boot() {
  watchUpdates();
  S.deviceId = await kvGet("deviceId");
  if (!S.deviceId) {
    S.deviceId = uuid();
    await kvSet("deviceId", S.deviceId);
  }
  S.settings = { ...DEFAULT_SETTINGS, ...((await kvGet("settings")) || {}) };
  S.changed = (await kvGet("settingsChanged")) || {};
  S.lang = S.settings.language || ((navigator.language || "").startsWith("am") ? "am" : "en");
  document.documentElement.lang = S.lang;
  S.keepAwake = (await kvGet("keepAwake")) === true;
  S.token = await kvGet("token");
  S.driver = await kvGet("driver");
  S.cursor = await kvGet("cursor");
  S.lastSync = await kvGet("lastSync");
  S.messages = (await kvGet("messages")) || [];
  S.seen = new Set((await kvGet("seen")) || []);
  S.notices = (await kvGet("notices")) || [];
  const tracking = await kvGet("tracking");
  S.tracking = tracking !== false;
  for (const sh of await all("shipments")) S.shipments.set(sh.id, sh);
  S.outbox = await all("outbox");
  S.pendingLocations = await tx("locations", "readonly", (s) => req2p(s.count()));
  S.config = await loadConfig();
  // A new version may send fields the stored shipments don't have yet:
  // download them again.
  const was = await kvGet("version");
  if (was !== VERSION) {
    if (was && S.token) toast(t("appUpdated"));
    S.cursor = null;
    await kvSet("cursor", null);
    await kvSet("version", VERSION);
  }
  S.storage = await storageEstimate();
  if (S.token) protectStorage();

  if (navigator.getBattery) {
    navigator.getBattery().then((b) => {
      const upd = () => {
        S.battery = Math.round(b.level * 100);
        S.charging = b.charging;
        updateTracking(); // battery saver may turn on or off
      };
      upd();
      b.addEventListener("levelchange", upd);
      b.addEventListener("chargingchange", upd);
    }).catch(() => {});
  }
  window.addEventListener("hashchange", render);
  window.addEventListener("online", () => { S.online = true; renderPill(); retryConfig(); scheduleSync(500); });
  window.addEventListener("offline", () => { S.online = false; renderPill(); });
  document.addEventListener("visibilitychange", () => {
    if (!document.hidden) {
      retryConfig();
      scheduleSync(300);
    }
    updateWakeLock(); // the browser lets go of it while the app is hidden
  });
  window.addEventListener("beforeinstallprompt", (e) => { e.preventDefault(); S.installPrompt = e; rerender(); });
  setInterval(renderPill, 30000);

  render();
  if (S.token) {
    updateTracking();
    sync();
  }
}

boot().catch((e) => {
  root().replaceChildren(h("div", { class: "boot" }, h("p", { text: "The app couldn't start: " + e.message })));
});
