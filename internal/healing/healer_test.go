package healing_test

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/healing"
)

func TestHealerMatchesRule(t *testing.T) {
	var executed []string
	var mu sync.Mutex

	h := healing.NewHealer()
	h.AddRule(healing.Rule{
		Name:  "restart-on-crash",
		Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error {
			mu.Lock()
			executed = append(executed, "restart")
			mu.Unlock()
			return nil
		},
	})

	h.Handle(event.New(event.TypeError, event.SeverityCritical, "process crashed"))
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(executed) != 1 || executed[0] != "restart" {
		t.Errorf("expected ['restart'], got %v", executed)
	}
}

func TestHealerIgnoresNonMatchingEvents(t *testing.T) {
	callCount := 0
	h := healing.NewHealer()
	h.AddRule(healing.Rule{
		Name:  "crash-only",
		Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error {
			callCount++
			return nil
		},
	})

	h.Handle(event.New(event.TypeLog, event.SeverityInfo, "all good"))
	time.Sleep(100 * time.Millisecond)

	if callCount != 0 {
		t.Errorf("expected 0 calls, got %d", callCount)
	}
}

func TestHealerMatchBySource(t *testing.T) {
	var matched bool
	var mu sync.Mutex

	h := healing.NewHealer()
	h.AddRule(healing.Rule{
		Name:  "api-crash",
		Match: healing.MatchSource("api-server"),
		Action: func(e *event.Event) error {
			mu.Lock()
			matched = true
			mu.Unlock()
			return nil
		},
	})

	h.Handle(event.New(event.TypeError, event.SeverityError, "crash").WithSource("api-server"))
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if !matched {
		t.Error("expected rule to match api-server source")
	}
}

func TestHealerGhostMode(t *testing.T) {
	actionCalled := false
	h := healing.NewHealer()
	h.SetGhostMode(true)

	h.AddRule(healing.Rule{
		Name:  "restart",
		Match: healing.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error {
			actionCalled = true
			return nil
		},
	})

	recommendations := h.Handle(event.New(event.TypeError, event.SeverityCritical, "crash"))
	time.Sleep(100 * time.Millisecond)

	if actionCalled {
		t.Error("ghost mode should NOT execute actions")
	}
	if len(recommendations) == 0 {
		t.Error("ghost mode should return recommendations")
	}
}

func TestActionExecCommand(t *testing.T) {
	cmd := "echo hello"
	if runtime.GOOS == "windows" {
		cmd = "cmd /c echo hello"
	}

	action := healing.ActionExec(cmd)
	err := action(event.New(event.TypeError, event.SeverityError, "test"))
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestActionExecCommandFailure(t *testing.T) {
	action := healing.ActionExec("nonexistent_command_12345")
	err := action(event.New(event.TypeError, event.SeverityError, "test"))
	if err == nil {
		t.Error("expected error for nonexistent command")
	}
}

func TestActionComposite(t *testing.T) {
	var order []int
	a1 := func(e *event.Event) error { order = append(order, 1); return nil }
	a2 := func(e *event.Event) error { order = append(order, 2); return nil }

	composite := healing.ActionSequence(a1, a2)
	err := composite(event.New(event.TypeError, event.SeverityError, "test"))

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Errorf("expected [1, 2], got %v", order)
	}
}
