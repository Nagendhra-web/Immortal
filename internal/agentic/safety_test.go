package agentic

import (
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/event"
)

// destructiveTool is a helper that returns a Tool with CostDestructive tier.
func destructiveTool(name string) Tool {
	return Tool{
		Name:        name,
		Description: "destructive test tool",
		CostTier:    CostDestructive,
		Reversible:  false,
		BlastRadius: 2,
		Fn:          func(args map[string]any) (string, error) { return "done:" + name, nil },
	}
}

// disruptiveTool returns a Tool with CostDisruptive tier.
func disruptiveTool(name string) Tool {
	return Tool{
		Name:        name,
		Description: "disruptive test tool",
		CostTier:    CostDisruptive,
		Reversible:  true,
		BlastRadius: 1,
		Fn:          func(args map[string]any) (string, error) { return "done:" + name, nil },
	}
}

// TestSafetyPolicy_MaxDestructive_Blocks verifies the second destructive call
// is blocked when MaxDestructivePerRun=1.
func TestSafetyPolicy_MaxDestructive_Blocks(t *testing.T) {
	policy := &SafetyPolicy{MaxDestructivePerRun: 1}
	tool := destructiveTool("failover")

	// First call: no history → should be allowed.
	v := policy.Guard(tool, nil, []Step{})
	if v != nil {
		t.Fatalf("expected first destructive call to pass, got violation: %s", v.Reason)
	}

	// Simulate history with one successful destructive call.
	history := []Step{
		{Tool: "failover", Observation: "failover:done", Error: ""},
	}

	// Second call: should be blocked.
	v = policy.Guard(tool, nil, history)
	if v == nil {
		t.Fatal("expected second destructive call to be blocked")
	}
	if v.Tool != "failover" {
		t.Errorf("violation Tool: got %q want %q", v.Tool, "failover")
	}
}

// TestSafetyPolicy_ForbiddenTool_Blocks verifies that any call to a forbidden
// tool is rejected regardless of history.
func TestSafetyPolicy_ForbiddenTool_Blocks(t *testing.T) {
	policy := &SafetyPolicy{ForbiddenTools: []string{"nuke_cluster"}}
	tool := Tool{Name: "nuke_cluster", CostTier: CostDestructive, BlastRadius: 1}

	v := policy.Guard(tool, nil, []Step{})
	if v == nil {
		t.Fatal("expected forbidden tool to be blocked")
	}
	if v.Tool != "nuke_cluster" {
		t.Errorf("violation Tool: got %q", v.Tool)
	}
}

// TestSafetyPolicy_RequireDryRunFor_Enforced verifies that calling scale
// without a prior dry_run is blocked, but succeeds after one.
func TestSafetyPolicy_RequireDryRunFor_Enforced(t *testing.T) {
	policy := &SafetyPolicy{RequireDryRunFor: []string{"scale"}}
	tool := disruptiveTool("scale")

	// No dry_run in history → blocked.
	v := policy.Guard(tool, nil, []Step{})
	if v == nil {
		t.Fatal("expected scale without dry_run to be blocked")
	}

	// History with a dry_run for "scale" → allowed.
	history := []Step{
		{
			Tool:        "dry_run",
			ToolArgs:    map[string]any{"tool": "scale"},
			Observation: "dry_run:scale:ok",
			Error:       "",
		},
	}
	v = policy.Guard(tool, nil, history)
	if v != nil {
		t.Fatalf("expected scale after dry_run to pass, got violation: %s", v.Reason)
	}
}

// forbiddenPlanner always requests the forbidden tool.
type forbiddenPlanner struct {
	toolName string
}

func (p *forbiddenPlanner) NextStep(_ *event.Event, _ []Step) (string, map[string]any, string, error) {
	return p.toolName, map[string]any{}, "trying forbidden tool", nil
}

// TestSafetyViolation_HaltsLoopAfterThree verifies that after 3 consecutive
// safety violations the loop exits with Resolved=false and a policy halt reason.
func TestSafetyViolation_HaltsLoopAfterThree(t *testing.T) {
	a := New(Config{
		MaxIterations: 10,
		Planner:       &forbiddenPlanner{toolName: "banned"},
		Safety: &SafetyPolicy{
			ForbiddenTools: []string{"banned"},
		},
	})

	// Register the tool so the loop doesn't reject it as unknown before safety check.
	a.RegisterTool(Tool{
		Name:        "banned",
		Description: "a banned tool",
		CostTier:    CostDestructive,
		BlastRadius: 1,
		Fn:          func(args map[string]any) (string, error) { return "oops", nil },
	})

	trace := a.Run(newIncident())

	if trace.Resolved {
		t.Fatal("expected Resolved=false when safety halts loop")
	}
	if trace.Reason != "safety policy halted the loop" {
		t.Errorf("unexpected Reason: %q", trace.Reason)
	}
	// Should have exactly 3 steps (the 3 violations before halting).
	if len(trace.Steps) != 3 {
		t.Errorf("expected 3 steps before halt, got %d", len(trace.Steps))
	}
	for i, s := range trace.Steps {
		if s.Error == "" {
			t.Errorf("step %d: expected safety violation error, got empty", i)
		}
	}
}
