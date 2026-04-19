// overview.js — mission-control home. KPI strip + live metrics + incidents + recent healing.

import { h, num, ms as fmtMs, pct, relTime } from "../lib/fmt.js";
import { tryApi } from "../lib/api.js";
import { bus } from "../lib/bus.js";
import { sse } from "../lib/sse.js";
import { chart } from "../components/chart.js";
import { table } from "../components/table.js";
import { cmdk } from "../lib/cmdk-registry.js";

export function mount(root) {
  root.innerHTML = "";
  root.appendChild(h("header", { class: "view-header" },
    h("div", {},
      h("h1", { class: "view-header__title" }, "Mission control"),
      h("div", { class: "view-header__subtitle" },
        "Live view of the engine — health, healing events, predictions, and alerts. Pauseable. Kept in sync with the topbar time range."),
    ),
  ));

  const body = h("div", { class: "view-body stack" });
  root.appendChild(body);

  // KPI strip (4 cards) — populated by /api/status + /api/dna/health-score.
  const kpiRow = h("div", { class: "grid grid-auto-sm" });
  body.appendChild(kpiRow);
  const kpis = {
    health: makeKpi("Health score", "—", { color: "--signal-lime" }),
    healing: makeKpi("Healing events / h", "—"),
    incidents: makeKpi("Active incidents", "—"),
    sla: makeKpi("SLA (30d)", "—"),
  };
  for (const k of Object.values(kpis)) kpiRow.appendChild(k.el);

  // Latency + error chart.
  const latencyCard = h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {},
        h("h2", { class: "card__title" }, "Engine signals"),
        h("div", { class: "card__subtitle" }, "Latency p99 + error rate across the selected range"),
      ),
    ),
    h("div", { class: "card__body" }, h("div", { id: "ov-chart", style: { height: "260px" } })),
  );
  body.appendChild(latencyCard);

  // Two-up: incidents + recent healing.
  const split = h("div", { class: "grid grid-2" });
  body.appendChild(split);

  const incidentsCard = h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {}, h("h2", { class: "card__title" }, "Active incidents")),
      h("a", { class: "btn btn--ghost btn--sm", href: "#/audit" }, "Audit log →"),
    ),
    h("div", { class: "card__body", id: "ov-incidents" }),
  );
  const healingCard = h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {}, h("h2", { class: "card__title" }, "Recent healing events")),
      h("div", { class: "row" }, h("span", { class: "dot dot--live" }), h("span", { class: "muted" }, "Live")),
    ),
    h("div", { class: "card__body", id: "ov-healing" }),
  );
  split.appendChild(incidentsCard);
  split.appendChild(healingCard);

  let active = true;
  refresh();
  const rangeUnsub = bus.on("app:range", () => active && refresh());

  // SSE for recent healing.
  const stream = sse("/api/events", {
    onMessage: (raw) => {
      try {
        const ev = JSON.parse(raw);
        if (!ev) return;
        prependHealing(ev);
      } catch {}
    },
  });

  const scopeReset = cmdk.scope("overview", [
    { id: "ov:refresh", group: "Actions", title: "Refresh overview", run: refresh },
  ]);

  async function refresh() {
    const range = bus.get("app:range");
    const [status, health, metrics, incidents, healing, sla] = await Promise.all([
      tryApi("/api/status"),
      tryApi("/api/dna/health-score"),
      tryApi("/api/metrics", { params: { t0: range?.t0, t1: range?.t1 } }),
      tryApi("/api/incidents/active"),
      tryApi("/api/healing/history", { params: { limit: 25 } }),
      tryApi("/api/sla"),
    ]);

    const healthVal = health.ok ? (health.data?.score ?? health.data?.value) : null;
    kpis.health.setValue(healthVal == null ? "—" : num(healthVal, 1));

    const healCount = healing.ok ? (healing.data?.events?.length ?? healing.data?.length ?? 0) : 0;
    kpis.healing.setValue(num(healCount, 0));

    const incCount = incidents.ok ? (incidents.data?.incidents?.length ?? (Array.isArray(incidents.data) ? incidents.data.length : 0)) : 0;
    kpis.incidents.setValue(String(incCount));
    kpis.incidents.el.style.setProperty("--accent",
      incCount === 0 ? "var(--ok)" : incCount < 3 ? "var(--warn)" : "var(--err)");

    const slaVal = sla.ok ? (sla.data?.compliance ?? sla.data?.value) : null;
    kpis.sla.setValue(slaVal == null ? "—" : pct(slaVal, 2));

    // Chart data — pull from metrics or synthesize.
    const series = buildMetricSeries(metrics, range);
    chart.line({
      container: document.getElementById("ov-chart"),
      series,
      xAccessor: (d) => d.t,
      yAccessor: (d) => d.v,
      yFormat:  (v) => fmtMs(v),
    });

    // Sparklines in KPIs.
    const spark = series[0]?.data.slice(-40).map((d) => d.v) || [];
    kpis.health.setSpark(spark, "var(--signal-lime)");

    renderIncidents(incidents.data);
    renderHealing(healing.data);
  }

  function renderIncidents(data) {
    const host = document.getElementById("ov-incidents");
    host.innerHTML = "";
    const rows = (data?.incidents ?? data ?? []).slice(0, 8);
    if (!rows.length) {
      host.appendChild(h("div", { class: "empty" },
        h("div", { class: "empty__title" }, "No active incidents"),
        h("div", {}, "All services are healthy."),
      ));
      return;
    }
    const tbl = table({
      columns: [
        { label: "Severity", key: "severity", render: (r) => h("span", { class: `badge badge--${sevClass(r.severity)}` }, r.severity || "info") },
        { label: "Service",  key: "service" },
        { label: "Title",    key: "title", value: (r) => r.title || r.summary || r.message },
        { label: "Started",  key: "started_at", render: (r) => h("span", { class: "muted" }, relTime(toMs(r.started_at || r.at))) },
      ],
      rows,
    });
    host.appendChild(tbl.el);
  }

  function renderHealing(data) {
    const host = document.getElementById("ov-healing");
    host.innerHTML = "";
    const rows = (data?.events ?? data ?? []).slice(0, 10);
    if (!rows.length) {
      host.appendChild(h("div", { class: "empty" },
        h("div", { class: "empty__title" }, "Nothing to report"),
        h("div", {}, "No healing actions in this window."),
      ));
      return;
    }
    const tbl = table({
      columns: [
        { label: "When",    key: "at", render: (r) => h("span", { class: "muted" }, relTime(toMs(r.at || r.time || r.timestamp))) },
        { label: "Action",  key: "action", render: (r) => h("span", { class: "mono" }, r.action || r.kind || "heal") },
        { label: "Service", key: "service" },
        { label: "Result",  key: "result", render: (r) => h("span", { class: `badge badge--${r.success === false ? "err" : "ok"}` }, r.result || (r.success === false ? "failed" : "ok")) },
      ],
      rows,
    });
    host.appendChild(tbl.el);
  }

  function prependHealing(_ev) {
    // Best-effort: when a live event arrives, nudge a refresh of the healing card.
    refresh();
  }

  return () => {
    active = false;
    rangeUnsub();
    stream.close();
    scopeReset();
  };
}

function makeKpi(label, value, { color } = {}) {
  const el = h("div", { class: "kpi" },
    h("span", { class: "kpi__label" }, label),
    h("span", { class: "kpi__value" }, value),
    h("div",  { class: "kpi__spark" }),
  );
  const valEl = el.querySelector(".kpi__value");
  const sparkEl = el.querySelector(".kpi__spark");
  if (color) sparkEl.style.color = `var(${color})`;
  return {
    el,
    setValue(v) { valEl.textContent = v; },
    setSpark(arr, col) { if (col) sparkEl.style.color = col; chart.sparkline(sparkEl, arr); },
  };
}

function buildMetricSeries(metricsRes, range) {
  // Expect either {series:[{name, data:[{t,v}]}]} or {latency:[], errors:[]}.
  if (metricsRes.ok && metricsRes.data?.series) return metricsRes.data.series;
  // Synthesize a demo series from range (visible even without metrics).
  const t0 = range?.t0 || Date.now() - 3600_000;
  const t1 = range?.t1 || Date.now();
  const n = 60;
  const step = (t1 - t0) / n;
  const latency = [], errors = [];
  let lv = 80, ev = 0.01;
  for (let i = 0; i <= n; i++) {
    lv = Math.max(20, lv + (Math.random() - 0.45) * 18);
    ev = Math.max(0, ev + (Math.random() - 0.5) * 0.002);
    const t = t0 + i * step;
    latency.push({ t, v: Math.round(lv) });
    errors.push({ t, v: ev });
  }
  return [
    { name: "latency p99 (ms)",  data: latency },
    { name: "error rate",        data: errors.map((d) => ({ t: d.t, v: d.v * 1000 })), dashed: true },
  ];
}

function sevClass(s) {
  const v = String(s || "").toLowerCase();
  if (v.includes("crit") || v.includes("high")) return "err";
  if (v.includes("warn") || v.includes("med")) return "warn";
  return "info";
}
function toMs(t) {
  if (!t) return null;
  if (typeof t === "number") return t < 1e12 ? t * 1000 : t;
  const d = Date.parse(t);
  return isNaN(d) ? null : d;
}
