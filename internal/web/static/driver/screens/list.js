// The driver's shipments, with the messages from the office above them.

import { fmtTime, h } from "../dom.js";
import { header, render } from "../router.js";
import { ACTIVE, S } from "../state.js";
import { kvSet, storageLow } from "../store.js";
import { effective } from "../sync.js";
import { msgText, statusLabel, t } from "../text.js";

function card(sh) {
  return h("a", { class: "ship " + (sh.priority !== "normal" ? "prio-" + sh.priority : ""), href: "#/s/" + sh.id },
    h("div", { class: "ship-top" },
      h("span", { class: "ref", text: "#" + sh.ref }),
      h("span", { class: "st st-" + sh.status, text: statusLabel(sh.status) }),
      sh.pending ? h("span", { class: "pending-dot", title: t("pendingSend") }) : null),
    h("div", { class: "ship-route" }, h("strong", { text: sh.origin_city || sh.origin_address }), " → ", h("strong", { text: sh.destination_city || sh.destination_address })),
    h("div", { class: "muted small" }, sh.customer_name, sh.eta && ACTIVE.includes(sh.status) ? " · " + t("due") + " " + fmtTime(sh.eta) : "",
      sh.delivered_at && sh.status === "delivered" ? " · " + t("deliveredAt") + " " + fmtTime(sh.delivered_at) : ""),
    sh.priority !== "normal" ? h("div", { class: "prio", text: t("priority_" + sh.priority) }) : null,
  );
}

export function screenList() {
  const list = [...S.shipments.values()].map(effective);
  const rank = { urgent: 0, high: 1, normal: 2 };
  const active = list.filter((s) => ACTIVE.includes(s.status))
    .sort((a, b) => (rank[a.priority] ?? 2) - (rank[b.priority] ?? 2) || (a.eta || "~").localeCompare(b.eta || "~"));
  const done = list.filter((s) => !ACTIVE.includes(s.status)).sort((a, b) => (b.updated_at || "").localeCompare(a.updated_at || ""));
  const unseen = S.messages.filter((m) => !S.seen.has(m.id));
  return h("div", {},
    header(t("myShipments")),
    h("main", { class: "page" },
      S.blocked ? h("div", { class: "notice caution", role: "alert" }, h("p", { text: t("blocked") })) : null,
      storageLow() ? h("div", { class: "notice caution" }, h("p", { text: t("storageLow") }), h("a", { class: "btn link", href: "#/menu", text: t("storage") })) : null,
      S.notices.map((n) => h("div", { class: "notice" }, h("span", {}, h("strong", { text: t("rejected") + ": " }), n.text),
        h("button", { type: "button", class: "btn link", text: t("dismiss"), onclick: async () => { S.notices = S.notices.filter((x) => x !== n); await kvSet("notices", S.notices); render(); } }))),
      unseen.length ? h("section", { class: "msgs" }, h("h2", { text: t("messages") }),
        unseen.slice(0, 3).map((m) => h("div", { class: "msg" }, h("p", { text: msgText(m) }), h("p", { class: "muted small", text: (m.tracking_number ? "#" + m.tracking_number.slice(-4) + " · " : "") + m.from + " · " + fmtTime(m.at) }))),
        h("button", { class: "btn link", type: "button", text: t("dismiss"), onclick: async () => { unseen.forEach((m) => S.seen.add(m.id)); await kvSet("seen", [...S.seen]); render(); } })) : null,
      h("h2", { class: "section", text: t("active") + " (" + active.length + ")" }),
      active.length ? active.map(card) : h("div", { class: "empty" }, h("p", { text: t("none") }), h("p", { class: "muted small", text: t("noneHelp") })),
      done.length ? h("details", { class: "done" }, h("summary", { text: t("done") + " (" + done.length + ")" }), done.map(card)) : null,
      S.installPrompt ? h("div", { class: "install" }, h("p", {}, h("strong", { text: t("install") }), h("br"), h("span", { class: "muted small", text: t("installHelp") })),
        h("button", { class: "btn", type: "button", text: t("install"), onclick: async () => { S.installPrompt.prompt(); S.installPrompt = null; render(); } })) : null,
    ));
}
