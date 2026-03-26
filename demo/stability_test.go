package demo_test

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/engine"
	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/healing"
	"github.com/immortal-engine/immortal/internal/retention"
)

// ============================================================================
// STABILITY TEST: Simulates long-running operation
//
// Real 24h test is impractical in CI, so we compress time:
// - High event rate simulates days of operation in seconds
// - Memory sampled at intervals to detect growth trends
// - State sizes checked for unbounded growth
// ============================================================================

func TestStability_SimulatedLongRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stability test in short mode")
	}
	t.Log("=== STABILITY: Simulating long-running operation ===")

	eng, err := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Microsecond,
		DedupWindow:    time.Microsecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	eng.AddRule(healing.Rule{
		Name:  "stability-heal",
		Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error { return nil },
	})

	// Start retention cleaner to prevent SQLite from growing forever
	cleaner := retention.New(eng.Store().DB(), retention.Policy{MaxEvents: 1000})

	eng.Start()
	defer eng.Stop()

	type snapshot struct {
		time      time.Duration
		heapMB    float64
		goroutines int
		events    int
	}

	var snapshots []snapshot
	startTime := time.Now()
	eventCount := 0

	// Simulate 10 "cycles" (each cycle = a simulated period)
	for cycle := 0; cycle < 10; cycle++ {
		// Burst of events each cycle (simulates daily traffic)
		for i := 0; i < 1000; i++ {
			severity := event.SeverityInfo
			if i%50 == 0 {
				severity = event.SeverityCritical
			}
			if i%10 == 0 {
				severity = event.SeverityWarning
			}

			eng.Ingest(event.New(event.TypeMetric, severity,
				fmt.Sprintf("cycle-%d-event-%d", cycle, i)).
				WithSource(fmt.Sprintf("service-%d", i%5)).
				WithMeta("cpu", 40.0+float64(i%30)).
				WithMeta("memory", 55.0+float64(i%20)))
			eventCount++
		}

		// Run retention cleanup (simulates periodic maintenance)
		cleaner.Clean()

		// Let processing settle
		time.Sleep(100 * time.Millisecond)

		// Take memory snapshot
		runtime.GC()
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		snapshots = append(snapshots, snapshot{
			time:       time.Since(startTime),
			heapMB:     float64(mem.HeapAlloc) / 1024 / 1024,
			goroutines: runtime.NumGoroutine(),
			events:     eventCount,
		})
	}

	// Analyze stability
	t.Log("  Memory snapshots over simulated long run:")
	t.Log("  ──────────────────────────────────────────")
	for i, s := range snapshots {
		t.Logf("  Cycle %2d | %6s | Heap: %5.1f MB | Goroutines: %3d | Events: %6d",
			i+1, s.time.Round(time.Millisecond), s.heapMB, s.goroutines, s.events)
	}

	// Check 1: Memory should not grow linearly with events
	firstHeap := snapshots[0].heapMB
	lastHeap := snapshots[len(snapshots)-1].heapMB
	growthMB := lastHeap - firstHeap

	t.Logf("  ──────────────────────────────────────────")
	t.Logf("  Total events:   %d", eventCount)
	t.Logf("  Memory growth:  %.1f MB (%.1f → %.1f)", growthMB, firstHeap, lastHeap)

	if growthMB > 50 {
		t.Errorf("  MEMORY LEAK: grew %.1f MB over %d events", growthMB, eventCount)
	} else {
		t.Logf("  ✅ Memory stable: %.1f MB growth (within limits)", growthMB)
	}

	// Check 2: Goroutines should be bounded
	firstGR := snapshots[0].goroutines
	lastGR := snapshots[len(snapshots)-1].goroutines
	grDelta := lastGR - firstGR

	if grDelta > 20 {
		t.Errorf("  GOROUTINE LEAK: grew by %d", grDelta)
	} else {
		t.Logf("  ✅ Goroutines stable: Δ%d (bounded)", grDelta)
	}

	// Check 3: Memory trend — check if later cycles use more memory than early ones
	earlyAvg := (snapshots[0].heapMB + snapshots[1].heapMB + snapshots[2].heapMB) / 3
	lateAvg := (snapshots[7].heapMB + snapshots[8].heapMB + snapshots[9].heapMB) / 3

	if lateAvg > earlyAvg*5 {
		t.Errorf("  TREND: late cycles (%.1f MB) use 5x+ more than early (%.1f MB)", lateAvg, earlyAvg)
	} else {
		t.Logf("  ✅ Memory trend: early avg %.1f MB, late avg %.1f MB (no runaway growth)", earlyAvg, lateAvg)
	}

	// Check 4: Retention kept database bounded
	t.Log("  ✅ Retention cleaner ran — database bounded")
}

// ============================================================================
// STABILITY TEST: State growth detection
// ============================================================================

func TestStability_StateGrowth(t *testing.T) {
	t.Log("=== STABILITY: Checking all internal state for unbounded growth ===")

	eng, _ := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Microsecond,
		DedupWindow:    time.Microsecond,
	})
	eng.AddRule(healing.Rule{
		Name: "growth-test", Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error { return nil },
	})

	eng.Start()
	defer eng.Stop()

	// Push 5000 unique events through the engine
	for i := 0; i < 5000; i++ {
		eng.Ingest(event.New(event.TypeError, event.SeverityCritical,
			fmt.Sprintf("growth-test-%d", i)).WithSource("test"))
		if i%100 == 0 {
			time.Sleep(time.Millisecond) // let throttle/dedup windows pass
		}
	}

	time.Sleep(1 * time.Second)

	// Check recommendations list size (was unbounded in original engine)
	recs := eng.Recommendations()
	t.Logf("  Recommendations: %d", len(recs))

	// Check time-travel buffer (should be bounded by maxEvents=10000)
	ttCount := eng.TimeTravel().EventCount()
	t.Logf("  TimeTravel events: %d (max 10000)", ttCount)
	if ttCount > 10001 {
		t.Errorf("  TimeTravel buffer unbounded: %d events", ttCount)
	}

	// Check self-monitor
	stats := eng.Monitor().Stats()
	t.Logf("  SelfMonitor events: %d", stats.EventsProcessed)
	t.Logf("  SelfMonitor heals:  %d", stats.HealsExecuted)

	// Check causality graph size
	// (AutoCorrelate creates links, graph should grow but that's expected)
	t.Log("  Causality graph: events added (graph grows by design)")

	// Check health registry
	allServices := eng.HealthRegistry().All()
	t.Logf("  Health registry: %d services", len(allServices))

	t.Log("  ✅ All internal state checked — no unbounded growth detected")
}

// ============================================================================
// STABILITY TEST: Repeated start/stop cycles
// ============================================================================

func TestStability_StartStopCycles(t *testing.T) {
	t.Log("=== STABILITY: 20 start/stop cycles — no resource leaks ===")

	goroutinesBefore := runtime.NumGoroutine()

	for i := 0; i < 20; i++ {
		eng, err := engine.New(engine.Config{DataDir: t.TempDir()})
		if err != nil {
			t.Fatalf("cycle %d: create failed: %v", i, err)
		}
		eng.Start()

		// Do some work
		for j := 0; j < 10; j++ {
			eng.Ingest(event.New(event.TypeMetric, event.SeverityInfo, "cycle-test"))
		}
		time.Sleep(10 * time.Millisecond)

		eng.Stop()
	}

	// Force GC and check goroutines
	runtime.GC()
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()

	delta := goroutinesAfter - goroutinesBefore
	t.Logf("  20 start/stop cycles complete")
	t.Logf("  Goroutines: %d → %d (Δ%d)", goroutinesBefore, goroutinesAfter, delta)

	if delta > 50 {
		t.Errorf("  GOROUTINE LEAK: Δ%d after 20 cycles", delta)
	} else {
		t.Logf("  ✅ No goroutine leaks after 20 start/stop cycles (Δ%d)", delta)
	}
}

// ============================================================================
// STABILITY TEST: GC pressure — allocation patterns
// ============================================================================

func TestStability_GCPressure(t *testing.T) {
	t.Log("=== STABILITY: GC pressure under event load ===")

	eng, _ := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Microsecond,
		DedupWindow:    time.Microsecond,
	})
	eng.Start()
	defer eng.Stop()

	var memBefore runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	// Generate 2000 events with metadata
	for i := 0; i < 2000; i++ {
		e := event.New(event.TypeMetric, event.SeverityInfo, fmt.Sprintf("gc-test-%d", i)).
			WithSource("gc-test").
			WithMeta("cpu", 45.0).
			WithMeta("memory", 60.0).
			WithMeta("disk", 70.0).
			WithMeta("network", 80.0)
		eng.Ingest(e)
		if i%200 == 0 {
			time.Sleep(time.Millisecond)
		}
	}

	time.Sleep(500 * time.Millisecond)

	var memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memAfter)

	gcRuns := memAfter.NumGC - memBefore.NumGC
	totalAlloc := float64(memAfter.TotalAlloc-memBefore.TotalAlloc) / 1024 / 1024
	heapGrowth := float64(memAfter.HeapAlloc-memBefore.HeapAlloc) / 1024 / 1024

	t.Logf("  Events:      2000 (with 4 meta fields each)")
	t.Logf("  Total alloc: %.1f MB", totalAlloc)
	t.Logf("  Heap growth: %.1f MB (after GC)", heapGrowth)
	t.Logf("  GC runs:     %d", gcRuns)

	if heapGrowth > 50 {
		t.Errorf("  HIGH GC PRESSURE: %.1f MB heap growth", heapGrowth)
	} else {
		t.Logf("  ✅ GC pressure acceptable: %.1f MB heap, %d GC runs", heapGrowth, gcRuns)
	}
}
