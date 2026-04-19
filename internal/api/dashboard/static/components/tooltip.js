// tooltip.js — attach to elements with [data-tooltip="...text..."].

let tip;
let target = null;
let showTimer = null;

function ensure() {
  if (tip) return tip;
  tip = document.createElement("div");
  tip.className = "tooltip";
  tip.setAttribute("role", "tooltip");
  tip.dataset.visible = "false";
  document.body.appendChild(tip);
  return tip;
}

function place(el) {
  ensure();
  const r = el.getBoundingClientRect();
  const pad = 8;
  tip.dataset.visible = "true";
  // Measure after making visible; fallback: assume 40x24.
  const tr = tip.getBoundingClientRect();
  let x = r.left + r.width / 2 - tr.width / 2;
  let y = r.top - tr.height - pad;
  if (y < 4) y = r.bottom + pad;
  x = Math.max(4, Math.min(window.innerWidth - tr.width - 4, x));
  tip.style.left = x + "px";
  tip.style.top  = y + "px";
}

function show(el) {
  const text = el.getAttribute("data-tooltip");
  if (!text) return;
  ensure();
  tip.textContent = text;
  place(el);
}

function hide() {
  if (tip) tip.dataset.visible = "false";
  target = null;
}

export function installTooltips() {
  document.addEventListener("mouseover", (e) => {
    const el = e.target.closest?.("[data-tooltip]");
    if (!el || el === target) return;
    target = el;
    clearTimeout(showTimer);
    showTimer = setTimeout(() => show(el), 400);
  });
  document.addEventListener("mouseout", (e) => {
    const el = e.target.closest?.("[data-tooltip]");
    if (el && el === target) {
      clearTimeout(showTimer);
      hide();
    }
  });
  document.addEventListener("focusin", (e) => {
    const el = e.target.closest?.("[data-tooltip]");
    if (el) { target = el; show(el); }
  });
  document.addEventListener("focusout", () => hide());
  window.addEventListener("scroll", () => hide(), true);
}
