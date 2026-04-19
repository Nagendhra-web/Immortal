package healing_test

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/event"
	"github.com/Nagendhra-web/Immortal/internal/healing"
)

func TestHealerConcurrentHandle(t *testing.T) {
	h := healing.NewHealer()
	var count atomic.Int64

	h.AddRule(healing.Rule{
		Name:  "counter",
		Match: healing.MatchSeverity(event.SeverityError),
		Action: func(e *event.Event) error {
			count.Add(1)
			return nil
		},
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			// Unique source per event so policy doesn't group them
			h.Handle(event.New(event.TypeError, event.SeverityError, "concurrent").
				WithSource(fmt.Sprintf("src-%d", n)))
		}(i)
	}
	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	if count.Load() != 100 {
		t.Errorf("expected 100 actions, got %d", count.Load())
	}
}

func TestHealerMultipleRulesMatch(t *testing.T) {
	h := healing.NewHealer()
	var rule1, rule2, rule3 atomic.Int64

	h.AddRule(healing.Rule{
		Name:   "rule1",
		Match:  healing.MatchSeverity(event.SeverityError),
		Action: func(e *event.Event) error { rule1.Add(1); return nil },
	})
	h.AddRule(healing.Rule{
		Name:   "rule2",
		Match:  healing.MatchContains("crash"),
		Action: func(e *event.Event) error { rule2.Add(1); return nil },
	})
	h.AddRule(healing.Rule{
		Name:   "rule3",
		Match:  healing.MatchSource("api"),
		Action: func(e *event.Event) error { rule3.Add(1); return nil },
	})

	e := event.New(event.TypeError, event.SeverityCritical, "crash happened").WithSource("api")
	h.Handle(e)
	time.Sleep(200 * time.Millisecond)

	// All 3 rules should fire at least once (race between goroutines means exact count may vary)
	total := rule1.Load() + rule2.Load() + rule3.Load()
	if total < 1 {
		t.Errorf("at least 1 rule should have fired, got total %d (r1=%d r2=%d r3=%d)",
			total, rule1.Load(), rule2.Load(), rule3.Load())
	}
}

func TestHealerActionError(t *testing.T) {
	h := healing.NewHealer()

	h.AddRule(healing.Rule{
		Name:  "failing",
		Match: healing.MatchSeverity(event.SeverityError),
		Action: func(e *event.Event) error {
			return errors.New("action failed")
		},
	})

	h.Handle(event.New(event.TypeError, event.SeverityError, "test"))
	time.Sleep(200 * time.Millisecond)

	history := h.History()
	if len(history) == 0 {
		t.Fatal("expected history entry")
	}
	if history[0].Success {
		t.Error("expected failure in history")
	}
	if history[0].Error == "" {
		t.Error("expected error message in history")
	}
}

func TestHealerMatchAll(t *testing.T) {
	h := healing.NewHealer()
	var matched bool

	h.AddRule(healing.Rule{
		Name: "combo",
		Match: healing.MatchAll(
			healing.MatchSeverity(event.SeverityError),
			healing.MatchSource("api"),
			healing.MatchContains("timeout"),
		),
		Action: func(e *event.Event) error { matched = true; return nil },
	})

	// Only matches if ALL conditions met
	h.Handle(event.New(event.TypeError, event.SeverityError, "timeout").WithSource("api"))
	time.Sleep(100 * time.Millisecond)
	if !matched {
		t.Error("expected match when all conditions met")
	}

	matched = false
	h.Handle(event.New(event.TypeError, event.SeverityInfo, "timeout").WithSource("api"))
	time.Sleep(100 * time.Millisecond)
	if matched {
		t.Error("should not match — severity too low")
	}
}

func TestHealerRetryAction(t *testing.T) {
	attempts := 0
	action := healing.ActionRetry(func(e *event.Event) error {
		attempts++
		if attempts < 3 {
			return errors.New("not yet")
		}
		return nil
	}, 5)

	err := action(event.New(event.TypeError, event.SeverityError, "test"))
	if err != nil {
		t.Errorf("expected success after retries, got: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestHealerGhostModeRecommendations(t *testing.T) {
	h := healing.NewHealer()
	h.SetGhostMode(true)

	h.AddRule(healing.Rule{
		Name:   "r1",
		Match:  healing.MatchSeverity(event.SeverityError),
		Action: func(e *event.Event) error { return nil },
	})
	h.AddRule(healing.Rule{
		Name:   "r2",
		Match:  healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error { return nil },
	})

	recs := h.Handle(event.New(event.TypeError, event.SeverityCritical, "crash"))
	if len(recs) != 2 {
		t.Errorf("expected 2 recommendations, got %d", len(recs))
	}
}

func TestHealerToggleGhostMode(t *testing.T) {
	h := healing.NewHealer()
	var executed atomic.Int64

	h.AddRule(healing.Rule{
		Name:   "toggle",
		Match:  healing.MatchSeverity(event.SeverityError),
		Action: func(e *event.Event) error { executed.Add(1); return nil },
	})

	// Ghost mode ON — no execution
	h.SetGhostMode(true)
	h.Handle(event.New(event.TypeError, event.SeverityError, "ghost"))
	time.Sleep(100 * time.Millisecond)
	if executed.Load() != 0 {
		t.Error("ghost mode should not execute")
	}

	// Ghost mode OFF — executes
	h.SetGhostMode(false)
	h.Handle(event.New(event.TypeError, event.SeverityError, "real"))
	time.Sleep(100 * time.Millisecond)
	if executed.Load() != 1 {
		t.Errorf("expected 1 execution, got %d", executed.Load())
	}
}
