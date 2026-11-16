// The menu: the driver's settings, what the phone holds, help, and signing
// out.

import { api } from "../api.js";
import { ago, fmtBytes, h, toast } from "../dom.js";
import { saving, stopTracking, updateTracking, updateWakeLock } from "../gps.js";
import { go, header, render } from "../router.js";
import { changeSetting } from "../settings.js";
import { S, VERSION } from "../state.js";
import { clear, kvSet, storageEstimate, storageLow } from "../store.js";
import { forgetShipments, scheduleSync, sync } from "../sync.js";
import { t } from "../text.js";

function toggle(label, checked, onchange, help) {
  return h("div", { class: "setting" },
    h("label", { class: "toggle" }, h("input", { type: "checkbox", checked, onchange: (e) => onchange(e.target.checked) }), h("span", { text: label })),
    help ? h("p", { class: "muted small", text: help }) : null);
}

export function screenMenu() {
  const pending = S.outbox.length;
  let confirmOut = false, confirmClear = false;
  const outBox = h("div", {});
  function drawOut() {
    outBox.replaceChildren(confirmOut
      ? h("div", { class: "notice" }, h("p", { text: t("signOutPending", { n: pending }) }),
          h("button", { class: "btn warn", type: "button", text: t("signOutAnyway"), onclick: signOut }),
          h("button", { class: "btn link", type: "button", text: t("cancel"), onclick: () => { confirmOut = false; drawOut(); } }))
      : h("button", { class: "btn block", type: "button", text: t("signOut"), onclick: () => (pending ? ((confirmOut = true), drawOut()) : signOut()) }));
  }
  drawOut();

  const storage = h("section", { class: "card" });
  function drawStorage() {
    const st = S.storage;
    storage.replaceChildren(...[
      h("h2", { text: t("storage") }),
      st && st.quota ? h("p", { text: t("storageUse", { used: fmtBytes(st.usage), free: fmtBytes(Math.max(0, st.quota - st.usage)) }) }) : null,
      storageLow() ? h("p", { class: "caution-text", text: t("storageLow") }) : null,
      navigator.storage && navigator.storage.persist ? h("p", { class: "muted small", text: S.persisted ? t("storageSafe") : t("storageUnsafe") }) : null,
      confirmClear
        ? h("div", { class: "notice caution" }, h("p", { text: t("clearDataConfirm") }), h("p", { class: "small", text: t("clearDataHelp") }),
            h("button", { class: "btn warn", type: "button", text: t("clearData"), onclick: clearSaved }),
            h("button", { class: "btn link", type: "button", text: t("cancel"), onclick: () => { confirmClear = false; drawStorage(); } }))
        : h("button", { class: "btn block", type: "button", text: t("clearData"), onclick: () => {
            if (!navigator.onLine) return toast(t("clearDataOffline"));
            confirmClear = true;
            drawStorage();
          } }),
    ].filter(Boolean));
  }
  drawStorage();
  storageEstimate().then((st) => {
    S.storage = st;
    if (storage.isConnected) drawStorage();
  });

  const d = S.driver || {};
  const lowBattery = saving() && !S.settings.battery_saver;
  return h("div", {},
    header(t("menu"), "#/"),
    h("main", { class: "page" },
      h("section", { class: "card" }, h("p", {}, h("strong", { text: d.name })), h("p", { class: "muted", text: d.phone }), h("p", { class: "muted", text: d.company + (d.vehicle ? " · " + d.vehicle : "") })),
      h("section", { class: "card stack" },
        h("h2", { text: t("settings") }),
        h("div", { class: "setting" },
          h("p", { class: "muted small", text: t("language") }),
          h("div", { class: "lang" }, ["en", "am"].map((l) => h("button", { type: "button", class: S.lang === l ? "on" : "", onclick: async () => { await changeSetting("language", l); render(); } }, l === "en" ? "English" : "አማርኛ")))),
        toggle(t("batterySaver"), S.settings.battery_saver, async (on) => { await changeSetting("battery_saver", on); render(); }, t("batterySaverHelp")),
        lowBattery ? h("p", { class: "caution-text small", text: t("savingNow") }) : null,
        toggle(t("vibrate"), S.settings.vibrate, (on) => changeSetting("vibrate", on)),
        "wakeLock" in navigator ? toggle(t("keepAwake"), S.keepAwake, async (on) => { S.keepAwake = on; await kvSet("keepAwake", on); updateWakeLock(); }, t("keepAwakeHelp")) : null),
      h("section", { class: "card" },
        toggle(t("location"), S.tracking, async (on) => { S.tracking = on; await kvSet("tracking", on); updateTracking(); scheduleSync(500); }, t("locationHelp"))),
      h("section", { class: "card" },
        h("button", { class: "btn block", type: "button", text: t("syncNow"), onclick: () => sync() }),
        h("p", { class: "muted small", text: [!navigator.onLine ? t("offline") : S.weak ? t("weak") : "", S.lastSync ? t("lastSync", { t: ago(S.lastSync) }) : t("neverSynced"), pending ? t("waiting", { n: pending }) : ""].filter(Boolean).join(" · ") })),
      storage,
      h("a", { class: "btn block", href: "#/pin", text: t("changePin") }),
      outBox,
      h("section", { class: "card" },
        h("h2", { text: t("help") }),
        d.office_phone ? h("a", { class: "btn big call", href: "tel:" + d.office_phone, text: "📞 " + t("callOffice") + " " + d.office_phone }) : null,
        d.sms_number ? h("p", { class: "muted small", text: t("smsHelpGeneric", { num: d.sms_number }) }) : null),
      h("p", { class: "muted small center", text: t("version") + " " + VERSION + (S.config ? " · " + S.config.version : "") }),
    ));
}

// Clear saved data: the shipments and messages on the phone go, and the
// current work downloads again. Nothing the driver recorded is lost.
async function clearSaved() {
  if (!navigator.onLine) return toast(t("clearDataOffline"));
  await forgetShipments();
  S.notices = [];
  await kvSet("notices", []);
  toast(t("clearDataDone"));
  go("#/");
  render();
  await sync();
  S.storage = await storageEstimate();
}

async function signOut() {
  try {
    if (navigator.onLine) await api("/driver/logout", { method: "POST" });
  } catch { /* signing out locally is what matters */ }
  S.token = null;
  await kvSet("token", null);
  await Promise.all([clear("outbox"), clear("locations"), forgetShipments(), kvSet("notices", [])]);
  S.outbox = [];
  S.notices = [];
  S.blocked = false;
  S.pendingLocations = 0;
  stopTracking();
  go("#/");
  render();
}
