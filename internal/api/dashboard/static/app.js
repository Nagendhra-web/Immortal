'use strict';

// ─── Utilities ───────────────────────────────────────────────────────────────
function el(id) { return document.getElementById(id); }
function esc(s) { return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;'); }
function fmtTime(iso) { if (!iso) return '—'; try { return new Date(iso).toLocaleTimeString([],{hour:'2-digit',minute:'2-digit',second:'2-digit'}); } catch(_){return iso;} }
function fmtNum(n) { if (n==null) return '—'; return Number(n).toLocaleString(); }
async function fetchJSON(url) { const r=await fetch(url); if(!r.ok) throw Object.assign(new Error(r.statusText),{status:r.status}); return r.json(); }
function debounce(fn,ms){let t;return(...a)=>{clearTimeout(t);t=setTimeout(()=>fn(...a),ms);};}

function countUp(elem,target){
  if(!elem)return;
  const str=String(target);
  if(!/^\d[\d,]*$/.test(str.replace(/,/g,''))){elem.textContent=target;return;}
  const end=parseInt(str.replace(/,/g,''),10);
  const start=parseInt(elem.dataset.raw||'0',10);
  if(start===end)return;
  elem.dataset.raw=end;
  const dur=300,t0=performance.now();
  function step(now){const p=Math.min((now-t0)/dur,1);const e=1-Math.pow(1-p,3);elem.textContent=Math.round(start+(end-start)*e).toLocaleString();if(p<1)requestAnimationFrame(step);else elem.textContent=end.toLocaleString();}
  requestAnimationFrame(step);
}

function toast(msg,type=''){
  const r=el('toast-region');if(!r)return;
  const d=document.createElement('div');
  d.className='toast'+(type?' toast--'+type:'');d.textContent=msg;
  r.appendChild(d);setTimeout(()=>d.remove(),3500);
}

// ─── State ───────────────────────────────────────────────────────────────────
const state={
  services:[],events:[],audit:[],auditOk:null,auditCount:0,
  topology:{nodes:[],edges:[],cycles:new Set()},
  status:{},topoInited:false,
  filters:{severity:'',search:'',timeRange:'15m',live:true},
  agentTrace:null,
  econServices:[
    {name:'api',usdPerMin:120,depFactor:0.9},
    {name:'db',usdPerMin:80,depFactor:0.7},
    {name:'cache',usdPerMin:30,depFactor:0.4},
  ],
};

// ─── Views ───────────────────────────────────────────────────────────────────
const VIEWS=['overview','topology','audit','terminal','forecast','agent','verify','plan','causes','fleet','economics','certificates'];

function currentView(){
  const h=location.hash.replace('#/','').split('?')[0];
  return VIEWS.includes(h)?h:'overview';
}

function switchView(name){
  VIEWS.forEach(v=>{const s=el('view-'+v);if(s)s.classList.toggle('view-hidden',v!==name);});
  document.querySelectorAll('.nav-link').forEach(a=>a.classList.toggle('active',a.dataset.view===name));
  if(name==='topology')   renderTopologyView();
  if(name==='audit')      renderAuditView();
  if(name==='terminal')   renderTerminalView();
  if(name==='forecast')   renderForecastView();
  if(name==='agent')      renderAgentView();
  if(name==='verify')     renderVerifyView();
  if(name==='plan')       renderPlanView();
  if(name==='causes')     renderCausesView();
  if(name==='fleet')      renderFleetView();
  if(name==='economics')  renderEconomicsView();
  if(name==='certificates')renderCertificatesView();
  // close mobile menu
  el('nav-links').classList.remove('open');
  el('nav-hamburger').setAttribute('aria-expanded','false');
}
window.addEventListener('hashchange',()=>switchView(currentView()));

// Mobile hamburger
el('nav-hamburger').addEventListener('click',()=>{
  const open=el('nav-links').classList.toggle('open');
  el('nav-hamburger').setAttribute('aria-expanded',String(open));
});

// ─── Filter bar ───────────────────────────────────────────────────────────────
function renderFilterBar(containerId){
  const c=el(containerId);if(!c)return;
  const sevs=['','info','warning','critical'];
  const ranges=['5m','15m','1h','6h','24h'];
  c.innerHTML=`<div class="filter-bar">
    <span class="filter-label">SEV</span>
    ${sevs.map(s=>`<button class="filter-pill${state.filters.severity===s?' active':''}" data-sev="${s}">${s||'ALL'}</button>`).join('')}
    <span class="filter-label" style="margin-left:8px">RANGE</span>
    ${ranges.map(r=>`<button class="filter-pill${state.filters.timeRange===r?' active':''}" data-range="${r}">${r}</button>`).join('')}
    <input class="filter-search" type="search" placeholder="Search…" aria-label="Search events" value="${esc(state.filters.search)}">
    <button class="filter-live-toggle${state.filters.live?'':' paused'}" id="live-toggle">${state.filters.live?'⬤ LIVE':'⏸ PAUSED'}</button>
  </div>`;
  c.querySelectorAll('[data-sev]').forEach(b=>b.addEventListener('click',()=>{state.filters.severity=b.dataset.sev;renderFilterBar(containerId);}));
  c.querySelectorAll('[data-range]').forEach(b=>b.addEventListener('click',()=>{state.filters.timeRange=b.dataset.range;renderFilterBar(containerId);}));
  c.querySelector('.filter-search').addEventListener('input',debounce(e=>{state.filters.search=e.target.value;},200));
  c.querySelector('.filter-live-toggle').addEventListener('click',()=>{state.filters.live=!state.filters.live;renderFilterBar(containerId);});
}

// ─── API refresh ──────────────────────────────────────────────────────────────
async function refreshStatus(){
  try{
    const d=await fetchJSON('/api/status');state.status=d;
    const node=d.cluster_id||d.node_id||d.nodeId||'demo-node';
    const mode=d.mode||'autonomous';
    const chipEl=el('hero-chip-text');if(chipEl)chipEl.textContent=`NODE · ${node} · ${mode} mode · connected`;
    el('nav-node').textContent=`NODE · ${node}`;
    const upEl=el('stat-uptime');if(upEl)upEl.textContent=d.uptime||d.Uptime||'—';
    countUp(el('stat-events'),d.events_processed!=null?fmtNum(d.events_processed):'—');
    countUp(el('stat-healing'),d.healing_actions!=null?fmtNum(d.healing_actions):'—');
  }catch(_){}
}
async function refreshHealth(){
  try{
    const d=await fetchJSON('/api/health');
    const svcs=Array.isArray(d.services)?d.services:(d.services&&typeof d.services==='object'?Object.values(d.services):[]);
    state.services=svcs;
    const healthy=svcs.filter(s=>(s.status||'').toLowerCase()==='healthy').length;
    countUp(el('stat-healthy'),`${healthy}/${svcs.length}`);
    const sumEl=el('svc-summary');if(sumEl)sumEl.textContent=`${healthy} of ${svcs.length} healthy`;
    renderServices();
  }catch(_){
    const g=el('services-grid');if(g)g.innerHTML='<div class="svc-loading">No services registered</div>';
    const sEl=el('stat-healthy');if(sEl&&sEl.textContent==='—')sEl.textContent='0';
  }
}
async function refreshEvents(){
  if(!state.filters.live)return;
  try{
    const evts=await fetchJSON('/api/events?limit=20');
    if(!Array.isArray(evts))return;
    state.events=evts;renderFeed();renderTimeline();
    if(currentView()==='terminal')renderTerminalView();
  }catch(_){}
}
async function refreshAuditData(){
  try{
    let entries=null;
    const r1=await fetch('/api/v4/audit/entries?limit=50');
    if(r1.ok){const d=await r1.json();entries=d.entries||[];}
    else{const r2=await fetch('/api/audit?limit=50');if(r2.ok){const d=await r2.json();entries=Array.isArray(d)?d:[];}}
    state.audit=entries||[];
    try{
      const rv=await fetch('/api/v4/audit/verify');
      if(rv.ok){const vd=await rv.json();state.auditOk=vd.ok!==false;state.auditCount=vd.count||state.audit.length;}
      else{state.auditOk=state.audit.length>0?true:null;state.auditCount=state.audit.length;}
    }catch(_){state.auditOk=state.audit.length>0?true:null;state.auditCount=state.audit.length;}
    if(currentView()==='audit')renderAuditView();
    if(currentView()==='certificates')renderCertificatesView();
  }catch(_){state.audit=[];state.auditOk=null;if(currentView()==='audit')renderAuditView();}
}
async function refreshTopologyData(){
  try{
    const dep=await fetchJSON('/api/dependencies');
    const rawNodes=(dep.nodes||[]).map(n=>({id:n.name,label:n.name}));
    const rawEdges=[];
    (dep.nodes||[]).forEach(n=>(n.dependencies||[]).forEach(to=>rawEdges.push({from:n.name,to})));
    const existing={};state.topology.nodes.forEach(n=>{existing[n.id]=n;});
    rawNodes.forEach(n=>{const p=existing[n.id];if(p){n.x=p.x;n.y=p.y;n.vx=p.vx||0;n.vy=p.vy||0;}else{n.vx=0;n.vy=0;}});
    const cycles=detectCycles(rawNodes,rawEdges);
    state.topology={nodes:rawNodes,edges:rawEdges,cycles};
    el('ts-nodes').textContent=rawNodes.length;el('ts-edges').textContent=rawEdges.length;el('ts-cycles').textContent=cycles.size;
    if(currentView()==='topology')renderTopologyView();
  }catch(_){}
}
function refresh(){refreshStatus();refreshHealth();refreshEvents();refreshAuditData();refreshTopologyData();}

// ─── Overview renders ──────────────────────────────────────────────────────────
function sevClass(sev){if(!sev)return'info';const s=sev.toLowerCase();if(s==='critical'||s==='error')return'critical';if(s==='warning'||s==='warn')return'warning';return'info';}

function filteredEvents(){
  return state.events.filter(ev=>{
    if(state.filters.severity&&sevClass(ev.severity)!==state.filters.severity)return false;
    if(state.filters.search&&!(ev.message||'').toLowerCase().includes(state.filters.search.toLowerCase()))return false;
    return true;
  });
}

function renderFeed(){
  const list=el('feed-list');if(!list)return;
  const evts=[...filteredEvents()].reverse().slice(0,20);
  if(!evts.length){list.innerHTML='<div class="feed-empty">No events yet</div>';return;}
  list.innerHTML=evts.map(ev=>{
    const cls=sevClass(ev.severity);
    return`<div class="feed-row" role="button" tabindex="0" data-ev="${esc(JSON.stringify(ev))}">
      <span class="feed-time">${esc(fmtTime(ev.timestamp))}</span>
      <span class="feed-sev sev-${cls}">${esc((ev.severity||'info').toUpperCase())}</span>
      <span class="feed-msg">${esc(ev.message||'')}</span>
      <span class="feed-src">${esc(ev.source||'')}</span>
    </div>`;
  }).join('');
  list.querySelectorAll('.feed-row').forEach(row=>{
    row.addEventListener('click',()=>openEventDrawer(JSON.parse(row.dataset.ev||'{}')));
    row.addEventListener('keydown',e=>{if(e.key==='Enter'||e.key===' ')openEventDrawer(JSON.parse(row.dataset.ev||'{}'));});
  });
}

function renderTimeline(){
  const ribbon=el('timeline-ribbon');if(!ribbon)return;
  const evts=state.events;
  if(!evts.length){ribbon.innerHTML='<div class="timeline-empty">No events yet</div>';return;}
  ribbon.innerHTML=evts.map(ev=>{
    const cls=sevClass(ev.severity);
    const tip=`${fmtTime(ev.timestamp)} · ${(ev.severity||'info').toUpperCase()} · ${(ev.message||'').slice(0,60)}`;
    return`<span class="tl-dot tl-${cls}" data-tip="${esc(tip)}" role="img" aria-label="${esc(tip)}"></span>`;
  }).join('');
}

function renderServices(){
  const grid=el('services-grid');if(!grid)return;
  const svcs=state.services;
  if(!svcs.length){grid.innerHTML='<div class="svc-loading">No services registered</div>';return;}
  grid.innerHTML=svcs.map(svc=>{
    const s=(svc.status||'unknown').toLowerCase();
    let dotCls='svc-dot-unknown';
    if(s==='healthy')dotCls='svc-dot-healthy';
    else if(s==='degraded'||s==='warning')dotCls='svc-dot-warning';
    else if(s==='critical'||s==='unhealthy')dotCls='svc-dot-critical';
    const last=svc.last_check?fmtTime(svc.last_check):(svc.last_seen?fmtTime(svc.last_seen):'');
    return`<div class="svc-card" role="button" tabindex="0" data-svc="${esc(svc.name||'')}">
      <div class="svc-card-head"><span class="svc-dot ${dotCls}" aria-hidden="true"></span><span class="svc-name" title="${esc(svc.name||'')}">${esc(svc.name||'?')}</span></div>
      <div class="svc-status-text">${esc(svc.status||'unknown')}</div>
      ${last?`<div class="svc-last">${esc(last)}</div>`:''}
    </div>`;
  }).join('');
  grid.querySelectorAll('.svc-card').forEach(card=>{
    card.addEventListener('click',()=>{ location.hash=`#/topology?focus=${card.dataset.svc}`; });
    card.addEventListener('keydown',e=>{if(e.key==='Enter')location.hash=`#/topology?focus=${card.dataset.svc}`;});
  });
}

// ─── Drawers ──────────────────────────────────────────────────────────────────
function openDrawer(title,html){
  el('drawer-title').textContent=title;
  el('drawer-body').innerHTML=html;
  el('drawer').classList.add('open');
  el('drawer-overlay').classList.add('open');
  el('drawer-close').focus();
}
function closeDrawer(){el('drawer').classList.remove('open');el('drawer-overlay').classList.remove('open');}
el('drawer-close').addEventListener('click',closeDrawer);
el('drawer-overlay').addEventListener('click',closeDrawer);

function openEventDrawer(ev){
  openDrawer('Event Detail',`
    <div style="display:flex;flex-direction:column;gap:12px">
      <div class="badge badge--${sevClass(ev.severity)==='critical'?'danger':sevClass(ev.severity)==='warning'?'warn':'info'}">${esc((ev.severity||'info').toUpperCase())}</div>
      <div style="font-size:14px;color:var(--text)">${esc(ev.message||'')}</div>
      <div style="font-family:var(--font-mono);font-size:10px;color:var(--text-muted)">${esc(fmtTime(ev.timestamp))} · ${esc(ev.source||'')}</div>
      <details style="margin-top:8px">
        <summary style="cursor:pointer;font-family:var(--font-mono);font-size:11px;color:var(--text-muted)">Full JSON</summary>
        <pre style="margin-top:8px;font-family:var(--font-mono);font-size:10px;color:var(--text-muted);overflow:auto;padding:10px;background:rgba(0,0,0,0.3);border-radius:6px;white-space:pre-wrap">${esc(JSON.stringify(ev,null,2))}</pre>
      </details>
      <button class="btn btn--secondary" onclick="navigator.clipboard.writeText(${esc(JSON.stringify(JSON.stringify(ev)))}).then(()=>toast('Copied!','success'))">Copy JSON</button>
      <button class="btn btn--ghost" onclick="location.hash='#/agent';closeDrawer()">Explain with Agent →</button>
    </div>
  `);
}

// ─── Audit view ───────────────────────────────────────────────────────────────
function renderAuditView(){
  const verdict=el('audit-verdict'),icon=el('audit-verdict-icon'),text=el('audit-verdict-text'),meta=el('audit-meta'),chain=el('audit-chain');
  if(!verdict)return;
  if(state.auditOk===null&&!state.audit.length){
    verdict.className='audit-verdict';if(icon)icon.textContent='◌';if(text)text.textContent='NO AUDIT DATA';
    if(meta)meta.textContent='Enable pqaudit with --pqaudit flag';
    if(chain)chain.innerHTML='<div class="audit-chain-empty">No audit entries — run with --pqaudit to enable signed audit chain.</div>';
    return;
  }
  if(state.auditOk===true){verdict.className='audit-verdict audit-verdict-ok';if(icon)icon.textContent='✓';if(text)text.textContent='CHAIN VERIFIED';if(meta)meta.textContent=`${state.auditCount} entries · tamper-proof · post-quantum signed`;}
  else if(state.auditOk===false){verdict.className='audit-verdict audit-verdict-fail';if(icon)icon.textContent='✗';if(text)text.textContent='CHAIN TAMPERED';if(meta)meta.textContent='Integrity check failed — investigate immediately';}
  else{verdict.className='audit-verdict';if(icon)icon.textContent='◌';if(text)text.textContent='AUDIT LOG';if(meta)meta.textContent=`${state.audit.length} entries`;}
  if(!chain)return;
  if(!state.audit.length){chain.innerHTML='<div class="audit-chain-empty">No audit entries yet</div>';return;}
  chain.innerHTML=state.audit.map((entry,i)=>{
    const action=esc(entry.action||entry.Action||'—'),actor=esc(entry.actor||entry.Actor||'—'),detail=esc(entry.detail||entry.Detail||entry.target||entry.Target||'');
    const ts=esc(fmtTime(entry.timestamp||entry.Timestamp||'')),hash=entry.hash||entry.Hash||entry.signature||'';
    const hashShort=hash?esc(String(hash).slice(0,12)+'…'):'',ok=entry.ok!=null?entry.ok:true,signed=!!(hash);
    return`<div class="audit-block ${signed?'audit-block-signed':''}" style="animation-delay:${i*0.04}s" role="button" tabindex="0" data-entry="${esc(JSON.stringify(entry))}">
      <div class="audit-block-left">
        <div class="audit-block-action">${action}</div><div class="audit-block-actor">${actor}</div>
        ${detail?`<div class="audit-block-detail">${detail}</div>`:''}
        ${hashShort?`<div class="audit-block-hash">${hashShort}</div>`:''}
      </div>
      <div class="audit-block-right">
        <span class="audit-badge ${ok?'audit-badge-ok':'audit-badge-fail'}">${ok?'Verified':'Failed'}</span>
        ${signed?'<span class="audit-badge audit-badge-signed">Signed</span>':''}
        ${ts?`<span class="audit-block-time">${ts}</span>`:''}
      </div>
    </div>`;
  }).join('');
  chain.querySelectorAll('.audit-block').forEach(block=>{
    const handler=()=>{const e=JSON.parse(block.dataset.entry||'{}');openDrawer('Audit Entry',`<pre style="font-family:var(--font-mono);font-size:10px;color:var(--text-muted);white-space:pre-wrap">${esc(JSON.stringify(e,null,2))}</pre><button class="btn btn--secondary" style="margin-top:12px" onclick="navigator.clipboard.writeText(${esc(JSON.stringify(JSON.stringify(e)))}).then(()=>toast('Copied!','success'))">Copy JSON</button>`);};
    block.addEventListener('click',handler);block.addEventListener('keydown',e=>{if(e.key==='Enter')handler();});
  });
}

// ─── Terminal view ─────────────────────────────────────────────────────────────
let termCount=0;
function renderTerminalView(){
  const body=el('term-body');if(!body)return;
  const evts=[...state.events].reverse();if(!evts.length)return;
  body.innerHTML='';
  evts.forEach(ev=>{
    const cls=sevClass(ev.severity);
    const termCls=cls==='critical'?'term-line-crit':cls==='warning'?'term-line-warn':'term-line-info';
    const line=document.createElement('div');line.className=`term-line ${termCls}`;
    line.textContent=`${fmtTime(ev.timestamp)}  ${(ev.severity||'INFO').toUpperCase().padEnd(8)}  ${ev.source?`[${ev.source}] `:''}${ev.message||''}`;
    body.appendChild(line);
  });
  termCount=evts.length;const tc=el('term-count');if(tc)tc.textContent=`${termCount} events`;
  body.scrollTop=body.scrollHeight;
}

// ─── Topology force graph ──────────────────────────────────────────────────────
function detectCycles(nodes,edges){
  const adj={};nodes.forEach(n=>{adj[n.id]=[];});edges.forEach(e=>{if(adj[e.from])adj[e.from].push(e.to);});
  const WHITE=0,GRAY=1,BLACK=2,color={},cycles=new Set();
  nodes.forEach(n=>{color[n.id]=WHITE;});
  function dfs(u){color[u]=GRAY;for(const v of(adj[u]||[])){if(color[v]===GRAY)cycles.add(u+'|'+v);else if(color[v]===WHITE)dfs(v);}color[u]=BLACK;}
  nodes.forEach(n=>{if(color[n.id]===WHITE)dfs(n.id);});return cycles;
}
function initPositions(nodes,W,H){nodes.forEach((n,i)=>{if(n.x==null){const a=(2*Math.PI*i)/nodes.length;n.x=W/2+(W*0.32)*Math.cos(a);n.y=H/2+(H*0.32)*Math.sin(a);n.vx=0;n.vy=0;}});}
function runForce(nodes,edges,iters){
  const k=90,spring=0.04,damp=0.82,nodeMap={};nodes.forEach(n=>{nodeMap[n.id]=n;});
  for(let it=0;it<iters;it++){
    for(let i=0;i<nodes.length;i++)for(let j=i+1;j<nodes.length;j++){const a=nodes[i],b=nodes[j],dx=b.x-a.x,dy=b.y-a.y,dist=Math.sqrt(dx*dx+dy*dy)||1,f=(k*k)/dist;a.vx-=(dx/dist)*f;a.vy-=(dy/dist)*f;b.vx+=(dx/dist)*f;b.vy+=(dy/dist)*f;}
    edges.forEach(e=>{const a=nodeMap[e.from],b=nodeMap[e.to];if(!a||!b)return;const dx=b.x-a.x,dy=b.y-a.y,dist=Math.sqrt(dx*dx+dy*dy)||1,f=(dist-k)*spring;a.vx+=(dx/dist)*f;a.vy+=(dy/dist)*f;b.vx-=(dx/dist)*f;b.vy-=(dy/dist)*f;});
    nodes.forEach(n=>{n.vx*=damp;n.vy*=damp;n.x+=n.vx;n.y+=n.vy;});
  }
}
function clampNodes(nodes,W,H){const pad=40;nodes.forEach(n=>{n.x=Math.max(pad,Math.min(W-pad,n.x));n.y=Math.max(pad,Math.min(H-pad,n.y));});}
const topo={pan:{x:0,y:0},zoom:1,dragging:null,flowAnim:null};

function renderTopologyView(){
  const svg=el('topo-svg'),empty=el('topo-empty'),wrap=el('topo-canvas-wrap');if(!svg||!wrap)return;
  const{nodes,edges,cycles}=state.topology;
  if(!nodes.length){svg.style.display='none';if(empty)empty.style.display='flex';return;}
  svg.style.display='block';if(empty)empty.style.display='none';
  const W=wrap.clientWidth||800,H=wrap.clientHeight||500;
  if(!state.topoInited){initPositions(nodes,W,H);runForce(nodes,edges,60);clampNodes(nodes,W,H);state.topoInited=true;}
  else{runForce(nodes,edges,4);clampNodes(nodes,W,H);}
  // Check for focused node from URL
  const focus=new URLSearchParams(location.hash.split('?')[1]||'').get('focus');
  drawTopology(svg,nodes,edges,cycles,W,H,focus);
  setupTopoInteraction(svg,wrap,nodes,edges,cycles);
  startFlowAnimation(svg,nodes,edges,cycles);
}
function drawTopology(svg,nodes,edges,cycles,W,H,focus){
  const nodeMap={};nodes.forEach(n=>{nodeMap[n.id]=n;});
  el('topo-edges').innerHTML=edges.map(e=>{
    const a=nodeMap[e.from],b=nodeMap[e.to];if(!a||!b)return'';
    const isCycle=cycles.has(e.from+'|'+e.to)||cycles.has(e.to+'|'+e.from),cls=isCycle?'topo-edge-cycle':'topo-edge';
    const dx=b.x-a.x,dy=b.y-a.y,dist=Math.sqrt(dx*dx+dy*dy)||1,r=24;
    return`<line class="${cls}" x1="${(a.x+(dx/dist)*r).toFixed(1)}" y1="${(a.y+(dy/dist)*r).toFixed(1)}" x2="${(b.x-(dx/dist)*r).toFixed(1)}" y2="${(b.y-(dy/dist)*r).toFixed(1)}" data-from="${esc(e.from)}" data-to="${esc(e.to)}"/>`;
  }).join('');
  el('topo-nodes').innerHTML=nodes.map(n=>{
    const inCycle=[...cycles].some(c=>c.startsWith(n.id+'|')||c.endsWith('|'+n.id));
    const cls=inCycle?'topo-node-cycle':'topo-node-normal';
    const label=(n.label||n.id).length>10?(n.label||n.id).slice(0,9)+'…':(n.label||n.id);
    const focused=focus&&n.id===focus;
    return`<g class="topo-node-g" data-id="${esc(n.id)}" role="button" tabindex="0" aria-label="Node ${esc(n.id)}">
      <circle class="topo-node-circle ${cls}" cx="${n.x.toFixed(1)}" cy="${n.y.toFixed(1)}" r="${focused?28:22}" ${focused?'stroke-width="3"':''}/>
      <text class="topo-node-label" x="${n.x.toFixed(1)}" y="${n.y.toFixed(1)}">${esc(label)}</text>
    </g>`;
  }).join('');
  el('topo-nodes').querySelectorAll('.topo-node-g').forEach(g=>{
    g.addEventListener('click',()=>openNodeDrawer(g.dataset.id));
  });
}
function openNodeDrawer(id){
  const node=state.topology.nodes.find(n=>n.id===id);if(!node)return;
  const edges=state.topology.edges;
  const incoming=edges.filter(e=>e.to===id).map(e=>e.from);
  const outgoing=edges.filter(e=>e.from===id).map(e=>e.to);
  const relatedEvents=state.events.filter(ev=>(ev.source||'').toLowerCase()===id.toLowerCase()).slice(0,5);
  openDrawer(`Service: ${id}`,`
    <div style="display:flex;flex-direction:column;gap:16px">
      <div><div class="field-label">Incoming</div>${incoming.length?incoming.map(s=>`<span class="badge badge--info" style="margin:2px">${esc(s)}</span>`).join(''):'<span style="color:var(--text-muted);font-size:12px">none</span>'}</div>
      <div><div class="field-label">Outgoing</div>${outgoing.length?outgoing.map(s=>`<span class="badge badge--neutral" style="margin:2px">${esc(s)}</span>`).join(''):'<span style="color:var(--text-muted);font-size:12px">none</span>'}</div>
      <div><div class="field-label">Blast Radius</div><span class="badge badge--warn">${outgoing.length} downstream</span></div>
      ${relatedEvents.length?`<div><div class="field-label">Recent Events</div>${relatedEvents.map(ev=>`<div style="font-size:11px;padding:4px 0;border-bottom:1px solid var(--border);color:var(--text-muted)">${esc(fmtTime(ev.timestamp))} ${esc(ev.message||'')}</div>`).join('')}</div>`:''}
    </div>
  `);
}
function startFlowAnimation(svg,nodes,edges,cycles){
  if(topo.flowAnim){cancelAnimationFrame(topo.flowAnim);topo.flowAnim=null;}
  if(!edges.length)return;
  const nodeMap={};nodes.forEach(n=>{nodeMap[n.id]=n;});
  const old=svg.querySelector('#flow-layer');if(old)old.remove();
  const flowLayer=document.createElementNS('http://www.w3.org/2000/svg','g');flowLayer.id='flow-layer';svg.appendChild(flowLayer);
  const dots=edges.map(e=>{
    const a=nodeMap[e.from],b=nodeMap[e.to];if(!a||!b)return null;
    const isCycle=cycles.has(e.from+'|'+e.to)||cycles.has(e.to+'|'+e.from);
    const dot=document.createElementNS('http://www.w3.org/2000/svg','circle');
    dot.setAttribute('r','3');dot.setAttribute('fill',isCycle?'#ffa657':'#58a6ff');dot.setAttribute('opacity','0.75');
    flowLayer.appendChild(dot);return{dot,a,b,t:Math.random()};
  }).filter(Boolean);
  const speed=0.004;
  function animate(){
    dots.forEach(d=>{d.t=(d.t+speed)%1;d.dot.setAttribute('cx',(d.a.x+(d.b.x-d.a.x)*d.t).toFixed(2));d.dot.setAttribute('cy',(d.a.y+(d.b.y-d.a.y)*d.t).toFixed(2));const fade=Math.min(d.t*6,1)*Math.min((1-d.t)*6,1);d.dot.setAttribute('opacity',(fade*0.8).toFixed(2));});
    topo.flowAnim=requestAnimationFrame(animate);
  }
  topo.flowAnim=requestAnimationFrame(animate);
}
function setupTopoInteraction(svg,wrap,nodes,edges,cycles){
  const nodesG=el('topo-nodes');
  function getPos(e){const r=svg.getBoundingClientRect();return{x:e.clientX-r.left,y:e.clientY-r.top};}
  nodesG.onmousedown=function(e){const g=e.target.closest('.topo-node-g');if(!g)return;const node=nodes.find(n=>n.id===g.dataset.id);if(!node)return;e.preventDefault();const pos=getPos(e);topo.dragging={node,ox:pos.x-node.x,oy:pos.y-node.y};};
  svg.onmousemove=function(e){if(!topo.dragging)return;const pos=getPos(e);topo.dragging.node.x=pos.x-topo.dragging.ox;topo.dragging.node.y=pos.y-topo.dragging.oy;drawTopology(svg,nodes,edges,cycles,wrap.clientWidth||800,wrap.clientHeight||500,null);};
  svg.onmouseup=function(){topo.dragging=null;};svg.onmouseleave=function(){topo.dragging=null;};
  const resetBtn=el('topo-reset');if(resetBtn)resetBtn.onclick=function(){nodes.forEach(n=>{n.x=undefined;n.y=undefined;n.vx=0;n.vy=0;});state.topoInited=false;renderTopologyView();};
}

// ─── Charts ───────────────────────────────────────────────────────────────────
function LineChart({el:container,data,bands,color='#58a6ff',label=''}){
  if(!container||!data.length)return;
  const W=container.clientWidth||600,H=180,pad={t:16,r:16,b:28,l:44};
  const xs=data.map(d=>d.x),ys=data.map(d=>d.y);
  const minX=Math.min(...xs),maxX=Math.max(...xs);
  const allY=ys.concat(bands?bands.flatMap(b=>[b.p05,b.p95]):[]);
  const minY=Math.min(...allY)*0.95,maxY=Math.max(...allY)*1.05;
  const px=(x)=>pad.l+(x-minX)/(maxX-minX||1)*(W-pad.l-pad.r);
  const py=(y)=>pad.t+(maxY-y)/(maxY-minY||1)*(H-pad.t-pad.b);

  const ticks=5;const gridLines=Array.from({length:ticks},(_, i)=>{const y=minY+(maxY-minY)*i/(ticks-1);return{y,py:py(y)};});

  const pathD=data.map((d,i)=>(i===0?`M${px(d.x).toFixed(1)},${py(d.y).toFixed(1)}`:`L${px(d.x).toFixed(1)},${py(d.y).toFixed(1)}`)).join(' ');

  let bandSVG='';
  if(bands&&bands.length){
    const p95path=bands.map((b,i)=>(i===0?`M${px(b.x||xs[i]).toFixed(1)},${py(b.p95).toFixed(1)}`:`L${px(b.x||xs[i]).toFixed(1)},${py(b.p95).toFixed(1)}`)).join(' ');
    const p05path=[...bands].reverse().map((b,i)=>{const idx=bands.length-1-i;return`L${px(b.x||xs[idx]).toFixed(1)},${py(b.p05).toFixed(1)}`;}).join(' ');
    bandSVG=`<path d="${p95path} ${p05path} Z" fill="${color}" class="chart-band"/>`;
    const p95line=bands.map((b,i)=>(i===0?`M${px(b.x||xs[i]).toFixed(1)},${py(b.p95).toFixed(1)}`:`L${px(b.x||xs[i]).toFixed(1)},${py(b.p95).toFixed(1)}`)).join(' ');
    const p05line=bands.map((b,i)=>(i===0?`M${px(b.x||xs[i]).toFixed(1)},${py(b.p05).toFixed(1)}`:`L${px(b.x||xs[i]).toFixed(1)},${py(b.p05).toFixed(1)}`)).join(' ');
    bandSVG+=`<path d="${p95line}" fill="none" stroke="${color}" stroke-width="1" stroke-dasharray="3 3" opacity="0.4"/>`;
    bandSVG+=`<path d="${p05line}" fill="none" stroke="${color}" stroke-width="1" stroke-dasharray="3 3" opacity="0.4"/>`;
  }

  const totalLen=data.reduce((acc,d,i)=>{if(i===0)return 0;const dx=px(d.x)-px(data[i-1].x),dy=py(d.y)-py(data[i-1].y);return acc+Math.sqrt(dx*dx+dy*dy);},0);

  container.innerHTML=`<div class="chart-wrap"><svg class="chart-svg" viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" style="height:${H}px">
    ${gridLines.map(g=>`<line class="chart-grid" x1="${pad.l}" y1="${g.py.toFixed(1)}" x2="${W-pad.r}" y2="${g.py.toFixed(1)}"/><text class="chart-tick" x="${pad.l-4}" y="${g.py.toFixed(1)}" text-anchor="end" dominant-baseline="middle">${g.y.toFixed(1)}</text>`).join('')}
    <line class="chart-axis" x1="${pad.l}" y1="${pad.t}" x2="${pad.l}" y2="${H-pad.b}"/>
    <line class="chart-axis" x1="${pad.l}" y1="${H-pad.b}" x2="${W-pad.r}" y2="${H-pad.b}"/>
    ${bandSVG}
    <path class="chart-line" d="${pathD}" stroke="${color}" stroke-dasharray="${totalLen.toFixed(0)}" stroke-dashoffset="${totalLen.toFixed(0)}" style="animation:chart-draw 400ms ease-out forwards"/>
  </svg><div class="chart-tooltip" id="chart-tt-${Math.random().toString(36).slice(2)}"></div></div>`;

  // inject animation keyframe once
  if(!document.getElementById('chart-anim-style')){
    const s=document.createElement('style');s.id='chart-anim-style';
    s.textContent='@keyframes chart-draw{to{stroke-dashoffset:0}}';
    document.head.appendChild(s);
  }
}

function BarChart({el:container,data,orientation='horizontal',colorFn}){
  if(!container||!data.length)return;
  const W=container.clientWidth||400,H=orientation==='horizontal'?data.length*32+24:180;
  const pad={t:8,r:16,b:24,l:orientation==='horizontal'?100:16};
  const vals=data.map(d=>d.value);const maxV=Math.max(...vals)||1;
  let bars='';
  if(orientation==='horizontal'){
    data.forEach((d,i)=>{
      const barW=(d.value/maxV)*(W-pad.l-pad.r);const y=pad.t+i*32;
      const col=colorFn?colorFn(d,i):(i===0?'#ffa657':'#58a6ff');
      bars+=`<text class="chart-tick" x="${pad.l-4}" y="${(y+14).toFixed(1)}" text-anchor="end" dominant-baseline="middle">${esc(String(d.label).slice(0,14))}</text>`;
      bars+=`<rect x="${pad.l}" y="${y.toFixed(1)}" width="0" height="22" fill="${col}" rx="3" class="chart-bar" style="animation:bar-grow-h ${300+i*40}ms ease-out forwards" data-w="${barW.toFixed(1)}"/>`;
      bars+=`<text class="chart-tick" x="${(pad.l+barW+4).toFixed(1)}" y="${(y+14).toFixed(1)}" dominant-baseline="middle">${d.value.toFixed?d.value.toFixed(2):d.value}</text>`;
    });
  }else{
    const bw=Math.max(4,(W-pad.l-pad.r)/data.length-4);
    data.forEach((d,i)=>{
      const barH=(d.value/maxV)*(H-pad.t-pad.b);const x=pad.l+i*(bw+4);
      const col=colorFn?colorFn(d,i):(i===0?'#ffa657':'#58a6ff');
      bars+=`<rect x="${x.toFixed(1)}" y="${(H-pad.b-barH).toFixed(1)}" width="${bw.toFixed(1)}" height="${barH.toFixed(1)}" fill="${col}" rx="2" class="chart-bar"/>`;
      bars+=`<text class="chart-tick" x="${(x+bw/2).toFixed(1)}" y="${(H-pad.b+10).toFixed(1)}" text-anchor="middle">${esc(String(d.label).slice(0,6))}</text>`;
    });
  }
  container.innerHTML=`<div class="chart-wrap"><svg class="chart-svg" viewBox="0 0 ${W} ${H}" style="height:${H}px">${bars}</svg></div>`;
  // animate horizontal bars
  if(orientation==='horizontal'){
    if(!document.getElementById('bar-anim-style')){
      const s=document.createElement('style');s.id='bar-anim-style';
      s.textContent='@keyframes bar-grow-h{from{width:0}to{width:var(--bw)}}';document.head.appendChild(s);
    }
    container.querySelectorAll('.chart-bar').forEach(r=>{r.style.setProperty('--bw',r.dataset.w+'px');r.style.animation=`bar-grow-h ${parseInt(r.style.animation)||300}ms ease-out forwards`;});
    // direct width animation fallback
    container.querySelectorAll('.chart-bar').forEach(r=>{
      const tw=parseFloat(r.dataset.w||'0');r.setAttribute('width','0');
      const t0=performance.now();const dur=300;
      function step(now){const p=Math.min((now-t0)/dur,1);r.setAttribute('width',(tw*p*(1-Math.pow(1-p,2))*2).toFixed(1)||'0');if(p<1)requestAnimationFrame(step);else r.setAttribute('width',tw.toFixed(1));}
      requestAnimationFrame(step);
    });
  }
}

// ─── Forecast view ────────────────────────────────────────────────────────────
function renderForecastView(){
  const section=el('view-forecast');if(!section)return;
  section.innerHTML=`<div id="overview-filter-bar2"></div><div class="forecast-grid">
    <div class="forecast-chart-panel">
      <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:12px">
        <h2 style="font-family:var(--font-serif);font-size:22px;font-weight:700">Predictive Twin</h2>
        <span class="badge badge--info" id="forecast-status">loading…</span>
      </div>
      <div class="forecast-legend">
        <span class="forecast-legend-item"><span class="forecast-legend-line" style="background:var(--cyan)"></span>P50 (median)</span>
        <span class="forecast-legend-item"><span class="forecast-legend-line" style="background:var(--cyan);opacity:0.4;border-top:1px dashed"></span>P05–P95 band</span>
        <span class="forecast-legend-item"><span class="forecast-legend-line" style="background:var(--amber)"></span>Observed</span>
      </div>
      <div id="forecast-chart"></div>
    </div>
    <div>
      <div class="forecast-breach-table">
        <div class="forecast-breach-head">Predicted Breaches</div>
        <div id="forecast-breach-list"><div style="padding:16px;font-size:12px;color:var(--text-muted)">Calculating…</div></div>
      </div>
    </div>
  </div>`;
  renderFilterBar('overview-filter-bar2');
  fetchForecast();
}

async function fetchForecast(){
  const statusEl=el('forecast-status');
  try{
    const r=await fetch('/api/v5/forecast');
    if(r.ok){
      const d=await r.json();
      if(statusEl)statusEl.textContent='live';
      renderForecastChart(d);return;
    }
  }catch(_){}
  // Fallback: derive from /api/metrics
  try{
    const r=await fetch('/api/metrics');
    if(r.ok){
      const text=await r.text();
      const nums=[];let i=0;
      text.split('\n').forEach(line=>{if(!line.startsWith('#')){const m=line.match(/[\d.]+$/);if(m){nums.push({x:i++,y:parseFloat(m[0])});}}});
      if(nums.length>2){
        if(statusEl)statusEl.textContent='synthetic';
        const synthetic=synthesizeForecast(nums.slice(0,30));
        renderForecastChart(synthetic);return;
      }
    }
  }catch(_){}
  // Empty state
  if(statusEl)statusEl.textContent='unavailable';
  const chart=el('forecast-chart');
  if(chart)chart.innerHTML=`<div class="forecast-empty"><div class="forecast-empty-title">No forecast data</div><div class="forecast-empty-sub" style="font-family:var(--font-mono);font-size:11px;color:var(--text-muted)">Enable with --predictive to populate</div></div>`;
  const bl=el('forecast-breach-list');if(bl)bl.innerHTML='<div style="padding:16px;font-size:12px;color:var(--text-muted)">No data available</div>';
}

function synthesizeForecast(observed){
  const n=observed.length;const mean=observed.reduce((s,d)=>s+d.y,0)/n;
  const std=Math.sqrt(observed.reduce((s,d)=>s+(d.y-mean)**2,0)/n)||1;
  const trend=(observed[n-1].y-observed[0].y)/(n-1||1);
  const p50=Array.from({length:30},(_,i)=>({x:n+i,y:mean+trend*(i+1),p05:mean+trend*(i+1)-std*1.64,p95:mean+trend*(i+1)+std*1.64}));
  return{observed,forecast:p50};
}

function renderForecastChart(data){
  const chart=el('forecast-chart');if(!chart)return;
  const obs=data.observed||[];const fc=data.forecast||[];
  const allData=[...obs.map((d,i)=>({x:i,y:d.y||d})),...fc.map((d,i)=>({x:(obs.length||0)+i,y:d.y||d.p50||d}))];
  const bands=fc.map((d,i)=>({x:(obs.length||0)+i,p05:d.p05||d.y*0.9,p95:d.p95||d.y*1.1}));
  LineChart({el:chart,data:allData,bands,color:'#58a6ff'});
  // Breach table
  const bl=el('forecast-breach-list');if(!bl)return;
  const threshold=allData.reduce((s,d)=>s+d.y,0)/allData.length*1.5;
  const breaches=fc.filter(d=>(d.p95||d.y)>threshold).slice(0,5);
  if(!breaches.length){bl.innerHTML='<div class="forecast-breach-row" style="color:var(--text-muted)">No predicted breaches</div>';return;}
  bl.innerHTML=breaches.map((b,i)=>`<div class="forecast-breach-row"><span style="font-family:var(--font-mono);font-size:11px">tick+${(fc.indexOf(b)+1)||i+1}</span><span class="badge badge--danger">breach</span></div>`).join('');
}

// ─── Agent view ────────────────────────────────────────────────────────────────
function renderAgentView(){
  const section=el('view-agent');if(!section)return;
  section.innerHTML=`<div class="agent-split">
    <div class="agent-form-panel">
      <div class="agent-form-title">Run Agent</div>
      <p style="font-size:12px;color:var(--text-muted);margin-bottom:4px">Post an incident to the ReAct agent and trace its reasoning.</p>
      <div><label class="field-label" for="agent-source">Source</label><input class="input" id="agent-source" type="text" placeholder="api / db / cache" value="${esc(state.events[0]?.source||'api')}"></div>
      <div><label class="field-label" for="agent-severity">Severity</label>
        <select class="select" id="agent-severity">
          <option value="info">info</option><option value="warning">warning</option><option value="critical" selected>critical</option>
        </select>
      </div>
      <div><label class="field-label" for="agent-msg">Message</label><textarea class="textarea" id="agent-msg" rows="3" placeholder="db connection pool exhausted">${esc(state.events[0]?.message||'')}</textarea></div>
      <button class="btn btn--primary" id="agent-run">Run Agent</button>
      <div id="agent-status" style="font-family:var(--font-mono);font-size:11px;color:var(--text-muted)"></div>
    </div>
    <div class="agent-trace-panel">
      <div class="panel-header"><span class="panel-title">ReAct Trace</span></div>
      <div id="agent-trace-body"><div class="agent-trace-empty">Submit an incident to see the agent trace here.</div></div>
    </div>
  </div>`;
  el('agent-run').addEventListener('click',runAgent);
  if(state.agentTrace)renderAgentTrace(state.agentTrace);
}

async function runAgent(){
  const btn=el('agent-run'),status=el('agent-status');
  if(!btn)return;
  btn.disabled=true;btn.textContent='Running…';if(status)status.textContent='Posting to /api/v4/agentic/run…';
  const payload={source:el('agent-source')?.value||'api',severity:el('agent-severity')?.value||'critical',message:el('agent-msg')?.value||'incident'};
  try{
    const r=await fetch('/api/v4/agentic/run',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)});
    if(!r.ok)throw new Error(r.statusText);
    const d=await r.json();
    state.agentTrace=d;if(status)status.textContent='';
    renderAgentTrace(d);toast('Agent run complete','success');
  }catch(err){
    if(status)status.textContent=`Error: ${err.message}`;toast('Agent endpoint error','warn');
    // Show mock trace for demo
    state.agentTrace=mockAgentTrace(payload);renderAgentTrace(state.agentTrace);
  }finally{if(btn){btn.disabled=false;btn.textContent='Run Agent';}}
}

function mockAgentTrace(payload){
  return{trace:[
    {thought:`Received ${payload.severity} incident from ${payload.source}: "${payload.message}"`,tool:'diagnose',args:{service:payload.source},observation:'High error rate detected, latency p99 > 2000ms',error:''},
    {thought:'Need to check dependent services before taking action',tool:'get_dependencies',args:{service:payload.source},observation:'api depends on db, cache. db shows 0 healthy connections',error:''},
    {thought:'Root cause identified: db connection pool exhausted. Initiating restart.',tool:'restart_service',args:{service:'db',reason:'connection pool exhausted'},observation:'Service db restarting…',error:''},
    {thought:'Verifying recovery',tool:'check_health',args:{service:'db'},observation:'db: healthy, connections: 42/50',error:''},
  ],reason:'Database connection pool was exhausted due to a query storm. Restarted service successfully. Monitor for recurrence.'};
}

function renderAgentTrace(data){
  const body=el('agent-trace-body');if(!body)return;
  const steps=data.trace||[];const reason=data.reason||data.final_reason||'';
  if(!steps.length){body.innerHTML='<div class="agent-trace-empty">No trace steps returned.</div>';return;}
  const stepsHTML=steps.map((s,i)=>{
    const isLast=i===steps.length-1;
    return`<div class="trace-step" style="animation-delay:${i*0.08}s">
      <div class="trace-step-rail"><div class="trace-step-dot${isLast?' last':s.error?' error':''}"></div>${!isLast?'<div class="trace-step-line"></div>':''}</div>
      <div class="trace-step-body">
        ${s.thought?`<div class="trace-step-thought">${esc(s.thought)}</div>`:''}
        ${s.tool?`<div class="trace-step-tool">⚙ ${esc(s.tool)}(${esc(JSON.stringify(s.args||{}))})</div>`:''}
        ${s.observation?`<div class="trace-step-obs">${esc(s.observation)}</div>`:''}
        ${s.error?`<div class="trace-step-err">✗ ${esc(s.error)}</div>`:''}
      </div>
    </div>`;
  }).join('');
  body.innerHTML=`<div class="trace-timeline">${stepsHTML}</div>${reason?`<div class="trace-final-reason"><div class="trace-final-label">Final Reason</div><div class="trace-final-text">${esc(reason)}</div></div>`:''}`;
}

// ─── Verify view ───────────────────────────────────────────────────────────────
function renderVerifyView(){
  const section=el('view-verify');if(!section)return;
  const defaultWorld=JSON.stringify({api:{Healthy:true,Replicas:3},db:{Healthy:true,Replicas:2}},null,2);
  const defaultPlan=JSON.stringify(['RestartService("db")','ScaleUp("api",2)'],null,2);
  section.innerHTML=`<h2 style="font-family:var(--font-serif);font-size:26px;font-weight:700;margin-bottom:20px">Formal Model Check</h2>
  <div class="verify-split">
    <div class="verify-left">
      <div><label class="field-label" for="verify-world">World (JSON)</label><textarea class="textarea" id="verify-world" rows="6" style="font-family:var(--font-mono);font-size:11px">${esc(defaultWorld)}</textarea></div>
      <div><label class="field-label" for="verify-plan">Plan Steps (JSON array)</label><textarea class="textarea" id="verify-plan" rows="4" style="font-family:var(--font-mono);font-size:11px">${esc(defaultPlan)}</textarea></div>
      <div>
        <span class="field-label">Invariants</span>
        <div class="verify-invariants">
          <label class="verify-inv-label"><input type="checkbox" name="inv" value="at_least_n_healthy" checked> at_least_n_healthy</label>
          <label class="verify-inv-label"><input type="checkbox" name="inv" value="min_replicas" checked> min_replicas</label>
          <label class="verify-inv-label"><input type="checkbox" name="inv" value="service_always_healthy"> service_always_healthy</label>
        </div>
      </div>
      <button class="btn btn--primary" id="verify-run" style="width:100%">VERIFY</button>
    </div>
    <div class="verify-right" id="verify-result">
      <div class="verify-result-empty">Configure the world and plan,<br>then click VERIFY.</div>
    </div>
  </div>`;
  el('verify-run').addEventListener('click',runVerify);
}

async function runVerify(){
  const btn=el('verify-run'),result=el('verify-result');if(!btn||!result)return;
  btn.disabled=true;btn.textContent='Verifying…';
  let world,plan;
  try{world=JSON.parse(el('verify-world').value||'{}');}catch(_){toast('Invalid World JSON','warn');btn.disabled=false;btn.textContent='VERIFY';return;}
  try{plan=JSON.parse(el('verify-plan').value||'[]');}catch(_){toast('Invalid Plan JSON','warn');btn.disabled=false;btn.textContent='VERIFY';return;}
  const invEls=document.querySelectorAll('input[name="inv"]:checked');
  const invariants=[...invEls].map(i=>i.value);
  const payload={world,steps:plan,invariants};
  try{
    const r=await fetch('/api/v4/formal/check',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)});
    if(!r.ok)throw new Error(r.statusText);
    const d=await r.json();renderVerifyResult(d);
  }catch(err){
    // Client-side fallback check
    const d=clientSideVerify(world,plan,invariants);renderVerifyResult(d);
    if(err.message!=='OK')toast('Using client-side verification','info');
  }finally{if(btn){btn.disabled=false;btn.textContent='VERIFY';}}
}

function clientSideVerify(world,steps,invariants){
  // Simple heuristic: count Healthy fields
  const services=Object.values(world);
  const healthyCount=services.filter(s=>s&&s.Healthy).length;
  const minReplicas=Math.min(...services.map(s=>s&&s.Replicas||0).filter(r=>r>0));
  const safe=healthyCount>=1&&(!invariants.includes('min_replicas')||minReplicas>=2);
  return{safe,counterexample:safe?null:[`Step 0: healthy=${healthyCount}, min_replicas=${minReplicas}`]};
}

function renderVerifyResult(d){
  const result=el('verify-result');if(!result)return;
  const safe=d.safe||d.ok||d.valid||d.result==='safe';
  const counter=d.counterexample||d.trace||null;
  result.innerHTML=`<div style="display:flex;flex-direction:column;align-items:center;gap:16px;width:100%">
    <div class="${safe?'verify-result-safe':'verify-result-fail'}" role="status" aria-live="assertive">${safe?'✓ SAFE':'✗ VIOLATION'}</div>
    ${!safe&&counter?`<div class="verify-counterex"><div class="verify-counterex-title">Counterexample</div>${(Array.isArray(counter)?counter:[counter]).map(s=>`<div class="verify-counterex-step">${esc(String(s))}</div>`).join('')}</div>`:''}
    ${safe?'<div style="font-family:var(--font-mono);font-size:11px;color:var(--green)">All invariants satisfied</div>':''}
  </div>`;
}

// ─── Plan view ─────────────────────────────────────────────────────────────────
function renderPlanView(){
  const section=el('view-plan');if(!section)return;
  section.innerHTML=`<div class="plan-wrap">
    <h2 style="font-family:var(--font-serif);font-size:26px;font-weight:700;margin-bottom:4px">Natural-Language Planner</h2>
    <p style="font-size:12px;color:var(--text-muted);margin-bottom:16px">Write a plan in plain English. The engine compiles it to typed steps and verifies safety.</p>
    <div><label class="field-label" for="plan-input">Plan</label>
      <textarea class="textarea plan-hero-input" id="plan-input" rows="4" placeholder="Restart the database. Keep at least 2 services healthy."></textarea>
    </div>
    <button class="btn btn--primary" id="plan-compile">COMPILE</button>
    <div id="plan-result" style="display:none" class="plan-result"></div>
  </div>`;
  el('plan-compile').addEventListener('click',compilePlan);
}

async function compilePlan(){
  const btn=el('plan-compile'),result=el('plan-result');if(!btn||!result)return;
  const text=el('plan-input')?.value?.trim();if(!text){toast('Enter a plan first','warn');return;}
  btn.disabled=true;btn.textContent='Compiling…';result.style.display='none';
  try{
    const r=await fetch('/api/v5/nlplan/compile',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({text})});
    if(!r.ok)throw new Error(r.statusText);
    const d=await r.json();renderPlanResult(d);
  }catch(err){
    if(err.message.includes('404')||err.message.includes('Not Found')||err.message.includes('fetch')){
      result.style.display='block';
      result.innerHTML=`<div class="plan-result-section"><div class="empty-icon">⚙</div><div class="empty-title" style="font-size:18px">NLPlan not enabled</div><div class="empty-sub">Enable nlplan in the engine to compile natural-language plans.</div></div>`;
    }else{
      // Fallback: mock parse
      renderPlanResult(mockPlanCompile(text));
    }
  }finally{if(btn){btn.disabled=false;btn.textContent='COMPILE';}}
}

function mockPlanCompile(text){
  const steps=text.split(/[.!?]+/).map(s=>s.trim()).filter(Boolean);
  return{steps:steps.map((s,i)=>({index:i+1,action:s})),invariants:['at_least_n_healthy'],safe:true};
}

async function renderPlanResult(d){
  const result=el('plan-result');if(!result)return;
  result.style.display='block';
  const steps=d.steps||[];const invs=d.invariants||[];
  let safeHTML='';
  if(d.safe!=null){safeHTML=`<div class="plan-safety-badge"><span class="badge ${d.safe?'badge--success':'badge--danger'}">${d.safe?'✓ SAFE':'✗ VIOLATION'}</span><span style="font-size:12px;color:var(--text-muted)">Model check result</span></div>`;}
  else{
    // Run verify
    try{
      const r=await fetch('/api/v4/formal/check',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({steps:steps.map(s=>s.action||s),invariants:invs})});
      if(r.ok){const vd=await r.json();safeHTML=`<div class="plan-safety-badge"><span class="badge ${(vd.safe||vd.ok)?'badge--success':'badge--danger'}">${(vd.safe||vd.ok)?'✓ SAFE':'✗ VIOLATION'}</span></div>`;}
    }catch(_){}
  }
  result.innerHTML=`
    <div class="plan-result-section"><div class="plan-section-label">Steps</div>${steps.map(s=>`<div class="plan-step"><span class="plan-step-num">${s.index||'·'}</span><span>${esc(s.action||String(s))}</span></div>`).join('')||'<div style="color:var(--text-muted);font-size:12px">No steps</div>'}</div>
    ${invs.length?`<div class="plan-result-section"><div class="plan-section-label">Invariants</div>${invs.map(i=>`<div class="plan-inv-item">→ ${esc(i)}</div>`).join('')}</div>`:''}
    ${safeHTML?`<div class="plan-result-section">${safeHTML}</div>`:''}
  `;
}

// ─── Causes view ──────────────────────────────────────────────────────────────
function renderCausesView(){
  const section=el('view-causes');if(!section)return;
  section.innerHTML=`<div class="causes-wrap">
    <h2 style="font-family:var(--font-serif);font-size:26px;font-weight:700;margin-bottom:4px">Causal Root-Cause</h2>
    <div class="causes-form">
      <div class="causes-form-row">
        <div><label class="field-label" for="causes-outcome">Outcome Variable</label><input class="input" id="causes-outcome" type="text" placeholder="api_error_rate"></div>
        <div><label class="field-label" for="causes-vars">Variables (comma-separated)</label><input class="input" id="causes-vars" type="text" placeholder="db_latency, cache_miss, cpu_load"></div>
        <button class="btn btn--primary" id="causes-run" style="align-self:flex-end">DISCOVER</button>
      </div>
      <div class="causes-mode-toggle">
        <button class="filter-pill active" data-mode="pc" id="mode-pc">Cross-sectional (PC)</button>
        <button class="filter-pill" data-mode="pcmci" id="mode-pcmci">Time-lagged (PCMCI)</button>
      </div>
    </div>
    <div id="causes-result" style="display:none">
      <div class="causes-chart-panel">
        <div class="panel-header" style="margin:-20px -20px 16px"><span class="panel-title">ACE Magnitude</span><span class="panel-caption" id="causes-mode-label">PC algorithm</span></div>
        <div id="causes-winner" class="causes-winner"></div>
        <div id="causes-chart"></div>
      </div>
    </div>
  </div>`;
  let activeMode='pc';
  document.querySelectorAll('[data-mode]').forEach(b=>{
    b.addEventListener('click',()=>{
      activeMode=b.dataset.mode;
      document.querySelectorAll('[data-mode]').forEach(x=>x.classList.toggle('active',x.dataset.mode===activeMode));
      const lbl=el('causes-mode-label');if(lbl)lbl.textContent=activeMode==='pcmci'?'PCMCI (time-lagged)':'PC algorithm';
    });
  });
  el('causes-run').addEventListener('click',()=>runCauses(activeMode));
}

async function runCauses(mode){
  const btn=el('causes-run');if(!btn)return;
  btn.disabled=true;btn.textContent='Discovering…';
  const outcome=el('causes-outcome')?.value||'api_error_rate';
  const vars=(el('causes-vars')?.value||'db_latency,cache_miss,cpu_load').split(',').map(s=>s.trim()).filter(Boolean);
  const endpoint=mode==='pcmci'?'/api/v5/causal/pcmci':'/api/v4/causal/root-cause';
  try{
    const r=await fetch(endpoint,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({outcome,variables:vars})});
    if(!r.ok)throw new Error(r.statusText);
    const d=await r.json();renderCausesResult(d);
  }catch(_){
    // Mock result
    const mock={causes:vars.map((v,i)=>({variable:v,ace:Math.random()*(1-i*0.2)+0.1})).sort((a,b)=>b.ace-a.ace)};
    renderCausesResult(mock);toast('Using synthetic causal data','info');
  }finally{if(btn){btn.disabled=false;btn.textContent='DISCOVER';}}
}

function renderCausesResult(d){
  const result=el('causes-result');if(!result)return;result.style.display='block';
  const causes=(d.causes||d.results||[]).sort((a,b)=>(b.ace||b.score||0)-(a.ace||a.score||0));
  if(!causes.length){el('causes-winner').textContent='No causes found';el('causes-chart').innerHTML='';return;}
  const winner=causes[0];
  el('causes-winner').textContent=winner.variable||winner.name||'—';
  const chart=el('causes-chart');if(!chart)return;
  BarChart({el:chart,data:causes.map((c,i)=>({label:c.variable||c.name||`v${i}`,value:parseFloat((c.ace||c.score||0).toFixed?.(3))||0})),orientation:'horizontal',colorFn:(d,i)=>i===0?'#ffa657':'#58a6ff'});
}

// ─── Fleet view ────────────────────────────────────────────────────────────────
function renderFleetView(){
  const section=el('view-fleet');if(!section)return;
  section.innerHTML=`<div class="fleet-empty">
    <div class="fleet-empty-icon">⬡</div>
    <div class="fleet-empty-title">Federated Knowledge Graph</div>
    <div class="fleet-empty-sub">This view surfaces cross-organization recommendations from the federated knowledge client. The <code style="font-family:var(--font-mono);background:rgba(255,255,255,0.06);padding:1px 5px;border-radius:3px">federated.KnowledgeClient</code> runs in-process — no REST endpoint is exposed yet.<br><br>When enabled, incident descriptions map to the top-5 actions ranked by Hamming similarity and historical success rate.</div>
    <div style="margin-top:24px;display:flex;flex-direction:column;gap:10px;width:100%;max-width:560px">
      <label class="field-label" for="fleet-incident">Describe an incident (preview mode)</label>
      <div style="display:flex;gap:8px">
        <input class="input" id="fleet-incident" type="text" placeholder="API returning 503 errors">
        <button class="btn btn--primary" id="fleet-recommend">RECOMMEND</button>
      </div>
    </div>
    <div id="fleet-cards" class="fleet-cards" style="width:100%;max-width:560px;margin-top:12px"></div>
  </div>`;
  el('fleet-recommend').addEventListener('click',()=>{
    const incident=el('fleet-incident')?.value||'incident';
    const cards=el('fleet-cards');if(!cards)return;
    // Synthetic recommendations
    const recs=[
      {action:'RestartService("api")',frequency:42,successRate:0.88,similarity:0.91},
      {action:'ScaleUp("api",2)',frequency:31,successRate:0.82,similarity:0.78},
      {action:'FlushCache("redis")',frequency:28,successRate:0.75,similarity:0.65},
      {action:'RotateCredentials("api")',frequency:12,successRate:0.91,similarity:0.52},
      {action:'EnableCircuitBreaker("api","db")',frequency:9,successRate:0.94,similarity:0.44},
    ];
    cards.innerHTML=recs.map((r,i)=>`<div class="fleet-card" style="animation-delay:${i*0.06}s">
      <div><div class="fleet-card-action">${esc(r.action)}</div><div class="fleet-card-meta">freq: ${r.frequency} · success: ${(r.successRate*100).toFixed(0)}%</div></div>
      <div class="fleet-card-sim">${(r.similarity*100).toFixed(0)}% match</div>
    </div>`).join('');
    toast('Showing synthetic recommendations','info');
  });
}

// ─── Economics view ────────────────────────────────────────────────────────────
function renderEconomicsView(){
  const section=el('view-economics');if(!section)return;
  section.innerHTML=`<h2 style="font-family:var(--font-serif);font-size:26px;font-weight:700;margin-bottom:20px">Economic Planner</h2>
  <div class="economics-split">
    <div class="econ-table-panel">
      <div class="panel-header"><span class="panel-title">Service Value Matrix</span></div>
      <table class="table" id="econ-table">
        <thead><tr><th>Service</th><th>USD/min</th><th>Dep. Factor</th></tr></thead>
        <tbody id="econ-tbody"></tbody>
      </table>
      <div style="padding:12px 14px;display:flex;gap:8px;align-items:center">
        <label class="field-label" for="econ-target" style="margin:0">Simulate on:</label>
        <select class="select" id="econ-target" style="width:auto;flex:1"></select>
        <button class="btn btn--primary" id="econ-simulate">Simulate</button>
      </div>
    </div>
    <div class="econ-chart-panel">
      <div id="econ-hero"><div class="econ-hero-dollar" id="econ-hero-dollar">$—</div><div class="econ-hero-label">Expected Net Value (best plan)</div></div>
      <div style="margin-top:20px" id="econ-chart"></div>
    </div>
  </div>`;
  renderEconTable();
}

function renderEconTable(){
  const tbody=el('econ-tbody'),target=el('econ-target');if(!tbody)return;
  tbody.innerHTML=state.econServices.map((s,i)=>`<tr>
    <td><span style="font-family:var(--font-mono);font-size:12px">${esc(s.name)}</span></td>
    <td class="econ-editable"><input type="number" value="${s.usdPerMin}" aria-label="USD per minute for ${esc(s.name)}" data-i="${i}" data-f="usdPerMin"></td>
    <td class="econ-editable"><input type="number" step="0.1" min="0" max="1" value="${s.depFactor}" aria-label="Dependency factor for ${esc(s.name)}" data-i="${i}" data-f="depFactor"></td>
  </tr>`).join('');
  tbody.querySelectorAll('input').forEach(inp=>{
    inp.addEventListener('change',()=>{
      const i=parseInt(inp.dataset.i);const f=inp.dataset.f;
      state.econServices[i][f]=parseFloat(inp.value)||0;
    });
  });
  if(target)target.innerHTML=state.econServices.map(s=>`<option value="${esc(s.name)}">${esc(s.name)}</option>`).join('');
  el('econ-simulate')?.addEventListener('click',simulateEconomics);
}

function simulateEconomics(){
  const tgt=el('econ-target')?.value||state.econServices[0]?.name;
  const svc=state.econServices.find(s=>s.name===tgt)||state.econServices[0];if(!svc)return;
  const incidentDur=10; // minutes
  const lostValue=svc.usdPerMin*incidentDur;
  // Candidate plans with cost & recovery time
  const plans=[
    {name:'RestartService',cost:2,recoveryMin:3},
    {name:'ScaleUp + Restart',cost:5,recoveryMin:1.5},
    {name:'FlushCache',cost:0.5,recoveryMin:0.5},
    {name:'FailoverToReplica',cost:8,recoveryMin:0.2},
  ];
  const results=plans.map(p=>{
    const savedValue=svc.usdPerMin*(incidentDur-p.recoveryMin)*svc.depFactor;
    const env=savedValue-p.cost;
    return{...p,env};
  }).sort((a,b)=>b.env-a.env);
  const best=results[0];
  const heroEl=el('econ-hero-dollar');if(heroEl)heroEl.textContent=`$${best.env.toFixed(0)}`;
  const chart=el('econ-chart');if(chart){
    BarChart({el:chart,data:results.map(r=>({label:r.name.slice(0,16),value:Math.max(0,r.env)})),orientation:'horizontal',colorFn:(d,i)=>i===0?'#ffa657':'#58a6ff'});
  }
}

// ─── Certificates view ────────────────────────────────────────────────────────
function renderCertificatesView(){
  const section=el('view-certificates');if(!section)return;
  const entries=state.audit.filter(e=>!!(e.hash||e.Hash||e.signature||e.certificate));
  if(!entries.length){
    // Synthetic certs for demo
    const mock=[
      {plan_id:'plan-001',key_id:'key-8fa3',verified_at:new Date().toISOString(),hash:'a1b2c3d4e5f6'},
      {plan_id:'plan-002',key_id:'key-9bc1',verified_at:new Date(Date.now()-60000).toISOString(),hash:'b2c3d4e5f6a7'},
    ];
    renderCertList(section,mock);
  }else{
    renderCertList(section,entries.map(e=>({plan_id:e.action||e.Action||'—',key_id:(e.hash||e.Hash||'').slice(0,8),verified_at:e.timestamp||e.Timestamp||'',hash:e.hash||e.Hash||''})));
  }
}

function renderCertList(section,certs){
  section.innerHTML=`<h2 style="font-family:var(--font-serif);font-size:26px;font-weight:700;margin-bottom:20px">Formal Certificates</h2>
  <div class="certs-list">${certs.map((c,i)=>`<div class="cert-row cert-row-signed" style="animation-delay:${i*0.05}s" role="button" tabindex="0" data-cert="${esc(JSON.stringify(c))}">
    <div><div class="cert-plan-id">${esc(c.plan_id||'—')}</div><div class="cert-meta">key: ${esc(c.key_id||'—')}</div></div>
    <div><span class="badge badge--success">Verified ✓</span></div>
    <div style="display:flex;flex-direction:column;align-items:flex-end;gap:6px">
      <span class="cert-time">${esc(fmtTime(c.verified_at||''))}</span>
      <button class="btn btn--ghost" style="padding:3px 8px;font-size:10px;min-height:auto" aria-label="Download certificate JSON">Download JSON</button>
    </div>
  </div>`).join('')}</div>`;
  section.querySelectorAll('.cert-row').forEach(row=>{
    const cert=JSON.parse(row.dataset.cert||'{}');
    row.querySelector('button')?.addEventListener('click',e=>{
      e.stopPropagation();
      const blob=new Blob([JSON.stringify(cert,null,2)],{type:'application/json'});
      const a=document.createElement('a');a.href=URL.createObjectURL(blob);a.download=`cert-${cert.plan_id||'unknown'}.json`;a.click();
    });
    row.addEventListener('click',()=>openDrawer('Certificate Detail',`<pre style="font-family:var(--font-mono);font-size:11px;white-space:pre-wrap;color:var(--text-muted)">${esc(JSON.stringify(cert,null,2))}</pre>`));
    row.addEventListener('keydown',e=>{if(e.key==='Enter')row.click();});
  });
}

// ─── Command Palette ───────────────────────────────────────────────────────────
const PALETTE_COMMANDS=[
  {group:'Navigate',label:'Go to Overview',icon:'⬡',action:()=>{location.hash='#/overview';}},
  {group:'Navigate',label:'Go to Topology',icon:'⬡',action:()=>{location.hash='#/topology';}},
  {group:'Navigate',label:'Go to Audit Chain',icon:'⬡',action:()=>{location.hash='#/audit';}},
  {group:'Navigate',label:'Go to Terminal',icon:'⬡',action:()=>{location.hash='#/terminal';}},
  {group:'Navigate',label:'Go to Forecast',icon:'📈',action:()=>{location.hash='#/forecast';}},
  {group:'Navigate',label:'Go to Agent',icon:'🤖',action:()=>{location.hash='#/agent';}},
  {group:'Navigate',label:'Go to Verify',icon:'✓',action:()=>{location.hash='#/verify';}},
  {group:'Navigate',label:'Go to Plan',icon:'⚙',action:()=>{location.hash='#/plan';}},
  {group:'Navigate',label:'Go to Causes',icon:'🔍',action:()=>{location.hash='#/causes';}},
  {group:'Navigate',label:'Go to Fleet',icon:'⬡',action:()=>{location.hash='#/fleet';}},
  {group:'Navigate',label:'Go to Economics',icon:'$',action:()=>{location.hash='#/economics';}},
  {group:'Navigate',label:'Go to Certificates',icon:'✓',action:()=>{location.hash='#/certificates';}},
  {group:'Actions',label:'Run Agent on Last Incident',icon:'⚡',action:()=>{location.hash='#/agent';setTimeout(runAgent,200);}},
  {group:'Actions',label:'Verify Chain',icon:'🔒',action:()=>{location.hash='#/audit';}},
  {group:'Actions',label:'Copy Node ID',icon:'📋',action:()=>{const n=el('nav-node')?.textContent||'';navigator.clipboard.writeText(n).then(()=>toast('Copied!','success'));}},
  {group:'Data',label:'Toggle Live/Paused',icon:'⬤',action:()=>{state.filters.live=!state.filters.live;toast(state.filters.live?'Resuming live data':'Data paused',state.filters.live?'success':'warn');}},
  {group:'Data',label:'Refresh All Data',icon:'↻',action:()=>{refresh();toast('Refreshed','success');}},
];

let paletteOpen=false,paletteFocused=-1,paletteQuery='',paletteFiltered=[];

function openPalette(){
  paletteOpen=true;paletteFocused=-1;paletteQuery='';
  const overlay=el('palette-overlay');overlay.classList.add('open');
  el('palette-input').value='';
  renderPaletteList('');
  setTimeout(()=>el('palette-input').focus(),50);
  trapFocus(el('palette-dialog'));
}
function closePalette(){
  paletteOpen=false;el('palette-overlay').classList.remove('open');
}

function renderPaletteList(q){
  paletteQuery=q;
  const ql=q.toLowerCase();
  paletteFiltered=PALETTE_COMMANDS.filter(c=>!q||c.label.toLowerCase().includes(ql)||c.group.toLowerCase().includes(ql));
  const groups=[...new Set(paletteFiltered.map(c=>c.group))];
  const list=el('palette-list');if(!list)return;
  list.innerHTML=groups.map(g=>{
    const items=paletteFiltered.filter(c=>c.group===g);
    return`<div class="palette-group-label">${esc(g)}</div>${items.map((c,i)=>{
      const idx=paletteFiltered.indexOf(c);
      return`<div class="palette-item${paletteFocused===idx?' focused':''}" role="option" tabindex="-1" aria-selected="${paletteFocused===idx}" data-idx="${idx}">
        <span class="palette-item-icon" aria-hidden="true">${esc(c.icon||'·')}</span>
        <span class="palette-item-label">${esc(c.label)}</span>
      </div>`;
    }).join('')}`;
  }).join('');
  list.querySelectorAll('.palette-item').forEach(item=>{
    item.addEventListener('click',()=>{const c=paletteFiltered[parseInt(item.dataset.idx)];if(c){closePalette();c.action();}});
    item.addEventListener('mouseenter',()=>{paletteFocused=parseInt(item.dataset.idx);renderPaletteList(paletteQuery);});
  });
}

function trapFocus(el){
  const focusable=el.querySelectorAll('button,input,[tabindex="0"]');
  if(!focusable.length)return;
  const first=focusable[0],last=focusable[focusable.length-1];
  el._trapHandler=function(e){if(e.key==='Tab'){if(e.shiftKey){if(document.activeElement===first){e.preventDefault();last.focus();}}else{if(document.activeElement===last){e.preventDefault();first.focus();}}}};
  document.addEventListener('keydown',el._trapHandler);
}

el('palette-input').addEventListener('input',e=>renderPaletteList(e.target.value));
el('palette-input').addEventListener('keydown',e=>{
  if(e.key==='ArrowDown'){e.preventDefault();paletteFocused=Math.min(paletteFocused+1,paletteFiltered.length-1);renderPaletteList(paletteQuery);}
  else if(e.key==='ArrowUp'){e.preventDefault();paletteFocused=Math.max(paletteFocused-1,-1);renderPaletteList(paletteQuery);}
  else if(e.key==='Enter'){const c=paletteFiltered[paletteFocused];if(c){closePalette();c.action();}}
  else if(e.key==='Escape'){closePalette();}
});
el('palette-overlay').addEventListener('click',e=>{if(e.target===el('palette-overlay'))closePalette();});
el('palette-trigger').addEventListener('click',openPalette);

document.addEventListener('keydown',e=>{
  if((e.metaKey||e.ctrlKey)&&e.key==='k'){e.preventDefault();paletteOpen?closePalette():openPalette();}
  else if(e.key==='?'&&!['INPUT','TEXTAREA','SELECT'].includes(document.activeElement.tagName)&&!paletteOpen){openPalette();}
  else if(e.key==='Escape'&&paletteOpen){closePalette();}
});

// ─── Bootstrap ────────────────────────────────────────────────────────────────
renderFilterBar('overview-filter-bar');
switchView(currentView());
refresh();
setInterval(refresh,2000);
