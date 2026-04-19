package agentic

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/event"
)

// scriptedPlanner returns steps from a fixed sequence.
type scriptedPlanner struct {
	mu    sync.Mutex
	steps []planStep
	pos   int
}

type planStep struct {
	tool    string
	args    map[string]any
	thought string
}

func (p *scriptedPlanner) NextStep(_ *event.Event, _ []Step) (string, map[string]any, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pos >= len(p.steps) {
		return "finish", map[string]any{"reason": "no more steps"}, "done", nil
	}
	s := p.steps[p.pos]
	p.pos++
	return s.tool, s.args, s.thought, nil
}

func newIncident() *event.Event {
	return event.New(event.TypeError, event.SeverityCritical, "test incident")
}

func TestRegisterAndListTools(t *testing.T) {
	a := New(Config{})
	before := len(a.Tools())
	if before == 0 {
		t.Fatal("expected built-in tools to be registered")
	}

	a.RegisterTool(Tool{
		Name:        "custom_tool",
		Description: "test",
		Schema:      map[string]string{"x": "string"},
		Fn:          func(args map[string]any) (string, error) { return "ok", nil },
	})

	after := len(a.Tools())
	if after != before+1 {
		t.Fatalf("expected %d tools after register, got %d", before+1, after)
	}
}

func TestScriptedPlannerResolves(t *testing.T) {
	healthy := false
	callCount := 0

	healthChecker := func(target string) (string, error) {
		callCount++
		if callCount >= 2 {
			healthy = true
		}
		if healthy {
			return "healthy", nil
		}
		return "unhealthy", nil
	}

	planner := &scriptedPlanner{
		steps: []planStep{
			{tool: "check_health", args: map[string]any{"target": "svc"}, thought: "check first"},
			{tool: "restart_service", args: map[string]any{"name": "svc"}, thought: "restart it"},
			{tool: "check_health", args: map[string]any{"target": "svc"}, thought: "verify"},
			{tool: "finish", args: map[string]any{"reason": "service recovered"}, thought: "done"},
		},
	}

	a := New(Config{
		Planner:       planner,
		HealthChecker: healthChecker,
	})

	trace := a.Run(newIncident())

	if len(trace.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(trace.Steps))
	}
	if !trace.Resolved {
		t.Fatal("expected trace to be resolved")
	}
	if trace.Reason != "service recovered" {
		t.Fatalf("unexpected reason: %s", trace.Reason)
	}
}

func TestMaxIterationsRespected(t *testing.T) {
	infinitePlanner := &scriptedPlanner{}
	// Override NextStep to always return the same tool.
	ap := &alwaysPlanner{tool: "check_health", args: map[string]any{"target": "x"}}

	a := New(Config{
		MaxIterations: 3,
		Planner:       ap,
	})

	trace := a.Run(newIncident())

	if len(trace.Steps) != 3 {
		t.Fatalf("expected exactly 3 steps, got %d", len(trace.Steps))
	}
	if trace.Resolved {
		t.Fatal("trace should not be resolved when max iterations hit")
	}
	_ = infinitePlanner
}

type alwaysPlanner struct {
	tool string
	args map[string]any
}

func (p *alwaysPlanner) NextStep(_ *event.Event, _ []Step) (string, map[string]any, string, error) {
	return p.tool, p.args, "always this tool", nil
}

func TestToolErrorCaptured(t *testing.T) {
	planner := &scriptedPlanner{
		steps: []planStep{
			{tool: "broken_tool", args: map[string]any{}, thought: "try broken tool"},
			{tool: "finish", args: map[string]any{"reason": "gave up"}, thought: "done"},
		},
	}

	a := New(Config{Planner: planner})
	a.RegisterTool(Tool{
		Name:        "broken_tool",
		Description: "always fails",
		Schema:      map[string]string{},
		Fn: func(args map[string]any) (string, error) {
			return "", errors.New("simulated failure")
		},
	})

	trace := a.Run(newIncident())

	if len(trace.Steps) < 2 {
		t.Fatalf("expected at least 2 steps, got %d", len(trace.Steps))
	}
	if trace.Steps[0].Error != "simulated failure" {
		t.Fatalf("expected error captured in step 0, got: %q", trace.Steps[0].Error)
	}
	if !trace.Resolved {
		t.Fatal("loop should have continued and resolved after tool error")
	}
}

func TestConcurrentRuns(t *testing.T) {
	a := New(Config{
		Planner: &scriptedPlanner{
			steps: []planStep{
				{tool: "check_health", args: map[string]any{"target": "svc"}, thought: "check"},
				{tool: "finish", args: map[string]any{"reason": "ok"}, thought: "done"},
			},
		},
	})

	var wg sync.WaitGroup
	results := make([]*Trace, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			// Each goroutine needs its own planner to avoid shared state.
			localAgent := New(Config{
				Planner: &scriptedPlanner{
					steps: []planStep{
						{tool: "check_health", args: map[string]any{"target": "svc"}, thought: "check"},
						{tool: "finish", args: map[string]any{"reason": "ok"}, thought: "done"},
					},
				},
			})
			results[i] = localAgent.Run(newIncident())
		}()
	}

	wg.Wait()

	for i, tr := range results {
		if !tr.Resolved {
			t.Errorf("goroutine %d: trace not resolved", i)
		}
	}
	_ = a
}

func TestDurationIsPositive(t *testing.T) {
	planner := &scriptedPlanner{
		steps: []planStep{
			{tool: "finish", args: map[string]any{"reason": "instant"}, thought: "done"},
		},
	}
	a := New(Config{Planner: planner})
	trace := a.Run(newIncident())
	if trace.Duration < 0 {
		t.Fatalf("expected non-negative duration, got %v", trace.Duration)
	}
}

func TestStepTimestampPopulated(t *testing.T) {
	planner := &scriptedPlanner{
		steps: []planStep{
			{tool: "restart_service", args: map[string]any{"name": "x"}, thought: "restart"},
			{tool: "finish", args: map[string]any{"reason": "done"}, thought: "done"},
		},
	}
	a := New(Config{Planner: planner})
	trace := a.Run(newIncident())
	for _, s := range trace.Steps {
		if s.Timestamp.IsZero() {
			t.Errorf("step %d has zero timestamp", s.Iteration)
		}
	}
}

func TestUnknownToolError(t *testing.T) {
	planner := &scriptedPlanner{
		steps: []planStep{
			{tool: "nonexistent", args: map[string]any{}, thought: "try it"},
			{tool: "finish", args: map[string]any{"reason": "done"}, thought: "done"},
		},
	}
	a := New(Config{Planner: planner})
	trace := a.Run(newIncident())
	if trace.Steps[0].Error == "" {
		t.Fatal("expected error for unknown tool")
	}
}

func TestPlannerErrorBreaksLoop(t *testing.T) {
	a := New(Config{Planner: &errorPlanner{}})
	trace := a.Run(newIncident())
	if len(trace.Steps) != 1 {
		t.Fatalf("expected 1 step after planner error, got %d", len(trace.Steps))
	}
	if trace.Steps[0].Error == "" {
		t.Fatal("expected error captured from planner failure")
	}
}

type errorPlanner struct{}

func (p *errorPlanner) NextStep(_ *event.Event, _ []Step) (string, map[string]any, string, error) {
	return "", nil, "", errors.New("planner internal error")
}

func TestStepTimeoutEnforcedByShortTimeout(t *testing.T) {
	planner := &scriptedPlanner{
		steps: []planStep{
			{tool: "slow_tool", args: map[string]any{}, thought: "slow"},
			{tool: "finish", args: map[string]any{"reason": "done"}, thought: "done"},
		},
	}
	a := New(Config{
		Planner:     planner,
		StepTimeout: 10 * time.Millisecond,
	})
	a.RegisterTool(Tool{
		Name: "slow_tool",
		Fn: func(args map[string]any) (string, error) {
			time.Sleep(200 * time.Millisecond)
			return "done", nil
		},
	})

	trace := a.Run(newIncident())
	if trace.Steps[0].Error == "" {
		t.Fatal("expected timeout error on slow tool")
	}
}
