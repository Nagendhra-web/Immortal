package benchmarks_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/engine"
	"github.com/Nagendhra-web/Immortal/internal/event"
	"github.com/Nagendhra-web/Immortal/internal/healing"
)

// newBenchEngine creates a minimal engine tuned for throughput benchmarks.
// ThrottleWindow and DedupWindow are set to 1ns so every event is processed.
func newBenchEngine(b *testing.B, opts engine.Config) *engine.Engine {
	b.Helper()
	opts.DataDir = b.TempDir()
	if opts.ThrottleWindow == 0 {
		opts.ThrottleWindow = time.Nanosecond
	}
	if opts.DedupWindow == 0 {
		opts.DedupWindow = time.Nanosecond
	}
	if opts.ConsensusMin == 0 {
		opts.ConsensusMin = 1
	}
	eng, err := engine.New(opts)
	if err != nil {
		b.Fatalf("engine.New: %v", err)
	}
	if err := eng.Start(); err != nil {
		b.Fatalf("eng.Start: %v", err)
	}
	b.Cleanup(func() { eng.Stop() })
	return eng
}

// BenchmarkIngest_Sustained measures raw ingest throughput with info-level
// metric events. No healing rules fire. Targets ≥100k events/sec.
func BenchmarkIngest_Sustained(b *testing.B) {
	eng := newBenchEngine(b, engine.Config{})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		eng.Ingest(event.New(event.TypeMetric, event.SeverityInfo, "bench-metric"))
	}
	b.StopTimer()

	eventsPerSec := float64(b.N) / b.Elapsed().Seconds()
	b.ReportMetric(eventsPerSec, "events/sec")
}

// BenchmarkIngest_WithHealing fires b.N events with a 10%/90% critical/info mix.
// One healing rule matches Critical events and executes a no-op action.
func BenchmarkIngest_WithHealing(b *testing.B) {
	eng := newBenchEngine(b, engine.Config{})
	eng.AddRule(healing.Rule{
		Name:   "bench-heal-critical",
		Match:  healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error { return nil },
	})

	infoMsg := "bench-metric-info"
	critMsg := "bench-metric-critical"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if i%10 == 0 {
			eng.Ingest(event.New(event.TypeError, event.SeverityCritical, critMsg))
		} else {
			eng.Ingest(event.New(event.TypeMetric, event.SeverityInfo, infoMsg))
		}
	}
	b.StopTimer()

	eventsPerSec := float64(b.N) / b.Elapsed().Seconds()
	b.ReportMetric(eventsPerSec, "events/sec")
}

// BenchmarkIngest_AllAdvancedEnabled enables PQAudit + Twin + Causal + Topology
// + Federated and measures the throughput drop vs BenchmarkIngest_Sustained.
// Expected: noticeable drop; document the ratio in README.md.
func BenchmarkIngest_AllAdvancedEnabled(b *testing.B) {
	eng := newBenchEngine(b, engine.Config{
		EnablePQAudit:     true,
		EnableTwin:        true,
		EnableAgentic:     true,
		EnableCausal:      true,
		EnableTopology:    true,
		EnableFormal:      true,
		FederatedClientID: fmt.Sprintf("bench-node-%d", time.Now().UnixNano()),
	})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		eng.Ingest(event.New(event.TypeMetric, event.SeverityInfo, "bench-advanced").
			WithMeta("latency", float64(i%100)))
	}
	b.StopTimer()

	eventsPerSec := float64(b.N) / b.Elapsed().Seconds()
	b.ReportMetric(eventsPerSec, "events/sec")
}
