package twin

import (
	"testing"
)

func TestSimulateMC_DeterministicWithSeed(t *testing.T) {
	tw := New(Config{})
	tw.Observe(State{Service: "api", CPU: 70, Memory: 60, Replicas: 2, Healthy: false, ErrorRate: 0.5, Latency: 200})

	plan := Plan{ID: "p1", Actions: []Action{{Type: "restart", Target: "api"}}}
	cfg := MCConfig{Runs: 100, NoiseScale: 0.05, Seed: 42}

	mc1 := tw.SimulateMC(plan, cfg)
	mc2 := tw.SimulateMC(plan, cfg)

	if mc1.ScoreP50 != mc2.ScoreP50 {
		t.Errorf("P50 not deterministic with same seed: %.4f vs %.4f", mc1.ScoreP50, mc2.ScoreP50)
	}
	if mc1.ScoreP95 != mc2.ScoreP95 {
		t.Errorf("P95 not deterministic with same seed: %.4f vs %.4f", mc1.ScoreP95, mc2.ScoreP95)
	}
}

func TestSimulateMC_P95WiderThanMean(t *testing.T) {
	tw := New(Config{})
	tw.Observe(State{Service: "api", CPU: 60, Memory: 50, Replicas: 2, Healthy: true, ErrorRate: 0.1, Latency: 100})

	plan := Plan{ID: "p2", Actions: []Action{{Type: "noop", Target: "api"}}}
	cfg := MCConfig{Runs: 500, NoiseScale: 0.2, Seed: 99}

	mc := tw.SimulateMC(plan, cfg)

	spread := mc.ScoreP95 - mc.ScoreP05
	if spread <= 0 {
		t.Errorf("expected P95-P05 > 0 with NoiseScale=0.2, got spread=%.4f (P05=%.4f P95=%.4f)",
			spread, mc.ScoreP05, mc.ScoreP95)
	}
	t.Logf("P05=%.4f P50=%.4f P95=%.4f spread=%.4f", mc.ScoreP05, mc.ScoreP50, mc.ScoreP95, spread)
}

func TestSimulateMC_RejectsBasedOnP05(t *testing.T) {
	// Use a custom effect model that makes things much worse.
	worseningModel := func(s State, a Action) (State, bool) {
		if a.Type != "degrade" {
			return s, false
		}
		s.ErrorRate = 1.0
		s.Healthy = false
		s.CPU = 100
		return s, true
	}

	tw := New(Config{
		EffectModels: []EffectModel{worseningModel},
		Tolerance:    0,
	})
	tw.Observe(State{Service: "svc", CPU: 50, Replicas: 2, Healthy: true, ErrorRate: 0.0, Latency: 50})

	plan := Plan{ID: "bad", Actions: []Action{{Type: "degrade", Target: "svc"}}}
	cfg := MCConfig{Runs: 200, NoiseScale: 0.05, Seed: 7}

	mc := tw.SimulateMC(plan, cfg)

	if mc.Accepted {
		t.Errorf("expected MC to reject plan when P05 is much worse than start; P05=%.4f StartScore=%.4f",
			mc.ScoreP05, mc.StartScore)
	}
	if !mc.Rejected {
		t.Error("expected Rejected=true")
	}
	t.Logf("P05=%.4f StartScore=%.4f", mc.ScoreP05, mc.StartScore)
}

func TestSimulateMC_RunsCount(t *testing.T) {
	tw := New(Config{})
	tw.Observe(State{Service: "api", CPU: 50, Replicas: 1, Healthy: true})

	plan := Plan{ID: "p3", Actions: []Action{{Type: "noop", Target: "api"}}}
	cfg := MCConfig{Runs: 50, Seed: 1}

	mc := tw.SimulateMC(plan, cfg)
	if mc.Runs != 50 {
		t.Errorf("expected Runs=50, got %d", mc.Runs)
	}
}

func TestSimulateMC_BestWorse(t *testing.T) {
	tw := New(Config{})
	tw.Observe(State{Service: "api", CPU: 70, Replicas: 2, Healthy: true, ErrorRate: 0.1, Latency: 100})

	plan := Plan{ID: "p4", Actions: []Action{{Type: "noop", Target: "api"}}}
	cfg := MCConfig{Runs: 200, NoiseScale: 0.15, Seed: 123}

	mc := tw.SimulateMC(plan, cfg)

	if mc.Best.EndScore < mc.Worst.EndScore {
		t.Errorf("Best.EndScore (%.4f) should be >= Worst.EndScore (%.4f)",
			mc.Best.EndScore, mc.Worst.EndScore)
	}
}
