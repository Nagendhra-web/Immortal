package autolearn_test

import (
	"testing"

	"github.com/immortal-engine/immortal/internal/autolearn"
)

func TestNewLearner(t *testing.T) {
	l := autolearn.New(5)
	if l == nil {
		t.Fatal("expected learner")
	}
}

func TestRecord(t *testing.T) {
	l := autolearn.New(5)
	l.Record("rule-1", "api-server", "crash", "critical", true)

	all := l.AllRules()
	if len(all) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(all))
	}
	if all[0].Pattern != "api-server:critical" {
		t.Errorf("expected api-server:critical, got %s", all[0].Pattern)
	}
}

func TestSuggestedRules(t *testing.T) {
	l := autolearn.New(3)

	// 5 successful heals for same pattern
	for i := 0; i < 5; i++ {
		l.Record("rule-1", "api-server", "crash", "critical", true)
	}

	suggested := l.SuggestedRules()
	if len(suggested) != 1 {
		t.Fatalf("expected 1 suggested rule, got %d", len(suggested))
	}
	if !suggested[0].Suggested {
		t.Error("expected suggested=true")
	}
	if suggested[0].Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %.2f", suggested[0].Confidence)
	}
}

func TestBelowThreshold(t *testing.T) {
	l := autolearn.New(10)

	for i := 0; i < 5; i++ {
		l.Record("rule-1", "api", "crash", "critical", true)
	}

	suggested := l.SuggestedRules()
	if len(suggested) != 0 {
		t.Errorf("expected 0 suggested, got %d", len(suggested))
	}
}

func TestConfidence(t *testing.T) {
	l := autolearn.New(3)

	// 3 success, 2 failure = 60% confidence
	for i := 0; i < 3; i++ {
		l.Record("rule-1", "api", "crash", "critical", true)
	}
	for i := 0; i < 2; i++ {
		l.Record("rule-1", "api", "crash", "critical", false)
	}

	c := l.Confidence("api:critical")
	if c < 0.59 || c > 0.61 {
		t.Errorf("expected confidence ~0.6, got %.2f", c)
	}
}

func TestAllRules(t *testing.T) {
	l := autolearn.New(3)

	for i := 0; i < 10; i++ {
		l.Record("r1", "api", "crash", "critical", true)
	}
	for i := 0; i < 5; i++ {
		l.Record("r2", "db", "timeout", "error", true)
	}

	all := l.AllRules()
	if len(all) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(all))
	}
	// sorted by occurrences desc
	if all[0].Occurrences < all[1].Occurrences {
		t.Error("expected sorted by occurrences desc")
	}
}

func TestForget(t *testing.T) {
	l := autolearn.New(3)
	l.Record("r1", "api", "crash", "critical", true)
	l.Forget("api:critical")

	if l.Confidence("api:critical") != 0 {
		t.Error("expected 0 confidence after forget")
	}
}

func TestReset(t *testing.T) {
	l := autolearn.New(3)
	for i := 0; i < 5; i++ {
		l.Record("r1", "api", "crash", "critical", true)
	}
	l.Reset()

	if len(l.AllRules()) != 0 {
		t.Error("expected 0 rules after reset")
	}
}

func TestStats(t *testing.T) {
	l := autolearn.New(3)
	for i := 0; i < 5; i++ {
		l.Record("r1", "api", "crash", "critical", true)
	}

	stats := l.Stats()
	if stats["total_events"].(int) != 5 {
		t.Errorf("expected 5 events, got %v", stats["total_events"])
	}
	if stats["total_rules"].(int) != 1 {
		t.Errorf("expected 1 rule, got %v", stats["total_rules"])
	}
	if stats["top_pattern"].(string) != "api:critical" {
		t.Errorf("expected api:critical, got %v", stats["top_pattern"])
	}
}

func TestMultiplePatterns(t *testing.T) {
	l := autolearn.New(2)

	l.Record("r1", "api", "crash", "critical", true)
	l.Record("r1", "api", "crash", "critical", true)
	l.Record("r2", "db", "slow", "warning", true)
	l.Record("r2", "db", "slow", "warning", true)

	suggested := l.SuggestedRules()
	if len(suggested) != 2 {
		t.Errorf("expected 2 suggested rules, got %d", len(suggested))
	}
}
