package topology

import (
	"sort"
	"testing"
)

func TestDiGraph_AddEdgeAndNodes(t *testing.T) {
	g := NewDiGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	nodes := g.Nodes()
	sort.Strings(nodes)
	want := []string{"A", "B", "C"}
	for i, n := range want {
		if nodes[i] != n {
			t.Errorf("node[%d]=%q want %q", i, nodes[i], n)
		}
	}
}

func TestDiGraph_RemoveEdge(t *testing.T) {
	g := NewDiGraph()
	g.AddEdge("A", "B")
	g.RemoveEdge("A", "B")
	if len(g.Edges()) != 0 {
		t.Errorf("expected 0 edges after removal, got %d", len(g.Edges()))
	}
}

func TestDiGraph_ConnectedComponents_TwoIslands(t *testing.T) {
	g := NewDiGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("X", "Y")
	cc := g.ConnectedComponents()
	if len(cc) != 2 {
		t.Errorf("expected 2 components, got %d: %v", len(cc), cc)
	}
}

func TestDiGraph_SCC_Cycle(t *testing.T) {
	g := NewDiGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("C", "A")
	sccs := g.StronglyConnectedComponents()
	bigSCC := 0
	for _, s := range sccs {
		if len(s) >= 3 {
			bigSCC++
		}
	}
	if bigSCC != 1 {
		t.Errorf("expected one SCC of size 3, got SCCs=%v", sccs)
	}
}

func TestDiGraph_SCC_Chain(t *testing.T) {
	g := NewDiGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	sccs := g.StronglyConnectedComponents()
	if len(sccs) != 3 {
		t.Errorf("chain should have 3 trivial SCCs, got %d", len(sccs))
	}
}

func TestDiGraph_BlastRadius(t *testing.T) {
	g := NewDiGraph()
	// D depends on C, C on B, B on A — failing A blasts {B, C, D}.
	g.AddEdge("D", "C")
	g.AddEdge("C", "B")
	g.AddEdge("B", "A")
	br := g.BlastRadius("A")
	sort.Strings(br)
	want := []string{"B", "C", "D"}
	if len(br) != 3 {
		t.Fatalf("blast radius size: got %d want 3 (%v)", len(br), br)
	}
	for i, n := range want {
		if br[i] != n {
			t.Errorf("blast[%d]=%q want %q", i, br[i], n)
		}
	}
}

func TestDiGraph_BlastRadius_Leaf(t *testing.T) {
	g := NewDiGraph()
	// A depends on B → if A fails, no service is affected (no one depends on A).
	g.AddEdge("A", "B")
	if br := g.BlastRadius("A"); len(br) != 0 {
		t.Errorf("expected 0 for top-of-chain leaf, got %v", br)
	}
}

func TestDiGraph_Clone_Independent(t *testing.T) {
	g := NewDiGraph()
	g.AddEdge("A", "B")
	c := g.Clone()
	c.AddEdge("X", "Y")
	if len(g.Nodes()) != 2 {
		t.Errorf("original mutated: %v", g.Nodes())
	}
}
