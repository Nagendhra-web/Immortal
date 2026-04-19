package benchmarks_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/engine"
	"github.com/Nagendhra-web/Immortal/internal/event"
)

// BenchmarkMemory_100kEvents measures the heap allocation delta while ingesting
// exactly 100 000 metric events. It reports HeapAlloc delta in MB.
func BenchmarkMemory_100kEvents(b *testing.B) {
	const N = 100_000

	for i := 0; i < b.N; i++ {
		eng := newBenchEngine(b, engine.Config{
			ThrottleWindow: time.Nanosecond,
			DedupWindow:    time.Nanosecond,
		})

		// Force GC + get baseline
		runtime.GC()
		var before runtime.MemStats
		runtime.ReadMemStats(&before)

		for j := 0; j < N; j++ {
			eng.Ingest(event.New(event.TypeMetric, event.SeverityInfo, "mem-bench").
				WithMeta("val", float64(j)))
		}

		// Give the async bus time to drain
		time.Sleep(200 * time.Millisecond)

		runtime.GC()
		var after runtime.MemStats
		runtime.ReadMemStats(&after)

		deltaBytes := int64(after.HeapAlloc) - int64(before.HeapAlloc)
		if deltaBytes < 0 {
			deltaBytes = 0
		}
		deltaMB := float64(deltaBytes) / (1024 * 1024)
		b.ReportMetric(deltaMB, "heap_delta_MB")
		b.ReportMetric(float64(after.TotalAlloc-before.TotalAlloc)/(1024*1024), "total_alloc_MB")
	}
}
