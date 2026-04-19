// fmt.js — formatting helpers.

export function h(tag, attrs = {}, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v === null || v === undefined || v === false) continue;
    if (k === "class" || k === "className") el.className = v;
    else if (k === "style" && typeof v === "object") Object.assign(el.style, v);
    else if (k === "html") el.innerHTML = v;
    else if (k.startsWith("on") && typeof v === "function") el.addEventListener(k.slice(2).toLowerCase(), v);
    else if (k === "dataset") { for (const [dk, dv] of Object.entries(v)) el.dataset[dk] = dv; }
    else if (v === true) el.setAttribute(k, "");
    else el.setAttribute(k, v);
  }
  for (const c of children.flat(Infinity)) {
    if (c === null || c === undefined || c === false) continue;
    if (c instanceof Node) el.appendChild(c);
    else el.appendChild(document.createTextNode(String(c)));
  }
  return el;
}

export function escapeHtml(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  }[c]));
}

export function num(n, digits = 2) {
  if (n === null || n === undefined || Number.isNaN(n)) return "—";
  const abs = Math.abs(n);
  if (abs >= 1e9) return (n / 1e9).toFixed(digits) + "B";
  if (abs >= 1e6) return (n / 1e6).toFixed(digits) + "M";
  if (abs >= 1e3) return (n / 1e3).toFixed(digits) + "k";
  if (abs < 0.01 && abs > 0) return n.toExponential(1);
  return Number(n).toFixed(digits).replace(/\.?0+$/, "");
}

export function pct(n, digits = 1) {
  if (n === null || n === undefined || Number.isNaN(n)) return "—";
  return (n * 100).toFixed(digits) + "%";
}

export function ms(n) {
  if (n === null || n === undefined || Number.isNaN(n)) return "—";
  if (n < 1) return (n * 1000).toFixed(0) + "µs";
  if (n < 1000) return n.toFixed(0) + "ms";
  return (n / 1000).toFixed(2) + "s";
}

export function bytes(n) {
  if (n === null || n === undefined) return "—";
  const u = ["B","KB","MB","GB","TB"];
  let i = 0; let v = n;
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++; }
  return v.toFixed(v < 10 ? 1 : 0) + u[i];
}

export function relTime(tsMs) {
  if (!tsMs) return "—";
  const diff = (Date.now() - tsMs) / 1000;
  if (diff < 5) return "just now";
  if (diff < 60) return Math.floor(diff) + "s ago";
  if (diff < 3600) return Math.floor(diff / 60) + "m ago";
  if (diff < 86400) return Math.floor(diff / 3600) + "h ago";
  return Math.floor(diff / 86400) + "d ago";
}

export function dateShort(tsMs) {
  if (!tsMs) return "—";
  const d = new Date(tsMs);
  return d.toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
}

export function clamp(n, lo, hi) { return Math.max(lo, Math.min(hi, n)); }

export function debounce(fn, delay = 120) {
  let t = null;
  return (...args) => {
    clearTimeout(t);
    t = setTimeout(() => fn(...args), delay);
  };
}

export function throttle(fn, interval = 100) {
  let last = 0; let timer = null;
  return (...args) => {
    const now = Date.now();
    const rem = interval - (now - last);
    if (rem <= 0) { last = now; fn(...args); }
    else { clearTimeout(timer); timer = setTimeout(() => { last = Date.now(); fn(...args); }, rem); }
  };
}

// Accessible color for a string (stable hash).
export function hueFor(s) {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) % 360;
  return h;
}

// Get computed color token from :root.
export function token(name) {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
}
