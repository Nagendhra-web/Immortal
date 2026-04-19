// command-palette.js — ⌘K palette.

import { h, debounce } from "../lib/fmt.js";
import { cmdk as registry } from "../lib/cmdk-registry.js";

let openDlg = null;

export function openCmdk() {
  if (openDlg) return;
  const input = h("input", {
    class: "cmdk__input",
    placeholder: "Search navigation, actions, services…",
    autocomplete: "off",
    spellcheck: "false",
    type: "text",
  });
  const list = h("ul", { class: "cmdk__list", role: "listbox" });
  const dlg = h("dialog", { class: "cmdk" },
    h("div", { class: "cmdk__input-row" },
      h("span", { class: "cmdk__input-row__icon" }, "⌕"),
      input,
      h("kbd", {}, "Esc"),
    ),
    list,
    h("div", { class: "cmdk__footer" },
      h("span", { class: "cmdk__footer__item" }, h("kbd", {}, "↑"), h("kbd", {}, "↓"), " navigate"),
      h("span", { class: "cmdk__footer__item" }, h("kbd", {}, "↵"), " run"),
      h("span", { class: "cmdk__footer__item" }, h("kbd", {}, "Esc"), " close"),
      h("span", { class: "cmdk__footer__item", style: { marginLeft: "auto" } }, `${registry.all().length} commands`),
    ),
  );
  document.body.appendChild(dlg);
  let results = [];
  let selected = 0;

  function render() {
    list.innerHTML = "";
    selected = Math.min(selected, results.length - 1);
    if (!results.length) {
      list.appendChild(h("div", { class: "cmdk__empty" }, "No matches"));
      return;
    }
    let lastGroup = null;
    results.forEach((c, i) => {
      const group = c.group || "Actions";
      if (group !== lastGroup) {
        list.appendChild(h("div", { class: "cmdk__group-label" }, group));
        lastGroup = group;
      }
      const li = h("li", {
        class: "cmdk__item",
        role: "option",
        "aria-selected": i === selected ? "true" : "false",
        dataset: { idx: i },
        onMouseenter: () => { selected = i; paintSel(); },
        onClick: () => runAt(i),
      },
        h("div", { class: "cmdk__item__title" },
          c.icon ? h("span", {}, c.icon) : null,
          c.title,
        ),
        c.shortcut ? h("div", { class: "cmdk__item__shortcut" },
          ...c.shortcut.map((k) => h("kbd", {}, k))
        ) : null,
      );
      list.appendChild(li);
    });
  }
  function paintSel() {
    Array.from(list.querySelectorAll(".cmdk__item")).forEach((el, i) => {
      el.setAttribute("aria-selected", String(i === selected));
      if (i === selected) el.scrollIntoView({ block: "nearest" });
    });
  }
  function runAt(i) {
    const c = results[i];
    if (!c) return;
    closeCmdk();
    registry.run(c);
  }
  function refresh() {
    results = registry.search(input.value.trim()).slice(0, 60);
    selected = 0;
    render();
  }

  input.addEventListener("input", debounce(refresh, 30));
  input.addEventListener("keydown", (e) => {
    if (e.key === "ArrowDown") { e.preventDefault(); selected = Math.min(results.length - 1, selected + 1); paintSel(); }
    if (e.key === "ArrowUp")   { e.preventDefault(); selected = Math.max(0, selected - 1); paintSel(); }
    if (e.key === "Enter")     { e.preventDefault(); runAt(selected); }
    if (e.key === "Escape")    { e.preventDefault(); closeCmdk(); }
  });
  dlg.addEventListener("click", (e) => { if (e.target === dlg) closeCmdk(); });
  dlg.addEventListener("close", cleanup);
  dlg.addEventListener("cancel", cleanup);

  refresh();
  dlg.showModal();
  setTimeout(() => input.focus(), 0);
  openDlg = dlg;

  function cleanup() { try { dlg.remove(); } catch {} openDlg = null; }
}

export function closeCmdk() {
  if (openDlg) { try { openDlg.close(); } catch {} openDlg = null; }
}
