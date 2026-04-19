// keyboard.js — global keyboard shortcuts.

import { router } from "./router.js";
import { bus } from "./bus.js";
import { cmdk as cmdkReg } from "./cmdk-registry.js";

const seq = { last: 0, first: null };

export function installGlobalShortcuts({ openCmdk, openHelp }) {
  window.addEventListener("keydown", (e) => {
    // Ignore when typing in inputs (unless modifier+K).
    const typing = ["INPUT", "TEXTAREA"].includes(document.activeElement?.tagName)
                || document.activeElement?.isContentEditable;

    // ⌘K / Ctrl-K
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
      e.preventDefault();
      openCmdk();
      return;
    }

    if (typing) return;

    // g <key> navigation combos
    if (e.key === "g") { seq.first = "g"; seq.last = Date.now(); return; }
    if (seq.first === "g" && Date.now() - seq.last < 1200) {
      const map = {
        o: "/overview", t: "/topology", a: "/audit", T: "/terminal",
        w: "/twin", A: "/agentic", f: "/formal", c: "/causal",
        n: "/planner/nl", e: "/planner/economic",
        F: "/federation", x: "/certificates",
      };
      if (map[e.key]) { e.preventDefault(); router.go(map[e.key]); seq.first = null; return; }
    }

    if (e.key === "?") { e.preventDefault(); openHelp(); return; }
    if (e.key === "/") {
      const input = document.querySelector("[data-role=view-filter]");
      if (input) { e.preventDefault(); input.focus(); }
      return;
    }
    if (e.key === ".") {
      const cur = bus.get("app:paused") || false;
      bus.emit("app:paused", !cur);
      return;
    }
  });

  // Expose registry under window for ad-hoc debugging.
  window.__immortal_cmdk = cmdkReg;
}

export const HELP_SHORTCUTS = [
  ["⌘K / Ctrl-K", "Command palette"],
  ["g o", "Go: Overview"],
  ["g t", "Go: Topology"],
  ["g a", "Go: Audit"],
  ["g ⇧T", "Go: Terminal"],
  ["g w", "Go: Twin forecasts"],
  ["g ⇧A", "Go: Agentic traces"],
  ["g f", "Go: Formal check"],
  ["g c", "Go: Causal root-cause"],
  ["g n", "Go: NL → Plan"],
  ["g e", "Go: Economic planner"],
  ["g ⇧F", "Go: Federation"],
  ["g x", "Go: Certificates"],
  ["/", "Focus filter"],
  [".", "Toggle live/paused"],
  ["?", "Keyboard help"],
  ["Esc", "Close overlay"],
];
