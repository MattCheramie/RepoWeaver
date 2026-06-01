// Self-contained analytics chart renderer (no external dependency).
//
// NOTE: the original plan called for Chart.js, but this build environment's
// network allowlist cannot fetch it (CDNs are blocked and Chart.js does not
// commit its built bundle to GitHub). This lightweight Canvas renderer provides
// the same horizontal-bar pageview visualization and can be swapped for Chart.js
// later: feed the same window.__repoweaverChart data to a Chart() instance on
// the #views-chart canvas.

(function () {
  "use strict";

  function readData() {
    var d = window.__repoweaverChart;
    return Array.isArray(d) ? d : [];
  }

  function cssVar(name, fallback) {
    var v = getComputedStyle(document.documentElement).getPropertyValue(name);
    return (v && v.trim()) || fallback;
  }

  function draw() {
    var canvas = document.getElementById("views-chart");
    if (!canvas || !canvas.getContext) return;
    var data = readData();
    if (!data.length) return;

    var ctx = canvas.getContext("2d");
    var dpr = window.devicePixelRatio || 1;

    var rowH = 34;
    var padL = 8;
    var padR = 56;
    var labelW = 200;
    var cssWidth = canvas.parentElement.clientWidth || 720;
    var cssHeight = data.length * rowH + 12;

    // High-DPI crisp rendering.
    canvas.width = cssWidth * dpr;
    canvas.height = cssHeight * dpr;
    canvas.style.width = cssWidth + "px";
    canvas.style.height = cssHeight + "px";
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, cssWidth, cssHeight);

    var maxViews = data.reduce(function (m, d) {
      return Math.max(m, d.views || 0);
    }, 1);

    var barArea = cssWidth - labelW - padL - padR;
    var accent = cssVar("--accent", "#7c9cff");
    var good = cssVar("--good", "#3fb950");
    var warn = cssVar("--warn", "#d29922");
    var text = cssVar("--text", "#e6e9ef");
    var muted = cssVar("--muted", "#9aa3b2");
    ctx.font = "13px -apple-system, Segoe UI, Roboto, sans-serif";
    ctx.textBaseline = "middle";

    data.forEach(function (d, i) {
      var y = i * rowH + rowH / 2 + 6;

      // Truncated label.
      var label = d.label || "";
      while (ctx.measureText(label).width > labelW - 12 && label.length > 1) {
        label = label.slice(0, -2);
      }
      if (label !== (d.label || "")) label += "…";
      ctx.fillStyle = text;
      ctx.textAlign = "left";
      ctx.fillText(label, padL, y);

      // Bar, coloured by bounce rate (lower is better).
      var w = Math.max(2, (d.views / maxViews) * barArea);
      var bounce = d.bounce || 0;
      ctx.fillStyle = bounce > 0.6 ? warn : bounce < 0.4 ? good : accent;
      var bx = padL + labelW;
      roundRect(ctx, bx, y - 9, w, 18, 4);
      ctx.fill();

      // Value annotation.
      ctx.fillStyle = muted;
      ctx.textAlign = "left";
      ctx.fillText(String(d.views), bx + w + 8, y);
    });
  }

  function roundRect(ctx, x, y, w, h, r) {
    r = Math.min(r, h / 2, w / 2);
    ctx.beginPath();
    ctx.moveTo(x + r, y);
    ctx.arcTo(x + w, y, x + w, y + h, r);
    ctx.arcTo(x + w, y + h, x, y + h, r);
    ctx.arcTo(x, y + h, x, y, r);
    ctx.arcTo(x, y, x + w, y, r);
    ctx.closePath();
  }

  var raf;
  function schedule() {
    cancelAnimationFrame(raf);
    raf = requestAnimationFrame(draw);
  }

  if (document.readyState !== "loading") schedule();
  else document.addEventListener("DOMContentLoaded", schedule);
  window.addEventListener("resize", schedule);
})();
