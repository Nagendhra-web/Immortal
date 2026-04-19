// router.js — hash router. URL is the source of truth.
// Register: router.register(pattern, mount). mount(root, ctx) returns unmount().
// Navigate:  router.go("/twin") or router.go("/twin?service=rest")

const routes = [];
let currentUnmount = null;
let currentPath = null;
let rootEl = null;

export const router = {
  register(pattern, mount) {
    routes.push({ pattern, mount });
  },
  start(el) {
    rootEl = el;
    window.addEventListener("hashchange", () => this.resolve());
    this.resolve();
  },
  go(path) {
    const next = path.startsWith("#") ? path : `#${path}`;
    if (location.hash === next) { this.resolve(); return; }
    location.hash = next;
  },
  current() { return currentPath; },
  resolve() {
    const hash = location.hash.replace(/^#/, "") || "/overview";
    const [path, query = ""] = hash.split("?");
    const params = Object.fromEntries(new URLSearchParams(query));
    const match = routes.find((r) => pathMatches(r.pattern, path));
    if (currentUnmount) { try { currentUnmount(); } catch {} currentUnmount = null; }
    rootEl.innerHTML = "";
    currentPath = path;
    if (!match) {
      rootEl.innerHTML = `<div class="empty"><div class="empty__title">Not found</div><div>${path}</div></div>`;
      return;
    }
    try {
      const unmount = match.mount(rootEl, { path, params });
      if (typeof unmount === "function") currentUnmount = unmount;
    } catch (e) {
      console.error("view mount failed", e);
      rootEl.innerHTML = `<div class="empty"><div class="empty__title">View error</div><pre class="mono" style="white-space:pre-wrap">${escapeHtml(String(e && e.stack || e))}</pre></div>`;
    }
    document.dispatchEvent(new CustomEvent("route:change", { detail: { path, params } }));
  },
};

function pathMatches(pattern, path) {
  // Supports exact match + suffix wildcard "/foo/*".
  if (pattern === path) return true;
  if (pattern.endsWith("/*")) {
    return path.startsWith(pattern.slice(0, -2));
  }
  return false;
}

function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, (c) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  }[c]));
}
