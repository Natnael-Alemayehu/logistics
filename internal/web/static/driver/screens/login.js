// Signing in, with the phone number and PIN, and a new PIN by SMS for a
// driver who forgot theirs. A different driver signing in on this phone
// clears what the last one left.

import { ApiError, api } from "../api.js";
import { field, h, toast } from "../dom.js";
import { go, render } from "../router.js";
import { changeSetting } from "../settings.js";
import { DEFAULT_SETTINGS, S } from "../state.js";
import { clear, kvGet, kvSet, protectStorage } from "../store.js";
import { sync } from "../sync.js";
import { t } from "../text.js";

export function screenLogin(mode = "login", state = {}) {
  S.formOpen = true;
  const wrap = h("main", { class: "auth" });
  const err = h("p", { class: "err", role: "alert" });
  const draw = () => wrap.replaceChildren(...content().filter(Boolean));
  const busy = (form, on) => form.querySelectorAll("button").forEach((b) => (b.disabled = on));
  const fail = (e) => (err.textContent = e instanceof ApiError ? e.message : t("networkError"));

  const langSwitch = () =>
    h("div", { class: "lang" },
      ["en", "am"].map((l) => h("button", { type: "button", class: S.lang === l ? "on" : "", onclick: async () => { pickedAtLogin = true; await changeSetting("language", l); draw(); } }, l === "en" ? "English" : "አማርኛ")));

  function content() {
    err.textContent = "";
    if (mode === "reset") {
      const phone = h("input", { name: "phone", type: "tel", inputmode: "tel", autocomplete: "tel", required: true, value: state.phone || "" });
      const form = h("form", { class: "stack", onsubmit: async (e) => {
        e.preventDefault();
        busy(form, true);
        try {
          await api("/driver/pin-reset/request", { method: "POST", auth: false, body: { phone: phone.value } });
          mode = "confirm";
          state.phone = phone.value;
          draw();
          toast(t("codeSent"));
        } catch (x) { fail(x); } finally { busy(form, false); }
      } }, field(t("phone"), phone), err, h("button", { class: "btn primary big", type: "submit", text: t("sendCode") }));
      return [h("h1", { text: t("forgotPin") }), form, h("button", { class: "btn link", type: "button", text: t("back"), onclick: () => { mode = "login"; draw(); } })];
    }
    if (mode === "confirm") {
      const code = h("input", { inputmode: "numeric", autocomplete: "one-time-code", maxlength: "6", required: true, class: "code" });
      const pin = h("input", { type: "password", inputmode: "numeric", maxlength: "6", required: true });
      const pin2 = h("input", { type: "password", inputmode: "numeric", maxlength: "6", required: true });
      const form = h("form", { class: "stack", onsubmit: async (e) => {
        e.preventDefault();
        if (pin.value !== pin2.value) return (err.textContent = t("pinsDiffer"));
        busy(form, true);
        try {
          const res = await api("/driver/pin-reset/confirm", { method: "POST", auth: false, body: { phone: state.phone, code: code.value, pin: pin.value, device_id: S.deviceId } });
          await signedIn(res);
        } catch (x) { fail(x); } finally { busy(form, false); }
      } }, field(t("code"), code), field(t("newPin"), pin), field(t("newPinAgain"), pin2), err, h("button", { class: "btn primary big", type: "submit", text: t("setPin") }));
      return [h("h1", { text: t("forgotPin") }), form, h("button", { class: "btn link", type: "button", text: t("back"), onclick: () => { mode = "reset"; draw(); } })];
    }
    const phone = h("input", { name: "phone", type: "tel", inputmode: "tel", autocomplete: "tel", required: true, value: state.phone || "", placeholder: "09…" });
    const pin = h("input", { name: "pin", type: "password", inputmode: "numeric", autocomplete: "current-password", maxlength: "6", required: true });
    const form = h("form", { class: "stack", onsubmit: async (e) => {
      e.preventDefault();
      busy(form, true);
      try {
        const res = await api("/driver/login", { method: "POST", auth: false, body: { phone: phone.value, pin: pin.value, device_id: S.deviceId } });
        await signedIn(res);
      } catch (x) { fail(x); } finally { busy(form, false); }
    } }, field(t("phone"), phone), field(t("pin"), pin), err, h("button", { class: "btn primary big", type: "submit", text: t("signIn") }));
    return [
      h("div", { class: "brand" }, h("span", { class: "bar-logo big", "aria-hidden": "true" }), h("strong", { text: (S.config && S.config.app_name) || "Kiflet" })),
      langSwitch(),
      h("p", { class: "muted", text: t("signInHelp") }),
      form,
      h("button", { class: "btn link", type: "button", text: t("forgotPin"), onclick: () => { mode = "reset"; state.phone = phone.value; draw(); } }),
      S.config && S.config.sms_number ? h("p", { class: "muted small", text: t("smsHelpGeneric", { num: S.config.sms_number }) }) : null,
    ];
  }
  draw();
  return wrap;
}

let pickedAtLogin = false; // the language was chosen on the sign-in screen
async function signedIn(res) {
  const prev = await kvGet("driver");
  if (prev && prev.id !== res.driver.id) {
    // A different driver on this phone: nothing of the last one carries
    // over, except the language just chosen to sign in.
    await Promise.all([clear("shipments"), clear("outbox"), clear("locations")]);
    S.shipments.clear();
    S.outbox = [];
    S.messages = [];
    S.cursor = null;
    S.settings = { ...DEFAULT_SETTINGS, language: S.lang };
    S.changed = pickedAtLogin ? { language: S.lang } : {};
    await kvSet("cursor", null);
    await kvSet("messages", []);
    await kvSet("settings", S.settings);
    await kvSet("settingsChanged", S.changed);
  }
  pickedAtLogin = false;
  protectStorage();
  S.token = res.token;
  S.driver = { ...res.driver, must_change_pin: res.must_change_pin };
  await kvSet("token", S.token);
  await kvSet("driver", S.driver);
  go("#/");
  render();
  sync();
}
