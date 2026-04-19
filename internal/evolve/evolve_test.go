package evolve

import (
	"strings"
	"testing"
)

func TestRuleAddCache_Triggers(t *testing.T) {
	a := New()
	out := a.Analyze(SignalBag{
		LatencyP99:   map[string]float64{"catalog": 220},
		CacheHitRate: map[string]float64{"catalog": 0.45},
	})
	if len(out) == 0 {
		t.Fatal("high-latency + low-cache service should produce an AddCache suggestion")
	}
	found := false
	for _, s := range out {
		if s.Kind == AddCache && s.Service == "catalog" {
			found = true
			if s.Score <= 0 {
				t.Errorf("AddCache score should be positive; got %v", s.Score)
			}
		}
	}
	if !found {
		t.Errorf("AddCache suggestion not found; got %d suggestions", len(out))
	}
}

func TestRuleAddCache_SkipsFastOrCachedService(t *testing.T) {
	a := New()
	out := a.Analyze(SignalBag{
		LatencyP99:   map[string]float64{"fast": 30, "cached": 220},
		CacheHitRate: map[string]float64{"fast": 0.5, "cached": 0.95},
	})
	for _, s := range out {
		if s.Kind == AddCache {
			t.Errorf("should not recommend cache for fast or well-cached services; got %+v", s)
		}
	}
}

func TestRuleSplitService_Triggers(t *testing.T) {
	a := New()
	out := a.Analyze(SignalBag{
		DependentCount:  map[string]int{"shared-utils": 12},
		LatencyCoeffVar: map[string]float64{"shared-utils": 0.8},
	})
	got := firstOfKind(out, SplitService, "shared-utils")
	if got == nil {
		t.Fatal("high fan-in + high CV should suggest SplitService")
	}
}

func TestRuleAddCircuitBreaker(t *testing.T) {
	a := New()
	out := a.Analyze(SignalBag{
		DependencyCount: map[string]int{"edge-api": 8},
		ErrorRate:       map[string]float64{"edge-api": 0.05},
	})
	got := firstOfKind(out, AddCircuitBreaker, "edge-api")
	if got == nil {
		t.Fatal("many deps + errors should suggest AddCircuitBreaker")
	}
	if got.Effort != Small {
		t.Errorf("circuit breaker should be Small effort; got %s", got.Effort)
	}
}

func TestRuleAddRetryBudget_OverridesTightenTimeout_WithHigherScore(t *testing.T) {
	a := New()
	out := a.Analyze(SignalBag{
		RetryRate: map[string]float64{"payments": 0.8},
	})
	// Both TightenTimeout and AddRetryBudget can fire; AddRetryBudget should
	// win because its score is higher at this retry rate.
	var tight, budget *Suggestion
	for i := range out {
		if out[i].Service == "payments" {
			switch out[i].Kind {
			case TightenTimeout:
				tight = &out[i]
			case AddRetryBudget:
				budget = &out[i]
			}
		}
	}
	if budget == nil {
		t.Fatal("expected AddRetryBudget for 0.8 retry rate")
	}
	if tight != nil && tight.Score >= budget.Score {
		t.Errorf("AddRetryBudget should score higher than TightenTimeout at 0.8 retry rate")
	}
}

func TestDedupe_MergesEvidence(t *testing.T) {
	in := []Suggestion{
		{Kind: AddCache, Service: "x", Score: 0.3, Evidence: []string{"a"}},
		{Kind: AddCache, Service: "x", Score: 0.8, Evidence: []string{"b"}},
	}
	out := dedupe(in)
	if len(out) != 1 {
		t.Fatalf("dedupe should collapse same (kind, service); got %d", len(out))
	}
	if out[0].Score != 0.8 {
		t.Errorf("should keep higher score; got %v", out[0].Score)
	}
	if len(out[0].Evidence) != 2 {
		t.Errorf("should merge evidence; got %v", out[0].Evidence)
	}
}

func TestAnalyze_SortsByScoreDescending(t *testing.T) {
	a := New()
	out := a.Analyze(SignalBag{
		LatencyP99:   map[string]float64{"slow": 500, "medium": 150},
		CacheHitRate: map[string]float64{"slow": 0.2, "medium": 0.6},
	})
	for i := 1; i < len(out); i++ {
		if out[i-1].Score < out[i].Score {
			t.Fatalf("suggestions should be sorted by Score desc; out[%d]=%v out[%d]=%v", i-1, out[i-1].Score, i, out[i].Score)
		}
	}
}

func TestFormat_StableOutput(t *testing.T) {
	s := Suggestion{Kind: AddCache, Service: "x", Score: 0.42, Effort: Medium, Rationale: "hot read path."}
	got := s.Format()
	if !strings.Contains(got, "add-cache") || !strings.Contains(got, "score=0.42") || !strings.Contains(got, "x:") {
		t.Errorf("unexpected format: %q", got)
	}
}

func TestEmptyBag(t *testing.T) {
	if got := New().Analyze(SignalBag{}); len(got) != 0 {
		t.Errorf("empty signals should produce no suggestions; got %d", len(got))
	}
}

func firstOfKind(ss []Suggestion, kind Kind, service string) *Suggestion {
	for i := range ss {
		if ss[i].Kind == kind && ss[i].Service == service {
			return &ss[i]
		}
	}
	return nil
}
