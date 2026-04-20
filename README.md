<div align="center">

# Immortal

### The self-healing engine for modern infrastructure.

Detect failures in milliseconds. Heal them autonomously. Prove every action with a signed, tamper-evident audit trail.

**79 Go packages · 12-view operator dashboard · 3 SDKs · Single binary · Apache 2.0**

[![CI](https://github.com/Nagendhra-web/Immortal/actions/workflows/ci.yml/badge.svg)](https://github.com/Nagendhra-web/Immortal/actions)
[![Pages](https://github.com/Nagendhra-web/Immortal/actions/workflows/pages.yml/badge.svg)](https://nagendhra-web.github.io/Immortal/)
[![Release](https://img.shields.io/github/v/release/Nagendhra-web/Immortal?color=00c48c)](https://github.com/Nagendhra-web/Immortal/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/Nagendhra-web/Immortal)](https://goreportcard.com/report/github.com/Nagendhra-web/Immortal)
[![Go Reference](https://pkg.go.dev/badge/github.com/Nagendhra-web/Immortal.svg)](https://pkg.go.dev/github.com/Nagendhra-web/Immortal)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![SLSA 3](https://slsa.dev/images/gh-badge-level3.svg)](SECURITY.md#verifying-a-release)
[![Stars](https://img.shields.io/github/stars/Nagendhra-web/Immortal?style=social)](https://github.com/Nagendhra-web/Immortal/stargazers)

<p>
<a href="#quick-start">Install</a> ·
<a href="#what-it-actually-does">Features</a> ·
<a href="#the-operator-dashboard">Dashboard</a> ·
<a href="#sdks">SDKs</a> ·
<a href="https://nagendhra-web.github.io/Immortal/">Landing</a> ·
<a href="docs/INSTALL.md">Docs</a> ·
<a href="CHANGELOG.md">Changelog</a> ·
<a href="https://github.com/Nagendhra-web/Immortal/issues">Issues</a>
</p>

</div>

---

```
3:00 AM.  Prod goes down.
  Traditional stack:   PagerDuty wakes you. You SSH in. You restart. 45 minutes of downtime.
  Immortal:            Detects in 200 ms. Heals automatically. Writes a signed receipt.
                       You sleep through it.
```

---

## Why Immortal

Every other layer of your stack has been automated. Deploys, tests, infra, security scans. **Recovery is still manual.** A human is still in the loop at 3 AM, reading stack traces, guessing at root causes.

Immortal closes that loop. It is the first open-source engine that combines:

- **Reactive healing**: rule-based responses to known failure signatures.
- **Predictive healing**: linear regression and anomaly detection catch breaches before they happen.
- **Agentic healing**: ReAct-style Plan > Act > Observe > Re-plan loops for the novel failures your rules miss.
- **Formal verification**: LTL/CTL properties checked against the system model, with counterexamples when they fail.
- **Causal inference**: PCMCI root-cause analysis that separates true causes from spurious correlations.
- **Post-quantum audit trail**: every action is hash-chained, Merkle-rooted, and signed. Tamper-evident end-to-end.
- **Digital twin**: every healing plan is simulated against a shadow state machine before it touches prod.

All of it ships as a **single Go binary** with an embedded vanilla-JS operator dashboard. Zero external dependencies. Run it on a Raspberry Pi or a 128-core bare-metal node.

---

## Quick Start

```bash
# macOS / Linux: one-liner installer (downloads release binary, falls back to go install)
curl -fsSL https://raw.githubusercontent.com/Nagendhra-web/Immortal/main/scripts/install.sh | bash

# Windows (PowerShell)
irm https://raw.githubusercontent.com/Nagendhra-web/Immortal/main/scripts/install.ps1 | iex

# Go toolchain (Go 1.25+)
go install github.com/Nagendhra-web/Immortal/cmd/immortal@latest

# Homebrew (macOS / Linux)
brew tap Nagendhra-web/immortal && brew install immortal

# Docker
docker run -p 7777:7777 ghcr.io/nagendhra-web/immortal:latest
```

```bash
# Start healing.
immortal start

# With every advanced feature on (twin simulation, agentic reasoning, causal RCA, formal checks).
immortal start --pqaudit --twin --agentic --causal --topology --formal

# Ghost mode: observe first, act later. Produces recommendations without side effects.
immortal start --ghost

# Watch specific targets.
immortal start --watch-url https://myapp.com --watch-process nginx --watch-log /var/log/app.log
```

Then open **http://127.0.0.1:7777/dashboard/** for the operator console and **http://127.0.0.1:7777/** for the landing page.

Full install options (pre-built binaries, Docker, source): [docs/INSTALL.md](docs/INSTALL.md).

---

## The Operator Dashboard

A zero-dependency, vanilla HTML/CSS/JS console embedded directly into the binary. **12 views**, command palette (Cmd/Ctrl+K), real SVG charts, SSE-backed live updates, full keyboard navigation.

**Mission Control**
- `/overview` : KPI strip, latency + error rate, active incidents, recent heals.
- `/topology` : force-directed service graph with health, blast-radius, drill-downs.
- `/audit` : Merkle-anchored log with filter DSL, one-click chain verification.
- `/terminal` : live log tail with severity filter.

**Intelligence**
- `/twin` : predictive forecasts with 90% bands, per-service drift + calibration.
- `/agentic` : ReAct trace viewer (thought / action / observation timeline, replayable).
- `/causal` : PCMCI DAG with ranked root causes and causal-vs-correlation toggle.
- `/formal` : model-check results with step-by-step counterexample viewer.

**Authoring**
- `/planner/nl` : natural-language goal compiles to a dependency graph of operations, each with rationale, pre/post conditions, and rollback.
- `/planner/economic` : Pareto frontier optimizer (cost vs. p99 latency) with per-service allocation detail.

**Knowledge**
- `/federation` : federated knowledge graph across peers with cryptographic provenance.
- `/certificates` : post-quantum signed attestations with verify-now and key rotation.

Built on a three-tier OKLCH design token system. Works from 320 px mobile up to 4K. Accessible by default (keyboard nav, ARIA tablists, focus rings, reduced-motion support).

---

## What It Actually Does

| Capability | Package | Summary |
|---|---|---|
| **Self-healing** | `engine` | Detect failures, match rules, execute fixes automatically. |
| **Anomaly detection** | `dna` | Learns normal baselines, flags 3-sigma deviations. |
| **Root cause analysis** | `causality` | Traces cascading failures (disk full -> DB crash -> API timeout). |
| **Predictive healing** | `predict` | Linear regression on metrics. Warns before thresholds breach. |
| **Pattern detection** | `pattern` | Recurring failures with sliding time windows. |
| **SLA tracking** | `sla` | Uptime percentage, violation alerts, worst-service ranking. |
| **Ghost mode** | `engine` | Observe-only mode. Recommends without acting. |
| **Time-travel** | `timetravel` | Replay events before a crash to understand what happened. |
| **Audit trail** | `audit` | Immutable log of every action with search and filter. |
| **Dependency graph** | `dependency` | Map service dependencies, analyze blast radius. |
| **Webhook alerts** | `webhook` | HMAC-signed HTTP notifications with retries. |
| **Event throttling** | `throttle` | Blocks event floods (tested: 999 of 1000 duplicates dropped). |
| **WAF** | `security/firewall` | SQLi, XSS, path traversal, command injection. |
| **RASP** | `security/rasp` | Runtime protection against dangerous operations. |
| **Rate limiting** | `security/ratelimit` | Per-IP request throttling. |
| **Anti-scrape** | `security/antiscrape` | Bot and scraper detection. |
| **Secret scanning** | `security/secrets` | Find leaked API keys, tokens, passwords. |
| **Zero-trust auth** | `security/zerotrust` | Service-to-service auth with expiring tokens. |
| **Circuit breaker** | `circuitbreaker` | Stop hammering failing services. |
| **Prometheus export** | `export` | Metrics in Prometheus text format. |
| **Notifications** | `notify` | Slack, Discord, console alerts. |
| **Chaos testing** | `chaos` | Inject failures, verify healing works, score effectiveness. |
| **Self-learning** | `autolearn` | Watches successful heals, suggests new rules automatically. |
| **Incident reports** | `incident` | Auto-generated postmortems (timeline, root cause, markdown export). |
| **Capacity forecast** | `capacity` | Multi-metric forecasting, exhaustion date prediction. |
| **Metric correlation** | `correlation` | Pearson correlation. Discovers leading indicators across metrics. |
| **Healing playbooks** | `playbook` | Multi-step healing with conditions, retries, auto-rollback. |
| **Agentic healing loop** | `agentic` | ReAct-style Plan > Act > Observe > Re-plan until resolved. |
| **Post-quantum audit** | `pqaudit` | Hash-chained signed audit ledger with Merkle root. Tamper-evident. |
| **Digital twin** | `twin` | Simulates healing plans against a shadow state machine. Rejects plans that worsen predicted score. |
| **Federated learning** | `federated` | FedAvg across a fleet with robust trim plus DP noise. Raw metrics stay local. |
| **Causal inference** | `causal` | PC-algorithm discovery plus do-calculus ACE. Identifies true root causes, not spurious correlations. |
| **Formal verification** | `formal` | LTL/CTL model checking with counterexample generation. |

---

## Proven in Tests

Every scenario below runs in CI on every commit:

```
PASS  TestScenario_APIReturns500_ImmortalHeals            server broke, Immortal healed it
PASS  TestScenario_LogErrors_ImmortalDetects              caught ERROR + FATAL from log tailing
PASS  TestScenario_CPUAnomaly_DNADetects                  flagged 95% CPU as anomaly (baseline 44.5%)
PASS  TestScenario_GhostMode_ObservesOnly                 observed without acting, produced recommendations
PASS  TestScenario_CascadingFailure_CausalityTracked      traced: disk full -> DB crash -> API timeout
PASS  TestScenario_EventFlood_ThrottlePrevents            blocked 999/1000 duplicate events
PASS  TestScenario_TimeTravel_ReplayBeforeFailure         replayed 5 events before crash
PASS  TestScenario_RESTAPI_QueryWhileRunning              queried health, metrics, Prometheus while live
PASS  TestRealWorld_DatabaseDown_AgentRestartsAndVerifies agentic loop: check -> restart -> verify -> done
PASS  TestRealWorld_AttackerTriesToHideAction             pqaudit Merkle root exposes attempted field mutation
PASS  TestRealWorld_CascadeFailure_TwinRejectsBadPlan     twin accepts failover+restart, rejects scale-to-1
PASS  TestRealWorld_FleetLearnsCPUBaseline                federated FedAvg resists malicious outlier via trim
PASS  TestRealWorld_CausalRootCauseBeatsCorrelation       r=0.83 red herring demoted below true causes

79 packages · 0 failures
```

---

## How It Compares

| | Immortal | Traditional APM | Kubernetes self-heal | Pure alerting |
|---|:---:|:---:|:---:|:---:|
| Detects failures | yes | yes | limited | yes |
| Heals automatically | yes | no | pod-level only | no |
| Predicts failures | yes | partial | no | no |
| Agentic reasoning for novel failures | yes | no | no | no |
| Formal verification of invariants | yes | no | no | no |
| Causal root-cause (not just correlation) | yes | no | no | no |
| Signed, post-quantum audit trail | yes | no | no | no |
| Digital twin simulation before apply | yes | no | no | no |
| Single binary, zero deps | yes | no (agents, collectors) | no | no |
| Works offline / air-gapped | yes | no | yes | partial |
| Open source, Apache 2.0 | yes | no (mostly) | yes | varies |

---

## CLI

```bash
immortal status       # Engine status, uptime, event count
immortal health       # Detailed service health per service
immortal logs -f      # Live event stream (tail -f style)
immortal sla          # SLA report, uptime percentage, violations
immortal predict      # Failure predictions with confidence
immortal patterns     # Recurring failure detection
immortal audit        # Full audit trail with search
immortal history      # Healing action history
immortal recommend    # Ghost mode recommendations
immortal metrics      # Prometheus metrics
immortal deps         # Service dependency graph plus blast radius
immortal causality    # Causality graph plus root cause tracing
immortal timetravel   # Replay events before a crash
```

---

## REST API

31 endpoints covering health, metrics, events, healing, audit, predictions, incidents, playbooks, and the full v4/v5 advanced surface (twin, agentic, causal, federated, formal, topology). Default port 7777.

<details>
<summary><b>Full endpoint list</b></summary>

| Endpoint | Description |
|---|---|
| `GET /api/status` | Engine status, uptime, event and heal counts |
| `GET /api/health` | Service health registry |
| `GET /api/events?type=&source=` | Stored events with filters |
| `GET /api/healing/history` | Healing action history |
| `GET /api/recommendations` | Ghost mode recommendations |
| `GET /api/metrics` | Prometheus metrics |
| `GET /api/monitor` | Self-monitoring (goroutines, uptime) |
| `GET /api/dna/baseline` | Learned metric baselines |
| `GET /api/dna/health-score` | Health score (0.0 to 1.0) |
| `GET /api/dna/anomaly?metric=&value=` | Anomaly check |
| `GET /api/patterns` | Recurring failure patterns |
| `GET /api/predictions` | Failure predictions |
| `GET /api/sla` | SLA report per service |
| `GET /api/audit` | Audit trail with search |
| `GET /api/dependencies` | Dependency graph plus critical path |
| `GET /api/dependencies/impact?service=` | Blast radius analysis |
| `GET /api/causality/graph` | Causality graph |
| `GET /api/causality/root-cause?event_id=` | Root cause chain |
| `GET /api/timetravel` | Event replay |
| `GET /api/logs/stream` | Live SSE event stream |
| `GET /api/logs/history` | Recent log entries |
| `GET /api/chaos/report` | Chaos test results and healing score |
| `GET /api/autolearn/rules` | Self-learned healing rules |
| `GET /api/autolearn/stats` | Learning statistics |
| `GET /api/incidents` | All incident reports |
| `GET /api/incidents/active` | Open incidents |
| `GET /api/capacity` | Capacity forecasts |
| `GET /api/capacity/critical?days=7` | Resources exhausting within N days |
| `GET /api/correlations?metric=X` | Cross-metric correlations |
| `GET /api/playbooks` | Registered healing playbooks |
| `GET /api/playbooks/history` | Playbook execution history |
| `GET /api/v4/audit/verify` | Verify PQ audit chain |
| `GET /api/v4/audit/merkle-root` | Current Merkle root |
| `GET /api/v4/audit/entries` | Audit entries |
| `GET /api/v4/twin/simulate` | Simulate a plan against the twin |
| `GET /api/v4/twin/states` | Twin state snapshots |
| `GET /api/v4/agentic/run` | Run an agentic investigation |
| `GET /api/v4/causal/root-cause` | Causal root-cause |
| `GET /api/v4/federated/snapshot` | Federated snapshot |
| `GET /api/v5/topology/snapshot` | Topology snapshot |
| `GET /api/v5/topology/events` | Topology change stream |
| `GET /api/v5/formal/check` | Formal property check |
| `GET /api/v5/causal/pcmci` | PCMCI causal discovery |
| `GET /api/v5/causal/counterfactual` | Counterfactual reasoning |
| `GET /api/v5/agentic/memory/recall` | Agentic memory recall |
| `GET /api/v5/agentic/meta-investigate` | Meta-investigation |
| `GET /api/v5/federated/close` | Federated close |

</details>

---

## SDKs

<details>
<summary><b>TypeScript</b></summary>

```typescript
import { Immortal } from '@immortal-engine/sdk';

const app = new Immortal({ name: 'my-api' });

app.heal({
  name: 'restart-on-crash',
  when: (e) => e.severity === 'critical',
  do: async (e) => { console.log('Healing:', e.message); },
});

app.start();
```
</details>

<details>
<summary><b>Python</b></summary>

```python
from immortal import Immortal, Severity

app = Immortal(name="my-api")

@app.healer("crash-recovery")
def handle_crash(event):
    print(f"Healing: {event.message}")

app.start()
```
</details>

<details>
<summary><b>Go</b></summary>

```go
eng, _ := engine.New(engine.Config{DataDir: "/tmp/immortal"})

eng.AddRule(healing.Rule{
    Name:   "restart-on-crash",
    Match:  healing.MatchSeverity(event.SeverityCritical),
    Action: healing.ActionExec("systemctl restart my-service"),
})

eng.Start()
```
</details>

---

## Architecture

```
+-----------------------------------------------------+
|                   AI BRAIN LAYER                    |
|   Auto-Learning / Metric Correlation / Prediction   |
|   Built-in ML + optional LLM + plugin interface     |
+-----------------------------------------------------+
|               INTELLIGENCE LAYER                    |
|   DNA Anomaly / Pattern Recognition / Capacity      |
|   Causality Graph / Dependency / Time-Travel        |
|   Digital Twin / Causal Inference / Federated       |
+-----------------------------------------------------+
|             HEALING ORCHESTRATOR                    |
|   Reactive / Predictive / Agentic (ReAct)           |
|   Multi-step Playbooks / Auto-rollback / Consensus  |
|   Ghost Mode / Incident Reports / SLA Tracking      |
+-----------------------------------------------------+
|          VERIFICATION + PROVENANCE                  |
|   Formal Model Checking (LTL/CTL)                   |
|   Post-Quantum Signed Audit Chain + Merkle Root     |
|   Chaos Injection / Healing Score                   |
+-----------------------------------------------------+
|              EXECUTION LAYER                        |
|   SDK (embed) / Agent (sidecar) / CLI / REST API    |
|   Embedded operator dashboard (vanilla HTML/CSS/JS) |
+-----------------------------------------------------+
|           UNIVERSAL CONNECTOR MESH                  |
|   Any language, any cloud, any database             |
|   Webhooks / Slack / Discord / Prometheus           |
+-----------------------------------------------------+
```

---

## Use Individual Packages

Every package works standalone. Import only what you need:

```bash
go get github.com/Nagendhra-web/Immortal
```

<details>
<summary><b>Anomaly Detection (DNA)</b></summary>

```go
import "github.com/Nagendhra-web/Immortal/internal/dna"

d := dna.New("api-server")

// Feed normal metrics. It learns automatically.
for i := 0; i < 100; i++ {
    d.Record("response_time_ms", 120.0 + rand.Float64()*30)
}

d.IsAnomaly("response_time_ms", 500.0) // true, far above normal
d.IsAnomaly("response_time_ms", 125.0) // false, within normal

score := d.HealthScore(map[string]float64{"response_time_ms": 500.0})
// score about 0.15, something is very wrong
```
</details>

<details>
<summary><b>Predictive Healing</b></summary>

```go
import "github.com/Nagendhra-web/Immortal/internal/predict"

pred := predict.New()
pred.SetThreshold("cpu_percent", 90.0)

pred.Feed("cpu_percent", 45.0)
pred.Feed("cpu_percent", 52.0)
pred.Feed("cpu_percent", 65.0)

p := pred.Predict("cpu_percent")
fmt.Printf("Breach in: %s (confidence: %.0f%%)\n", p.TimeToThreshold, p.Confidence*100)
```
</details>

<details>
<summary><b>Digital Twin (plan validation)</b></summary>

```go
import "github.com/Nagendhra-web/Immortal/internal/twin"

t := twin.New()
t.SnapshotState(currentMetrics)

plan := twin.Plan{Actions: []string{"failover:db", "restart:api"}}
if t.Simulate(plan).ImprovesHealth() {
    engine.ApplyPlan(plan)   // accepted
} else {
    fmt.Println("twin rejected the plan")
}
```
</details>

<details>
<summary><b>Causal Inference (PC + ACE)</b></summary>

```go
import "github.com/Nagendhra-web/Immortal/internal/causal"

c := causal.New()
c.Record("cpu", 0.8);     c.Record("latency", 240)
c.Record("cpu", 0.2);     c.Record("latency", 80)
// ... time series keep coming ...

graph := c.DiscoverPC()              // PC-algorithm DAG
ace   := c.AverageCausalEffect("cpu", "latency")
fmt.Printf("ACE(cpu -> latency) = %.3f\n", ace)
```
</details>

<details>
<summary><b>Post-Quantum Audit Chain</b></summary>

```go
import "github.com/Nagendhra-web/Immortal/internal/pqaudit"

chain := pqaudit.New()
chain.Append(pqaudit.Entry{Kind: "heal", Service: "payments", Result: "ok"})
root := chain.MerkleRoot()
ok   := chain.Verify()   // false if anyone has mutated a historical entry
```
</details>

<details>
<summary><b>Federated Anomaly Learning</b></summary>

```go
import "github.com/Nagendhra-web/Immortal/internal/federated"

fl := federated.New(federated.Config{DP: true, Trim: 0.2})
fl.AddPeer("node-a", localModelA)
fl.AddPeer("node-b", localModelB)
global := fl.FedAvg()   // robust trimmed mean + DP noise, raw data never leaves a node
```
</details>

<details>
<summary><b>SLA Tracking</b></summary>

```go
import "github.com/Nagendhra-web/Immortal/internal/sla"

tracker := sla.New()
tracker.SetTarget("api-server", 99.9)
tracker.RecordStatus("api-server", true)
tracker.RecordStatus("api-server", false)

tracker.Uptime("api-server")
tracker.IsViolating("api-server")
```
</details>

<details>
<summary><b>Dependency Graph + Blast Radius</b></summary>

```go
import "github.com/Nagendhra-web/Immortal/internal/dependency"

g := dependency.New()
g.AddDependency("api", "database")
g.AddDependency("worker", "database")

g.TransitiveDependents("database") // ["api", "worker"]
g.ImpactOf("database")             // 2 services affected
g.CriticalPath()                   // sorted by impact
```
</details>

<details>
<summary><b>Chaos Testing</b></summary>

```go
import "github.com/Nagendhra-web/Immortal/internal/chaos"

ch := chaos.New(engine.Ingest)
ch.InjectHTTPError("api-server", 500)
ch.InjectProcessCrash("nginx")
ch.InjectCPUSpike(95.0)

fmt.Printf("Healing score: %.0f%%\n", ch.Score()*100)
```
</details>

<details>
<summary><b>Healing Playbooks</b></summary>

```go
import "github.com/Nagendhra-web/Immortal/internal/playbook"

runner := playbook.New()
runner.Register("deploy-recovery", []playbook.Step{
    {Name: "backup-db", Action: backupDB, Rollback: restoreDB},
    {Name: "run-migration", Action: migrate, Retries: 3},
    {Name: "restart-service", Action: restart, Condition: func() bool { return isServiceDown() }},
})

exec, err := runner.Run("deploy-recovery") // auto-rollback on failure
```
</details>

<details>
<summary><b>More: WAF, RASP, Rate Limiter, Secret Scanner, Webhook, Audit, Pattern, Correlation, Capacity, Incident, Zero-Trust, Circuit Breaker</b></summary>

Browse [internal/](internal/) for the full list. Every package works independently with no dependency on the engine.
</details>

---

## Project Structure

```
cmd/immortal/          CLI entrypoint
internal/
  engine/              Core healing orchestrator
  agentic/             ReAct-style reasoning loop
  twin/                Digital twin simulator
  causal/              PC-algorithm + do-calculus
  federated/           FedAvg with DP and robust trim
  formal/              LTL/CTL model checker
  pqaudit/             Post-quantum audit chain
  dna/                 Anomaly detection
  causality/           Causality graph
  predict/             Predictive healing
  pattern/             Recurring failure detection
  sla/                 SLA tracking
  audit/               Immutable audit log
  dependency/          Service dependency graph
  webhook/             HMAC-signed notifications
  healing/             Rules and execution
  chaos/               Fault injection + scoring
  autolearn/           Self-learning healer
  incident/            Auto postmortem generator
  capacity/            Capacity forecasting
  correlation/         Cross-metric correlation
  playbook/            Multi-step healing with rollback
  security/            WAF, RASP, rate limiter, anti-scrape, secrets, zero-trust
  api/rest/            REST API server
  api/dashboard/       Embedded operator dashboard (vanilla JS, zero deps)
  cli/                 CLI commands
  ... and 50+ more packages
sdk/
  typescript/          Node SDK
  python/              Python SDK
  go/                  Go SDK
```

---

## Community and Contributing

- **Star** the repo if Immortal helps you. It genuinely helps us reach more operators.
- **Issues**: bug reports and feature requests at [github.com/Nagendhra-web/Immortal/issues](https://github.com/Nagendhra-web/Immortal/issues).
- **Discussions**: architecture and use-case conversations at [github.com/Nagendhra-web/Immortal/discussions](https://github.com/Nagendhra-web/Immortal/discussions).
- **Pull requests**: please read [CONTRIBUTING.md](CONTRIBUTING.md). Small, focused PRs get merged fastest.

---

## Roadmap Highlights

- [x] Post-quantum audit chain with Merkle root
- [x] Digital twin plan simulation
- [x] Agentic ReAct healing loop
- [x] PCMCI causal root-cause
- [x] Federated anomaly learning with DP
- [x] LTL/CTL formal verification
- [x] Vanilla-JS operator dashboard (12 views, command palette, SVG charts)
- [ ] Native Kubernetes operator
- [ ] Remote-write support for Mimir, VictoriaMetrics, Thanos
- [ ] eBPF-based zero-overhead syscall observer
- [ ] OpenTelemetry trace ingest

See [open milestones](https://github.com/Nagendhra-web/Immortal/milestones) for details.

---

## License

[Apache 2.0](LICENSE). Free forever, commercial use allowed, no restrictions.

---

<div align="center">

**[Install](docs/INSTALL.md) · [Documentation](docs/) · [Releases](https://github.com/Nagendhra-web/Immortal/releases) · [Report a bug](https://github.com/Nagendhra-web/Immortal/issues/new)**

*Built to keep your apps alive.*

</div>
