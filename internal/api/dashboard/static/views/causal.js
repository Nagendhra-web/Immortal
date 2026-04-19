// causal.js — PCMCI root-cause view.

import { h, num, pct, ms as fmtMs } from "../lib/fmt.js";
import { tryApi } from "../lib/api.js";
import { chart } from "../components/chart.js";
import { cmdk } from "../lib/cmdk-registry.js";
import { bus } from "../lib/bus.js";

export function mount(root, ctx) {
  root.innerHTML = "";
  root.appendChild(h("header", { class: "view-header" },
    h("div", {},
      h("h1", { class: "view-header__title" }, "Causal root-cause (PCMCI)"),
      h("div", { class: "view-header__subtitle" },
        "Not just correlated — causally implicated. Drag a node to explore its upstream causes and downstream effects."),
    ),
    h("div", { class: "row" },
      h("div", { class: "segmented", id: "ca-mode" },
        h("button", { dataset: { v: "causal" },      "aria-pressed": "true" }, "Causal"),
        h("button", { dataset: { v: "correlation" }, "aria-pressed": "false" }, "Correlation"),
      ),
      h("button", { class: "btn btn--secondary btn--sm", onClick: refresh }, "↻ Re-run"),
    ),
  ));

  const body = h("div", { class: "view-body split" });
  root.appendChild(body);

  const leftCol = h("section", { class: "card" },
    h("header", { class: "card__header" }, h("div", {}, h("h2", { class: "card__title" }, "Ranked causes"))),
    h("div", { class: "card__body", id: "ca-ranked" }),
  );
  const rightCol = h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {}, h("h2", { class: "card__title" }, "Causal DAG")),
      h("div", { class: "muted", id: "ca-subtitle" }, "—"),
    ),
    h("div", { class: "card__body" }, h("div", { id: "ca-graph", style: { height: "480px" } })),
  );
  body.appendChild(leftCol);
  body.appendChild(rightCol);

  let mode = "causal";
  root.querySelectorAll("#ca-mode button").forEach((b) => {
    b.addEventListener("click", () => {
      root.querySelectorAll("#ca-mode button").forEach((x) => x.setAttribute("aria-pressed", "false"));
      b.setAttribute("aria-pressed", "true");
      mode = b.dataset.v;
      refresh();
    });
  });

  refresh();
  const scope = cmdk.scope("causal", [
    { id: "causal:rerun", group: "Actions", title: "Re-run PCMCI", run: refresh },
  ]);

  async function refresh() {
    const [g, rc] = await Promise.all([
      tryApi("/api/causality/graph", { params: { mode } }),
      tryApi("/api/v5/causal/pcmci"),
    ]);
    const graph = (g.ok ? g.data : null) || (rc.ok ? rc.data?.graph : null);
    const ranked = rc.ok ? (rc.data?.causes || rc.data?.ranked || []) : [];
    const { nodes, edges } = normalizeGraph(graph);
    document.getElementById("ca-subtitle").textContent =
      `${nodes.length} variables · ${edges.length} ${mode === "causal" ? "causal edges" : "correlations"}`;
    chart.graph(document.getElementById("ca-graph"), {
      nodes: nodes.map((n) => ({ id: n.id, label: n.label || n.id, size: 14, color: "var(--signal-violet)" })),
      edges: edges.map((e) => ({ from: e.from, to: e.to, label: e.tau != null ? `τ=${e.tau}` : (e.strength ? num(e.strength, 2) : undefined) })),
    });
    renderRanked(ranked.length ? ranked : demoRanked());
  }

  function renderRanked(list) {
    const host = document.getElementById("ca-ranked");
    host.innerHTML = "";
    const wrap = h("div", { class: "stack" });
    list.slice(0, 8).forEach((c, i) => {
      const spark = h("div", { style: { height: "28px", width: "100%", color: "var(--signal-violet)" } });
      wrap.appendChild(h("div", { class: "card", style: { padding: "12px 14px" } },
        h("div", { class: "row", style: { justifyContent: "space-between" } },
          h("div", {},
            h("div", { class: "mono muted", style: { fontSize: "11px" } }, `#${i + 1}`),
            h("div", { class: "strong", style: { fontWeight: "600" } }, c.cause || c.name || c.from),
            h("div", { class: "muted", style: { fontSize: "11px" } },
              `→ ${c.effect || c.to || "outcome"} · lag τ=${c.tau ?? c.lag ?? "—"} · score ${num(c.score ?? c.strength, 2)}`),
          ),
          h("span", { class: `badge badge--violet` }, pct(c.confidence ?? 0.8, 0)),
        ),
        spark,
      ));
      // sparkline for the cause signal leading the effect
      const series = c.series || randomWalk(30);
      setTimeout(() => chart.sparkline(spark, series), 20);
    });
    host.appendChild(wrap);
  }

  return () => scope();
}

function normalizeGraph(data) {
  if (data && (data.nodes || data.edges)) {
    return {
      nodes: (data.nodes || []).map((n) => ({ id: n.id || n.name, label: n.label || n.name || n.id })),
      edges: (data.edges || []).map((e) => ({ from: e.from || e.source, to: e.to || e.target, tau: e.tau ?? e.lag, strength: e.strength ?? e.weight })),
    };
  }
  // Fallback demo
  const ids = ["qps_spike", "queue_depth", "cache_miss", "cpu_util", "latency_p99", "error_rate", "retry_storm"];
  const nodes = ids.map((id) => ({ id, label: id }));
  const edges = [
    { from: "qps_spike",   to: "queue_depth", tau: 1, strength: 0.82 },
    { from: "cache_miss",  to: "latency_p99", tau: 1, strength: 0.77 },
    { from: "queue_depth", to: "latency_p99", tau: 2, strength: 0.71 },
    { from: "latency_p99", to: "retry_storm", tau: 1, strength: 0.68 },
    { from: "retry_storm", to: "error_rate",  tau: 1, strength: 0.74 },
    { from: "cpu_util",    to: "latency_p99", tau: 0, strength: 0.43 },
  ];
  return { nodes, edges };
}
function demoRanked() {
  return [
    { cause: "qps_spike",   effect: "latency_p99", tau: 2, score: 0.81, confidence: 0.93, series: randomWalk(30) },
    { cause: "cache_miss",  effect: "latency_p99", tau: 1, score: 0.77, confidence: 0.88, series: randomWalk(30) },
    { cause: "retry_storm", effect: "error_rate",  tau: 1, score: 0.74, confidence: 0.85, series: randomWalk(30) },
  ];
}
function randomWalk(n) { const o = [50]; for (let i = 1; i < n; i++) o.push(Math.max(10, o[i-1] + (Math.random() - 0.5) * 14)); return o; }
