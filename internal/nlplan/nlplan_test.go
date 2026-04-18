package nlplan

import (
	"strings"
	"testing"

	"github.com/immortal-engine/immortal/internal/formal"
)

func TestCompiler_GrammarOnlyPath_ProducesValidPlan(t *testing.T) {
	c := NewCompiler(Config{})
	cp, err := c.Compile(CompileRequest{
		Text:     "restart api. keep at least 1 services healthy.",
		Services: []string{"api"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cp.Plan.Steps) != 1 {
		t.Errorf("want 1 step, got %d", len(cp.Plan.Steps))
	}
	if len(cp.Invariants) != 1 {
		t.Errorf("want 1 invariant, got %d", len(cp.Invariants))
	}
}

func TestCompiler_LLMFallback_CalledWhenGrammarEmpty(t *testing.T) {
	called := false
	llmFn := func(sys, user string) (string, error) {
		called = true
		return `{
			"plan_id": "test",
			"steps": [{"action":"restart","target":"api","value":null}],
			"invariants": [{"kind":"at_least_n_healthy","n":1}],
			"trace": [{"span":"fix api","interpreted":"restart(api)"}],
			"warnings": []
		}`, nil
	}
	c := NewCompiler(Config{LLM: llmFn})
	cp, err := c.Compile(CompileRequest{
		Text:     "fix api by doing magic",
		Services: []string{"api"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("LLM was not called")
	}
	if len(cp.Plan.Steps) != 1 {
		t.Errorf("want 1 step, got %d", len(cp.Plan.Steps))
	}
	if len(cp.Invariants) != 1 {
		t.Errorf("want 1 invariant, got %d", len(cp.Invariants))
	}
}

func TestCompiler_LLMReturnsGarbage_ErrorsCleanly(t *testing.T) {
	llmFn := func(sys, user string) (string, error) {
		return "this is not json at all!!!!", nil
	}
	c := NewCompiler(Config{LLM: llmFn})
	_, err := c.Compile(CompileRequest{
		Text: "do something unparseable by grammar",
	})
	if err == nil {
		t.Fatal("want error for garbage LLM response, got nil")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("expected 'invalid JSON' in error, got: %v", err)
	}
}

func TestCompiler_UnknownServiceInRequest_WarningRecorded(t *testing.T) {
	c := NewCompiler(Config{})
	cp, err := c.Compile(CompileRequest{
		Text:     "restart ghost-service",
		Services: []string{"api", "database"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, w := range cp.Warnings {
		if strings.Contains(w, "ghost-service") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning about ghost-service, got: %v", cp.Warnings)
	}
}

func TestCompiler_RespectsMaxSteps(t *testing.T) {
	c := NewCompiler(Config{})
	cp, err := c.Compile(CompileRequest{
		Text:     "restart api. restart database. restart cache.",
		Services: []string{"api", "database", "cache"},
		MaxSteps: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cp.Plan.Steps) > 2 {
		t.Errorf("want at most 2 steps, got %d", len(cp.Plan.Steps))
	}
	truncated := false
	for _, w := range cp.Warnings {
		if strings.Contains(w, "truncated") {
			truncated = true
		}
	}
	if !truncated {
		t.Error("expected truncation warning")
	}
}

func TestCompileAndVerify_DetectsViolation(t *testing.T) {
	c := NewCompiler(Config{})
	initial := formal.World{
		"api": {Name: "api", Healthy: true, Replicas: 3},
	}
	// scale api to 0; invariant: keep api at at least 1 replicas
	cp, result, err := c.CompileAndVerify(CompileRequest{
		Text:     "scale api to 0. api always has at least 1 replicas.",
		Services: []string{"api"},
	}, initial)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = cp
	if result.Safe {
		t.Error("expected violation, got Safe=true")
	}
}

func TestCompileAndVerify_Accepts_SafePlan(t *testing.T) {
	c := NewCompiler(Config{})
	initial := formal.World{
		"api":      {Name: "api", Healthy: false, Replicas: 2},
		"database": {Name: "database", Healthy: true, Replicas: 2},
	}
	cp, result, err := c.CompileAndVerify(CompileRequest{
		Text:     "restart api. keep at least 1 services healthy.",
		Services: []string{"api", "database"},
	}, initial)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = cp
	if !result.Safe {
		t.Errorf("expected Safe plan, got violation: %+v", result.Violation)
	}
}

func TestCompiler_PlainEnglish_EndToEnd(t *testing.T) {
	c := NewCompiler(Config{})
	initial := formal.World{
		"api":      {Name: "api", Healthy: true, Replicas: 2},
		"database": {Name: "database", Healthy: true, Replicas: 2},
	}
	cp, result, err := c.CompileAndVerify(CompileRequest{
		Text:     "Restart the database. Keep at least 2 services healthy.",
		Services: []string{"api", "database"},
	}, initial)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cp.Plan.Steps) != 1 {
		t.Errorf("want 1 step, got %d", len(cp.Plan.Steps))
	}
	if len(cp.Invariants) != 1 {
		t.Errorf("want 1 invariant, got %d", len(cp.Invariants))
	}
	if len(cp.Warnings) != 0 {
		t.Errorf("want no warnings, got %v", cp.Warnings)
	}
	if !result.Safe {
		t.Errorf("expected Safe plan, got violation: %+v", result.Violation)
	}
}
