// sheet.js — slide-in right panel for drill-downs.

import { h } from "../lib/fmt.js";

export function sheet({ title, subtitle, body, footer }) {
  const backdrop = h("div", { class: "sheet-backdrop" });
  const panel = h("aside", { class: "sheet", role: "dialog", "aria-modal": "true" },
    h("div", { class: "sheet__header" },
      h("div", {},
        h("div", { class: "sheet__title" }, title || ""),
        subtitle ? h("div", { class: "sheet__subtitle" }, subtitle) : null,
      ),
      h("button", {
        class: "btn btn--icon btn--sm",
        "aria-label": "Close",
        onClick: () => close(),
      }, "✕"),
    ),
    h("div", { class: "sheet__body" }, body || ""),
    footer ? h("div", { class: "sheet__footer" }, footer) : null,
  );
  const root = document.getElementById("overlay-root") || document.body;
  root.appendChild(backdrop);
  root.appendChild(panel);
  requestAnimationFrame(() => { backdrop.dataset.open = "true"; panel.dataset.open = "true"; });

  const onKey = (e) => { if (e.key === "Escape") close(); };
  backdrop.addEventListener("click", () => close());
  document.addEventListener("keydown", onKey);

  function close() {
    backdrop.dataset.open = "false";
    panel.dataset.open = "false";
    document.removeEventListener("keydown", onKey);
    setTimeout(() => { try { backdrop.remove(); panel.remove(); } catch {} }, 240);
  }
  return { close, panel };
}
