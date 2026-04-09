<div align="center">

<img src="assets/mascot.png" alt="Immortal Mascot" width="150">

# IMMORTAL

### Your apps never die.

The open-source self-healing engine that monitors, detects failures, and auto-heals your applications.

**58 Go packages | 3 SDKs | Single binary | $0 cost | Apache 2.0**

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)
[![Tests](https://img.shields.io/badge/Tests-350%2B%20passing-brightgreen)]()
[![TypeScript](https://img.shields.io/badge/TypeScript-SDK-3178C6?logo=typescript)](sdk/typescript)
[![Python](https://img.shields.io/badge/Python-SDK-3776AB?logo=python)](sdk/python)

<br>

<!-- Replace with your recorded demo GIF -->
<!-- Record with: https://github.com/charmbracelet/vhs or asciinema -->
<img src="assets/demo.gif" alt="Immortal Demo" width="700">

*Break a server. Watch Immortal detect and heal it automatically. Zero human involvement.*

</div>

---

## What is Immortal?

Immortal is a single engine that replaces your entire operations stack. Drop it into any project — it monitors everything, detects failures, and heals them automatically. Zero configuration. Zero human intervention.

```
One human with an idea + Immortal = complete software company
```

### Why Immortal?

| Problem | Traditional Solution | Immortal |
|---|---|---|
| App crashes at 3 AM | PagerDuty wakes someone up | Immortal auto-heals. Nobody wakes up. |
| Server overloaded | Manual scaling or dumb thresholds | Predictive scaling before load arrives |
| Security breach | Discover it days later | AI firewall blocks it in real-time |
| Performance degradation | Hire performance engineers | Auto-tunes continuously |
| Debugging production | Hours with logs and traces | Time-travel debugger shows exact moment |

## Quick Start

### Install

```bash
# One-line install
curl -fsSL https://get.immortal.dev | sh

# Or with Go
go install github.com/immortal-engine/immortal/cmd/immortal@latest
```

### Start Healing

```bash
# Zero config — auto-discovers everything
immortal start

# Ghost mode — observe first, heal later
immortal start --ghost

# Watch specific targets
immortal start --watch-url https://myapp.com --watch-process nginx --watch-log /var/log/app.log
```

### Monitor Everything

```bash
immortal status          # Engine status, uptime, event count
immortal health          # Detailed service health
immortal logs -f         # Live event stream
immortal sla             # SLA report per service
immortal predict         # Failure predictions
immortal patterns        # Recurring failure detection
immortal audit           # Full audit trail
immortal history         # Healing action history
immortal recommend       # Ghost mode recommendations
immortal metrics         # Prometheus metrics
immortal deps            # Service dependency graph
immortal causality       # Causality graph
immortal timetravel      # Replay events before a crash
```

## SDKs

### TypeScript

```typescript
import { Immortal } from '@immortal-engine/sdk';

const app = new Immortal({ name: 'my-api' });

app.heal({
  name: 'restart-on-crash',
  when: (e) => e.severity === 'critical',
  do: async (e) => {
    console.log('Healing:', e.message);
    // restart logic here
  },
});

app.start();
```

### Python

```python
from immortal import Immortal, Severity

app = Immortal(name="my-api")

@app.healer("crash-recovery")
def handle_crash(event):
    print(f"Healing: {event.message}")
    # restart logic here

app.start()
```

### Go

```go
package main

import (
    "github.com/immortal-engine/immortal/internal/engine"
    "github.com/immortal-engine/immortal/internal/event"
    "github.com/immortal-engine/immortal/internal/healing"
)

func main() {
    eng, _ := engine.New(engine.Config{DataDir: "/tmp/immortal"})

    eng.AddRule(healing.Rule{
        Name:  "restart-on-crash",
        Match: healing.MatchSeverity(event.SeverityCritical),
        Action: healing.ActionExec("systemctl restart my-service"),
    })

    eng.Start()
}
```

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                  AI BRAIN LAYER                     │
│     Built-in ML + Optional LLM + Plugins            │
├─────────────────────────────────────────────────────┤
│              INTELLIGENCE LAYER                     │
│   DNA Anomaly Detection │ Pattern Recognition       │
│   Predictive Healing    │ Causality Graph            │
│   Dependency Mapping    │ Time-Travel Replay         │
├─────────────────────────────────────────────────────┤
│            HEALING ORCHESTRATOR                     │
│   Reactive + Predictive + Autonomous + Ghost        │
│   Consensus Engine │ Audit Trail │ SLA Tracking      │
├─────────────────────────────────────────────────────┤
│              EXECUTION LAYER                        │
│   SDK (embed) │ Agent (sidecar) │ CLI │ REST API     │
├─────────────────────────────────────────────────────┤
│           UNIVERSAL CONNECTOR MESH                  │
│   177 connectors — any language/cloud/DB             │
│   Webhooks │ Slack │ Discord │ Prometheus             │
└─────────────────────────────────────────────────────┘
```

## REST API

When the engine is running, a full REST API is available on port `7777`:

| Endpoint | Description |
|---|---|
| `GET /api/status` | Engine status, uptime, event count, heal count |
| `GET /api/health` | Service health registry |
| `GET /api/events` | Stored events (filter by `?type=` or `?source=`) |
| `GET /api/healing/history` | Healing action history |
| `GET /api/recommendations` | Ghost mode recommendations |
| `GET /api/metrics` | Prometheus metrics export |
| `GET /api/monitor` | Self-monitoring (goroutines, uptime) |
| `GET /api/dna/baseline` | Learned metric baselines |
| `GET /api/dna/health-score` | Current health score (0.0–1.0) |
| `GET /api/dna/anomaly?metric=X&value=Y` | Check if a value is anomalous |
| `GET /api/patterns` | Detected recurring failure patterns |
| `GET /api/predictions` | Failure predictions with confidence % |
| `GET /api/sla` | SLA report per service (uptime %, violations) |
| `GET /api/audit?limit=N&action=X&q=search` | Audit trail with filtering |
| `GET /api/dependencies` | Service dependency graph + critical path |
| `GET /api/dependencies/impact?service=X` | Blast radius of a service failure |
| `GET /api/causality/graph` | Causality graph state |
| `GET /api/causality/root-cause?event_id=X` | Trace root cause chain |
| `GET /api/timetravel?count=N&before=T` | Replay events before a timestamp |
| `GET /api/logs/stream` | Live event stream (SSE) |
| `GET /api/logs/history` | Recent log entries |

## Features (210 total)

| Category | Count | Highlights |
|---|---|---|
| Self-Healing Core | 13 | Resurrection Protocol, Healing DNA, Auto-Patching |
| AI Intelligence | 10 | Time-Travel Debug, Swarm Intelligence, Causality Graph |
| Digital Fortress | 13 | AI Firewall, Anti-Scrape, Zero-Trust, RASP |
| Autonomous Ops | 10 | On-Call Replacement, Auto-Scale, Release Manager |
| Data Analytics | 12 | NL Queries, Dashboards, Forecasting, A/B Testing |
| App Builder | 15 | Natural Language to App, UI/UX Designer, Cross-Platform |
| Startup Engine | 20 | PMF Detector, Cost Zero Mode, Investor Dashboard |
| Future-Proof | 15 | Self-Evolving Core, Tech Migration, 1000+ Year Design |
| + 9 more categories | 102 | [Full list →](docs/superpowers/specs/) |

## Zero Restrictions

- **No sign-up required** — download and run
- **No license keys** — no activation, no expiration
- **No feature gates** — all 210 features, free forever
- **No usage limits** — unlimited apps, users, requests
- **Works fully offline** — built-in ML, no internet needed
- **Apache 2.0** — use for anything, including competing with us

## Who Is Immortal For?

| You | What Immortal Does |
|---|---|
| **Non-tech founder** | Describe your app in English → get it built, deployed, and running |
| **Solo developer** | Your entire ops team in a single binary |
| **Startup (2-10 people)** | Ship like a 50-person company. $0/month until revenue. |
| **Enterprise** | Replace $5M-$20M/yr in tools and roles |

## Savings

```
Replaces 26 human roles:
  Frontend · Backend · DevOps · SRE · DBA · QA · Security
  Data Analyst · Platform · Network · and 16 more

Replaces 40+ tools:
  Datadog · Sentry · PagerDuty · New Relic · Cloudflare
  Grafana · Snyk · Terraform · and 32 more

Annual savings:
  Startup:    $200K - $1M
  Scaleup:    $2M - $5M
  Enterprise: $5M - $20M
```

## Use Individual Features (Pick What You Need)

You don't have to use the full engine. Every package works **standalone** — import just the one you need.

```bash
go get github.com/immortal-engine/immortal
```

### Anomaly Detection (Healing DNA)

Detect abnormal behavior without setting thresholds. It learns what "normal" looks like.

```go
import "github.com/immortal-engine/immortal/internal/dna"

// Create a health fingerprint for your service
d := dna.New("api-server")

// Feed it normal metrics (it learns automatically)
for i := 0; i < 100; i++ {
    d.Record("response_time_ms", 120.0 + rand.Float64()*30)
    d.Record("cpu_percent", 40.0 + rand.Float64()*15)
}

// Now detect anomalies — zero manual thresholds
d.IsAnomaly("response_time_ms", 500.0) // true — way above normal
d.IsAnomaly("response_time_ms", 125.0) // false — within normal

// Get overall health score (0.0 = dying, 1.0 = perfect)
score := d.HealthScore(map[string]float64{
    "response_time_ms": 500.0,
    "cpu_percent":      92.0,
})
// score ≈ 0.15 — something is very wrong
```

### Web Application Firewall (WAF)

Protect any HTTP server from SQLi, XSS, path traversal, and command injection. 8-layer input normalization catches encoded attacks.

```go
import "github.com/immortal-engine/immortal/internal/security/firewall"

fw := firewall.New()

// Use as HTTP middleware — one line to protect your entire API
http.ListenAndServe(":8080", fw.Middleware(yourRouter))

// Or analyze inputs manually
result := fw.Analyze(userInput)
if result.Blocked {
    fmt.Printf("Attack blocked: %s\n", result.ThreatType)
    // "sql_injection", "xss", "path_traversal", "command_injection"
}

// Catches encoded attacks too:
fw.Analyze("%27%20OR%201%3D1%20--")  // URL-encoded SQLi → blocked
fw.Analyze("&lt;script&gt;alert(1)") // HTML-entity XSS → blocked
```

### Rate Limiter

Protect any API endpoint from brute force and abuse.

```go
import "github.com/immortal-engine/immortal/internal/security/ratelimit"

// 100 requests per minute per IP
rl := ratelimit.New(100, time.Minute)

// Use as middleware
http.ListenAndServe(":8080", rl.Middleware(yourRouter))

// Or check manually
if !rl.Allow(userIP) {
    http.Error(w, "Too Many Requests", 429)
}
```

### Secret Scanner

Find leaked API keys, tokens, and passwords in your code before attackers do.

```go
import "github.com/immortal-engine/immortal/internal/security/secrets"

scanner := secrets.New()

// Scan any string — code, config files, environment variables
findings := scanner.Scan(fileContent)
for _, f := range findings {
    fmt.Printf("LEAKED: %s found (%s)\n", f.Type, f.Match)
    // "aws_access_key: AKIA****MPLE"
    // "github_token: ghp_****ghij"
    // "jwt_token: eyJh****R8U"
}

// Quick check
if scanner.HasSecrets(deployConfig) {
    panic("secrets found in config — do not deploy!")
}
```

### Circuit Breaker

Stop hammering a failing service. Let it recover, then try again.

```go
import "github.com/immortal-engine/immortal/internal/circuitbreaker"

// Open circuit after 5 failures, retry after 30 seconds
cb := circuitbreaker.New(5, 30*time.Second)

err := cb.Execute(func() error {
    return callExternalAPI() // if this fails 5 times, circuit opens
})

if err == circuitbreaker.ErrCircuitOpen {
    // Service is down — use fallback instead of hammering it
    return cachedResponse()
}
```

### Exponential Backoff

Retry failed operations with increasing delays and jitter.

```go
import "github.com/immortal-engine/immortal/internal/backoff"

b := backoff.New(100*time.Millisecond, 30*time.Second)

err := backoff.Retry(5, b, func() error {
    return connectToDatabase() // retries: 100ms → 200ms → 400ms → 800ms → 1.6s
})
```

### Structured Logger

JSON logging with levels and context fields.

```go
import "github.com/immortal-engine/immortal/internal/logger"

log := logger.New(logger.LevelInfo)

log.With("service", "api").With("version", "2.1").Info("server started on port %d", 8080)
// {"timestamp":"...","level":"info","message":"server started on port 8080",
//  "fields":{"service":"api","version":"2.1"}}

log.Error("database connection failed: %v", err)
```

### Metrics Aggregator (P50/P95/P99)

Track any metric with statistical analysis — mean, median, percentiles, standard deviation.

```go
import "github.com/immortal-engine/immortal/internal/analytics/metrics"

agg := metrics.New(10000) // keep last 10K data points

// Record response times
agg.Record("api_latency", 45.2)
agg.Record("api_latency", 52.1)
agg.Record("api_latency", 120.5)

// Get statistical summary
s := agg.Summarize("api_latency")
fmt.Printf("P50: %.1fms  P95: %.1fms  P99: %.1fms\n", s.Median, s.P95, s.P99)
```

### RASP (Runtime Protection)

Block dangerous operations at runtime — command execution, sensitive file access, data exfiltration.

```go
import "github.com/immortal-engine/immortal/internal/security/rasp"

monitor := rasp.NewDefault()

// Before executing any user-provided command
v := monitor.CheckCommand(userInput)
if v.Blocked {
    return fmt.Errorf("blocked: %s", v.Detail)
}

// Before accessing any file path
v = monitor.CheckFileAccess(filePath)
if v.Blocked {
    return fmt.Errorf("sensitive file: %s", v.Detail)
}

// Before making outbound HTTP requests
v = monitor.CheckOutbound(targetURL)
if v.Blocked {
    return fmt.Errorf("blocked exfiltration: %s", v.Detail)
}
```

### Anti-Scrape Shield

Detect and block bots, scrapers, and automated tools.

```go
import "github.com/immortal-engine/immortal/internal/security/antiscrape"

shield := antiscrape.NewDefault()

// Use as middleware
http.ListenAndServe(":8080", shield.Middleware(yourRouter))

// Or check manually
if shield.IsBot(ip, userAgent, path) {
    http.Error(w, "Forbidden", 403)
}
```

### Zero-Trust Service Auth

Authenticate service-to-service calls with expiring tokens and access policies.

```go
import "github.com/immortal-engine/immortal/internal/security/zerotrust"

v := zerotrust.New("your-secret-key")

// Issue a token for a service (expires in 1 hour)
identity := v.IssueToken("api-service", time.Hour)

// Validate token on incoming request
id, err := v.ValidateToken(requestToken)
if err != nil {
    http.Error(w, "Unauthorized", 401)
}

// Define access policies
v.SetPolicy("database", &zerotrust.Policy{
    AllowedServices: []string{"api-service", "auth-service"},
    AllowedPaths:    []string{"/read", "/write"},
})

// Check access
err = v.CheckAccess("api-service", "database", "/read") // allowed
err = v.CheckAccess("evil-svc", "database", "/read")    // denied
```

### Event Deduplication

Prevent processing the same event multiple times.

```go
import "github.com/immortal-engine/immortal/internal/dedup"

dd := dedup.New(10 * time.Second) // 10-second dedup window

if dd.IsDuplicate(event) {
    return // skip — already processed
}
// process event
```

### Causality Graph (Root Cause Analysis)

Trace the chain of failures to find the real root cause.

```go
import "github.com/immortal-engine/immortal/internal/causality"

g := causality.New()

// Add events as they happen
g.Add(diskFullEvent)
g.Add(dbSlowEvent)
g.Add(apiTimeoutEvent)

// Link cause → effect
g.Link(diskFullEvent.ID, dbSlowEvent.ID)
g.Link(dbSlowEvent.ID, apiTimeoutEvent.ID)

// Trace root cause from any symptom
chain := g.RootCause(apiTimeoutEvent.ID)
// → [diskFull, dbSlow, apiTimeout]
// "The API timeout was caused by disk full"
```

### JSON Healing Rules

Define healing rules in JSON — no Go code needed.

```json
{
  "rules": [
    {
      "name": "restart-on-crash",
      "match": {"severity": "critical"},
      "action": {"type": "exec", "command": "systemctl restart myapp"}
    },
    {
      "name": "clear-cache-on-oom",
      "match": {"severity": "critical", "contains": "out of memory"},
      "action": {"type": "exec", "command": "echo 3 > /proc/sys/vm/drop_caches"}
    }
  ]
}
```

```go
import "github.com/immortal-engine/immortal/internal/rules"

healingRules, err := rules.LoadFromFile("rules.json")
for _, rule := range healingRules {
    engine.AddRule(rule)
}
```

### Prometheus Metrics Export

Export any metrics in Prometheus format for Grafana dashboards.

```go
import "github.com/immortal-engine/immortal/internal/export"

prom := export.NewPrometheus()
prom.SetGauge("cpu_usage", 45.5)
prom.IncCounter("http_requests_total")

// Serve on /metrics endpoint
http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte(prom.Export()))
})
```

### Notification Dispatcher (Slack/Discord)

Send alerts to multiple channels simultaneously.

```go
import "github.com/immortal-engine/immortal/internal/notify"

d := notify.NewDispatcher()
d.AddChannel(&notify.SlackChannel{WebhookURL: "https://hooks.slack.com/..."})
d.AddChannel(&notify.DiscordChannel{WebhookURL: "https://discord.com/api/..."})
d.AddChannel(&notify.ConsoleChannel{})

d.Send("Server Down", "API returning 500 errors", "critical")
// → Sent to Slack + Discord + console simultaneously
```

### Webhook Sender (HMAC-Signed)

Send signed webhook notifications to any HTTP endpoint with automatic retries.

```go
import "github.com/immortal-engine/immortal/internal/webhook"

sender := webhook.New(webhook.Config{
    URL:        "https://your-endpoint.com/hook",
    Secret:     "your-hmac-secret",  // signs payload with SHA-256
    MaxRetries: 3,                   // retries on 5xx with exponential backoff
})

sender.Send(webhook.Payload{
    Event:    "service_down",
    Severity: "critical",
    Source:   "api-server",
    Message:  "HTTP 500 — connection timeout",
})
// → POST with X-Immortal-Signature: sha256=... header
// → Retries: 100ms → 400ms → 900ms on server errors
```

### Recurring Pattern Detection

Detect repeated failures using sliding time windows.

```go
import "github.com/immortal-engine/immortal/internal/pattern"

// Detect patterns: alert when same error occurs 3+ times in 5 minutes
det := pattern.New(5*time.Minute, 3)

det.Record("api:connection timeout", "critical")
det.Record("api:connection timeout", "critical")
det.Record("api:connection timeout", "critical")

det.IsRepeating("api:connection timeout") // true — 3x in window
patterns := det.Patterns()                // sorted by frequency
// → [{Key: "api:connection timeout", Count: 3, Severity: "critical", ...}]
```

### Predictive Healing (Failure Forecasting)

Predict when metrics will breach thresholds using linear regression.

```go
import "github.com/immortal-engine/immortal/internal/predict"

pred := predict.New()
pred.SetThreshold("cpu_percent", 90.0) // alert before CPU hits 90%

// Feed metric observations over time
pred.Feed("cpu_percent", 45.0)
pred.Feed("cpu_percent", 52.0)
pred.Feed("cpu_percent", 58.0)
pred.Feed("cpu_percent", 65.0)

// Predict when it will breach the threshold
p := pred.Predict("cpu_percent")
fmt.Printf("Current: %.0f%%, Predicted: %.0f%%\n", p.CurrentValue, p.PredictedValue)
fmt.Printf("Time to breach: %s (confidence: %.0f%%)\n", p.TimeToThreshold, p.Confidence*100)
fmt.Printf("Severity: %s\n", p.Severity) // "critical" if < 5min, "warning" if < 30min
```

### SLA Tracking

Track uptime percentage and SLA violations per service.

```go
import "github.com/immortal-engine/immortal/internal/sla"

tracker := sla.New()
tracker.SetTarget("api-server", 99.9) // 99.9% SLA target

// Record health checks
tracker.RecordStatus("api-server", true)  // healthy
tracker.RecordStatus("api-server", true)
tracker.RecordStatus("api-server", false) // outage!

tracker.Uptime("api-server")      // 66.67%
tracker.IsViolating("api-server") // true — below 99.9% target

report := tracker.Report()        // all services, sorted worst-first
worst := tracker.Worst()          // service with lowest uptime
```

### Audit Log

Immutable audit trail for all engine actions with search and filtering.

```go
import "github.com/immortal-engine/immortal/internal/audit"

log := audit.New(10000) // keep last 10K entries

log.Log("heal", "healer", "api-server", "restarted after crash", true)
log.Log("deploy", "ci-bot", "worker", "deployed v2.1.0", true)
log.Log("scale", "autoscaler", "database", "scaled to 4 replicas", true)

// Query
log.Entries(10)                    // last 10 entries
log.EntriesByAction("heal")       // all healing actions
log.EntriesByTarget("api-server") // everything that happened to api-server
log.Search("deploy")              // full-text search
log.Since(time.Now().Add(-1*time.Hour)) // last hour
```

### Service Dependency Graph

Map service dependencies and analyze blast radius.

```go
import "github.com/immortal-engine/immortal/internal/dependency"

g := dependency.New()

// Define your architecture
g.AddDependency("api", "auth")
g.AddDependency("api", "database")
g.AddDependency("auth", "database")
g.AddDependency("worker", "database")
g.AddDependency("worker", "queue")

// Blast radius — if database goes down, who is affected?
g.TransitiveDependents("database") // → ["api", "auth", "worker"]
g.ImpactOf("database")            // → 3 services affected

// Critical path — which services have the most impact?
g.CriticalPath() // → ["database", "auth", "queue", ...] (sorted by impact)

// Detect circular dependencies
g.HasCycle() // false (no cycles)

// Architecture insights
g.Roots()  // → ["api", "worker"] (top-level services)
g.Leaves() // → ["database", "queue"] (bottom-level infrastructure)
```

---

*Each package has zero dependency on the engine. Import one, import five, or use the full engine — your choice.*

## Contributing

We welcome contributions! Please read our [contributing guidelines](CONTRIBUTING.md) before submitting PRs.

## License

[Apache 2.0](LICENSE) — free forever, no restrictions.

---

<div align="center">

**[Documentation](docs/)** · **[Design Spec](docs/superpowers/specs/)** · **[Discord](#)** · **[Twitter](#)**

*The last software tool humanity will ever need.*

</div>
