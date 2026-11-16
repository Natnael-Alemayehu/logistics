// A message to the office about a shipment.

import { h, toast } from "../dom.js";
import { go, header, render } from "../router.js";
import { S } from "../state.js";
import { enqueue } from "../sync.js";
import { t } from "../text.js";

export function screenMessage(sh) {
  S.formOpen = true;
  const text = h("textarea", { rows: "4", maxlength: "1000", required: true, placeholder: t("messagePlaceholder") });
  const form = h("form", { class: "stack", onsubmit: async (e) => {
    e.preventDefault();
    if (!text.value.trim()) return;
    await enqueue("message", { shipment_id: sh.id, text: text.value.trim(), recorded_at: new Date().toISOString() });
    toast(t("messageSaved"));
    go("#/s/" + sh.id);
    render();
  } }, text,
    h("div", { class: "chips", role: "group", "aria-label": t("quickMessages") },
      ["onway", "unloading", "callme", "roadclosed"].map((k) => h("button", { type: "button", class: "chip", text: t("quick_" + k), onclick: () => { text.value = t("quick_" + k); text.focus(); } }))),
    h("button", { class: "btn primary big", type: "submit", text: t("send") }), h("a", { class: "btn link", href: "#/s/" + sh.id, text: t("cancel") }));
  return h("div", {}, header(t("messageBtn"), "#/s/" + sh.id), h("main", { class: "page" }, form));
}
