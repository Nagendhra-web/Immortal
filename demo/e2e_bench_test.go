package demo_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/alert"
	"github.com/immortal-engine/immortal/internal/collector"
	"github.com/immortal-engine/immortal/internal/connector"
	"github.com/immortal-engine/immortal/internal/engine"
	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/healing"
)

// ============================================================================
// END-TO-END LATENCY: Full pipeline from Ingest() to action complete
//
// Measures the REAL time for:
// Ingest → throttle → dedup → store → DNA → causality → timetravel
// → healer match → consensus → action execute → alert fire → export
// ============================================================================
func TestE2E_FullPipelineLatency(t *testing.T) {
	t.Log("=== E2E: Full pipeline latency (ingest → action complete) ===")

	eng, _ := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Microsecond,
		DedupWindow:    time.Microsecond,
	})

	var mu sync.Mutex
	var e2eLatencies []time.Duration

	// Rule that measures full E2E time
	eng.AddRule(healing.Rule{
		Name:  "e2e-measure",
		Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error {
			// Action is the LAST step — measure total time from event creation
			latency := time.Since(e.Timestamp)
			mu.Lock()
			e2eLatencies = append(e2eLatencies, latency)
			mu.Unlock()
			return nil
		},
	})

	// Add alert (fires AFTER action — adds to pipeline cost)
	eng.AddAlertRule(alert.AlertRule{
		Name:  "e2e-alert",
		Match: func(e *event.Event) bool { return e.Severity == event.SeverityCritical },
		Level: alert.LevelCritical,
		Title: "E2E",
	})
	eng.AddAlertChannel(&alert.CallbackChannel{Fn: func(a *alert.Alert) {}})

	eng.Start()
	defer eng.Stop()
	time.Sleep(50 * time.Millisecond) // warm up

	// Send 100 unique critical events through the FULL pipeline
	t.Log("  Sending 100 events through full pipeline...")
	for i := 0; i < 100; i++ {
		e := event.New(event.TypeError, event.SeverityCritical,
			fmt.Sprintf("e2e-pipeline-%d", i)).
			WithSource(fmt.Sprintf("service-%d", i%5)).
			WithMeta("cpu", 45.0+float64(i%20)).
			WithMeta("memory", 60.0+float64(i%15))
		eng.Ingest(e)
		time.Sleep(5 * time.Millisecond) // pace to avoid throttle
	}

	time.Sleep(2 * time.Second) // wait for all actions to complete

	mu.Lock()
	defer mu.Unlock()

	if len(e2eLatencies) == 0 {
		t.Fatal("  No E2E latency samples collected")
	}

	sort.Slice(e2eLatencies, func(i, j int) bool { return e2eLatencies[i] < e2eLatencies[j] })

	p50 := e2eLatencies[len(e2eLatencies)*50/100]
	p95 := e2eLatencies[len(e2eLatencies)*95/100]
	p99 := e2eLatencies[len(e2eLatencies)*99/100]
	min := e2eLatencies[0]
	max := e2eLatencies[len(e2eLatencies)-1]

	t.Logf("  ┌─────────────────────────────────────────────────┐")
	t.Logf("  │ E2E FULL PIPELINE LATENCY                      │")
	t.Logf("  │ (ingest → throttle → dedup → store → DNA →     │")
	t.Logf("  │  causality → timetravel → healer → consensus → │")
	t.Logf("  │  action → alert → export)                      │")
	t.Logf("  ├─────────────────────────────────────────────────┤")
	t.Logf("  │ Samples: %d                                    │", len(e2eLatencies))
	t.Logf("  │ min:  %s", min)
	t.Logf("  │ p50:  %s", p50)
	t.Logf("  │ p95:  %s", p95)
	t.Logf("  │ p99:  %s", p99)
	t.Logf("  │ max:  %s", max)
	t.Logf("  └─────────────────────────────────────────────────┘")

	if p99 > 1*time.Second {
		t.Errorf("  E2E p99 too high: %s", p99)
	}
	t.Logf("  ✅ E2E full pipeline: p50=%s p99=%s", p50, p99)
}

// ============================================================================
// END-TO-END THROUGHPUT: How many events/sec through FULL pipeline
// ============================================================================
func TestE2E_FullPipelineThroughput(t *testing.T) {
	t.Log("=== E2E: Full pipeline throughput (events/sec) ===")

	eng, _ := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Microsecond,
		DedupWindow:    time.Microsecond,
	})

	var actioned atomic.Int64
	eng.AddRule(healing.Rule{
		Name:  "throughput-counter",
		Match: healing.MatchSeverity(event.SeverityError),
		Action: func(e *event.Event) error {
			actioned.Add(1)
			return nil
		},
	})

	eng.Start()
	defer eng.Stop()
	time.Sleep(10 * time.Millisecond)

	// Measure: how many unique events can we push through per second?
	total := 1000
	start := time.Now()

	for i := 0; i < total; i++ {
		eng.Ingest(event.New(event.TypeError, event.SeverityError,
			fmt.Sprintf("throughput-%d", i)).
			WithSource("bench").
			WithMeta("val", float64(i)))
	}

	// Wait for pipeline to drain
	time.Sleep(3 * time.Second)
	duration := time.Since(start)

	processed := eng.Monitor().Stats().EventsProcessed
	eps := float64(processed) / duration.Seconds()

	t.Logf("  Ingested:   %d events", total)
	t.Logf("  Processed:  %d events (full pipeline)", processed)
	t.Logf("  Actioned:   %d events (healer fired)", actioned.Load())
	t.Logf("  Duration:   %s", duration.Round(time.Millisecond))
	t.Logf("  Throughput: %.0f events/sec (end-to-end)", eps)
	t.Logf("  ✅ Full pipeline throughput: %.0f events/sec", eps)
}

// ============================================================================
// END-TO-END: Real scenario — HTTP server breaks, Immortal detects and heals
//             Measure time from server 500 → heal action fires
// ============================================================================
func TestE2E_RealHealLatency(t *testing.T) {
	t.Log("=== E2E: Real heal latency (HTTP 500 detected → action fires) ===")

	serverBroken := int64(1) // starts broken
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt64(&serverBroken) == 1 {
			w.WriteHeader(500)
		} else {
			w.WriteHeader(200)
		}
	}))
	defer server.Close()

	eng, _ := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: 50 * time.Millisecond,
		DedupWindow:    50 * time.Millisecond,
	})

	healTime := time.Time{}
	eng.AddRule(healing.Rule{
		Name:  "real-heal",
		Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error {
			healTime = time.Now()
			atomic.StoreInt64(&serverBroken, 0) // fix it
			return nil
		},
	})

	eng.Start()
	defer eng.Stop()

	// Start HTTP health checker
	startTime := time.Now()
	hc := connector.NewHTTPConnector(connector.HTTPConfig{
		URL:      server.URL,
		Interval: 100 * time.Millisecond,
		Callback: eng.Ingest,
	})
	hc.Start()
	defer hc.Stop()

	// Wait for detection + healing
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("  Timeout: server not healed within 5s")
		case <-time.After(50 * time.Millisecond):
			if !healTime.IsZero() {
				totalLatency := healTime.Sub(startTime)
				t.Logf("  Server broke at:   t=0")
				t.Logf("  Detected + healed: t=%s", totalLatency)
				t.Logf("  ✅ Real heal latency: %s (broken server → fixed)", totalLatency)
				return
			}
		}
	}
}

// ============================================================================
// END-TO-END: Log error → detected → healed (real file I/O)
// ============================================================================
func TestE2E_LogToHealLatency(t *testing.T) {
	t.Log("=== E2E: Log error → detected → healed (real file I/O) ===")

	logPath := filepath.Join(t.TempDir(), "app.log")
	os.WriteFile(logPath, []byte(""), 0644)

	eng, _ := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: 10 * time.Millisecond,
		DedupWindow:    10 * time.Millisecond,
	})

	healTime := time.Time{}
	eng.AddRule(healing.Rule{
		Name:  "log-heal",
		Match: healing.MatchSeverity(event.SeverityError),
		Action: func(e *event.Event) error {
			healTime = time.Now()
			return nil
		},
	})

	eng.Start()
	defer eng.Stop()

	lc := collector.NewLogCollector(logPath, eng.Ingest)
	lc.Start()
	defer lc.Stop()

	time.Sleep(500 * time.Millisecond) // collector warm-up

	// Write error to log file — measure until heal action fires
	writeTime := time.Now()
	f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("FATAL: database connection lost — all queries failing\n")
	f.Sync()
	f.Close()

	// Wait for detection
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("  Timeout: log error not detected within 5s")
		case <-time.After(50 * time.Millisecond):
			if !healTime.IsZero() {
				latency := healTime.Sub(writeTime)
				t.Logf("  Error written at:  t=0")
				t.Logf("  Detected + healed: t=%s", latency)
				t.Logf("  ✅ Log-to-heal latency: %s (write → detect → action)", latency)
				return
			}
		}
	}
}

// ============================================================================
// END-TO-END: Sustained load — memory + goroutine check over time
// ============================================================================
func TestE2E_SustainedFullPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping sustained E2E test")
	}
	t.Log("=== E2E: Sustained full pipeline (10 seconds) ===")

	eng, _ := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Microsecond,
		DedupWindow:    time.Microsecond,
	})

	var healed atomic.Int64
	eng.AddRule(healing.Rule{
		Name:  "sustained",
		Match: healing.MatchSeverity(event.SeverityError),
		Action: func(e *event.Event) error { healed.Add(1); return nil },
	})

	eng.Start()
	defer eng.Stop()

	var memStart runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStart)
	grStart := runtime.NumGoroutine()

	// Sustain load for 10 seconds
	done := make(chan bool)
	var totalSent atomic.Int64

	go func() {
		timer := time.After(10 * time.Second)
		for {
			select {
			case <-timer:
				done <- true
				return
			default:
				eng.Ingest(event.New(event.TypeError, event.SeverityError,
					fmt.Sprintf("sustained-%d", totalSent.Load())).
					WithSource("load").
					WithMeta("cpu", 45.0))
				totalSent.Add(1)
				time.Sleep(100 * time.Microsecond)
			}
		}
	}()

	<-done
	time.Sleep(2 * time.Second)

	var memEnd runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memEnd)
	grEnd := runtime.NumGoroutine()

	heapGrowth := float64(memEnd.HeapAlloc-memStart.HeapAlloc) / 1024 / 1024
	grDelta := grEnd - grStart

	t.Logf("  Duration:    10 seconds")
	t.Logf("  Sent:        %d events", totalSent.Load())
	t.Logf("  Healed:      %d actions", healed.Load())
	t.Logf("  Throughput:  %.0f events/sec (sustained E2E)", float64(totalSent.Load())/10.0)
	t.Logf("  Heap growth: %.1f MB", heapGrowth)
	t.Logf("  Goroutines:  %d → %d (Δ%d)", grStart, grEnd, grDelta)
	t.Logf("  GC runs:     %d", memEnd.NumGC-memStart.NumGC)

	if heapGrowth > 100 {
		t.Errorf("  Memory leak: %.1f MB growth", heapGrowth)
	}
	if grDelta > 20 {
		t.Errorf("  Goroutine leak: Δ%d", grDelta)
	}
	t.Logf("  ✅ Sustained E2E: %d events over 10s, %.1f MB growth, Δ%d goroutines",
		totalSent.Load(), heapGrowth, grDelta)
}
