// The outbox and the sync. Every change the driver makes goes into the
// outbox and stays there until the server has it. A sync sends the outbox
// and the positions recorded, and brings back the shipments, messages and
// settings. It runs every minute (every five when saving battery), sooner
// after a change, and less often while the server can't be reached.

import { ApiError, api } from "./api.js";
import { toast } from "./dom.js";
import { saving, stopTracking, updateTracking } from "./gps.js";
import { go, render, renderPill, rerender } from "./router.js";
import { adoptSettings } from "./settings.js";
import { ACTIVE, S, VERSION, uuid } from "./state.js";
import { clear, del, dropDrafts, kvSet, put, storageEstimate, tx } from "./store.js";
import { msgText, t } from "./text.js";

export async function enqueue(kind, payload) {
  const item = { client_id: uuid(), kind, payload: { ...payload }, created: Date.now() };
  item.payload.client_id = item.client_id;
  await put("outbox", item);
  S.outbox.push(item);
  renderPill();
  scheduleSync(800);
  return item;
}

// effective returns the shipment as the driver sees it: the server's copy
// with any changes still waiting in the outbox applied on top.
export function effective(sh) {
  const out = { ...sh, pending: false };
  const mine = S.outbox
    .filter((o) => o.payload.shipment_id === sh.id && (o.kind === "status" || o.kind === "pod"))
    .sort((a, b) => a.payload.recorded_at.localeCompare(b.payload.recorded_at));
  for (const o of mine) {
    out.pending = true;
    if (o.kind === "status") {
      out.status = o.payload.status;
      out.status_reason = o.payload.reason || "";
    } else {
      out.status = "delivered";
      out.delivered_at = o.payload.recorded_at;
    }
  }
  return out;
}

export function fixForEvent(maxAgeMs = 10 * 60 * 1000) {
  const f = S.lastFix;
  if (f && Date.now() - f.time < maxAgeMs) return { lat: f.lat, lng: f.lng };
  return { lat: null, lng: null };
}

export function freshFix(timeout = 10000) {
  return new Promise((resolve) => {
    if (!navigator.geolocation) return resolve(null);
    navigator.geolocation.getCurrentPosition(
      (p) => {
        S.lastFix = { lat: p.coords.latitude, lng: p.coords.longitude, acc: p.coords.accuracy, time: Date.now() };
        resolve(S.lastFix);
      },
      () => resolve(null),
      { enableHighAccuracy: true, timeout, maximumAge: 60000 },
    );
  });
}

let syncTimer = null;
export function scheduleSync(ms) {
  clearTimeout(syncTimer);
  syncTimer = setTimeout(sync, ms);
}

export async function sync() {
  clearTimeout(syncTimer);
  if (!S.token || S.syncing) return;
  if (!navigator.onLine) {
    S.online = false;
    renderPill();
    scheduleSync(30000);
    return;
  }
  S.syncing = true;
  renderPill();
  try {
    const statuses = S.outbox.filter((o) => o.kind === "status").map((o) => o.payload);
    const pods = S.outbox.filter((o) => o.kind === "pod").slice(0, S.weak ? 1 : 2).map((o) => o.payload);
    const messages = S.outbox.filter((o) => o.kind === "message").map((o) => o.payload);
    const sent = new Set([...statuses, ...pods, ...messages].map((p) => p.client_id));
    const locKeys = [];
    const locations = [];
    await tx("locations", "readonly", (s) => {
      const r = s.openCursor();
      return new Promise((resolve) => {
        r.onsuccess = () => {
          const c = r.result;
          if (!c || locations.length >= 1000) return resolve();
          locKeys.push(c.key);
          locations.push(c.value);
          c.continue();
        };
      });
    });
    S.storage = await storageEstimate();
    const rev = S.settingsRev;
    const changed = Object.keys(S.changed).length ? S.changed : undefined;
    const body = {
      device_id: S.deviceId, app_version: VERSION, device_time: new Date().toISOString(),
      battery: S.battery, pending: S.outbox.length, tracking_paused: !S.tracking, power_saving: saving(),
      storage_kb: S.storage ? Math.round(S.storage.usage / 1024) : null,
      settings: changed,
      since: S.cursor, statuses, pods, locations, messages,
    };
    const started = Date.now();
    // Long enough for photos over a slow connection; a stalled one ends.
    const resp = await api("/driver/sync", { method: "POST", body, compress: true, timeout: pods.length ? 180000 : 60000 });
    if (changed && rev === S.settingsRev) {
      S.changed = {};
      await kvSet("settingsChanged", {});
    }
    await applySync(resp, locKeys);
    S.online = true;
    S.failures = 0;
    // A round trip this slow means a weak signal (AC-015.1). Photos take
    // longer anyway.
    S.weak = Date.now() - started > (pods.length ? 60000 : 10000);
    S.lastSync = Date.now();
    await kvSet("lastSync", S.lastSync);
    // More waiting: carry on straight away. PODs go two at a time, a
    // response carries only so many messages, and what the driver recorded
    // while this sync ran wasn't in it (enqueue's sync gave way to this
    // one). Saving battery, the app syncs every five minutes unless the
    // driver acts.
    const more = resp.more || S.outbox.some((o) => o.kind === "pod" || !sent.has(o.client_id));
    scheduleSync(more ? 500 : saving() ? 300000 : 60000);
    if (S.blocked) {
      S.blocked = false;
      rerender();
    }
  } catch (err) {
    S.failures++;
    if (err instanceof ApiError && err.status === 401) {
      await signedOut();
      return;
    }
    // The company's account is suspended: say so, and ask again now and
    // then rather than every few seconds. Nothing recorded is dropped.
    if (err instanceof ApiError && err.status === 403) {
      if (!S.blocked) {
        S.blocked = true;
        rerender();
      }
      scheduleSync(600000);
      return;
    }
    // The phone says it is online, but the server can't be reached.
    if (!(err instanceof ApiError)) {
      if (!navigator.onLine) S.online = false;
      else if (S.failures >= 2) S.weak = true;
    }
    scheduleSync(Math.min(300000, 15000 * 2 ** Math.min(S.failures, 4)));
  } finally {
    S.syncing = false;
    renderPill();
  }
}

async function applySync(resp, locKeys) {
  // Outbox: drop everything the server accepted or refused for good. What
  // was refused stays on screen until the driver dismisses it, even if
  // the app restarts.
  let noticesChanged = false;
  const byId = new Map(S.outbox.map((o) => [o.client_id, o]));
  for (const r of resp.results || []) {
    const item = byId.get(r.client_id);
    if (!item) continue;
    if (r.ok || !r.retry) {
      await del("outbox", r.client_id);
      byId.delete(r.client_id);
      if (!r.ok && !S.notices.some((n) => n.id === r.client_id)) {
        const sh = S.shipments.get(item.payload.shipment_id);
        S.notices.push({ id: r.client_id, text: (sh ? sh.ref + ": " : "") + r.message });
        noticesChanged = true;
      }
    }
  }
  S.outbox = [...byId.values()];
  if (noticesChanged) await kvSet("notices", S.notices);
  if (locKeys.length) {
    await tx("locations", "readwrite", (s) => locKeys.forEach((k) => s.delete(k)));
    S.pendingLocations = Math.max(0, S.pendingLocations - locKeys.length);
  }

  // Shipments: the server sends what the device should hold.
  for (const sh of resp.shipments || []) {
    S.shipments.set(sh.id, sh);
    await put("shipments", sh);
  }
  for (const id of resp.removed_shipment_ids || []) {
    if (S.shipments.has(id)) {
      S.shipments.delete(id);
      await del("shipments", id);
      await del("kv", "draft:" + id);
      if (location.hash.includes(id)) toast(t("shipmentGone"));
    }
  }
  const weekAgo = Date.now() - 7 * 864e5;
  for (const sh of S.shipments.values()) {
    if (!ACTIVE.includes(sh.status) && new Date(sh.updated_at).getTime() < weekAgo) {
      S.shipments.delete(sh.id);
      await del("shipments", sh.id);
    }
  }

  // The driver's settings first: they say whether a new message vibrates.
  if (resp.driver) {
    S.driver = { ...S.driver, ...resp.driver };
    await kvSet("driver", S.driver);
    await adoptSettings(resp.driver.settings);
  }

  // Messages from the office. The toast shows the newest that is new to
  // the phone, not whatever happens to be newest.
  let newest = null;
  const known = new Set(S.messages.map((m) => m.id));
  for (const m of resp.messages || []) {
    if (!known.has(m.id)) {
      known.add(m.id);
      S.messages.push(m);
      if (!S.seen.has(m.id) && (!newest || m.at > newest.at)) newest = m;
    }
  }
  S.messages.sort((a, b) => b.at.localeCompare(a.at));
  S.messages = S.messages.slice(0, 50);
  await kvSet("messages", S.messages);
  if (newest) {
    toast(t("messages") + ": " + msgText(newest));
    if (S.settings.vibrate && navigator.vibrate) navigator.vibrate([120, 80, 120]);
  }
  S.cursor = resp.cursor;
  await kvSet("cursor", S.cursor);
  updateTracking();
  rerender();
}

// The session was ended elsewhere: the office signed this phone out, or
// the PIN was reset. Customers' names and numbers go with it. What the
// driver recorded and hasn't sent stays for their next sign-in; another
// driver signing in clears it (signedIn).
export async function signedOut() {
  S.token = null;
  await kvSet("token", null);
  await forgetShipments();
  stopTracking();
  toast(t("signedOut"));
  go("#/");
  render();
}

export async function forgetShipments() {
  await dropDrafts();
  await clear("shipments");
  S.shipments.clear();
  S.messages = [];
  S.cursor = null;
  await kvSet("messages", []);
  await kvSet("cursor", null);
}
