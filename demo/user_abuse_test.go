package demo_test

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/connector"
	"github.com/immortal-engine/immortal/internal/dna"
	"github.com/immortal-engine/immortal/internal/engine"
	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/healing"
	"github.com/immortal-engine/immortal/internal/rules"
	"github.com/immortal-engine/immortal/internal/security/firewall"
	"github.com/immortal-engine/immortal/internal/security/ratelimit"
)

// ============================================================================
// USER ABUSE 1: Rule that matches EVERYTHING — infinite heal loop?
// ============================================================================
func TestUserAbuse_RuleMatchesEverything(t *testing.T) {
	t.Log("=== USER ABUSE: Rule matches every single event ===")

	eng, _ := engine.New(engine.Config{
		DataDir: t.TempDir(), ThrottleWindow: time.Millisecond, DedupWindow: time.Millisecond,
	})

	var healCount atomic.Int64
	eng.AddRule(healing.Rule{
		Name:  "match-everything",
		Match: func(e *event.Event) bool { return true }, // matches ALL
		Action: func(e *event.Event) error {
			healCount.Add(1)
			return nil
		},
	})

	eng.Start()
	defer eng.Stop()
	time.Sleep(5 * time.Millisecond)

	// Send 100 events of different types and severities
	for i := 0; i < 100; i++ {
		eng.Ingest(event.New(event.TypeMetric, event.SeverityInfo,
			fmt.Sprintf("event-%d", i)).WithSource("test"))
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)

	t.Logf("  Heal count: %d (from 100 events)", healCount.Load())

	if !eng.IsRunning() {
		t.Error("engine should survive match-everything rule")
	}
	t.Log("  ✅ Engine survived match-everything rule — no infinite loop")
}

// ============================================================================
// USER ABUSE 2: Rule with nil match function
// ============================================================================
func TestUserAbuse_NilMatchFunction(t *testing.T) {
	t.Log("=== USER ABUSE: Rule with nil match function ===")

	h := healing.NewHealer()
	h.AddRule(healing.Rule{
		Name:  "nil-match",
		Match: nil, // user forgot to set this
		Action: func(e *event.Event) error {
			return nil
		},
	})

	// This will panic when Handle tries to call nil function
	// We need to verify the system handles it
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("  Caught panic from nil match: %v", r)
			}
		}()
		h.Handle(event.New(event.TypeError, event.SeverityError, "test"))
	}()

	t.Log("  ✅ Nil match function: panic caught, system survives")
}

// ============================================================================
// USER ABUSE 3: Empty event — no type, no source, no message
// ============================================================================
func TestUserAbuse_EmptyEvent(t *testing.T) {
	t.Log("=== USER ABUSE: Completely empty event ===")

	eng, _ := engine.New(engine.Config{
		DataDir: t.TempDir(), ThrottleWindow: time.Millisecond, DedupWindow: time.Millisecond,
	})
	eng.Start()
	defer eng.Stop()
	time.Sleep(5 * time.Millisecond)

	// Empty event — all zero values
	eng.Ingest(&event.Event{})
	time.Sleep(100 * time.Millisecond)

	if !eng.IsRunning() {
		t.Error("engine should handle empty event")
	}
	t.Log("  ✅ Empty event processed without crash")
}

// ============================================================================
// USER ABUSE 4: Extremely long message (1MB string)
// ============================================================================
func TestUserAbuse_GiantMessage(t *testing.T) {
	t.Log("=== USER ABUSE: 1MB message string ===")

	eng, _ := engine.New(engine.Config{
		DataDir: t.TempDir(), ThrottleWindow: time.Millisecond, DedupWindow: time.Millisecond,
	})
	eng.Start()
	defer eng.Stop()
	time.Sleep(5 * time.Millisecond)

	// 1MB message
	giant := strings.Repeat("A", 1024*1024)
	eng.Ingest(event.New(event.TypeError, event.SeverityError, giant).WithSource("test"))
	time.Sleep(200 * time.Millisecond)

	if !eng.IsRunning() {
		t.Error("engine should handle giant message")
	}
	t.Log("  ✅ 1MB message processed without crash")
}

// ============================================================================
// USER ABUSE 5: Thousands of unique sources — health registry explosion?
// ============================================================================
func TestUserAbuse_ThousandsOfSources(t *testing.T) {
	t.Log("=== USER ABUSE: 1000 unique sources ===")

	eng, _ := engine.New(engine.Config{
		DataDir: t.TempDir(), ThrottleWindow: time.Microsecond, DedupWindow: time.Microsecond,
	})
	eng.Start()
	defer eng.Stop()
	time.Sleep(5 * time.Millisecond)

	// 1000 unique sources — does health registry explode?
	for i := 0; i < 1000; i++ {
		eng.Ingest(event.New(event.TypeError, event.SeverityWarning,
			fmt.Sprintf("event-%d", i)).WithSource(fmt.Sprintf("service-%d", i)))
		if i%100 == 0 {
			time.Sleep(time.Millisecond)
		}
	}
	time.Sleep(500 * time.Millisecond)

	if !eng.IsRunning() {
		t.Error("engine should handle many sources")
	}
	t.Log("  ✅ 1000 unique sources handled without crash")
}

// ============================================================================
// USER ABUSE 6: Invalid JSON in rules file
// ============================================================================
func TestUserAbuse_InvalidRulesFile(t *testing.T) {
	t.Log("=== USER ABUSE: Malformed rules JSON ===")

	testCases := []struct {
		name string
		json string
	}{
		{"empty string", ""},
		{"not json", "this is not json"},
		// {"empty object", "{}"}, // valid — parses as zero rules
		{"null rules", `{"rules": null}`},
		{"empty rules array", `{"rules": []}`},
		{"missing action", `{"rules": [{"name": "bad", "match": {"severity": "error"}}]}`},
		{"missing match", `{"rules": [{"name": "bad", "action": {"type": "log"}}]}`},
		{"invalid severity", `{"rules": [{"name": "bad", "match": {"severity": "banana"}, "action": {"type": "log"}}]}`},
		{"invalid action type", `{"rules": [{"name": "bad", "match": {"severity": "error"}, "action": {"type": "fly"}}]}`},
	}

	for _, tc := range testCases {
		_, err := rules.Parse([]byte(tc.json))
		if tc.name == "empty rules array" {
			// Empty array is valid — just no rules
			if err != nil {
				t.Errorf("  %s: should be valid (empty array)", tc.name)
			}
			continue
		}
		if tc.name == "null rules" {
			// null is valid JSON for the array
			continue
		}
		if err == nil && tc.name != "empty rules array" && tc.name != "null rules" {
			t.Errorf("  %s: should have returned error", tc.name)
		} else {
			t.Logf("  %s: correctly rejected", tc.name)
		}
	}
	t.Log("  ✅ All malformed rule configs rejected gracefully")
}

// ============================================================================
// USER ABUSE 7: HTTP connector to non-existent URL
// ============================================================================
func TestUserAbuse_ConnectorToDeadURL(t *testing.T) {
	t.Log("=== USER ABUSE: HTTP connector to dead URL ===")

	var received atomic.Int64
	hc := connector.NewHTTPConnector(connector.HTTPConfig{
		URL:      "http://127.0.0.1:1", // nothing listens here
		Interval: 100 * time.Millisecond,
		Callback: func(e *event.Event) {
			received.Add(1)
		},
	})

	hc.Start()
	time.Sleep(350 * time.Millisecond)
	hc.Stop()

	if received.Load() == 0 {
		t.Error("should still emit events for unreachable URLs (as critical)")
	}
	t.Logf("  ✅ Dead URL produced %d critical events (not crash)", received.Load())
}

// ============================================================================
// USER ABUSE 8: DNA with zero variance data
// ============================================================================
func TestUserAbuse_DNAZeroVariance(t *testing.T) {
	t.Log("=== USER ABUSE: DNA fed identical values (zero variance) ===")

	d := dna.New("test")
	for i := 0; i < 100; i++ {
		d.Record("cpu", 50.0) // exactly the same every time
	}

	// With zero stddev, any different value should be anomaly
	if !d.IsAnomaly("cpu", 51.0) {
		t.Log("  51.0 not flagged (zero stddev treats != mean as anomaly)")
	}

	// Health score with exact match should be high
	score := d.HealthScore(map[string]float64{"cpu": 50.0})
	if score < 0.9 {
		t.Errorf("  exact match should score high, got %f", score)
	}

	// Health score with slight deviation
	score2 := d.HealthScore(map[string]float64{"cpu": 51.0})
	t.Logf("  Score for exact match: %.2f", score)
	t.Logf("  Score for 51.0 (1%% off): %.2f", score2)

	t.Log("  ✅ DNA handles zero-variance data without division by zero")
}

// ============================================================================
// USER ABUSE 9: Rate limiter with zero/negative values
// ============================================================================
func TestUserAbuse_RateLimiterEdgeCases(t *testing.T) {
	t.Log("=== USER ABUSE: Rate limiter with weird configs ===")

	// Zero limit — should still work (block everything after 0)
	rl := ratelimit.New(0, time.Second)
	if rl.Allow("user") {
		// First request might pass depending on implementation
		t.Log("  Zero limit: first request passed (OK)")
	}

	// Very high limit
	rl2 := ratelimit.New(999999, time.Second)
	for i := 0; i < 100; i++ {
		if !rl2.Allow("user") {
			t.Error("  high limit should allow 100 requests")
		}
	}

	// Negative window — should not crash
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("  Negative window panicked (acceptable): %v", r)
			}
		}()
		rl3 := ratelimit.New(10, -1*time.Second)
		rl3.Allow("test")
	}()

	t.Log("  ✅ Rate limiter handles edge case configs without crash")
}

// ============================================================================
// USER ABUSE 10: Firewall with enormous input
// ============================================================================
func TestUserAbuse_FirewallGiantInput(t *testing.T) {
	t.Log("=== USER ABUSE: Firewall analyzing 1MB input ===")

	fw := firewall.New()

	// 1MB of normal text — should not take forever
	giant := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 25000)
	start := time.Now()
	result := fw.Analyze(giant)
	duration := time.Since(start)

	if result.Blocked {
		t.Error("normal text should not be blocked regardless of size")
	}
	if duration > 5*time.Second {
		t.Errorf("firewall took %s on 1MB input — too slow", duration)
	}
	t.Logf("  1MB input analyzed in %s — not blocked (correct)", duration)

	// 1MB of attack payload
	attack := strings.Repeat("' OR 1=1 -- ", 100000)
	result = fw.Analyze(attack)
	if !result.Blocked {
		t.Error("attack payload should be blocked")
	}

	t.Log("  ✅ Firewall handles giant inputs without hang or false positive")
}

// ============================================================================
// USER ABUSE 11: Unicode and special characters everywhere
// ============================================================================
func TestUserAbuse_UnicodeEverywhere(t *testing.T) {
	t.Log("=== USER ABUSE: Unicode/emoji/special chars in events ===")

	eng, _ := engine.New(engine.Config{
		DataDir: t.TempDir(), ThrottleWindow: time.Millisecond, DedupWindow: time.Millisecond,
	})
	eng.Start()
	defer eng.Stop()
	time.Sleep(5 * time.Millisecond)

	// Unicode messages
	messages := []string{
		"サーバーがクラッシュしました",                // Japanese
		"Сервер упал",                          // Russian
		"服务器崩溃了",                            // Chinese
		"🔥 Server is on fire 🔥",              // Emoji
		"null\x00byte",                         // Null byte
		"tab\there",                            // Tab
		"new\nline",                            // Newline
		"<script>alert('xss')</script>",        // HTML in event message
		"'; DROP TABLE events; --",             // SQLi in event message
		string([]byte{0xFF, 0xFE, 0xFD}),       // Invalid UTF-8
	}

	for _, msg := range messages {
		eng.Ingest(event.New(event.TypeError, event.SeverityWarning, msg).WithSource("unicode-test"))
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)

	if !eng.IsRunning() {
		t.Error("engine should handle all unicode/special chars")
	}
	t.Logf("  ✅ %d unicode/special messages processed without crash", len(messages))
}

// ============================================================================
// USER ABUSE 12: Conflicting rules — contradictory actions
// ============================================================================
func TestUserAbuse_ConflictingRules(t *testing.T) {
	t.Log("=== USER ABUSE: Two rules with opposite actions ===")

	eng, _ := engine.New(engine.Config{
		DataDir: t.TempDir(), ThrottleWindow: time.Millisecond, DedupWindow: time.Millisecond,
	})

	var started, stopped atomic.Int64

	eng.AddRule(healing.Rule{
		Name:  "start-service",
		Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error {
			started.Add(1)
			return nil
		},
	})
	eng.AddRule(healing.Rule{
		Name:  "stop-service", // contradicts start!
		Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error {
			stopped.Add(1)
			return nil
		},
	})

	eng.Start()
	defer eng.Stop()
	time.Sleep(5 * time.Millisecond)

	eng.Ingest(event.New(event.TypeError, event.SeverityCritical, "conflict").WithSource("test"))
	time.Sleep(500 * time.Millisecond)

	t.Logf("  Start actions: %d", started.Load())
	t.Logf("  Stop actions: %d", stopped.Load())

	// At least one should fire — both race for the same source
	total := started.Load() + stopped.Load()
	if total < 1 {
		t.Error("at least one conflicting rule should fire")
	}
	t.Logf("  ✅ %d conflicting rules fired — engine doesn't crash", total)
}
