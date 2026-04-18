package agentic

import (
	"sync/atomic"
	"testing"

	"github.com/immortal-engine/immortal/internal/event"
)

// TestRealWorldScenario_DatabaseDown_AgentRestartsAndVerifies simulates a
// realistic incident: the DB is down, the agent detects it, restarts it, then
// confirms recovery before finishing.
func TestRealWorldScenario_DatabaseDown_AgentRestartsAndVerifies(t *testing.T) {
	// restartCount tracks how many times restart_service has been called so
	// the health checker can flip state after the first restart.
	var restartCount atomic.Int32

	healthChecker := func(target string) (string, error) {
		if target == "db" && restartCount.Load() == 0 {
			return "unhealthy", nil
		}
		return "healthy", nil
	}

	planner := &scriptedPlanner{
		steps: []planStep{
			{
				tool:    "check_health",
				args:    map[string]any{"target": "db"},
				thought: "incident says db connection refused — check health first",
			},
			{
				tool:    "restart_service",
				args:    map[string]any{"name": "db"},
				thought: "db is unhealthy, restart it",
			},
			{
				tool:    "check_health",
				args:    map[string]any{"target": "db"},
				thought: "verify db recovered after restart",
			},
			{
				tool:    "finish",
				args:    map[string]any{"reason": "db is healthy after restart"},
				thought: "incident resolved",
			},
		},
	}

	a := New(Config{
		Planner:       planner,
		HealthChecker: healthChecker,
	})

	// Override restart_service to also increment restartCount so the health
	// checker knows the restart happened.
	a.RegisterTool(Tool{
		Name:        "restart_service",
		Description: "Restart a named service (stateful for this test).",
		Schema:      map[string]string{"name": "string"},
		Fn: func(args map[string]any) (string, error) {
			name, _ := args["name"].(string)
			restartCount.Add(1)
			return "restarted:" + name, nil
		},
	})

	inc := event.New(event.TypeError, event.SeverityCritical, "db connection refused").
		WithSource("db-monitor").
		WithMeta("host", "db-primary-01")

	trace := a.Run(inc)

	if trace.IncidentID != inc.ID {
		t.Errorf("IncidentID mismatch: got %s want %s", trace.IncidentID, inc.ID)
	}
	if !trace.Resolved {
		t.Fatal("expected trace to be resolved")
	}
	if trace.Reason != "db is healthy after restart" {
		t.Fatalf("unexpected reason: %q", trace.Reason)
	}
	if len(trace.Steps) < 4 {
		t.Fatalf("expected at least 4 steps, got %d", len(trace.Steps))
	}
	if trace.Duration < 0 {
		t.Fatalf("expected non-negative duration, got %v", trace.Duration)
	}

	// Verify step sequence makes sense.
	assertStep(t, trace.Steps[0], 0, "check_health", "unhealthy")
	assertStep(t, trace.Steps[1], 1, "restart_service", "restarted:db")
	assertStep(t, trace.Steps[2], 2, "check_health", "healthy")
	// Step 3 is "finish" — no tool execution, loop exits cleanly.
	if trace.Steps[3].Tool != "finish" {
		t.Errorf("step 3 tool: got %q want %q", trace.Steps[3].Tool, "finish")
	}

	if restartCount.Load() != 1 {
		t.Errorf("expected exactly 1 restart, got %d", restartCount.Load())
	}
}

func assertStep(t *testing.T, s Step, iter int, tool, observation string) {
	t.Helper()
	if s.Iteration != iter {
		t.Errorf("step iteration: got %d want %d", s.Iteration, iter)
	}
	if s.Tool != tool {
		t.Errorf("step %d tool: got %q want %q", iter, s.Tool, tool)
	}
	if s.Observation != observation {
		t.Errorf("step %d observation: got %q want %q", iter, s.Observation, observation)
	}
	if s.Error != "" {
		t.Errorf("step %d unexpected error: %q", iter, s.Error)
	}
	if s.Timestamp.IsZero() {
		t.Errorf("step %d has zero timestamp", iter)
	}
}
