// agentic.js — ReAct trace viewer.

import { h, relTime, ms as fmtMs, dateShort } from "../lib/fmt.js";
import { tryApi } from "../lib/api.js";
import { cmdk } from "../lib/cmdk-registry.js";
import { toast } from "../components/toast.js";
import { bus } from "../lib/bus.js";

export function mount(root) {
  root.innerHTML = "";
  root.appendChild(h("header", { class: "view-header" },
    h("div", {},
      h("h1", { class: "view-header__title" }, "Agentic traces"),
      h("div", { class: "view-header__subtitle" },
        "The engine's reasoning trajectory for each decision. Flight-data-recorder for autonomous healing."),
    ),
    h("div", { class: "row" },
      h("button", { class: "btn btn--accent btn--sm", onClick: replaySelected }, "↻ Replay trace"),
      h("button", { class: "btn btn--secondary btn--sm", onClick: refreshTraces }, "↻ Refresh"),
    ),
  ));

  const body = h("div", { class: "view-body split" });
  root.appendChild(body);

  const listCol = h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {}, h("h2", { class: "card__title" }, "Traces")),
    ),
    h("div", { style: { padding: "8px 12px", borderBottom: "1px solid var(--border)" } },
      h("input", {
        class: "input", placeholder: "Filter by goal / service…",
        dataset: { role: "view-filter" },
        onInput: (e) => { filterText = e.target.value.toLowerCase(); renderList(); },
      }),
    ),
    h("div", { id: "ag-list", style: { maxHeight: "calc(100vh - 260px)", overflow: "auto" } }),
  );
  const detailCol = h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {},
        h("h2", { class: "card__title", id: "ag-detail-title" }, "Select a trace"),
        h("div", { class: "card__subtitle", id: "ag-detail-sub" }, "—"),
      ),
    ),
    h("div", { class: "card__body", id: "ag-detail", style: { maxHeight: "calc(100vh - 220px)", overflow: "auto" } }),
  );
  body.appendChild(listCol);
  body.appendChild(detailCol);

  let traces = [], filterText = "", selectedId = null;

  refreshTraces();
  const scope = cmdk.scope("agentic", [
    { id: "agentic:refresh", group: "Actions", title: "Refresh traces", run: refreshTraces },
  ]);

  async function refreshTraces() {
    const [recent] = await Promise.all([tryApi("/api/v5/agentic/memory/recall", { params: { limit: 40 } })]);
    traces = recent.ok ? (recent.data?.traces || recent.data || []) : demoTraces();
    if (!traces.length) traces = demoTraces();
    renderList();
    if (traces[0]) selectTrace(traces[0]);
  }

  function renderList() {
    const host = document.getElementById("ag-list");
    host.innerHTML = "";
    const filtered = traces.filter((t) =>
      !filterText ||
      (t.goal || "").toLowerCase().includes(filterText) ||
      (t.service || "").toLowerCase().includes(filterText)
    );
    if (!filtered.length) {
      host.appendChild(h("div", { class: "empty" }, h("div", { class: "empty__title" }, "No traces")));
      return;
    }
    for (const t of filtered) host.appendChild(traceRow(t));
  }

  function traceRow(t) {
    const row = h("div", {
      class: "sidebar__item",
      style: { display: "block", height: "auto", padding: "10px 14px", margin: "2px 6px", cursor: "pointer" },
      dataset: t.id === selectedId ? { active: "true" } : {},
      onClick: () => selectTrace(t),
    },
      h("div", { class: "row", style: { justifyContent: "space-between" } },
        h("span", { class: "mono muted", style: { fontSize: "11px" } }, shortId(t.id)),
        h("span", { class: `badge badge--${verdictClass(t.verdict)}` }, t.verdict || "ok"),
      ),
      h("div", { style: { color: "var(--text-strong)", marginTop: "4px", fontWeight: "500" } }, t.goal || "—"),
      h("div", { class: "row muted", style: { justifyContent: "space-between", marginTop: "4px", fontSize: "11px" } },
        h("span", {}, `${(t.steps || t.trace?.length || 0)} steps · ${t.service || "—"}`),
        h("span", {}, relTime(toMs(t.started_at || t.at))),
      ),
    );
    if (t.id === selectedId) row.setAttribute("aria-current", "page");
    return row;
  }

  function selectTrace(t) {
    selectedId = t.id;
    renderList();
    document.getElementById("ag-detail-title").textContent = t.goal || "Trace " + shortId(t.id);
    document.getElementById("ag-detail-sub").innerHTML = "";
    const sub = document.getElementById("ag-detail-sub");
    sub.append(
      `${t.service || "—"} · `,
      `${(t.steps || t.trace?.length || 0)} steps · `,
      `${fmtMs(t.duration_ms || 0)} · `,
      dateShort(toMs(t.started_at || t.at)),
    );
    const host = document.getElementById("ag-detail");
    host.innerHTML = "";
    const steps = t.trace || t.steps_list || demoSteps();
    const wrap = h("div", { class: "trace" });
    steps.forEach((s, i) => wrap.appendChild(stepEl(s, i)));
    host.appendChild(wrap);
  }

  function stepEl(s, i) {
    const kind = (s.kind || s.type || "thought").toLowerCase();
    const kindCls = kind.includes("act") ? "action" : kind.includes("obs") ? "observation" : kind.includes("err") ? "error" : "thought";
    const body = s.text || s.content || s.message || s.observation || s.action || "";
    const extra = s.data ? JSON.stringify(s.data, null, 2) : (s.args ? JSON.stringify(s.args, null, 2) : null);
    return h("div", { class: `trace__step trace__step--${kindCls}` },
      h("div", { class: "trace__step__idx" },
        h("span", {}, String(i + 1).padStart(2, "0")),
        s.duration_ms != null ? h("span", {}, fmtMs(s.duration_ms)) : null,
      ),
      h("div", {},
        h("div", { class: "trace__step__kind" }, kind),
        h("div", { class: "trace__step__body" },
          body,
          extra ? h("pre", {}, extra) : null,
        ),
      ),
    );
  }

  async function replaySelected() {
    if (!selectedId) { toast.info("Select a trace first"); return; }
    const t = traces.find((x) => x.id === selectedId);
    toast.info("Replaying trace", t?.goal || selectedId);
    const r = await tryApi("/api/v5/agentic/meta-investigate", { method: "POST", body: { trace_id: selectedId } });
    if (r.ok) toast.ok("Replay complete", r.data?.summary || "verdict: " + (r.data?.verdict || "ok"));
    else toast.warn("Replay endpoint unavailable", "running on the local demo dataset");
  }

  return () => scope();
}

function shortId(id) { if (!id) return "—"; return String(id).slice(0, 8); }
function verdictClass(v) {
  const s = String(v || "").toLowerCase();
  if (s.includes("fail") || s.includes("err"))  return "err";
  if (s.includes("escal") || s.includes("warn")) return "warn";
  return "ok";
}
function toMs(t) { if (!t) return Date.now(); if (typeof t === "number") return t < 1e12 ? t*1000 : t; const d = Date.parse(t); return isNaN(d) ? Date.now() : d; }

function demoTraces() {
  const goals = [
    "Investigate p99 latency spike on /checkout",
    "Diagnose repeated 503s from payments",
    "Plan scale-out for billing before batch window",
    "Verify restart did not regress SLO",
    "Root-cause cache stampede on session-store",
  ];
  return goals.map((g, i) => ({
    id: "tr_" + (Math.random() * 1e9 | 0).toString(16),
    goal: g,
    service: ["rest","payments","billing","auth","session-store"][i],
    steps: 6 + (i % 4) * 2,
    verdict: i === 1 ? "escalated" : i === 4 ? "failed" : "ok",
    duration_ms: 1200 + i * 300,
    started_at: Date.now() - (i + 1) * 180_000,
  }));
}
function demoSteps() {
  return [
    { kind: "thought",     text: "Goal: investigate latency spike on /checkout. Start by checking topology + recent deploys." },
    { kind: "action",      text: "tool: topology.get(service=checkout, depth=2)", data: { service: "checkout", depth: 2 } },
    { kind: "observation", text: "Received 8-node subgraph. payments and billing are downstream. billing last-deployed 12m ago." },
    { kind: "thought",     text: "Hypothesis: billing deploy introduced regression. Check p99 distribution split." },
    { kind: "action",      text: "tool: metrics.query(service=billing, metric=latency_p99, t0=-30m)" },
    { kind: "observation", text: "p99 jumped from 42ms to 210ms at deploy+3m, flat since. cpu normal, qps flat." },
    { kind: "thought",     text: "Matches deploy-induced regression pattern. Propose rollback + notify owner." },
    { kind: "action",      text: "tool: playbook.run(name=safe-rollback, service=billing)" },
    { kind: "observation", text: "rollback completed. verifying p99…" },
  ];
}
