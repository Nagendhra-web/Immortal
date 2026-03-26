# Immortal Engine — Performance Characteristics

## Honest Numbers (What We Measured vs What It Means)

### Throughput (End-to-End, Not Synthetic)

| Metric | Measured | Context |
|---|---|---|
| Bus publish speed | ~1.2M events/sec | In-memory only. NOT the real-world number. |
| **Full E2E pipeline** | **~1,785 events/sec** | **Real number.** Includes throttle → dedup → SQLite store → DNA → causality → timetravel → healer → consensus → action → alert → export. Sustained over 10 seconds. |
| **Real-world with I/O** | **~333 events/sec** | With unique events, full metadata, SQLite writes. This is what you get in production. |

### Latency (End-to-End, Not Decision-Only)

| Metric | p50 | p95 | p99 | What It Measures |
|---|---|---|---|---|
| **Full E2E pipeline** | **1.3ms** | **3.0ms** | **49ms** | Complete path: Ingest → all 12 pipeline stages → action fires |
| **HTTP detect → heal** | - | - | **~103ms** | Real scenario: HTTP health check detects 500 → action fires. Includes poll interval. |
| **Log detect → heal** | - | - | **~103ms** | Real scenario: error written to log → tailed → action fires. Includes poll interval. |
| Action execution | Depends | Depends | Depends | **Not our latency** — depends on what the action does (restart: ~100ms, API call: ~50-500ms) |

**Key distinction:**
- **Decision latency (p50=1.3ms):** Time for Immortal to process an event through the full pipeline and fire the action
- **Detection latency (~100ms):** Time from real-world failure to detection. Bounded by poll interval (configurable, default 5-10s in production, 100ms in tests)
- **Action latency:** Whatever your healing command takes. Not Immortal's time.

### Backpressure

| Scenario | Behavior |
|---|---|
| Buffer has space | Event queued, processed by worker pool |
| Buffer full + info/warning event | Event dropped gracefully |
| Buffer full + critical/fatal event | Event **blocks until space** — never dropped |

**Priority guarantee:** Critical and fatal events are guaranteed delivery even under extreme load. Info/warning events are sacrificed to protect the system from OOM.

**Starvation note:** Under sustained extreme load, lower-priority events may be continuously dropped. This is by design — survival over perfection. In practice, the 10K buffer handles normal burst patterns. If you're consistently dropping events, scale up (more workers, larger buffer, or multi-node cluster).

### Memory

| Scenario | Observed |
|---|---|
| Idle | ~1 MB heap |
| 10K events | ~7 MB heap |
| After GC | Returns to ~1-2 MB |
| Sustained load (5K events) | ~4 MB stable |

**Bounded state:**
- Event bus: bounded channel (configurable, default 10K)
- TimeTravel: capped at 10K events (ring buffer)
- Recommendations: capped at 10K (oldest evicted)
- Healing history: grows in memory (bounded by event rate + throttle)
- SQLite: bounded by retention policy (configurable max age/count)

### What We Fixed (and What Remains)

| Concern | What We Did | Remaining Trade-off |
|---|---|---|
| SQLite write speed | Async batch writes in transactions. Critical events written synchronously (never lost). Info events batched (may lose last batch on crash). | Info events in the last 500ms batch can be lost on hard crash. Critical/fatal events are always persisted immediately. |
| Regex WAF | 8-layer normalization (URL, double-URL, HTML entities, Unicode, null bytes, whitespace, SQL comments, case). | Still pattern-based. Semantic attacks, logic-level API abuse, and novel zero-day payloads may bypass. Use as defense-in-depth, not sole protection. |
| Single-node | Cluster mode with TCP event sharing, leader election, distributed locks. | No Raft/Paxos consensus. Split-brain possible if network partitions. Clock drift can affect TTL locks. Leader death during action = potential duplicate heal. |
| Action timeout | 30-second timeout on all healing actions. | The goroutine running the timed-out action can't be killed in Go (no goroutine cancellation). It will eventually return or GC will collect it, but it lingers until then. |
| History growth | Capped at 10K (history + recommendations). | Eviction loses oldest entries. No persistent history beyond SQLite events table. |
| Priority queue | Critical events block-enqueue (never dropped). Info events drop under backpressure. | Under sustained extreme load, info events can starve completely. No aging/fairness mechanism — by design (survival > fairness). |
| Index growth | Indexes on type, source, severity for query speed. | Large event tables + indexes = write amplification + storage growth. Mitigated by retention policy, but operator must configure it. |

### Honest Trade-offs (Every System Has Them)

1. **Durability vs Speed:** Info events are batched (fast but lossy on crash). Critical events are sync (safe but slower). You choose the severity threshold.

2. **Fairness vs Survival:** Under extreme load, info events are sacrificed so critical events always get through. This is intentional — a self-healing engine that crashes under load is useless.

3. **Simplicity vs Distributed Consistency:** Cluster mode uses simple TCP + leader election. It's not Raft. Split-brain, network partitions, and clock drift are real risks at scale. For strong consistency, use an external coordination service (etcd, consul).

4. **Detection vs Evasion:** Regex + normalization catches 95%+ of known attack patterns. The remaining 5% requires AST-level parsing or ML-based behavioral analysis, which is a future roadmap item.

5. **Single Binary vs Scale:** The embedded SQLite + single binary architecture is the strength (zero deps, instant deploy) and the constraint (single-writer, single-node bottleneck). For >10K events/sec sustained, plan for external storage.

### How to Benchmark Yourself

```bash
# Run built-in benchmarks
go test -bench=. -benchmem ./internal/event/... ./internal/dna/...

# Run chaos tests (burst, sustained, backpressure)
go test -v -run "TestChaos" ./demo/...

# Run latency tests
go test -v -run "TestProduction_EventProcessingLatency" ./demo/...
go test -v -run "TestProduction_HealLatency" ./demo/...

# Run stability tests (memory, goroutines, GC)
go test -v -run "TestStability" ./demo/...
```
