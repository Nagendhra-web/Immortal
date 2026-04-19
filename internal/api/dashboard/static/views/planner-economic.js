// planner-economic.js — cost-constrained optimizer + Pareto frontier.

import { h, num, pct, ms as fmtMs } from "../lib/fmt.js";
import { tryApi } from "../lib/api.js";
import { toast } from "../components/toast.js";
import { cmdk } from "../lib/cmdk-registry.js";

export function mount(root) {
  root.innerHTML = "";
  root.appendChild(h("header", { class: "view-header" },
    h("div", {},
      h("h1", { class: "view-header__title" }, "Economic planner"),
      h("div", { class: "view-header__subtitle" },
        "Optimize under cost constraints. Points show candidate allocations. Hover the frontier to see tradeoffs; click to explore a specific allocation."),
    ),
  ));

  const body = h("div", { class: "view-body stack" });
  root.appendChild(body);

  const form = h("section", { class: "card" },
    h("header", { class: "card__header" }, h("div", {}, h("h2", { class: "card__title" }, "Constraints"))),
    h("div", { class: "card__body", style: { display: "grid", gridTemplateColumns: "1fr 1fr 1fr auto", gap: "var(--s-3)", alignItems: "end" } },
      field("Budget ($/hour)",   "ep-budget", "12.00", "number"),
      field("SLO target (p99 ms)", "ep-slo",   "150",   "number"),
      field("Horizon (hours)",   "ep-horizon", "4",    "number"),
      h("button", { class: "btn btn--accent", onClick: solve }, "↯ Solve"),
    ),
  );
  body.appendChild(form);

  const frontierCard = h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {},
        h("h2", { class: "card__title" }, "Pareto frontier"),
        h("div", { class: "card__subtitle" }, "Cost vs. latency. Cyan = current · lime = recommended."),
      ),
    ),
    h("div", { class: "card__body" }, h("div", { id: "ep-frontier", style: { height: "360px" } })),
  );
  body.appendChild(frontierCard);

  const detailCard = h("section", { class: "card" },
    h("header", { class: "card__header" }, h("div", {}, h("h2", { class: "card__title", id: "ep-detail-title" }, "Allocation"))),
    h("div", { class: "card__body", id: "ep-detail" }),
  );
  body.appendChild(detailCard);

  let points = [], current = null, recommended = null;
  solve();
  const scope = cmdk.scope("planner-economic", [
    { id: "planner-eco:solve", group: "Actions", title: "Solve economic plan", run: solve },
  ]);

  async function solve() {
    const body = {
      budget: Number(document.getElementById("ep-budget").value),
      slo:    Number(document.getElementById("ep-slo").value),
      horizon: Number(document.getElementById("ep-horizon").value),
    };
    toast.info("Solving…");
    const r = await tryApi("/api/capacity", { method: "POST", body });
    const data = r.ok ? r.data : null;
    points = (data?.allocations || []).length ? data.allocations : demoAllocations(body);
    current = data?.current || points.find((p) => p.tag === "current") || points[Math.floor(points.length / 2)];
    recommended = data?.recommended || bestOn(points, body);
    drawFrontier();
    selectAllocation(recommended);
  }

  function drawFrontier() {
    const host = document.getElementById("ep-frontier");
    host.innerHTML = "";
    const W = host.clientWidth || 700;
    const H = 360;
    const padL = 60, padR = 20, padT = 20, padB = 40;
    const xs = points.map((p) => p.cost), ys = points.map((p) => p.p99);
    const xMin = Math.min(...xs) * 0.9, xMax = Math.max(...xs) * 1.05;
    const yMin = Math.min(...ys) * 0.9, yMax = Math.max(...ys) * 1.05;
    const x = (v) => padL + (v - xMin) / (xMax - xMin) * (W - padL - padR);
    const y = (v) => H - padB - (v - yMin) / (yMax - yMin) * (H - padT - padB);
    const svgNS = "http://www.w3.org/2000/svg";
    const svg = document.createElementNS(svgNS, "svg");
    svg.setAttribute("viewBox", `0 0 ${W} ${H}`);
    svg.classList.add("chart");

    // axes
    const axis = (attrs) => { const l = document.createElementNS(svgNS, "line"); for (const k in attrs) l.setAttribute(k, attrs[k]); l.setAttribute("class", "chart__axis-line"); return l; };
    svg.appendChild(axis({ x1: padL, x2: W - padR, y1: H - padB, y2: H - padB }));
    svg.appendChild(axis({ x1: padL, x2: padL,     y1: padT,      y2: H - padB }));

    // points
    const pareto = paretoFront(points);
    // frontier polyline
    const pathD = pareto.map((p, i) => (i === 0 ? "M " : "L ") + x(p.cost) + "," + y(p.p99)).join(" ");
    const path = document.createElementNS(svgNS, "path");
    path.setAttribute("d", pathD);
    path.setAttribute("stroke", "var(--signal-lime)");
    path.setAttribute("stroke-width", "2");
    path.setAttribute("fill", "none");
    path.setAttribute("stroke-dasharray", "4 3");
    svg.appendChild(path);

    // dots
    for (const p of points) {
      const c = document.createElementNS(svgNS, "circle");
      c.setAttribute("cx", x(p.cost));
      c.setAttribute("cy", y(p.p99));
      c.setAttribute("r", 5);
      const isCurrent = p === current || (p.id && current && p.id === current.id);
      const isRec     = p === recommended || (p.id && recommended && p.id === recommended.id);
      c.setAttribute("fill",
        isRec ? "var(--signal-lime)" :
        isCurrent ? "var(--signal-cyan)" :
        "var(--ink-5)");
      c.setAttribute("stroke", isRec || isCurrent ? "var(--bg)" : "none");
      c.setAttribute("stroke-width", "2");
      c.style.cursor = "pointer";
      c.addEventListener("click", () => selectAllocation(p));
      c.setAttribute("data-tooltip", `$${num(p.cost, 2)}/h · ${num(p.p99, 0)}ms`);
      svg.appendChild(c);
    }

    // axis labels
    const label = (x0, y0, text, anchor = "middle") => { const t = document.createElementNS(svgNS, "text"); t.setAttribute("x", x0); t.setAttribute("y", y0); t.setAttribute("text-anchor", anchor); t.setAttribute("class", "chart__tick"); t.textContent = text; return t; };
    svg.appendChild(label(W / 2, H - 8, "cost ($/hour) →"));
    const yLabel = document.createElementNS(svgNS, "text");
    yLabel.setAttribute("transform", `rotate(-90 16 ${H/2})`);
    yLabel.setAttribute("x", 16); yLabel.setAttribute("y", H / 2);
    yLabel.setAttribute("class", "chart__tick");
    yLabel.setAttribute("text-anchor", "middle");
    yLabel.textContent = "p99 latency (ms) →";
    svg.appendChild(yLabel);

    host.appendChild(svg);
  }

  function selectAllocation(p) {
    if (!p) return;
    document.getElementById("ep-detail-title").textContent =
      `${p.tag || "candidate"} · $${num(p.cost, 2)}/h · ${num(p.p99, 0)}ms`;
    const host = document.getElementById("ep-detail");
    host.innerHTML = "";
    const svc = p.services || demoServices();
    host.appendChild(h("div", { class: "stack" },
      h("div", { class: "grid grid-3" },
        h("div", { class: "kpi" }, h("span", { class: "kpi__label" }, "Cost"),     h("span", { class: "kpi__value" }, "$" + num(p.cost, 2) + "/h")),
        h("div", { class: "kpi" }, h("span", { class: "kpi__label" }, "p99"),      h("span", { class: "kpi__value" }, fmtMs(p.p99))),
        h("div", { class: "kpi" }, h("span", { class: "kpi__label" }, "SLO met"),  h("span", { class: "kpi__value" }, pct(p.slo_prob ?? 0.95, 1))),
      ),
      h("h3", {}, "Per-service allocation"),
      h("div", { class: "stack", style: { gap: "6px" } },
        ...svc.map((s) => h("div", { class: "row", style: { gap: "12px" } },
          h("div", { style: { width: "120px" } }, s.name),
          h("div", { style: { flex: 1, height: "8px", background: "var(--bg-elevated)", borderRadius: "999px", overflow: "hidden" } },
            h("div", { style: { width: (s.pct * 100) + "%", height: "100%", background: "var(--accent)" } })),
          h("div", { class: "mono muted", style: { width: "80px", textAlign: "right" } }, num(s.replicas, 0) + " × " + s.size),
        )),
      ),
      h("div", { class: "row" },
        h("button", { class: "btn btn--accent btn--sm",
          onClick: () => toast.ok("Recommended applied (demo)") }, "Apply this allocation"),
        h("button", { class: "btn btn--secondary btn--sm",
          onClick: () => toast.info("Diff computed", "showing under audit → diff viewer") },
          "Diff vs. current"),
      ),
    ));
  }

  return () => scope();
}

function field(label, id, val, type = "text") {
  return h("div", { class: "field" },
    h("span", { class: "field__label" }, label),
    h("input", { class: "input", id, type, value: val }),
  );
}

function bestOn(points, constraints) {
  const ok = points.filter((p) => p.p99 <= constraints.slo);
  if (!ok.length) return points.slice().sort((a, b) => a.p99 - b.p99)[0];
  return ok.sort((a, b) => a.cost - b.cost)[0];
}
function paretoFront(points) {
  const sorted = [...points].sort((a, b) => a.cost - b.cost);
  const front = [];
  let bestY = Infinity;
  for (const p of sorted) {
    if (p.p99 < bestY) { front.push(p); bestY = p.p99; }
  }
  return front;
}
function demoAllocations(c) {
  const out = [];
  for (let i = 0; i < 22; i++) {
    const cost = c.budget * (0.4 + Math.random() * 1.1);
    const p99  = c.slo    * (2.2 - Math.log10(1 + cost)) + (Math.random() - 0.4) * 30;
    out.push({ id: "a" + i, cost, p99: Math.max(20, p99), slo_prob: Math.max(0, Math.min(1, (c.slo - p99 + 100) / 200)) });
  }
  out[Math.floor(out.length / 2)].tag = "current";
  return out;
}
function demoServices() {
  return [
    { name: "rest",       replicas: 12, size: "m",  pct: 0.32 },
    { name: "payments",   replicas: 6,  size: "m",  pct: 0.22 },
    { name: "billing",    replicas: 4,  size: "s",  pct: 0.14 },
    { name: "auth",       replicas: 3,  size: "s",  pct: 0.12 },
    { name: "catalog",    replicas: 3,  size: "l",  pct: 0.10 },
    { name: "session-store", replicas: 2, size: "s", pct: 0.10 },
  ];
}
