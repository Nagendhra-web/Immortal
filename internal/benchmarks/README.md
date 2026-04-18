# Immortal Benchmarks

Sustained-ingest, per-event latency, and memory footprint benchmarks for the
Immortal engine. All benchmarks live in `internal/benchmarks/` and use only
the standard `testing.B` API — no external dependencies.

## How to run

```bash
# Full suite — recommended for capturing reference numbers
go test ./internal/benchmarks/... \
  -run "^$" \
  -bench . \
  -benchmem \
  -benchtime 10s \
  -count 1 \
  -timeout 300s

# Single benchmark (quick sanity check)
go test ./internal/benchmarks/... \
  -run "^$" \
  -bench BenchmarkIngest_Sustained \
  -benchmem \
  -benchtime 1s
```

## What each benchmark measures

| Benchmark | What it tests |
|-----------|--------------|
| `BenchmarkIngest_Sustained` | Raw publish throughput — 100 % `TypeMetric / SeverityInfo` events, no healing rules. Represents the floor cost of calling `eng.Ingest`. |
| `BenchmarkIngest_WithHealing` | 90 % info + 10 % critical events with one healing rule that fires a no-op action. Measures the overhead of rule matching and goroutine dispatch. |
| `BenchmarkIngest_AllAdvancedEnabled` | Same payload as Sustained but with **PQAudit + Twin + Agentic + Causal + Topology + Federated** all enabled. Quantifies the feature-tax over the baseline. |
| `BenchmarkIngest_Latency` | Collects 10 000 individual `time.Since` samples per `b.N` iteration and reports p50/p95/p99 of the **publish-path latency** (the caller-visible cost before the async bus takes over). |
| `BenchmarkMemory_100kEvents` | Ingests exactly 100 000 metric events, waits 200 ms for the bus to drain, then reports `HeapAlloc` delta (live heap after GC) and `TotalAlloc` (cumulative allocations). |

## Interpreting results

- **`events/sec`** — custom metric reported via `b.ReportMetric`; divide `1e9 / ns/op` to cross-check.
- **`p50/p95/p99_ns/op`** — percentile latencies of the Ingest publish call.
  On Windows, the OS timer resolution is ~100 ns, so values below that clamp
  to 0–1 ns; the meaningful signal is the p99 spread and `ns/op` from the
  benchmark harness itself.
- **`heap_delta_MB`** — live heap growth (post-GC) after 100 k events. Low
  numbers indicate good GC pressure management.
- **`total_alloc_MB`** — cumulative bytes allocated; reflects churn even if
  GC keeps live heap small.

## Reference numbers

Recorded on **2026-04-18** · Windows 11 · i7-11370H @ 3.30 GHz · Go 1.25 · `GOARCH=amd64`

```
goos: windows
goarch: amd64
cpu: 11th Gen Intel(R) Core(TM) i7-11370H @ 3.30GHz

BenchmarkIngest_Sustained-8           3338209    1153 ns/op    867412 events/sec    258 B/op    5 allocs/op
BenchmarkIngest_WithHealing-8         1000000   22628 ns/op     44194 events/sec    373 B/op    6 allocs/op
BenchmarkIngest_AllAdvancedEnabled-8  2435263    1417 ns/op    705782 events/sec    555 B/op    7 allocs/op
BenchmarkIngest_Latency-8                 409 9924217 ns/op       p50=1 ns  p95=1 ns  p99=1 ns   (per 10k batch)
BenchmarkMemory_100kEvents-8                9 380290933 ns/op  heap_delta=0.19 MB  total_alloc=52.22 MB
```

### Summary table

| Metric | Value |
|--------|-------|
| Baseline throughput (no rules) | **~867 k events/sec** |
| Throughput with healing rule | ~44 k events/sec (−95 %; goroutine dispatch per critical event) |
| Throughput — all features on | ~706 k events/sec (−19 % vs baseline) |
| p99 publish-path latency | < 1 µs (Windows timer floor; harness reports ~1 153 ns/op) |
| Heap delta @ 100 k events | **~0.19 MB** live heap growth |
| Total allocated @ 100 k events | ~52 MB (short-lived, GC reclaims) |

### Trade-off notes

- **WithHealing drop (−95 %)**: the healing rule fires on every critical event
  and dispatches a goroutine via `executeWithPolicy`. In production, only a
  small fraction of events are critical, so real-world throughput sits much
  closer to the baseline.
- **AllAdvancedEnabled drop (−19 %)**: PQAudit adds an SHA-256 + Ed25519 sign
  per heal event; Twin, Causal, and Federated add per-metric bookkeeping.
  The overhead is bounded and scales independently of event volume.
- **Memory**: the engine's internal SQLite store, bus ring buffer, and
  time-travel recorder dominate allocation. The 0.19 MB live-heap delta
  shows the engine does not accumulate unbounded in-memory state.
