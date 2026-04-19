// terminal.js — live log viewer via /api/logs/stream (SSE).

import { h, escapeHtml, relTime } from "../lib/fmt.js";
import { tryApi } from "../lib/api.js";
import { sse } from "../lib/sse.js";
import { cmdk } from "../lib/cmdk-registry.js";

export function mount(root) {
  root.innerHTML = "";
  root.appendChild(h("header", { class: "view-header" },
    h("div", {},
      h("h1", { class: "view-header__title" }, "Terminal"),
      h("div", { class: "view-header__subtitle" }, "Live tail of engine logs. Filter by text or severity."),
    ),
    h("div", { class: "row" },
      h("div", { class: "segmented", id: "term-levels" },
        h("button", { "aria-pressed": "true", dataset: { level: "all" } },  "All"),
        h("button", { "aria-pressed": "false", dataset: { level: "error" } },"Error"),
        h("button", { "aria-pressed": "false", dataset: { level: "warn" } }, "Warn"),
        h("button", { "aria-pressed": "false", dataset: { level: "info" } }, "Info"),
      ),
      h("input", {
        class: "input", style: { width: "260px" },
        placeholder: "Filter text…",
        id: "term-filter",
        dataset: { role: "view-filter" },
      }),
      h("button", { class: "btn btn--secondary btn--sm", onClick: () => { term.innerHTML = ""; buf = []; } }, "Clear"),
    ),
  ));

  const body = h("div", { class: "view-body" });
  root.appendChild(body);
  const term = h("div", { class: "terminal", id: "term-body" });
  body.appendChild(term);

  let buf = [];           // keep last 2000 lines
  let level = "all", textFilter = "";

  body.querySelector;

  // Segment level.
  const segs = root.querySelectorAll("#term-levels button");
  segs.forEach((s) => s.addEventListener("click", () => {
    segs.forEach((x) => x.setAttribute("aria-pressed", "false"));
    s.setAttribute("aria-pressed", "true");
    level = s.dataset.level;
    repaint();
  }));
  root.querySelector("#term-filter").addEventListener("input", (e) => { textFilter = e.target.value.toLowerCase(); repaint(); });

  // Preload recent history.
  tryApi("/api/logs/history", { params: { limit: 500 } }).then((r) => {
    if (r.ok) {
      const entries = r.data?.entries || r.data || [];
      for (const e of entries) push(e);
    } else {
      push({ level: "info", msg: "Log history endpoint not available — streaming only." });
    }
    repaint();
  });

  const stream = sse("/api/logs/stream", {
    onMessage: (raw) => {
      try { const e = JSON.parse(raw); push(e); appendOne(e); }
      catch { push({ level: "info", msg: raw }); appendOne({ level: "info", msg: raw }); }
    },
  });

  function push(e) { buf.push(e); if (buf.length > 2000) buf.splice(0, buf.length - 2000); }

  function repaint() {
    term.innerHTML = "";
    for (const e of buf) if (visible(e)) term.appendChild(lineEl(e));
    term.scrollTop = term.scrollHeight;
  }
  function appendOne(e) {
    if (!visible(e)) return;
    term.appendChild(lineEl(e));
    term.scrollTop = term.scrollHeight;
  }
  function visible(e) {
    if (level !== "all" && (e.level || "info") !== level) return false;
    if (!textFilter) return true;
    return (e.msg || "").toLowerCase().includes(textFilter) ||
           (e.service || "").toLowerCase().includes(textFilter);
  }
  function lineEl(e) {
    const cls = e.level === "error" ? "terminal__line--err"
              : e.level === "warn"  ? "terminal__line--warn"
              : e.level === "ok"    ? "terminal__line--ok"
              : "";
    const prefix = (e.service ? `[${e.service}] ` : "");
    const ts = e.at || e.time || e.timestamp;
    const tsStr = ts ? new Date(toMs(ts)).toISOString().slice(11, 23) : "";
    return h("div", { class: `terminal__line ${cls}` },
      tsStr ? h("span", { class: "terminal__line--dim" }, tsStr + " ") : null,
      e.level ? h("span", { style: { color: levelColor(e.level) } }, (e.level || "").toUpperCase().padEnd(5) + " ") : null,
      prefix,
      e.msg || "",
    );
  }

  const scope = cmdk.scope("terminal", [
    { id: "term:clear", group: "Actions", title: "Clear terminal", run: () => { buf = []; term.innerHTML = ""; } },
  ]);

  return () => { stream.close(); scope(); };
}

function toMs(t) { if (!t) return Date.now(); if (typeof t === "number") return t < 1e12 ? t*1000 : t; const d = Date.parse(t); return isNaN(d) ? Date.now() : d; }
function levelColor(l) {
  return l === "error" ? "var(--err)" : l === "warn" ? "var(--warn)" : l === "ok" ? "var(--ok)" : "var(--accent)";
}
