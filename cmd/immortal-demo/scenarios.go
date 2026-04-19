package main

import (
	"context"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/engine"
	"github.com/Nagendhra-web/Immortal/internal/event"
)

// scenario is a function that injects chaos events into the engine.
// Each scenario is deterministic and uses real time.Sleep between steps.
type scenario func(ctx context.Context, eng *engine.Engine, p printer) error

// scenarioRegistry maps name → implementation.
var scenarioRegistry = map[string]scenario{
	"db_failure": dbFailure,
	"cascade":    cascade,
	"flapping":   flapping,
	"quiet":      quiet,
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
