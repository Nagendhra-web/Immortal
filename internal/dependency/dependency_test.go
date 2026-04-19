package dependency_test

import (
	"sync"
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/dependency"
)

func TestNewEmpty(t *testing.T) {
	g := dependency.New()
	if g == nil {
		t.Fatal("expected non-nil graph")
	}
	if nodes := g.All(); len(nodes) != 0 {
		t.Errorf("expected empty graph, got %d nodes", len(nodes))
	}
}

func TestAddNodeAndDependency(t *testing.T) {
	g := dependency.New()
	g.AddDependency("A", "B")

	all := g.All()
	names := make(map[string]bool)
	for _, n := range all {
		names[n.Name] = true
	}
	if !names["A"] || !names["B"] {
		t.Error("expected both A and B to be registered")
	}
}

func TestAddNodeExplicit(t *testing.T) {
	g := dependency.New()
	g.AddNode("standalone")
	all := g.All()
	if len(all) != 1 || all[0].Name != "standalone" {
		t.Error("expected standalone node")
	}
}

func TestDependencies(t *testing.T) {
	g := dependency.New()
	g.AddDependency("A", "B")
	g.AddDependency("A", "C")

	deps := g.Dependencies("A")
	if len(deps) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(deps))
	}
	if deps[0] != "B" || deps[1] != "C" {
		t.Errorf("expected [B C], got %v", deps)
	}
}

func TestDependents(t *testing.T) {
	g := dependency.New()
	g.AddDependency("A", "C")
	g.AddDependency("B", "C")

	dependents := g.Dependents("C")
	if len(dependents) != 2 {
		t.Fatalf("expected 2 dependents, got %d", len(dependents))
	}
	if dependents[0] != "A" || dependents[1] != "B" {
		t.Errorf("expected [A B], got %v", dependents)
	}
}

func TestTransitiveDependencies(t *testing.T) {
	g := dependency.New()
	g.AddDependency("A", "B")
	g.AddDependency("B", "C")
	g.AddDependency("C", "D")

	deps := g.TransitiveDependencies("A")
	if len(deps) != 3 {
		t.Fatalf("expected 3 transitive deps, got %d: %v", len(deps), deps)
	}
	expected := map[string]bool{"B": true, "C": true, "D": true}
	for _, d := range deps {
		if !expected[d] {
			t.Errorf("unexpected dependency: %q", d)
		}
	}
}

func TestTransitiveDependents(t *testing.T) {
	g := dependency.New()
	g.AddDependency("A", "B")
	g.AddDependency("B", "C")

	dependents := g.TransitiveDependents("C")
	if len(dependents) != 2 {
		t.Fatalf("expected 2 transitive dependents, got %d: %v", len(dependents), dependents)
	}
	expected := map[string]bool{"A": true, "B": true}
	for _, d := range dependents {
		if !expected[d] {
			t.Errorf("unexpected dependent: %q", d)
		}
	}
}

func TestImpactOf(t *testing.T) {
	g := dependency.New()
	g.AddDependency("A", "B")
	g.AddDependency("B", "C")
	g.AddDependency("D", "C")

	// C is depended on by B and D directly, and A transitively via B
	impact := g.ImpactOf("C")
	if impact != 3 {
		t.Errorf("expected impact 3 for C, got %d", impact)
	}
	if g.ImpactOf("A") != 0 {
		t.Errorf("expected impact 0 for A (top-level), got %d", g.ImpactOf("A"))
	}
}

func TestCriticalPath(t *testing.T) {
	g := dependency.New()
	g.AddDependency("A", "C")
	g.AddDependency("B", "C")
	g.AddDependency("D", "C")

	cp := g.CriticalPath()
	if len(cp) == 0 {
		t.Fatal("expected non-empty critical path")
	}
	// C has the most dependents (3), should be first
	if cp[0] != "C" {
		t.Errorf("expected C first in critical path, got %q", cp[0])
	}
}

func TestAll(t *testing.T) {
	g := dependency.New()
	g.AddDependency("A", "B")
	g.AddDependency("A", "C")
	g.AddDependency("B", "C")

	all := g.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(all))
	}
	// sorted by name: A, B, C
	if all[0].Name != "A" || all[1].Name != "B" || all[2].Name != "C" {
		t.Errorf("expected sorted names [A B C], got %v", []string{all[0].Name, all[1].Name, all[2].Name})
	}

	// verify node A
	nodeA := all[0]
	if len(nodeA.Dependencies) != 2 {
		t.Errorf("expected A to have 2 dependencies, got %d", len(nodeA.Dependencies))
	}
	if len(nodeA.Dependents) != 0 {
		t.Errorf("expected A to have 0 dependents, got %d", len(nodeA.Dependents))
	}

	// verify node C
	nodeC := all[2]
	if len(nodeC.Dependencies) != 0 {
		t.Errorf("expected C to have 0 dependencies, got %d", len(nodeC.Dependencies))
	}
	if len(nodeC.Dependents) != 2 {
		t.Errorf("expected C to have 2 dependents, got %d", len(nodeC.Dependents))
	}
}

func TestHasCycle(t *testing.T) {
	g := dependency.New()
	g.AddDependency("A", "B")
	g.AddDependency("B", "C")
	g.AddDependency("C", "A")

	if !g.HasCycle() {
		t.Error("expected cycle to be detected")
	}
}

func TestNoCycle(t *testing.T) {
	g := dependency.New()
	g.AddDependency("A", "B")
	g.AddDependency("B", "C")
	g.AddDependency("A", "C")

	if g.HasCycle() {
		t.Error("expected no cycle in DAG")
	}
}

func TestRoots(t *testing.T) {
	g := dependency.New()
	g.AddDependency("A", "B")
	g.AddDependency("A", "C")
	g.AddDependency("D", "C")

	roots := g.Roots()
	// A and D have no dependents
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d: %v", len(roots), roots)
	}
	if roots[0] != "A" || roots[1] != "D" {
		t.Errorf("expected roots [A D], got %v", roots)
	}
}

func TestLeaves(t *testing.T) {
	g := dependency.New()
	g.AddDependency("A", "B")
	g.AddDependency("A", "C")

	leaves := g.Leaves()
	// B and C have no dependencies
	if len(leaves) != 2 {
		t.Fatalf("expected 2 leaves, got %d: %v", len(leaves), leaves)
	}
	if leaves[0] != "B" || leaves[1] != "C" {
		t.Errorf("expected leaves [B C], got %v", leaves)
	}
}

func TestRemoveDependency(t *testing.T) {
	g := dependency.New()
	g.AddDependency("A", "B")
	g.AddDependency("A", "C")

	g.RemoveDependency("A", "B")

	deps := g.Dependencies("A")
	if len(deps) != 1 || deps[0] != "C" {
		t.Errorf("expected [C] after removal, got %v", deps)
	}
}

func TestConcurrentAccess(t *testing.T) {
	g := dependency.New()
	var wg sync.WaitGroup
	n := 50
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			from := "service-a"
			to := "service-b"
			g.AddDependency(from, to)
			_ = g.Dependencies(from)
			_ = g.Dependents(to)
		}(i)
	}
	wg.Wait()

	// Should have exactly one edge A->B (dedup)
	deps := g.Dependencies("service-a")
	if len(deps) != 1 || deps[0] != "service-b" {
		t.Errorf("expected [service-b], got %v", deps)
	}
}
