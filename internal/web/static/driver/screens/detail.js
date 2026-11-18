// One shipment: where it goes, the way there, and the buttons that move it
// on (picked up, arrived, delayed, a problem, delivered).

import { ago, fmtDist, fmtTime, h, toast } from "../dom.js";
import { metres, unpark, updateTracking } from "../gps.js";
import { go, header, render } from "../router.js";
import { ACTIVE, S } from "../state.js";
import { enqueue, fixForEvent, freshFix } from "../sync.js";
import { msgText, reasonLabel, statusLabel, t } from "../text.js";

function directions(sh) {
  if (sh.destination_lat != null) return `https://www.google.com/maps/dir/?api=1&destination=${sh.destination_lat},${sh.destination_lng}`;
  return "https://www.google.com/maps/search/?api=1&query=" + encodeURIComponent(sh.destination_address + ", " + sh.destination_city + ", Ethiopia");
}

export async function setStatus(sh, status, reason = "", note = "") {
  const fix = fixForEvent();
  await enqueue("status", { shipment_id: sh.id, status, reason, note, lat: fix.lat, lng: fix.lng, recorded_at: new Date().toISOString() });
  if (navigator.vibrate) navigator.vibrate(40);
  toast(t("statusSaved"));
  unpark();
  updateTracking();
  go("#/s/" + sh.id);
  render();
}

// Straight-line distance and compass direction to the drop-off from the
// phone's last position, so it works without signal (FR-017).
const DIRS = ["n", "ne", "e", "se", "s", "sw", "w", "nw"];
function bearing(a, b) {
  const rad = Math.PI / 180;
  const dLng = (b.lng - a.lng) * rad;
  const y = Math.sin(dLng) * Math.cos(b.lat * rad);
  const x = Math.cos(a.lat * rad) * Math.sin(b.lat * rad) - Math.sin(a.lat * rad) * Math.cos(b.lat * rad) * Math.cos(dLng);
  return (Math.atan2(y, x) / rad + 360) % 360;
}
export const dropOff = (sh) => (sh.destination_lat != null ? { lat: sh.destination_lat, lng: sh.destination_lng } : null);
function awayLine(sh) {
  const line = h("p", { class: "away" });
  const draw = () => {
    const f = S.lastFix, to = dropOff(sh);
    if (!f || !to) return line.replaceChildren();
    const dir = t("dir_" + DIRS[Math.round(bearing(f, to) / 45) % 8]);
    line.replaceChildren(h("strong", { text: t("awayFrom", { d: fmtDist(metres(f, to)), dir }) }), h("br"),
      h("span", { class: "muted small", text: t("fixFrom", { t: ago(f.time) }) }));
  };
  draw();
  if (!S.tracking) line.replaceChildren(h("span", { class: "muted small", text: t("locationOffHere") }));
  else if (!S.lastFix || Date.now() - S.lastFix.time > 5 * 60000) freshFix(15000).then(() => line.isConnected && draw());
  return line;
}

// Leaflet is loaded the first time a map is shown, and only with signal.
let leafletP = null;
function leaflet() {
  if (window.L) return Promise.resolve(window.L);
  if (!leafletP) {
    leafletP = new Promise((resolve, reject) => {
      document.head.append(h("link", { rel: "stylesheet", href: "/static/vendor/leaflet/leaflet.css" }));
      document.head.append(h("script", { src: "/static/vendor/leaflet/leaflet.js", onload: () => resolve(window.L), onerror: reject }));
    }).catch((e) => {
      leafletP = null;
      throw e;
    });
  }
  return leafletP;
}
function miniMap(sh) {
  const to = dropOff(sh);
  if (!to || !navigator.onLine || !S.config.tile_url) return null;
  const box = h("div", { class: "minimap" });
  const wrap = h("section", { class: "card map-card" }, box,
    h("p", { class: "map-key small muted" }, h("span", { class: "dot you" }), t("you"), h("span", { class: "dot drop" }), t("dropOff")));
  leaflet().then((L) => {
    if (!box.isConnected) return;
    const map = L.map(box, { zoomControl: false, scrollWheelZoom: false });
    L.tileLayer(S.config.tile_url, { attribution: S.config.tile_credit, maxZoom: 18 }).addTo(map);
    // In the theme's colours, as the dashboard's maps: the drop-off in
    // "bad", the driver in "info".
    const css = getComputedStyle(document.documentElement);
    const tok = (name, fallback) => css.getPropertyValue(name).trim() || fallback;
    const dot = (ll, color) => L.circleMarker(ll, { radius: 8, weight: 3, color: tok("--surface", "#fff"), fillColor: color, fillOpacity: 1 }).addTo(map);
    dot([to.lat, to.lng], tok("--bad", "#b42318"));
    const f = S.lastFix;
    if (f) {
      dot([f.lat, f.lng], tok("--info", "#1e40af"));
      map.fitBounds([[to.lat, to.lng], [f.lat, f.lng]], { padding: [24, 24], maxZoom: 15 });
    } else map.setView([to.lat, to.lng], 13);
  }).catch(() => wrap.remove());
  return wrap;
}

export function screenDetail(sh) {
  const moves = (S.config.transitions[sh.status] || []).filter((m) => m !== "delivered");
  const canDeliver = S.config.pod_from.includes(sh.status);
  const actions = [];
  if (moves.includes("in_transit")) actions.push(h("button", { class: "btn primary big", type: "button", text: sh.status === "assigned" ? t("startTrip") : t("backOnRoad"), onclick: () => setStatus(sh, "in_transit") }));
  if (moves.includes("arrived")) actions.push(h("button", { class: "btn big st-btn-arrived", type: "button", text: t("arrivedBtn"), onclick: () => setStatus(sh, "arrived") }));
  if (canDeliver) actions.push(h("a", { class: "btn big " + (sh.status === "arrived" ? "primary" : "good"), href: "#/s/" + sh.id + "/deliver", text: t("deliverBtn") }));
  if (moves.includes("delayed")) actions.push(h("a", { class: "btn", href: "#/s/" + sh.id + "/report/delayed", text: t("delayBtn") }));
  if (moves.includes("issue")) actions.push(h("a", { class: "btn warn", href: "#/s/" + sh.id + "/report/issue", text: t("problemBtn") }));
  // A new cause for the delay or problem the shipment is already in.
  if (sh.status === "delayed") actions.push(h("a", { class: "btn", href: "#/s/" + sh.id + "/report/delayed", text: t("updateDelay") }));
  if (sh.status === "issue") actions.push(h("a", { class: "btn warn", href: "#/s/" + sh.id + "/report/issue", text: t("updateProblem") }));
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
          ACTIVE.includes(sh.status) ? awayLine(sh) : null,
          ACTIVE.includes(sh.status) ? h("a", { class: "btn small", href: directions(sh), target: "_blank", rel: "noopener", text: "➜ " + t("navigate") }) : null),
        sh.planned_pickup && sh.status === "assigned" ? h("p", {}, h("span", { class: "muted", text: t("pickup") + ": " }), fmtTime(sh.planned_pickup)) : null,
        sh.eta && ACTIVE.includes(sh.status) ? h("p", {}, h("span", { class: "muted", text: t("due") + ": " }), fmtTime(sh.eta)) : null),
      ACTIVE.includes(sh.status) ? miniMap(sh) : null,
      // Who takes delivery comes first: that is who the driver is going to.
      sh.consignee_phone ? h("section", { class: "card" },
        h("p", { class: "muted small", text: t("receiver") }),
        h("p", {}, h("strong", { text: sh.consignee_name })),
        h("a", { class: "btn big call", href: "tel:" + sh.consignee_phone, text: "📞 " + t("call") + " " + sh.consignee_phone })) : null,
      h("section", { class: "card" },
        h("p", { class: "muted small", text: t("customer") }),
        h("p", {}, h("strong", { text: sh.customer_name })),
        h("a", { class: "btn call" + (sh.consignee_phone ? "" : " big"), href: "tel:" + sh.customer_phone, text: "📞 " + t("call") + " " + sh.customer_phone })),
      sh.cargo || sh.weight_kg || sh.vehicle ? h("section", { class: "card" },
        sh.cargo ? h("p", {}, h("span", { class: "muted", text: t("cargo") + ": " }), sh.cargo, sh.pieces ? " · " + sh.pieces : "", sh.weight_kg ? " · " + sh.weight_kg + " " + t("kg") : "") : null,
        sh.vehicle ? h("p", {}, h("span", { class: "muted", text: t("vehicle") + ": " }), sh.vehicle) : null) : null,
      sh.instructions ? h("section", { class: "card instructions" }, h("p", { class: "muted small", text: t("instructions") }), h("p", { class: "pre", text: sh.instructions })) : null,
      msgs.length ? h("section", { class: "card" }, h("p", { class: "muted small", text: t("messages") }),
        msgs.map((m) => h("div", { class: "msg" }, h("p", { text: msgText(m) }), h("p", { class: "muted small", text: m.from + " · " + fmtTime(m.at) })))) : null,
      ACTIVE.includes(sh.status) ? h("a", { class: "btn block", href: "#/s/" + sh.id + "/message", text: "✉ " + t("messageBtn") }) : null,
      smsNum && ACTIVE.includes(sh.status) ? h("p", { class: "muted small sms-help", text: t("smsHelp", { ref: "ET " + sh.ref, num: smsNum }) }) : null,
    ));
}
