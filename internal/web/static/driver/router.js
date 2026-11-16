// The router: the address's hash picks the screen (#/s/<id>/deliver, for
// one), and render draws it. Changes from a sync redraw the screen through
// rerender, which leaves a screen with a form alone: each such screen sets
// S.formOpen, and render clears it. header is the bar at the top of every
// screen, with the sync pill.

import { ago, h, root } from "./dom.js";
import { screenDeliver } from "./screens/deliver.js";
import { screenDetail } from "./screens/detail.js";
import { screenList } from "./screens/list.js";
import { screenLogin } from "./screens/login.js";
import { screenMenu } from "./screens/menu.js";
import { screenMessage } from "./screens/message.js";
import { screenPin } from "./screens/pin.js";
import { screenReport } from "./screens/report.js";
import { S } from "./state.js";
import { effective, sync } from "./sync.js";
import { t } from "./text.js";

export function go(hash) {
  if (location.hash !== hash) location.hash = hash;
}
export function rerender() {
  if (!S.formOpen) render();
  else renderPill();
}

export function render() {
  const app = root();
  S.formOpen = false;
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

export function header(title, backHref) {
  return h("header", { class: "bar" },
    backHref ? h("a", { class: "bar-btn", href: backHref, "aria-label": t("back"), text: "‹" }) : h("span", { class: "bar-logo", "aria-hidden": "true" }),
    h("div", { class: "bar-title" }, h("strong", { text: title }), S.driver ? h("span", { class: "bar-sub", text: S.driver.company }) : null),
    h("button", { class: "pill-sync", type: "button", "data-pill": true, onclick: () => sync() }),
    h("a", { class: "bar-btn", href: "#/menu", "aria-label": t("menu"), text: "☰" }),
  );
}

export function renderPill() {
  const pill = document.querySelector("[data-pill]");
  if (!pill) return;
  const waiting = S.outbox.length;
  let cls = "ok", txt = t("allSent");
  if (S.syncing) (cls = "busy"), (txt = t("syncing"));
  else if (!S.online || !navigator.onLine) (cls = "off"), (txt = waiting ? t("waiting", { n: waiting }) : t("offline"));
  else if (S.weak) (cls = "weak"), (txt = waiting ? t("waiting", { n: waiting }) : t("weak"));
  else if (waiting) (cls = "wait"), (txt = t("waiting", { n: waiting }));
  pill.className = "pill-sync " + cls;
  pill.textContent = txt;
  pill.title = (S.weak ? t("weak") + " · " : "") + (S.lastSync ? t("lastSync", { t: ago(S.lastSync) }) : t("neverSynced"));
}
