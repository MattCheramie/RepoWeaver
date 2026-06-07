// Preview-only client renderers. The hero banner and charts are pre-rendered as
// inline SVG on the server, so this script only initializes the libraries that
// must run in the browser: Mermaid for ```mermaid diagrams and (optionally) D3
// for ```d3 blocks. Everything degrades gracefully when a vendored bundle is
// absent — the original diagram source stays visible.
(function () {
  "use strict";

  function initMermaid() {
    if (!window.mermaid || typeof window.mermaid.initialize !== "function") return;
    try {
      window.mermaid.initialize({ startOnLoad: false, theme: "dark", securityLevel: "loose" });
      if (typeof window.mermaid.run === "function") {
        window.mermaid.run({ querySelector: ".mermaid" });
      } else if (typeof window.mermaid.init === "function") {
        window.mermaid.init(undefined, document.querySelectorAll(".mermaid"));
      }
    } catch (e) {
      // Leave the raw diagram source visible on failure.
    }
  }

  function initD3() {
    var nodes = document.querySelectorAll(".rw-d3");
    if (!nodes.length || !window.d3 || typeof window.rwRenderD3 !== "function") return;
    nodes.forEach(function (el) {
      try {
        window.rwRenderD3(el, el.getAttribute("data-spec") || "");
      } catch (e) {
        /* leave the container empty */
      }
    });
  }

  function init() {
    initMermaid();
    initD3();
  }

  if (document.readyState !== "loading") init();
  else document.addEventListener("DOMContentLoaded", init);
})();
