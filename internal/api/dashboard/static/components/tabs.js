// tabs.js — ARIA tablist.
// tabs.init(el, { onChange }) or auto-wire [data-role=tabs].

import { h } from "../lib/fmt.js";

export function tabsInit(el, { onChange } = {}) {
  const list = el.querySelector("[role=tablist]");
  const tabs = Array.from(el.querySelectorAll("[role=tab]"));
  const panels = Array.from(el.querySelectorAll("[role=tabpanel]"));

  function activate(idx, { focus = false } = {}) {
    tabs.forEach((t, i) => {
      const on = i === idx;
      t.setAttribute("aria-selected", on ? "true" : "false");
      t.tabIndex = on ? 0 : -1;
    });
    panels.forEach((p, i) => p.dataset.active = i === idx ? "true" : "false");
    if (focus) tabs[idx].focus();
    onChange && onChange(idx, tabs[idx].dataset.value || idx);
  }

  list.addEventListener("keydown", (e) => {
    const idx = tabs.indexOf(document.activeElement);
    if (idx < 0) return;
    if (e.key === "ArrowRight" || e.key === "ArrowDown") { e.preventDefault(); activate((idx + 1) % tabs.length, { focus: true }); }
    if (e.key === "ArrowLeft"  || e.key === "ArrowUp")   { e.preventDefault(); activate((idx - 1 + tabs.length) % tabs.length, { focus: true }); }
    if (e.key === "Home") { e.preventDefault(); activate(0, { focus: true }); }
    if (e.key === "End")  { e.preventDefault(); activate(tabs.length - 1, { focus: true }); }
  });
  tabs.forEach((t, i) => t.addEventListener("click", () => activate(i)));

  const initial = tabs.findIndex((t) => t.getAttribute("aria-selected") === "true");
  activate(initial < 0 ? 0 : initial);

  return { activate };
}

// Convenience builder — returns a wired element.
export function buildTabs({ tabs, panels, active = 0, onChange } = {}) {
  const id = "t" + Math.random().toString(36).slice(2, 8);
  const root = h("div", { class: "tabs", role: "tabs" });
  const list = h("div", { class: "tabs__list", role: "tablist" });
  tabs.forEach((t, i) => {
    list.appendChild(h("button", {
      role: "tab",
      id: `${id}-tab-${i}`,
      "aria-controls": `${id}-panel-${i}`,
      "aria-selected": i === active ? "true" : "false",
      tabIndex: i === active ? 0 : -1,
      class: "tabs__tab",
      dataset: { value: t.value ?? i },
    }, t.label));
  });
  root.appendChild(list);
  panels.forEach((p, i) => {
    root.appendChild(h("div", {
      role: "tabpanel",
      id: `${id}-panel-${i}`,
      "aria-labelledby": `${id}-tab-${i}`,
      class: "tabs__panel",
      dataset: { active: i === active ? "true" : "false" },
    }, p));
  });
  tabsInit(root, { onChange });
  return root;
}
