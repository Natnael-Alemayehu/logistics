// Service worker for the driver app. It keeps the app's files in the cache,
// one version whole, and serves them from there: the app starts at once,
// with or without signal, and never runs the files of two versions together.
//
// The server describes the version it serves in /driver/build.json: every
// file with its SHA-256 (internal/web/driver.go). When the app starts, and
// every hour while it runs, the worker compares it with the version it has
// (check). A new version is downloaded beside the one in use, and kept only
// once every file matches its sum. An update cut short by a weak signal, or
// caught in the middle of a deploy, is thrown away and tried again at the
// next check, and the version on the phone runs on meanwhile. The version
// downloaded takes over the next time the app starts, or sooner when the
// app says it is safe (app.js, watchUpdates).
//
// The worker's own code is not part of a version. The browser replaces it
// when sw.js changes, and whichever worker runs serves the version in
// STATE, so a new worker must read STATE as this one writes it.
//
// API calls are never cached: the app keeps its own data in IndexedDB and
// queues changes there.
const PREFIX = "kiflet-driver-";
// Which version is in use ("current") and which is downloaded and waits to
// take over ("ready"). A version's files are in the cache PREFIX + version.
const STATE = PREFIX + "state";

self.addEventListener("install", (event) => {
  // The worker serves the version in STATE, whichever worker it is, so a new
  // one can take over at once. Without signal it installs all the same and
  // downloads the app at the next check.
  event.waitUntil(check().catch(() => {}).then(() => self.skipWaiting()));
});

self.addEventListener("activate", (event) => {
  event.waitUntil(locked(async () => tidy(await state())).then(() => self.clients.claim()));
});

self.addEventListener("message", (event) => {
  const reply = (message) => event.source && event.source.postMessage(message);
  if (event.data === "check") {
    event.waitUntil(check().then((ready) => ready && reply("ready"), () => {}));
  } else if (event.data === "take-over") {
    event.waitUntil(locked(takeOver).then((done) => done && reply("switched")));
  }
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  const url = new URL(req.url);
  if (req.method !== "GET" || url.origin !== location.origin || !url.pathname.startsWith("/driver/")) return;
  // Navigations within the app all load the shell.
  const navigate = req.mode === "navigate";
  event.respondWith(serve(req, navigate ? "/driver/" : url.pathname, navigate));
});

// serve answers from the version in use. A file the cache lost (the browser
// may clear it when the phone is short of space) comes from the network.
async function serve(req, key, navigate) {
  // The app starting is when a downloaded version takes over.
  if (navigate) await locked(takeOver).catch(() => {});
  const { current } = await state();
  const hit = current && (await (await caches.open(PREFIX + current)).match(key));
  return hit || fetch(req);
}

// check downloads the version the server serves if the phone hasn't got it,
// and reports the version waiting to take over, if there is one. Checks
// asked for while one runs share it.
let checking = null;
function check() {
  checking = checking || update().finally(() => (checking = null));
  return checking;
}

async function update() {
  const res = await fetch("/driver/build.json", { cache: "no-store" });
  if (!res.ok) throw new Error(`build.json: ${res.status}`);
  const build = await res.json();
  const before = await state();
  if (build.version !== before.current && build.version !== before.ready) await download(build);
  return locked(async () => {
    let { current, ready } = await state();
    if (build.version === current) {
      ready = null; // the server went back to the version in use
    } else if (build.version !== ready) {
      if (current && (await caches.has(PREFIX + current))) {
        ready = build.version;
      } else {
        // Nothing in use to interrupt: the first version, or one whose files
        // the browser cleared.
        current = build.version;
        ready = null;
      }
    }
    await save({ current, ready });
    return ready;
  });
}

// download fetches every file of a version and checks it against its sum,
// then stores them all. If one fails, nothing is kept.
async function download(build) {
  const name = PREFIX + build.version;
  try {
    const files = await Promise.all(
      Object.entries(build.files).map(async ([path, sum]) => {
        const res = await fetch(path, { cache: "no-store" });
        if (!res.ok) throw new Error(`${path}: ${res.status}`);
        const body = await res.arrayBuffer();
        const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", body));
        const hex = Array.from(digest, (b) => b.toString(16).padStart(2, "0")).join("");
        if (hex !== sum) throw new Error(`${path} isn't the file of version ${build.version}`);
        return [path, new Response(body, { headers: res.headers })];
      }),
    );
    const cache = await caches.open(name);
    await Promise.all(files.map(([path, res]) => cache.put(path, res)));
  } catch (err) {
    await caches.delete(name);
    throw err;
  }
}

// takeOver puts the downloaded version in use, and reports whether there was
// one.
async function takeOver() {
  const { ready } = await state();
  if (!ready) return false;
  await save({ current: ready, ready: null });
  return true;
}

async function state() {
  const cache = await caches.open(STATE);
  const read = async (key) => {
    const res = await cache.match(key);
    return res ? res.text() : null;
  };
  return { current: await read("current"), ready: await read("ready") };
}

async function save(s) {
  const cache = await caches.open(STATE);
  for (const key of ["current", "ready"]) {
    if (s[key]) await cache.put(key, new Response(s[key]));
    else await cache.delete(key);
  }
  await tidy(s);
}

// tidy deletes the files of every version but the two in STATE, and those
// of earlier workers.
async function tidy({ current, ready }) {
  const keep = [STATE, PREFIX + current, PREFIX + ready];
  const names = await caches.keys();
  await Promise.all(names.filter((n) => n.startsWith("kiflet-driver") && !keep.includes(n)).map((n) => caches.delete(n)));
}

// locked runs the changes to STATE one at a time. Downloads run outside it,
// so an app starting never waits for one.
let queue = Promise.resolve();
function locked(fn) {
  const run = queue.then(fn);
  queue = run.catch(() => {});
  return run;
}
