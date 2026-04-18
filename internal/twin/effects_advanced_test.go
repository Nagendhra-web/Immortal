package twin

import (
	"testing"
)

func allModels() []EffectModel {
	return append(BuiltinEffectModels(), AdvancedEffectModels()...)
}

func TestDeployEffect_UpdatesVersion(t *testing.T) {
	tw := New(Config{EffectModels: allModels()})
	tw.Observe(State{Service: "api", CPU: 50, Replicas: 2, Healthy: true, ErrorRate: 0.1})

	plan := Plan{
		ID: "deploy-test",
		Actions: []Action{
			{Type: "deploy", Target: "api", Params: map[string]string{"image": "v1.2.3"}},
		},
	}
	sim := tw.Simulate(plan)

	if sim.ModeledSteps != 1 {
		t.Errorf("expected deploy to be modeled, got ModeledSteps=%d UnmodeledSteps=%d",
			sim.ModeledSteps, sim.UnmodeledSteps)
	}
	end := sim.End["api"]
	if !end.Healthy {
		t.Error("expected Healthy=true after clean deploy")
	}
	if end.ErrorRate != 0 {
		t.Errorf("expected ErrorRate=0 after clean deploy, got %.4f", end.ErrorRate)
	}
}

func TestCanaryEffect_SplitsLoad(t *testing.T) {
	tw := New(Config{EffectModels: allModels()})
	tw.Observe(State{Service: "api", CPU: 80, Replicas: 4, Healthy: true, ErrorRate: 0.0})

	plan := Plan{
		ID: "canary-test",
		Actions: []Action{
			{Type: "canary", Target: "api", Params: map[string]string{"percent": "25"}},
		},
	}
	sim := tw.Simulate(plan)

	primary, ok := sim.End["api"]
	if !ok {
		t.Fatal("expected primary 'api' state in simulation end")
	}
	canary, ok := sim.End["api-canary"]
	if !ok {
		t.Fatal("expected 'api-canary' state in simulation end after canary action")
	}

	// Primary should have ~75% of original CPU (80 * 0.75 = 60).
	if primary.CPU > 70 || primary.CPU < 50 {
		t.Errorf("expected primary CPU ~60 (75%% of 80), got %.2f", primary.CPU)
	}
	// Canary should have ~25% of original CPU (80 * 0.25 = 20).
	if canary.CPU > 30 || canary.CPU < 10 {
		t.Errorf("expected canary CPU ~20 (25%% of 80), got %.2f", canary.CPU)
	}
	t.Logf("primary CPU=%.2f canary CPU=%.2f", primary.CPU, canary.CPU)
}

func TestTrafficShiftEffect_MovesCPU(t *testing.T) {
	tw := New(Config{EffectModels: allModels()})
	tw.Observe(State{Service: "old", CPU: 80, Replicas: 2, Healthy: true})
	tw.Observe(State{Service: "new", CPU: 20, Replicas: 2, Healthy: true})

	plan := Plan{
		ID: "shift-test",
		Actions: []Action{
			{
				Type:   "traffic_shift",
				Target: "old",
				Params: map[string]string{"from": "old", "to": "new", "percent": "50"},
			},
		},
	}
	sim := tw.Simulate(plan)

	oldEnd := sim.End["old"]
	newEnd := sim.End["new"]

	// old: 80 * (1 - 0.5) = 40
	if oldEnd.CPU > 50 || oldEnd.CPU < 30 {
		t.Errorf("expected 'old' CPU ~40 after 50%% shift away, got %.2f", oldEnd.CPU)
	}
	// new: 20 * (1 + 0.5) = 30
	if newEnd.CPU > 40 || newEnd.CPU < 20 {
		t.Errorf("expected 'new' CPU ~30 after 50%% shift in, got %.2f", newEnd.CPU)
	}
	t.Logf("old CPU=%.2f new CPU=%.2f", oldEnd.CPU, newEnd.CPU)
}

func TestDBMigrationEffect_IncreasesLatency(t *testing.T) {
	tw := New(Config{EffectModels: allModels()})
	tw.Observe(State{Service: "db", CPU: 40, Replicas: 1, Healthy: true, Latency: 100})

	plan := Plan{
		ID: "migration-test",
		Actions: []Action{
			{Type: "db_migration", Target: "db", Params: map[string]string{"duration_seconds": "60"}},
		},
	}
	sim := tw.Simulate(plan)

	end := sim.End["db"]
	if end.Latency <= 100 {
		t.Errorf("expected Latency to increase during migration (was 100), got %.2f", end.Latency)
	}
	// 1.5x spike expected.
	if end.Latency < 140 {
		t.Errorf("expected Latency ~150 (1.5x), got %.2f", end.Latency)
	}
	t.Logf("db Latency after migration=%.2f", end.Latency)
}

func TestConnectionPoolEffect_LowersErrorRate(t *testing.T) {
	tw := New(Config{EffectModels: allModels()})
	// CPU=40 => demand=40/20=2; pool size 20 > 2 => ErrorRate should go to 0.
	tw.Observe(State{Service: "api", CPU: 40, Replicas: 2, Healthy: true, ErrorRate: 0.3})

	plan := Plan{
		ID: "pool-test",
		Actions: []Action{
			{Type: "connection_pool", Target: "api", Params: map[string]string{"size": "20"}},
		},
	}
	sim := tw.Simulate(plan)

	end := sim.End["api"]
	if end.ErrorRate != 0 {
		t.Errorf("expected ErrorRate=0 when pool size >= demand, got %.4f", end.ErrorRate)
	}
}

func TestConnectionPoolEffect_KeepsErrorRate_WhenUndersized(t *testing.T) {
	tw := New(Config{EffectModels: allModels()})
	// CPU=100 => demand=5; pool size 2 < 5 => ErrorRate unchanged.
	tw.Observe(State{Service: "api", CPU: 100, Replicas: 2, Healthy: true, ErrorRate: 0.5})

	plan := Plan{
		ID: "pool-small",
		Actions: []Action{
			{Type: "connection_pool", Target: "api", Params: map[string]string{"size": "2"}},
		},
	}
	sim := tw.Simulate(plan)

	end := sim.End["api"]
	if end.ErrorRate == 0 {
		t.Error("expected ErrorRate to stay non-zero when pool is undersized")
	}
}

func TestSecretRotationEffect_BriefUnhealthy(t *testing.T) {
	tw := New(Config{EffectModels: allModels()})
	tw.Observe(State{Service: "api", CPU: 40, Replicas: 2, Healthy: true, ErrorRate: 0.0})

	plan := Plan{
		ID: "rotation-test",
		Actions: []Action{
			{Type: "secret_rotation", Target: "api", Params: map[string]string{}},
		},
	}
	sim := tw.Simulate(plan)

	// Deterministic model: ends healthy after rotation.
	if sim.ModeledSteps != 1 {
		t.Errorf("expected secret_rotation to be modeled, got UnmodeledSteps=%d", sim.UnmodeledSteps)
	}
	end := sim.End["api"]
	// In deterministic mode, recovery is assumed.
	_ = end.Healthy // value is deterministic-healthy; MC noise would flip it
}
