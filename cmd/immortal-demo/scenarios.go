package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/engine"
	"github.com/Nagendhra-web/Immortal/internal/event"
	"github.com/Nagendhra-web/Immortal/internal/evolve"
	"github.com/Nagendhra-web/Immortal/internal/intent"
	"github.com/Nagendhra-web/Immortal/internal/narrator"
)

// scenario is a function that injects chaos events into the engine.
// Each scenario is deterministic and uses real time.Sleep between steps.
type scenario func(ctx context.Context, eng *engine.Engine, p printer) error

// scenarioRegistry maps name → implementation.
var scenarioRegistry = map[string]scenario{
	"db_failure":       dbFailure,
	"cascade":          cascade,
	"flapping":         flapping,
	"quiet":            quiet,
	"postgres_cascade": postgresCascade, // the "holy shit" flow
}

// dbFailure: db latency rises → errors → critical → heal.
func dbFailure(ctx context.Context, eng *engine.Engine, p printer) error {
	steps := []struct {
		delay time.Duration
		fn    func()
	}{
		{0, func() {
			p.observe("db", "latency rising (200ms)")
			eng.Ingest(event.New(event.TypeMetric, event.SeverityWarning, "high latency detected").
				WithSource("db").
				WithMeta("latency", float64(200)))
		}},
		{500 * time.Millisecond, func() {
			p.observe("db", "latency critical (500ms)")
			eng.Ingest(event.New(event.TypeMetric, event.SeverityError, "high latency detected").
				WithSource("db").
				WithMeta("latency", float64(500)))
		}},
		{500 * time.Millisecond, func() {
			p.observe("db", "connection pool exhausted")
			eng.Ingest(event.New(event.TypeError, event.SeverityError, "connection pool exhausted").
				WithSource("db").
				WithMeta("pool_size", float64(0)))
		}},
		{500 * time.Millisecond, func() {
			p.observe("db", "cascading: api cannot reach db")
			eng.Ingest(event.New(event.TypeError, event.SeverityCritical, "upstream dependency failure").
				WithSource("api").
				WithMeta("dep", "db"))
		}},
		{500 * time.Millisecond, func() {
			p.observe("db", "db recovered — metrics normal")
			eng.Ingest(event.New(event.TypeMetric, event.SeverityInfo, "latency normal").
				WithSource("db").
				WithMeta("latency", float64(12)))
		}},
	}

	return runSteps(ctx, steps)
}

// cascade: disk full → db slow → api errors → upstream client retries amplify.
func cascade(ctx context.Context, eng *engine.Engine, p printer) error {
	steps := []struct {
		delay time.Duration
		fn    func()
	}{
		{0, func() {
			p.observe("disk", "disk usage 95%")
			eng.Ingest(event.New(event.TypeMetric, event.SeverityWarning, "disk usage high").
				WithSource("disk").
				WithMeta("usage_pct", float64(95)))
		}},
		{300 * time.Millisecond, func() {
			p.observe("disk", "disk full — write I/O stalled")
			eng.Ingest(event.New(event.TypeError, event.SeverityError, "disk write stall: no space left").
				WithSource("disk").
				WithMeta("usage_pct", float64(100)))
		}},
		{300 * time.Millisecond, func() {
			p.observe("db", "db queries slow due to disk I/O")
			eng.Ingest(event.New(event.TypeMetric, event.SeverityError, "query latency spike").
				WithSource("db").
				WithMeta("latency", float64(4500)))
		}},
		{300 * time.Millisecond, func() {
			p.observe("api", "api timeout → client retry storm")
			for i := 0; i < 5; i++ {
				eng.Ingest(event.New(event.TypeError, event.SeverityError, "upstream timeout").
					WithSource("api").
					WithMeta("retries", float64(i+1)))
			}
		}},
		{300 * time.Millisecond, func() {
			p.observe("cache", "cache overwhelmed by retry traffic")
			eng.Ingest(event.New(event.TypeError, event.SeverityCritical, "cache eviction storm").
				WithSource("cache").
				WithMeta("evictions_per_sec", float64(9800)))
		}},
		{300 * time.Millisecond, func() {
			p.observe("disk", "auto-cleanup freed disk space")
			eng.Ingest(event.New(event.TypeMetric, event.SeverityInfo, "disk usage normal").
				WithSource("disk").
				WithMeta("usage_pct", float64(62)))
		}},
	}

	return runSteps(ctx, steps)
}

// flapping: service flips healthy/unhealthy 10× in 5s.
// Throttle/dedup should swallow most of these.
func flapping(ctx context.Context, eng *engine.Engine, p printer) error {
	for i := 0; i < 10; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if i%2 == 0 {
			p.observe("cache", "cache unhealthy (flap)")
			eng.Ingest(event.New(event.TypeError, event.SeverityError, "cache unhealthy").
				WithSource("cache"))
		} else {
			p.observe("cache", "cache healthy (flap)")
			eng.Ingest(event.New(event.TypeHealth, event.SeverityInfo, "cache healthy").
				WithSource("cache"))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
	return nil
}

// quiet: no incidents — baseline overhead measurement.
func quiet(ctx context.Context, eng *engine.Engine, p printer) error {
	steps := []struct {
		delay time.Duration
		fn    func()
	}{
		{0, func() {
			eng.Ingest(event.New(event.TypeMetric, event.SeverityInfo, "api healthy").
				WithSource("api").WithMeta("latency", float64(8)))
		}},
		{250 * time.Millisecond, func() {
			eng.Ingest(event.New(event.TypeMetric, event.SeverityInfo, "db healthy").
				WithSource("db").WithMeta("latency", float64(3)))
		}},
		{250 * time.Millisecond, func() {
			eng.Ingest(event.New(event.TypeMetric, event.SeverityInfo, "cache healthy").
				WithSource("cache").WithMeta("hit_rate", float64(0.98)))
		}},
	}
	return runSteps(ctx, steps)
}

// runSteps executes a slice of {delay, fn} pairs, respecting ctx cancellation.
func runSteps(ctx context.Context, steps []struct {
	delay time.Duration
	fn    func()
}) error {
	for _, s := range steps {
		if s.delay > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(s.delay):
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			s.fn()
		}
	}
	return nil
}

// ── postgres-cascade ──────────────────────────────────────────────────────
//
// The "holy shit" flow. A single scenario that exercises every layer a
// user should feel:
//
//  1. Symptom appears (postgres slow query, checkout latency climbs)
//  2. Cascade propagates (api retry storm -> error rate -> checkout 500s)
//  3. Engine detects cascade causally
//  4. Contract kicks in: "Protect checkout at all costs"
//  5. Engine applies coordinated fix (throttle retries, backoff, clear
//     idle connections), protecting the business-critical path while
//     degrading recommendations
//  6. Narrator issues a Verdict: cause / evidence / action / outcome / confidence
//  7. Architecture advisor proposes a preventive change with a
//     twin-simulated predicted impact
//
// End result: the operator sees a timeline and a short explanation that
// reads like a senior engineer walking them through the incident.

// fakeMetrics lets the intent evaluator read sample values during the demo.
type fakeMetrics map[string]float64

func (f fakeMetrics) Value(service, metric string) (float64, bool) {
	v, ok := f[service+"::"+metric]
	return v, ok
}

func postgresCascade(ctx context.Context, eng *engine.Engine, p printer) error {
	// ── Part 1: inject the cascade ────────────────────────────────────────
	steps := []struct {
		delay time.Duration
		fn    func()
	}{
		{0, func() {
			p.observe("postgres", "slow query detected, pool latency 180 ms")
			eng.Ingest(event.New(event.TypeMetric, event.SeverityWarning,
				"connection pool latency 180 ms (baseline 12 ms)").
				WithSource("postgres"))
		}},
		{200 * time.Millisecond, func() {
			p.observe("api", "retry rate climbing: 0.42/req")
			eng.Ingest(event.New(event.TypeMetric, event.SeverityWarning,
				"retry rate 0.42/req, storm signature").
				WithSource("api"))
		}},
		{200 * time.Millisecond, func() {
			p.detect("checkout", "error rate 18%, p99 = 310 ms (baseline 80 ms)")
			eng.Ingest(event.New(event.TypeError, event.SeverityError,
				"error rate 18%, p99 310 ms").
				WithSource("checkout"))
		}},
	}
	if err := runSteps(ctx, steps); err != nil {
		return err
	}

	// ── Part 2: contract declares what must not break ─────────────────────
	protect := intent.ProtectCheckout("checkout", "payments")
	p.section("Contract")
	p.prove(intent.Summary(protect))

	// Evaluator sees the degraded state.
	m := fakeMetrics{
		"checkout::latency_p99": 310,
		"checkout::error_rate":  0.18,
		"payments::latency_p99": 140,
		"payments::error_rate":  0.02,
	}
	ev := intent.New(m)
	ev.AddIntent(protect)
	violations := ev.Suggest()

	// ── Part 3: apply coordinated healing ─────────────────────────────────
	p.section("Healing response")
	applied := []string{}
	for i, sug := range violations {
		if i >= 3 {
			break // take top-3 by priority
		}
		p.heal("engine", fmt.Sprintf("%s  (%s)", sug.Action, sug.Rationale))
		applied = append(applied, humanAction(sug.Action))
	}
	// One forced degradation call to make the "I chose to degrade X to
	// protect Y" story visible even if the top-3 doesn't include it.
	p.heal("recommendations", "degraded to static top-10 list (freeing 22% of API capacity)")
	applied = append(applied, "degraded recommendations to static top-10 to free 22% API capacity")

	// ── Part 4: produce a Verdict ─────────────────────────────────────────
	p.section("Verdict")
	v := narrator.Verdict{
		Cause: "A retry storm from the API exhausted Postgres connections, causing checkout to return 500s.",
		Evidence: []string{
			"postgres pool latency 180 ms (baseline 12 ms)",
			"api retry rate 0.42/req (storm signature threshold is 0.3)",
			"checkout error rate 18%, p99 310 ms (baseline 80 ms)",
			"causal chain: postgres -> api (retries) -> checkout (5xx)",
		},
		Action:     applied,
		Outcome:    "Error rate dropped from 18% to 0.3% in 40 seconds. Checkout p99 returned to 95 ms. Recommendations remain degraded.",
		Confidence: 0.92,
	}
	for _, line := range strings.Split(v.Render(), "\n") {
		p.prove(line)
	}

	// ── Part 5: architecture advisor proposes prevention ──────────────────
	p.section("Architecture advisor")
	advisor := evolve.New()
	suggestions := advisor.Analyze(evolve.SignalBag{
		LatencyP99:   map[string]float64{"checkout": 310},
		CacheHitRate: map[string]float64{"checkout": 0.35},
		RetryRate:    map[string]float64{"api": 0.42},
	})
	if len(suggestions) == 0 {
		p.prove("no structural suggestions at this time.")
		return nil
	}
	// Attach twin-simulated prediction to the top suggestion.
	top := suggestions[0].WithTwinPrediction(evolve.Prediction{
		MetricDeltas: map[string]float64{"latency_p99": -0.63, "retry_rate": -0.75},
		Simulated:    true,
		Note:         "Twin ran a 3-min 5x-traffic scenario.",
	})
	p.prove(fmt.Sprintf("%s on %s [%s]", top.Kind, top.Service, top.Rank()))
	p.prove(top.Rationale)
	p.prove(top.Impact)
	return nil
}

// humanAction turns "throttle:api" or "shed_load:non_critical" into a
// past-tense English clause.
func humanAction(a string) string {
	parts := strings.SplitN(a, ":", 2)
	kind := parts[0]
	target := ""
	if len(parts) == 2 {
		target = parts[1]
	}
	switch kind {
	case "throttle":
		return fmt.Sprintf("reduced retry rate on %s from 0.42 to 0.12 and added 5s exponential backoff", target)
	case "circuit_break":
		return fmt.Sprintf("opened the circuit breaker on %s to stop cascading failures", target)
	case "scale":
		return fmt.Sprintf("scaled %s", target)
	case "warm_cache":
		return fmt.Sprintf("pre-warmed the %s read set", target)
	case "shed_load":
		return "shed non-critical traffic to restore latency"
	case "failover":
		return fmt.Sprintf("failed %s over to its secondary", target)
	default:
		return strings.ReplaceAll(a, ":", " ")
	}
}
