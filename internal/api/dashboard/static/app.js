// app.js — bootstrap. Registers routes, wires topbar + sidebar + keyboard.

import { router }       from "./lib/router.js";
import { bus, loadState } from "./lib/bus.js";
import { cmdk }          from "./lib/cmdk-registry.js";
import { installGlobalShortcuts } from "./lib/keyboard.js";
import { installTooltips } from "./components/tooltip.js";
import { openCmdk }      from "./components/command-palette.js";
import { renderSidebar, renderTopbar, NAV_GROUPS } from "./views/_shell.js";
import { openHelp }      from "./views/_help.js";

// View modules — dynamic import to keep cold-start small.
const routes = {
  "/overview":         () => import("./views/overview.js"),
  "/topology":         () => import("./views/topology.js"),
  "/audit":            () => import("./views/audit.js"),
  "/terminal":         () => import("./views/terminal.js"),
  "/twin":             () => import("./views/twin.js"),
  "/agentic":          () => import("./views/agentic.js"),
  "/causal":           () => import("./views/causal.js"),
  "/formal":           () => import("./views/formal.js"),
  "/intent":           () => import("./views/intent.js"),
  "/evolve":           () => import("./views/evolve.js"),
  "/planner/nl":       () => import("./views/planner-nl.js"),
  "/planner/economic": () => import("./views/planner-economic.js"),
  "/federation":       () => import("./views/federation.js"),
  "/certificates":     () => import("./views/certificates.js"),
};

for (const [path, load] of Object.entries(routes)) {
  router.register(path, (root, ctx) => {
    const busy = document.createElement("div");
    busy.className = "empty";
    busy.innerHTML = `<div class="skeleton" style="width:48px;height:48px;border-radius:50%"></div><div class="subtle">Loading…</div>`;
    root.appendChild(busy);
    let unmount = null;
    load().then((mod) => {
      root.innerHTML = "";
      const fn = mod.mount || mod.default;
      if (typeof fn !== "function") { root.innerHTML = `<div class="empty"><div class="empty__title">Bad view module</div><div>${path}</div></div>`; return; }
      unmount = fn(root, ctx) || null;
    }).catch((err) => {
      root.innerHTML = `<div class="empty"><div class="empty__title">Failed to load</div><pre class="mono">${String(err)}</pre></div>`;
    });
    return () => { if (typeof unmount === "function") unmount(); };
  });
}

// Global nav commands.
for (const g of NAV_GROUPS) {
  for (const it of g.items) {
    cmdk.register({
      id: `nav:${it.path}`,
      group: "Navigation",
      title: `Go to ${it.label}`,
      keywords: [g.label, it.path],
      run: () => router.go(it.path),
    });
  }
}
cmdk.register({ id: "app:help", group: "Actions", title: "Keyboard shortcuts", run: () => openHelp() });
cmdk.register({ id: "app:pause", group: "Actions", title: "Toggle live / paused",
  run: () => bus.emit("app:paused", !(bus.get("app:paused") || false)) });

// Time range commands.
for (const p of [["5m","Last 5 min"], ["1h","Last hour"], ["6h","Last 6 hours"], ["24h","Last 24 hours"], ["7d","Last 7 days"]]) {
  cmdk.register({
    id: `range:${p[0]}`, group: "Time range", title: `Set range: ${p[1]}`, keywords: [p[0]],
    run: () => {
      const now = Date.now();
      const map = { "5m": 5*60e3, "1h": 60*60e3, "6h": 6*60*60e3, "24h": 24*60*60e3, "7d": 7*24*60*60e3 };
      bus.emit("app:range", { preset: p[0], t0: now - map[p[0]], t1: now });
      const s = loadState(); s.range.preset = p[0]; try { localStorage.setItem("immortal:ui:state", JSON.stringify(s)); } catch {}
    },
  });
}

// Boot order matters: shell first, then router.
renderSidebar(document.getElementById("sidebar"), { onOpenCmdk: openCmdk });
renderTopbar(document.getElementById("topbar"),  { onOpenCmdk: openCmdk });
installTooltips();
installGlobalShortcuts({ openCmdk, openHelp });
router.start(document.getElementById("main"));

// Expose for ad-hoc inspection in devtools.
window.immortal = { router, bus, cmdk, openCmdk };
