// Editorial calendar drag-and-drop. Posts schedule changes and swaps in the
// re-rendered calendar fragment returned by the server.

function calDrag(ev) {
  ev.dataTransfer.setData("text/plain", ev.target.dataset.id);
  ev.dataTransfer.effectAllowed = "move";
}

function calAllow(ev) {
  ev.preventDefault();
  ev.dataTransfer.dropEffect = "move";
}

function calDrop(ev) {
  ev.preventDefault();
  const id = ev.dataTransfer.getData("text/plain");
  if (!id) return;
  // The drop target may be a child element; climb to the cell/aside.
  const cell = ev.target.closest("[data-date]");
  if (!cell) return;
  const date = cell.dataset.date || ""; // empty => unschedule
  const root = document.getElementById("calendar-root");
  const month = root ? root.dataset.month : "";

  const body = new URLSearchParams({ date: date, month: month });
  fetch(`/content/${id}/schedule`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: body.toString(),
  })
    .then((r) => r.text())
    .then((html) => {
      if (root) {
        root.innerHTML = html;
        if (window.htmx) window.htmx.process(root);
      }
    })
    .catch(() => {
      const s = document.getElementById("cal-status");
      if (s) {
        s.textContent = "Could not update schedule.";
        s.className = "hint error";
      }
    });
}
