package collector_test

import (
	"sync"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/collector"
	"github.com/immortal-engine/immortal/internal/event"
)

func TestMetricCollectorEmitsEvents(t *testing.T) {
	var received []*event.Event
	var mu sync.Mutex

	mc := collector.NewMetricCollector(100*time.Millisecond, func(e *event.Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	})

	mc.Start()
	defer mc.Stop()

	time.Sleep(350 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) < 2 {
		t.Errorf("expected at least 2 metric events, got %d", len(received))
	}

	found := false
	for _, e := range received {
		if e.Meta["cpu_percent"] != nil {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one event with cpu_percent metadata")
	}
}
