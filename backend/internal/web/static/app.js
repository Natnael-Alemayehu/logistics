// Dashboard behaviour. Every page works without this script; it adds
// conveniences (copy buttons, live updates, tooltips) on top.
(function () {
  "use strict";

  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));

  function el(tag, attrs, ...children) {
    const n = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs || {})) {
      if (k === "class") n.className = v;
      else if (k === "text") n.textContent = v;
      else n.setAttribute(k, v);
    }
    for (const c of children) n.append(c);
    return n;
  }

  // ---- toasts ---------------------------------------------------------------
  function toast(message, opts = {}) {
    const box = $("[data-toasts]");
    if (!box) return;
    const t = el("div", { class: "toast " + (opts.kind || ""), role: "status" });
    if (opts.title) t.append(el("strong", { text: opts.title }), document.createElement("br"));
    t.append(document.createTextNode(message));
    if (opts.href) {
      t.append(" ", el("a", { href: opts.href, text: opts.linkText || "Open" }));
    }
    box.append(t);
    setTimeout(() => t.remove(), opts.ttl || 6000);
  }
  window.mengedToast = toast;

  // ---- navigation drawer -------------------------------------------------------
  document.addEventListener("click", (e) => {
    const toggle = e.target.closest("[data-toggle='nav']");
    if (toggle) {
      const shown = document.body.classList.toggle("nav-shown");
      $$(".nav-open").forEach((b) => b.setAttribute("aria-expanded", String(shown)));
      $$(".scrim").forEach((s) => (s.hidden = !shown));
    }
  });

  // ---- small helpers -----------------------------------------------------------
  document.addEventListener("click", async (e) => {
    const t = e.target;

    const dismiss = t.closest("[data-dismiss]");
    if (dismiss) dismiss.closest("[data-flash]")?.remove();

    const copy = t.closest("[data-copy]");
    if (copy) {
      try {
        await navigator.clipboard.writeText(copy.dataset.copy);
        toast("Copied to the clipboard.", { ttl: 2500 });
      } catch {
        toast("Couldn't copy. Select the text and copy it instead.");
      }
    }

    if (t.closest("[data-print]")) window.print();

    const back = t.closest("[data-back]");
    if (back && history.length > 1) {
      e.preventDefault();
      history.back();
    }

    // Whole table rows link to their record, except when the click lands on
    // something interactive inside the row.
    const row = t.closest("tr[data-href]");
    if (row && !t.closest("a, button, input, select, textarea, label, summary")) {
      if (e.metaKey || e.ctrlKey) window.open(row.dataset.href, "_blank");
      else location.href = row.dataset.href;
    }

    // One popover open at a time; a click outside closes it.
    $$("details.pop[open], details.menu[open]").forEach((d) => {
      if (!d.contains(t)) d.open = false;
    });
  });

  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") {
      $$("details.pop[open], details.menu[open]").forEach((d) => (d.open = false));
      document.body.classList.remove("nav-shown");
    }
  });

  document.addEventListener("change", (e) => {
    if (e.target.matches("[data-autosubmit]")) e.target.form?.submit();

    // Picking a driver selects their usual vehicle.
    if (e.target.matches("[data-driver-select]")) {
      const opt = e.target.selectedOptions[0];
      const vehicles = e.target.form?.querySelector("[data-vehicle-select]");
      if (opt && vehicles && opt.dataset.vehicle) vehicles.value = opt.dataset.vehicle;
    }
  });

  // A form submits once: a second click while the first is in flight is
  // swallowed, so a slow network can't book a shipment twice.
  document.addEventListener("submit", (e) => {
    const form = e.target;
    if (form.method.toLowerCase() !== "post") return;
    if (form.dataset.submitting) {
      e.preventDefault();
      return;
    }
    form.dataset.submitting = "1";
    $$("button[type='submit'], button:not([type])", form).forEach((b) => b.setAttribute("aria-busy", "true"));
    setTimeout(() => delete form.dataset.submitting, 8000);
  });

  // ---- chart tooltip -------------------------------------------------------------
  function initChart(chart) {
    const tip = $(".viz-tip", chart);
    if (!tip) return;
    function show(bar) {
      const [title, ...rows] = bar.dataset.tip.split("|");
      tip.replaceChildren(el("p", { class: "tip-title", text: title }));
      rows.forEach((r, i) => {
        const [value, ...label] = r.split(" ");
        const key = el("span", { class: "key" });
        key.style.background = i === 0 ? "var(--text-2)" : "var(--viz-on)";
        tip.append(el("p", { class: "tip-row" }, key, el("strong", { text: value }), el("span", { class: "muted", text: label.join(" ") })));
      });
      tip.hidden = false;
      const box = chart.getBoundingClientRect();
      const b = bar.getBoundingClientRect();
      let x = b.left - box.left + b.width / 2 - tip.offsetWidth / 2;
      x = Math.max(0, Math.min(x, box.width - tip.offsetWidth));
      tip.style.left = x + "px";
      tip.style.top = Math.max(0, b.top - box.top - tip.offsetHeight + 40) + "px";
    }
    $$(".viz-bar", chart).forEach((bar) => {
      bar.addEventListener("pointerenter", () => show(bar));
      bar.addEventListener("focus", () => show(bar));
      bar.addEventListener("pointerleave", () => (tip.hidden = true));
      bar.addEventListener("blur", () => (tip.hidden = true));
    });
  }

  // ---- live updates --------------------------------------------------------------
  function setBadge(n) {
    $$("[data-badge]").forEach((b) => {
      b.textContent = n;
      b.hidden = n <= 0;
    });
  }

  function refreshBadge() {
    fetch("/app/api/badge", { headers: { Accept: "application/json" } })
      .then((r) => (r.ok ? r.json() : null))
      .then((j) => j && setBadge(j.unread))
      .catch(() => {});
  }

  function connectLive(path) {
    const state = $("[data-live-state]");
    let delay = 1000;
    let socket;
    let everOpened = false;

    function setState(on) {
      if (!state) return;
      state.classList.toggle("on", on);
      state.classList.toggle("off", !on);
      state.textContent = on ? "Live" : "Reconnecting…";
    }

    function open() {
      const url = (location.protocol === "https:" ? "wss://" : "ws://") + location.host + path;
      socket = new WebSocket(url);
      socket.addEventListener("open", () => {
        delay = 1000;
        setState(true);
        if (everOpened) refreshBadge(); // catch up on anything missed
        everOpened = true;
      });
      socket.addEventListener("message", (e) => {
        let msg;
        try {
          msg = JSON.parse(e.data);
        } catch {
          return;
        }
        window.dispatchEvent(new CustomEvent("menged:live", { detail: msg }));
        handle(msg);
      });
      socket.addEventListener("close", () => {
        setState(false);
        setTimeout(open, delay);
        delay = Math.min(delay * 2, 30000);
      });
    }
    open();
  }

  function handle(msg) {
    const d = msg.data || {};
    const page = $("[data-shipment]");
    if (msg.type === "alert") {
      const badge = $("[data-badge]");
      setBadge((parseInt(badge?.textContent, 10) || 0) + 1);
      toast(d.message, {
        title: d.label,
        kind: d.severity,
        href: d.shipment_id ? "/app/shipments/" + d.shipment_id : "/app/alerts",
        ttl: d.severity === "critical" ? 12000 : 7000,
      });
    } else if (msg.type === "shipment" && page && page.dataset.shipment === d.id) {
      toast("This shipment was updated (" + d.status_label.toLowerCase() + ").", {
        href: location.pathname,
        linkText: "Reload",
        ttl: 15000,
      });
    }
  }

  document.addEventListener("DOMContentLoaded", () => {
    $$("[data-chart]").forEach(initChart);
    const live = document.body.dataset.live;
    if (live && "WebSocket" in window) connectLive(live);
    // Flash messages fade after a while; errors stay until dismissed.
    $$("[data-flash].flash-success, [data-flash].flash-info").forEach((f) => setTimeout(() => f.remove(), 8000));
  });
})();
