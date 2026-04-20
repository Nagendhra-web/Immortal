// _shell.js — sidebar + topbar renderers.

import { h } from "../lib/fmt.js";
import { bus, loadState, saveState } from "../lib/bus.js";
import { router } from "../lib/router.js";
import { popover } from "../components/popover.js";

export const NAV_GROUPS = [
  {
    label: "Mission Control",
    items: [
      { path: "/overview", label: "Overview",   icon: "◉" },
      { path: "/topology", label: "Topology",   icon: "◇" },
      { path: "/audit",    label: "Audit",      icon: "⎈" },
      { path: "/terminal", label: "Terminal",   icon: "▶" },
    ],
  },
  {
    label: "Intelligence",
    items: [
      { path: "/twin",     label: "Twin Forecasts", icon: "~" },
      { path: "/agentic",  label: "Agentic Traces", icon: "⟲" },
      { path: "/causal",   label: "Causal RCA",     icon: "⇄" },
      { path: "/formal",   label: "Formal Check",   icon: "⊢" },
    ],
  },
  {
    label: "Authoring",
    items: [
      { path: "/intent",           label: "Intents",           icon: "◆" },
      { path: "/evolve",           label: "Architecture",      icon: "✺" },
      { path: "/planner/nl",       label: "NL → Plan",         icon: "✎" },
      { path: "/planner/economic", label: "Economic Planner",  icon: "$" },
    ],
  },
  {
    label: "Knowledge",
    items: [
      { path: "/federation",   label: "Federation",     icon: "◎" },
      { path: "/certificates", label: "Certificates",   icon: "◈" },
    ],
  },
];

export function renderSidebar(root, { onOpenCmdk }) {
  const nav = h("nav", { class: "sidebar__nav", role: "navigation", "aria-label": "Primary" });

  NAV_GROUPS.forEach((g) => {
    const group = h("div", { class: "sidebar__group" });
    group.appendChild(h("div", { class: "sidebar__group-label" }, g.label));
    g.items.forEach((item) => {
      const a = h("a", {
        class: "sidebar__item",
        href: "#" + item.path,
        dataset: { path: item.path },
      },
        h("span", { class: "sidebar__item__icon" }, item.icon),
        h("span", {}, item.label),
      );
      group.appendChild(a);
    });
    nav.appendChild(group);
  });

  root.innerHTML = "";
  root.appendChild(h("div", { class: "sidebar__brand" },
    h("div", { class: "sidebar__brand__logo" }, "I"),
    h("div", {},
      h("div", { class: "sidebar__brand__name" }, "Immortal"),
      h("div", { class: "sidebar__brand__meta" }, "self-healing engine"),
    ),
  ));
  root.appendChild(nav);
  root.appendChild(h("div", { class: "sidebar__footer" },
    h("span", {}, "v0.5.0"),
    h("button", {
      class: "sidebar__footer__cmdk",
      type: "button",
      onClick: onOpenCmdk,
    },
      "⌕",
      h("kbd", {}, "⌘K"),
    ),
  ));

  function paint(path) {
    root.querySelectorAll(".sidebar__item").forEach((el) => {
      const p = el.dataset.path;
      if (path === p || (path && path.startsWith(p + "/"))) el.setAttribute("aria-current", "page");
      else el.removeAttribute("aria-current");
    });
  }
  paint(router.current() || "/overview");
  document.addEventListener("route:change", (e) => paint(e.detail.path));
}

const TIME_PRESETS = [
  { value: "5m",  label: "Last 5 min" },
  { value: "1h",  label: "Last hour" },
  { value: "6h",  label: "Last 6 hours" },
  { value: "24h", label: "Last 24 hours" },
  { value: "7d",  label: "Last 7 days" },
];

export function renderTopbar(root, { onOpenCmdk }) {
  const state = loadState();
  bus.emit("app:range", presetToRange(state.range.preset));
  bus.emit("app:env",    state.env);
  bus.emit("app:paused", state.paused);

  root.innerHTML = "";
  const crumb   = h("div", { class: "topbar__breadcrumb" },
    h("span", {}, "Immortal"),
    h("span", {}, "›"),
    h("span", { class: "topbar__breadcrumb__cur", id: "crumb-cur" }, "Overview"),
  );

  const status  = h("span", { class: "topbar__status", id: "live-status" },
    h("span", { class: "dot dot--live" }), "LIVE",
  );

  const rangeBtn = h("button", {
    class: "select",
    type: "button",
    "aria-haspopup": "listbox",
    onClick: (e) => openRangePopover(e.currentTarget),
  },
    h("span", { class: "select__label" }, "Range"),
    h("span", { class: "select__value", id: "range-val" }, presetLabel(state.range.preset)),
    h("span", { class: "select__chevron" }, "▾"),
  );

  const envBtn = h("button", {
    class: "select",
    type: "button",
    onClick: (e) => openEnvPopover(e.currentTarget),
  },
    h("span", { class: "select__label" }, "Env"),
    h("span", { class: "select__value", id: "env-val" }, state.env),
    h("span", { class: "select__chevron" }, "▾"),
  );

  const pauseBtn = h("button", {
    class: "btn btn--secondary btn--sm",
    "aria-pressed": state.paused ? "true" : "false",
    onClick: () => {
      const cur = bus.get("app:paused") || false;
      bus.emit("app:paused", !cur);
    },
    "data-tooltip": "Pause live updates (.)",
  }, state.paused ? "▶ Resume" : "❚❚ Pause");

  const cmdkBtn = h("button", {
    class: "btn btn--secondary btn--sm",
    onClick: onOpenCmdk,
    "data-tooltip": "Command palette (⌘K)",
  }, "⌕ Search", h("kbd", { style: { marginLeft: "6px" } }, "⌘K"));

  const help = h("button", {
    class: "btn btn--icon btn--sm",
    "aria-label": "Keyboard help",
    "data-tooltip": "Keyboard shortcuts (?)",
    onClick: () => import("./_help.js").then((m) => m.openHelp()),
  }, "?");

  root.appendChild(crumb);
  root.appendChild(h("div", { class: "topbar__spacer" }));
  root.appendChild(h("div", { class: "topbar__controls" },
    status, rangeBtn, envBtn, pauseBtn, cmdkBtn, help,
  ));

  // React to state changes.
  bus.on("app:paused", (p) => {
    pauseBtn.setAttribute("aria-pressed", p ? "true" : "false");
    pauseBtn.textContent = p ? "▶ Resume" : "❚❚ Pause";
    status.className = "topbar__status" + (p ? " topbar__status--paused" : "");
    status.innerHTML = "";
    status.appendChild(h("span", { class: p ? "dot" : "dot dot--live" }));
    status.appendChild(document.createTextNode(p ? " PAUSED" : " LIVE"));
    const s = loadState(); s.paused = p; saveState(s);
  });
  bus.on("app:range", (range) => {
    document.getElementById("range-val").textContent = presetLabel(range.preset);
  });

  document.addEventListener("route:change", (e) => {
    const cur = navLabel(e.detail.path);
    document.getElementById("crumb-cur").textContent = cur;
    document.title = `Immortal — ${cur}`;
  });

  function openRangePopover(anchor) {
    const list = h("ul", { class: "popover__list", role: "listbox" },
      ...TIME_PRESETS.map((p) => h("li", {
        class: "popover__item",
        role: "option",
        "aria-selected": p.value === (bus.get("app:range")?.preset) ? "true" : "false",
        onClick: () => { setRange(p.value); pop.close(); },
      }, p.label))
    );
    const pop = popover({ anchor, content: list });
  }
  function openEnvPopover(anchor) {
    const envs = ["prod", "staging", "local"];
    const list = h("ul", { class: "popover__list", role: "listbox" },
      ...envs.map((e) => h("li", {
        class: "popover__item",
        role: "option",
        "aria-selected": e === bus.get("app:env") ? "true" : "false",
        onClick: () => { setEnv(e); pop.close(); },
      }, e))
    );
    const pop = popover({ anchor, content: list });
  }
}

function setRange(preset) {
  const r = presetToRange(preset);
  bus.emit("app:range", r);
  const s = loadState(); s.range = { preset, ...r }; saveState(s);
}
function setEnv(env) {
  bus.emit("app:env", env);
  document.getElementById("env-val").textContent = env;
  const s = loadState(); s.env = env; saveState(s);
}

function presetLabel(preset) {
  const p = TIME_PRESETS.find((x) => x.value === preset);
  return p ? p.label : preset;
}
function presetToRange(preset) {
  const now = Date.now();
  const map = { "5m": 5*60e3, "1h": 60*60e3, "6h": 6*60*60e3, "24h": 24*60*60e3, "7d": 7*24*60*60e3 };
  const ms = map[preset] ?? 60*60e3;
  return { preset, t0: now - ms, t1: now };
}
function navLabel(path) {
  for (const g of NAV_GROUPS) for (const i of g.items) if (path === i.path || path.startsWith(i.path + "/")) return i.label;
  return "Dashboard";
}
