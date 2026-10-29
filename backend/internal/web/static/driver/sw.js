// Service worker for the driver app. The app shell is served network-first so
// a deploy reaches drivers whenever they have signal, and from the cache when
// they don't. API calls are never cached: the app keeps its own data in
// IndexedDB and queues changes there.
const CACHE = "menged-driver-v1";
const SHELL = [
  "/driver/",
  "/driver/app.js",
  "/driver/app.css",
  "/driver/manifest.webmanifest",
  "/driver/icon.svg",
  "/driver/icon-maskable.svg",
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
  const key = req.mode === "navigate" ? "/driver/" : req;
  event.respondWith(
    fetch(req)
      .then((res) => {
        if (res.ok) {
          const copy = res.clone();
          caches.open(CACHE).then((c) => c.put(key, copy));
        }
        return res;
      })
      .catch(() => caches.match(key).then((hit) => hit || caches.match("/driver/"))),
  );
});
