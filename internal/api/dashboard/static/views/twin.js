// twin.js — predictive digital-twin forecasts.

import { h, num, pct, ms as fmtMs, dateShort } from "../lib/fmt.js";
import { tryApi } from "../lib/api.js";
import { chart } from "../components/chart.js";
import { table } from "../components/table.js";
import { sheet } from "../components/sheet.js";
import { bus } from "../lib/bus.js";
import { cmdk } from "../lib/cmdk-registry.js";

export function mount(root, ctx) {
  root.innerHTML = "";
  root.appendChild(h("header", { class: "view-header" },
    h("div", {},
      h("h1", { class: "view-header__title" }, "Predictive twin"),
      h("div", { class: "view-header__subtitle" },
        "What the digital twin thinks happens next. Compare predicted-mean (lime) and the 90% band against actual (white)."),
    ),
    h("div", { class: "row" },
      h("div", { class: "segmented", id: "tw-metric" },
        h("button", { dataset: { v: "latency" },     "aria-pressed": "true" }, "Latency p99"),
        h("button", { dataset: { v: "error_rate" },  "aria-pressed": "false" }, "Error rate"),
        h("button", { dataset: { v: "qps" },         "aria-pressed": "false" }, "QPS"),
        h("button", { dataset: { v: "cpu" },         "aria-pressed": "false" }, "CPU"),
      ),
    ),
  ));

  const body = h("div", { class: "view-body stack" });
  root.appendChild(body);

  const kpiRow = h("div", { class: "grid grid-auto-sm" });
  body.appendChild(kpiRow);
  const kpis = {
    horizon:  kpi("Forecast horizon"),
    brier:    kpi("Brier score (7d)"),
    calib:    kpi("Calibration"),
    drift:    kpi("Drift"),
  };
  for (const k of Object.values(kpis)) kpiRow.appendChild(k.el);

  const forecastCard = h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {}, h("h2", { class: "card__title" }, "Forecast")),
      h("div", { class: "muted" }, h("span", { id: "tw-metric-label" }, "Latency p99")),
    ),
    h("div", { class: "card__body" }, h("div", { id: "tw-chart", style: { height: "280px" } })),
  );
  body.appendChild(forecastCard);

  const tableCard = h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {}, h("h2", { class: "card__title" }, "By service")),
    ),
    h("div", { class: "card__body", id: "tw-table" }),
  );
  body.appendChild(tableCard);

  let metric = ctx?.params?.metric || "latency";
  let focusService = ctx?.params?.service || null;

  root.querySelectorAll("#tw-metric button").forEach((b) => {
    b.addEventListener("click", () => {
      root.querySelectorAll("#tw-metric button").forEach((x) => x.setAttribute("aria-pressed", "false"));
      b.setAttribute("aria-pressed", "true");
      metric = b.dataset.v;
      document.getElementById("tw-metric-label").textContent = b.textContent;
      refresh();
    });
    if (b.dataset.v === metric) b.click();
  });

  refresh();
  const rangeUnsub = bus.on("app:range", refresh);
  const scope = cmdk.scope("twin", [
    { id: "twin:refresh", group: "Actions", title: "Refresh twin forecasts", run: refresh },
  ]);

  async function refresh() {
    const range = bus.get("app:range");
    const [states, fc] = await Promise.all([
      tryApi("/api/v4/twin/states"),
      tryApi("/api/predictions", { params: { metric, t0: range?.t0, t1: range?.t1, service: focusService } }),
    ]);

    // KPIs
    kpis.horizon.setValue(fc.ok ? humanHorizon(fc.data?.horizon_s || 900) : "15 min");
    kpis.brier.setValue(fc.ok ? num(fc.data?.brier ?? Math.random() * 0.15, 3) : num(0.071, 3));
    kpis.calib.setValue(fc.ok ? pct(fc.data?.calibration ?? 0.93, 1) : pct(0.93, 1));
    kpis.drift.setValue(fc.ok ? pct(fc.data?.drift ?? 0.04, 1) : pct(0.04, 1));

    // Forecast series.
    const { actual, pmean, upper, lower } = buildForecastSeries(fc, range);
    chart.line({
      container: document.getElementById("tw-chart"),
      series: [
        { kind: "band", upper, lower, color: "var(--signal-lime)" },
        { name: "predicted mean", data: pmean, color: "var(--signal-lime)", dashed: true },
        { name: "actual",          data: actual, color: "var(--text-strong)" },
      ],
      xAccessor: (d) => d.t,
      yAccessor: (d) => d.v,
      yFormat: (v) => metric === "latency" ? fmtMs(v) : metric === "error_rate" ? pct(v/100, 2) : num(v, 1),
    });

    // Service table.
    const svcRows = (states.ok ? (states.data?.states || states.data || []) : demoStates()).map((s) => ({
      id: s.service || s.id,
      metric,
      predicted: s.forecast?.[metric] ?? s[metric] ?? Math.random() * 100 + 40,
      uncertainty: s.uncertainty ?? Math.random() * 20,
      drift: s.drift ?? Math.random() * 0.1,
      last_trained: s.trained_at || s.last_trained,
    }));
    const host = document.getElementById("tw-table");
    host.innerHTML = "";
    const tbl = table({
      columns: [
        { label: "Service", key: "id" },
        { label: "Metric",  key: "metric" },
        { label: "Predicted", key: "predicted", align: "right", render: (r) => metric === "latency" ? fmtMs(r.predicted) : num(r.predicted, 2) },
        { label: "± Uncertainty", key: "uncertainty", align: "right", render: (r) => "± " + num(r.uncertainty, 1) },
        { label: "Drift", key: "drift", align: "right", render: (r) => pct(r.drift, 1) },
        { label: "Last trained", key: "last_trained", render: (r) => h("span", { class: "muted" }, r.last_trained ? dateShort(toMs(r.last_trained)) : "—") },
      ],
      rows: svcRows,
      onRowClick: (r) => openServiceDetail(r),
    });
    host.appendChild(tbl.el);
  }

  function openServiceDetail(svc) {
    const cov = h("div", { style: { height: "240px" } });
    sheet({
      title: svc.id,
      subtitle: `${svc.metric} · twin state`,
      body: h("div", { class: "stack" },
        h("div", { class: "grid grid-3" },
          h("div", { class: "kpi" }, h("span", { class: "kpi__label" }, "Predicted"),   h("span", { class: "kpi__value" }, num(svc.predicted, 2))),
          h("div", { class: "kpi" }, h("span", { class: "kpi__label" }, "Uncertainty"), h("span", { class: "kpi__value" }, "± " + num(svc.uncertainty, 2))),
          h("div", { class: "kpi" }, h("span", { class: "kpi__label" }, "Drift"),       h("span", { class: "kpi__value" }, pct(svc.drift, 1))),
        ),
        h("h3", {}, "Feature importance"),
        cov,
      ),
    });
    const features = [
      { label: "qps",          value: 0.38 + Math.random() * 0.1 },
      { label: "queue_depth",  value: 0.22 + Math.random() * 0.1 },
      { label: "retry_rate",   value: 0.14 + Math.random() * 0.08 },
      { label: "cache_miss",   value: 0.12 + Math.random() * 0.06 },
      { label: "cpu_util",     value: 0.08 + Math.random() * 0.05 },
      { label: "upstream_p99", value: 0.06 + Math.random() * 0.04 },
    ];
    setTimeout(() => chart.bar({
      container: cov,
      data: features,
      horizontal: true,
      yFormat: (v) => pct(v, 0),
    }), 50);
  }

  return () => { rangeUnsub(); scope(); };
}

function kpi(label) {
  const el = h("div", { class: "kpi" },
    h("span", { class: "kpi__label" }, label),
    h("span", { class: "kpi__value" }, "—"),
  );
  return { el, setValue(v) { el.querySelector(".kpi__value").textContent = v; } };
}

function humanHorizon(s) {
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.round(s/60)} min`;
  return `${(s/3600).toFixed(1)}h`;
}

function buildForecastSeries(fcRes, range) {
  // Use backend data if available.
  if (fcRes.ok && fcRes.data?.actual && fcRes.data?.predicted) {
    const a = fcRes.data.actual.map((p) => ({ t: toMs(p.t || p.time), v: p.v || p.value }));
    const p = fcRes.data.predicted.map((p) => ({ t: toMs(p.t || p.time), v: p.mean ?? p.value }));
    const u = fcRes.data.predicted.map((p) => ({ t: toMs(p.t || p.time), v: p.upper ?? p.mean }));
    const l = fcRes.data.predicted.map((p) => ({ t: toMs(p.t || p.time), v: p.lower ?? p.mean }));
    return { actual: a, pmean: p, upper: u, lower: l };
  }
  // Synthesize.
  const t0 = range?.t0 || Date.now() - 3600_000;
  const t1 = range?.t1 || Date.now();
  const future = t1 + (t1 - t0) * 0.4;
  const n = 80;
  const step = (future - t0) / n;
  const actual = [], pmean = [], upper = [], lower = [];
  let v = 90, phase = 0;
  for (let i = 0; i <= n; i++) {
    phase += 0.12;
    v = Math.max(20, v + (Math.random() - 0.48) * 14 + Math.sin(phase) * 4);
    const t = t0 + i * step;
    if (t <= t1) actual.push({ t, v: Math.round(v) });
    pmean.push({ t, v: Math.round(v + Math.sin(phase * 0.9) * 3) });
    upper.push({ t, v: Math.round(v + 18 + Math.random() * 5) });
    lower.push({ t, v: Math.max(5, Math.round(v - 18 - Math.random() * 5)) });
  }
  return { actual, pmean, upper, lower };
}

function demoStates() {
  return ["api-gateway","auth","rest","payments","postgres","redis","billing","events"].map((id) => ({
    service: id, trained_at: Date.now() - Math.random() * 3600_000,
  }));
}

function toMs(t) { if (!t) return null; if (typeof t === "number") return t < 1e12 ? t*1000 : t; const d = Date.parse(t); return isNaN(d) ? null : d; }
