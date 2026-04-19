// federation.js — federated knowledge graph explorer.

import { h, relTime, num } from "../lib/fmt.js";
import { tryApi } from "../lib/api.js";
import { chart } from "../components/chart.js";
import { sheet } from "../components/sheet.js";
import { cmdk } from "../lib/cmdk-registry.js";

export function mount(root) {
  root.innerHTML = "";
  root.appendChild(h("header", { class: "view-header" },
    h("div", {},
      h("h1", { class: "view-header__title" }, "Federation"),
      h("div", { class: "view-header__subtitle" },
        "Each node contributes to the knowledge graph without centralizing data. Dashed edges are cross-peer — hover for proof hashes."),
    ),
    h("div", { class: "row" },
      h("button", { class: "btn btn--secondary btn--sm", onClick: refresh }, "↻ Refresh"),
    ),
  ));

  const body = h("div", { class: "view-body stack" });
  root.appendChild(body);

  const peersCard = h("section", { class: "card" },
    h("header", { class: "card__header" }, h("div", {}, h("h2", { class: "card__title" }, "Federated peers"))),
    h("div", { class: "card__body" }, h("div", { class: "row", id: "fd-peers", style: { overflowX: "auto", gap: "12px", padding: "4px 0" } })),
  );
  body.appendChild(peersCard);

  const graphCard = h("section", { class: "card" },
    h("header", { class: "card__header" },
      h("div", {},
        h("h2", { class: "card__title" }, "Knowledge graph"),
        h("div", { class: "card__subtitle", id: "fd-gsub" }, "Select a peer to scope"),
      ),
    ),
    h("div", { class: "card__body" },
      h("div", { class: "row", style: { marginBottom: "12px" } },
        h("input", { class: "input", placeholder: "Search entities…",
          dataset: { role: "view-filter" },
          onInput: (e) => { entityQ = e.target.value.toLowerCase(); renderGraph(); } }),
      ),
      h("div", { id: "fd-graph", style: { height: "460px" } }),
    ),
  );
  body.appendChild(graphCard);

  let peers = [], graph = { nodes: [], edges: [] }, scopePeer = null, entityQ = "";
  refresh();
  const sc = cmdk.scope("federation", [
    { id: "fed:refresh", group: "Actions", title: "Refresh federation", run: refresh },
  ]);

  async function refresh() {
    const [s, c] = await Promise.all([
      tryApi("/api/v4/federated/snapshot"),
      tryApi("/api/v5/federated/close"),
    ]);
    peers = (s.ok && (s.data?.peers || s.data)) || demoPeers();
    graph = (c.ok && c.data?.graph) || demoGraph(peers);
    renderPeers();
    renderGraph();
  }

  function renderPeers() {
    const host = document.getElementById("fd-peers");
    host.innerHTML = "";
    peers.forEach((p) => {
      const tile = h("button", {
        class: "card",
        style: { padding: "12px 14px", minWidth: "220px", textAlign: "left", cursor: "pointer",
                 background: scopePeer?.id === p.id ? "color-mix(in oklab, var(--accent) 12%, transparent)" : "" },
        onClick: () => { scopePeer = (scopePeer?.id === p.id ? null : p); renderGraph(); renderPeers(); },
      },
        h("div", { class: "row", style: { justifyContent: "space-between" } },
          h("span", { class: "mono", style: { color: "var(--text-strong)" } }, p.id),
          h("span", { class: `badge badge--${p.status === "synced" ? "ok" : "warn"}` }, p.status || "online"),
        ),
        h("div", { class: "muted", style: { marginTop: "4px", fontSize: "11px" } }, `${p.region || "—"} · ${num(p.entities, 0)} entities`),
        h("div", { class: "muted", style: { marginTop: "2px", fontSize: "11px" } }, "last sync " + relTime(p.last_sync)),
      );
      host.appendChild(tile);
    });
  }

  function renderGraph() {
    const host = document.getElementById("fd-graph");
    const sub  = document.getElementById("fd-gsub");
    let { nodes, edges } = graph;
    if (scopePeer) {
      nodes = nodes.filter((n) => n.peer === scopePeer.id || edges.some((e) => (e.from === n.id || e.to === n.id) && (nodes.find((x) => x.id === e.from)?.peer === scopePeer.id || nodes.find((x) => x.id === e.to)?.peer === scopePeer.id)));
    }
    if (entityQ) nodes = nodes.filter((n) => (n.label || n.id).toLowerCase().includes(entityQ));
    const ids = new Set(nodes.map((n) => n.id));
    const es  = edges.filter((e) => ids.has(e.from) && ids.has(e.to));
    sub.textContent = `${nodes.length} entities · ${es.length} relations${scopePeer ? " · scoped to " + scopePeer.id : ""}`;

    chart.graph(host, {
      nodes: nodes.map((n) => ({ id: n.id, label: n.label || n.id, size: 12, color: peerColor(n.peer) })),
      edges: es.map((e) => ({ from: e.from, to: e.to, label: e.relation })),
      onNodeClick: (nd) => {
        const entity = nodes.find((n) => n.id === nd.id);
        sheet({
          title: entity.label || entity.id,
          subtitle: entity.peer ? ("contributed by " + entity.peer) : "",
          body: h("div", { class: "stack" },
            h("div", { class: "grid grid-3" },
              h("div", { class: "kpi" }, h("span", { class: "kpi__label" }, "Type"),     h("span", { class: "kpi__value" }, entity.type || "—")),
              h("div", { class: "kpi" }, h("span", { class: "kpi__label" }, "Provenance"), h("span", { class: "kpi__value", style: { fontSize: "14px" } }, entity.peer || "local")),
              h("div", { class: "kpi" }, h("span", { class: "kpi__label" }, "Proof"),    h("span", { class: "kpi__value mono", style: { fontSize: "14px" } }, (entity.proof || "sig-unsigned").slice(0, 10) + "…")),
            ),
            h("h3", {}, "Relations"),
            ...es.filter((e) => e.from === entity.id || e.to === entity.id).map((e) =>
              h("div", { class: "row", style: { borderBottom: "1px solid var(--border)", padding: "8px 0" } },
                h("span", { class: "mono" }, e.from), h("span", { class: "muted" }, e.relation || "—"), h("span", { class: "mono" }, e.to),
                e.proof ? h("span", { class: "mono muted", style: { marginLeft: "auto", fontSize: "11px" } }, e.proof.slice(0, 10) + "…") : null,
              ),
            ),
          ),
        });
      },
    });
  }

  return () => sc();
}

function peerColor(p) {
  if (!p) return "var(--ink-5)";
  const palette = ["--signal-cyan","--signal-violet","--signal-lime","--signal-amber","--signal-pink","--signal-blue"];
  let h = 0; for (const c of String(p)) h = (h * 31 + c.charCodeAt(0)) % palette.length;
  return `var(${palette[h]})`;
}
function demoPeers() {
  return [
    { id: "peer-us-east",  region: "us-east-1",  entities: 1280, last_sync: Date.now() - 32_000, status: "synced" },
    { id: "peer-eu-west",  region: "eu-west-2",  entities: 1024, last_sync: Date.now() - 95_000, status: "synced" },
    { id: "peer-ap-south", region: "ap-south-1", entities:  812, last_sync: Date.now() - 420_000, status: "syncing" },
    { id: "peer-local",    region: "local",      entities:  96,  last_sync: Date.now() - 5_000,   status: "synced" },
  ];
}
function demoGraph(peers) {
  const nodes = [];
  const edges = [];
  const kinds = ["svc", "incident", "playbook", "sla", "alert"];
  peers.forEach((p, pi) => {
    for (let i = 0; i < 6; i++) {
      const id = `${p.id}:${kinds[i%kinds.length]}/${i}`;
      nodes.push({ id, label: `${kinds[i%kinds.length]}-${pi}${i}`, peer: p.id, type: kinds[i%kinds.length], proof: randomHex(40) });
    }
  });
  for (let i = 0; i < nodes.length - 1; i++) {
    if (Math.random() < 0.5) edges.push({ from: nodes[i].id, to: nodes[(i + 3) % nodes.length].id, relation: "related_to", proof: randomHex(40) });
  }
  return { nodes, edges };
}
function randomHex(n) { let s = ""; const c = "0123456789abcdef"; for (let i = 0; i < n; i++) s += c[Math.random() * 16 | 0]; return s; }
