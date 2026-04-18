package agentic

import (
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
)

func makeStep(iter int, tool, observation, errStr, thought string) Step {
	return Step{
		Iteration:   iter,
		Tool:        tool,
		Observation: observation,
		Error:       errStr,
		Thought:     thought,
		Timestamp:   time.Now(),
	}
}

// TestDefaultReflector_RestartFollowedByUnhealthy_Critiques verifies that
// after a restart action the reflector notices the service is still unhealthy
// and suggests failover.
func TestDefaultReflector_RestartFollowedByUnhealthy_Critiques(t *testing.T) {
	r := DefaultReflector{}
	step := makeStep(0, "restart_service", "unhealthy", "", "restart the service")

	ref := r.Reflect(step, nil)

	if ref.Matched {
		t.Error("expected Matched=false for restart+unhealthy")
	}
	if ref.Critique != "restart insufficient, try failover" {
		t.Errorf("unexpected critique: %q", ref.Critique)
	}
	if ref.Confidence >= 0.5 {
		t.Errorf("expected low confidence, got %.2f", ref.Confidence)
	}
	if ref.Step != 0 {
		t.Errorf("expected Step=0, got %d", ref.Step)
	}
}

// TestDefaultReflector_ToolError_Critiques verifies that a tool error produces
// the "try a different approach" critique.
func TestDefaultReflector_ToolError_Critiques(t *testing.T) {
	r := DefaultReflector{}
	step := makeStep(1, "some_tool", "", "connection refused", "try some_tool")

	ref := r.Reflect(step, nil)

	if ref.Matched {
		t.Error("expected Matched=false for tool error")
	}
	if ref.Critique != "tool failed; try a different approach" {
		t.Errorf("unexpected critique: %q", ref.Critique)
	}
	if ref.Confidence > 0.2 {
		t.Errorf("expected very low confidence for tool error, got %.2f", ref.Confidence)
	}
}

// TestDefaultReflector_HealthyObservation_HighConfidence ensures a healthy
// observation produces a positive, high-confidence reflection.
func TestDefaultReflector_HealthyObservation_HighConfidence(t *testing.T) {
	r := DefaultReflector{}
	step := makeStep(2, "check_health", "healthy", "", "check health")

	ref := r.Reflect(step, nil)

	if !ref.Matched {
		t.Error("expected Matched=true for healthy observation")
	}
	if ref.Confidence < 0.8 {
		t.Errorf("expected high confidence, got %.2f", ref.Confidence)
	}
}

// TestReflectingPlannerReceivesHistory verifies that when the planner
// implements ReflectingPlanner, the agent calls NextStepWithReflection and
// passes the accumulated reflection slice.
func TestReflectingPlannerReceivesHistory(t *testing.T) {
	var capturedReflections []Reflection

	// Build a scripted planner that satisfies ReflectingPlanner.
	inner := &scriptedPlanner{
		steps: []planStep{
			{tool: "check_health", args: map[string]any{"target": "svc"}, thought: "check first"},
			{tool: "restart_service", args: map[string]any{"name": "svc"}, thought: "restart"},
			{tool: "finish", args: map[string]any{"reason": "done"}, thought: "done"},
		},
	}

	rp := &reflectingPlannerAdapter{
		Planner: inner,
		fn: func(inc *event.Event, history []Step, reflections []Reflection) (string, map[string]any, string, error) {
			capturedReflections = make([]Reflection, len(reflections))
			copy(capturedReflections, reflections)
			return inner.NextStep(inc, history)
		},
	}

	a := New(Config{
		Planner: rp,
	})

	trace := a.Run(newIncident())

	// The loop ran at least 2 tool steps before finish, so we should have
	// received reflections by the time finish was called.
	if len(capturedReflections) == 0 {
		t.Fatal("expected ReflectingPlanner to receive at least one reflection")
	}
	// Each captured reflection should have its Step field set.
	for i, ref := range capturedReflections {
		if ref.Step < 0 {
			t.Errorf("reflection %d: invalid Step %d", i, ref.Step)
		}
		if ref.Critique == "" {
			t.Errorf("reflection %d: empty Critique", i)
		}
	}
	if !trace.Resolved {
		t.Error("expected trace to be resolved")
	}
}
