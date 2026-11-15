// Service worker for the driver app. The app shell is served network-first so
// a deploy reaches drivers whenever they have signal, and from the cache when
// they don't, or when the network takes more than a few seconds to answer: a
// weak signal mustn't keep the driver looking at a blank screen. API calls are
// never cached: the app keeps its own data in IndexedDB and queues changes
// there.
const CACHE = "kiflet-driver-v2";
const WAIT_MS = 3000;
const SHELL = [
  "/driver/",
  "/driver/app.js",
  "/driver/app.css",
  "/driver/manifest.webmanifest",
  "/driver/icon.svg",
  "/driver/icon-maskable.svg",
  "/driver/icon-192.png",
  "/driver/icon-512.png",
  "/driver/icon-maskable-192.png",
  "/driver/icon-maskable-512.png",
  "/driver/apple-touch-icon.png",
];

self.addEventListener("install", (event) => {
  event.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting()));
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  const url = new URL(req.url);
  if (req.method !== "GET" || url.origin !== location.origin) return;

  // Besides the shell, only the config endpoint is cached: it is small and
  // the app needs it to start offline.
  const cacheable = url.pathname.startsWith("/driver/") || url.pathname === "/api/v1/driver/config";
  if (!cacheable) return;

  // Navigations within the app all load the shell.
  const navigate = req.mode === "navigate";
  const key = navigate ? "/driver/" : req;
  const network = fetch(req).then((res) => {
    if (res.ok) {
      const copy = res.clone();
      return caches.open(CACHE).then((c) => c.put(key, copy)).then(() => res, () => res);
    }
    return res;
  });
  // The cache is brought up to date even when the answer came from it.
  event.waitUntil(network.catch(() => {}));
  event.respondWith(respond(network, key, navigate));
});

// respond answers from the network, or from the cache if the network fails
// or is slow. A slow answer that isn't cached is still waited for.
async function respond(network, key, navigate) {
  const cached = async () => (await caches.match(key)) || (navigate ? caches.match("/driver/") : undefined);
  let timer;
  const slow = new Promise((resolve) => (timer = setTimeout(resolve, WAIT_MS, "slow")));
  try {
    const first = await Promise.race([network, slow]);
    if (first !== "slow") return first;
    return (await cached()) || (await network);
  } catch (err) {
    const hit = await cached();
    if (hit) return hit;
    throw err;
  } finally {
    clearTimeout(timer);
  }
}
