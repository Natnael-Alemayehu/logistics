// What the app holds while it runs: the driver, the shipments, the outbox,
// how the sync is going, and the driver's settings. IndexedDB keeps what
// must outlive a restart (store.js). Values that more than one module
// changes live in S, since a module can't assign to another's variables.

export const VERSION = "1.2.0";
export const API = "/api/v1";
export const ON_ROAD = ["in_transit", "delayed", "arrived", "issue"];
export const ACTIVE = ["assigned", ...ON_ROAD];
export const MAX_PHOTOS = 3;
export const DEFAULT_SETTINGS = { language: "", battery_saver: false, vibrate: true };

export const S = {
  token: null, driver: null, config: null, deviceId: null, cursor: null, lang: "en",
  shipments: new Map(), outbox: [], messages: [], seen: new Set(), notices: [],
  tracking: true, lastFix: null, lastRecorded: null, battery: null, charging: false, pendingLocations: 0,
  online: navigator.onLine, weak: false, syncing: false, lastSync: null, failures: 0, installPrompt: null,
  // The driver's choices, kept on the server too. changed: what was
  // changed here since the server last heard; rev tells a change made
  // during a sync.
  settings: { ...DEFAULT_SETTINGS }, changed: {}, settingsRev: 0,
  keepAwake: false, storage: null, persisted: false, parked: false, anchor: null,
  // blocked: the company's account is suspended, so syncs are refused.
  blocked: false,
  // formOpen: a screen with a form is showing, which background syncs
  // don't redraw and updates don't interrupt.
  formOpen: false,
};

export function uuid() {
  if (crypto.randomUUID) return crypto.randomUUID();
  const b = crypto.getRandomValues(new Uint8Array(16));
  b[6] = (b[6] & 0x0f) | 0x40;
  b[8] = (b[8] & 0x3f) | 0x80;
  const h = [...b].map((x) => x.toString(16).padStart(2, "0")).join("");
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}
