// toast.js — push transient notifications.

import { h } from "../lib/fmt.js";

let region;
function getRegion() {
  if (region) return region;
  region = document.getElementById("toast-region") || (() => {
    const r = document.createElement("ol");
    r.id = "toast-region"; r.className = "toast-region";
    r.setAttribute("aria-live", "polite");
    document.body.appendChild(r);
    return r;
  })();
  return region;
}

const ICONS = { ok: "✓", warn: "!", err: "✕", info: "i" };

export const toast = {
  push({ level = "info", title, msg, ttl = 4500 } = {}) {
    const r = getRegion();
    const el = h("li", { class: `toast toast--${level}`, role: "status" },
      h("div", { class: "toast__icon" }, ICONS[level] || "i"),
      h("div", { class: "toast__body" },
        title ? h("div", { class: "toast__title" }, title) : null,
        msg   ? h("div", { class: "toast__msg" },   msg)   : null,
      ),
      h("button", {
        class: "toast__close",
        "aria-label": "Dismiss",
        onClick: () => dismiss(el),
      }, "✕"),
    );
    r.appendChild(el);
    let timer = ttl > 0 ? setTimeout(() => dismiss(el), ttl) : null;
    el.addEventListener("mouseenter", () => { if (timer) { clearTimeout(timer); timer = null; } });
    el.addEventListener("mouseleave", () => { if (ttl > 0 && !timer) timer = setTimeout(() => dismiss(el), 1500); });
    return el;
  },
  ok(title, msg)   { return this.push({ level: "ok",   title, msg }); },
  warn(title, msg) { return this.push({ level: "warn", title, msg }); },
  err(title, msg)  { return this.push({ level: "err",  title, msg, ttl: 6500 }); },
  info(title, msg) { return this.push({ level: "info", title, msg }); },
};

function dismiss(el) {
  if (!el.isConnected) return;
  el.dataset.leaving = "true";
  setTimeout(() => { try { el.remove(); } catch {} }, 220);
}
