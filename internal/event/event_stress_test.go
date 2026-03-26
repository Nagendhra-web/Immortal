package event_test

import (
	"sync"
	"testing"

	"github.com/immortal-engine/immortal/internal/event"
)

func TestEventIDsAreUnique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 10000; i++ {
		e := event.New(event.TypeError, event.SeverityError, "test")
		if ids[e.ID] {
			t.Fatalf("duplicate ID found at iteration %d: %s", i, e.ID)
		}
		ids[e.ID] = true
	}
}

func TestEventConcurrentCreation(t *testing.T) {
	var wg sync.WaitGroup
	ids := sync.Map{}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				e := event.New(event.TypeError, event.SeverityError, "concurrent")
				if _, loaded := ids.LoadOrStore(e.ID, true); loaded {
					t.Errorf("duplicate ID in concurrent creation: %s", e.ID)
				}
			}
		}()
	}
	wg.Wait()
}

func TestEventMetaConcurrent(t *testing.T) {
	e := event.New(event.TypeError, event.SeverityError, "test")
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			e.WithMeta("key", n)
		}(i)
	}
	wg.Wait()
	// Should not panic
}

func TestAllEventTypes(t *testing.T) {
	types := []event.Type{event.TypeError, event.TypeMetric, event.TypeLog, event.TypeTrace, event.TypeHealth}
	for _, typ := range types {
		e := event.New(typ, event.SeverityInfo, "test")
		if e.Type != typ {
			t.Errorf("expected type %s, got %s", typ, e.Type)
		}
	}
}

func TestAllSeverityLevels(t *testing.T) {
	severities := []event.Severity{
		event.SeverityDebug, event.SeverityInfo, event.SeverityWarning,
		event.SeverityError, event.SeverityCritical, event.SeverityFatal,
	}
	for i, s := range severities {
		if s.Level() != i {
			t.Errorf("severity %s expected level %d, got %d", s, i, s.Level())
		}
	}
}

func TestEventWithEmptyMessage(t *testing.T) {
	e := event.New(event.TypeError, event.SeverityError, "")
	if e.Message != "" {
		t.Error("expected empty message")
	}
	if e.ID == "" {
		t.Error("ID should still be generated")
	}
}

func TestEventChaining(t *testing.T) {
	e := event.New(event.TypeError, event.SeverityError, "test").
		WithSource("src").
		WithMeta("a", 1).
		WithMeta("b", "two").
		WithMeta("c", true)

	if e.Source != "src" {
		t.Error("source not set")
	}
	if len(e.Meta) != 3 {
		t.Errorf("expected 3 meta entries, got %d", len(e.Meta))
	}
}

func TestUnknownSeverityLevel(t *testing.T) {
	s := event.Severity("unknown")
	if s.Level() != -1 {
		t.Errorf("unknown severity should return -1, got %d", s.Level())
	}
}
