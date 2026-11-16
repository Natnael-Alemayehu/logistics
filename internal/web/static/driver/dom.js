// Building screens: h makes an element, toast shows a passing message,
// field labels an input, and the rest format times, distances and sizes for
// the driver.

import { S } from "./state.js";
import { t } from "./text.js";

export function h(tag, props, ...children) {
  const n = document.createElement(tag);
  for (const [k, v] of Object.entries(props || {})) {
    if (v == null || v === false) continue;
    if (k === "class") n.className = v;
    else if (k === "text") n.textContent = v;
    else if (k.startsWith("on")) n.addEventListener(k.slice(2), v);
    else if (v === true) n.setAttribute(k, "");
    else n.setAttribute(k, v);
  }
  for (const c of children.flat()) {
    if (c == null || c === false) continue;
    n.append(c instanceof Node ? c : document.createTextNode(String(c)));
  }
  return n;
}
export const root = () => document.getElementById("app");

export function toast(msg, kind) {
  let box = document.querySelector(".toasts");
  if (!box) document.body.append((box = h("div", { class: "toasts", "aria-live": "polite" })));
  const n = h("div", { class: "toast " + (kind || ""), role: "status", text: msg });
  box.append(n);
  setTimeout(() => n.remove(), 4500);
}

export function fmtTime(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  const now = new Date();
  const opts = { hour: "2-digit", minute: "2-digit" };
  if (d.toDateString() !== now.toDateString()) Object.assign(opts, { weekday: "short", day: "numeric", month: "short" });
  return d.toLocaleString(S.lang === "am" ? "am-ET" : "en-GB", opts);
}
export function fmtDist(m) {
  if (m < 1000) return Math.max(10, Math.round(m / 10) * 10) + " " + t("m");
  return (m < 10000 ? (m / 1000).toFixed(1) : Math.round(m / 1000)) + " " + t("km");
}
export function fmtBytes(b) {
  if (b < 1048576) return Math.max(1, Math.round(b / 1024)) + " KB";
  if (b < 1073741824) return (b / 1048576).toFixed(1) + " MB";
  return (b / 1073741824).toFixed(1) + " GB";
}
export function ago(ms) {
  const s = (Date.now() - ms) / 1000;
  if (s < 60) return S.lang === "am" ? "አሁን" : "just now";
  if (s < 3600) return Math.floor(s / 60) + (S.lang === "am" ? " ደቂቃ በፊት" : " min ago");
  return Math.floor(s / 3600) + (S.lang === "am" ? " ሰዓት በፊት" : " h ago");
}

export function field(label, input, error) {
  return h("label", { class: "field" }, h("span", { text: label }), input, error ? h("span", { class: "err", text: error }) : null);
}
