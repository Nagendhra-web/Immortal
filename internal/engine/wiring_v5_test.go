package engine_test

import (
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/engine"
	"github.com/immortal-engine/immortal/internal/formal"
)

// TestWiring_Topology_TracksDependencyChanges proves the engine wires the
// topology tracker end-to-end: as services and dependencies change, the
// tracker records snapshots that reflect the current homology of the
// service graph (Components / Cycles / BlastRadius).
func TestWiring_Topology_TracksDependencyChanges(t *testing.T) {
	eng, err := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Millisecond,
		DedupWindow:    time.Millisecond,
		EnableTopology: true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer eng.Stop()
	if eng.Topology() == nil {
		t.Fatal("Topology tracker should be non-nil when EnableTopology=true")
	}

	// Build a small dependency graph through the engine's existing dependency API.
	eng.AddDependency("api", "db")
	eng.AddDependency("db", "cache")
	snap1, err := eng.SnapshotTopology()
	if err != nil {
		t.Fatalf("SnapshotTopology: %v", err)
	}
	if snap1.NodeCount < 3 {
		t.Errorf("snap1 expected ≥3 nodes, got %d", snap1.NodeCount)
	}
	if snap1.Cycles != 0 {
		t.Errorf("snap1 (chain) should have 0 cycles, got %d", snap1.Cycles)
	}

	// Closing a 3-cycle: cache now depends on api → api→db→cache→api.
	eng.AddDependency("cache", "api")
	snap2, _ := eng.SnapshotTopology()
	if snap2.Cycles == 0 {
		t.Errorf("snap2 should detect a cycle (api→db→cache→api), Cycles=%d", snap2.Cycles)
	}

	// History must show both snapshots.
	if got := len(eng.Topology().History()); got < 2 {
		t.Errorf("expected ≥2 snapshots in history, got %d", got)
	}
}

// TestWiring_Topology_DisabledByDefault confirms the new feature is opt-in.
func TestWiring_Topology_DisabledByDefault(t *testing.T) {
	eng, err := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Millisecond,
		DedupWindow:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Stop()
	if eng.Topology() != nil {
		t.Error("Topology should be nil when flag not set")
	}
	if _, err := eng.SnapshotTopology(); err == nil {
		t.Error("SnapshotTopology should error when topology disabled")
	}
}

// TestWiring_Formal_EnabledFlagSurfaced confirms the engine reports formal
// model-checking as available when the flag is set. The formal package is
// stateless — REST/CLI surfaces use the flag to decide whether to expose
// the endpoint.
func TestWiring_Formal_EnabledFlagSurfaced(t *testing.T) {
	eng, err := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Millisecond,
		DedupWindow:    time.Millisecond,
		EnableFormal:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Stop()
	if !eng.FormalEnabled() {
		t.Error("FormalEnabled() should be true after EnableFormal=true")
	}

	// Sanity-check that the formal package is reachable and works on a tiny world.
	world := formal.World{
		"api": {Name: "api", Healthy: true, Replicas: 3},
	}
	plan := formal.Plan{
		ID: "scale-to-zero",
		Steps: []formal.Action{
			{Name: "scale-to-zero", Fn: func(w formal.World) formal.World {
				s := w["api"]
				s.Replicas = 0
				w["api"] = s
				return w
			}},
		},
	}
	r := formal.Check(world, plan, []formal.Invariant{formal.MinReplicas("api", 1)})
	if r.Safe {
		t.Error("expected formal.Check to flag scale-to-zero as a violation")
	}
	if r.Violation == nil {
		t.Error("expected violation pointer, got nil")
	}
}

// TestWiring_Formal_DisabledByDefault confirms the new feature is opt-in.
func TestWiring_Formal_DisabledByDefault(t *testing.T) {
	eng, err := engine.New(engine.Config{
		DataDir:        t.TempDir(),
		ThrottleWindow: time.Millisecond,
		DedupWindow:    time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Stop()
	if eng.FormalEnabled() {
		t.Error("FormalEnabled() should be false when flag not set")
	}
}
