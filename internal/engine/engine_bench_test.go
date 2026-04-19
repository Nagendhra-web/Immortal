package engine_test

import (
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/engine"
	"github.com/Nagendhra-web/Immortal/internal/event"
	"github.com/Nagendhra-web/Immortal/internal/healing"
)

func BenchmarkEngineIngest(b *testing.B) {
	eng, _ := engine.New(engine.Config{
		DataDir:        b.TempDir(),
		ThrottleWindow: time.Nanosecond,
		DedupWindow:    time.Nanosecond,
	})
	eng.Start()
	defer eng.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eng.Ingest(event.New(event.TypeMetric, event.SeverityInfo, "bench"))
	}
}

func BenchmarkEngineIngestWithHealing(b *testing.B) {
	eng, _ := engine.New(engine.Config{
		DataDir:        b.TempDir(),
		ThrottleWindow: time.Nanosecond,
		DedupWindow:    time.Nanosecond,
	})
	eng.AddRule(healing.Rule{
		Name:   "bench-rule",
		Match:  healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error { return nil },
	})
	eng.Start()
	defer eng.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eng.Ingest(event.New(event.TypeError, event.SeverityCritical, "bench-crash"))
	}
}
