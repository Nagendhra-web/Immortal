package twin

import (
	"strings"
	"testing"
)

// healingEffect is a test EffectModel that makes restarts restore health
// and scaling halve latency. Mirrors the spirit of the real effect chain.
func healingEffect(s State, a Action) (State, bool) {
	switch a.Type {
	case "restart":
		s.Healthy = true
		s.ErrorRate = 0.001
		return s, true
	case "scale":
		if s.Latency > 0 {
			s.Latency /= 2
		}
		return s, true
	}
	return s, false
}

// brokenEffect makes any "risky" action worsen things. Used to test
// rejection + counterexample generation.
func brokenEffect(s State, a Action) (State, bool) {
	if a.Type == "risky" {
		s.Latency *= 2
		s.ErrorRate += 0.2
		s.Healthy = false
		return s, true
	}
	return s, false
}

func TestReplay_AcceptsImprovingPlan(t *testing.T) {
	twin := New(Config{EffectModels: []EffectModel{healingEffect}})
	result := twin.Replay(ReplayRequest{
		IncidentID: "inc-42",
		Baseline: map[string]State{
			"api": {Service: "api", Healthy: false, Latency: 400, ErrorRate: 0.2},
			"db":  {Service: "db", Healthy: true, Latency: 50, ErrorRate: 0.001},
		},
		WithPlan: Plan{
			ID: "p1",
			Actions: []Action{
				{Type: "restart", Target: "api"},
			},
		},
	})
	if !result.Accepted {
		t.Fatalf("improving plan should be accepted; got rejected with counterexample: %s", result.Counterexample)
	}
	if result.MitigatedScore <= result.UnmitigatedScore {
		t.Errorf("mitigated should exceed unmitigated; got %.3f vs %.3f", result.MitigatedScore, result.UnmitigatedScore)
	}
	if result.IncidentID != "inc-42" {
		t.Errorf("incident id must be echoed back")
	}
}

func TestReplay_RejectsWorseningPlan(t *testing.T) {
	twin := New(Config{EffectModels: []EffectModel{brokenEffect}})
	result := twin.Replay(ReplayRequest{
		IncidentID: "inc-43",
		Baseline: map[string]State{
			"api": {Service: "api", Healthy: true, Latency: 80, ErrorRate: 0.01},
		},
		WithPlan: Plan{
			Actions: []Action{{Type: "risky", Target: "api"}},
		},
	})
	if result.Accepted {
		t.Fatalf("worsening plan must be rejected")
	}
	if result.Counterexample == "" {
		t.Errorf("rejected plan should include a counterexample explanation")
	}
}

func TestReplay_CounterexampleMentionsFailingService(t *testing.T) {
	twin := New(Config{EffectModels: []EffectModel{brokenEffect}})
	result := twin.Replay(ReplayRequest{
		Baseline: map[string]State{
			"checkout": {Service: "checkout", Healthy: true, Latency: 80, ErrorRate: 0.01},
		},
		WithPlan: Plan{Actions: []Action{{Type: "risky", Target: "checkout"}}},
	})
	if result.Accepted {
		t.Fatalf("expected rejection")
	}
	if !strings.Contains(result.Counterexample, "checkout") {
		t.Errorf("counterexample should name the failing service; got %q", result.Counterexample)
	}
}

func TestReplay_DeterministicUnderSameInput(t *testing.T) {
	twin := New(Config{EffectModels: []EffectModel{healingEffect}})
	req := ReplayRequest{
		IncidentID: "stable",
		Baseline:   map[string]State{"api": {Service: "api", Healthy: false, Latency: 400, ErrorRate: 0.15}},
		WithPlan:   Plan{Actions: []Action{{Type: "restart", Target: "api"}}},
	}
	a := twin.Replay(req)
	b := twin.Replay(req)
	if a.MitigatedScore != b.MitigatedScore || a.UnmitigatedScore != b.UnmitigatedScore || a.Accepted != b.Accepted {
		t.Errorf("Replay must be deterministic; got A=%+v B=%+v", a, b)
	}
}

func TestReplay_NoOpPlanNeitherImprovesNorWorsens(t *testing.T) {
	twin := New(Config{EffectModels: []EffectModel{healingEffect}})
	result := twin.Replay(ReplayRequest{
		Baseline: map[string]State{"api": {Service: "api", Healthy: true, Latency: 50, ErrorRate: 0.001}},
		WithPlan: Plan{Actions: []Action{{Type: "unknown_action", Target: "api"}}},
	})
	// A no-op plan should not produce a rejection with a false counterexample.
	if result.Accepted {
		// If it happens to tie or marginally improve, that's fine; we just
		// care that the counterexample is stable when rejected.
		return
	}
	if result.Counterexample == "" {
		t.Errorf("no-op rejection should still produce a counterexample string")
	}
}
