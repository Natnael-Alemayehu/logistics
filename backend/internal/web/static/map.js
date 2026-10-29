// Maps: the fleet map on the overview, a shipment's route, a driver's last
// position, the public tracking map and the drop-off pin picker. Data comes
// from a JSON <script> next to each map; live positions arrive as
// "menged:live" events from app.js.
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

  function readJSON(el) {
    const src = el.dataset.source && document.getElementById(el.dataset.source);
    if (!src) return null;
    try {
      return JSON.parse(src.textContent);
    } catch {
      return null;
    }
  }

  function base(el, opts = {}) {
    const map = L.map(el, { scrollWheelZoom: false, zoomControl: true, ...opts }).setView(ETHIOPIA, 6);
    L.tileLayer(el.dataset.tiles || "https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 18,
      attribution: el.dataset.credit || "&copy; OpenStreetMap contributors",
    }).addTo(map);
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
    window.addEventListener("menged:live", (e) => {
      if (e.detail && e.detail.type === "location") fn(e.detail.data);
    });
  }

  function fit(map, points, maxZoom = 12) {
    const valid = points.filter((p) => p && isFinite(p[0]) && isFinite(p[1]));
    if (valid.length === 1) map.setView(valid[0], Math.min(maxZoom, 11));
    else if (valid.length > 1) map.fitBounds(valid, { padding: [30, 30], maxZoom });
  }

  // ---- fleet map --------------------------------------------------------------
  function fleet(el) {
    const map = base(el);
    const drivers = new Map();
    const dests = L.layerGroup().addTo(map);

    function driverPopup(d) {
      return popup(
        text("strong", d.name),
        document.createElement("br"),
        d.phone ? link("tel:" + d.phone, d.phone) : null,
        text("div", "Position " + ago(d.at) + (d.speed_kph > 3 ? " · " + Math.round(d.speed_kph) + " km/h" : ""), "muted"),
        d.battery != null ? text("div", "Battery " + d.battery + "%", "muted") : null,
        d.paused ? text("div", "Location sharing is off", "text-warn") : null,
        link("/app/drivers/" + d.id, "Driver page →"),
      );
    }

    function upsertDriver(d) {
      const st = state(d.at);
      let m = drivers.get(d.id);
      if (!m) {
        m = L.marker([d.lat, d.lng], { icon: driverIcon(st), riseOnHover: true }).addTo(map);
        m.bindTooltip(d.name, { direction: "top", offset: [0, -8], className: "map-label" });
        drivers.set(d.id, m);
      } else {
        m.setLatLng([d.lat, d.lng]);
        m.setIcon(driverIcon(st));
      }
      m.data = { ...(m.data || {}), ...d };
      m.bindPopup(() => driverPopup(m.data));
    }

    function draw(data, refit) {
      (data.drivers || []).forEach(upsertDriver);
      dests.clearLayers();
      (data.shipments || []).forEach((s) => {
        if (s.lat == null) return;
        L.marker([s.lat, s.lng], { icon: destIcon(s.priority) })
          .bindPopup(() =>
            popup(
              link("/app/shipments/" + s.id, s.tracking_number),
              text("div", s.destination),
              text("div", (s.driver_name || "Unassigned") + " · " + s.status.replace("_", " "), "muted"),
            ),
          )
          .addTo(dests);
      });
      if (refit) {
        const pts = (data.drivers || []).map((d) => [d.lat, d.lng]);
        (data.shipments || []).forEach((s) => s.lat != null && pts.push([s.lat, s.lng]));
        fit(map, pts, 10);
      }
    }

    draw(readJSON(el) || {}, true);
    onLive((d) => upsertDriver({ id: d.driver_id, name: d.name, lat: d.lat, lng: d.lng, at: d.at, speed_kph: d.speed_kph, battery: d.battery }));
    // Connection states age even when nothing arrives; refresh them.
    setInterval(() => {
      if (document.hidden) return;
      fetch("/app/api/map", { headers: { Accept: "application/json" } })
        .then((r) => (r.ok ? r.json() : null))
        .then((data) => data && draw(data, false))
        .catch(() => {});
    }, 60000);
  }

  // ---- one shipment -----------------------------------------------------------
  function shipment(el) {
    const data = readJSON(el) || {};
    const map = base(el);
    const pts = [];
    if (data.origin) {
      L.circleMarker([data.origin.lat, data.origin.lng], { radius: 6, color: "#52514e", weight: 2, fillColor: "#fff", fillOpacity: 1 })
        .bindTooltip("Pickup: " + data.origin.label).addTo(map);
      pts.push([data.origin.lat, data.origin.lng]);
    }
    if (data.destination) {
      L.marker([data.destination.lat, data.destination.lng], { icon: destIcon() })
        .bindTooltip("Delivery: " + data.destination.label).addTo(map);
      L.circle([data.destination.lat, data.destination.lng], {
        radius: data.tolerance || 5000, color: "#2a78d6", weight: 1, fillOpacity: 0.05, dashArray: "4 4",
      }).bindTooltip("Deliveries recorded inside this circle count as at the destination").addTo(map);
      pts.push([data.destination.lat, data.destination.lng]);
    }
    const route = L.polyline(data.route || [], { color: "#2a78d6", weight: 3, opacity: 0.85 }).addTo(map);
    (data.route || []).forEach((p) => pts.push(p));
    let truck = null;
    const last = (data.route || []).slice(-1)[0];
    if (last && !data.pod) {
      truck = L.marker(last, { icon: driverIcon("stale") }).bindTooltip("Last known position").addTo(map);
    }
    if (data.pod) {
      L.circleMarker([data.pod.lat, data.pod.lng], { radius: 7, color: "#fff", weight: 2, fillColor: "#0ca30c", fillOpacity: 1 })
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
      L.circleMarker([data.origin.lat, data.origin.lng], { radius: 6, color: "#52514e", weight: 2, fillColor: "#fff", fillOpacity: 1 })
        .bindTooltip(data.origin.label, { permanent: true, direction: "top", className: "map-label" }).addTo(map);
      pts.push([data.origin.lat, data.origin.lng]);
    }
    if (data.destination) {
      L.marker([data.destination.lat, data.destination.lng], { icon: destIcon() })
        .bindTooltip(data.destination.label, { permanent: true, direction: "top", className: "map-label" }).addTo(map);
      pts.push([data.destination.lat, data.destination.lng]);
    }
    if (data.origin && data.destination) {
      L.polyline(pts, { color: "#898781", weight: 2, dashArray: "6 6" }).addTo(map);
    }
    if (data.current) {
      L.circle([data.current.lat, data.current.lng], { radius: 4000, color: "#2a78d6", weight: 2, fillOpacity: 0.15 })
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
