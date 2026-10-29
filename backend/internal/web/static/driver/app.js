// Menged driver app. Offline-first: everything the driver does is written to
// IndexedDB first and sent to the server by a background sync loop, so the
// app behaves the same with or without signal.
(() => {
  "use strict";

  const VERSION = "1.0.0";
  const API = "/api/v1";
  const ON_ROAD = ["in_transit", "delayed", "arrived", "issue"];
  const ACTIVE = ["assigned", ...ON_ROAD];
  const MAX_PHOTOS = 3;

  // ---------------------------------------------------------------------------
  // Text. Amharic is the default for drivers whose phone is set to it.
  // ---------------------------------------------------------------------------
  const TEXT = {
    en: {
      signIn: "Sign in", phone: "Phone number", pin: "PIN", forgotPin: "Forgot your PIN?",
      signInHelp: "Use the phone number your company registered and the PIN you received by SMS.",
      sendCode: "Send me a code", code: "Code from SMS", newPin: "New PIN (4–6 digits)", newPinAgain: "New PIN again",
      setPin: "Save PIN", choosePin: "Choose your PIN", choosePinHelp: "Pick a PIN only you know. You'll use it to sign in.",
      currentPin: "Current PIN", pinsDiffer: "The PINs don't match.", back: "Back", cancel: "Cancel", save: "Save", send: "Send",
      myShipments: "My shipments", active: "Active", done: "Finished this week", none: "No shipments assigned to you right now.",
      noneHelp: "New work appears here automatically when the office assigns it.",
      offline: "No signal", syncing: "Sending…", waiting: "{n} waiting to send", allSent: "All sent", lastSync: "Updated {t}",
      neverSynced: "Not synced yet", syncNow: "Sync now", from: "From", to: "To", due: "Due", customer: "Customer",
      cargo: "Cargo", instructions: "Instructions from the office", call: "Call", navigate: "Directions", vehicle: "Vehicle",
      startTrip: "Start trip", backOnRoad: "Back on the road", arrivedBtn: "I've arrived", delayBtn: "Report a delay",
      problemBtn: "Report a problem", deliverBtn: "Record delivery", messageBtn: "Message the office",
      pendingSend: "Saved on the phone — will send when there's signal", reason: "Reason", note: "Note (optional)",
      recipient: "Received by (name)", recipientPhone: "Their phone (optional)", signature: "Signature",
      signHere: "Ask the recipient to sign here", clear: "Clear", photos: "Photos (up to 3)", addPhoto: "Take photo",
      remove: "Remove", deliveryNotes: "Notes (optional)", confirmDelivery: "Confirm delivery",
      needName: "Enter who received the cargo.", needSignature: "The recipient must sign.",
      locating: "Getting your location…", noFix: "Couldn't get a GPS fix. The delivery is saved without a location.",
      delivered: "Delivery saved.", statusSaved: "Saved.", messageSaved: "Message saved. It goes with the next sync.",
      messages: "From the office", messagePlaceholder: "e.g. Road closed at Awash, taking the long route",
      menu: "Menu", language: "Language", location: "Share my location while working", locationHelp: "The office sees where you are while you have active shipments. Your location is never shown to customers.",
      changePin: "Change PIN", signOut: "Sign out", signOutPending: "{n} updates haven't been sent yet. If you sign out now they are lost.",
      signOutAnyway: "Sign out anyway", smsHelp: "No data? Text {ref} and a code to {num}: 1 departed, 2 delayed, 3 arrived, 4 delivered, 5 problem.",
      smsHelpGeneric: "No data? Text the shipment's last 4 digits and a code to {num}: 1 departed, 2 delayed, 3 arrived, 4 delivered, 5 problem.",
      rejected: "Not applied", dismiss: "Dismiss", priority_high: "High priority", priority_urgent: "Urgent",
      signedOut: "You've been signed out. Sign in again.", networkError: "No connection. Try again when you have signal.",
      codeSent: "If this number belongs to a driver, a code is on its way.", version: "Version", deliveredAt: "Delivered",
      shipmentGone: "This shipment is no longer assigned to you.", install: "Install the app", installHelp: "Add it to your home screen so it opens without the browser.",
      locationOff: "Location is off", battery: "Battery", appUpdated: "App updated.", pinChanged: "PIN changed.",
      kg: "kg",
    },
    am: {
      signIn: "ግባ", phone: "ስልክ ቁጥር", pin: "ፒን", forgotPin: "ፒንዎን ረሱ?",
      signInHelp: "ድርጅትዎ የመዘገበውን ስልክ ቁጥር እና በኤስኤምኤስ የደረሰዎትን ፒን ያስገቡ።",
      sendCode: "ኮድ ላክልኝ", code: "በኤስኤምኤስ የመጣው ኮድ", newPin: "አዲስ ፒን (4–6 አሃዝ)", newPinAgain: "አዲሱን ፒን ይድገሙ",
      setPin: "ፒን አስቀምጥ", choosePin: "ፒንዎን ይምረጡ", choosePinHelp: "እርስዎ ብቻ የሚያውቁትን ፒን ይምረጡ።",
      currentPin: "የአሁኑ ፒን", pinsDiffer: "ፒኖቹ አይመሳሰሉም።", back: "ተመለስ", cancel: "ተው", save: "አስቀምጥ", send: "ላክ",
      myShipments: "ጭነቶቼ", active: "በሂደት ላይ", done: "በዚህ ሳምንት የተጠናቀቁ", none: "አሁን የተመደበልዎት ጭነት የለም።",
      noneHelp: "ቢሮው ሥራ ሲመድብልዎ እዚህ ይታያል።",
      offline: "ኔትወርክ የለም", syncing: "በመላክ ላይ…", waiting: "{n} ለመላክ በመጠባበቅ ላይ", allSent: "ሁሉም ተልኳል", lastSync: "የተዘመነው {t}",
      neverSynced: "ገና አልተመሳሰለም", syncNow: "አሁን አመሳስል", from: "ከ", to: "ወደ", due: "የሚደርስበት", customer: "ደንበኛ",
      cargo: "ጭነት", instructions: "ከቢሮ የተሰጠ መመሪያ", call: "ደውል", navigate: "አቅጣጫ", vehicle: "ተሽከርካሪ",
      startTrip: "ጉዞ ጀምር", backOnRoad: "ጉዞ ቀጥል", arrivedBtn: "ደርሻለሁ", delayBtn: "መዘግየት አሳውቅ",
      problemBtn: "ችግር አሳውቅ", deliverBtn: "ርክክብ መዝግብ", messageBtn: "ለቢሮ መልዕክት",
      pendingSend: "በስልኩ ተቀምጧል — ኔትወርክ ሲገኝ ይላካል", reason: "ምክንያት", note: "ማስታወሻ (አማራጭ)",
      recipient: "የተረከበው ሰው ስም", recipientPhone: "ስልኩ (አማራጭ)", signature: "ፊርማ",
      signHere: "ተረካቢው እዚህ ይፈርሙ", clear: "አጥፋ", photos: "ፎቶዎች (እስከ 3)", addPhoto: "ፎቶ አንሳ",
      remove: "አስወግድ", deliveryNotes: "ማስታወሻ (አማራጭ)", confirmDelivery: "ርክክቡን አረጋግጥ",
      needName: "ጭነቱን የተረከበውን ሰው ስም ያስገቡ።", needSignature: "ተረካቢው መፈረም አለበት።",
      locating: "አካባቢዎን በማግኘት ላይ…", noFix: "ጂፒኤስ አልተገኘም። ርክክቡ ያለ አካባቢ ተቀምጧል።",
      delivered: "ርክክቡ ተቀምጧል።", statusSaved: "ተቀምጧል።", messageSaved: "መልዕክቱ ተቀምጧል። በሚቀጥለው ማመሳሰል ይላካል።",
      messages: "ከቢሮ", messagePlaceholder: "ለምሳሌ፦ አዋሽ ላይ መንገድ ተዘግቷል",
      menu: "ምናሌ", language: "ቋንቋ", location: "በሥራ ላይ ሳለሁ አካባቢዬን አጋራ", locationHelp: "ንቁ ጭነት ሲኖርዎት ቢሮው የት እንዳሉ ያያል። አካባቢዎ ለደንበኞች አይታይም።",
      changePin: "ፒን ቀይር", signOut: "ውጣ", signOutPending: "{n} ያልተላኩ መረጃዎች አሉ። አሁን ከወጡ ይጠፋሉ።",
      signOutAnyway: "ቢሆንም ውጣ", smsHelp: "ዳታ የለም? {ref} እና ኮድ ወደ {num} ይላኩ፦ 1 ተነሳ፣ 2 ዘገየ፣ 3 ደረሰ፣ 4 ተረከበ፣ 5 ችግር።",
      smsHelpGeneric: "ዳታ የለም? የጭነቱን የመጨረሻ 4 አሃዝ እና ኮድ ወደ {num} ይላኩ፦ 1 ተነሳ፣ 2 ዘገየ፣ 3 ደረሰ፣ 4 ተረከበ፣ 5 ችግር።",
      rejected: "አልተተገበረም", dismiss: "ዝጋ", priority_high: "ከፍተኛ ቅድሚያ", priority_urgent: "አስቸኳይ",
      signedOut: "ከመለያዎ ወጥተዋል። እንደገና ይግቡ።", networkError: "ግንኙነት የለም። ኔትወርክ ሲኖር እንደገና ይሞክሩ።",
      codeSent: "ቁጥሩ የሹፌር ከሆነ ኮድ ይላካል።", version: "ስሪት", deliveredAt: "የተረከበው",
      shipmentGone: "ይህ ጭነት ከእንግዲህ ለእርስዎ አልተመደበም።", install: "መተግበሪያውን ጫን", installHelp: "ያለ ብሮውዘር እንዲከፈት ወደ መነሻ ገጽ ያክሉት።",
      locationOff: "አካባቢ ጠፍቷል", battery: "ባትሪ", appUpdated: "መተግበሪያው ተዘምኗል።", pinChanged: "ፒን ተቀይሯል።",
      kg: "ኪ.ግ",
    },
  };
  const STATUS_AM = {
    pending: "በመጠባበቅ ላይ", assigned: "ተመድቧል", in_transit: "በጉዞ ላይ", delayed: "ዘግይቷል",
    arrived: "መድረሻ ደርሷል", delivered: "ተረክቧል", issue: "ችግር አለ", cancelled: "ተሰርዟል",
  };
  const REASON_AM = {
    road_conditions: "የመንገድ ሁኔታ", weather: "የአየር ሁኔታ", security_checkpoint: "የፍተሻ ኬላ", mechanical_issue: "የመኪና ብልሽት",
    traffic: "የትራፊክ መጨናነቅ", accident: "አደጋ", breakdown: "መኪናው ተበላሽቷል", recipient_unavailable: "ተቀባዩ የለም",
    wrong_address: "የተሳሳተ አድራሻ", cargo_damaged: "ጭነቱ ተጎድቷል", recipient_refused: "ተቀባዩ አልተቀበለም", other: "ሌላ",
  };

  // ---------------------------------------------------------------------------
  // Storage
  // ---------------------------------------------------------------------------
  const DB_NAME = "menged-driver";
  let dbp = null;
  function idb() {
    if (dbp) return dbp;
    dbp = new Promise((resolve, reject) => {
      const req = indexedDB.open(DB_NAME, 1);
      req.onupgradeneeded = () => {
        const d = req.result;
        d.createObjectStore("kv");
        d.createObjectStore("shipments", { keyPath: "id" });
        d.createObjectStore("outbox", { keyPath: "client_id" });
        d.createObjectStore("locations", { autoIncrement: true });
      };
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    });
    return dbp;
  }
  async function tx(store, mode, fn) {
    const d = await idb();
    return new Promise((resolve, reject) => {
      const t = d.transaction(store, mode);
      const s = t.objectStore(store);
      let out;
      Promise.resolve(fn(s)).then((v) => (out = v));
      t.oncomplete = () => resolve(out);
      t.onerror = () => reject(t.error);
      t.onabort = () => reject(t.error);
    });
  }
  const req2p = (r) => new Promise((res, rej) => ((r.onsuccess = () => res(r.result)), (r.onerror = () => rej(r.error))));
  const kvGet = (k) => tx("kv", "readonly", (s) => req2p(s.get(k)));
  const kvSet = (k, v) => tx("kv", "readwrite", (s) => s.put(v, k));
  const all = (store) => tx(store, "readonly", (s) => req2p(s.getAll()));
  const put = (store, v) => tx(store, "readwrite", (s) => s.put(v));
  const del = (store, k) => tx(store, "readwrite", (s) => s.delete(k));
  const clear = (store) => tx(store, "readwrite", (s) => s.clear());

  // ---------------------------------------------------------------------------
  // State
  // ---------------------------------------------------------------------------
  const S = {
    token: null, driver: null, config: null, deviceId: null, cursor: null, lang: "en",
    shipments: new Map(), outbox: [], messages: [], seen: new Set(), notices: [],
    tracking: true, lastFix: null, lastRecorded: null, battery: null, pendingLocations: 0,
    online: navigator.onLine, syncing: false, lastSync: null, failures: 0, installPrompt: null,
  };

  function t(key, vars) {
    let s = (TEXT[S.lang] && TEXT[S.lang][key]) || TEXT.en[key] || key;
    for (const [k, v] of Object.entries(vars || {})) s = s.replace("{" + k + "}", v);
    return s;
  }
  function statusLabel(st) {
    if (S.lang === "am" && STATUS_AM[st]) return STATUS_AM[st];
    const found = S.config && S.config.statuses.find((x) => x.code === st);
    return found ? found.label : st;
  }
  function reasonLabel(code) {
    if (S.lang === "am" && REASON_AM[code]) return REASON_AM[code];
    const list = S.config ? [...S.config.delay_reasons, ...S.config.issue_reasons] : [];
    const found = list.find((x) => x.code === code);
    return found ? found.label : code;
  }

  function uuid() {
    if (crypto.randomUUID) return crypto.randomUUID();
    const b = crypto.getRandomValues(new Uint8Array(16));
    b[6] = (b[6] & 0x0f) | 0x40;
    b[8] = (b[8] & 0x3f) | 0x80;
    const h = [...b].map((x) => x.toString(16).padStart(2, "0")).join("");
    return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
  }

  // ---------------------------------------------------------------------------
  // DOM
  // ---------------------------------------------------------------------------
  function h(tag, props, ...children) {
    const n = document.createElement(tag);
    for (const [k, v] of Object.entries(props || {})) {
      if (v == null || v === false) continue;
      if (k === "class") n.className = v;
      else if (k === "text") n.textContent = v;
      else if (k.startsWith("on")) n.addEventListener(k.slice(2), v);
      else if (v === true) n.setAttribute(k, "");
      else n.setAttribute(k, v);
    }
    for (const c of children.flat()) {
      if (c == null || c === false) continue;
      n.append(c instanceof Node ? c : document.createTextNode(String(c)));
    }
    return n;
  }
  const root = () => document.getElementById("app");

  function toast(msg, kind) {
    let box = document.querySelector(".toasts");
    if (!box) document.body.append((box = h("div", { class: "toasts", "aria-live": "polite" })));
    const n = h("div", { class: "toast " + (kind || ""), role: "status", text: msg });
    box.append(n);
    setTimeout(() => n.remove(), 4500);
  }

  function fmtTime(iso) {
    if (!iso) return "";
    const d = new Date(iso);
    const now = new Date();
    const opts = { hour: "2-digit", minute: "2-digit" };
    if (d.toDateString() !== now.toDateString()) Object.assign(opts, { weekday: "short", day: "numeric", month: "short" });
    return d.toLocaleString(S.lang === "am" ? "am-ET" : "en-GB", opts);
  }
  function ago(ms) {
    const s = (Date.now() - ms) / 1000;
    if (s < 60) return S.lang === "am" ? "አሁን" : "just now";
    if (s < 3600) return Math.floor(s / 60) + (S.lang === "am" ? " ደቂቃ በፊት" : " min ago");
    return Math.floor(s / 3600) + (S.lang === "am" ? " ሰዓት በፊት" : " h ago");
  }

  // ---------------------------------------------------------------------------
  // Server
  // ---------------------------------------------------------------------------
  class ApiError extends Error {
    constructor(status, body) {
      super((body && body.error && body.error.message) || "Request failed");
      this.status = status;
      this.code = body && body.error && body.error.code;
      this.field = body && body.error && body.error.field;
    }
  }

  async function gzip(text) {
    if (!("CompressionStream" in window)) return null;
    const stream = new Blob([text]).stream().pipeThrough(new CompressionStream("gzip"));
    return new Response(stream).arrayBuffer();
  }

  async function api(path, { method = "GET", body, auth = true, compress = false } = {}) {
    const headers = { Accept: "application/json" };
    let payload;
    if (body !== undefined) {
      headers["Content-Type"] = "application/json";
      payload = JSON.stringify(body);
      if (compress && payload.length > 2048) {
        const gz = await gzip(payload);
        if (gz) {
          payload = gz;
          headers["Content-Encoding"] = "gzip";
        }
      }
    }
    if (auth && S.token) headers.Authorization = "Bearer " + S.token;
    const res = await fetch(API + path, { method, headers, body: payload, cache: "no-store" });
    let data = null;
    try {
      data = await res.json();
    } catch {
      /* empty body */
    }
    if (!res.ok) throw new ApiError(res.status, data);
    return data;
  }

  // ---------------------------------------------------------------------------
  // Outbox: every change the driver makes, until the server has it.
  // ---------------------------------------------------------------------------
  async function enqueue(kind, payload) {
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
  function effective(sh) {
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

  function fixForEvent(maxAgeMs = 10 * 60 * 1000) {
    const f = S.lastFix;
    if (f && Date.now() - f.time < maxAgeMs) return { lat: f.lat, lng: f.lng };
    return { lat: null, lng: null };
  }

  function freshFix(timeout = 10000) {
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

  // ---------------------------------------------------------------------------
  // Sync
  // ---------------------------------------------------------------------------
  let syncTimer = null;
  function scheduleSync(ms) {
    clearTimeout(syncTimer);
    syncTimer = setTimeout(sync, ms);
  }

  async function sync() {
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
      const pods = S.outbox.filter((o) => o.kind === "pod").slice(0, 2).map((o) => o.payload);
      const messages = S.outbox.filter((o) => o.kind === "message").map((o) => o.payload);
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
      const body = {
        device_id: S.deviceId, app_version: VERSION, device_time: new Date().toISOString(),
        battery: S.battery, pending: S.outbox.length, tracking_paused: !S.tracking,
        since: S.cursor, statuses, pods, locations, messages,
      };
      const resp = await api("/driver/sync", { method: "POST", body, compress: true });
      await applySync(resp, locKeys);
      S.online = true;
      S.failures = 0;
      S.lastSync = Date.now();
      await kvSet("lastSync", S.lastSync);
      // More waiting (PODs go two at a time): carry on straight away.
      scheduleSync(S.outbox.some((o) => o.kind === "pod") ? 500 : 60000);
    } catch (err) {
      S.failures++;
      if (err instanceof ApiError && err.status === 401) {
        await signedOut();
        return;
      }
      if (!(err instanceof ApiError)) S.online = false;
      scheduleSync(Math.min(300000, 15000 * 2 ** Math.min(S.failures, 4)));
    } finally {
      S.syncing = false;
      renderPill();
    }
  }

  async function applySync(resp, locKeys) {
    // Outbox: drop everything the server accepted or refused for good.
    const byId = new Map(S.outbox.map((o) => [o.client_id, o]));
    for (const r of resp.results || []) {
      const item = byId.get(r.client_id);
      if (!item) continue;
      if (r.ok || !r.retry) {
        await del("outbox", r.client_id);
        byId.delete(r.client_id);
        if (!r.ok) {
          const sh = S.shipments.get(item.payload.shipment_id);
          S.notices.push({ id: r.client_id, text: (sh ? sh.ref + ": " : "") + r.message });
        }
      }
    }
    S.outbox = [...byId.values()];
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

    // Messages from the office.
    let fresh = 0;
    const known = new Set(S.messages.map((m) => m.id));
    for (const m of resp.messages || []) {
      if (!known.has(m.id)) {
        S.messages.push(m);
        if (!S.seen.has(m.id)) fresh++;
      }
    }
    S.messages.sort((a, b) => b.at.localeCompare(a.at));
    S.messages = S.messages.slice(0, 50);
    await kvSet("messages", S.messages);
    if (fresh) {
      toast(t("messages") + ": " + S.messages[0].text);
      if (navigator.vibrate) navigator.vibrate([120, 80, 120]);
    }

    if (resp.driver) {
      S.driver = { ...S.driver, ...resp.driver };
      await kvSet("driver", S.driver);
    }
    S.cursor = resp.cursor;
    await kvSet("cursor", S.cursor);
    updateTracking();
    rerender();
  }

  async function signedOut() {
    S.token = null;
    await kvSet("token", null);
    stopTracking();
    toast(t("signedOut"));
    go("#/");
    render();
  }

  // ---------------------------------------------------------------------------
  // GPS
  // ---------------------------------------------------------------------------
  let watchId = null;
  function onRoadShipment() {
    const list = [...S.shipments.values()].map(effective);
    return list.find((s) => ON_ROAD.includes(s.status)) || list.find((s) => s.status === "assigned");
  }
  function updateTracking() {
    const want = S.token && S.tracking && navigator.geolocation && onRoadShipment();
    if (want && watchId == null) {
      watchId = navigator.geolocation.watchPosition(onFix, () => {}, { enableHighAccuracy: true, maximumAge: 30000, timeout: 60000 });
    } else if (!want) stopTracking();
  }
  function stopTracking() {
    if (watchId != null) navigator.geolocation.clearWatch(watchId);
    watchId = null;
  }
  function metres(a, b) {
    const R = 6371000, rad = Math.PI / 180;
    const dLat = (b.lat - a.lat) * rad, dLng = (b.lng - a.lng) * rad;
    const x = Math.sin(dLat / 2) ** 2 + Math.cos(a.lat * rad) * Math.cos(b.lat * rad) * Math.sin(dLng / 2) ** 2;
    return 2 * R * Math.asin(Math.sqrt(x));
  }
  async function onFix(p) {
    const fix = { lat: p.coords.latitude, lng: p.coords.longitude, acc: p.coords.accuracy, time: Date.now() };
    S.lastFix = fix;
    if (fix.acc > 500) return; // too vague to be useful
    const last = S.lastRecorded;
    const elapsed = last ? fix.time - last.time : Infinity;
    // A point a minute when moving, one every five minutes when parked.
    if (last && !(elapsed >= 60000 && metres(last, fix) > 75) && elapsed < 300000) return;
    S.lastRecorded = fix;
    const sh = onRoadShipment();
    await tx("locations", "readwrite", (s) =>
      s.add({
        client_id: uuid(), shipment_id: sh ? sh.id : "", lat: fix.lat, lng: fix.lng,
        accuracy: fix.acc, speed: p.coords.speed != null ? p.coords.speed * 3.6 : null,
        heading: p.coords.heading, battery: S.battery, recorded_at: new Date(p.timestamp || fix.time).toISOString(),
      }),
    );
    S.pendingLocations++;
  }

  // ---------------------------------------------------------------------------
  // Router and screens
  // ---------------------------------------------------------------------------
  function go(hash) {
    if (location.hash !== hash) location.hash = hash;
  }
  let formOpen = false; // screens with forms are not redrawn by background syncs
  function rerender() {
    if (!formOpen) render();
    else renderPill();
  }

  function render() {
    const app = root();
    formOpen = false;
    if (!S.config) return app.replaceChildren(h("div", { class: "boot" }, h("p", { text: t("networkError") })));
    if (!S.token) return app.replaceChildren(screenLogin());
    if (S.driver && S.driver.must_change_pin) return app.replaceChildren(screenPin(true));
    const parts = location.hash.replace(/^#\/?/, "").split("/");
    let view;
    if (parts[0] === "s" && parts[1]) {
      const sh = S.shipments.get(parts[1]);
      if (!sh) view = screenList();
      else if (parts[2] === "deliver") view = screenDeliver(effective(sh));
      else if (parts[2] === "report") view = screenReport(effective(sh), parts[3]);
      else if (parts[2] === "message") view = screenMessage(effective(sh));
      else view = screenDetail(effective(sh));
    } else if (parts[0] === "menu") view = screenMenu();
    else if (parts[0] === "pin") view = screenPin(false);
    else view = screenList();
    app.replaceChildren(view);
    renderPill();
    window.scrollTo(0, 0);
  }

  function header(title, backHref) {
    return h("header", { class: "bar" },
      backHref ? h("a", { class: "bar-btn", href: backHref, "aria-label": t("back"), text: "‹" }) : h("span", { class: "bar-logo", "aria-hidden": "true" }),
      h("div", { class: "bar-title" }, h("strong", { text: title }), S.driver ? h("span", { class: "bar-sub", text: S.driver.company }) : null),
      h("button", { class: "pill-sync", type: "button", "data-pill": true, onclick: () => sync() }),
      h("a", { class: "bar-btn", href: "#/menu", "aria-label": t("menu"), text: "☰" }),
    );
  }

  function renderPill() {
    const pill = document.querySelector("[data-pill]");
    if (!pill) return;
    const waiting = S.outbox.length;
    let cls = "ok", txt = t("allSent");
    if (S.syncing) (cls = "busy"), (txt = t("syncing"));
    else if (!S.online || !navigator.onLine) (cls = "off"), (txt = waiting ? t("waiting", { n: waiting }) : t("offline"));
    else if (waiting) (cls = "wait"), (txt = t("waiting", { n: waiting }));
    pill.className = "pill-sync " + cls;
    pill.textContent = txt;
    pill.title = S.lastSync ? t("lastSync", { t: ago(S.lastSync) }) : t("neverSynced");
  }

  // ---- sign in ------------------------------------------------------------
  function field(label, input, error) {
    return h("label", { class: "field" }, h("span", { text: label }), input, error ? h("span", { class: "err", text: error }) : null);
  }

  function screenLogin(mode = "login", state = {}) {
    formOpen = true;
    const wrap = h("main", { class: "auth" });
    const err = h("p", { class: "err", role: "alert" });
    const draw = () => wrap.replaceChildren(...content().filter(Boolean));
    const busy = (form, on) => form.querySelectorAll("button").forEach((b) => (b.disabled = on));
    const fail = (e) => (err.textContent = e instanceof ApiError ? e.message : t("networkError"));

    const langSwitch = () =>
      h("div", { class: "lang" },
        ["en", "am"].map((l) => h("button", { type: "button", class: S.lang === l ? "on" : "", onclick: async () => { S.lang = l; await kvSet("lang", l); draw(); } }, l === "en" ? "English" : "አማርኛ")));

    function content() {
      err.textContent = "";
      if (mode === "reset") {
        const phone = h("input", { name: "phone", type: "tel", inputmode: "tel", autocomplete: "tel", required: true, value: state.phone || "" });
        const form = h("form", { class: "stack", onsubmit: async (e) => {
          e.preventDefault();
          busy(form, true);
          try {
            await api("/driver/pin-reset/request", { method: "POST", auth: false, body: { phone: phone.value } });
            mode = "confirm";
            state.phone = phone.value;
            draw();
            toast(t("codeSent"));
          } catch (x) { fail(x); } finally { busy(form, false); }
        } }, field(t("phone"), phone), err, h("button", { class: "btn primary big", type: "submit", text: t("sendCode") }));
        return [h("h1", { text: t("forgotPin") }), form, h("button", { class: "btn link", type: "button", text: t("back"), onclick: () => { mode = "login"; draw(); } })];
      }
      if (mode === "confirm") {
        const code = h("input", { inputmode: "numeric", autocomplete: "one-time-code", maxlength: "6", required: true, class: "code" });
        const pin = h("input", { type: "password", inputmode: "numeric", maxlength: "6", required: true });
        const pin2 = h("input", { type: "password", inputmode: "numeric", maxlength: "6", required: true });
        const form = h("form", { class: "stack", onsubmit: async (e) => {
          e.preventDefault();
          if (pin.value !== pin2.value) return (err.textContent = t("pinsDiffer"));
          busy(form, true);
          try {
            const res = await api("/driver/pin-reset/confirm", { method: "POST", auth: false, body: { phone: state.phone, code: code.value, pin: pin.value, device_id: S.deviceId } });
            await signedIn(res);
          } catch (x) { fail(x); } finally { busy(form, false); }
        } }, field(t("code"), code), field(t("newPin"), pin), field(t("newPinAgain"), pin2), err, h("button", { class: "btn primary big", type: "submit", text: t("setPin") }));
        return [h("h1", { text: t("forgotPin") }), form, h("button", { class: "btn link", type: "button", text: t("back"), onclick: () => { mode = "reset"; draw(); } })];
      }
      const phone = h("input", { name: "phone", type: "tel", inputmode: "tel", autocomplete: "tel", required: true, value: state.phone || "", placeholder: "09…" });
      const pin = h("input", { name: "pin", type: "password", inputmode: "numeric", autocomplete: "current-password", maxlength: "6", required: true });
      const form = h("form", { class: "stack", onsubmit: async (e) => {
        e.preventDefault();
        busy(form, true);
        try {
          const res = await api("/driver/login", { method: "POST", auth: false, body: { phone: phone.value, pin: pin.value, device_id: S.deviceId } });
          await signedIn(res);
        } catch (x) { fail(x); } finally { busy(form, false); }
      } }, field(t("phone"), phone), field(t("pin"), pin), err, h("button", { class: "btn primary big", type: "submit", text: t("signIn") }));
      return [
        h("div", { class: "brand" }, h("span", { class: "bar-logo big", "aria-hidden": "true" }), h("strong", { text: (S.config && S.config.app_name) || "Menged" })),
        langSwitch(),
        h("p", { class: "muted", text: t("signInHelp") }),
        form,
        h("button", { class: "btn link", type: "button", text: t("forgotPin"), onclick: () => { mode = "reset"; state.phone = phone.value; draw(); } }),
        S.config && S.config.sms_number ? h("p", { class: "muted small", text: t("smsHelpGeneric", { num: S.config.sms_number }) }) : null,
      ];
    }
    draw();
    return wrap;
  }

  async function signedIn(res) {
    const prev = await kvGet("driver");
    if (prev && prev.id !== res.driver.id) {
      // A different driver on this phone: nothing of the last one carries over.
      await Promise.all([clear("shipments"), clear("outbox"), clear("locations")]);
      S.shipments.clear();
      S.outbox = [];
      S.messages = [];
      S.cursor = null;
      await kvSet("cursor", null);
      await kvSet("messages", []);
    }
    S.token = res.token;
    S.driver = { ...res.driver, must_change_pin: res.must_change_pin };
    await kvSet("token", S.token);
    await kvSet("driver", S.driver);
    go("#/");
    render();
    sync();
  }

  function screenPin(forced) {
    formOpen = true;
    const cur = h("input", { type: "password", inputmode: "numeric", maxlength: "6", required: true, autocomplete: "current-password" });
    const pin = h("input", { type: "password", inputmode: "numeric", maxlength: "6", required: true, autocomplete: "new-password" });
    const pin2 = h("input", { type: "password", inputmode: "numeric", maxlength: "6", required: true, autocomplete: "new-password" });
    const err = h("p", { class: "err", role: "alert" });
    const form = h("form", { class: "stack", onsubmit: async (e) => {
      e.preventDefault();
      err.textContent = "";
      if (pin.value !== pin2.value) return (err.textContent = t("pinsDiffer"));
      form.querySelector("button").disabled = true;
      try {
        await api("/driver/pin", { method: "POST", body: { current_pin: cur.value, new_pin: pin.value } });
        S.driver.must_change_pin = false;
        await kvSet("driver", S.driver);
        toast(t("pinChanged"));
        go("#/");
        render();
      } catch (x) {
        if (x instanceof ApiError && x.status === 401) return signedOut();
        err.textContent = x instanceof ApiError ? x.message : t("networkError");
      } finally {
        form.querySelector("button").disabled = false;
      }
    } }, field(t("currentPin"), cur), field(t("newPin"), pin), field(t("newPinAgain"), pin2), err,
      h("button", { class: "btn primary big", type: "submit", text: t("setPin") }));
    return h("div", {},
      forced ? h("header", { class: "bar" }, h("span", { class: "bar-logo" }), h("div", { class: "bar-title" }, h("strong", { text: t("choosePin") }))) : header(t("changePin"), "#/menu"),
      h("main", { class: "page" }, forced ? h("p", { class: "muted", text: t("choosePinHelp") }) : null, form));
  }

  // ---- shipment list ----------------------------------------------------------
  function card(sh) {
    return h("a", { class: "ship " + (sh.priority !== "normal" ? "prio-" + sh.priority : ""), href: "#/s/" + sh.id },
      h("div", { class: "ship-top" },
        h("span", { class: "ref", text: "#" + sh.ref }),
        h("span", { class: "st st-" + sh.status, text: statusLabel(sh.status) }),
        sh.pending ? h("span", { class: "pending-dot", title: t("pendingSend") }) : null),
      h("div", { class: "ship-route" }, h("strong", { text: sh.origin_city || sh.origin_address }), " → ", h("strong", { text: sh.destination_city || sh.destination_address })),
      h("div", { class: "muted small" }, sh.customer_name, sh.eta && ACTIVE.includes(sh.status) ? " · " + t("due") + " " + fmtTime(sh.eta) : "",
        sh.delivered_at && sh.status === "delivered" ? " · " + t("deliveredAt") + " " + fmtTime(sh.delivered_at) : ""),
      sh.priority !== "normal" ? h("div", { class: "prio", text: t("priority_" + sh.priority) }) : null,
    );
  }

  function screenList() {
    const list = [...S.shipments.values()].map(effective);
    const rank = { urgent: 0, high: 1, normal: 2 };
    const active = list.filter((s) => ACTIVE.includes(s.status))
      .sort((a, b) => (rank[a.priority] ?? 2) - (rank[b.priority] ?? 2) || (a.eta || "~").localeCompare(b.eta || "~"));
    const done = list.filter((s) => !ACTIVE.includes(s.status)).sort((a, b) => (b.updated_at || "").localeCompare(a.updated_at || ""));
    const unseen = S.messages.filter((m) => !S.seen.has(m.id));
    return h("div", {},
      header(t("myShipments")),
      h("main", { class: "page" },
        S.notices.map((n) => h("div", { class: "notice" }, h("span", {}, h("strong", { text: t("rejected") + ": " }), n.text),
          h("button", { type: "button", class: "btn link", text: t("dismiss"), onclick: () => { S.notices = S.notices.filter((x) => x !== n); render(); } }))),
        unseen.length ? h("section", { class: "msgs" }, h("h2", { text: t("messages") }),
          unseen.slice(0, 3).map((m) => h("div", { class: "msg" }, h("p", { text: m.text }), h("p", { class: "muted small", text: (m.tracking_number ? "#" + m.tracking_number.slice(-4) + " · " : "") + m.from + " · " + fmtTime(m.at) }))),
          h("button", { class: "btn link", type: "button", text: t("dismiss"), onclick: async () => { unseen.forEach((m) => S.seen.add(m.id)); await kvSet("seen", [...S.seen]); render(); } })) : null,
        h("h2", { class: "section", text: t("active") + " (" + active.length + ")" }),
        active.length ? active.map(card) : h("div", { class: "empty" }, h("p", { text: t("none") }), h("p", { class: "muted small", text: t("noneHelp") })),
        done.length ? h("details", { class: "done" }, h("summary", { text: t("done") + " (" + done.length + ")" }), done.map(card)) : null,
        S.installPrompt ? h("div", { class: "install" }, h("p", {}, h("strong", { text: t("install") }), h("br"), h("span", { class: "muted small", text: t("installHelp") })),
          h("button", { class: "btn", type: "button", text: t("install"), onclick: async () => { S.installPrompt.prompt(); S.installPrompt = null; render(); } })) : null,
      ));
  }

  // ---- one shipment ------------------------------------------------------------
  function directions(sh) {
    if (sh.destination_lat != null) return `https://www.google.com/maps/dir/?api=1&destination=${sh.destination_lat},${sh.destination_lng}`;
    return "https://www.google.com/maps/search/?api=1&query=" + encodeURIComponent(sh.destination_address + ", " + sh.destination_city + ", Ethiopia");
  }

  async function setStatus(sh, status, reason = "", note = "") {
    const fix = fixForEvent();
    await enqueue("status", { shipment_id: sh.id, status, reason, note, lat: fix.lat, lng: fix.lng, recorded_at: new Date().toISOString() });
    if (navigator.vibrate) navigator.vibrate(40);
    toast(t("statusSaved"));
    updateTracking();
    go("#/s/" + sh.id);
    render();
  }

  function screenDetail(sh) {
    const moves = (S.config.transitions[sh.status] || []).filter((m) => m !== "delivered");
    const canDeliver = S.config.pod_from.includes(sh.status);
    const actions = [];
    if (moves.includes("in_transit")) actions.push(h("button", { class: "btn primary big", type: "button", text: sh.status === "assigned" ? t("startTrip") : t("backOnRoad"), onclick: () => setStatus(sh, "in_transit") }));
    if (moves.includes("arrived")) actions.push(h("button", { class: "btn big st-btn-arrived", type: "button", text: t("arrivedBtn"), onclick: () => setStatus(sh, "arrived") }));
    if (canDeliver) actions.push(h("a", { class: "btn big " + (sh.status === "arrived" ? "primary" : "good"), href: "#/s/" + sh.id + "/deliver", text: t("deliverBtn") }));
    if (moves.includes("delayed")) actions.push(h("a", { class: "btn", href: "#/s/" + sh.id + "/report/delayed", text: t("delayBtn") }));
    if (moves.includes("issue")) actions.push(h("a", { class: "btn warn", href: "#/s/" + sh.id + "/report/issue", text: t("problemBtn") }));
    const msgs = S.messages.filter((m) => m.shipment_id === sh.id);
    const smsNum = S.driver && S.driver.sms_number;
    return h("div", {},
      header("#" + sh.ref, "#/"),
      h("main", { class: "page" },
        h("div", { class: "detail-head" },
          h("span", { class: "st st-" + sh.status + " big", text: statusLabel(sh.status) + (sh.status_reason ? " · " + reasonLabel(sh.status_reason) : "") }),
          sh.priority !== "normal" ? h("span", { class: "prio", text: t("priority_" + sh.priority) }) : null,
          h("p", { class: "mono small muted", text: sh.tracking_number })),
        sh.pending ? h("p", { class: "pending", text: t("pendingSend") }) : null,
        actions.length ? h("div", { class: "actions" }, actions) : null,
        h("section", { class: "card" },
          h("div", { class: "leg" }, h("span", { class: "leg-label", text: t("from") }), h("strong", { text: sh.origin_city }), h("span", { text: sh.origin_address })),
          h("div", { class: "leg" }, h("span", { class: "leg-label", text: t("to") }), h("strong", { text: sh.destination_city }), h("span", { text: sh.destination_address }),
            ACTIVE.includes(sh.status) ? h("a", { class: "btn small", href: directions(sh), target: "_blank", rel: "noopener", text: "➜ " + t("navigate") }) : null),
          sh.eta && ACTIVE.includes(sh.status) ? h("p", {}, h("span", { class: "muted", text: t("due") + ": " }), fmtTime(sh.eta)) : null),
        h("section", { class: "card" },
          h("p", { class: "muted small", text: t("customer") }),
          h("p", {}, h("strong", { text: sh.customer_name })),
          h("a", { class: "btn big call", href: "tel:" + sh.customer_phone, text: "📞 " + t("call") + " " + sh.customer_phone })),
        sh.cargo || sh.weight_kg || sh.vehicle ? h("section", { class: "card" },
          sh.cargo ? h("p", {}, h("span", { class: "muted", text: t("cargo") + ": " }), sh.cargo, sh.weight_kg ? " · " + sh.weight_kg + " " + t("kg") : "") : null,
          sh.vehicle ? h("p", {}, h("span", { class: "muted", text: t("vehicle") + ": " }), sh.vehicle) : null) : null,
        sh.instructions ? h("section", { class: "card instructions" }, h("p", { class: "muted small", text: t("instructions") }), h("p", { class: "pre", text: sh.instructions })) : null,
        msgs.length ? h("section", { class: "card" }, h("p", { class: "muted small", text: t("messages") }),
          msgs.map((m) => h("div", { class: "msg" }, h("p", { text: m.text }), h("p", { class: "muted small", text: m.from + " · " + fmtTime(m.at) })))) : null,
        ACTIVE.includes(sh.status) ? h("a", { class: "btn block", href: "#/s/" + sh.id + "/message", text: "✉ " + t("messageBtn") }) : null,
        smsNum && ACTIVE.includes(sh.status) ? h("p", { class: "muted small sms-help", text: t("smsHelp", { ref: "ET " + sh.ref, num: smsNum }) }) : null,
      ));
  }

  // ---- delay / problem -------------------------------------------------------------
  function screenReport(sh, status) {
    formOpen = true;
    const reasons = status === "delayed" ? S.config.delay_reasons : S.config.issue_reasons;
    let chosen = "";
    const err = h("p", { class: "err", role: "alert" });
    const note = h("textarea", { rows: "3", maxlength: "500" });
    const opts = h("div", { class: "choices" }, reasons.map((r) =>
      h("label", { class: "choice" }, h("input", { type: "radio", name: "reason", value: r.code, onchange: () => (chosen = r.code) }), h("span", { text: reasonLabel(r.code) }))));
    const form = h("form", { class: "stack", onsubmit: async (e) => {
      e.preventDefault();
      if (!chosen) return (err.textContent = t("reason"));
      await setStatus(sh, status, chosen, note.value.trim());
    } }, h("p", { class: "muted", text: t("reason") }), opts, field(t("note"), note), err,
      h("button", { class: "btn primary big", type: "submit", text: t("save") }),
      h("a", { class: "btn link", href: "#/s/" + sh.id, text: t("cancel") }));
    return h("div", {}, header(status === "delayed" ? t("delayBtn") : t("problemBtn"), "#/s/" + sh.id), h("main", { class: "page" }, form));
  }

  function screenMessage(sh) {
    formOpen = true;
    const text = h("textarea", { rows: "4", maxlength: "1000", required: true, placeholder: t("messagePlaceholder") });
    const form = h("form", { class: "stack", onsubmit: async (e) => {
      e.preventDefault();
      if (!text.value.trim()) return;
      await enqueue("message", { shipment_id: sh.id, text: text.value.trim(), recorded_at: new Date().toISOString() });
      toast(t("messageSaved"));
      go("#/s/" + sh.id);
      render();
    } }, text, h("button", { class: "btn primary big", type: "submit", text: t("send") }), h("a", { class: "btn link", href: "#/s/" + sh.id, text: t("cancel") }));
    return h("div", {}, header(t("messageBtn"), "#/s/" + sh.id), h("main", { class: "page" }, form));
  }

  // ---- proof of delivery --------------------------------------------------------------
  function signaturePad() {
    const canvas = h("canvas", { class: "sig", "aria-label": t("signHere") });
    const ctx = canvas.getContext("2d");
    let strokes = 0, drawing = false, last = null;
    function size() {
      const r = canvas.getBoundingClientRect();
      if (!r.width) return;
      const ratio = Math.max(1, window.devicePixelRatio || 1);
      canvas.width = Math.round(r.width * ratio);
      canvas.height = Math.round(r.height * ratio);
      ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
      ctx.fillStyle = "#fff";
      ctx.fillRect(0, 0, r.width, r.height);
      ctx.lineWidth = 2.4;
      ctx.lineCap = ctx.lineJoin = "round";
      ctx.strokeStyle = "#111";
      strokes = 0;
    }
    const pos = (e) => {
      const r = canvas.getBoundingClientRect();
      return { x: e.clientX - r.left, y: e.clientY - r.top };
    };
    canvas.addEventListener("pointerdown", (e) => {
      e.preventDefault();
      canvas.setPointerCapture(e.pointerId);
      drawing = true;
      last = pos(e);
      ctx.beginPath();
      ctx.arc(last.x, last.y, 1.1, 0, Math.PI * 2);
      ctx.fillStyle = "#111";
      ctx.fill();
      strokes++;
    });
    canvas.addEventListener("pointermove", (e) => {
      if (!drawing) return;
      const p = pos(e);
      ctx.beginPath();
      ctx.moveTo(last.x, last.y);
      ctx.lineTo(p.x, p.y);
      ctx.stroke();
      last = p;
    });
    const end = () => (drawing = false);
    canvas.addEventListener("pointerup", end);
    canvas.addEventListener("pointercancel", end);
    requestAnimationFrame(size);
    return { canvas, clear: size, empty: () => strokes === 0, data: () => canvas.toDataURL("image/png") };
  }

  function compress(file) {
    return new Promise((resolve, reject) => {
      const img = new Image();
      const url = URL.createObjectURL(file);
      img.onload = () => {
        // Fit inside 800×600 (600×800 for portrait shots): plenty to read a
        // label or see damage, small enough to send over 2G.
        const landscape = img.width >= img.height;
        const ratio = Math.min(1, (landscape ? 800 : 600) / img.width, (landscape ? 600 : 800) / img.height);
        const cw = Math.round(img.width * ratio), ch = Math.round(img.height * ratio);
        const c = document.createElement("canvas");
        c.width = cw;
        c.height = ch;
        c.getContext("2d").drawImage(img, 0, 0, cw, ch);
        URL.revokeObjectURL(url);
        resolve(c.toDataURL("image/jpeg", 0.72));
      };
      img.onerror = () => {
        URL.revokeObjectURL(url);
        reject(new Error("unreadable image"));
      };
      img.src = url;
    });
  }

  function screenDeliver(sh) {
    formOpen = true;
    const photos = [];
    const err = h("p", { class: "err", role: "alert" });
    const name = h("input", { required: true, autocomplete: "off", maxlength: "120" });
    const phone = h("input", { type: "tel", inputmode: "tel", maxlength: "20" });
    const notes = h("textarea", { rows: "2", maxlength: "500" });
    const pad = signaturePad();
    const thumbs = h("div", { class: "thumbs" });
    const fileInput = h("input", { type: "file", accept: "image/*", capture: "environment", class: "visually-hidden", onchange: async () => {
      const f = fileInput.files[0];
      fileInput.value = "";
      if (!f || photos.length >= MAX_PHOTOS) return;
      try {
        photos.push(await compress(f));
        drawThumbs();
      } catch {
        toast("Photo could not be read.");
      }
    } });
    const addBtn = h("button", { type: "button", class: "btn", onclick: () => fileInput.click() }, "📷 " + t("addPhoto"));
    function drawThumbs() {
      thumbs.replaceChildren(...photos.map((p, i) => h("figure", {}, h("img", { src: p, alt: "" }),
        h("button", { type: "button", class: "btn link small", text: t("remove"), onclick: () => { photos.splice(i, 1); drawThumbs(); } }))));
      addBtn.hidden = photos.length >= MAX_PHOTOS;
    }
    const submit = h("button", { class: "btn primary big", type: "submit", text: t("confirmDelivery") });
    const form = h("form", { class: "stack", onsubmit: async (e) => {
      e.preventDefault();
      err.textContent = "";
      if (!name.value.trim()) return (err.textContent = t("needName"));
      if (pad.empty()) return (err.textContent = t("needSignature"));
      submit.disabled = true;
      submit.textContent = t("locating");
      const fix = (await freshFix(10000)) || null;
      await enqueue("pod", {
        shipment_id: sh.id, recipient_name: name.value.trim(), recipient_phone: phone.value.trim(),
        signature: pad.data(), photos: photos.slice(), notes: notes.value.trim(),
        lat: fix ? fix.lat : null, lng: fix ? fix.lng : null, recorded_at: new Date().toISOString(),
      });
      if (navigator.vibrate) navigator.vibrate([60, 40, 60]);
      toast(fix ? t("delivered") : t("noFix"));
      updateTracking();
      go("#/s/" + sh.id);
      render();
    } },
      field(t("recipient"), name), field(t("recipientPhone"), phone),
      h("div", { class: "field" }, h("span", { text: t("signature") }), h("div", { class: "sig-wrap" }, pad.canvas, h("span", { class: "sig-hint", text: t("signHere") })),
        h("button", { type: "button", class: "btn link small", text: t("clear"), onclick: () => pad.clear() })),
      h("div", { class: "field" }, h("span", { text: t("photos") }), thumbs, addBtn, fileInput),
      field(t("deliveryNotes"), notes), err, submit,
      h("a", { class: "btn link", href: "#/s/" + sh.id, text: t("cancel") }));
    return h("div", {}, header(t("deliverBtn") + " · #" + sh.ref, "#/s/" + sh.id),
      h("main", { class: "page" }, h("p", { class: "muted small" }, sh.customer_name + " · " + sh.destination_address), form));
  }

  // ---- menu -------------------------------------------------------------------
  function screenMenu() {
    const pending = S.outbox.length;
    let confirmOut = false;
    const outBox = h("div", {});
    function drawOut() {
      outBox.replaceChildren(confirmOut
        ? h("div", { class: "notice" }, h("p", { text: t("signOutPending", { n: pending }) }),
            h("button", { class: "btn warn", type: "button", text: t("signOutAnyway"), onclick: signOut }),
            h("button", { class: "btn link", type: "button", text: t("cancel"), onclick: () => { confirmOut = false; drawOut(); } }))
        : h("button", { class: "btn block", type: "button", text: t("signOut"), onclick: () => (pending ? ((confirmOut = true), drawOut()) : signOut()) }));
    }
    drawOut();
    const d = S.driver || {};
    return h("div", {},
      header(t("menu"), "#/"),
      h("main", { class: "page" },
        h("section", { class: "card" }, h("p", {}, h("strong", { text: d.name })), h("p", { class: "muted", text: d.phone }), h("p", { class: "muted", text: d.company + (d.vehicle ? " · " + d.vehicle : "") })),
        h("section", { class: "card" },
          h("p", { class: "muted small", text: t("language") }),
          h("div", { class: "lang" }, ["en", "am"].map((l) => h("button", { type: "button", class: S.lang === l ? "on" : "", onclick: async () => { S.lang = l; await kvSet("lang", l); document.documentElement.lang = l; render(); } }, l === "en" ? "English" : "አማርኛ")))),
        h("section", { class: "card" },
          h("label", { class: "toggle" }, h("input", { type: "checkbox", checked: S.tracking, onchange: async (e) => { S.tracking = e.target.checked; await kvSet("tracking", S.tracking); updateTracking(); scheduleSync(500); } }), h("span", { text: t("location") })),
          h("p", { class: "muted small", text: t("locationHelp") })),
        h("section", { class: "card" },
          h("button", { class: "btn block", type: "button", text: t("syncNow"), onclick: () => sync() }),
          h("p", { class: "muted small", text: (S.lastSync ? t("lastSync", { t: ago(S.lastSync) }) : t("neverSynced")) + (pending ? " · " + t("waiting", { n: pending }) : "") })),
        h("a", { class: "btn block", href: "#/pin", text: t("changePin") }),
        outBox,
        d.sms_number ? h("p", { class: "muted small", text: t("smsHelpGeneric", { num: d.sms_number }) }) : null,
        h("p", { class: "muted small center", text: t("version") + " " + VERSION + (S.config ? " · " + S.config.version : "") }),
      ));
  }

  async function signOut() {
    try {
      if (navigator.onLine) await api("/driver/logout", { method: "POST" });
    } catch { /* signing out locally is what matters */ }
    S.token = null;
    await kvSet("token", null);
    await Promise.all([clear("outbox"), clear("locations")]);
    S.outbox = [];
    stopTracking();
    go("#/");
    render();
  }

  // ---------------------------------------------------------------------------
  // Start
  // ---------------------------------------------------------------------------
  async function loadConfig() {
    try {
      const cfg = await api("/driver/config", { auth: false });
      await kvSet("config", cfg);
      return cfg;
    } catch {
      return kvGet("config");
    }
  }

  async function boot() {
    if ("serviceWorker" in navigator) {
      navigator.serviceWorker.register("/driver/sw.js", { scope: "/driver/" }).catch(() => {});
    }
    S.deviceId = await kvGet("deviceId");
    if (!S.deviceId) {
      S.deviceId = uuid();
      await kvSet("deviceId", S.deviceId);
    }
    const savedLang = await kvGet("lang");
    S.lang = savedLang || ((navigator.language || "").startsWith("am") ? "am" : "en");
    document.documentElement.lang = S.lang;
    S.token = await kvGet("token");
    S.driver = await kvGet("driver");
    S.cursor = await kvGet("cursor");
    S.lastSync = await kvGet("lastSync");
    S.messages = (await kvGet("messages")) || [];
    S.seen = new Set((await kvGet("seen")) || []);
    const tracking = await kvGet("tracking");
    S.tracking = tracking !== false;
    for (const sh of await all("shipments")) S.shipments.set(sh.id, sh);
    S.outbox = await all("outbox");
    S.pendingLocations = await tx("locations", "readonly", (s) => req2p(s.count()));
    S.config = await loadConfig();

    if (navigator.getBattery) {
      navigator.getBattery().then((b) => {
        const upd = () => (S.battery = Math.round(b.level * 100));
        upd();
        b.addEventListener("levelchange", upd);
      }).catch(() => {});
    }
    window.addEventListener("hashchange", render);
    window.addEventListener("online", () => { S.online = true; renderPill(); scheduleSync(500); });
    window.addEventListener("offline", () => { S.online = false; renderPill(); });
    document.addEventListener("visibilitychange", () => { if (!document.hidden) scheduleSync(300); });
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
})();
