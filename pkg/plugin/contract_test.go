package plugin_test

import (
	"testing"

	"github.com/immortal-engine/immortal/pkg/plugin"
)

// -------------------------------------------------------------------------
// EffectModel round-trip
// -------------------------------------------------------------------------

// doubleReplicaModel is a test effect model that doubles the replica count.
type doubleReplicaModel struct{}

func (d *doubleReplicaModel) Name() string { return "double_replicas" }
func (d *doubleReplicaModel) Apply(s plugin.State, _ map[string]string) (plugin.State, bool) {
	s.Replicas *= 2
	s.Healthy = true
	return s, true
}

func TestAdapters_EffectModelRoundTrips(t *testing.T) {
	model := &doubleReplicaModel{}
	fn := plugin.AsEffectModelFn(model)

	initial := plugin.State{
		Service:  "api",
		Replicas: 3,
		Healthy:  false,
	}

	// Matching action type — should apply and return modeled=true.
	next, modeled := fn(initial, "double_replicas", "api", nil)
	if !modeled {
		t.Fatal("expected modeled=true for matching action type")
	}
	if next.Replicas != 6 {
		t.Errorf("expected Replicas=6, got %d", next.Replicas)
	}
	if !next.Healthy {
		t.Error("expected Healthy=true after apply")
	}
	// Original state must not be mutated.
	if initial.Replicas != 3 {
		t.Errorf("original state mutated: Replicas=%d", initial.Replicas)
	}

	// Non-matching action type — should pass through unchanged.
	passthrough, modeled2 := fn(initial, "some_other_action", "api", nil)
	if modeled2 {
		t.Fatal("expected modeled=false for non-matching action type")
	}
	if passthrough.Replicas != initial.Replicas {
		t.Errorf("pass-through changed state: got Replicas=%d", passthrough.Replicas)
	}
}

// -------------------------------------------------------------------------
// Invariant round-trip
// -------------------------------------------------------------------------

// minOneHealthyInvariant asserts at least one service is healthy.
type minOneHealthyInvariant struct{}

func (m *minOneHealthyInvariant) Name() string        { return "min_one_healthy" }
func (m *minOneHealthyInvariant) Description() string { return "at least one service must be healthy" }
func (m *minOneHealthyInvariant) Holds(world map[string]plugin.ServiceState) bool {
	for _, s := range world {
		if s.Healthy {
			return true
		}
	}
	return false
}

func TestAdapters_InvariantRoundTrips(t *testing.T) {
	inv := &minOneHealthyInvariant{}
	fn := plugin.AsInvariantFn(inv)

	healthy := map[string]plugin.ServiceState{
		"api": {Name: "api", Healthy: true, Replicas: 2},
		"db":  {Name: "db", Healthy: false, Replicas: 1},
	}
	if !fn(healthy) {
		t.Error("expected Holds=true when at least one service is healthy")
	}

	allUnhealthy := map[string]plugin.ServiceState{
		"api": {Name: "api", Healthy: false, Replicas: 0},
		"db":  {Name: "db", Healthy: false, Replicas: 1},
	}
	if fn(allUnhealthy) {
		t.Error("expected Holds=false when all services are unhealthy")
	}

	// Empty world also violates the invariant.
	if fn(map[string]plugin.ServiceState{}) {
		t.Error("expected Holds=false for empty world")
	}
}

// -------------------------------------------------------------------------
// Tool round-trip
// -------------------------------------------------------------------------

// echoTool returns the "msg" argument as its result.
type echoTool struct{}

func (e *echoTool) Name() string                            { return "echo" }
func (e *echoTool) Description() string                     { return "echoes msg argument" }
func (e *echoTool) Schema() map[string]string               { return map[string]string{"msg": "string"} }
func (e *echoTool) CostTier() plugin.CostTier               { return plugin.CostRead }
func (e *echoTool) Invoke(args map[string]any) (string, error) {
	msg, _ := args["msg"].(string)
	return msg, nil
}

func TestAdapters_ToolRoundTrips(t *testing.T) {
	tool := &echoTool{}
	invoker := plugin.AsToolInvoker(tool)

	result, err := invoker(map[string]any{"msg": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected result=hello, got %q", result)
	}

	// Missing argument returns empty string, no error.
	result2, err2 := invoker(map[string]any{})
	if err2 != nil {
		t.Fatalf("unexpected error on empty args: %v", err2)
	}
	if result2 != "" {
		t.Errorf("expected empty result for missing arg, got %q", result2)
	}
}
