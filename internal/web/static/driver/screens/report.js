// Reporting a delay or a problem, with its reason and a note.

import { field, h } from "../dom.js";
import { header } from "../router.js";
import { S } from "../state.js";
import { reasonLabel, t } from "../text.js";
import { setStatus } from "./detail.js";

export function screenReport(sh, status) {
  S.formOpen = true;
  const reasons = status === "delayed" ? S.config.delay_reasons : S.config.issue_reasons;
  let chosen = "";
  const err = h("p", { class: "err", role: "alert" });
  const note = h("textarea", { rows: "3", maxlength: "500" });
  const opts = h("div", { class: "choices" }, reasons.map((r) =>
    h("label", { class: "choice" }, h("input", { type: "radio", name: "reason", value: r.code, onchange: () => (chosen = r.code) }), h("span", { text: reasonLabel(r.code) }))));
  const form = h("form", { class: "stack", onsubmit: async (e) => {
    e.preventDefault();
    if (!chosen) return (err.textContent = t("reason"));
    await setStatus(sh, status, chosen, note.value.trim());
  } }, h("p", { class: "muted", text: t("reason") }), opts, field(t("note"), note), err,
    h("button", { class: "btn primary big", type: "submit", text: t("save") }),
    h("a", { class: "btn link", href: "#/s/" + sh.id, text: t("cancel") }));
  const title = status === "delayed" ? (sh.status === "delayed" ? t("updateDelay") : t("delayBtn")) : (sh.status === "issue" ? t("updateProblem") : t("problemBtn"));
  return h("div", {}, header(title, "#/s/" + sh.id), h("main", { class: "page" }, form));
}
