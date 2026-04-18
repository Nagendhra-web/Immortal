'use strict';

// ─── Utilities ───────────────────────────────────────────────────────────────

function el(id) { return document.getElementById(id); }

function esc(s) {
  return String(s)
    .replace(/&/g,'&amp;').replace(/</g,'&lt;')
    .replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

function fmtTime(iso) {
  if (!iso) return '—';
  try {
    const d = new Date(iso);
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  } catch (_) { return iso; }
}

function fmtNum(n) {
  if (n == null) return '—';
  return Number(n).toLocaleString();
}

async function fetchJSON(url) {
  const r = await fetch(url);
  if (!r.ok) throw Object.assign(new Error(r.statusText), { status: r.status });
  return r.json();
}

// Count-up animation for big numbers
function countUp(elem, target) {
  if (!elem) return;
  const str = String(target);
  // only animate pure-number values
  if (!/^\d[\d,]*$/.test(str.replace(/,/g,''))) {
    elem.textContent = target;
    return;
  }
  const end = parseInt(str.replace(/,/g,''), 10);
  const start = parseInt(elem.dataset.raw || '0', 10);
  if (start === end) return;
  elem.dataset.raw = end;
  const dur = 300;
  const t0 = performance.now();
  function step(now) {
    const p = Math.min((now - t0) / dur, 1);
    const ease = 1 - Math.pow(1 - p, 3); // ease-out-cubic
    const val = Math.round(start + (end - start) * ease);
    elem.textContent = val.toLocaleString();
    if (p < 1) requestAnimationFrame(step);
    else elem.textContent = end.toLocaleString();
  }
  requestAnimationFrame(step);
}

// ─── State ───────────────────────────────────────────────────────────────────

const state = {
  services:  [],
  events:    [],
  audit:     [],
  auditOk:   null,
  auditCount: 0,
  topology:  { nodes: [], edges: [], cycles: new Set() },
  status:    {},
  topoInited: false,
};

// ─── Routing ─────────────────────────────────────────────────────────────────

const VIEWS = ['overview', 'topology', 'audit', 'terminal'];

function currentView() {
  const h = location.hash.replace('#/', '').split('/')[0];
  return VIEWS.includes(h) ? h : 'overview';
}

function switchView(name) {
  VIEWS.forEach(v => {
    const s = el('view-' + v);
    if (s) s.classList.toggle('view-hidden', v !== name);
  });
  document.querySelectorAll('.nav-link').forEach(a => {
    a.classList.toggle('active', a.dataset.view === name);
  });
  if (name === 'topology') renderTopologyView();
  if (name === 'audit')    renderAuditView();
  if (name === 'terminal') renderTerminalView();
}

window.addEventListener('hashchange', () => switchView(currentView()));

// ─── API refresh ──────────────────────────────────────────────────────────────

async function refreshStatus() {
  try {
    const d = await fetchJSON('/api/status');
    state.status = d;

    // Hero chip
    const node   = d.cluster_id || d.node_id || d.nodeId || 'demo-node';
    const mode   = d.mode || 'autonomous';
    const chipEl = el('hero-chip-text');
    if (chipEl) chipEl.textContent = `NODE · ${node} · ${mode} mode · connected`;

    el('nav-node').textContent = `NODE · ${node}`;

    // Stats
    const up = d.uptime || d.Uptime || '—';
    const upEl = el('stat-uptime');
    if (upEl) upEl.textContent = up;

    const evCount = d.events_processed != null ? fmtNum(d.events_processed) : '—';
    countUp(el('stat-events'), evCount);

    const heal = d.healing_actions != null ? fmtNum(d.healing_actions) : '—';
    countUp(el('stat-healing'), heal);
  } catch (_) {}
}

async function refreshHealth() {
  try {
    const d = await fetchJSON('/api/health');
    const svcs = Array.isArray(d.services) ? d.services
      : (d.services && typeof d.services === 'object' ? Object.values(d.services) : []);
    state.services = svcs;

    const healthy = svcs.filter(s => (s.status||'').toLowerCase() === 'healthy').length;
    countUp(el('stat-healthy'), `${healthy}/${svcs.length}`);
    const sumEl = el('svc-summary');
    if (sumEl) sumEl.textContent = `${healthy} of ${svcs.length} healthy`;

    renderServices();
  } catch (_) {
    const g = el('services-grid');
    if (g) g.innerHTML = '<div class="svc-loading">No services registered</div>';
    const statEl = el('stat-healthy');
    if (statEl && statEl.textContent === '—') statEl.textContent = '0';
  }
}

async function refreshEvents() {
  try {
    const evts = await fetchJSON('/api/events?limit=20');
    if (!Array.isArray(evts)) return;
    state.events = evts;
    renderFeed();
    renderTimeline();
  } catch (_) {}
}

async function refreshAuditData() {
  try {
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
    state.audit = entries || [];

    // Verify chain
    try {
      const rv = await fetch('/api/v4/audit/verify');
      if (rv.ok) {
        const vd = await rv.json();
        state.auditOk    = vd.ok !== false;
        state.auditCount = vd.count || state.audit.length;
      } else {
        state.auditOk = state.audit.length > 0 ? true : null;
        state.auditCount = state.audit.length;
      }
    } catch (_) {
      state.auditOk = state.audit.length > 0 ? true : null;
      state.auditCount = state.audit.length;
    }

    if (currentView() === 'audit') renderAuditView();
  } catch (_) {
    state.audit = [];
    state.auditOk = null;
    if (currentView() === 'audit') renderAuditView();
  }
}

async function refreshTopologyData() {
  try {
    const dep = await fetchJSON('/api/dependencies');
    const rawNodes = (dep.nodes || []).map(n => ({ id: n.name, label: n.name }));
    const rawEdges = [];
    (dep.nodes || []).forEach(n => {
      (n.dependencies || []).forEach(to => rawEdges.push({ from: n.name, to }));
    });

    // Merge existing positions to avoid jitter
    const existing = {};
    state.topology.nodes.forEach(n => { existing[n.id] = n; });
    rawNodes.forEach(n => {
      const p = existing[n.id];
      if (p) { n.x = p.x; n.y = p.y; n.vx = p.vx || 0; n.vy = p.vy || 0; }
      else     { n.vx = 0; n.vy = 0; }
    });

    const cycles = detectCycles(rawNodes, rawEdges);
    state.topology = { nodes: rawNodes, edges: rawEdges, cycles };

    el('ts-nodes').textContent = rawNodes.length;
    el('ts-edges').textContent = rawEdges.length;
    el('ts-cycles').textContent = cycles.size;

    if (currentView() === 'topology') renderTopologyView();
  } catch (_) {
    // topology not enabled — empty state shown in renderTopologyView
  }
}

function refresh() {
  refreshStatus();
  refreshHealth();
  refreshEvents();
  refreshAuditData();
  refreshTopologyData();
}

// ─── Overview renders ─────────────────────────────────────────────────────────

function sevClass(sev) {
  if (!sev) return 'info';
  const s = sev.toLowerCase();
  if (s === 'critical' || s === 'error') return 'critical';
  if (s === 'warning'  || s === 'warn')  return 'warning';
  return 'info';
}

function renderFeed() {
  const list = el('feed-list');
  if (!list) return;
  const evts = [...state.events].reverse().slice(0, 20);
  if (!evts.length) {
    list.innerHTML = '<div class="feed-empty">No events yet</div>';
    return;
  }
  list.innerHTML = evts.map(ev => {
    const cls = sevClass(ev.severity);
    return `<div class="feed-row">
      <span class="feed-time">${esc(fmtTime(ev.timestamp))}</span>
      <span class="feed-sev sev-${cls}">${esc((ev.severity || 'info').toUpperCase())}</span>
      <span class="feed-msg">${esc(ev.message || '')}</span>
      <span class="feed-src">${esc(ev.source || '')}</span>
    </div>`;
  }).join('');
}

function renderTimeline() {
  const ribbon = el('timeline-ribbon');
  if (!ribbon) return;
  const evts = state.events;
  if (!evts.length) {
    ribbon.innerHTML = '<div class="timeline-empty">No events yet</div>';
    return;
  }
  ribbon.innerHTML = evts.map(ev => {
    const cls = sevClass(ev.severity);
    const tip = `${fmtTime(ev.timestamp)} · ${(ev.severity||'info').toUpperCase()} · ${(ev.message||'').slice(0,60)}`;
    return `<span class="tl-dot tl-${cls}" data-tip="${esc(tip)}"></span>`;
  }).join('');
}

function renderServices() {
  const grid = el('services-grid');
  if (!grid) return;
  const svcs = state.services;
  if (!svcs.length) {
    grid.innerHTML = '<div class="svc-loading">No services registered</div>';
    return;
  }
  grid.innerHTML = svcs.map(svc => {
    const s = (svc.status || 'unknown').toLowerCase();
    let dotCls = 'svc-dot-unknown';
    if (s === 'healthy') dotCls = 'svc-dot-healthy';
    else if (s === 'degraded' || s === 'warning') dotCls = 'svc-dot-warning';
    else if (s === 'critical' || s === 'unhealthy') dotCls = 'svc-dot-critical';
    const last = svc.last_check ? fmtTime(svc.last_check) : (svc.last_seen ? fmtTime(svc.last_seen) : '');
    return `<div class="svc-card">
      <div class="svc-card-head">
        <span class="svc-dot ${dotCls}"></span>
        <span class="svc-name" title="${esc(svc.name||'')}">${esc(svc.name||'?')}</span>
      </div>
      <div class="svc-status-text">${esc(svc.status||'unknown')}</div>
      ${last ? `<div class="svc-last">${esc(last)}</div>` : ''}
    </div>`;
  }).join('');
}

// ─── Audit view ───────────────────────────────────────────────────────────────

function renderAuditView() {
  const verdict = el('audit-verdict');
  const icon    = el('audit-verdict-icon');
  const text    = el('audit-verdict-text');
  const meta    = el('audit-meta');
  const chain   = el('audit-chain');

  if (!verdict) return;

  if (state.auditOk === null && !state.audit.length) {
    verdict.className = 'audit-verdict';
    if (icon) icon.textContent = '◌';
    if (text) text.textContent = 'NO AUDIT DATA';
    if (meta) meta.textContent = 'Enable pqaudit with --pqaudit flag';
    if (chain) chain.innerHTML = '<div class="audit-chain-empty">No audit entries — run with --pqaudit to enable signed audit chain.</div>';
    return;
  }

  if (state.auditOk === true) {
    verdict.className = 'audit-verdict audit-verdict-ok';
    if (icon) icon.textContent = '✓';
    if (text) text.textContent = 'CHAIN VERIFIED';
    if (meta) meta.textContent = `${state.auditCount} entries · tamper-proof · post-quantum signed`;
  } else if (state.auditOk === false) {
    verdict.className = 'audit-verdict audit-verdict-fail';
    if (icon) icon.textContent = '✗';
    if (text) text.textContent = 'CHAIN TAMPERED';
    if (meta) meta.textContent = 'Integrity check failed — investigate immediately';
  } else {
    verdict.className = 'audit-verdict';
    if (icon) icon.textContent = '◌';
    if (text) text.textContent = 'AUDIT LOG';
    if (meta) meta.textContent = `${state.audit.length} entries`;
  }

  if (!chain) return;
  if (!state.audit.length) {
    chain.innerHTML = '<div class="audit-chain-empty">No audit entries yet</div>';
    return;
  }

  chain.innerHTML = state.audit.map((entry, i) => {
    const action = esc(entry.action || entry.Action || '—');
    const actor  = esc(entry.actor  || entry.Actor  || '—');
    const detail = esc(entry.detail || entry.Detail || entry.target || entry.Target || '');
    const ts     = esc(fmtTime(entry.timestamp || entry.Timestamp || ''));
    const hash   = entry.hash || entry.Hash || entry.signature || '';
    const hashShort = hash ? esc(String(hash).slice(0, 12) + '…') : '';
    const ok     = entry.ok != null ? entry.ok : true;
    const signed = !!(hash);
    const delay  = `animation-delay:${i * 0.04}s`;

    return `<div class="audit-block ${signed ? 'audit-block-signed' : ''}" style="${delay}">
      <div class="audit-block-left">
        <div class="audit-block-action">${action}</div>
        <div class="audit-block-actor">${actor}</div>
        ${detail ? `<div class="audit-block-detail">${detail}</div>` : ''}
        ${hashShort ? `<div class="audit-block-hash">${hashShort}</div>` : ''}
      </div>
      <div class="audit-block-right">
        <span class="audit-badge ${ok ? 'audit-badge-ok' : 'audit-badge-fail'}">${ok ? 'Verified' : 'Failed'}</span>
        ${signed ? '<span class="audit-badge audit-badge-signed">Signed</span>' : ''}
        ${ts ? `<span class="audit-block-time">${ts}</span>` : ''}
      </div>
    </div>`;
  }).join('');
}

// ─── Terminal view ────────────────────────────────────────────────────────────

let termCount = 0;

function renderTerminalView() {
  // Render current events into terminal lines
  const body = el('term-body');
  if (!body) return;
  const evts = [...state.events].reverse();
  if (!evts.length) return;

  body.innerHTML = '';
  evts.forEach(ev => {
    const cls = sevClass(ev.severity);
    const termCls = cls === 'critical' ? 'term-line-crit'
                  : cls === 'warning'  ? 'term-line-warn'
                  : 'term-line-info';
    const line = document.createElement('div');
    line.className = `term-line ${termCls}`;
    const ts  = fmtTime(ev.timestamp);
    const sev = (ev.severity || 'INFO').toUpperCase().padEnd(8);
    const src = ev.source ? `[${ev.source}] ` : '';
    line.textContent = `${ts}  ${sev}  ${src}${ev.message || ''}`;
    body.appendChild(line);
  });

  termCount = evts.length;
  const tc = el('term-count');
  if (tc) tc.textContent = `${termCount} events`;

  body.scrollTop = body.scrollHeight;
}

// ─── Topology force graph ─────────────────────────────────────────────────────

function detectCycles(nodes, edges) {
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

// Force simulation
function initPositions(nodes, W, H) {
  nodes.forEach((n, i) => {
    if (n.x == null) {
      const angle = (2 * Math.PI * i) / nodes.length;
      n.x = W / 2 + (W * 0.32) * Math.cos(angle);
      n.y = H / 2 + (H * 0.32) * Math.sin(angle);
      n.vx = 0; n.vy = 0;
    }
  });
}

function runForce(nodes, edges, iters) {
  const k = 90, spring = 0.04, damp = 0.82;
  const nodeMap = {};
  nodes.forEach(n => { nodeMap[n.id] = n; });

  for (let it = 0; it < iters; it++) {
    for (let i = 0; i < nodes.length; i++) {
      for (let j = i + 1; j < nodes.length; j++) {
        const a = nodes[i], b = nodes[j];
        const dx = b.x - a.x, dy = b.y - a.y;
        const dist = Math.sqrt(dx*dx + dy*dy) || 1;
        const f = (k*k) / dist;
        a.vx -= (dx/dist)*f; a.vy -= (dy/dist)*f;
        b.vx += (dx/dist)*f; b.vy += (dy/dist)*f;
      }
    }
    edges.forEach(e => {
      const a = nodeMap[e.from], b = nodeMap[e.to];
      if (!a || !b) return;
      const dx = b.x - a.x, dy = b.y - a.y;
      const dist = Math.sqrt(dx*dx + dy*dy) || 1;
      const f = (dist - k) * spring;
      a.vx += (dx/dist)*f; a.vy += (dy/dist)*f;
      b.vx -= (dx/dist)*f; b.vy -= (dy/dist)*f;
    });
    nodes.forEach(n => {
      n.vx *= damp; n.vy *= damp;
      n.x += n.vx; n.y += n.vy;
    });
  }
}

function clamp(nodes, W, H) {
  const pad = 40;
  nodes.forEach(n => {
    n.x = Math.max(pad, Math.min(W - pad, n.x));
    n.y = Math.max(pad, Math.min(H - pad, n.y));
  });
}

// Topology interaction state
const topo = {
  pan: { x: 0, y: 0 },
  zoom: 1,
  dragging: null,
  dragOffset: { x: 0, y: 0 },
  flowAnim: null,
};

function renderTopologyView() {
  const svg    = el('topo-svg');
  const empty  = el('topo-empty');
  const wrap   = el('topo-canvas-wrap');
  if (!svg || !wrap) return;

  const { nodes, edges, cycles } = state.topology;

  if (!nodes.length) {
    svg.style.display = 'none';
    if (empty) empty.style.display = 'flex';
    return;
  }

  svg.style.display = 'block';
  if (empty) empty.style.display = 'none';

  const W = wrap.clientWidth  || 800;
  const H = wrap.clientHeight || 500;

  if (!state.topoInited) {
    initPositions(nodes, W, H);
    runForce(nodes, edges, 60);
    clamp(nodes, W, H);
    state.topoInited = true;
  } else {
    runForce(nodes, edges, 4);
    clamp(nodes, W, H);
  }

  drawTopology(svg, nodes, edges, cycles, W, H);
  setupTopoInteraction(svg, wrap, nodes, edges, cycles);
  startFlowAnimation(svg, nodes, edges, cycles);
}

function drawTopology(svg, nodes, edges, cycles, W, H) {
  const nodeMap = {};
  nodes.forEach(n => { nodeMap[n.id] = n; });

  // Edges
  const edgesG = el('topo-edges');
  edgesG.innerHTML = edges.map(e => {
    const a = nodeMap[e.from], b = nodeMap[e.to];
    if (!a || !b) return '';
    const isCycle = cycles.has(e.from + '|' + e.to) || cycles.has(e.to + '|' + e.from);
    const cls = isCycle ? 'topo-edge-cycle' : 'topo-edge';
    // Shorten line to not overlap node circles (r=22)
    const dx = b.x - a.x, dy = b.y - a.y;
    const dist = Math.sqrt(dx*dx + dy*dy) || 1;
    const r = 24;
    const x1 = a.x + (dx/dist)*r, y1 = a.y + (dy/dist)*r;
    const x2 = b.x - (dx/dist)*r, y2 = b.y - (dy/dist)*r;
    return `<line class="${cls}" x1="${x1.toFixed(1)}" y1="${y1.toFixed(1)}" x2="${x2.toFixed(1)}" y2="${y2.toFixed(1)}" data-from="${esc(e.from)}" data-to="${esc(e.to)}"/>`;
  }).join('');

  // Nodes
  const nodesG = el('topo-nodes');
  nodesG.innerHTML = nodes.map(n => {
    const inCycle = [...cycles].some(c => c.startsWith(n.id+'|') || c.endsWith('|'+n.id));
    const cls = inCycle ? 'topo-node-cycle' : 'topo-node-normal';
    const label = (n.label || n.id).length > 10
      ? (n.label || n.id).slice(0, 9) + '…'
      : (n.label || n.id);
    return `<g class="topo-node-g" data-id="${esc(n.id)}">
      <circle class="topo-node-circle ${cls}" cx="${n.x.toFixed(1)}" cy="${n.y.toFixed(1)}" r="22"/>
      <text class="topo-node-label" x="${n.x.toFixed(1)}" y="${n.y.toFixed(1)}">${esc(label)}</text>
    </g>`;
  }).join('');
}

function startFlowAnimation(svg, nodes, edges, cycles) {
  // Cancel any existing animation
  if (topo.flowAnim) { cancelAnimationFrame(topo.flowAnim); topo.flowAnim = null; }
  if (!edges.length) return;

  const nodeMap = {};
  nodes.forEach(n => { nodeMap[n.id] = n; });

  // Create one flow dot per edge
  const flowLayer = document.createElementNS('http://www.w3.org/2000/svg', 'g');
  flowLayer.id = 'flow-layer';
  // Remove old flow layer if any
  const old = svg.querySelector('#flow-layer');
  if (old) old.remove();
  svg.appendChild(flowLayer);

  const dots = edges.map(e => {
    const a = nodeMap[e.from], b = nodeMap[e.to];
    if (!a || !b) return null;
    const isCycle = cycles.has(e.from + '|' + e.to) || cycles.has(e.to + '|' + e.from);
    const dot = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
    dot.setAttribute('r', '3');
    dot.setAttribute('fill', isCycle ? '#ffa657' : '#58a6ff');
    dot.setAttribute('opacity', '0.75');
    flowLayer.appendChild(dot);
    return { dot, a, b, t: Math.random() }; // stagger start
  }).filter(Boolean);

  const speed = 0.004; // fraction of path per frame

  function animate() {
    dots.forEach(d => {
      d.t = (d.t + speed) % 1;
      const x = d.a.x + (d.b.x - d.a.x) * d.t;
      const y = d.a.y + (d.b.y - d.a.y) * d.t;
      d.dot.setAttribute('cx', x.toFixed(2));
      d.dot.setAttribute('cy', y.toFixed(2));
      // Fade near source/dest
      const fade = Math.min(d.t * 6, 1) * Math.min((1 - d.t) * 6, 1);
      d.dot.setAttribute('opacity', (fade * 0.8).toFixed(2));
    });
    topo.flowAnim = requestAnimationFrame(animate);
  }
  topo.flowAnim = requestAnimationFrame(animate);
}

function setupTopoInteraction(svg, wrap, nodes, edges, cycles) {
  // Drag nodes
  const nodesG = el('topo-nodes');

  function getMouseSVG(e) {
    const rect = svg.getBoundingClientRect();
    return { x: e.clientX - rect.left, y: e.clientY - rect.top };
  }

  nodesG.onmousedown = function(e) {
    const g = e.target.closest('.topo-node-g');
    if (!g) return;
    const id = g.dataset.id;
    const node = nodes.find(n => n.id === id);
    if (!node) return;
    e.preventDefault();
    const pos = getMouseSVG(e);
    topo.dragging = { node, ox: pos.x - node.x, oy: pos.y - node.y };
  };

  svg.onmousemove = function(e) {
    if (!topo.dragging) return;
    const pos = getMouseSVG(e);
    topo.dragging.node.x = pos.x - topo.dragging.ox;
    topo.dragging.node.y = pos.y - topo.dragging.oy;
    drawTopology(svg, nodes, edges, cycles,
      wrap.clientWidth || 800, wrap.clientHeight || 500);
  };

  svg.onmouseup = function() { topo.dragging = null; };
  svg.onmouseleave = function() { topo.dragging = null; };

  // Reset button
  const resetBtn = el('topo-reset');
  if (resetBtn) {
    resetBtn.onclick = function() {
      nodes.forEach(n => { n.x = undefined; n.y = undefined; n.vx = 0; n.vy = 0; });
      state.topoInited = false;
      renderTopologyView();
    };
  }
}

// ─── Bootstrap ───────────────────────────────────────────────────────────────

switchView(currentView());
refresh();
setInterval(refresh, 2000);
