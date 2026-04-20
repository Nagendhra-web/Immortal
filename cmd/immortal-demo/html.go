package main

import (
	"fmt"
	"html"
	"math"
	"os"
	"strings"
	"time"
)

// writeHTML renders a Story to a standalone HTML file at path. Inline
// CSS and SVG only, no external dependencies. File opens the same in any
// browser, offline, on any OS.
func writeHTML(path string, s *Story) error {
	var b strings.Builder
	renderHTML(&b, s)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func renderHTML(b *strings.Builder, s *Story) {
	b.WriteString(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>Immortal Incident Report</title>
<style>
  :root {
    color-scheme: dark;
    --bg: oklch(0.09 0.012 252);
    --card: oklch(0.12 0.015 252);
    --elev: oklch(0.16 0.018 252);
    --border: oklch(0.30 0.020 252);
    --text: oklch(0.82 0.010 252);
    --muted: oklch(0.58 0.015 252);
    --strong: oklch(0.98 0.004 252);
    --ok: oklch(0.74 0.18 148);
    --warn: oklch(0.80 0.17 78);
    --err: oklch(0.68 0.22 25);
    --accent: oklch(0.78 0.14 210);
    --violet: oklch(0.70 0.20 295);
    --lime: oklch(0.82 0.18 130);
    --pink: oklch(0.72 0.22 0);
  }
  * { box-sizing: border-box; }
  body {
    margin: 0; padding: 48px 24px; background: var(--bg);
    color: var(--text);
    font: 14px/1.55 ui-sans-serif, system-ui, -apple-system, Inter, sans-serif;
    background-image:
      radial-gradient(ellipse 80% 40% at 20% 0%, color-mix(in oklab, var(--accent) 6%, transparent) 0%, transparent 60%),
      radial-gradient(ellipse 80% 40% at 80% 100%, color-mix(in oklab, var(--violet) 5%, transparent) 0%, transparent 60%);
    background-attachment: fixed;
    min-height: 100vh;
  }
  .wrap { max-width: 1120px; margin: 0 auto; }
  .hero { margin-bottom: 48px; }
  .eyebrow {
    font: 700 11px/1 ui-monospace, monospace;
    letter-spacing: 0.24em;
    color: var(--ok);
    text-transform: uppercase;
    margin-bottom: 16px;
  }
  .eyebrow .dot {
    display: inline-block; width: 8px; height: 8px; border-radius: 50%;
    background: var(--ok); margin-right: 8px;
    box-shadow: 0 0 0 4px color-mix(in oklab, var(--ok) 20%, transparent);
  }
  h1 {
    font: 700 44px/1.05 ui-serif, Georgia, serif;
    letter-spacing: -0.02em;
    color: var(--strong);
    margin: 0 0 12px 0;
    max-width: 900px;
  }
  h1 .accent { color: var(--lime); }
  .subtitle { color: var(--muted); font-size: 16px; max-width: 720px; }
  .meta {
    display: flex; gap: 24px; flex-wrap: wrap;
    margin-top: 24px;
    font: 400 12px/1.4 ui-monospace, monospace;
    color: var(--muted);
  }
  .meta b { color: var(--text); font-weight: 600; }

  section { margin-bottom: 40px; }
  .section-title {
    font: 700 11px/1 ui-monospace, monospace;
    letter-spacing: 0.2em;
    color: var(--muted);
    text-transform: uppercase;
    margin-bottom: 16px;
  }

  .card {
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 14px;
    padding: 24px;
    box-shadow: inset 0 0 0 1px rgb(255 255 255 / 0.04);
  }

  /* Dramatic headline card */
  .dramatic {
    background: linear-gradient(135deg,
      color-mix(in oklab, var(--lime) 14%, var(--card)) 0%,
      var(--card) 60%);
    border: 1px solid color-mix(in oklab, var(--lime) 30%, var(--border));
    padding: 32px;
  }
  .dramatic h2 {
    font: 700 24px/1.25 ui-serif, Georgia, serif;
    color: var(--strong);
    margin: 0 0 8px 0;
    letter-spacing: -0.01em;
  }
  .dramatic p { margin: 0; color: var(--muted); font-size: 15px; }

  /* Timeline */
  .timeline { position: relative; padding-left: 32px; }
  .timeline::before {
    content: ""; position: absolute; left: 8px; top: 6px; bottom: 6px;
    width: 2px;
    background: linear-gradient(to bottom,
      color-mix(in oklab, var(--accent) 60%, transparent),
      color-mix(in oklab, var(--violet) 60%, transparent));
    border-radius: 2px;
  }
  .tle { position: relative; margin-bottom: 16px; padding: 12px 16px;
         background: var(--elev); border: 1px solid var(--border); border-radius: 10px; }
  .tle::before {
    content: ""; position: absolute; left: -28px; top: 18px;
    width: 10px; height: 10px; border-radius: 50%;
    background: var(--muted); border: 2px solid var(--bg);
  }
  .tle[data-kind="observe"]::before { background: var(--accent); }
  .tle[data-kind="detect"]::before  { background: var(--warn); }
  .tle[data-kind="contract"]::before{ background: var(--violet); }
  .tle[data-kind="heal"]::before    { background: var(--lime); }
  .tle[data-kind="verdict"]::before { background: var(--strong); box-shadow: 0 0 0 4px color-mix(in oklab, var(--lime) 30%, transparent); }
  .tle[data-kind="advisor"]::before { background: var(--pink); }
  .tle[data-emphasis="true"] { border-color: color-mix(in oklab, var(--lime) 40%, var(--border)); background: color-mix(in oklab, var(--lime) 6%, var(--elev)); }
  .tle-head { display: flex; gap: 12px; align-items: baseline; }
  .tle-ts   { font: 400 11px/1 ui-monospace, monospace; color: var(--muted); }
  .tle-kind { font: 700 10px/1 ui-monospace, monospace; color: var(--muted); text-transform: uppercase; letter-spacing: 0.12em; }
  .tle-src  { font-weight: 600; color: var(--strong); }
  .tle-msg  { margin-top: 4px; color: var(--text); }
  .tle-detail { margin-top: 6px; color: var(--muted); font-size: 13px; font-style: italic; }

  /* Counterfactual */
  .vs { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
  @media (max-width: 720px) { .vs { grid-template-columns: 1fr; } }
  .vs .half { padding: 24px; border-radius: 14px; border: 1px solid var(--border); }
  .vs .without { background: color-mix(in oklab, var(--err) 8%, var(--card)); border-color: color-mix(in oklab, var(--err) 40%, var(--border)); }
  .vs .with    { background: color-mix(in oklab, var(--lime) 8%, var(--card)); border-color: color-mix(in oklab, var(--lime) 40%, var(--border)); }
  .vs h3 { margin: 0 0 6px 0; font: 700 13px/1 ui-monospace, monospace; letter-spacing: 0.16em; text-transform: uppercase; }
  .vs .without h3 { color: var(--err); }
  .vs .with h3    { color: var(--lime); }
  .vs p.sub { margin: 0 0 20px 0; color: var(--muted); font-size: 12px; }
  .metric { display: grid; grid-template-columns: 1fr auto; align-items: center; gap: 8px; margin-bottom: 14px; }
  .metric .lbl { color: var(--muted); font-size: 12px; text-transform: uppercase; letter-spacing: 0.08em; font-weight: 600; }
  .metric .val { font: 700 20px/1 ui-monospace, monospace; color: var(--strong); }
  .bar { grid-column: 1 / -1; height: 6px; border-radius: 3px; background: color-mix(in oklab, var(--border) 80%, transparent); overflow: hidden; }
  .bar .fill { height: 100%; border-radius: 3px; }
  .without .bar .fill { background: var(--err); }
  .with .bar .fill    { background: var(--lime); }

  /* Causal graph */
  .svgwrap { background: var(--card); border: 1px solid var(--border); border-radius: 14px; padding: 24px; }
  .node { fill: var(--elev); stroke: var(--border); stroke-width: 1.5; }
  .node.root     { stroke: var(--err);    fill: color-mix(in oklab, var(--err)    10%, var(--elev)); }
  .node.relay    { stroke: var(--warn);   fill: color-mix(in oklab, var(--warn)   10%, var(--elev)); }
  .node.victim   { stroke: var(--accent); fill: color-mix(in oklab, var(--accent) 10%, var(--elev)); }
  .node.action   { stroke: var(--lime);   fill: color-mix(in oklab, var(--lime)   10%, var(--elev)); }
  .nlabel { fill: var(--strong); font: 700 12px/1 ui-sans-serif, system-ui, sans-serif; text-anchor: middle; }
  .nsub   { fill: var(--muted);  font: 400 10px/1 ui-monospace, monospace; text-anchor: middle; }
  .edge   { stroke: var(--muted); stroke-width: 1.5; fill: none; }
  .edge.healed_by { stroke: var(--lime); stroke-dasharray: 4 3; }
  .elabel { fill: var(--muted); font: 400 10px/1 ui-monospace, monospace; text-anchor: middle; }

  /* Verdict card */
  .verdict h3 { margin: 0 0 12px 0; font: 700 11px/1 ui-monospace, monospace; letter-spacing: 0.2em; text-transform: uppercase; color: var(--muted); }
  .verdict .cause { color: var(--strong); font-size: 16px; line-height: 1.5; margin-bottom: 20px; }
  .verdict ul { margin: 0 0 20px 0; padding-left: 18px; color: var(--text); }
  .verdict ul li { margin-bottom: 6px; }
  .verdict .outcome { color: var(--text); background: color-mix(in oklab, var(--lime) 8%, transparent); padding: 14px 16px; border-radius: 10px; border: 1px solid color-mix(in oklab, var(--lime) 25%, var(--border)); margin-bottom: 20px; }
  .verdict .conf { font: 700 14px/1 ui-monospace, monospace; color: var(--lime); }

  /* Advisor */
  .advisor-card { background: var(--elev); border: 1px solid var(--border); border-radius: 10px; padding: 16px 18px; }
  .advisor-tag { display: inline-block; font: 700 10px/1 ui-monospace, monospace; letter-spacing: 0.14em; padding: 4px 8px; border-radius: 999px; background: color-mix(in oklab, var(--pink) 18%, transparent); color: var(--pink); text-transform: uppercase; margin-right: 10px; }
  .advisor-title { font-weight: 600; color: var(--strong); }
  .advisor-rationale { color: var(--text); margin: 8px 0 0 0; }
  .advisor-impact { margin-top: 8px; padding-top: 8px; border-top: 1px solid var(--border); color: var(--muted); font-size: 12px; font-family: ui-monospace, monospace; }

  footer { margin-top: 56px; padding-top: 24px; border-top: 1px solid var(--border); color: var(--muted); font-size: 12px; }
  footer code { background: var(--elev); padding: 3px 8px; border-radius: 4px; color: var(--text); }
</style>
</head>
<body>
<div class="wrap">
`)
	// ── Hero ────────────────────────────────────────────────────────────────
	fmt.Fprintf(b, `<header class="hero">
<div class="eyebrow"><span class="dot"></span>Immortal · Incident Report</div>
<h1>%s</h1>
<p class="subtitle">%s</p>
<div class="meta">
  <div>scenario: <b>%s</b></div>
  <div>duration: <b>%s</b></div>
  <div>started: <b>%s</b></div>
  <div>contract: <b>%s</b></div>
</div>
</header>
`,
		htmlEscape(s.Headline),
		htmlEscape(s.Tagline),
		htmlEscape(s.Scenario),
		s.Duration().Round(time.Millisecond).String(),
		s.StartedAt.Format("15:04:05 MST"),
		htmlEscape(s.Contract),
	)

	// ── Dramatic card (repeat + commentary) ────────────────────────────────
	fmt.Fprintf(b, `<section class="dramatic">
<h2>%s</h2>
<p>%s</p>
</section>
`,
		htmlEscape(s.Headline),
		htmlEscape(s.Tagline),
	)

	// ── Counterfactual ─────────────────────────────────────────────────────
	if len(s.Counterfact) > 0 {
		b.WriteString(`<section><div class="section-title">What would have happened vs what actually happened</div><div class="vs">`)
		renderHalf(b, "without", "Without Immortal", "30-minute projection with a human in the loop", s.Counterfact, false)
		renderHalf(b, "with", "With Immortal", "what actually happened, end-to-end autonomous", s.Counterfact, true)
		b.WriteString(`</div></section>`)
	}

	// ── Causal graph ───────────────────────────────────────────────────────
	if len(s.Causal.Nodes) > 0 {
		b.WriteString(`<section><div class="section-title">Causal chain</div><div class="svgwrap">`)
		renderCausalSVG(b, s.Causal.Nodes, s.Causal.Edges)
		b.WriteString(`</div></section>`)
	}

	// ── Timeline ───────────────────────────────────────────────────────────
	if len(s.Timeline) > 0 {
		b.WriteString(`<section><div class="section-title">Timeline</div><div class="timeline">`)
		for _, e := range s.Timeline {
			emphasis := ""
			if e.Emphasis {
				emphasis = `data-emphasis="true"`
			}
			fmt.Fprintf(b, `<div class="tle" data-kind="%s" %s>
<div class="tle-head">
  <span class="tle-ts">%s</span>
  <span class="tle-kind">%s</span>
  <span class="tle-src">%s</span>
</div>
<div class="tle-msg">%s</div>`,
				htmlEscape(e.Kind),
				emphasis,
				e.At.Format("15:04:05.000"),
				htmlEscape(e.Kind),
				htmlEscape(e.Source),
				htmlEscape(e.Message),
			)
			if e.Detail != "" {
				fmt.Fprintf(b, `<div class="tle-detail">%s</div>`, htmlEscape(e.Detail))
			}
			b.WriteString(`</div>`)
		}
		b.WriteString(`</div></section>`)
	}

	// ── Verdict ────────────────────────────────────────────────────────────
	if s.Verdict.Cause != "" {
		v := s.Verdict
		b.WriteString(`<section><div class="section-title">Verdict</div><div class="card verdict">`)
		b.WriteString(`<h3>What happened</h3>`)
		fmt.Fprintf(b, `<p class="cause">%s</p>`, htmlEscape(v.Cause))
		if len(v.Evidence) > 0 {
			b.WriteString(`<h3>Evidence</h3><ul>`)
			for _, e := range v.Evidence {
				fmt.Fprintf(b, `<li>%s</li>`, htmlEscape(e))
			}
			b.WriteString(`</ul>`)
		}
		if len(v.Action) > 0 {
			b.WriteString(`<h3>What I did</h3><ul>`)
			for _, a := range v.Action {
				fmt.Fprintf(b, `<li>%s</li>`, htmlEscape(a))
			}
			b.WriteString(`</ul>`)
		}
		if v.Outcome != "" {
			b.WriteString(`<h3>Outcome</h3>`)
			fmt.Fprintf(b, `<p class="outcome">%s</p>`, htmlEscape(v.Outcome))
		}
		fmt.Fprintf(b, `<p class="conf">Confidence: %.0f%% this resolves the root cause.</p>`, v.Confidence*100)
		b.WriteString(`</div></section>`)
	}

	// ── Advisor ────────────────────────────────────────────────────────────
	if s.TopSuggestion != nil {
		sg := s.TopSuggestion
		b.WriteString(`<section><div class="section-title">Architecture advisor</div><div class="advisor-card">`)
		fmt.Fprintf(b, `<div><span class="advisor-tag">%s</span><span class="advisor-title">%s on %s</span></div>`,
			htmlEscape(sg.Rank()), htmlEscape(sg.Kind.String()), htmlEscape(sg.Service))
		fmt.Fprintf(b, `<p class="advisor-rationale">%s</p>`, htmlEscape(sg.Rationale))
		if sg.Impact != "" {
			fmt.Fprintf(b, `<div class="advisor-impact">%s</div>`, htmlEscape(sg.Impact))
		}
		b.WriteString(`</div></section>`)
	}

	// ── Footer ─────────────────────────────────────────────────────────────
	fmt.Fprintf(b, `<footer>
Regenerate with: <code>immortal-demo --scenario %s --html report.html</code><br>
Immortal %s · Apache 2.0
</footer>
</div></body></html>`, htmlEscape(s.Scenario), htmlEscape("v0.6.3"))
}

func renderHalf(b *strings.Builder, class, title, sub string, metrics []CounterfactualMetric, withImmortal bool) {
	fmt.Fprintf(b, `<div class="half %s"><h3>%s</h3><p class="sub">%s</p>`, class, htmlEscape(title), htmlEscape(sub))
	for _, m := range metrics {
		v := m.Without
		if withImmortal {
			v = m.With
		}
		// Compute the shown fraction for the bar.
		denom := math.Max(math.Abs(m.Without), math.Abs(m.With))
		frac := 0.0
		if denom > 0 {
			frac = math.Abs(v) / denom
		}
		fmt.Fprintf(b, `<div class="metric">
  <div class="lbl">%s</div>
  <div class="val">%s</div>
  <div class="bar"><div class="fill" style="width: %.1f%%"></div></div>
</div>`,
			htmlEscape(m.Label),
			htmlEscape(formatMetricValue(v, m.Unit)),
			frac*100,
		)
	}
	b.WriteString(`</div>`)
}

func formatMetricValue(v float64, unit string) string {
	switch unit {
	case "%":
		return fmt.Sprintf("%.1f%%", v)
	case "ms":
		return fmt.Sprintf("%.0f ms", v)
	case "/min":
		return fmt.Sprintf("+%.0f/min", v)
	default:
		return fmt.Sprintf("%.2f %s", v, unit)
	}
}

func renderCausalSVG(b *strings.Builder, nodes []CausalNode, edges []CausalEdge) {
	// Simple horizontal layout: nodes spread left-to-right in insertion order.
	width := 1040
	height := 220
	if len(nodes) > 5 {
		width = 140 + len(nodes)*160
	}
	margin := 60.0
	stride := 0.0
	if len(nodes) > 1 {
		stride = (float64(width) - 2*margin) / float64(len(nodes)-1)
	}
	pos := make(map[string][2]float64, len(nodes))
	for i, n := range nodes {
		x := margin + stride*float64(i)
		y := 110.0
		if n.Role == "action" {
			y = 180.0 // actions sit below the chain
		}
		pos[n.ID] = [2]float64{x, y}
	}

	fmt.Fprintf(b, `<svg viewBox="0 0 %d %d" xmlns="http://www.w3.org/2000/svg" style="width:100%%;height:auto;display:block">`, width, height)
	// defs: arrowhead
	b.WriteString(`<defs>
  <marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto">
    <path d="M0,0 L10,5 L0,10 z" fill="currentColor" style="color: color-mix(in oklab, var(--muted) 80%, transparent)" />
  </marker>
  <marker id="arrow-lime" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto">
    <path d="M0,0 L10,5 L0,10 z" style="fill: var(--lime)" />
  </marker>
</defs>`)

	// edges first (under the nodes)
	for _, e := range edges {
		p1, ok1 := pos[e.From]
		p2, ok2 := pos[e.To]
		if !ok1 || !ok2 {
			continue
		}
		cls := "edge"
		marker := "url(#arrow)"
		if e.Kind == "healed_by" {
			cls = "edge healed_by"
			marker = "url(#arrow-lime)"
		}
		// Curved line for visual interest.
		midX := (p1[0] + p2[0]) / 2
		midY := (p1[1] + p2[1]) / 2 - 18
		fmt.Fprintf(b, `<path class="%s" d="M %.0f %.0f Q %.0f %.0f %.0f %.0f" marker-end="%s" />`,
			cls, p1[0], p1[1], midX, midY, p2[0], p2[1], marker)
		if e.Label != "" {
			fmt.Fprintf(b, `<text class="elabel" x="%.0f" y="%.0f">%s</text>`, midX, midY-4, htmlEscape(e.Label))
		}
	}

	// nodes on top
	for _, n := range nodes {
		p := pos[n.ID]
		role := n.Role
		if role == "" {
			role = "relay"
		}
		w, h := 132.0, 54.0
		fmt.Fprintf(b, `<g transform="translate(%.0f %.0f)">`, p[0]-w/2, p[1]-h/2)
		fmt.Fprintf(b, `<rect class="node %s" width="%.0f" height="%.0f" rx="10" />`, htmlEscape(role), w, h)
		fmt.Fprintf(b, `<text class="nlabel" x="%.0f" y="22">%s</text>`, w/2, htmlEscape(n.Label))
		if n.Delta != "" {
			fmt.Fprintf(b, `<text class="nsub" x="%.0f" y="40">%s</text>`, w/2, htmlEscape(n.Delta))
		}
		b.WriteString(`</g>`)
	}

	b.WriteString(`</svg>`)
}

func htmlEscape(s string) string { return html.EscapeString(s) }
