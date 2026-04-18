package twin

import (
	"sync"
	"testing"
)

func TestObserveAndGet(t *testing.T) {
	tw := New(Config{})
	s := State{Service: "api", CPU: 50, Memory: 60, Replicas: 2, Healthy: true, Latency: 100, ErrorRate: 0.01}
	tw.Observe(s)

	got, ok := tw.Get("api")
	if !ok {
		t.Fatal("expected to find 'api' state")
	}
	if got.CPU != 50 || got.Replicas != 2 || !got.Healthy {
		t.Errorf("unexpected state: %+v", got)
	}

	_, ok = tw.Get("nonexistent")
	if ok {
		t.Error("expected false for nonexistent service")
	}
}

func TestSimulateDoesNotMutateTwin(t *testing.T) {
	tw := New(Config{})
	tw.Observe(State{Service: "svc", CPU: 80, Replicas: 2, Healthy: false, ErrorRate: 0.5})

	before := tw.States()

	plan := Plan{ID: "p1", Actions: []Action{{Type: "restart", Target: "svc"}}}
	tw.Simulate(plan)

	after := tw.States()
	if after["svc"].Healthy != before["svc"].Healthy {
		t.Error("Simulate mutated twin state")
	}
	if after["svc"].ErrorRate != before["svc"].ErrorRate {
		t.Error("Simulate mutated twin ErrorRate")
	}
}

func TestDefaultScore_HealthyBeatsUnhealthy(t *testing.T) {
	healthy := map[string]State{
		"svc": {Service: "svc", Healthy: true, Replicas: 2, ErrorRate: 0, Latency: 10},
	}
	unhealthy := map[string]State{
		"svc": {Service: "svc", Healthy: false, Replicas: 2, ErrorRate: 0.5, Latency: 500},
	}
	hs := DefaultScore(healthy)
	us := DefaultScore(unhealthy)
	if hs <= us {
		t.Errorf("healthy score (%.2f) should beat unhealthy score (%.2f)", hs, us)
	}
}

func TestRestartEffect_ImprovesHealth(t *testing.T) {
	tw := New(Config{})
	tw.Observe(State{Service: "api", Healthy: false, ErrorRate: 0.9, CPU: 95, Replicas: 1})

	plan := Plan{ID: "p2", Actions: []Action{{Type: "restart", Target: "api"}}}
	sim := tw.Simulate(plan)

	end := sim.End["api"]
	if !end.Healthy {
		t.Error("expected Healthy=true after restart")
	}
	if end.ErrorRate != 0 {
		t.Errorf("expected ErrorRate=0 after restart, got %.2f", end.ErrorRate)
	}
	if end.CPU >= 95 {
		t.Errorf("expected CPU < 95 after restart, got %.2f", end.CPU)
	}
}

func TestScaleEffect_IncreasesReplicas(t *testing.T) {
	tw := New(Config{})
	tw.Observe(State{Service: "worker", Replicas: 2, CPU: 80, Latency: 200, Healthy: true})

	plan := Plan{
		ID: "p3",
		Actions: []Action{
			{Type: "scale", Target: "worker", Params: map[string]string{"replicas": "4"}},
		},
	}
	sim := tw.Simulate(plan)

	end := sim.End["worker"]
	if end.Replicas != 4 {
		t.Errorf("expected 4 replicas, got %d", end.Replicas)
	}
	// CPU should halve (2->4 doubles capacity)
	if end.CPU >= 80 {
		t.Errorf("expected CPU < 80 after scale, got %.2f", end.CPU)
	}
}

func TestDependencyRecoveryPropagates(t *testing.T) {
	tw := New(Config{})
	// A depends on B
	tw.Observe(State{Service: "A", Latency: 300, ErrorRate: 0.4, Healthy: true, Replicas: 1, Dependencies: []string{"B"}})
	tw.Observe(State{Service: "B", Healthy: false, ErrorRate: 0.8, Replicas: 1})

	plan := Plan{ID: "p4", Actions: []Action{{Type: "restart", Target: "B"}}}
	sim := tw.Simulate(plan)

	endA := sim.End["A"]
	if endA.ErrorRate >= 0.4 {
		t.Errorf("expected A's ErrorRate to decrease after B restart, got %.2f", endA.ErrorRate)
	}
	if endA.Latency >= 300 {
		t.Errorf("expected A's Latency to decrease after B restart, got %.2f", endA.Latency)
	}
}

func TestPlanRejected_WhenScoreWorsens(t *testing.T) {
	// Custom effect model that actively worsens state
	worseningModel := func(s State, a Action) (State, bool) {
		if a.Type != "noop" {
			return s, false
		}
		s.Healthy = false
		s.ErrorRate = 1.0
		s.CPU = 100
		return s, true
	}

	tw := New(Config{
		EffectModels: []EffectModel{worseningModel},
	})
	tw.Observe(State{Service: "svc", Healthy: true, Replicas: 2, ErrorRate: 0.0, CPU: 50})

	plan := Plan{ID: "bad", Actions: []Action{{Type: "noop", Target: "svc"}}}
	sim := tw.Simulate(plan)

	if !sim.Rejected {
		t.Errorf("expected plan to be rejected, got Accepted=true, reason: %s", sim.Reason)
	}
	if sim.Accepted {
		t.Error("expected Accepted=false")
	}
}

func TestUnmodeledActionCounted(t *testing.T) {
	tw := New(Config{})
	tw.Observe(State{Service: "svc", Replicas: 1, Healthy: true})

	plan := Plan{ID: "p5", Actions: []Action{{Type: "frobnicate", Target: "svc"}}}
	sim := tw.Simulate(plan)

	if sim.UnmodeledSteps != 1 {
		t.Errorf("expected UnmodeledSteps=1, got %d", sim.UnmodeledSteps)
	}
	if sim.ModeledSteps != 0 {
		t.Errorf("expected ModeledSteps=0, got %d", sim.ModeledSteps)
	}
}

func TestConcurrentObserveSimulate(t *testing.T) {
	tw := New(Config{})
	tw.Observe(State{Service: "api", CPU: 50, Replicas: 2, Healthy: true})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			tw.Observe(State{Service: "api", CPU: float64(n % 100), Replicas: 2, Healthy: true})
		}(i)
		go func() {
			defer wg.Done()
			plan := Plan{ID: "concurrent", Actions: []Action{{Type: "restart", Target: "api"}}}
			tw.Simulate(plan)
		}()
	}
	wg.Wait()
}
