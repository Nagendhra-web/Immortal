// chart.js — pure-SVG chart primitives. No deps.
//
// API:
//   chart.line({ container, series, xAccessor, yAccessor, ... })
//   chart.area({ ... })            alias for line with area=true
//   chart.bar({ container, data, xAccessor, yAccessor, horizontal })
//   chart.sparkline(container, values, { color, area })
//   chart.heatmap(container, matrix, { rowLabels, colLabels, max })
//   chart.graph(container, { nodes, edges, onNodeClick })
//
// All primitives:
//   - respect container size via ResizeObserver
//   - read colors from CSS tokens (--chart-series-N) at render time
//   - render an inline chart tooltip on hover
//   - emit CustomEvent('chart:hover', { t }) on container for cross-chart sync

import { h, token, num as fmtNum, throttle } from "../lib/fmt.js";

const NS = "http://www.w3.org/2000/svg";
function s(tag, attrs = {}, ...children) {
  const el = document.createElementNS(NS, tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v === null || v === undefined || v === false) continue;
    if (k === "class")  el.setAttribute("class", v);
    else if (k.startsWith("on") && typeof v === "function") el.addEventListener(k.slice(2).toLowerCase(), v);
    else el.setAttribute(k, v);
  }
  for (const c of children.flat(Infinity)) {
    if (c === null || c === undefined || c === false) continue;
    if (c instanceof Node) el.appendChild(c);
    else el.appendChild(document.createTextNode(String(c)));
  }
  return el;
}

function seriesColor(idx) {
  return token(`--chart-series-${(idx % 6) + 1}`) || "#7cc";
}

function linearScale(domain, range) {
  const [d0, d1] = domain;
  const [r0, r1] = range;
  const span = d1 - d0 || 1;
  return (v) => r0 + ((v - d0) / span) * (r1 - r0);
}

function niceTicks(d0, d1, count = 5) {
  if (d0 === d1) return [d0];
  const span = d1 - d0;
  const step0 = span / count;
  const mag = Math.pow(10, Math.floor(Math.log10(step0)));
  const norm = step0 / mag;
  let step;
  if      (norm < 1.5) step = 1 * mag;
  else if (norm < 3)   step = 2 * mag;
  else if (norm < 7)   step = 5 * mag;
  else                 step = 10 * mag;
  const ticks = [];
  const start = Math.ceil(d0 / step) * step;
  for (let v = start; v <= d1 + 1e-9; v += step) ticks.push(v);
  return ticks;
}

function axisFormat(v) {
  const abs = Math.abs(v);
  if (abs >= 1e9) return (v / 1e9).toFixed(1) + "B";
  if (abs >= 1e6) return (v / 1e6).toFixed(1) + "M";
  if (abs >= 1e3) return (v / 1e3).toFixed(1) + "k";
  if (abs < 1 && abs > 0) return v.toFixed(2);
  return v.toFixed(0);
}

function timeFormat(t) {
  const d = new Date(t);
  return d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
}

// ── line / area ───────────────────────────────────────────────────────────
function renderLine({ container, series, xAccessor = (d) => d[0], yAccessor = (d) => d[1], area = false, yFormat = axisFormat, xFormat = timeFormat, yDomain, xDomain } = {}) {
  const padL = 48, padR = 16, padT = 18, padB = 28;
  const W = container.clientWidth  || 600;
  const H = container.clientHeight || 220;

  const xs = []; const ys = [];
  for (const sr of series) for (const d of sr.data) { xs.push(xAccessor(d)); ys.push(yAccessor(d)); }
  const xd = xDomain || [Math.min(...xs), Math.max(...xs)];
  let yd = yDomain || [Math.min(...ys), Math.max(...ys)];
  if (yd[0] === yd[1]) yd = [yd[0] - 1, yd[1] + 1];
  yd[0] = Math.min(0, yd[0]);

  const x = linearScale(xd, [padL, W - padR]);
  const y = linearScale(yd, [H - padB, padT]);

  const root = s("svg", { viewBox: `0 0 ${W} ${H}`, preserveAspectRatio: "none" });

  // grid + y axis
  const yTicks = niceTicks(yd[0], yd[1], 5);
  for (const t of yTicks) {
    root.appendChild(s("line", { class: "chart__grid", x1: padL, x2: W - padR, y1: y(t), y2: y(t) }));
    root.appendChild(s("text", { class: "chart__tick", x: padL - 6, y: y(t) + 3, "text-anchor": "end" }, yFormat(t)));
  }
  // x axis
  const xTicks = niceTicks(xd[0], xd[1], 5);
  for (const t of xTicks) {
    root.appendChild(s("text", { class: "chart__tick", x: x(t), y: H - padB + 14, "text-anchor": "middle" }, xFormat(t)));
  }
  root.appendChild(s("line", { class: "chart__axis-line", x1: padL, x2: W - padR, y1: H - padB, y2: H - padB }));

  // bands first
  series.forEach((sr, i) => {
    if (sr.kind === "band" && sr.lower && sr.upper) {
      const path = [];
      for (const d of sr.upper) path.push(`${x(xAccessor(d))},${y(yAccessor(d))}`);
      for (let j = sr.lower.length - 1; j >= 0; j--) { const d = sr.lower[j]; path.push(`${x(xAccessor(d))},${y(yAccessor(d))}`); }
      root.appendChild(s("polygon", {
        points: path.join(" "),
        fill: sr.color || seriesColor(i),
        class: "chart__area",
      }));
    }
  });

  // series
  series.forEach((sr, i) => {
    if (sr.kind === "band") return;
    const color = sr.color || seriesColor(i);
    const pts = sr.data.map((d) => `${x(xAccessor(d))},${y(yAccessor(d))}`).join(" L ");
    if (area || sr.area) {
      const first = sr.data[0], last = sr.data[sr.data.length - 1];
      const areaPath = `M ${x(xAccessor(first))},${y(yd[0])} L ${pts} L ${x(xAccessor(last))},${y(yd[0])} Z`;
      root.appendChild(s("path", { class: "chart__area", d: areaPath, fill: color }));
    }
    root.appendChild(s("path", {
      class: "chart__series" + (sr.dashed ? " chart__series--dashed" : ""),
      d: `M ${pts}`,
      stroke: color,
    }));
  });

  // crosshair + tooltip
  const crosshair = s("line", { class: "chart__crosshair", y1: padT, y2: H - padB });
  crosshair.setAttribute("x1", -9999); crosshair.setAttribute("x2", -9999);
  root.appendChild(crosshair);

  const tooltip = h("div", { class: "chart__tooltip", style: { opacity: "0" } });
  container.appendChild(tooltip);

  const onMove = throttle((evt) => {
    const rect = root.getBoundingClientRect();
    const px = ((evt.clientX - rect.left) / rect.width) * W;
    if (px < padL || px > W - padR) { crosshair.setAttribute("x1", -9999); crosshair.setAttribute("x2", -9999); tooltip.style.opacity = "0"; return; }
    // Snap to nearest x in first series.
    const ref = series.find((s) => s.data && s.data.length);
    if (!ref) return;
    let nearest = ref.data[0], nd = Infinity;
    for (const d of ref.data) {
      const dd = Math.abs(x(xAccessor(d)) - px);
      if (dd < nd) { nd = dd; nearest = d; }
    }
    const snapX = x(xAccessor(nearest));
    crosshair.setAttribute("x1", snapX); crosshair.setAttribute("x2", snapX);

    tooltip.innerHTML = "";
    tooltip.appendChild(h("div", { class: "chart__tooltip__title" }, xFormat(xAccessor(nearest))));
    series.forEach((sr, i) => {
      if (sr.kind === "band") return;
      const color = sr.color || seriesColor(i);
      const match = sr.data.find((d) => xAccessor(d) === xAccessor(nearest));
      if (!match) return;
      tooltip.appendChild(h("div", { class: "chart__tooltip__row" },
        h("span", {}, h("span", { class: "chart__tooltip__swatch", style: { background: color } }), sr.name || `s${i+1}`),
        h("span", {}, yFormat(yAccessor(match))),
      ));
    });
    const px2 = snapX / W * rect.width + rect.left - container.getBoundingClientRect().left;
    tooltip.style.left = px2 + "px";
    tooltip.style.top  = (y(yAccessor(nearest)) / H * rect.height - 10) + "px";
    tooltip.style.opacity = "1";
    container.dispatchEvent(new CustomEvent("chart:hover", { detail: { t: xAccessor(nearest) } }));
  }, 16);

  root.addEventListener("mousemove", onMove);
  root.addEventListener("mouseleave", () => { crosshair.setAttribute("x1", -9999); tooltip.style.opacity = "0"; });

  container.innerHTML = "";
  container.classList.add("chart");
  container.style.position ||= "relative";
  container.appendChild(root);
  container.appendChild(tooltip);

  // legend
  const legend = h("div", { class: "chart__legend" });
  series.forEach((sr, i) => {
    if (sr.kind === "band") return;
    legend.appendChild(h("div", { class: "chart__legend__item" },
      h("span", { class: "chart__legend__swatch", style: { background: sr.color || seriesColor(i) } }),
      sr.name || `s${i+1}`,
    ));
  });
  if (legend.childElementCount) container.appendChild(legend);
}

function bindResize(container, render) {
  let frame = null;
  const ro = new ResizeObserver(() => {
    cancelAnimationFrame(frame);
    frame = requestAnimationFrame(render);
  });
  ro.observe(container);
  return () => ro.disconnect();
}

// ── public line ───────────────────────────────────────────────────────────
function line(opts) {
  const render = () => renderLine(opts);
  render();
  return { destroy: bindResize(opts.container, render) };
}

// ── bar ───────────────────────────────────────────────────────────────────
function bar({ container, data, xAccessor = (d) => d.label, yAccessor = (d) => d.value, horizontal = false, colorAccessor, yFormat = axisFormat } = {}) {
  const render = () => {
    const padL = horizontal ? 120 : 40, padR = 16, padT = 12, padB = 28;
    const W = container.clientWidth || 600;
    const H = container.clientHeight || Math.max(200, data.length * 28 + 40);
    const root = s("svg", { viewBox: `0 0 ${W} ${H}` });
    const maxV = Math.max(1, ...data.map(yAccessor));

    if (horizontal) {
      const rowH = (H - padT - padB) / data.length;
      const x = linearScale([0, maxV], [padL, W - padR]);
      data.forEach((d, i) => {
        const y0 = padT + i * rowH + 3;
        const label = xAccessor(d);
        const v = yAccessor(d);
        const color = colorAccessor ? colorAccessor(d, i) : seriesColor(i);
        root.appendChild(s("text", { class: "chart__tick", x: padL - 6, y: y0 + rowH / 2 + 3, "text-anchor": "end" }, label));
        root.appendChild(s("rect", {
          class: "chart__bar",
          x: padL, y: y0, width: Math.max(0, x(v) - padL), height: rowH - 6,
          fill: color, rx: 3,
        }));
        root.appendChild(s("text", {
          class: "chart__tick", x: x(v) + 6, y: y0 + rowH / 2 + 3, "text-anchor": "start",
        }, yFormat(v)));
      });
    } else {
      const barW = (W - padL - padR) / data.length;
      const y = linearScale([0, maxV], [H - padB, padT]);
      data.forEach((d, i) => {
        const x0 = padL + i * barW + barW * 0.15;
        const w  = barW * 0.7;
        const v = yAccessor(d);
        root.appendChild(s("rect", { class: "chart__bar", x: x0, y: y(v), width: w, height: H - padB - y(v), fill: seriesColor(i), rx: 3 }));
        root.appendChild(s("text", { class: "chart__tick", x: x0 + w / 2, y: H - padB + 14, "text-anchor": "middle" }, xAccessor(d)));
      });
      const ticks = niceTicks(0, maxV, 4);
      for (const t of ticks) {
        root.appendChild(s("line", { class: "chart__grid", x1: padL, x2: W - padR, y1: y(t), y2: y(t) }));
        root.appendChild(s("text", { class: "chart__tick", x: padL - 4, y: y(t) + 3, "text-anchor": "end" }, yFormat(t)));
      }
    }
    container.innerHTML = ""; container.classList.add("chart"); container.appendChild(root);
  };
  render();
  return { destroy: bindResize(container, render) };
}

// ── sparkline ─────────────────────────────────────────────────────────────
function sparkline(container, values, { color = "currentColor", area = true } = {}) {
  if (!values || values.length < 2) { container.innerHTML = ""; return; }
  const W = container.clientWidth  || 96;
  const H = container.clientHeight || 28;
  const d0 = Math.min(...values), d1 = Math.max(...values);
  const y = linearScale([d0, d1 === d0 ? d1 + 1 : d1], [H - 2, 2]);
  const x = linearScale([0, values.length - 1], [0, W]);
  const pts = values.map((v, i) => `${x(i)},${y(v)}`).join(" L ");
  const root = s("svg", { class: "sparkline", viewBox: `0 0 ${W} ${H}`, preserveAspectRatio: "none", style: `color:${color}` });
  if (area) root.appendChild(s("path", { class: "area", d: `M 0,${H} L ${pts} L ${W},${H} Z` }));
  root.appendChild(s("path", { d: `M ${pts}`, stroke: color }));
  container.innerHTML = "";
  container.appendChild(root);
}

// ── heatmap ───────────────────────────────────────────────────────────────
function heatmap(container, matrix, { rowLabels = [], colLabels = [], colorLow, colorHigh, max } = {}) {
  const rows = matrix.length, cols = matrix[0]?.length || 0;
  const padL = 90, padR = 8, padT = 18, padB = 8;
  const W = container.clientWidth || 480;
  const cellW = (W - padL - padR) / cols;
  const cellH = 20;
  const H = padT + padB + rows * cellH;
  const root = s("svg", { viewBox: `0 0 ${W} ${H}` });
  const lo = colorLow  || token("--ink-2");
  const hi = colorHigh || token("--accent");
  const m = max ?? Math.max(...matrix.flat().map(Math.abs), 0.0001);

  for (let r = 0; r < rows; r++) {
    root.appendChild(s("text", { class: "chart__tick", x: padL - 4, y: padT + r * cellH + cellH / 2 + 3, "text-anchor": "end" }, rowLabels[r] || ""));
    for (let c = 0; c < cols; c++) {
      const v = matrix[r][c] ?? 0;
      const t = Math.min(1, Math.abs(v) / m);
      root.appendChild(s("rect", {
        class: "heatmap__cell",
        x: padL + c * cellW, y: padT + r * cellH,
        width: cellW, height: cellH,
        fill: `color-mix(in oklab, ${hi} ${Math.round(t * 100)}%, ${lo})`,
      }));
    }
  }
  for (let c = 0; c < cols; c++) {
    root.appendChild(s("text", { class: "chart__tick", x: padL + c * cellW + cellW / 2, y: padT - 4, "text-anchor": "middle" }, colLabels[c] || ""));
  }
  container.innerHTML = ""; container.classList.add("chart"); container.appendChild(root);
}

// ── graph (force layout) ──────────────────────────────────────────────────
function graph(container, { nodes, edges, onNodeClick, highlighted } = {}) {
  const W = container.clientWidth  || 600;
  const H = container.clientHeight || 420;
  const n = nodes.length;
  // Initial positions: circle.
  const cx = W / 2, cy = H / 2, R = Math.min(W, H) * 0.36;
  nodes.forEach((nd, i) => {
    if (nd.x == null) nd.x = cx + Math.cos((i / n) * Math.PI * 2) * R;
    if (nd.y == null) nd.y = cy + Math.sin((i / n) * Math.PI * 2) * R;
  });
  const byId = new Map(nodes.map((nd) => [nd.id, nd]));
  const linksIn  = new Map(nodes.map((n) => [n.id, new Set()]));
  const linksOut = new Map(nodes.map((n) => [n.id, new Set()]));
  edges.forEach((e) => {
    if (linksOut.has(e.from)) linksOut.get(e.from).add(e.to);
    if (linksIn.has(e.to))    linksIn.get(e.to).add(e.from);
  });

  // Run simple Fruchterman-Reingold for 200 iters.
  const area = W * H;
  const k = Math.sqrt(area / n);
  let temp = Math.min(W, H) / 8;
  for (let iter = 0; iter < 200; iter++) {
    for (const v of nodes) { v.dx = 0; v.dy = 0; }
    // repulsion
    for (let i = 0; i < n; i++) {
      for (let j = i + 1; j < n; j++) {
        const a = nodes[i], b = nodes[j];
        let dx = a.x - b.x, dy = a.y - b.y;
        let dist = Math.sqrt(dx * dx + dy * dy) || 0.01;
        const force = (k * k) / dist;
        dx = (dx / dist) * force; dy = (dy / dist) * force;
        a.dx += dx; a.dy += dy; b.dx -= dx; b.dy -= dy;
      }
    }
    // attraction via edges
    for (const e of edges) {
      const a = byId.get(e.from), b = byId.get(e.to);
      if (!a || !b) continue;
      const dx = a.x - b.x, dy = a.y - b.y;
      const dist = Math.sqrt(dx * dx + dy * dy) || 0.01;
      const force = (dist * dist) / k;
      const fx = (dx / dist) * force, fy = (dy / dist) * force;
      a.dx -= fx; a.dy -= fy; b.dx += fx; b.dy += fy;
    }
    for (const v of nodes) {
      const d = Math.sqrt(v.dx * v.dx + v.dy * v.dy) || 0.01;
      v.x += (v.dx / d) * Math.min(d, temp);
      v.y += (v.dy / d) * Math.min(d, temp);
      v.x = Math.max(30, Math.min(W - 30, v.x));
      v.y = Math.max(30, Math.min(H - 30, v.y));
    }
    temp *= 0.95;
  }

  const root = s("svg", { viewBox: `0 0 ${W} ${H}` });
  // Arrowhead def.
  root.appendChild(s("defs", {},
    s("marker", { id: "arrow", viewBox: "0 0 10 10", refX: 8, refY: 5, markerWidth: 6, markerHeight: 6, orient: "auto" },
      s("path", { d: "M0,0 L10,5 L0,10 z", fill: token("--chart-grid") })
    )
  ));

  // edges
  edges.forEach((e) => {
    const a = byId.get(e.from), b = byId.get(e.to);
    if (!a || !b) return;
    const active = highlighted?.nodes?.has(e.from) && highlighted?.nodes?.has(e.to);
    root.appendChild(s("line", {
      class: "graph__edge" + (active ? " graph__edge--active" : ""),
      x1: a.x, y1: a.y, x2: b.x, y2: b.y,
      "marker-end": "url(#arrow)",
    }));
    if (e.label) {
      const mx = (a.x + b.x) / 2, my = (a.y + b.y) / 2;
      root.appendChild(s("text", { class: "graph__edge-label", x: mx, y: my - 4, "text-anchor": "middle" }, e.label));
    }
  });

  // nodes
  nodes.forEach((nd) => {
    const r = nd.size || 14;
    const color = nd.color || token("--signal-cyan");
    const g = s("g", {
      style: "cursor:pointer",
      onClick: () => onNodeClick && onNodeClick(nd),
    });
    g.appendChild(s("circle", {
      class: "graph__node",
      cx: nd.x, cy: nd.y, r,
      fill: color,
    }));
    g.appendChild(s("text", {
      class: "graph__node-label",
      x: nd.x, y: nd.y + r + 12, "text-anchor": "middle",
    }, nd.label || nd.id));
    root.appendChild(g);
  });

  container.innerHTML = "";
  container.classList.add("chart");
  container.appendChild(root);
}

export const chart = {
  line,
  area: (o) => line({ ...o, area: true }),
  bar,
  sparkline,
  heatmap,
  graph,
};
