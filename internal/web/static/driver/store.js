// IndexedDB: the shipments, the outbox, the positions not yet sent, and
// small values by key (kv). What the driver records is written here before
// anything else happens to it.

import { S } from "./state.js";

const DB_NAME = "kiflet-driver";
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
export async function tx(store, mode, fn) {
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
export const req2p = (r) => new Promise((res, rej) => ((r.onsuccess = () => res(r.result)), (r.onerror = () => rej(r.error))));
export const kvGet = (k) => tx("kv", "readonly", (s) => req2p(s.get(k)));
export const kvSet = (k, v) => tx("kv", "readwrite", (s) => s.put(v, k));
export const all = (store) => tx(store, "readonly", (s) => req2p(s.getAll()));
export const put = (store, v) => tx(store, "readwrite", (s) => s.put(v));
export const del = (store, k) => tx(store, "readwrite", (s) => s.delete(k));
export const clear = (store) => tx(store, "readwrite", (s) => s.clear());
// Delivery forms being filled in, kept under draft:<shipment id>.
export const dropDrafts = () => tx("kv", "readwrite", (s) => req2p(s.getAllKeys()).then((keys) =>
  keys.filter((k) => String(k).startsWith("draft:")).forEach((k) => s.delete(k))));

// The browser is asked to keep this app's data when space runs low, so
// unsent deliveries survive (AC-014.7), and the office sees how much the
// app holds (AC-014.6).
export async function storageEstimate() {
  if (!navigator.storage || !navigator.storage.estimate) return null;
  try {
    const e = await navigator.storage.estimate();
    return { usage: e.usage || 0, quota: e.quota || 0 };
  } catch {
    return null;
  }
}
// Short of space: under 50 MB left, or more than 80% of the allowance used.
export function storageLow() {
  const st = S.storage;
  return !!(st && st.quota && (st.quota - st.usage < 50 * 1048576 || st.usage / st.quota > 0.8));
}
export async function protectStorage() {
  if (!navigator.storage || !navigator.storage.persist) return;
  try {
    S.persisted = (await navigator.storage.persisted()) || (await navigator.storage.persist());
  } catch {
    S.persisted = false;
  }
}
