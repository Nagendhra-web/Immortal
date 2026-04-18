'use strict';

// ─── Helpers ────────────────────────────────────────────────────────────────

function el(id) { return document.getElementById(id); }

function esc(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function fmtTime(iso) {
  if (!iso) return '—';
  try {
    const d = new Date(iso);
    return d.toLocaleTimeString();
  } catch (_) { return iso; }
}

function fmtUptime(s) {
  if (!s) return '—';
  return String(s);
}

async function fetchJSON(url) {
  const r = await fetch(url);
  if (!r.ok) throw Object.assign(new Error(r.statusText), { status: r.status });
  return r.json();
}

// ─── Panel 1: Header ────────────────────────────────────────────────────────

async function refreshHeader() {
  try {
    const d = await fetchJSON('/api/status');
    el('hdr-version').textContent = d.version ? 'v' + d.version : 'v?';
    el('hdr-mode').textContent = d.mode || 'autonomous';
    el('hdr-uptime').textContent = fmtUptime(d.uptime);
    el('hdr-events').textContent = d.events_processed != null ? Number(d.events_processed).toLocaleString() : '—';
  } catch (_) { /* silently ignore */ }
}

// ─── Panel 2: Service health grid ───────────────────────────────────────────

function statusDotClass(status) {
  if (!status) return 'dot-unknown';
  const s = String(status).toLowerCase();
  if (s === 'healthy') return 'dot-healthy';
  if (s === 'degraded' || s === 'warning') return 'dot-warning';
  if (s === 'critical' || s === 'unhealthy') return 'dot-critical';
  return 'dot-unknown';
}

function renderServiceCard(name, svc) {
  const dotCls = statusDotClass(svc.status);
  const lastSeen = svc.last_seen ? fmtTime(svc.last_seen) : '—';
  return `<div class="svc-card">
    <div class="svc-name" title="${esc(name)}">${esc(name)}</div>
    <div class="svc-status"><span class="dot ${esc(dotCls)}"></span>${esc(svc.status || 'unknown')}</div>
    <div class="svc-last">${esc(lastSeen)}</div>
  </div>`;
}

async function refreshServices() {
  try {
    const d = await fetchJSON('/api/health');
    const services = d.services || {};
    const names = Object.keys(services);
    if (names.length === 0) {
      el('services-grid').innerHTML = '<span class="panel-disabled">No services registered</span>';
      return;
    }
    el('services-grid').innerHTML = names.map(n => renderServiceCard(n, services[n])).join('');
  } catch (_) {
    el('services-grid').innerHTML = '<span class="panel-disabled">Feature not enabled</span>';
  }
}

// ─── Panel 3: Live event stream ──────────────────────────────────────────────

function sevClass(sev) {
  if (!sev) return 'ev-sev-info';
  const s = String(sev).toLowerCase();
  if (s === 'critical' || s === 'error') return 'ev-sev-critical';
  if (s === 'warning' || s === 'warn') return 'ev-sev-warning';
  return 'ev-sev-info';
}

function renderEventRow(ev) {
  const cls = sevClass(ev.severity);
  return `<div class="ev-row">
    <span class="ev-time">${esc(fmtTime(ev.timestamp))}</span>
    <span class="ev-sev ${esc(cls)}">${esc(ev.severity || 'info')}</span>
    <span class="ev-msg">${esc(ev.message || '')}</span>
    <span class="ev-src">${esc(ev.source || '')}</span>
  </div>`;
}

async function refreshEvents() {
  try {
    const r = await fetch('/api/events?limit=20');
    if (!r.ok) throw new Error(r.statusText);
    const events = await r.json();
    if (!Array.isArray(events) || events.length === 0) {
      el('event-list').innerHTML = '<span class="panel-disabled">No events yet</span>';
      return;
    }
    // newest first
    const rows = [...events].reverse().slice(0, 20).map(renderEventRow).join('');
    el('event-list').innerHTML = rows;
  } catch (_) {
    el('event-list').innerHTML = '<span class="panel-disabled">Feature not enabled</span>';
  }
}

// ─── Panel 4: Topology force-directed graph ──────────────────────────────────

const topoState = { nodes: [], edges: [], cycles: new Set(), initialized: false };

function initForce(nodes, edges) {
  const svgEl = el('topo-svg');
  const W = svgEl.clientWidth || 400;
  const H = svgEl.clientHeight || 260;

  nodes.forEach((n, i) => {
    if (!n.x) {
      const angle = (2 * Math.PI * i) / nodes.length;
      n.x = W / 2 + (W * 0.35) * Math.cos(angle);
      n.y = H / 2 + (H * 0.35) * Math.sin(angle);
      n.vx = 0; n.vy = 0;
    }
  });
  return { W, H };
}

function runForce(nodes, edges, iters) {
  const k = 80;     // repulsion constant
  const spring = 0.05; // attraction
  const damp = 0.85;

  for (let iter = 0; iter < iters; iter++) {
    // repulsion between all pairs
    for (let i = 0; i < nodes.length; i++) {
      for (let j = i + 1; j < nodes.length; j++) {
        const a = nodes[i], b = nodes[j];
        const dx = b.x - a.x, dy = b.y - a.y;
        const dist = Math.sqrt(dx * dx + dy * dy) || 1;
        const force = (k * k) / dist;
        const fx = (dx / dist) * force;
        const fy = (dy / dist) * force;
        a.vx -= fx; a.vy -= fy;
        b.vx += fx; b.vy += fy;
      }
    }

    // spring attraction along edges
    const nodeMap = {};
    nodes.forEach(n => { nodeMap[n.id] = n; });
    edges.forEach(e => {
      const a = nodeMap[e.from], b = nodeMap[e.to];
      if (!a || !b) return;
      const dx = b.x - a.x, dy = b.y - a.y;
      const dist = Math.sqrt(dx * dx + dy * dy) || 1;
      const force = (dist - k) * spring;
      const fx = (dx / dist) * force;
      const fy = (dy / dist) * force;
      a.vx += fx; a.vy += fy;
      b.vx -= fx; b.vy -= fy;
    });

    // integrate + damp
    nodes.forEach(n => {
      n.vx *= damp; n.vy *= damp;
      n.x += n.vx; n.y += n.vy;
    });
  }
}

function clampNodes(nodes, W, H) {
  const pad = 30;
  nodes.forEach(n => {
    n.x = Math.max(pad, Math.min(W - pad, n.x));
    n.y = Math.max(pad, Math.min(H - pad, n.y));
  });
}

function renderTopology(nodes, edges, cycles, W, H) {
  const nodeMap = {};
  nodes.forEach(n => { nodeMap[n.id] = n; });

  const lines = edges.map(e => {
    const a = nodeMap[e.from], b = nodeMap[e.to];
    if (!a || !b) return '';
    const isCycle = cycles.has(e.from + '|' + e.to) || cycles.has(e.to + '|' + e.from);
    return `<line x1="${a.x.toFixed(1)}" y1="${a.y.toFixed(1)}" x2="${b.x.toFixed(1)}" y2="${b.y.toFixed(1)}"${isCycle ? ' class="cycle-edge"' : ''}/>`;
  }).join('');

  const circles = nodes.map(n => {
    return `<g>
      <circle cx="${n.x.toFixed(1)}" cy="${n.y.toFixed(1)}" r="14" fill="#161b22" stroke="#58a6ff"/>
      <text x="${n.x.toFixed(1)}" y="${(n.y + 4).toFixed(1)}" text-anchor="middle">${esc(n.label)}</text>
    </g>`;
  }).join('');

  el('topo-svg').innerHTML = lines + circles;
}

function buildTopoGraph(snapshot) {
  // snapshot: { nodes: [{id, ...}], edges: [{from,to,...}] }
  // or snapshot may be a DiGraph with Nodes/Edges keys
  let rawNodes = [], rawEdges = [];

  if (snapshot && Array.isArray(snapshot.nodes)) {
    rawNodes = snapshot.nodes;
    rawEdges = snapshot.edges || [];
  } else if (snapshot && Array.isArray(snapshot.Nodes)) {
    rawNodes = snapshot.Nodes;
    rawEdges = snapshot.Edges || [];
  } else if (snapshot && snapshot.adjacency) {
    // adjacency map: {a: [b,c], ...}
    const seen = new Set();
    Object.entries(snapshot.adjacency).forEach(([from, tos]) => {
      seen.add(from);
      (tos || []).forEach(to => {
        seen.add(to);
        rawEdges.push({ from, to });
      });
    });
    rawNodes = [...seen].map(id => ({ id }));
  } else if (snapshot && typeof snapshot === 'object') {
    // Try to infer from keys: adjacency-style flat object
    const allNodes = new Set();
    Object.entries(snapshot).forEach(([k, v]) => {
      if (Array.isArray(v)) {
        allNodes.add(k);
        v.forEach(t => { allNodes.add(t); rawEdges.push({ from: k, to: t }); });
      }
    });
    rawNodes = [...allNodes].map(id => ({ id }));
  }

  // Normalise node ids
  const nodes = rawNodes.map(n => ({
    id: n.id || n.ID || n.name || n.Name || String(n),
    label: n.label || n.id || n.ID || n.name || n.Name || String(n),
    x: undefined, y: undefined, vx: 0, vy: 0,
  }));

  // Normalise edges
  const edges = rawEdges.map(e => ({
    from: e.from || e.From || e.source || e.Source || '',
    to:   e.to   || e.To   || e.target || e.Target || '',
  })).filter(e => e.from && e.to);

  return { nodes, edges };
}

function detectCycles(nodes, edges) {
  // Simple DFS cycle detection — mark cyclic edges
  const adj = {};
  nodes.forEach(n => { adj[n.id] = []; });
  edges.forEach(e => { if (adj[e.from]) adj[e.from].push(e.to); });

  const WHITE = 0, GRAY = 1, BLACK = 2;
  const color = {};
  nodes.forEach(n => { color[n.id] = WHITE; });
  const cycles = new Set();

  function dfs(u) {
    color[u] = GRAY;
    for (const v of (adj[u] || [])) {
      if (color[v] === GRAY) cycles.add(u + '|' + v);
      else if (color[v] === WHITE) dfs(v);
    }
    color[u] = BLACK;
  }
  nodes.forEach(n => { if (color[n.id] === WHITE) dfs(n.id); });
  return cycles;
}

async function refreshTopology() {
  try {
    const r = await fetch('/api/v5/topology/snapshot');
    if (r.status === 404) {
      el('topo-svg').innerHTML = '<text x="50%" y="50%" text-anchor="middle" fill="#8b949e" font-size="12">Feature not enabled</text>';
      return;
    }
    if (!r.ok) throw new Error(r.statusText);
    const data = await r.json();
    const snapshot = data.snapshot || data;
    const { nodes, edges } = buildTopoGraph(snapshot);

    if (nodes.length === 0) {
      el('topo-svg').innerHTML = '<text x="50%" y="50%" text-anchor="middle" fill="#8b949e" font-size="12">No topology data</text>';
      return;
    }

    // Merge with existing node positions to avoid jitter on refresh
    const existingMap = {};
    topoState.nodes.forEach(n => { existingMap[n.id] = n; });
    nodes.forEach(n => {
      const prev = existingMap[n.id];
      if (prev) { n.x = prev.x; n.y = prev.y; n.vx = prev.vx; n.vy = prev.vy; }
    });

    const { W, H } = initForce(nodes, edges);
    const iters = topoState.initialized ? 5 : 50;
    runForce(nodes, edges, iters);
    clampNodes(nodes, W, H);

    const cycles = detectCycles(nodes, edges);
    topoState.nodes = nodes;
    topoState.edges = edges;
    topoState.cycles = cycles;
    topoState.initialized = true;

    renderTopology(nodes, edges, cycles, W, H);
  } catch (_) {
    el('topo-svg').innerHTML = '<text x="50%" y="50%" text-anchor="middle" fill="#8b949e" font-size="12">Feature not enabled</text>';
  }
}

// ─── Panel 5: Audit ledger ───────────────────────────────────────────────────

function renderAuditRow(entry) {
  const action = entry.action || entry.Action || '—';
  const actor  = entry.actor  || entry.Actor  || '—';
  const detail = entry.detail || entry.Detail || entry.target || entry.Target || '';
  const ok     = entry.ok != null ? entry.ok : (entry.Ok != null ? entry.Ok : true);
  const badge  = ok
    ? '<span class="badge-verified">verified</span>'
    : '<span class="badge-unverified">unverified</span>';
  return `<div class="audit-row">
    <span class="audit-action">${esc(action)}</span>
    <span class="audit-actor">${esc(actor)}</span>
    <span class="audit-detail">${esc(detail)}</span>
    ${badge}
  </div>`;
}

async function refreshAudit() {
  const body = el('audit-body');
  try {
    // Try pqaudit first; fall back to legacy
    let entries = null;
    const r1 = await fetch('/api/v4/audit/entries?limit=10');
    if (r1.ok) {
      const d = await r1.json();
      entries = d.entries || [];
    } else {
      const r2 = await fetch('/api/audit?limit=10');
      if (r2.ok) {
        const d = await r2.json();
        entries = Array.isArray(d) ? d : [];
      }
    }

    if (!entries || entries.length === 0) {
      body.innerHTML = '<span class="panel-disabled">No audit entries</span>';
      return;
    }
    body.innerHTML = entries.map(renderAuditRow).join('');
  } catch (_) {
    body.innerHTML = '<span class="panel-disabled">Feature not enabled</span>';
  }
}

// ─── Panel 6: Healing recommendations ────────────────────────────────────────

function renderRecRow(rec) {
  const rule = rec.rule_name || rec.RuleName || rec.rule || '—';
  const msg  = rec.message   || rec.Message  || '';
  return `<div class="rec-row">
    <div class="rec-rule">${esc(rule)}</div>
    <div class="rec-msg">${esc(msg)}</div>
  </div>`;
}

async function refreshRecs() {
  const body = el('recs-body');
  try {
    const r = await fetch('/api/recommendations');
    if (!r.ok) throw new Error(r.statusText);
    const recs = await r.json();
    if (!Array.isArray(recs) || recs.length === 0) {
      body.innerHTML = '<span class="panel-disabled">No pending recommendations</span>';
      return;
    }
    body.innerHTML = recs.map(renderRecRow).join('');
  } catch (_) {
    body.innerHTML = '<span class="panel-disabled">Feature not enabled</span>';
  }
}

// ─── Refresh orchestration ────────────────────────────────────────────────────

function refresh() {
  refreshHeader();
  refreshServices();
  refreshEvents();
  refreshTopology();
  refreshAudit();
  refreshRecs();
}

// Initial load
refresh();

// Auto-refresh every 5 seconds
setInterval(refresh, 5000);
