// topology.js — service graph.

import { h, num, relTime } from "../lib/fmt.js";
import { tryApi } from "../lib/api.js";
import { chart } from "../components/chart.js";
import { sheet } from "../components/sheet.js";
import { cmdk } from "../lib/cmdk-registry.js";
import { table } from "../components/table.js";
import { bus } from "../lib/bus.js";

export function mount(root) {
  root.innerHTML = "";
  root.appendChild(h("header", { class: "view-header" },
    h("div", {},
      h("h1", { class: "view-header__title" }, "Service topology"),
      h("div", { class: "view-header__subtitle" }, "Live dependency graph with health, depth, and impact blast-radius. Click a node for detail."),
    ),
    h("div", { class: "row" },
      h("button", { class: "btn btn--secondary btn--sm", onClick: refresh }, "↻ Refresh"),
    ),
  ));

  const body = h("div", { class: "view-body" });
  root.appendChild(body);

  const graphCard = h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {},
        h("h2", { class: "card__title" }, "Dependency graph"),
        h("div", { class: "card__subtitle", id: "tp-subtitle" }, "—"),
      ),
    ),
    h("div", { class: "card__body" }, h("div", { id: "tp-graph", style: { height: "520px" } })),
  );
  body.appendChild(graphCard);

  const listCard = h("section", { class: "card", style: { marginTop: "var(--s-4)" } },
    h("header", { class: "card__header" },
      h("div", {}, h("h2", { class: "card__title" }, "Services")),
    ),
    h("div", { class: "card__body", id: "tp-list" }),
  );
  body.appendChild(listCard);

  let state = { snapshot: null };
  refresh();
  const scope = cmdk.scope("topology", [
    { id: "topology:refresh", group: "Actions", title: "Refresh topology", run: refresh },
  ]);

  async function refresh() {
    const [snap, deps] = await Promise.all([
      tryApi("/api/v5/topology/snapshot"),
      tryApi("/api/dependencies"),
    ]);
    const data = snap.ok ? snap.data : deps.ok ? deps.data : null;
    const { nodes, edges } = buildGraph(data);
    state.snapshot = data;
    document.getElementById("tp-subtitle").textContent =
      `${nodes.length} services · ${edges.length} dependencies · updated ${relTime(Date.now())}`;

    chart.graph(document.getElementById("tp-graph"), {
      nodes: nodes.map((n) => ({
        id: n.id,
        label: n.label || n.id,
        size: 12 + Math.min(18, (n.degree || 1) * 2),
        color: healthColor(n.health),
      })),
      edges,
      onNodeClick: (nd) => openDetail(nd.id, nodes.find((x) => x.id === nd.id)),
    });

    renderServiceList(nodes);
  }

  function renderServiceList(nodes) {
    const host = document.getElementById("tp-list");
    host.innerHTML = "";
    const tbl = table({
      columns: [
        { label: "Service", key: "id" },
        { label: "Health", key: "health", render: (n) => h("span", { class: `badge badge--${healthBadge(n.health)}` }, String(n.health || "healthy")) },
        { label: "Depth",  key: "depth", align: "right" },
        { label: "Upstream",   key: "upstream",   align: "right" },
        { label: "Downstream", key: "downstream", align: "right" },
        { label: "Last seen",  key: "lastSeen", render: (n) => h("span", { class: "muted" }, relTime(n.lastSeen)) },
      ],
      rows: nodes,
      onRowClick: (n) => openDetail(n.id, n),
    });
    host.appendChild(tbl.el);
  }

  function openDetail(id, node) {
    sheet({
      title: id,
      subtitle: `depth ${node?.depth ?? "—"} · ${healthBadge(node?.health)}`,
      body: h("div", { class: "stack" },
        h("div", { class: "grid grid-3" },
          h("div", { class: "kpi" }, h("span", { class: "kpi__label" }, "Upstream"),   h("span", { class: "kpi__value" }, String(node?.upstream ?? 0))),
          h("div", { class: "kpi" }, h("span", { class: "kpi__label" }, "Downstream"), h("span", { class: "kpi__value" }, String(node?.downstream ?? 0))),
          h("div", { class: "kpi" }, h("span", { class: "kpi__label" }, "Depth"),      h("span", { class: "kpi__value" }, String(node?.depth ?? "—"))),
        ),
        h("p", { class: "muted" }, "Quick actions:"),
        h("div", { class: "row" },
          h("a", { class: "btn btn--secondary btn--sm", href: `#/twin?service=${encodeURIComponent(id)}` }, "Twin forecast →"),
          h("a", { class: "btn btn--secondary btn--sm", href: `#/causal?service=${encodeURIComponent(id)}` }, "Causal RCA →"),
          h("a", { class: "btn btn--secondary btn--sm", href: `#/agentic?service=${encodeURIComponent(id)}` }, "Agentic traces →"),
        ),
      ),
    });
  }

  return () => scope();
}

function buildGraph(data) {
  if (!data) {
    // demo fallback
    const ids = ["api-gateway", "auth", "rest", "payments", "postgres", "redis", "billing", "events"];
    const nodes = ids.map((id, i) => ({ id, health: i % 4 === 2 ? "degraded" : "healthy", depth: Math.floor(i / 3), upstream: 1, downstream: 1, degree: 2, lastSeen: Date.now() - 5000 }));
    const edges = [
      { from: "api-gateway", to: "auth" },  { from: "api-gateway", to: "rest" },
      { from: "rest", to: "postgres" }, { from: "rest", to: "redis" },
      { from: "rest", to: "payments" }, { from: "payments", to: "billing" },
      { from: "billing", to: "events" }, { from: "auth", to: "postgres" },
    ];
    return { nodes: dressNodes(nodes, edges), edges };
  }
  const rawNodes = data.services || data.nodes || [];
  const rawEdges = data.edges || data.dependencies || [];
  const nodes = rawNodes.map((n) => ({
    id: n.id || n.name || n.service,
    label: n.label || n.name,
    health: n.health || n.status || "healthy",
    depth: n.depth ?? 0,
    lastSeen: toMs(n.last_seen || n.lastSeen || n.updated_at) || Date.now(),
  }));
  const edges = rawEdges.map((e) => ({ from: e.from || e.source || e.parent, to: e.to || e.target || e.child, label: e.label }));
  return { nodes: dressNodes(nodes, edges), edges };
}

function dressNodes(nodes, edges) {
  const degree = new Map();
  const upMap = new Map(), downMap = new Map();
  for (const e of edges) {
    degree.set(e.from, (degree.get(e.from) || 0) + 1);
    degree.set(e.to,   (degree.get(e.to)   || 0) + 1);
    upMap.set(e.to, (upMap.get(e.to) || 0) + 1);
    downMap.set(e.from, (downMap.get(e.from) || 0) + 1);
  }
  return nodes.map((n) => ({
    ...n,
    degree: degree.get(n.id) || 1,
    upstream: upMap.get(n.id) || 0,
    downstream: downMap.get(n.id) || 0,
  }));
}

function healthColor(h) {
  const v = String(h || "").toLowerCase();
  if (v.includes("down") || v.includes("fail") || v.includes("crit")) return `var(--err)`;
  if (v.includes("degraded") || v.includes("warn")) return `var(--warn)`;
  return `var(--ok)`;
}
function healthBadge(h) {
  const v = String(h || "").toLowerCase();
  if (v.includes("down") || v.includes("fail") || v.includes("crit")) return "err";
  if (v.includes("degraded") || v.includes("warn")) return "warn";
  return "ok";
}
function toMs(t) { if (!t) return null; if (typeof t === "number") return t < 1e12 ? t*1000 : t; const d = Date.parse(t); return isNaN(d) ? null : d; }
