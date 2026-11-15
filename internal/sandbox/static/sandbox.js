// Keeps the sandbox's pages current. Every two seconds, while the tab is
// shown, the page is fetched again and each part marked data-poll whose
// content changed is swapped in; data-poll="end" keeps a thread scrolled to
// its newest text. Open log entries stay open.
(function () {
  "use strict";

  function atEnd(el) {
    return el.scrollTop + el.clientHeight >= el.scrollHeight - 24;
  }

  function scrollEnds() {
    document.querySelectorAll('[data-poll="end"]').forEach(function (el) {
      el.scrollTop = el.scrollHeight;
    });
  }

  var busy = false;
  function refresh() {
    var parts = document.querySelectorAll("[data-poll][id]");
    if (busy || document.hidden || parts.length === 0) return;
    busy = true;
    fetch(location.href, { cache: "no-store", headers: { "X-Sandbox-Poll": "1" } })
      .then(function (res) { return res.ok ? res.text() : null; })
      .then(function (html) {
        if (html === null) return;
        var doc = new DOMParser().parseFromString(html, "text/html");
        parts.forEach(function (el) {
          var fresh = doc.getElementById(el.id);
          if (!fresh || fresh.innerHTML === el.innerHTML) return;
          var open = Array.prototype.map.call(el.querySelectorAll("details[open][data-id]"), function (d) { return d.dataset.id; });
          var follow = el.dataset.poll === "end" && atEnd(el);
          el.innerHTML = fresh.innerHTML;
          open.forEach(function (id) {
            var d = el.querySelector('details[data-id="' + id + '"]');
            if (d) d.open = true;
          });
          if (follow) el.scrollTop = el.scrollHeight;
        });
      })
      .catch(function () { /* The sandbox stopped: try again next time. */ })
      .then(function () { busy = false; });
  }

  // data-copy="id" copies the text of the element with that id.
  document.addEventListener("click", function (e) {
    var btn = e.target.closest("[data-copy]");
    if (!btn || !navigator.clipboard) return;
    var src = document.getElementById(btn.dataset.copy);
    if (!src) return;
    navigator.clipboard.writeText(src.textContent).then(function () {
      var label = btn.textContent;
      btn.textContent = "Copied";
      setTimeout(function () { btn.textContent = label; }, 1500);
    });
  });

  scrollEnds();
  setInterval(refresh, 2000);
})();
