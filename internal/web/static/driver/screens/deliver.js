// The proof of delivery: who received the load, photos, a signature, and
// why it was delivered away from the drop-off if it was. The form is kept
// as a draft until it is sent, so a restart doesn't lose it.

import { field, fmtDist, h, toast } from "../dom.js";
import { metres, updateTracking } from "../gps.js";
import { go, header, render } from "../router.js";
import { signaturePad } from "../signature.js";
import { MAX_PHOTOS, S } from "../state.js";
import { del, kvGet, kvSet } from "../store.js";
import { enqueue, freshFix } from "../sync.js";
import { reasonLabel, t } from "../text.js";
import { dropOff } from "./detail.js";

function compress(file) {
  return new Promise((resolve, reject) => {
    const img = new Image();
    const url = URL.createObjectURL(file);
    img.onload = () => {
      // Fit inside 800×600 (600×800 for portrait shots): plenty to read a
      // label or see damage, small enough to send over 2G.
      const landscape = img.width >= img.height;
      const ratio = Math.min(1, (landscape ? 800 : 600) / img.width, (landscape ? 600 : 800) / img.height);
      const cw = Math.round(img.width * ratio), ch = Math.round(img.height * ratio);
      const c = document.createElement("canvas");
      c.width = cw;
      c.height = ch;
      c.getContext("2d").drawImage(img, 0, 0, cw, ch);
      URL.revokeObjectURL(url);
      resolve(c.toDataURL("image/jpeg", 0.72));
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error("unreadable image"));
    };
    img.src = url;
  });
}

export function screenDeliver(sh) {
  S.formOpen = true;
  const photos = [];
  const err = h("p", { class: "err", role: "alert" });
  // The form is saved as it is filled in: a phone short of memory often
  // reloads the page when the camera opens, and the driver would otherwise
  // start again.
  const draftKey = "draft:" + sh.id;
  let draftTimer = null, restoring = true;
  const saveDraft = () => {
    if (restoring) return;
    clearTimeout(draftTimer);
    draftTimer = setTimeout(() => {
      kvSet(draftKey, {
        name: name.value, phone: phone.value, notes: notes.value, photos: photos.slice(),
        strokes: pad.lines(), why, saved: Date.now(),
      }).catch(() => {});
    }, 400);
  };
  const dropDraft = () => {
    clearTimeout(draftTimer);
    restoring = true; // nothing more to save
    return del("kv", draftKey).catch(() => {});
  };
  const name = h("input", { required: true, autocomplete: "off", maxlength: "120", oninput: saveDraft });
  const phone = h("input", { type: "tel", inputmode: "tel", maxlength: "20", oninput: saveDraft });
  const notes = h("textarea", { rows: "2", maxlength: "500", oninput: saveDraft });
  const pad = signaturePad(saveDraft);
  const thumbs = h("div", { class: "thumbs" });
  const fileInput = h("input", { type: "file", accept: "image/*", capture: "environment", class: "visually-hidden", onchange: async () => {
    const f = fileInput.files[0];
    fileInput.value = "";
    if (!f || photos.length >= MAX_PHOTOS) return;
    try {
      photos.push(await compress(f));
      drawThumbs();
      saveDraft();
    } catch {
      toast(t("photoUnreadable"));
    }
  } });
  const addBtn = h("button", { type: "button", class: "btn", onclick: () => fileInput.click() }, "📷 " + t("addPhoto"));
  function drawThumbs() {
    thumbs.replaceChildren(...photos.map((p, i) => h("figure", {}, h("img", { src: p, alt: "" }),
      h("button", { type: "button", class: "btn link small", text: t("remove"), onclick: () => { photos.splice(i, 1); drawThumbs(); saveDraft(); } }))));
    addBtn.hidden = photos.length >= MAX_PHOTOS;
  }
  const submit = h("button", { class: "btn primary big", type: "submit", text: t("confirmDelivery") });

  // Who usually signs for this receiver, to save typing (AC-020.4).
  const taken = new Set();
  const names = [sh.consignee_name || sh.customer_name, ...(sh.recent_recipients || [])].filter((n) => {
    const k = (n || "").trim().toLowerCase();
    return k && !taken.has(k) && taken.add(k);
  }).slice(0, 6);
  const suggestions = names.length ? h("div", { class: "chips", role: "group", "aria-label": t("suggested") },
    names.map((n) => h("button", { type: "button", class: "chip", text: n, onclick: () => { name.value = n; saveDraft(); phone.focus(); } }))) : null;
  const quickNotes = h("div", { class: "chips", role: "group", "aria-label": t("quickNotes") },
    ["security", "reception", "partial", "damaged"].map((k) => h("button", { type: "button", class: "chip", text: t("note_" + k), onclick: () => {
      const cur = notes.value.trim(), add = t("note_" + k);
      if (!cur.split("\n").includes(add)) notes.value = (cur ? cur + "\n" + add : add).slice(0, 500);
      err.textContent = "";
      saveDraft();
    } })));

  // Too far from the drop-off (AC-022.2): the driver says why, or goes
  // closer. The phone checks as the screen opens, before anyone signs,
  // and again with the position the delivery is saved with.
  let why = "";
  const far = h("div", { class: "far", hidden: true });
  const distance = (fix) => (fix && dropOff(sh) && sh.tolerance_m ? metres(fix, dropOff(sh)) : 0);
  function drawFar(d) {
    far.hidden = false;
    submit.textContent = t("deliverAnyway");
    far.replaceChildren(
      h("p", {}, h("strong", { text: t("farTitle", { d: fmtDist(d) }) })),
      h("p", { class: "small", text: t("farHelp", { tol: fmtDist(sh.tolerance_m) }) }),
      h("p", { class: "small" }, h("strong", { text: t("whyHere") })),
      h("div", { class: "choices" }, (S.config.location_reasons || []).map((r) =>
        h("label", { class: "choice" }, h("input", { type: "radio", name: "why", value: r.code, checked: why === r.code, onchange: () => ((why = r.code), (err.textContent = ""), saveDraft()) }), h("span", { text: reasonLabel(r.code) })))),
      h("a", { class: "btn", href: "#/s/" + sh.id, text: t("goBack") }));
  }
  freshFix(15000).then((fix) => {
    const d = distance(fix);
    if (far.isConnected && d > sh.tolerance_m) drawFar(d);
  });

  const form = h("form", { class: "stack", onsubmit: async (e) => {
    e.preventDefault();
    err.textContent = "";
    if (!name.value.trim()) return (err.textContent = t("needName"));
    if (pad.empty()) return (err.textContent = t("needSignature"));
    submit.disabled = true;
    submit.textContent = t("locating");
    const fix = (await freshFix(10000)) || null;
    const d = distance(fix);
    const stop = (msg) => {
      submit.disabled = false;
      submit.textContent = t("deliverAnyway");
      err.textContent = msg;
      far.scrollIntoView({ block: "start" });
    };
    if (d > sh.tolerance_m) {
      if (far.hidden) {
        drawFar(d);
        return stop(t("needWhy"));
      }
      if (!why) return stop(t("needWhy"));
      if (why === "other" && !notes.value.trim()) return stop(t("needWhyNote"));
    } else why = ""; // close enough after all
    await enqueue("pod", {
      shipment_id: sh.id, recipient_name: name.value.trim(), recipient_phone: phone.value.trim(),
      signature: pad.data(), photos: photos.slice(), notes: notes.value.trim(), location_reason: why,
      lat: fix ? fix.lat : null, lng: fix ? fix.lng : null, recorded_at: new Date().toISOString(),
    });
    await dropDraft();
    if (navigator.vibrate) navigator.vibrate([60, 40, 60]);
    toast(fix ? t("delivered") : t("noFix"));
    updateTracking();
    go("#/s/" + sh.id);
    render();
  } },
    far, field(t("recipient"), name), suggestions, field(t("recipientPhone"), phone),
    h("div", { class: "field" }, h("span", { text: t("signature") }), h("div", { class: "sig-wrap" }, pad.canvas, h("span", { class: "sig-hint", text: t("signHere") })),
      h("button", { type: "button", class: "btn link small", text: t("clear"), onclick: () => pad.clear() })),
    h("div", { class: "field" }, h("span", { text: t("photos") }), thumbs, addBtn, fileInput),
    field(t("deliveryNotes"), notes), quickNotes, err, submit,
    // Cancel throws the form away; going back keeps it for later.
    h("a", { class: "btn link", href: "#/s/" + sh.id, text: t("cancel"), onclick: () => { dropDraft(); } }));

  kvGet(draftKey).then((d) => {
    if (d && form.isConnected !== false) {
      name.value = d.name || "";
      phone.value = d.phone || "";
      notes.value = d.notes || "";
      photos.push(...(d.photos || []).slice(0, MAX_PHOTOS));
      drawThumbs();
      if (d.strokes) pad.restore(d.strokes);
      if (d.why) why = d.why;
      if (d.name || (d.strokes || []).length || photos.length) toast(t("draftRestored"));
    }
  }).catch(() => {}).finally(() => { restoring = false; });

  return h("div", {}, header(t("deliverBtn") + " · #" + sh.ref, "#/s/" + sh.id),
    h("main", { class: "page" }, h("p", { class: "muted small" }, (sh.consignee_name || sh.customer_name) + " · " + sh.destination_address), form));
}
