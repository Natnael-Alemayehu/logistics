// GPS (FR-019). A position a minute while moving, one every five minutes
// when parked. Battery saver, chosen or on by itself below 20% when not
// charging, uses the coarse location and keeps one every 15 minutes. After
// 10 minutes parked the GPS goes off, and a check every 5 minutes (15 when
// saving) notices when the truck moves again.

import { ON_ROAD, S, uuid } from "./state.js";
import { tx } from "./store.js";
import { effective } from "./sync.js";

const MOVED_M = 75;
const PARKED_MS = 10 * 60000;
let watchId = null, watchSaving = null, parkTimer = null;
export function saving() {
  return !!(S.settings.battery_saver || (S.battery != null && S.battery < 20 && !S.charging));
}
function onRoadShipment() {
  const list = [...S.shipments.values()].map(effective);
  return list.find((s) => ON_ROAD.includes(s.status)) || list.find((s) => s.status === "assigned");
}
function wantTracking() {
  return !!(S.token && S.tracking && navigator.geolocation && onRoadShipment());
}
export function updateTracking() {
  if (!wantTracking()) return stopTracking();
  updateWakeLock();
  if (S.parked) return; // the parked checks hold the GPS until the truck moves
  const sv = saving();
  if (watchId != null && watchSaving === sv) return;
  if (watchId != null) navigator.geolocation.clearWatch(watchId);
  watchSaving = sv;
  watchId = navigator.geolocation.watchPosition(onFix, () => {}, sv
    ? { enableHighAccuracy: false, maximumAge: 300000, timeout: 120000 }
    : { enableHighAccuracy: true, maximumAge: 30000, timeout: 60000 });
}
export function stopTracking() {
  if (watchId != null) navigator.geolocation.clearWatch(watchId);
  watchId = null;
  clearTimeout(parkTimer);
  S.parked = false;
  updateWakeLock();
}
const checkEvery = () => (saving() ? 15 : 5) * 60000;
function park() {
  if (watchId != null) navigator.geolocation.clearWatch(watchId);
  watchId = null;
  S.parked = true;
  clearTimeout(parkTimer);
  parkTimer = setTimeout(parkedCheck, checkEvery());
}
// The driver doing something, or the truck moving, brings the GPS back.
export function unpark() {
  if (!S.parked) return;
  clearTimeout(parkTimer);
  S.parked = false;
  S.anchor = null;
  updateTracking();
}
function parkedCheck() {
  if (!wantTracking()) return stopTracking();
  const again = () => (parkTimer = setTimeout(parkedCheck, checkEvery()));
  navigator.geolocation.getCurrentPosition(async (p) => {
    const fix = fixOf(p);
    S.lastFix = fix;
    if (fix.acc <= maxAccuracy()) await record(p, fix);
    if (S.anchor && metres(S.anchor, fix) > Math.max(150, fix.acc)) return unpark();
    again();
  }, again, { enableHighAccuracy: !saving(), maximumAge: 60000, timeout: 60000 });
}
// Coarse positions are all battery saver gets on the open road.
const maxAccuracy = () => (saving() ? 3000 : 500);
const fixOf = (p) => ({ lat: p.coords.latitude, lng: p.coords.longitude, acc: p.coords.accuracy, time: Date.now() });

// Keep the screen on while charging, if the driver asked: the browser
// only tracks while the app is open.
let wakeLock = null, wakeLockBusy = false;
export async function updateWakeLock() {
  const want = S.keepAwake && S.charging && !document.hidden && wantTracking() && "wakeLock" in navigator;
  if (wakeLockBusy || want === !!wakeLock) return;
  wakeLockBusy = true;
  try {
    if (want) {
      wakeLock = await navigator.wakeLock.request("screen");
      wakeLock.addEventListener("release", () => (wakeLock = null));
    } else {
      const w = wakeLock;
      wakeLock = null;
      await w.release();
    }
  } catch {
    wakeLock = null;
  } finally {
    wakeLockBusy = false;
  }
}
export function metres(a, b) {
  const R = 6371000, rad = Math.PI / 180;
  const dLat = (b.lat - a.lat) * rad, dLng = (b.lng - a.lng) * rad;
  const x = Math.sin(dLat / 2) ** 2 + Math.cos(a.lat * rad) * Math.cos(b.lat * rad) * Math.sin(dLng / 2) ** 2;
  return 2 * R * Math.asin(Math.sqrt(x));
}
async function onFix(p) {
  const fix = fixOf(p);
  S.lastFix = fix;
  if (fix.acc > maxAccuracy()) return; // too vague to be useful
  // Parked: no further than 75 m from where the truck stopped, for 10 minutes.
  if (!S.anchor || metres(S.anchor, fix) > Math.max(MOVED_M, fix.acc)) S.anchor = fix;
  else if (fix.time - S.anchor.time > PARKED_MS) {
    await record(p, fix);
    return park();
  }
  const last = S.lastRecorded;
  const elapsed = last ? fix.time - last.time : Infinity;
  if (last) {
    if (saving() && elapsed < 15 * 60000) return;
    if (!saving() && !(elapsed >= 60000 && metres(last, fix) > MOVED_M) && elapsed < 300000) return;
  }
  await record(p, fix);
}
async function record(p, fix) {
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
