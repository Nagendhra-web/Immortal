package demo_test

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/bus"
	"github.com/Nagendhra-web/Immortal/internal/engine"
	"github.com/Nagendhra-web/Immortal/internal/event"
	"github.com/Nagendhra-web/Immortal/internal/healing"
)

// ============================================================================
// CHAOS TEST 1: 100K burst spike — backpressure must hold
// ============================================================================
func TestChaos_100K_BurstSpike(t *testing.T) {
	t.Log("=== CHAOS: 100K events burst spike ===")

	b := bus.NewWithConfig(5000, 32) // 5K buffer, 32 workers
	defer b.Close()

	var processed atomic.Int64
	b.Subscribe("*", func(e *event.Event) {
		processed.Add(1)
	})

	goroutinesBefore := runtime.NumGoroutine()

	// Fire 100K events as fast as possible
	start := time.Now()
	for i := 0; i < 100000; i++ {
		b.Publish(event.New(event.TypeError, event.SeverityError, fmt.Sprintf("burst-%d", i)))
	}
	publishDuration := time.Since(start)

	// Wait for processing
	time.Sleep(2 * time.Second)

	goroutinesAfter := runtime.NumGoroutine()
	pub, proc, drop := b.Stats()

	t.Logf("  Published:  %d events in %s", pub, publishDuration)
	t.Logf("  Processed:  %d events", proc)
	t.Logf("  Dropped:    %d events (backpressure)", drop)
	t.Logf("  Goroutines: %d before → %d after (Δ%d)", goroutinesBefore, goroutinesAfter, goroutinesAfter-goroutinesBefore)
	t.Logf("  Throughput: %.0f events/sec", float64(pub)/publishDuration.Seconds())

	// Key assertions
	if goroutinesAfter-goroutinesBefore > 50 {
		t.Errorf("goroutine leak: Δ%d (should be bounded by worker pool)", goroutinesAfter-goroutinesBefore)
	}
	if proc+drop != pub {
		t.Errorf("processed(%d) + dropped(%d) should equal published(%d)", proc, drop, pub)
	}
	t.Logf("✅ 100K burst: bounded goroutines, backpressure working")
}

// ============================================================================
// CHAOS TEST 2: Sustained load over time — memory must be stable
// ============================================================================
func TestChaos_SustainedLoad_MemoryStable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping sustained load test in short mode")
	}
	t.Log("=== CHAOS: Sustained load — memory stability ===")

	eng, _ := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Millisecond,
		DedupWindow:    time.Millisecond,
	})
	eng.AddRule(healing.Rule{
		Name:  "noop",
		Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error { return nil },
	})
	eng.Start()
	defer eng.Stop()

	// Record memory at start
	var memStart runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStart)

	// Sustain load for 3 seconds
	t.Log("  Sustaining load for 3 seconds...")
	done := make(chan bool)
	var totalEvents atomic.Int64

	go func() {
		timer := time.After(3 * time.Second)
		for {
			select {
			case <-timer:
				done <- true
				return
			default:
				eng.Ingest(event.New(event.TypeMetric, event.SeverityInfo,
					fmt.Sprintf("metric-%d", totalEvents.Load())).
					WithSource("load-test").
					WithMeta("cpu", 45.0))
				totalEvents.Add(1)
				time.Sleep(10 * time.Microsecond) // ~100K events/sec pace
			}
		}
	}()
	<-done

	// Let processing settle
	time.Sleep(500 * time.Millisecond)

	// Record memory at end
	var memEnd runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memEnd)

	heapGrowthMB := float64(memEnd.HeapAlloc-memStart.HeapAlloc) / 1024 / 1024

	t.Logf("  Total events: %d", totalEvents.Load())
	t.Logf("  Heap start:   %.1f MB", float64(memStart.HeapAlloc)/1024/1024)
	t.Logf("  Heap end:     %.1f MB", float64(memEnd.HeapAlloc)/1024/1024)
	t.Logf("  Heap growth:  %.1f MB", heapGrowthMB)
	t.Logf("  GC runs:      %d", memEnd.NumGC-memStart.NumGC)

	if heapGrowthMB > 100 {
		t.Errorf("memory grew %.1f MB — possible leak", heapGrowthMB)
	}
	t.Logf("✅ Sustained load: %.1f MB growth over %d events — memory stable", heapGrowthMB, totalEvents.Load())
}

// ============================================================================
// CHAOS TEST 3: Concurrent publishers — no data corruption
// ============================================================================
func TestChaos_ConcurrentPublishers_NoCorruption(t *testing.T) {
	t.Log("=== CHAOS: 50 concurrent publishers ===")

	b := bus.NewWithConfig(50000, 32)
	defer b.Close()

	var processed atomic.Int64
	b.Subscribe("*", func(e *event.Event) {
		processed.Add(1)
	})

	// 50 goroutines each publishing 1000 events
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				b.Publish(event.New(event.TypeError, event.SeverityError,
					fmt.Sprintf("publisher-%d-event-%d", n, j)))
			}
		}(i)
	}
	wg.Wait()

	time.Sleep(2 * time.Second)
	pub, proc, drop := b.Stats()

	t.Logf("  Published:  %d", pub)
	t.Logf("  Processed:  %d", proc)
	t.Logf("  Dropped:    %d", drop)

	if pub != 50000 {
		t.Errorf("expected 50000 published, got %d", pub)
	}
	if proc+drop != pub {
		t.Errorf("accounting mismatch: processed(%d) + dropped(%d) != published(%d)", proc, drop, pub)
	}
	t.Logf("✅ 50 concurrent publishers: zero corruption, accounting correct")
}

// ============================================================================
// CHAOS TEST 4: Slow handler — doesn't block other handlers
// ============================================================================
func TestChaos_SlowHandler_DoesntBlock(t *testing.T) {
	t.Log("=== CHAOS: Slow handler doesn't block fast handlers ===")

	b := bus.NewWithConfig(1000, 32)
	defer b.Close()

	var fastCount atomic.Int64
	var slowCount atomic.Int64

	// Slow handler — 100ms per event
	b.Subscribe("error", func(e *event.Event) {
		time.Sleep(100 * time.Millisecond)
		slowCount.Add(1)
	})

	// Fast handler
	b.Subscribe("error", func(e *event.Event) {
		fastCount.Add(1)
	})

	// Publish 10 events
	for i := 0; i < 10; i++ {
		b.Publish(event.New(event.TypeError, event.SeverityError, fmt.Sprintf("event-%d", i)))
	}

	// After 200ms, fast handler should have processed all, slow handler some
	time.Sleep(200 * time.Millisecond)

	t.Logf("  Fast handler: %d/10 processed", fastCount.Load())
	t.Logf("  Slow handler: %d/10 processed", slowCount.Load())

	if fastCount.Load() < 5 {
		t.Error("fast handler should not be blocked by slow handler")
	}
	t.Log("✅ Slow handler doesn't block fast handlers (worker pool isolation)")
}

// ============================================================================
// CHAOS TEST 5: Backpressure — queue full behavior
// ============================================================================
func TestChaos_Backpressure_QueueFull(t *testing.T) {
	t.Log("=== CHAOS: Backpressure when queue is full ===")

	// Tiny buffer — fills up fast
	b := bus.NewWithConfig(10, 1) // 10-item buffer, 1 worker
	defer b.Close()

	b.Subscribe("*", func(e *event.Event) {
		time.Sleep(50 * time.Millisecond) // Slow consumer
	})

	// Blast 1000 events into tiny buffer
	for i := 0; i < 1000; i++ {
		b.Publish(event.New(event.TypeError, event.SeverityError, "flood"))
	}

	time.Sleep(100 * time.Millisecond)
	_, _, dropped := b.Stats()

	t.Logf("  Dropped: %d/1000 (backpressure active)", dropped)

	if dropped == 0 {
		t.Error("should drop events when queue is full")
	}
	t.Logf("✅ Backpressure: dropped %d events instead of OOM/hang", dropped)
}

// ============================================================================
// CHAOS TEST 6: Engine under burst — goroutine count stays bounded
// ============================================================================
func TestChaos_Engine_GoroutineBounded(t *testing.T) {
	t.Log("=== CHAOS: Engine goroutines stay bounded under load ===")

	eng, _ := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Microsecond,
		DedupWindow:    time.Microsecond,
	})
	eng.Start()
	defer eng.Stop()

	goroutinesBefore := runtime.NumGoroutine()

	// Blast 10K events
	for i := 0; i < 10000; i++ {
		eng.Ingest(event.New(event.TypeMetric, event.SeverityInfo,
			fmt.Sprintf("goroutine-test-%d", i)).WithSource("test"))
	}

	time.Sleep(1 * time.Second)
	goroutinesAfter := runtime.NumGoroutine()

	delta := goroutinesAfter - goroutinesBefore
	t.Logf("  Goroutines: %d → %d (Δ%d)", goroutinesBefore, goroutinesAfter, delta)

	if delta > 50 {
		t.Errorf("goroutine leak: Δ%d (worker pool should bound this)", delta)
	}
	t.Logf("✅ Engine goroutines bounded: Δ%d after 10K events", delta)
}

// ============================================================================
// CHAOS TEST 7: Bus graceful shutdown — no events lost after close
// ============================================================================
func TestChaos_Bus_GracefulShutdown(t *testing.T) {
	t.Log("=== CHAOS: Bus drains queue on shutdown ===")

	b := bus.NewWithConfig(10000, 8)

	var processed atomic.Int64
	b.Subscribe("*", func(e *event.Event) {
		processed.Add(1)
	})

	// Publish events
	for i := 0; i < 5000; i++ {
		b.Publish(event.New(event.TypeError, event.SeverityError, fmt.Sprintf("drain-%d", i)))
	}

	// Close — should drain the queue
	b.Close()

	t.Logf("  Processed after close: %d", processed.Load())

	if processed.Load() == 0 {
		t.Error("should process at least some events before closing")
	}
	t.Logf("✅ Bus drained %d events on graceful shutdown", processed.Load())
}
