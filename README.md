<div align="center">

# IMMORTAL

### Your apps never die.

The open-source self-healing engine that monitors, detects failures, and auto-heals your applications — with zero configuration.

**69 packages | 3 SDKs | 70 test suites | Single binary | Apache 2.0**

[![CI](https://github.com/Nagendhra-web/Immortal/actions/workflows/ci.yml/badge.svg)](https://github.com/Nagendhra-web/Immortal/actions)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![TypeScript](https://img.shields.io/badge/SDK-TypeScript-3178C6?logo=typescript&logoColor=white)](sdk/typescript)
[![Python](https://img.shields.io/badge/SDK-Python-3776AB?logo=python&logoColor=white)](sdk/python)
[![Version](https://img.shields.io/badge/version-0.3.0-green)]()

</div>

---

```
Server crashes at 3 AM.
  Traditional: PagerDuty pages you → you wake up → you SSH in → you restart → 45 min downtime.
  Immortal:    Detects in 200ms → heals automatically → 0 downtime → you sleep through it.
```

---

## What It Actually Does

Immortal is a Go engine that watches your applications and heals them when things break. No magic — just solid engineering.

**The healing loop:**
```
Monitor → Detect → Throttle → Deduplicate → Analyze → Heal → Audit
  ↑                                                          |
  └──────────────────────────────────────────────────────────┘
```

**What's real and tested:**

| Capability | Package | What it does |
|---|---|---|
| **Self-healing** | `engine` | Detect failures, match rules, execute fixes automatically |
| **Anomaly detection** | `dna` | Learns normal baselines, flags deviations (3-sigma rule) |
| **Root cause analysis** | `causality` | Traces cascading failures (disk full → DB crash → API timeout) |
| **Predictive healing** | `predict` | Linear regression on metrics — warns before thresholds breach |
| **Pattern detection** | `pattern` | Detects recurring failures with sliding time windows |
| **SLA tracking** | `sla` | Uptime %, violation alerts, worst-service ranking |
| **Ghost mode** | `engine` | Observe-only mode — recommends but doesn't act |
| **Time-travel** | `timetravel` | Replay events before a crash to understand what happened |
| **Audit trail** | `audit` | Immutable log of every action with search/filter |
| **Dependency graph** | `dependency` | Map service dependencies, analyze blast radius |
| **Webhook alerts** | `webhook` | HMAC-signed HTTP notifications with retries |
| **Event throttling** | `throttle` | Blocks event floods (999/1000 duplicates blocked in tests) |
| **WAF** | `security/firewall` | SQLi, XSS, path traversal, command injection protection |
| **RASP** | `security/rasp` | Runtime protection against dangerous operations |
| **Rate limiting** | `security/ratelimit` | Per-IP request throttling |
| **Anti-scrape** | `security/antiscrape` | Bot and scraper detection |
| **Secret scanning** | `security/secrets` | Find leaked API keys, tokens, passwords |
| **Zero-trust auth** | `security/zerotrust` | Service-to-service auth with expiring tokens |
| **Circuit breaker** | `circuitbreaker` | Stop hammering failing services |
| **Prometheus export** | `export` | Metrics in Prometheus format |
| **Notifications** | `notify` | Slack, Discord, console alerts |
| **Chaos testing** | `chaos` | Inject failures, verify healing works, score effectiveness |
| **Self-learning** | `autolearn` | Watches successful heals, suggests new rules automatically |
| **Incident reports** | `incident` | Auto-generated postmortems with timeline, root cause, markdown export |
| **Capacity forecast** | `capacity` | Multi-metric forecasting, exhaustion date prediction |
| **Metric correlation** | `correlation` | Pearson correlation, discovers leading indicators across metrics |
| **Healing playbooks** | `playbook` | Multi-step healing with conditions, retries, and auto-rollback |
| **Agentic healing loop** | `agentic` | ReAct-style multi-step reasoning — Plan → Act → Observe → Re-plan until resolved |
| **Post-quantum audit chain** | `pqaudit` | Hash-chained, signed audit ledger with Merkle root; tamper-evident end-to-end |
| **Digital twin simulator** | `twin` | Simulates healing plans against a shadow state machine; rejects plans that worsen predicted score |
| **Federated anomaly learning** | `federated` | FedAvg across a fleet of Immortal instances with robust trim + DP noise — raw metrics stay local |
| **Causal inference** | `causal` | PC-algorithm discovery + do-calculus ACE — identifies *true* root causes, not spurious correlations |

---

## Proven in Tests

These aren't hypothetical — every scenario runs in CI:

```
PASS  TestScenario_APIReturns500_ImmortalHeals     — server broke, Immortal healed it
PASS  TestScenario_LogErrors_ImmortalDetects        — caught ERROR + FATAL from log tailing
PASS  TestScenario_CPUAnomaly_DNADetects            — flagged 95% CPU as anomaly (baseline: 44.5%)
PASS  TestScenario_GhostMode_ObservesOnly           — observed without acting, produced recommendations
PASS  TestScenario_CascadingFailure_CausalityTracked — traced: disk full → DB crash → API timeout
PASS  TestScenario_EventFlood_ThrottlePrevents      — blocked 999/1000 duplicate events
PASS  TestScenario_TimeTravel_ReplayBeforeFailure   — replayed 5 events before crash
PASS  TestScenario_RESTAPI_QueryWhileRunning        — queried health, metrics, Prometheus while live
PASS  TestRealWorld_DatabaseDown_AgentRestartsAndVerifies — agentic loop: check → restart → verify → finish
PASS  TestRealWorld_AttackerTriesToHideAction        — pqaudit Merkle root exposes attempted field mutation
PASS  TestRealWorld_CascadeFailure_TwinRejectsBadPlan — twin accepts failover:db+restart:api, rejects scale-to-1
PASS  TestRealWorld_FleetLearnsCPUBaseline           — federated FedAvg resists malicious outlier via trim
PASS  TestRealWorld_CausalRootCauseBeatsCorrelation  — r=0.83 red herring demoted below true causes

69 packages | 0 failures
```

---

## Quick Start

```bash
# Install
go install github.com/immortal-engine/immortal/cmd/immortal@latest

# Start healing
immortal start

# Ghost mode — observe first, heal later
immortal start --ghost

# Watch specific targets
immortal start --watch-url https://myapp.com --watch-process nginx --watch-log /var/log/app.log
```

## CLI

```bash
immortal status       # Engine status, uptime, event count
immortal health       # Detailed service health per service
immortal logs -f      # Live event stream (like tail -f)
immortal sla          # SLA report — uptime %, violations
immortal predict      # Failure predictions with confidence %
immortal patterns     # Recurring failure detection
immortal audit        # Full audit trail with search
immortal history      # Healing action history
immortal recommend    # Ghost mode recommendations
immortal metrics      # Prometheus metrics
immortal deps         # Service dependency graph + blast radius
immortal causality    # Causality graph + root cause tracing
immortal timetravel   # Replay events before a crash
```

## REST API (31 endpoints)

All features accessible over HTTP when the engine runs (default port `7777`):

<details>
<summary><b>View all endpoints</b></summary>

| Endpoint | Description |
|---|---|
| `GET /api/status` | Engine status, uptime, event/heal counts |
| `GET /api/health` | Service health registry |
| `GET /api/events?type=&source=` | Stored events with filters |
| `GET /api/healing/history` | Healing action history |
| `GET /api/recommendations` | Ghost mode recommendations |
| `GET /api/metrics` | Prometheus metrics (text format) |
| `GET /api/monitor` | Self-monitoring (goroutines, uptime) |
| `GET /api/dna/baseline` | Learned metric baselines |
| `GET /api/dna/health-score` | Health score (0.0-1.0) |
| `GET /api/dna/anomaly?metric=&value=` | Anomaly check |
| `GET /api/patterns` | Recurring failure patterns |
| `GET /api/predictions` | Failure predictions |
| `GET /api/sla` | SLA report per service |
| `GET /api/audit?limit=&action=&q=` | Audit trail with search |
| `GET /api/dependencies` | Dependency graph + critical path |
| `GET /api/dependencies/impact?service=` | Blast radius analysis |
| `GET /api/causality/graph` | Causality graph |
| `GET /api/causality/root-cause?event_id=` | Root cause chain |
| `GET /api/timetravel?count=&before=` | Event replay |
| `GET /api/logs/stream` | Live SSE event stream |
| `GET /api/logs/history` | Recent log entries |
| `GET /api/chaos/report` | Chaos test results and healing score |
| `GET /api/autolearn/rules?suggested=true` | Self-learned healing rules |
| `GET /api/autolearn/stats` | Learning statistics |
| `GET /api/incidents` | All incident reports |
| `GET /api/incidents/active` | Open incidents |
| `GET /api/capacity` | Capacity forecasts for all metrics |
| `GET /api/capacity/critical?days=7` | Resources exhausting within N days |
| `GET /api/correlations?metric=X` | Cross-metric correlations and leading indicators |
| `GET /api/playbooks` | Registered healing playbooks |
| `GET /api/playbooks/history` | Playbook execution history |

</details>

## SDKs

<details>
<summary><b>TypeScript</b></summary>

```typescript
import { Immortal } from '@immortal-engine/sdk';

const app = new Immortal({ name: 'my-api' });

app.heal({
  name: 'restart-on-crash',
  when: (e) => e.severity === 'critical',
  do: async (e) => {
    console.log('Healing:', e.message);
  },
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
    Name:  "restart-on-crash",
    Match: healing.MatchSeverity(event.SeverityCritical),
    Action: healing.ActionExec("systemctl restart my-service"),
})

eng.Start()
```
</details>

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                  AI BRAIN LAYER                     │
│   Auto-Learning │ Metric Correlation │ Prediction    │
│   Built-in ML + Optional LLM + Plugins              │
├─────────────────────────────────────────────────────┤
│              INTELLIGENCE LAYER                     │
│   DNA Anomaly Detection │ Pattern Recognition       │
│   Capacity Forecasting  │ Causality Graph            │
│   Dependency Mapping    │ Time-Travel Replay         │
├─────────────────────────────────────────────────────┤
│            HEALING ORCHESTRATOR                     │
│   Reactive + Predictive + Autonomous + Ghost        │
│   Multi-Step Playbooks │ Auto-Rollback │ Consensus   │
│   Incident Reports │ Audit Trail │ SLA Tracking      │
├─────────────────────────────────────────────────────┤
│              CHAOS + VERIFICATION                   │
│   Fault Injection │ Healing Validation │ Scoring     │
├─────────────────────────────────────────────────────┤
│              EXECUTION LAYER                        │
│   SDK (embed) │ Agent (sidecar) │ CLI │ REST API     │
├─────────────────────────────────────────────────────┤
│           UNIVERSAL CONNECTOR MESH                  │
│   177 connectors — any language/cloud/DB             │
│   Webhooks │ Slack │ Discord │ Prometheus             │
└─────────────────────────────────────────────────────┘
```

## Use Individual Packages

Every package works **standalone** — import just what you need:

```bash
go get github.com/immortal-engine/immortal
```

<details>
<summary><b>Anomaly Detection (DNA)</b></summary>

```go
import "github.com/immortal-engine/immortal/internal/dna"

d := dna.New("api-server")

// Feed normal metrics — it learns automatically
for i := 0; i < 100; i++ {
    d.Record("response_time_ms", 120.0 + rand.Float64()*30)
}

d.IsAnomaly("response_time_ms", 500.0) // true — way above normal
d.IsAnomaly("response_time_ms", 125.0) // false — within normal

score := d.HealthScore(map[string]float64{"response_time_ms": 500.0})
// score ≈ 0.15 — something is very wrong
```
</details>

<details>
<summary><b>Predictive Healing</b></summary>

```go
import "github.com/immortal-engine/immortal/internal/predict"

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
<summary><b>SLA Tracking</b></summary>

```go
import "github.com/immortal-engine/immortal/internal/sla"

tracker := sla.New()
tracker.SetTarget("api-server", 99.9)

tracker.RecordStatus("api-server", true)
tracker.RecordStatus("api-server", false) // outage

tracker.Uptime("api-server")      // uptime %
tracker.IsViolating("api-server") // true if below target
```
</details>

<details>
<summary><b>Dependency Graph</b></summary>

```go
import "github.com/immortal-engine/immortal/internal/dependency"

g := dependency.New()
g.AddDependency("api", "database")
g.AddDependency("worker", "database")

g.TransitiveDependents("database") // ["api", "worker"]
g.ImpactOf("database")            // 2 services affected
g.CriticalPath()                   // sorted by impact
```
</details>

<details>
<summary><b>WAF (Web Application Firewall)</b></summary>

```go
import "github.com/immortal-engine/immortal/internal/security/firewall"

fw := firewall.New()
http.ListenAndServe(":8080", fw.Middleware(yourRouter))

result := fw.Analyze(userInput)
if result.Blocked {
    fmt.Printf("Blocked: %s\n", result.ThreatType) // sql_injection, xss, etc.
}
```
</details>

<details>
<summary><b>Webhook Sender</b></summary>

```go
import "github.com/immortal-engine/immortal/internal/webhook"

sender := webhook.New(webhook.Config{
    URL:    "https://your-endpoint.com/hook",
    Secret: "hmac-secret", // SHA-256 signed
})

sender.Send(webhook.Payload{
    Event: "service_down", Severity: "critical",
    Source: "api-server", Message: "HTTP 500",
})
```
</details>

<details>
<summary><b>Audit Log</b></summary>

```go
import "github.com/immortal-engine/immortal/internal/audit"

log := audit.New(10000)
log.Log("heal", "healer", "api-server", "restarted after crash", true)

log.Entries(10)              // last 10
log.EntriesByAction("heal")  // filter by action
log.Search("deploy")         // full-text search
```
</details>

<details>
<summary><b>Pattern Detection</b></summary>

```go
import "github.com/immortal-engine/immortal/internal/pattern"

det := pattern.New(5*time.Minute, 3) // 3+ occurrences in 5 min = pattern

det.Record("api:connection timeout", "critical")
det.Record("api:connection timeout", "critical")
det.Record("api:connection timeout", "critical")

det.IsRepeating("api:connection timeout") // true
det.Patterns()                            // sorted by frequency
```
</details>

<details>
<summary><b>Chaos Testing</b></summary>

```go
import "github.com/immortal-engine/immortal/internal/chaos"

// Create a chaos engine that injects events into your healing engine
ch := chaos.New(engine.Ingest)

// Inject faults and see if your healing rules catch them
ch.InjectHTTPError("api-server", 500)
ch.InjectProcessCrash("nginx")
ch.InjectCPUSpike(95.0)

// Record whether each fault was detected and healed
ch.RecordResult(fault.ID, true, 200*time.Millisecond, true, 500*time.Millisecond)

// Score your healing effectiveness (0.0 to 1.0)
fmt.Printf("Healing score: %.0f%%\n", ch.Score()*100)
```
</details>

<details>
<summary><b>Self-Learning Healer</b></summary>

```go
import "github.com/immortal-engine/immortal/internal/autolearn"

learner := autolearn.New(5) // suggest rules after 5 occurrences

// The engine feeds successful heals automatically
learner.Record("restart-rule", "api-server", "crash", "critical", true)
// ... after enough observations ...

// Get suggested rules the engine learned on its own
suggested := learner.SuggestedRules()
for _, rule := range suggested {
    fmt.Printf("Suggested: %s (confidence: %.0f%%)\n", rule.Pattern, rule.Confidence*100)
}
```
</details>

<details>
<summary><b>Incident Reports</b></summary>

```go
import "github.com/immortal-engine/immortal/internal/incident"

mgr := incident.New()

// Open an incident
inc := mgr.Open("API Outage", "critical")
mgr.AddEvent(inc.ID, "api-server", "critical", "HTTP 500 errors")
mgr.AddEvent(inc.ID, "database", "error", "connection pool exhausted")
mgr.SetRootCause(inc.ID, "database connection limit reached")
mgr.AddAffectedService(inc.ID, "api-server")
mgr.AddHealingAction(inc.ID, "restarted connection pool")
mgr.Resolve(inc.ID, "Connection pool restarted, API recovered")

// Auto-generate a markdown postmortem
fmt.Println(mgr.GenerateMarkdown(inc.ID))
```
</details>

<details>
<summary><b>Capacity Forecasting</b></summary>

```go
import "github.com/immortal-engine/immortal/internal/capacity"

planner := capacity.New()
planner.SetCapacity("disk_gb", 500) // 500 GB total

// Feed observations over time
planner.Record("disk_gb", 200)
planner.Record("disk_gb", 220)
planner.Record("disk_gb", 245)

f := planner.Forecast("disk_gb")
fmt.Printf("Trend: %s, Growth: %.1f GB/hour\n", f.Trend, f.GrowthRate)
fmt.Printf("Disk full in: %.0f days\n", f.DaysUntilFull)

// Find resources running out within 7 days
critical := planner.Critical(7)
```
</details>

<details>
<summary><b>Metric Correlation</b></summary>

```go
import "github.com/immortal-engine/immortal/internal/correlation"

engine := correlation.New()

// Feed metrics over time
for i := 0; i < 100; i++ {
    engine.Record("cpu", float64(i)*2)
    engine.Record("memory", float64(i)*1.5)
    engine.Record("latency", float64(i)*3)
}

// Discover hidden relationships
all := engine.AllCorrelations() // sorted by strength

// Find leading indicators for a metric
leaders := engine.LeadingIndicators("latency")
// "memory spikes 5 minutes before latency increases"
```
</details>

<details>
<summary><b>Healing Playbooks</b></summary>

```go
import "github.com/immortal-engine/immortal/internal/playbook"

runner := playbook.New()

runner.Register("deploy-recovery", []playbook.Step{
    {
        Name:     "backup-db",
        Action:   func() error { /* backup */ return nil },
        Rollback: func() error { /* restore */ return nil },
    },
    {
        Name:    "run-migration",
        Action:  func() error { /* migrate */ return nil },
        Retries: 3, // retry up to 3 times
    },
    {
        Name:      "restart-service",
        Action:    func() error { /* restart */ return nil },
        Condition: func() bool { return isServiceDown() },
    },
})

// Execute (auto-rollback if any step fails)
exec, err := runner.Run("deploy-recovery")

// Or dry-run first
exec, _ = runner.DryRun("deploy-recovery")
```
</details>

<details>
<summary><b>More: Circuit Breaker, Rate Limiter, Secret Scanner, RASP, Anti-Scrape, Zero-Trust, Backoff, Dedup, Causality, Logger, Metrics, Notifications, Prometheus</b></summary>

See the [full package list](internal/) — each one works independently with zero dependencies on the engine.
</details>

## Project Structure

```
cmd/immortal/        CLI entrypoint
internal/
  engine/            Core healing orchestrator
  dna/               Anomaly detection (learns baselines)
  causality/         Root cause analysis graph
  predict/           Predictive healing (linear regression)
  pattern/           Recurring failure detection
  sla/               SLA tracking per service
  audit/             Immutable audit log
  dependency/        Service dependency graph
  webhook/           HMAC-signed HTTP notifications
  healing/           Healing rules and execution
  chaos/             Chaos testing (fault injection + scoring)
  autolearn/         Self-learning healer (auto-suggests rules)
  incident/          Incident report generator (auto postmortem)
  capacity/          Capacity forecasting (exhaustion prediction)
  correlation/       Cross-metric correlation (leading indicators)
  playbook/          Multi-step healing playbooks (auto-rollback)
  security/          WAF, RASP, rate limiter, anti-scrape, secrets, zero-trust
  api/rest/          REST API server (31 endpoints)
  cli/               CLI commands (16 commands)
  ... and 30+ more packages
sdk/
  typescript/        Node.js SDK
  python/            Python SDK
```

## Contributing

Contributions welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) before submitting PRs.

## License

[Apache 2.0](LICENSE) — free forever, no restrictions.

---

<div align="center">

**[Documentation](docs/) · [Design Spec](docs/superpowers/specs/)**

*Built to keep your apps alive.*

</div>
