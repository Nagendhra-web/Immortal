// table.js — build a sortable, keyboard-navigable table.
// table({ columns, rows, onRowClick, rowKey, emptyText })

import { h } from "../lib/fmt.js";

export function table({ columns, rows, onRowClick, rowKey, emptyText = "No rows" } = {}) {
  const wrap = h("div", { class: "table-wrap" });
  const tbl = h("table", { class: "table" });
  const thead = h("thead");
  const trh = h("tr");
  const state = { sortIdx: -1, sortDir: "asc", rows: rows || [] };

  columns.forEach((c, i) => {
    const th = h("th", {
      dataset: c.sortable === false ? {} : { sortable: "true" },
      style: c.width ? { width: c.width } : undefined,
      onClick: c.sortable === false ? undefined : () => sortBy(i),
    }, c.label);
    trh.appendChild(th);
  });
  thead.appendChild(trh);
  tbl.appendChild(thead);
  const tbody = h("tbody");
  tbl.appendChild(tbody);
  wrap.appendChild(tbl);
  render();

  function sortBy(i) {
    if (state.sortIdx === i) state.sortDir = state.sortDir === "asc" ? "desc" : "asc";
    else { state.sortIdx = i; state.sortDir = "asc"; }
    Array.from(trh.children).forEach((th, idx) => {
      if (idx === i) th.dataset.sort = state.sortDir;
      else delete th.dataset.sort;
    });
    render();
  }

  function sorted() {
    const rows = [...state.rows];
    if (state.sortIdx < 0) return rows;
    const col = columns[state.sortIdx];
    const key = col.value || ((r) => r[col.key]);
    rows.sort((a, b) => {
      const av = key(a), bv = key(b);
      if (av == null && bv == null) return 0;
      if (av == null) return 1;
      if (bv == null) return -1;
      if (typeof av === "number" && typeof bv === "number") return state.sortDir === "asc" ? av - bv : bv - av;
      return state.sortDir === "asc" ? String(av).localeCompare(String(bv)) : String(bv).localeCompare(String(av));
    });
    return rows;
  }

  function render() {
    tbody.textContent = "";
    const data = sorted();
    if (!data.length) {
      const tr = h("tr");
      tr.appendChild(h("td", { colSpan: columns.length, class: "table__dim", style: { textAlign: "center", padding: "32px" } }, emptyText));
      tbody.appendChild(tr);
      return;
    }
    data.forEach((r, i) => {
      const tr = h("tr", {
        tabIndex: 0,
        dataset: { rowlink: onRowClick ? "true" : "" },
        onClick: onRowClick ? () => onRowClick(r, i) : undefined,
        onKeydown: (e) => {
          if (!onRowClick) return;
          if (e.key === "Enter") { e.preventDefault(); onRowClick(r, i); }
          if (e.key === "j" || e.key === "ArrowDown") { e.preventDefault(); tr.nextElementSibling?.focus(); }
          if (e.key === "k" || e.key === "ArrowUp")   { e.preventDefault(); tr.previousElementSibling?.focus(); }
        },
      });
      columns.forEach((c) => {
        const td = h("td", {
          class: [c.align === "right" ? "table__num" : "", c.dim ? "table__dim" : ""].filter(Boolean).join(" "),
        });
        const v = c.render ? c.render(r) : (c.value ? c.value(r) : r[c.key]);
        if (v instanceof Node) td.appendChild(v);
        else if (v !== null && v !== undefined) td.textContent = v;
        tr.appendChild(td);
      });
      tbody.appendChild(tr);
    });
  }

  return {
    el: wrap,
    update(newRows) { state.rows = newRows || []; render(); },
  };
}
