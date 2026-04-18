package agentic

import (
	"sync/atomic"
	"testing"

	"github.com/immortal-engine/immortal/internal/event"
)

type fixedFinishPlanner struct {
	reason string
	hits   *atomic.Int32
}

func (p *fixedFinishPlanner) NextStep(_ *event.Event, _ []Step) (string, map[string]any, string, error) {
	if p.hits != nil {
		p.hits.Add(1)
	}
	return "finish", map[string]any{"reason": p.reason}, "done", nil
}

func TestMetaAgent_AllBranchesRun(t *testing.T) {
	var counter atomic.Int32
	hyps := []Hypothesis{
		{Name: "h1", Planner: &fixedFinishPlanner{reason: "h1", hits: &counter}, MaxSteps: 2},
		{Name: "h2", Planner: &fixedFinishPlanner{reason: "h2", hits: &counter}, MaxSteps: 2},
		{Name: "h3", Planner: &fixedFinishPlanner{reason: "h3", hits: &counter}, MaxSteps: 2},
	}
	ma := NewMetaAgent(MetaConfig{})
	res := ma.Investigate(event.New(event.TypeError, event.SeverityCritical, "x"), hyps)
	if got := counter.Load(); got != 3 {
		t.Errorf("expected 3 planner calls, got %d", got)
	}
	if len(res.Branches) != 3 {
		t.Errorf("expected 3 traces, got %d", len(res.Branches))
	}
}

func TestMetaAgent_WinnerIsAResolved(t *testing.T) {
	hyps := []Hypothesis{
		{Name: "wins", Planner: &fixedFinishPlanner{reason: "ok"}, MaxSteps: 2},
	}
	ma := NewMetaAgent(MetaConfig{})
	res := ma.Investigate(event.New(event.TypeError, event.SeverityCritical, "x"), hyps)
	if res.Winner < 0 {
		t.Errorf("expected a resolved winner, got -1")
	}
}

func TestMetaAgent_NoBranches(t *testing.T) {
	ma := NewMetaAgent(MetaConfig{})
	res := ma.Investigate(event.New(event.TypeError, event.SeverityCritical, "x"), nil)
	if res == nil {
		t.Fatal("Investigate should never return nil")
	}
	if len(res.Branches) != 0 {
		t.Errorf("expected 0 branches, got %d", len(res.Branches))
	}
}

func TestHypothesisResourceExhaustion_Builds(t *testing.T) {
	checker := func(target, metric string) (float64, error) { return 42.0, nil }
	h := HypothesisResourceExhaustion("svc-a", checker)
	if h.Planner == nil {
		t.Error("hypothesis should have a planner")
	}
}

func TestHypothesisDependencyFailure_Builds(t *testing.T) {
	checker := func(target string) ([]string, error) { return []string{"db"}, nil }
	h := HypothesisDependencyFailure("api", checker)
	if h.Planner == nil {
		t.Error("hypothesis should have a planner")
	}
}
