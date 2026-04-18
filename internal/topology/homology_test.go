package topology

import (
	"testing"
)

func TestSnapshotOf_Triangle(t *testing.T) {
	g := NewDiGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("C", "A")
	s := SnapshotOf(g)
	if s.Components != 1 {
		t.Errorf("triangle should have 1 component, got %d", s.Components)
	}
	if s.Cycles < 1 {
		t.Errorf("triangle should have ≥1 cycle, got %d", s.Cycles)
	}
}

func TestSnapshotOf_Chain(t *testing.T) {
	g := NewDiGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("C", "D")
	s := SnapshotOf(g)
	if s.Components != 1 {
		t.Errorf("chain should have 1 component, got %d", s.Components)
	}
	if s.Cycles != 0 {
		t.Errorf("chain should have 0 cycles, got %d", s.Cycles)
	}
}

func TestTracker_RecordsSnapshots(t *testing.T) {
	tr := NewTracker(10)
	g := NewDiGraph()
	g.AddEdge("A", "B")
	tr.Record(g)
	g.AddEdge("B", "A")
	tr.Record(g)
	if len(tr.History()) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(tr.History()))
	}
}

func TestTracker_DetectsCycleBirth(t *testing.T) {
	tr := NewTracker(10)
	g := NewDiGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	tr.Record(g)
	g.AddEdge("C", "A") // closes a cycle
	tr.Record(g)
	events := tr.Events()
	sawCycle := false
	for _, e := range events {
		if e.Kind == CycleBirth {
			sawCycle = true
		}
	}
	if !sawCycle {
		t.Errorf("expected CycleBirth event, got %v", events)
	}
}

func TestTracker_DetectsFragmentation(t *testing.T) {
	tr := NewTracker(10)
	g := NewDiGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	tr.Record(g)
	g.RemoveEdge("A", "B") // splits into two components
	tr.Record(g)
	if tr.Latest().Components < 2 {
		t.Errorf("expected ≥2 components after split, got %d", tr.Latest().Components)
	}
}
