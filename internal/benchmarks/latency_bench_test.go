package benchmarks_test

import (
	"sort"
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/engine"
	"github.com/Nagendhra-web/Immortal/internal/event"
)

// BenchmarkIngest_Latency records per-event ingest latency and reports
// p50 / p95 / p99 via b.ReportMetric so they appear in bench output.
//
// Because eng.Ingest publishes asynchronously to an internal bus, the
// individual call takes only a few hundred ns. We measure latency of the
// Ingest call itself (the publish path) which is the caller-visible cost.
// We collect a batch of 10 000 individual timings per b.N iteration so the
// percentile slice is large enough to be meaningful.
func BenchmarkIngest_Latency(b *testing.B) {
	const batchSize = 10_000

	eng := newBenchEngine(b, engine.Config{
		ThrottleWindow: time.Nanosecond,
		DedupWindow:    time.Nanosecond,
	})

	// Pre-allocate timing slice
	latencies := make([]int64, 0, b.N*batchSize)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		for j := 0; j < batchSize; j++ {
			t0 := time.Now()
			eng.Ingest(event.New(event.TypeMetric, event.SeverityInfo, "latency-bench"))
			ns := time.Since(t0).Nanoseconds()
			// On Windows the timer may round to 0; clamp to 1 so sort is sensible
			if ns < 1 {
				ns = 1
			}
			latencies = append(latencies, ns)
		}
	}
	b.StopTimer()

	if len(latencies) == 0 {
		return
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	p50 := percentile(latencies, 50)
	p95 := percentile(latencies, 95)
	p99 := percentile(latencies, 99)

	b.ReportMetric(float64(p50), "p50_ns/op")
	b.ReportMetric(float64(p95), "p95_ns/op")
	b.ReportMetric(float64(p99), "p99_ns/op")

	// Also report in µs for readability
	b.ReportMetric(float64(p50)/1000, "p50_us/op")
	b.ReportMetric(float64(p99)/1000, "p99_us/op")
}

// percentile returns the value at the given percentile (0-100) from a sorted slice.
func percentile(sorted []int64, pct int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := (pct * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
