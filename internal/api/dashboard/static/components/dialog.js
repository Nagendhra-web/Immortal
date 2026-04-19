// dialog.js — thin wrapper around <dialog>.

import { h } from "../lib/fmt.js";

export function dialog({ title, body, footer }) {
  const dlg = h("dialog", { class: "dialog" },
    h("div", { class: "dialog__header" },
      h("div", { class: "dialog__title" }, title || ""),
      h("button", {
        class: "btn btn--icon btn--sm",
        "aria-label": "Close",
        onClick: () => close(),
      }, "✕"),
    ),
    h("div", { class: "dialog__body" }, body || ""),
    footer ? h("div", { class: "dialog__footer" }, footer) : null,
  );
  document.body.appendChild(dlg);
  dlg.addEventListener("click", (e) => {
    if (e.target === dlg) close(); // backdrop click
  });
  dlg.addEventListener("cancel", () => cleanup());
  dlg.addEventListener("close",  () => cleanup());

  function cleanup() { try { dlg.remove(); } catch {} }
  function close() { dlg.close(); }
  dlg.showModal();
  return { close, el: dlg };
}
