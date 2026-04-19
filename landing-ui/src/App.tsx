import { useEffect, useState } from "react"
import {
  Bot, Network, ShieldCheck, Users, Zap, CheckCircle2, Activity, Gauge, Box,
  Github, ArrowRight, Star, Check,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Card } from "@/components/ui/card"
import {
  Accordion, AccordionContent, AccordionItem, AccordionTrigger,
} from "@/components/ui/accordion"

const FEATURES = [
  { Icon: Bot,          title: "Agentic healing",        body: "Multi-step ReAct loop — plan → act → observe → re-plan." },
  { Icon: Network,      title: "Digital twin",           body: "Simulates healing plans against a shadow state before running them." },
  { Icon: ShieldCheck,  title: "Post-quantum audit",     body: "Ed25519-signed, Merkle-rooted chain. Every action proven." },
  { Icon: Users,        title: "Federated learning",     body: "Fleet-wide anomaly baselines. Raw metrics never leave the node." },
  { Icon: Zap,          title: "Causal inference",       body: "PC + FCI + do-calculus. Finds true root causes, not correlations." },
  { Icon: CheckCircle2, title: "Formal verification",    body: "TLA+-style model checker with cryptographic certificates." },
  { Icon: Activity,     title: "OpenTelemetry native",   body: "OTLP/HTTP at :4318. Topology auto-discovers from traces." },
  { Icon: Gauge,        title: "867k events/sec",        body: "Sustained throughput on a laptop. −19% with every feature on." },
  { Icon: Box,          title: "Single binary",          body: "16 MB Go binary. No agent, no runtime deps, no external services." },
]

const FAQS = [
  {
    q: "How is this different from Kubernetes healing?",
    a: "Kubernetes restarts pods when a liveness probe fails. Immortal reasons about incidents — it predicts them before they fire, simulates the fix against a shadow of your system, and proves the action was safe with a signed certificate. Restarting pods is table stakes; the thinking part is new.",
  },
  {
    q: "Does it require a specific observability tool?",
    a: "No. It accepts OpenTelemetry OTLP/HTTP at port 4318, so anything already emitting OTel (Datadog SDK, OTel SDK, any language's OTel exporter) flows in without code changes. Native ingest via webhooks or the HTTP API also works.",
  },
  {
    q: "Can I run it on-prem or in an air-gapped environment?",
    a: "Yes. Apache-licensed single Go binary, no external service calls, SQLite for persistence. It's designed to run inside a regulated bank, a FedRAMP cloud, or a submarine.",
  },
  {
    q: "What makes the audit chain post-quantum?",
    a: "The signing algorithm is pluggable via a Signer interface. Today it ships with Ed25519 (classical); the chain structure, canonical bytes, and verifier logic already include algorithm + key-id fields so you can swap in SPHINCS+ or Dilithium without re-signing history. The KEK/DEK envelope encryption uses AES-256-GCM with AES-KeyWrap (RFC 3394).",
  },
  {
    q: "How does federated learning preserve privacy?",
    a: "Clients compute Welford statistics locally (mean, variance, count) and send ONLY those summaries to the aggregator — never raw metrics. Bonawitz pairwise masking ensures the aggregator sees the sum across clients but not any individual contribution. Laplace/Gaussian DP noise is optional with a proper ε-budget.",
  },
  {
    q: "Can I extend it with my own healing actions?",
    a: "Yes. Public pkg/plugin SDK exposes interfaces for EffectModel, HealingAction, Tool, and Invariant. Plugin authors import only the public package; internal types never leak. Registry handles name deduplication and adapter closures.",
  },
]

const GITHUB_REPO = "Nagendhra-web/Immortal"

// On GitHub Pages the dashboard does not exist (it only runs inside the local
// binary). Redirect CTAs to the install Quick Start instead of a dead link.
const IS_PAGES = (import.meta as any).env?.VITE_TARGET === "pages"
const DASHBOARD_HREF = IS_PAGES
  ? `https://github.com/${GITHUB_REPO}#quick-start`
  : "/dashboard/"
const DASHBOARD_EXTERNAL = IS_PAGES
  ? { target: "_blank", rel: "noreferrer" }
  : {}

function useGithubStars() {
  const [stars, setStars] = useState<number | null>(null)
  useEffect(() => {
    let abort = false
    fetch(`https://api.github.com/repos/${GITHUB_REPO}`)
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => {
        if (!abort && d && typeof d.stargazers_count === "number") {
          setStars(d.stargazers_count)
        }
      })
      .catch(() => {})
    return () => { abort = true }
  }, [])
  return stars
}

function fmtStars(n: number): string {
  if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, "") + "k"
  return String(n)
}

function LogLine({ tag, body, color, delay }: { tag: string; body: string; color: string; delay: number }) {
  const [visible, setVisible] = useState(false)
  useEffect(() => {
    const t = setTimeout(() => setVisible(true), delay)
    return () => clearTimeout(t)
  }, [delay])
  return (
    <div className={`flex gap-3 transition-opacity duration-300 ${visible ? "opacity-100" : "opacity-0"}`}>
      <span className={`font-semibold shrink-0 w-24 ${color}`}>{tag}</span>
      <span className="text-zinc-300">{body}</span>
    </div>
  )
}

export default function App() {
  const stars = useGithubStars()
  return (
    <div className="min-h-screen bg-background text-foreground font-sans">
      {/* Nav */}
      <nav className="sticky top-0 z-50 bg-background/80 backdrop-blur-md border-b border-border/50">
        <div className="max-w-6xl mx-auto px-6 h-16 flex items-center">
          <a href="/" className="flex items-center gap-2 font-semibold tracking-tight">
            <span className="h-7 w-7 rounded-md bg-foreground flex items-center justify-center">
              <Activity className="h-4 w-4 text-background" strokeWidth={2.5} />
            </span>
            <span>Immortal</span>
          </a>
          <div className="hidden md:flex items-center gap-8 ml-12 text-sm text-muted-foreground">
            <a href="#features" className="hover:text-foreground transition-colors">Features</a>
            <a href="#how" className="hover:text-foreground transition-colors">How it works</a>
            <a href="#benchmarks" className="hover:text-foreground transition-colors">Benchmarks</a>
            <a href="#faq" className="hover:text-foreground transition-colors">FAQ</a>
          </div>
          <div className="ml-auto flex items-center gap-3">
            <Button variant="outline" size="sm" asChild>
              <a href={`https://github.com/${GITHUB_REPO}`} target="_blank" rel="noreferrer">
                <Github className="h-4 w-4 mr-2" />
                <span className="hidden sm:inline">GitHub</span>
                {stars !== null && (
                  <>
                    <Star className="h-3 w-3 ml-2 fill-ember-500 text-ember-500" />
                    <span className="ml-1 text-xs font-medium">{fmtStars(stars)}</span>
                  </>
                )}
              </a>
            </Button>
            <Button size="sm" asChild>
              <a href={DASHBOARD_HREF} {...DASHBOARD_EXTERNAL}>
                {IS_PAGES ? "Install" : "Launch Dashboard"} <ArrowRight className="h-3.5 w-3.5 ml-1.5" />
              </a>
            </Button>
          </div>
        </div>
      </nav>

      {/* Hero */}
      <section className="max-w-6xl mx-auto px-6 pt-24 md:pt-32 pb-20">
        <div className="animate-fade-in">
          <Badge variant="outline" className="border-ember-300 bg-ember-50 text-ember-700 font-medium">
            <span className="h-1.5 w-1.5 rounded-full bg-ember-500 mr-2 animate-pulse" />
            v0.4 — predictive healing shipped
          </Badge>
          <h1 className="mt-8 font-serif font-black leading-[0.95] tracking-[-0.04em]"
              style={{ fontSize: "clamp(48px, 7vw, 112px)" }}>
            Your apps<br />never die.
          </h1>
          <p className="mt-8 text-lg md:text-xl text-muted-foreground max-w-2xl leading-relaxed">
            The open-source self-healing engine that predicts outages, simulates fixes,
            and proves every action it took. Before your pager ever goes off.
          </p>
          <div className="mt-10 flex flex-wrap gap-3">
            <Button size="lg" asChild>
              <a href={DASHBOARD_HREF} {...DASHBOARD_EXTERNAL}>
                Get started <ArrowRight className="h-4 w-4 ml-2" />
              </a>
            </Button>
            <Button size="lg" variant="outline" asChild>
              <a href="https://github.com/Nagendhra-web/Immortal" target="_blank" rel="noreferrer">
                <Github className="h-4 w-4 mr-2" /> View on GitHub
              </a>
            </Button>
          </div>
          <p className="mt-6 text-sm text-muted-foreground">
            Apache 2.0 · 78 packages · zero runtime dependencies
          </p>
        </div>
      </section>

      {/* Install snippet */}
      <section className="max-w-5xl mx-auto px-6 pb-32">
        <div className="rounded-2xl border border-border bg-zinc-950 shadow-2xl overflow-hidden animate-glow-pulse">
          <div className="flex items-center gap-2 px-4 py-3 border-b border-zinc-800 bg-zinc-900/60">
            <span className="h-3 w-3 rounded-full bg-red-500/70" />
            <span className="h-3 w-3 rounded-full bg-amber-500/70" />
            <span className="h-3 w-3 rounded-full bg-green-500/70" />
            <span className="ml-3 text-xs text-zinc-500 font-mono">~/apps/production</span>
          </div>
          <div className="p-6 md:p-8 font-mono text-sm md:text-[15px] leading-relaxed">
            <div className="text-zinc-400">$ <span className="text-zinc-100">curl -fsSL https://raw.githubusercontent.com/Nagendhra-web/Immortal/main/scripts/install.sh | bash</span></div>
            <div className="text-zinc-400 mt-1">$ <span className="text-zinc-100">immortal start --pqaudit --twin --agentic --causal --topology --formal</span></div>
            <div className="mt-6 space-y-1.5">
              <LogLine tag="[OBSERVE]" body="metric cpu=92% svc=db" color="text-cyan-400" delay={400} />
              <LogLine tag="[PREDICT]" body="breach in 4m20s, confidence 0.87" color="text-amber-400" delay={700} />
              <LogLine tag="[PLAN]"    body="failover(db) + restart(api) — expected $12,400 loss avoided" color="text-violet-400" delay={1000} />
              <LogLine tag="[VERIFY]"  body="plan proven safe, cert ABC123..." color="text-orange-400" delay={1300} />
              <LogLine tag="[HEAL]"    body="executed in 842ms" color="text-green-400" delay={1600} />
              <LogLine tag="[PROVE]"   body="audit entry signed, chain verified" color="text-emerald-400" delay={1900} />
            </div>
            <div className="mt-6 pt-6 border-t border-zinc-800 text-zinc-300 flex items-center gap-2">
              <Check className="h-4 w-4 text-green-500" />
              <span>Incident resolved before it happened.</span>
            </div>
          </div>
        </div>
      </section>

      {/* Features */}
      <section id="features" className="max-w-6xl mx-auto px-6 py-24 md:py-32">
        <div className="max-w-2xl mb-16">
          <p className="text-sm font-medium text-ember-600 uppercase tracking-wider mb-3">Capabilities</p>
          <h2 className="font-serif font-bold text-4xl md:text-5xl tracking-[-0.02em] leading-[1.05]">
            Nothing else in the space<br />comes close.
          </h2>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {FEATURES.map(({ Icon, title, body }) => (
            <Card key={title} className="p-6 border-border/60 hover:border-foreground/20 transition-colors">
              <Icon className="h-5 w-5 text-ember-600 mb-4" strokeWidth={1.75} />
              <h3 className="font-semibold text-base mb-1.5">{title}</h3>
              <p className="text-sm text-muted-foreground leading-relaxed">{body}</p>
            </Card>
          ))}
        </div>
      </section>

      {/* How it works */}
      <section id="how" className="max-w-6xl mx-auto px-6 py-24 md:py-32 border-t border-border/60">
        <div className="max-w-2xl mb-16">
          <p className="text-sm font-medium text-ember-600 uppercase tracking-wider mb-3">The loop</p>
          <h2 className="font-serif font-bold text-4xl md:text-5xl tracking-[-0.02em] leading-[1.05]">
            See. Reason. Prove.
          </h2>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
          {[
            { n: "01", title: "Observe", body: "OpenTelemetry ingestion. Metric, log, and trace normalization. Predictive twin observes every signal." },
            { n: "02", title: "Reason",  body: "Agentic loop plans multi-step healing. Causal inference finds true root causes. Economic planner picks the cheapest safe plan." },
            { n: "03", title: "Prove",   body: "Formal model-checker verifies invariants before execution. Post-quantum audit chain signs every action. Cryptographic certificate for auditors." },
          ].map(({ n, title, body }) => (
            <div key={n}>
              <div className="font-mono text-sm text-ember-600 mb-3">{n}</div>
              <h3 className="font-serif font-bold text-2xl mb-3 tracking-[-0.02em]">{title}</h3>
              <p className="text-muted-foreground leading-relaxed">{body}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Dashboard preview */}
      <section className="max-w-6xl mx-auto px-6 py-24 md:py-32 border-t border-border/60">
        <div className="max-w-2xl mb-12">
          <p className="text-sm font-medium text-ember-600 uppercase tracking-wider mb-3">The dashboard</p>
          <h2 className="font-serif font-bold text-4xl md:text-5xl tracking-[-0.02em] leading-[1.05]">
            See everything at a glance.
          </h2>
        </div>
        <Card className="border-border/60 overflow-hidden p-0">
          <div className="border-b border-border flex items-center gap-2 px-4 py-3 bg-muted/40">
            <span className="h-3 w-3 rounded-full bg-red-400/70" />
            <span className="h-3 w-3 rounded-full bg-amber-400/70" />
            <span className="h-3 w-3 rounded-full bg-green-400/70" />
            <div className="ml-4 flex-1 bg-background border border-border rounded-md px-3 py-1 text-xs text-muted-foreground font-mono max-w-lg">
              immortal.local/dashboard
            </div>
          </div>
          <div className="grid grid-cols-[200px_1fr] h-[420px]">
            <div className="border-r border-border bg-muted/30 p-4 text-xs">
              <div className="font-semibold mb-3">Immortal</div>
              <div className="space-y-1 text-muted-foreground">
                <div className="text-[10px] uppercase tracking-wider mt-3 mb-1">Monitor</div>
                <div className="text-foreground bg-secondary rounded px-2 py-1">Overview</div>
                <div className="px-2 py-1">Topology</div>
                <div className="px-2 py-1">Audit</div>
                <div className="text-[10px] uppercase tracking-wider mt-3 mb-1">Reason</div>
                <div className="px-2 py-1">Forecast</div>
                <div className="px-2 py-1">Agent</div>
                <div className="px-2 py-1">Verify</div>
              </div>
            </div>
            <div className="p-6">
              <div className="text-lg font-semibold mb-4">Overview</div>
              <div className="grid grid-cols-4 gap-3 mb-4">
                {[
                  ["Uptime", "47d 12h"],
                  ["Events", "2.1M"],
                  ["Heals", "84"],
                  ["Services", "12/12"],
                ].map(([k, v]) => (
                  <div key={k} className="border border-border rounded-lg p-3">
                    <div className="text-[10px] text-muted-foreground uppercase">{k}</div>
                    <div className="font-mono text-lg font-semibold mt-1">{v}</div>
                  </div>
                ))}
              </div>
              <div className="border border-border rounded-lg p-4 h-44">
                <div className="text-xs font-semibold mb-3">Live feed</div>
                {[
                  ["14:32:04", "heal",    "db · restarted",           "text-ember-600"],
                  ["14:31:58", "predict", "cpu breach in 4m",         "text-amber-600"],
                  ["14:30:12", "verify",  "chain ok · 1,284 entries", "text-emerald-600"],
                ].map(([t, kind, msg, color], i) => (
                  <div key={i} className="flex gap-3 text-xs py-1 font-mono">
                    <span className="text-muted-foreground">{t}</span>
                    <span className={`font-semibold w-20 ${color}`}>{kind}</span>
                    <span>{msg}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </Card>
        <div className="mt-8 flex justify-center">
          <Button size="lg" asChild>
            <a href={DASHBOARD_HREF} {...DASHBOARD_EXTERNAL}>
              {IS_PAGES ? "Install to try it" : "Try the dashboard"} <ArrowRight className="h-4 w-4 ml-2" />
            </a>
          </Button>
        </div>
      </section>

      {/* Benchmarks */}
      <section id="benchmarks" className="max-w-4xl mx-auto px-6 py-24 md:py-32 border-t border-border/60">
        <div className="mb-12">
          <p className="text-sm font-medium text-ember-600 uppercase tracking-wider mb-3">Benchmarks</p>
          <h2 className="font-serif font-bold text-4xl md:text-5xl tracking-[-0.02em] leading-[1.05] mb-4">
            Built to survive production.
          </h2>
          <p className="text-muted-foreground">Real numbers from `go test -bench` on an i7-11370H.</p>
        </div>
        <Card className="border-border/60 overflow-hidden">
          <table className="w-full">
            <tbody>
              {[
                ["Sustained ingest throughput",    "867,000 events / sec"],
                ["With every advanced feature on", "706,000 events / sec   (−19%)"],
                ["Publish-path latency (p99)",     "< 1 µs"],
                ["Heap delta at 100k events",      "0.19 MB (after GC)"],
                ["Total binary size",              "16 MB"],
                ["External runtime dependencies",  "0"],
              ].map(([metric, value], i) => (
                <tr key={i} className={i > 0 ? "border-t border-border" : ""}>
                  <td className="px-6 py-4 text-muted-foreground">{metric}</td>
                  <td className="px-6 py-4 font-mono font-semibold text-right">{value}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      </section>

      {/* Pricing / OSS */}
      <section className="max-w-4xl mx-auto px-6 py-24 md:py-32 border-t border-border/60">
        <Card className="border-ember-200 bg-ember-50/40 p-12 md:p-16 text-center">
          <Badge className="bg-ember-500 hover:bg-ember-600 text-white border-transparent mb-6">Apache 2.0</Badge>
          <h2 className="font-serif font-bold text-4xl md:text-6xl tracking-[-0.03em] leading-[1] mb-6">
            Free. Open source.<br />Forever.
          </h2>
          <p className="text-lg text-muted-foreground max-w-xl mx-auto mb-8">
            No seats. No per-host billing. No phone-home. If you run it, it's yours.
          </p>
          <div className="flex flex-wrap justify-center gap-3">
            <Button size="lg" asChild>
              <a href="https://github.com/Nagendhra-web/Immortal" target="_blank" rel="noreferrer">
                <Github className="h-4 w-4 mr-2" /> Star on GitHub
              </a>
            </Button>
            <Button size="lg" variant="outline" asChild>
              <a href="mailto:hello@immortal.dev">Enterprise support</a>
            </Button>
          </div>
        </Card>
      </section>

      {/* FAQ */}
      <section id="faq" className="max-w-3xl mx-auto px-6 py-24 md:py-32 border-t border-border/60">
        <div className="mb-12">
          <p className="text-sm font-medium text-ember-600 uppercase tracking-wider mb-3">Questions</p>
          <h2 className="font-serif font-bold text-4xl md:text-5xl tracking-[-0.02em] leading-[1.05]">
            Asked honestly.
          </h2>
        </div>
        <Accordion type="single" collapsible className="w-full">
          {FAQS.map(({ q, a }, i) => (
            <AccordionItem key={i} value={`q-${i}`} className="border-border/60">
              <AccordionTrigger className="text-left font-semibold py-5 hover:no-underline">
                {q}
              </AccordionTrigger>
              <AccordionContent className="text-muted-foreground leading-relaxed pb-6">
                {a}
              </AccordionContent>
            </AccordionItem>
          ))}
        </Accordion>
      </section>

      {/* Footer */}
      <footer className="border-t border-border/60 mt-24 bg-muted/30">
        <div className="max-w-6xl mx-auto px-6 py-16">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-8 mb-12 text-sm">
            {[
              { h: "Product",   items: ["Features", "Dashboard", "Docs", "Changelog"] },
              { h: "Company",   items: ["About", "Blog", "Careers", "Contact"] },
              { h: "Resources", items: ["GitHub", "Discord", "Twitter", "Status"] },
              { h: "Legal",     items: ["Apache 2.0", "Security", "Privacy"] },
            ].map(({ h, items }) => (
              <div key={h}>
                <div className="font-semibold mb-3">{h}</div>
                <ul className="space-y-2 text-muted-foreground">
                  {items.map((it) => (
                    <li key={it}><a href="#" className="hover:text-foreground transition-colors">{it}</a></li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
          <div className="flex flex-wrap items-center justify-between gap-4 pt-8 border-t border-border/60 text-sm text-muted-foreground">
            <div className="flex items-center gap-2">
              <span className="h-6 w-6 rounded bg-foreground flex items-center justify-center">
                <Activity className="h-3 w-3 text-background" strokeWidth={2.5} />
              </span>
              <span>© 2026 Immortal Engine</span>
            </div>
            <div className="font-serif italic">Your apps never die.</div>
          </div>
        </div>
      </footer>
    </div>
  )
}
