// popover.js — manually positioned floating panel (no deps).

import { h } from "../lib/fmt.js";

let openPop = null;

export function popover({ anchor, content, placement = "bottom-start", onClose } = {}) {
  closeCurrent();
  const el = h("div", { class: "popover", role: "dialog" }, content);
  document.body.appendChild(el);
  position(el, anchor, placement);
  // show next tick to allow transition.
  requestAnimationFrame(() => el.dataset.open = "true");

  const onDocClick = (e) => {
    if (el.contains(e.target) || anchor.contains(e.target)) return;
    close();
  };
  const onKey = (e) => { if (e.key === "Escape") close(); };
  const onScroll = () => position(el, anchor, placement);
  document.addEventListener("mousedown", onDocClick);
  document.addEventListener("keydown",    onKey);
  window.addEventListener("scroll",       onScroll, true);
  window.addEventListener("resize",       onScroll);

  function close() {
    el.dataset.open = "false";
    setTimeout(() => { try { el.remove(); } catch {} }, 160);
    document.removeEventListener("mousedown", onDocClick);
    document.removeEventListener("keydown",    onKey);
    window.removeEventListener("scroll",       onScroll, true);
    window.removeEventListener("resize",       onScroll);
    openPop = null;
    onClose && onClose();
  }
  openPop = { close };
  return { el, close };
}

export function closeCurrent() { if (openPop) openPop.close(); }

function position(el, anchor, placement) {
  const r = anchor.getBoundingClientRect();
  const er = el.getBoundingClientRect();
  let x = r.left, y = r.bottom + 4;
  if (placement === "bottom-end")   x = r.right - er.width;
  if (placement === "top-start")    y = r.top - er.height - 4;
  if (placement === "right-start") { x = r.right + 4; y = r.top; }
  // Clamp.
  x = Math.max(8, Math.min(window.innerWidth  - er.width  - 8, x));
  y = Math.max(8, Math.min(window.innerHeight - er.height - 8, y));
  el.style.left = x + "px";
  el.style.top  = y + "px";
}
