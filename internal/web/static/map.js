// Maps: the fleet map on the overview, a shipment's route, a driver's last
// position, the public tracking map and the drop-off pin picker. Data comes
// from a JSON <script> next to each map; live positions arrive as
// "kiflet:live" events from app.js.
(function () {
  "use strict";
  if (!window.L) return;

  const ETHIOPIA = [9.0, 39.5];

  function text(tag, value, cls) {
    const n = document.createElement(tag);
    n.textContent = value;
    if (cls) n.className = cls;
    return n;
  }

  function popup(...lines) {
    const box = document.createElement("div");
    for (const l of lines) if (l) box.append(l);
    return box;
  }

  function link(href, label) {
    const a = text("a", label);
    a.href = href;
    return a;
  }

  function ago(iso) {
    if (!iso) return "never";
    const s = (Date.now() - new Date(iso).getTime()) / 1000;
    if (s < 60) return "just now";
    if (s < 3600) return Math.floor(s / 60) + " min ago";
    if (s < 86400) return Math.floor(s / 3600) + " h ago";
    return Math.floor(s / 86400) + " days ago";
  }

  function state(iso) {
    if (!iso) return "never";
    const s = (Date.now() - new Date(iso).getTime()) / 1000;
    if (s <= 900) return "live";
    if (s <= 4 * 3600) return "stale";
    return "offline";
  }

  function readJSON(el, key = "source") {
    const src = el.dataset[key] && document.getElementById(el.dataset[key]);
    if (!src) return null;
    try {
      return JSON.parse(src.textContent);
    } catch {
      return null;
    }
  }

  function base(el, opts = {}) {
    const map = L.map(el, { scrollWheelZoom: false, zoomControl: true, ...opts }).setView(ETHIOPIA, 6);
    const streets = L.tileLayer(el.dataset.tiles || "https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 18,
      className: "tiles-street",
      attribution: el.dataset.credit || "&copy; OpenStreetMap contributors",
    }).addTo(map);
    // A satellite view when the deployment offers one (AC-028.7).
    if (el.dataset.satellite) {
      const satellite = L.tileLayer(el.dataset.satellite, { maxZoom: 18, attribution: el.dataset.satelliteCredit || "" });
      map.layersControl = L.control.layers({ Map: streets, Satellite: satellite }, {}, { position: "topright" }).addTo(map);
    }
    // Wheel zoom only once the map has been clicked, so scrolling the page
    // doesn't get trapped.
    map.once("focus", () => map.scrollWheelZoom.enable());
    // The layout can still be settling when the map starts (fonts, the
    // scrollbar appearing), which leaves unpainted strips at the edge.
    if (window.ResizeObserver) new ResizeObserver(() => map.invalidateSize()).observe(el);
    return map;
  }

  function driverIcon(st) {
    return L.divIcon({ className: "map-pin " + st, iconSize: [16, 16] });
  }

  function destIcon(priority) {
    return L.divIcon({ className: "map-dest p-" + (priority || "normal"), iconSize: [12, 12] });
  }

  function onLive(fn) {
    window.addEventListener("kiflet:live", (e) => {
      if (e.detail && e.detail.type === "location") fn(e.detail.data);
    });
  }

  function fit(map, points, maxZoom = 12) {
    const valid = points.filter((p) => p && isFinite(p[0]) && isFinite(p[1]));
    if (valid.length === 1) map.setView(valid[0], Math.min(maxZoom, 11));
    else if (valid.length > 1) map.fitBounds(valid, { padding: [30, 30], maxZoom });
  }

  // The maps draw in the page's palette (Desk or Night), read once from the
  // tokens in app.css: routes and the truck in "info", the drop-off in
  // "bad", a delivery in "good", the start in "muted".
  const css = getComputedStyle(document.documentElement);
  const tok = (name, fallback) => css.getPropertyValue(name).trim() || fallback;
  const C = {
    info: tok("--info", "#1e40af"), good: tok("--good", "#17663a"), muted: tok("--muted", "#5f5b53"),
    line: tok("--focus", "#8a867d"), surface: tok("--surface", "#fff"),
  };

  // ---- a GPS trail -------------------------------------------------------------
  // The road a truck took: its phone's positions joined up. Where the phone
  // went quiet and the truck moved meanwhile (a gap), the line is dashed and
  // says so, rather than passing for road driven. Returns the last solid
  // line, which live positions extend.
  const TRAIL = { color: C.info, weight: 3, opacity: 0.9 };
  const TRAIL_GAP = { color: C.muted, weight: 2, opacity: 0.8, dashArray: "4 6" };

  function minutes(n) {
    return n < 90 ? n + " min" : Math.round(n / 60) + " h";
  }

  function drawTrail(layer, route, gaps) {
    route = route || [];
    let from = 0;
    (gaps || []).forEach((g) => {
      if (g.at <= from || g.at >= route.length) return;
      if (g.at - from > 1) L.polyline(route.slice(from, g.at), TRAIL).addTo(layer);
      L.polyline([route[g.at - 1], route[g.at]], TRAIL_GAP)
        .bindTooltip("No positions for " + minutes(g.minutes) + ": the driver's app was closed or had no signal")
        .addTo(layer);
      from = g.at;
    });
    return L.polyline(route.slice(from), TRAIL).addTo(layer);
  }

  // ---- fleet map --------------------------------------------------------------
  // Drivers at their last position, the destinations of active shipments and
  // a line from where each was collected, filtered by status, driver and
  // region (FR-028). Picking a load draws the road its truck took so far.
  const ROAD = ["in_transit", "delayed", "arrived", "issue"];
  const ATTENTION = ["delayed", "issue"];
  const ROUTE = { color: C.line, weight: 1.5, opacity: 0.7, dashArray: "5 6" };
  const ROUTE_ON = { color: C.info, weight: 3, opacity: 0.95, dashArray: null };

  function fleet(el) {
    const map = base(el);
    const meta = readJSON(el, "meta") || {};
    const drivers = new Map();
    const routes = L.layerGroup().addTo(map);
    const picked = L.layerGroup().addTo(map);
    const trails = new Map(); // shipment ID → its trail, fetched once a minute at most
    const dests = L.layerGroup().addTo(map);
    map.layersControl?.addOverlay(routes, "Routes");
    const controls = document.querySelector("[data-map-filters]");
    const count = controls?.querySelector("[data-map-count]");
    const filter = { status: "", driver: "", region: "" };
    let data = readJSON(el) || {};

    // Not animated: a fit that follows must not be undone when a zoom
    // animation ends.
    function home() {
      const c = meta.center;
      if (c) map.setView([c.lat, c.lng], c.zoom, { animate: false });
    }

    function statusMatch(s) {
      switch (filter.status) {
        case "road":
          return ROAD.includes(s.status);
        case "attention":
          return ATTENTION.includes(s.status);
        case "assigned":
          return s.status === "assigned";
        default:
          return true;
      }
    }

    function shipmentShown(s) {
      return statusMatch(s) && (!filter.driver || s.driver_id === filter.driver) && (!filter.region || (s.regions || []).includes(filter.region));
    }

    function loads(driverID) {
      return (data.shipments || []).filter((s) => s.driver_id === driverID);
    }

    // A driver shows when the filters leave one of their loads on the map;
    // with no shipment filter, every driver shows, loaded or not.
    function driverShown(d) {
      if (filter.driver && d.id !== filter.driver) return false;
      const own = loads(d.id);
      if (filter.status && !own.some(statusMatch)) return false;
      if (filter.region && d.region !== filter.region && !own.some(shipmentShown)) return false;
      return true;
    }

    function driverPopup(d) {
      const own = loads(d.id);
      let list = null;
      if (own.length) {
        list = document.createElement("ul");
        list.className = "map-popup-list";
        own.forEach((s) => {
          const li = document.createElement("li");
          li.append(link("/app/shipments/" + s.id, s.tracking_number), " ", text("span", (s.destination ? "to " + s.destination + " · " : "") + s.status_label, "muted"));
          list.append(li);
        });
      }
      return popup(
        text("strong", d.name),
        document.createElement("br"),
        d.phone ? link("tel:" + d.phone, d.phone) : null,
        text("div", "Position " + ago(d.at) + (d.speed_kph > 3 ? " · " + Math.round(d.speed_kph) + " km/h" : ""), "muted"),
        d.battery != null ? text("div", "Battery " + d.battery + "%" + (d.power_saving ? " · saving battery, a position every 15 min" : ""), "muted") : null,
        d.paused ? text("div", "Location sharing is off", "text-warn") : null,
        list || text("div", "No shipment assigned", "muted"),
        link("/app/drivers/" + d.id, "Driver page →"),
      );
    }

    function driverClass(d) {
      return state(d.at) + (loads(d.id).some((s) => ATTENTION.includes(s.status)) ? " attention" : "");
    }

    function upsertDriver(d) {
      let m = drivers.get(d.id);
      if (!m) {
        m = L.marker([d.lat, d.lng], { icon: driverIcon(driverClass(d)), riseOnHover: true });
        m.bindTooltip(() => m.data.name + " · " + ago(m.data.at), { direction: "top", offset: [0, -8], className: "map-label" });
        m.bindPopup(() => driverPopup(m.data));
        drivers.set(d.id, m);
      } else {
        m.setLatLng([d.lat, d.lng]);
      }
      m.data = { ...(m.data || {}), ...d };
      m.setIcon(driverIcon(driverClass(m.data)));
      if (driverShown(m.data)) m.addTo(map);
      else m.remove();
    }

    function draw(refit) {
      (data.drivers || []).forEach(upsertDriver);
      routes.clearLayers();
      dests.clearLayers();
      let shown = 0;
      (data.shipments || []).forEach((s) => {
        if (!shipmentShown(s)) return;
        shown++;
        if (s.lat == null) return;
        let line = null;
        if (s.origin_lat != null) {
          line = L.polyline([[s.origin_lat, s.origin_lng], [s.lat, s.lng]], { ...ROUTE, interactive: false }).addTo(routes);
        }
        const pin = L.marker([s.lat, s.lng], { icon: destIcon(s.priority) })
          .bindTooltip(s.tracking_number + (s.destination ? " · " + s.destination : ""), { direction: "top", offset: [0, -6], className: "map-label" })
          .bindPopup(() =>
            popup(
              link("/app/shipments/" + s.id, s.tracking_number),
              text("div", s.customer),
              text("div", (s.origin || "?") + " → " + (s.destination || "?")),
              text("div", (s.driver_name || "Unassigned") + " · " + s.status_label, "muted"),
              s.eta ? text("div", "Due " + new Date(s.eta).toLocaleString([], { weekday: "short", day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" }), "muted") : null,
            ),
          )
          .addTo(dests);
        // The picked shipment's route stands out: the road taken so far,
        // or the straight line until its truck has sent a position.
        pin.on("popupopen", () => {
          if (line) line.setStyle(ROUTE_ON).bringToFront();
          showTrail(s, line, pin);
        });
        pin.on("popupclose", () => {
          if (line) line.setStyle(ROUTE);
          picked.clearLayers();
        });
      });
      if (count) {
        const on = filter.status || filter.driver || filter.region;
        count.textContent = on ? shown + (shown === 1 ? " shipment" : " shipments") : "";
      }
      if (refit) frame();
    }

    function showTrail(s, line, pin) {
      const known = trails.get(s.id);
      const fresh = known && Date.now() - known.at < 60000;
      const draw = (t) => {
        picked.clearLayers();
        if (!t.route || t.route.length < 2) return;
        if (line) line.setStyle(ROUTE);
        drawTrail(picked, t.route, t.gaps);
        // The rest of the way, as the crow flies.
        L.polyline([t.route[t.route.length - 1], [s.lat, s.lng]], { ...ROUTE, interactive: false }).addTo(picked);
      };
      if (fresh) return draw(known.trail);
      fetch("/app/shipments/" + s.id + "/trail", { headers: { Accept: "application/json" } })
        .then((r) => (r.ok ? r.json() : null))
        .then((t) => {
          if (!t) return;
          trails.set(s.id, { trail: t, at: Date.now() });
          if (pin.isPopupOpen()) draw(t); // not if it was closed meanwhile
        })
        .catch(() => {});
    }

    function frame() {
      const pts = [];
      drivers.forEach((m) => map.hasLayer(m) && pts.push([m.data.lat, m.data.lng]));
      dests.eachLayer((l) => pts.push(l.getLatLng()));
      routes.eachLayer((l) => l.getLatLngs().forEach((p) => pts.push(p)));
      const region = filter.region && (meta.regions || []).find((r) => r.name === filter.region);
      if (pts.length) fit(map, pts.map((p) => (Array.isArray(p) ? p : [p.lat, p.lng])), 10);
      else if (region) map.fitBounds(region.bounds);
      else home();
    }

    // A refresh redraws the pins, which would close an open popup; it waits.
    let reading = false;
    map.on("popupopen", () => (reading = true));
    map.on("popupclose", () => (reading = false));

    draw(true);

    if (controls) {
      controls.hidden = false;
      controls.addEventListener("change", (e) => {
        const key = e.target.dataset.filter;
        if (!key) return;
        filter[key] = e.target.value;
        draw(true);
      });
    }

    onLive((d) => {
      const known = (data.drivers || []).find((x) => x.id === d.driver_id);
      const fix = { id: d.driver_id, name: d.name, lat: d.lat, lng: d.lng, at: d.at, speed_kph: d.speed_kph, battery: d.battery };
      if (known) Object.assign(known, fix);
      else (data.drivers = data.drivers || []).push(fix);
      upsertDriver(fix);
    });
    // Connection states age even when nothing arrives, and shipments change
    // hands; refresh both.
    setInterval(() => {
      if (document.hidden || reading) return;
      fetch("/app/api/map", { headers: { Accept: "application/json" } })
        .then((r) => (r.ok ? r.json() : null))
        .then((fresh) => {
          if (!fresh || reading) return;
          data = fresh;
          draw(false);
        })
        .catch(() => {});
    }, 60000);
  }

  // ---- one shipment -----------------------------------------------------------
  function shipment(el) {
    const data = readJSON(el) || {};
    const map = base(el);
    const pts = [];
    if (data.origin) {
      L.circleMarker([data.origin.lat, data.origin.lng], { radius: 6, color: C.muted, weight: 2, fillColor: C.surface, fillOpacity: 1 })
        .bindTooltip("Pickup: " + data.origin.label).addTo(map);
      pts.push([data.origin.lat, data.origin.lng]);
    }
    if (data.destination) {
      L.marker([data.destination.lat, data.destination.lng], { icon: destIcon() })
        .bindTooltip("Delivery: " + data.destination.label).addTo(map);
      L.circle([data.destination.lat, data.destination.lng], {
        radius: data.tolerance || 5000, color: C.info, weight: 1, fillOpacity: 0.05, dashArray: "4 4",
      }).bindTooltip("Deliveries recorded inside this circle count as at the destination").addTo(map);
      pts.push([data.destination.lat, data.destination.lng]);
    }
    const route = drawTrail(map, data.route, data.gaps);
    (data.route || []).forEach((p) => pts.push(p));
    let truck = null;
    const last = (data.route || []).slice(-1)[0];
    if (last && !data.pod) {
      truck = L.marker(last, { icon: driverIcon("stale") }).bindTooltip("Last known position").addTo(map);
    }
    if (data.pod) {
      L.circleMarker([data.pod.lat, data.pod.lng], { radius: 7, color: C.surface, weight: 2, fillColor: C.good, fillOpacity: 1 })
        .bindTooltip(data.pod.label).addTo(map);
      pts.push([data.pod.lat, data.pod.lng]);
    }
    fit(map, pts);
    const driverID = el.dataset.driverId;
    if (driverID && !data.pod) {
      onLive((d) => {
        if (d.driver_id !== driverID) return;
        route.addLatLng([d.lat, d.lng]);
        if (!truck) truck = L.marker([d.lat, d.lng]).addTo(map);
        truck.setLatLng([d.lat, d.lng]);
        truck.setIcon(driverIcon("live"));
      });
    }
  }

  // ---- a single point (driver page) ---------------------------------------------
  function point(el) {
    const p = readJSON(el);
    if (!p) return;
    const map = base(el);
    const m = L.marker([p.lat, p.lng], { icon: driverIcon("stale") }).bindTooltip(p.label).addTo(map);
    map.setView([p.lat, p.lng], 11);
    const driverID = el.dataset.driverId;
    onLive((d) => {
      if (d.driver_id !== driverID) return;
      m.setLatLng([d.lat, d.lng]);
      m.setIcon(driverIcon("live"));
      map.panTo([d.lat, d.lng]);
    });
  }

  // ---- public tracking ----------------------------------------------------------
  function track(el) {
    const data = readJSON(el) || {};
    const map = base(el, { dragging: !L.Browser.mobile });
    const pts = [];
    if (data.origin) {
      L.circleMarker([data.origin.lat, data.origin.lng], { radius: 6, color: C.muted, weight: 2, fillColor: C.surface, fillOpacity: 1 })
        .bindTooltip(data.origin.label, { permanent: true, direction: "top", className: "map-label" }).addTo(map);
      pts.push([data.origin.lat, data.origin.lng]);
    }
    if (data.destination) {
      L.marker([data.destination.lat, data.destination.lng], { icon: destIcon() })
        .bindTooltip(data.destination.label, { permanent: true, direction: "top", className: "map-label" }).addTo(map);
      pts.push([data.destination.lat, data.destination.lng]);
    }
    if (data.origin && data.destination) {
      L.polyline(pts, { color: C.line, weight: 2, dashArray: "6 6" }).addTo(map);
    }
    if (data.current) {
      L.circle([data.current.lat, data.current.lng], { radius: 4000, color: C.info, weight: 2, fillOpacity: 0.15 })
        .bindTooltip(data.current.label).addTo(map);
      pts.push([data.current.lat, data.current.lng]);
    }
    fit(map, pts, 9);
  }

  // ---- drop-off pin picker -----------------------------------------------------
  function picker(el) {
    const form = el.closest("form");
    const lat = form.querySelector(`[name="${el.dataset.lat}"]`);
    const lng = form.querySelector(`[name="${el.dataset.lng}"]`);
    const town = form.querySelector(`[data-town="${el.dataset.townInput}"]`);
    const details = el.closest("details");
    let map = null;
    let pin = null;

    function townPoint() {
      if (!town || !town.value) return null;
      const opt = Array.from(document.querySelectorAll("#towns option")).find(
        (o) => o.value.toLowerCase() === town.value.trim().toLowerCase(),
      );
      return opt && opt.dataset.lat ? [parseFloat(opt.dataset.lat), parseFloat(opt.dataset.lng)] : null;
    }

    function place(ll) {
      lat.value = ll.lat.toFixed(6);
      lng.value = ll.lng.toFixed(6);
      if (!pin) {
        pin = L.marker(ll, { draggable: true }).addTo(map);
        pin.on("dragend", () => place(pin.getLatLng()));
      } else pin.setLatLng(ll);
    }

    function init() {
      if (map) {
        map.invalidateSize();
        return;
      }
      map = base(el, { scrollWheelZoom: true });
      if (lat.value && lng.value) {
        const ll = L.latLng(parseFloat(lat.value), parseFloat(lng.value));
        place(ll);
        map.setView(ll, 14);
      } else {
        const t = townPoint();
        if (t) map.setView(t, 13);
      }
      map.on("click", (e) => place(e.latlng));
      town?.addEventListener("change", () => {
        const t = townPoint();
        if (t && !pin) map.setView(t, 13);
      });
    }

    form.querySelector("[data-clear-pin]")?.addEventListener("click", () => {
      lat.value = "";
      lng.value = "";
      if (pin) {
        pin.remove();
        pin = null;
      }
    });

    if (details && !details.open) details.addEventListener("toggle", () => details.open && init(), { once: false });
    else init();
  }

  const kinds = { fleet, shipment, point, track, picker };
  document.querySelectorAll("[data-map]").forEach((el) => {
    const fn = kinds[el.dataset.map];
    if (fn) fn(el);
  });
})();
