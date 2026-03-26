package engine_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/engine"
	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/healing"
)

func TestEngineStartStop(t *testing.T) {
	cfg := engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Millisecond,
		DedupWindow:    time.Millisecond,
	}

	e, err := engine.New(cfg)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	err = e.Start()
	if err != nil {
		t.Fatalf("failed to start engine: %v", err)
	}

	if !e.IsRunning() {
		t.Error("expected engine to be running")
	}

	err = e.Stop()
	if err != nil {
		t.Fatalf("failed to stop engine: %v", err)
	}
}

func TestEngineProcessesEvents(t *testing.T) {
	cfg := engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Millisecond,
		DedupWindow:    time.Millisecond,
	}

	e, err := engine.New(cfg)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	var healed bool
	var mu sync.Mutex

	e.AddRule(healing.Rule{
		Name:  "test-rule",
		Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(ev *event.Event) error {
			mu.Lock()
			healed = true
			mu.Unlock()
			return nil
		},
	})

	e.Start()
	defer e.Stop()

	time.Sleep(5 * time.Millisecond)
	e.Ingest(event.New(event.TypeError, event.SeverityCritical, "test crash"))
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if !healed {
		t.Error("expected healing action to execute")
	}
}

func TestEngineGhostMode(t *testing.T) {
	cfg := engine.Config{
		DataDir:        t.TempDir(),
		GhostMode:      true,
		ThrottleWindow: time.Millisecond,
		DedupWindow:    time.Millisecond,
	}

	e, err := engine.New(cfg)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	actionCalled := false
	e.AddRule(healing.Rule{
		Name:  "ghost-test",
		Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(ev *event.Event) error {
			actionCalled = true
			return nil
		},
	})

	e.Start()
	defer e.Stop()

	time.Sleep(5 * time.Millisecond)
	e.Ingest(event.New(event.TypeError, event.SeverityCritical, "crash"))
	time.Sleep(200 * time.Millisecond)

	if actionCalled {
		t.Error("ghost mode should not execute actions")
	}

	recs := e.Recommendations()
	if len(recs) == 0 {
		t.Error("ghost mode should produce recommendations")
	}
}

func TestEngineFullPipeline(t *testing.T) {
	e, err := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Millisecond, // fast for tests
		DedupWindow:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	var healed sync.WaitGroup
	healed.Add(1)
	e.AddRule(healing.Rule{
		Name:  "test-full-pipeline",
		Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(ev *event.Event) error {
			healed.Done()
			return nil
		},
	})

	e.Start()
	defer e.Stop()

	// Wait for throttle/dedup windows to pass
	time.Sleep(5 * time.Millisecond)

	e.Ingest(event.New(event.TypeError, event.SeverityCritical, "full pipeline test").WithSource("api"))

	done := make(chan struct{})
	go func() { healed.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pipeline didn't heal within timeout")
	}
}

func TestEngineThrottling(t *testing.T) {
	e, err := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Second,
		DedupWindow:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	healCount := int64(0)
	e.AddRule(healing.Rule{
		Name:  "counter",
		Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(ev *event.Event) error {
			atomic.AddInt64(&healCount, 1)
			return nil
		},
	})

	e.Start()
	defer e.Stop()

	// Send same event 10 times rapidly — throttle should block most
	for i := 0; i < 10; i++ {
		e.Ingest(event.New(event.TypeError, event.SeverityCritical, "same crash").WithSource("api"))
	}
	time.Sleep(500 * time.Millisecond)

	if atomic.LoadInt64(&healCount) > 2 {
		t.Errorf("throttle should prevent repeated heals, got %d", healCount)
	}
}

func TestEngineMetricsExport(t *testing.T) {
	e, err := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Millisecond,
		DedupWindow:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	e.Start()
	defer e.Stop()

	time.Sleep(5 * time.Millisecond)
	e.Ingest(event.New(event.TypeError, event.SeverityError, "test export").WithSource("svc"))
	time.Sleep(200 * time.Millisecond)

	output := e.Exporter().Export()
	if output == "" {
		t.Error("expected prometheus metrics output")
	}
}
