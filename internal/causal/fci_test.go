package causal_test

import (
	"math/rand/v2"
	"testing"

	"github.com/immortal-engine/immortal/internal/causal"
)

// TestFCI_ChainGraph verifies that FCI recovers the correct skeleton for
// X → Y → Z and produces reasonable PAG edges.
func TestFCI_ChainGraph(t *testing.T) {
	rng := rand.New(rand.NewPCG(10, 0))
	n := 600
	ds := causal.NewDataset([]string{"X", "Y", "Z"})
	for i := 0; i < n; i++ {
		x := rng.NormFloat64()
		y := 0.8*x + 0.2*rng.NormFloat64()
		z := 0.8*y + 0.2*rng.NormFloat64()
		_ = ds.Add(map[string]float64{"X": x, "Y": y, "Z": z})
	}

	pag, err := causal.DiscoverFCI(ds, causal.FCIConfig{Alpha: 0.05, MaxCondSetSize: 2})
	if err != nil {
		t.Fatalf("DiscoverFCI: %v", err)
	}

	// X-Y and Y-Z edges must be in skeleton; X-Z must be absent.
	if !pag.HasEdge("X", "Y") {
		t.Error("expected X-Y edge in skeleton")
	}
	if !pag.HasEdge("Y", "Z") {
		t.Error("expected Y-Z edge in skeleton")
	}
	if pag.HasEdge("X", "Z") {
		t.Error("expected no X-Z edge (chain: X→Y→Z conditioned on Y removes X-Z)")
	}
}

// TestFCI_LatentConfounder_BidirectedEdge is the KEY test.
// U → X and U → Y, but U is NOT in the dataset (latent confounder).
// FCI on {X, Y} should detect that X and Y are NOT conditionally independent
// on any observed set, and produce a <-> (bidirected) edge indicating a
// latent common cause. PC would incorrectly orient or leave a directed edge.
func TestFCI_LatentConfounder_BidirectedEdge(t *testing.T) {
	rng := rand.New(rand.NewPCG(99, 7))
	n := 800
	// Only X and Y are observed; U is latent.
	ds := causal.NewDataset([]string{"X", "Y"})
	for i := 0; i < n; i++ {
		u := rng.NormFloat64()
		x := 0.8*u + 0.2*rng.NormFloat64()
		y := 0.8*u + 0.2*rng.NormFloat64()
		_ = ds.Add(map[string]float64{"X": x, "Y": y})
	}

	pag, err := causal.DiscoverFCI(ds, causal.FCIConfig{Alpha: 0.05, MaxCondSetSize: 1})
	if err != nil {
		t.Fatalf("DiscoverFCI: %v", err)
	}

	// With only X and Y observed and no way to condition out U,
	// FCI must retain the X-Y edge (they are NOT conditionally independent
	// given any subset of observed variables).
	if !pag.HasEdge("X", "Y") {
		t.Fatal("expected X-Y edge to survive (latent confounder U makes X,Y dependent)")
	}

	// Find the X-Y edge and check its marks.
	var foundEdge *causal.PAGEdge
	for i := range pag.Edges {
		e := &pag.Edges[i]
		if (e.From == "X" && e.To == "Y") || (e.From == "Y" && e.To == "X") {
			foundEdge = e
			break
		}
	}
	if foundEdge == nil {
		t.Fatal("could not locate X-Y edge in PAG")
	}

	// With no observed common parent to orient as a v-structure,
	// FCI leaves the edge as o-o or orients both ends as arrows (<->).
	// Both indicate possible latent common cause. A directed --> edge would
	// be wrong (it would claim direct causation without latent).
	// Accept o-o or <->; reject --> (tail on one side without both arrows).
	xSide, ySide := foundEdge.FromMark, foundEdge.ToMark
	if foundEdge.From == "Y" {
		xSide, ySide = foundEdge.ToMark, foundEdge.FromMark
	}

	// A definite tail on either side would wrongly claim directed causation.
	if xSide == causal.MarkTail || ySide == causal.MarkTail {
		t.Errorf("X-Y edge has a definite tail (FromMark=%d ToMark=%d), "+
			"which incorrectly claims directed causation in presence of latent U",
			foundEdge.FromMark, foundEdge.ToMark)
	}
	// Both circle or both arrow = correct FCI output
	t.Logf("X-Y edge marks: FromMark=%d ToMark=%d (0=circle,1=arrow,2=tail)", foundEdge.FromMark, foundEdge.ToMark)
}

// TestFCI_ThreeVar_Collider verifies that X → Z ← Y is identified as a
// collider: FCI orients Z with arrowheads from both X and Y.
func TestFCI_ThreeVar_Collider(t *testing.T) {
	rng := rand.New(rand.NewPCG(55, 3))
	n := 600
	ds := causal.NewDataset([]string{"X", "Y", "Z"})
	for i := 0; i < n; i++ {
		x := rng.NormFloat64()
		y := rng.NormFloat64() // X and Y are independent
		z := 0.7*x + 0.7*y + 0.1*rng.NormFloat64()
		_ = ds.Add(map[string]float64{"X": x, "Y": y, "Z": z})
	}

	pag, err := causal.DiscoverFCI(ds, causal.FCIConfig{Alpha: 0.05, MaxCondSetSize: 1})
	if err != nil {
		t.Fatalf("DiscoverFCI: %v", err)
	}

	// X-Z and Y-Z must be in skeleton; X-Y must not.
	if !pag.HasEdge("X", "Z") {
		t.Error("expected X-Z edge")
	}
	if !pag.HasEdge("Y", "Z") {
		t.Error("expected Y-Z edge")
	}
	if pag.HasEdge("X", "Y") {
		t.Error("expected no X-Y edge (X and Y are independent)")
	}

	// Z should be a collider: arrowheads pointing TO Z from both X and Y.
	// Find edge X-Z and check the Z-side mark.
	checkColliderMark := func(from, to string) {
		for _, e := range pag.Edges {
			if e.From == from && e.To == to {
				if e.ToMark != causal.MarkArrow {
					t.Errorf("edge %s->%s: expected arrowhead at %s (got mark %d)", from, to, to, e.ToMark)
				}
				return
			}
			if e.From == to && e.To == from {
				if e.FromMark != causal.MarkArrow {
					t.Errorf("edge %s->%s: expected arrowhead at %s (got mark %d)", to, from, from, e.FromMark)
				}
				return
			}
		}
		t.Errorf("edge %s-%s not found", from, to)
	}
	checkColliderMark("X", "Z")
	checkColliderMark("Y", "Z")
}

// TestPAG_DefiniteAncestorsOnlyDirected verifies that DefiniteAncestors
// only follows --> edges (MarkTail → MarkArrow).
func TestPAG_DefiniteAncestorsOnlyDirected(t *testing.T) {
	pag := causal.PAG{
		Nodes: []string{"A", "B", "C"},
		Edges: []causal.PAGEdge{
			// A --> B  (definite directed)
			{From: "A", To: "B", FromMark: causal.MarkTail, ToMark: causal.MarkArrow},
			// B o-> C  (possible directed, not definite)
			{From: "B", To: "C", FromMark: causal.MarkCircle, ToMark: causal.MarkArrow},
		},
	}

	// Definite ancestors of B: only A (via -->)
	dA := pag.DefiniteAncestors("B")
	if len(dA) != 1 || dA[0] != "A" {
		t.Errorf("DefiniteAncestors(B): want [A], got %v", dA)
	}

	// Definite ancestors of C: none (B o-> C is not a --> edge)
	dC := pag.DefiniteAncestors("C")
	if len(dC) != 0 {
		t.Errorf("DefiniteAncestors(C): want [], got %v", dC)
	}
}

// TestPAG_PossibleAncestorsIncludesUndetermined verifies that PossibleAncestors
// follows any edge that could represent ancestry (o-o, o->, <->).
func TestPAG_PossibleAncestorsIncludesUndetermined(t *testing.T) {
	pag := causal.PAG{
		Nodes: []string{"A", "B", "C", "D"},
		Edges: []causal.PAGEdge{
			// A o-o B  (undetermined)
			{From: "A", To: "B", FromMark: causal.MarkCircle, ToMark: causal.MarkCircle},
			// B --> C  (definite directed)
			{From: "B", To: "C", FromMark: causal.MarkTail, ToMark: causal.MarkArrow},
			// D --> C  (definite tail at D, so D cannot be ancestor via this edge going the other way)
			{From: "D", To: "C", FromMark: causal.MarkTail, ToMark: causal.MarkArrow},
		},
	}

	// Possible ancestors of C: B (via -->), D (via -->), A (via o-o→B→C)
	pA := pag.PossibleAncestors("C")
	has := func(s string) bool {
		for _, v := range pA {
			if v == s {
				return true
			}
		}
		return false
	}
	if !has("B") {
		t.Errorf("PossibleAncestors(C): expected B, got %v", pA)
	}
	if !has("D") {
		t.Errorf("PossibleAncestors(C): expected D, got %v", pA)
	}
	if !has("A") {
		t.Errorf("PossibleAncestors(C): expected A (reachable via B), got %v", pA)
	}
}
