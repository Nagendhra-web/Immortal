# How Immortal's anomaly detection actually works

*3-sigma is the baseline. Making it useful in production took the other 90%.*

The `internal/dna` package is the thing that decides whether a metric value you just observed is worth paying attention to. Every other feature in Immortal downstream of that decision (healing rules, predictive alerts, incident open, agentic loop trigger) starts with DNA saying "this is not normal".

The short version: per-service baselines, Welford online stats, and a 3-sigma cutoff with a couple of real-world adjustments. This post walks through why those choices, where the math helps and where it actively lies, and what we had to add on top.

## The problem

You have a metric stream: request latency, error rate, CPU, queue depth. You want to know when "this value is weird enough that something is probably breaking".

The naive answer is a fixed threshold. "Alert if latency over 500 ms". You will run into two problems inside a month:

1. **The threshold is always wrong somewhere.** Services that run at 600 ms baseline flood you. Services that run at 40 ms baseline never fire until they are already on fire.
2. **The baseline drifts.** Your service is not faster or slower today than it was in January by a fixed amount. It depends on what day, what traffic, what downstream is degraded. A static threshold ignores all of that.

The second answer is percentile-over-window. "Alert if p99 exceeds the p99 of the last 7 days by 2x". Better, but you pay for it in compute and storage, and you lose the ability to react to the first minute of a new incident because the window has not moved enough yet.

We wanted something between those two: learns the baseline automatically, reacts fast, is cheap.

## The 3-sigma rule

If you assume a metric is roughly normally distributed, then 99.7% of observations fall within three standard deviations of the mean. Anything outside that range is "anomalous" with 0.3% probability under the assumption.

In Go:

```go
func (d *DNA) IsAnomaly(metric string, value float64) bool {
    b, ok := d.baselines[metric]
    if !ok {
        return false // no baseline yet
    }
    return math.Abs(value - b.Mean) > 3 * b.StdDev
}
```

That is it. In production we learned to lean on two tricks: how you compute `Mean` and `StdDev`, and when you refuse to fire.

## Welford's algorithm for online mean + variance

The textbook way to get a running mean + variance is:

```
sum += x
sumSq += x * x
mean = sum / n
variance = (sumSq / n) - mean * mean
```

Two issues. First, `sum` and `sumSq` grow without bound. After a few million samples on a running service, you are looking at precision loss. Second, the `sumSq - mean*mean` formula has catastrophic cancellation when the variance is small relative to the mean (which is almost always the case for latency metrics).

Welford's algorithm avoids both:

```go
func (b *Baseline) Record(x float64) {
    b.n++
    delta := x - b.mean
    b.mean += delta / float64(b.n)
    delta2 := x - b.mean
    b.m2 += delta * delta2
}

func (b *Baseline) Variance() float64 {
    if b.n < 2 {
        return 0
    }
    return b.m2 / float64(b.n - 1)
}
```

Mean updates with a running delta, so it never blows up. Variance is accumulated in `m2` which is an aggregate of squared deviations from the current mean, not from zero. Precision stays stable indefinitely.

We use Welford on every metric Immortal tracks. Zero extra storage beyond three `float64`s and one `int64` per metric per service.

## Warm-up period

The rule "more than 3 sigma is anomalous" only works if the baseline is trustworthy. The first sample has no baseline at all. The second has a sample size of 2 and a variance of roughly zero, which makes 3-sigma ridiculously sensitive. The fifth sample has a variance that might be accurate but might still be wildly off.

Rule: do not fire anomalies until you have observed at least 30 samples. That is a classical CLT heuristic; the sample mean starts behaving like a normal distribution around sample size 30 for most real-world metrics.

```go
func (d *DNA) IsAnomaly(metric string, value float64) bool {
    b, ok := d.baselines[metric]
    if !ok || b.n < 30 {
        return false
    }
    return math.Abs(value - b.Mean) > 3 * b.StdDev
}
```

This means the first 30 requests after an engine restart cannot fire an anomaly. That is a feature: it prevents a flurry of false positives every time someone redeploys. In practice 30 samples is typically under a second on any metric worth watching.

## Per-service baselines

One baseline per metric per service, not one baseline per metric. Latency on `checkout` and latency on `auth` are different distributions. Mixing them produces a baseline that describes neither.

```go
type DNA struct {
    mu        sync.RWMutex
    baselines map[string]*Baseline // key is "service::metric"
}
```

We hash on service + metric. Memory overhead is maybe 40 bytes per baseline. Even a service with 100 metrics and 50 downstreams ends up in the single-digit megabytes.

## The drift problem

Welford assumes the underlying distribution is stationary. Production distributions are not. Your latency drifts up as traffic grows. Your error rate drifts down as you fix bugs. The static 3-sigma band becomes wrong over time.

We handle this with an exponentially-weighted variant. Instead of recording every sample with equal weight, newer samples are weighted more heavily than older ones:

```go
func (b *Baseline) RecordEW(x float64, alpha float64) {
    b.n++
    delta := x - b.mean
    b.mean = (1 - alpha) * b.mean + alpha * x
    b.m2 = (1 - alpha) * b.m2 + alpha * delta * delta
}
```

`alpha` is typically 0.01, giving a characteristic window of roughly 100 samples. The half-life is adjustable per metric.

Trade-off: fast adaptation means a genuine incident that goes on for more than a few minutes will eventually be absorbed into the baseline, and the engine will stop flagging it. That is why DNA is one signal, not the only signal. The `pattern`, `correlation`, and `predict` packages feed from it but apply their own longer-memory filters.

## What 3-sigma misses

Three-sigma is good at detecting "this value is far from the normal range". It is bad at:

- **Bimodal distributions.** If the service sometimes responds in 50 ms and sometimes in 500 ms, the mean is 275 with a huge variance, and both modes are "normal". 3-sigma will under-trigger.
- **Slow creeps.** A latency that drifts from 80 ms to 130 ms over two hours never violates 3-sigma in the moment, but the cumulative drift is a real incident. `internal/predict` handles this with linear regression.
- **Compound shifts.** Latency alone is fine. Error rate alone is fine. Both moving together is alarming. `internal/correlation` handles this with Pearson correlation of per-metric streams.

DNA is one line in the observability pipeline. It is the cheapest, fastest filter. Everything heavier runs on top.

## Code you can actually read

```go
// internal/dna/dna.go
func (d *DNA) IsAnomaly(metric string, value float64) bool {
    d.mu.RLock()
    defer d.mu.RUnlock()
    b, ok := d.baselines[metric]
    if !ok || b.n < 30 {
        return false
    }
    if b.StdDev == 0 {
        return false
    }
    return math.Abs(value - b.Mean) > 3 * b.StdDev
}

func (d *DNA) Record(metric string, value float64) {
    d.mu.Lock()
    defer d.mu.Unlock()
    b, ok := d.baselines[metric]
    if !ok {
        b = &Baseline{}
        d.baselines[metric] = b
    }
    b.Record(value)
}

func (d *DNA) HealthScore(current map[string]float64) float64 {
    // Score of 1.0 means every metric is within 1 sigma of its baseline.
    // Score of 0.0 means every metric is outside 3 sigma.
    d.mu.RLock()
    defer d.mu.RUnlock()
    if len(current) == 0 {
        return 1.0
    }
    total := 0.0
    for metric, value := range current {
        b, ok := d.baselines[metric]
        if !ok || b.n < 30 || b.StdDev == 0 {
            total += 1.0
            continue
        }
        z := math.Abs(value - b.Mean) / b.StdDev
        score := 1.0 - math.Min(z / 3.0, 1.0)
        total += score
    }
    return total / float64(len(current))
}
```

One mutex, one map, three formulas. The whole package is 186 lines.

## What is next

The layer above DNA is `internal/causal` (PC algorithm plus do-calculus), which turns "these two things are correlated" into "this caused that". That is a much longer post. Subscribe for it if you like the direction.

If you want to poke at DNA yourself, it is standalone:

```go
import "github.com/Nagendhra-web/Immortal/internal/dna"

d := dna.New("api-server")
for i := 0; i < 100; i++ {
    d.Record("latency_ms", 120.0 + rand.Float64() * 30)
}
d.IsAnomaly("latency_ms", 500.0) // true
d.IsAnomaly("latency_ms", 125.0) // false

score := d.HealthScore(map[string]float64{"latency_ms": 500.0})
// score around 0.15 — something is very wrong
```

No external dependencies. No registration. Works offline. The whole thing fits in your head.
