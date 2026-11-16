// Changing the PIN: on the driver's request, or before anything else when
// the office has set a temporary one.

import { ApiError, api } from "../api.js";
import { field, h, toast } from "../dom.js";
import { go, header, render } from "../router.js";
import { S } from "../state.js";
import { kvSet } from "../store.js";
import { signedOut } from "../sync.js";
import { t } from "../text.js";

export function screenPin(forced) {
  S.formOpen = true;
  const cur = h("input", { type: "password", inputmode: "numeric", maxlength: "6", required: true, autocomplete: "current-password" });
  const pin = h("input", { type: "password", inputmode: "numeric", maxlength: "6", required: true, autocomplete: "new-password" });
  const pin2 = h("input", { type: "password", inputmode: "numeric", maxlength: "6", required: true, autocomplete: "new-password" });
  const err = h("p", { class: "err", role: "alert" });
  const form = h("form", { class: "stack", onsubmit: async (e) => {
    e.preventDefault();
    err.textContent = "";
    if (pin.value !== pin2.value) return (err.textContent = t("pinsDiffer"));
    form.querySelector("button").disabled = true;
    try {
      await api("/driver/pin", { method: "POST", body: { current_pin: cur.value, new_pin: pin.value } });
      S.driver.must_change_pin = false;
      await kvSet("driver", S.driver);
      toast(t("pinChanged"));
      go("#/");
      render();
    } catch (x) {
      if (x instanceof ApiError && x.status === 401) return signedOut();
      err.textContent = x instanceof ApiError ? x.message : t("networkError");
    } finally {
      form.querySelector("button").disabled = false;
    }
  } }, field(t("currentPin"), cur), field(t("newPin"), pin), field(t("newPinAgain"), pin2), err,
    h("button", { class: "btn primary big", type: "submit", text: t("setPin") }));
  return h("div", {},
    forced ? h("header", { class: "bar" }, h("span", { class: "bar-logo" }), h("div", { class: "bar-title" }, h("strong", { text: t("choosePin") }))) : header(t("changePin"), "#/menu"),
    h("main", { class: "page" }, forced ? h("p", { class: "muted", text: t("choosePinHelp") }) : null, form));
}
