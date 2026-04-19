package event_test

import (
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/event"
)

func BenchmarkEventNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		event.New(event.TypeError, event.SeverityError, "benchmark event")
	}
}

func BenchmarkEventWithMeta(b *testing.B) {
	e := event.New(event.TypeError, event.SeverityError, "bench")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.WithMeta("key", i)
	}
}
