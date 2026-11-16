// The signature box on the proof of delivery. The strokes are kept as
// points, so the signature is drawn again unchanged when the phone turns,
// and saved with the draft.

import { h } from "./dom.js";
import { t } from "./text.js";

// onChange is told when a stroke ends or the box is cleared.
export function signaturePad(onChange) {
  const canvas = h("canvas", { class: "sig", "aria-label": t("signHere") });
  const ctx = canvas.getContext("2d");
  // lines: each stroke's points, with the size of the box it was drawn in.
  let lines = [], drawing = null, box = null;
  function scaleOf(l) {
    return Math.min(box.width / l.w, box.height / l.h);
  }
  function drawLine(l) {
    const k = scaleOf(l);
    const [x0, y0] = l.pts[0];
    ctx.beginPath();
    if (l.pts.length === 1) {
      ctx.arc(x0 * k, y0 * k, 1.1, 0, Math.PI * 2);
      ctx.fill();
      return;
    }
    ctx.moveTo(x0 * k, y0 * k);
    for (const [x, y] of l.pts.slice(1)) ctx.lineTo(x * k, y * k);
    ctx.stroke();
  }
  // size fits the canvas to the box it is shown in and draws the strokes
  // again.
  function size() {
    const r = canvas.getBoundingClientRect();
    if (!r.width) return;
    if (box && box.width === r.width && box.height === r.height) return;
    const ratio = Math.max(1, window.devicePixelRatio || 1);
    canvas.width = Math.round(r.width * ratio);
    canvas.height = Math.round(r.height * ratio);
    ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
    box = { width: r.width, height: r.height };
    redraw();
  }
  function redraw() {
    ctx.fillStyle = "#fff";
    ctx.fillRect(0, 0, box.width, box.height);
    ctx.lineWidth = 2.4;
    ctx.lineCap = ctx.lineJoin = "round";
    ctx.strokeStyle = ctx.fillStyle = "#111";
    lines.forEach(drawLine);
  }
  function clearPad() {
    lines = [];
    if (box) redraw();
    if (onChange) onChange();
  }
  // restore draws saved strokes back.
  function restore(saved) {
    if (!Array.isArray(saved)) return;
    lines = saved.filter((l) => l && l.w > 0 && l.h > 0 && Array.isArray(l.pts) && l.pts.length);
    if (!box) size();
    else redraw();
  }
  const pos = (e) => {
    const r = canvas.getBoundingClientRect();
    return [Math.round((e.clientX - r.left) * 10) / 10, Math.round((e.clientY - r.top) * 10) / 10];
  };
  canvas.addEventListener("pointerdown", (e) => {
    e.preventDefault();
    if (!box) return;
    canvas.setPointerCapture(e.pointerId);
    drawing = { w: box.width, h: box.height, pts: [pos(e)] };
    lines.push(drawing);
    drawLine(drawing);
  });
  canvas.addEventListener("pointermove", (e) => {
    if (!drawing) return;
    const last = drawing.pts[drawing.pts.length - 1];
    const p = pos(e);
    drawing.pts.push(p);
    ctx.beginPath();
    ctx.moveTo(last[0], last[1]);
    ctx.lineTo(p[0], p[1]);
    ctx.stroke();
  });
  const end = () => {
    if (!drawing) return;
    drawing = null;
    if (onChange) onChange();
  };
  canvas.addEventListener("pointerup", end);
  canvas.addEventListener("pointercancel", end);
  // The phone turned or the window changed size: fit the box again. The
  // listener goes once the form has.
  let resizeTimer = null;
  const onResize = () => {
    if (!canvas.isConnected) {
      window.removeEventListener("resize", onResize);
      window.removeEventListener("orientationchange", onResize);
      return;
    }
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(size, 150);
  };
  window.addEventListener("resize", onResize);
  window.addEventListener("orientationchange", onResize);
  requestAnimationFrame(() => box || size());
  return {
    canvas, clear: clearPad, restore, empty: () => lines.length === 0,
    lines: () => lines, data: () => canvas.toDataURL("image/png"),
  };
}
