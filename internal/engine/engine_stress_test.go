package engine_test

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/engine"
	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/healing"
)

func TestEngineConcurrentIngest(t *testing.T) {
	e, err := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Millisecond, // fast for test
		DedupWindow:    time.Millisecond, // fast for test
	})
	if err != nil {
		t.Fatal(err)
	}

	var healed atomic.Int64
	e.AddRule(healing.Rule{
		Name:  "counter",
		Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(ev *event.Event) error {
			healed.Add(1)
			return nil
		},
	})

	e.Start()
	defer e.Stop()

	// Send unique events so throttle/dedup don't block them
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				time.Sleep(2 * time.Millisecond) // let throttle/dedup windows pass
				e.Ingest(event.New(event.TypeError, event.SeverityCritical,
					fmt.Sprintf("crash-%d-%d", n, j)).WithSource(fmt.Sprintf("svc-%d", n)))
			}
		}(i)
	}
	wg.Wait()
	time.Sleep(1 * time.Second)

	if healed.Load() < 10 {
		t.Errorf("expected at least 10 heals, got %d", healed.Load())
	}
}

func TestEngineMultipleRules(t *testing.T) {
	e, err := engine.New(engine.Config{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	var rule1Count, rule2Count atomic.Int64

	e.AddRule(healing.Rule{
		Name:   "severity-rule",
		Match:  healing.MatchSeverity(event.SeverityCritical),
		Action: func(ev *event.Event) error { rule1Count.Add(1); return nil },
	})
	e.AddRule(healing.Rule{
		Name:   "source-rule",
		Match:  healing.MatchSource("db"),
		Action: func(ev *event.Event) error { rule2Count.Add(1); return nil },
	})

	e.Start()
	defer e.Stop()

	// Only matches rule1
	e.Ingest(event.New(event.TypeError, event.SeverityCritical, "api crash").WithSource("api"))
	// Matches both
	e.Ingest(event.New(event.TypeError, event.SeverityCritical, "db crash").WithSource("db"))
	// Matches neither
	e.Ingest(event.New(event.TypeMetric, event.SeverityInfo, "metric").WithSource("system"))

	time.Sleep(500 * time.Millisecond)

	if rule1Count.Load() != 2 {
		t.Errorf("severity rule expected 2, got %d", rule1Count.Load())
	}
	if rule2Count.Load() != 1 {
		t.Errorf("source rule expected 1, got %d", rule2Count.Load())
	}
}

func TestEngineStartStopMultipleTimes(t *testing.T) {
	for i := 0; i < 5; i++ {
		e, err := engine.New(engine.Config{DataDir: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		e.Start()
		if !e.IsRunning() {
			t.Error("should be running")
		}
		e.Stop()
		if e.IsRunning() {
			t.Error("should be stopped")
		}
	}
}
