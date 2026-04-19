import { useQuery } from "@tanstack/react-query"
import { useEffect, useRef } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { api } from "@/lib/api"

interface Node { id: string; x: number; y: number; vx: number; vy: number }
interface Edge { source: string; target: string }

export default function Topology() {
  const { data } = useQuery({ queryKey: ["topology"], queryFn: api.topologySnapshot })
  const svgRef = useRef<SVGSVGElement>(null)

  useEffect(() => {
    if (!svgRef.current) return
    const svg = svgRef.current
    const W = svg.clientWidth || 800
    const H = svg.clientHeight || 500

    const rawNodes: any[] = data?.nodes ?? data?.vertices ?? []
    const rawEdges: any[] = data?.edges ?? data?.links ?? []

    if (!rawNodes.length) return

    const nodes: Node[] = rawNodes.map((n: any, i: number) => ({
      id: n.id ?? n.name ?? String(i),
      x: W * 0.1 + Math.random() * W * 0.8,
      y: H * 0.1 + Math.random() * H * 0.8,
      vx: 0, vy: 0,
    }))
    const edges: Edge[] = rawEdges.map((e: any) => ({ source: e.source ?? e.from, target: e.target ?? e.to }))

    const g = document.createElementNS("http://www.w3.org/2000/svg", "g")
    svg.innerHTML = ""
    svg.appendChild(g)

    function tick() {
      // simple repulsion + spring
      for (let i = 0; i < nodes.length; i++) {
        for (let j = i + 1; j < nodes.length; j++) {
          const dx = nodes[j].x - nodes[i].x
          const dy = nodes[j].y - nodes[i].y
          const d = Math.sqrt(dx * dx + dy * dy) || 1
          const f = 3000 / (d * d)
          nodes[i].vx -= f * dx / d; nodes[i].vy -= f * dy / d
          nodes[j].vx += f * dx / d; nodes[j].vy += f * dy / d
        }
      }
      for (const e of edges) {
        const s = nodes.find(n => n.id === e.source)
        const t = nodes.find(n => n.id === e.target)
        if (!s || !t) continue
        const dx = t.x - s.x; const dy = t.y - s.y
        const d = Math.sqrt(dx * dx + dy * dy) || 1
        const f = (d - 120) * 0.05
        s.vx += f * dx / d; s.vy += f * dy / d
        t.vx -= f * dx / d; t.vy -= f * dy / d
      }
      for (const n of nodes) {
        n.vx *= 0.85; n.vy *= 0.85
        n.x = Math.max(30, Math.min(W - 30, n.x + n.vx))
        n.y = Math.max(30, Math.min(H - 30, n.y + n.vy))
      }
      render()
    }

    function render() {
      g.innerHTML = ""
      for (const e of edges) {
        const s = nodes.find(n => n.id === e.source)
        const t = nodes.find(n => n.id === e.target)
        if (!s || !t) continue
        const line = document.createElementNS("http://www.w3.org/2000/svg", "line")
        line.setAttribute("x1", String(s.x)); line.setAttribute("y1", String(s.y))
        line.setAttribute("x2", String(t.x)); line.setAttribute("y2", String(t.y))
        line.setAttribute("stroke", "#94a3b8"); line.setAttribute("stroke-width", "1.5")
        g.appendChild(line)
      }
      for (const n of nodes) {
        const circle = document.createElementNS("http://www.w3.org/2000/svg", "circle")
        circle.setAttribute("cx", String(n.x)); circle.setAttribute("cy", String(n.y))
        circle.setAttribute("r", "14"); circle.setAttribute("fill", "#6366f1")
        circle.setAttribute("stroke", "#fff"); circle.setAttribute("stroke-width", "2")
        g.appendChild(circle)
        const text = document.createElementNS("http://www.w3.org/2000/svg", "text")
        text.setAttribute("x", String(n.x)); text.setAttribute("y", String(n.y + 26))
        text.setAttribute("text-anchor", "middle"); text.setAttribute("font-size", "11")
        text.setAttribute("fill", "#475569")
        text.textContent = n.id.slice(0, 12)
        g.appendChild(text)
      }
    }

    let frame = 0
    const interval = setInterval(() => { tick(); if (++frame > 120) clearInterval(interval) }, 16)
    return () => clearInterval(interval)
  }, [data])

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">Topology</h1>
      <Card>
        <CardHeader><CardTitle>Service Graph</CardTitle></CardHeader>
        <CardContent>
          {!data && <p className="text-muted-foreground text-sm">Loading topology…</p>}
          <svg ref={svgRef} className="w-full" style={{ height: 500, border: "1px solid hsl(var(--border))", borderRadius: 8 }} />
        </CardContent>
      </Card>
    </div>
  )
}
