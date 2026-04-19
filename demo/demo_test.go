package demo_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/alert"
	"github.com/Nagendhra-web/Immortal/internal/collector"
	"github.com/Nagendhra-web/Immortal/internal/connector"
	"github.com/Nagendhra-web/Immortal/internal/engine"
	"github.com/Nagendhra-web/Immortal/internal/event"
	"github.com/Nagendhra-web/Immortal/internal/healing"
)

// ============================================================================
// SCENARIO 1: Web server starts returning 500 errors → Immortal detects & heals
// ============================================================================
func TestScenario_APIReturns500_ImmortalHeals(t *testing.T) {
	t.Log("=== SCENARIO: API starts returning 500 → Immortal detects and heals ===")

	// Simulate a web server that starts healthy then breaks
	requestCount := int64(0)
	serverFixed := int64(0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt64(&requestCount, 1)
		if count > 3 && atomic.LoadInt64(&serverFixed) == 0 {
			// After 3 healthy checks, start failing
			w.WriteHeader(500)
			w.Write([]byte("Internal Server Error"))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	t.Logf("  Demo server started at %s", server.URL)

	// Create Immortal engine
	eng, err := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: 100 * time.Millisecond,
		DedupWindow:    100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Track healing actions
	var healed atomic.Int64
	var alerts atomic.Int64

	// Add healing rule: when HTTP returns 500, "fix" the server
	eng.AddRule(healing.Rule{
		Name:  "fix-api-500",
		Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error {
			t.Logf("  [HEAL] Fixing server: %s", e.Message)
			atomic.StoreInt64(&serverFixed, 1) // "fix" the server
			healed.Add(1)
			return nil
		},
	})

	// Add alert rule
	eng.AddAlertRule(alert.AlertRule{
		Name:  "api-down-alert",
		Match: func(e *event.Event) bool { return e.Severity == event.SeverityCritical },
		Level: alert.LevelCritical,
		Title: "API Down",
	})
	eng.AddAlertChannel(&alert.CallbackChannel{Fn: func(a *alert.Alert) {
		t.Logf("  [ALERT] %s: %s", a.Title, a.Message)
		alerts.Add(1)
	}})

	// Start engine
	eng.Start()
	defer eng.Stop()

	// Start HTTP health checker (fast interval for test)
	hc := connector.NewHTTPConnector(connector.HTTPConfig{
		URL:      server.URL,
		Interval: 200 * time.Millisecond,
		Callback: eng.Ingest,
	})
	hc.Start()
	defer hc.Stop()

	t.Log("  Immortal watching... waiting for server to break...")

	// Wait for detection + healing
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("  TIMEOUT: Immortal didn't heal within 5 seconds")
		case <-time.After(100 * time.Millisecond):
			if healed.Load() > 0 {
				t.Logf("  ✅ Server healed after %d health checks!", atomic.LoadInt64(&requestCount))
				t.Logf("  ✅ Healing actions: %d", healed.Load())
				t.Logf("  ✅ Alerts fired: %d", alerts.Load())

				// Verify server is actually fixed now
				time.Sleep(500 * time.Millisecond)
				resp, err := http.Get(server.URL)
				if err != nil {
					t.Fatal(err)
				}
				if resp.StatusCode != 200 {
					t.Errorf("  Server should be fixed, got %d", resp.StatusCode)
				} else {
					t.Log("  ✅ Server confirmed healthy after healing!")
				}
				return
			}
		}
	}
}

// ============================================================================
// SCENARIO 2: Log file fills with errors → Immortal detects pattern
// ============================================================================
func TestScenario_LogErrors_ImmortalDetects(t *testing.T) {
	t.Log("=== SCENARIO: App logs errors → Immortal detects and responds ===")

	// Create a temp log file
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "app.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	logFile.Close()

	// Create engine
	eng, err := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: 50 * time.Millisecond,
		DedupWindow:    50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	var detected atomic.Int64

	eng.AddRule(healing.Rule{
		Name:  "log-error-handler",
		Match: healing.MatchSeverity(event.SeverityError),
		Action: func(e *event.Event) error {
			t.Logf("  [HEAL] Detected log error: %s", e.Message)
			detected.Add(1)
			return nil
		},
	})

	eng.Start()
	defer eng.Stop()

	// Start log collector
	lc := collector.NewLogCollector(logPath, eng.Ingest)
	lc.Start()
	defer lc.Stop()

	t.Log("  Immortal watching log file...")

	// Wait for collector to start
	time.Sleep(500 * time.Millisecond)

	// Simulate app writing errors to log
	t.Log("  App writing errors to log...")
	f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("2026-03-26 INFO: Server started\n")
	f.WriteString("2026-03-26 INFO: Handling request\n")
	time.Sleep(100 * time.Millisecond)

	f.WriteString("2026-03-26 ERROR: Database connection timeout after 30s\n")
	time.Sleep(100 * time.Millisecond)

	f.WriteString("2026-03-26 FATAL: Out of memory - cannot allocate 512MB\n")
	f.Sync()
	f.Close()

	// Wait for detection
	time.Sleep(1 * time.Second)

	if detected.Load() < 1 {
		t.Error("  Immortal should have detected at least 1 error from logs")
	} else {
		t.Logf("  ✅ Detected %d errors from log file!", detected.Load())
	}
}

// ============================================================================
// SCENARIO 3: CPU spike → DNA detects anomaly → Predictive healing
// ============================================================================
func TestScenario_CPUAnomaly_DNADetects(t *testing.T) {
	t.Log("=== SCENARIO: CPU spikes → DNA detects anomaly ===")

	eng, err := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Millisecond,
		DedupWindow:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	eng.Start()
	defer eng.Stop()

	// Feed normal CPU metrics to establish baseline
	t.Log("  Training DNA with normal CPU metrics (40-50%)...")
	for i := 0; i < 100; i++ {
		e := event.New(event.TypeMetric, event.SeverityInfo, "cpu metric").
			WithSource("system").
			WithMeta("cpu_percent", 40.0+float64(i%10))
		eng.Ingest(e)
		time.Sleep(2 * time.Millisecond) // let throttle/dedup windows pass
	}

	time.Sleep(200 * time.Millisecond)

	// Check DNA learned the baseline
	baseline := eng.DNA().Baseline()
	cpuStats, ok := baseline["cpu_percent"]
	if !ok {
		t.Fatal("  DNA should have learned cpu_percent baseline")
	}
	t.Logf("  DNA baseline: mean=%.1f stddev=%.1f", cpuStats.Mean, cpuStats.StdDev)

	// Now send anomalous CPU
	t.Log("  Sending anomalous CPU: 95%...")
	isAnomaly := eng.DNA().IsAnomaly("cpu_percent", 95.0)
	if !isAnomaly {
		t.Error("  95% CPU should be anomaly against 40-50% baseline")
	} else {
		t.Log("  ✅ DNA correctly detected 95% CPU as ANOMALY!")
	}

	// Check health score
	score := eng.DNA().HealthScore(map[string]float64{
		"cpu_percent": 95.0,
	})
	t.Logf("  Health score with 95%% CPU: %.2f", score)
	if score > 0.5 {
		t.Error("  Health score should be low for anomalous CPU")
	} else {
		t.Log("  ✅ Health score correctly LOW for anomalous metrics!")
	}
}

// ============================================================================
// SCENARIO 4: Ghost mode — observes but doesn't act
// ============================================================================
func TestScenario_GhostMode_ObservesOnly(t *testing.T) {
	t.Log("=== SCENARIO: Ghost mode — Immortal watches but doesn't touch ===")

	eng, err := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		GhostMode:      true, // GHOST MODE
		ThrottleWindow: time.Millisecond,
		DedupWindow:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	actionExecuted := false
	eng.AddRule(healing.Rule{
		Name:  "would-restart",
		Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error {
			actionExecuted = true
			return nil
		},
	})

	eng.Start()
	defer eng.Stop()

	time.Sleep(5 * time.Millisecond)

	// Send critical event
	t.Log("  Sending critical event in ghost mode...")
	eng.Ingest(event.New(event.TypeError, event.SeverityCritical, "server crashed").WithSource("api"))

	time.Sleep(500 * time.Millisecond)

	if actionExecuted {
		t.Error("  Ghost mode should NOT execute actions!")
	} else {
		t.Log("  ✅ Ghost mode correctly did NOT execute any action!")
	}

	recs := eng.Recommendations()
	if len(recs) == 0 {
		t.Error("  Ghost mode should still produce recommendations")
	} else {
		t.Logf("  ✅ Ghost mode produced %d recommendations", len(recs))
		for _, r := range recs {
			t.Logf("     → %s", r.Message)
		}
	}
}

// ============================================================================
// SCENARIO 5: Multiple services — cascading failure
// ============================================================================
func TestScenario_CascadingFailure_CausalityTracked(t *testing.T) {
	t.Log("=== SCENARIO: Cascading failure across services ===")

	eng, err := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Millisecond,
		DedupWindow:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	eng.Start()
	defer eng.Stop()

	time.Sleep(5 * time.Millisecond)

	// Simulate cascade: disk full → DB crash → API timeout
	t.Log("  Simulating cascade: disk full → DB crash → API timeout")

	e1 := event.New(event.TypeError, event.SeverityError, "disk 95% full").WithSource("storage")
	eng.Ingest(e1)
	time.Sleep(10 * time.Millisecond)

	e2 := event.New(event.TypeError, event.SeverityCritical, "database write failed").WithSource("postgres")
	eng.Ingest(e2)
	time.Sleep(10 * time.Millisecond)

	e3 := event.New(event.TypeError, event.SeverityCritical, "API request timeout").WithSource("api-server")
	eng.Ingest(e3)
	time.Sleep(200 * time.Millisecond)

	// Check causality graph
	graph := eng.CausalityGraph()
	graph.AutoCorrelate()

	chain := graph.RootCause(e3.ID)
	t.Logf("  Causality chain (tracing API timeout):")
	for i, ev := range chain {
		t.Logf("    %d. [%s] %s — %s", i+1, ev.Severity, ev.Source, ev.Message)
	}

	if len(chain) >= 2 {
		t.Logf("  ✅ Causality traced %d events in the chain!", len(chain))
	} else {
		t.Error("  Should trace at least 2 events in causality chain")
	}

	// Check impact of disk full
	impact := graph.Impact(e1.ID)
	t.Logf("  Impact of 'disk full': %d downstream events affected", len(impact))
}

// ============================================================================
// SCENARIO 6: Full pipeline — event throttling prevents flood
// ============================================================================
func TestScenario_EventFlood_ThrottlePrevents(t *testing.T) {
	t.Log("=== SCENARIO: 1000 identical events → Throttle prevents flood ===")

	eng, err := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Second,
		DedupWindow:    time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	var healCount atomic.Int64
	eng.AddRule(healing.Rule{
		Name:  "counter",
		Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error {
			healCount.Add(1)
			return nil
		},
	})

	eng.Start()
	defer eng.Stop()

	// Flood with 1000 identical events
	t.Log("  Flooding engine with 1000 identical crash events...")
	for i := 0; i < 1000; i++ {
		eng.Ingest(event.New(event.TypeError, event.SeverityCritical, "same crash").WithSource("api"))
	}

	time.Sleep(1 * time.Second)

	count := healCount.Load()
	t.Logf("  Throttle result: %d/1000 events passed through", count)

	if count > 5 {
		t.Errorf("  Throttle should block most duplicates, but %d got through", count)
	} else {
		t.Logf("  ✅ Throttle correctly blocked %d/1000 duplicate events!", 1000-count)
	}
}

// ============================================================================
// SCENARIO 7: Time-travel — replay events before a failure
// ============================================================================
func TestScenario_TimeTravel_ReplayBeforeFailure(t *testing.T) {
	t.Log("=== SCENARIO: Time-travel — replay what happened before crash ===")

	eng, err := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Millisecond,
		DedupWindow:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	eng.Start()
	defer eng.Stop()

	// Feed events over time
	t.Log("  Recording normal events...")
	for i := 0; i < 5; i++ {
		eng.Ingest(event.New(event.TypeMetric, event.SeverityInfo,
			fmt.Sprintf("cpu: %d%%", 40+i*2)).WithSource("system"))
		time.Sleep(10 * time.Millisecond)
	}

	crashTime := time.Now()
	time.Sleep(10 * time.Millisecond)

	t.Log("  CRASH happens now!")
	eng.Ingest(event.New(event.TypeError, event.SeverityCritical, "FATAL: process crashed").WithSource("api"))

	time.Sleep(200 * time.Millisecond)

	// Time-travel: what happened before the crash?
	t.Log("  Rewinding to before the crash...")
	recorder := eng.TimeTravel()
	beforeCrash := recorder.RewindBefore(crashTime, 5)

	t.Logf("  Events before crash:")
	for i, e := range beforeCrash {
		t.Logf("    %d. [%s] %s — %s", i+1, e.Severity, e.Source, e.Message)
	}

	if len(beforeCrash) > 0 {
		t.Logf("  ✅ Time-travel successfully replayed %d events before crash!", len(beforeCrash))
	} else {
		t.Error("  Should have events before crash")
	}
}

// ============================================================================
// SCENARIO 8: REST API — query Immortal while it's running
// ============================================================================
func TestScenario_RESTAPI_QueryWhileRunning(t *testing.T) {
	t.Log("=== SCENARIO: Query Immortal via REST API while it's running ===")

	eng, err := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Millisecond,
		DedupWindow:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	eng.RegisterService("api-server")
	eng.Start()
	defer eng.Stop()

	time.Sleep(5 * time.Millisecond)

	// Feed some events
	eng.Ingest(event.New(event.TypeError, event.SeverityError, "connection timeout").WithSource("api-server"))
	time.Sleep(100 * time.Millisecond)

	// Query via the internal API objects (simulating REST calls)
	// Check health registry
	health := eng.HealthRegistry()
	svc := health.Get("api-server")
	if svc == nil {
		t.Fatal("  Service should be registered")
	}
	t.Logf("  Service: %s — Status: %s — Checks: %d", svc.Name, svc.Status, svc.Checks)

	// Check self-monitor
	stats := eng.Monitor().Stats()
	t.Logf("  Self-monitor: goroutines=%d, events=%d, heals=%d, uptime=%s",
		stats.Goroutines, stats.EventsProcessed, stats.HealsExecuted, stats.Uptime.Round(time.Millisecond))

	// Check metrics export
	metrics := eng.Exporter().Export()
	if metrics != "" {
		t.Log("  ✅ Prometheus metrics available!")
		// Count metric lines
		lines := 0
		for _, c := range metrics {
			if c == '\n' {
				lines++
			}
		}
		t.Logf("  Prometheus output: %d lines", lines)
	}

	t.Log("  ✅ All internal APIs accessible while engine is running!")
}
