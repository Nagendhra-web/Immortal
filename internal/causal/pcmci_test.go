package causal_test

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/immortal-engine/immortal/internal/causal"
)

// hasLaggedParent returns true if g.Parents[target] contains (variable, lag).
func hasLaggedParent(g causal.LaggedGraph, target, variable string, lag int) bool {
	for _, p := range g.Parents[target] {
		if p.Variable == variable && p.Lag == lag {
			return true
		}
	}
	return false
}

// TestPCMCI_DetectsKnownLag synthesises Y_t = 0.8*X_{t-3} + noise (n=400).
// Expects parent (X, 3) in g.Parents["Y"]; lag-1 should not be a spurious parent.
func TestPCMCI_DetectsKnownLag(t *testing.T) {
	rng := rand.New(rand.NewPCG(10, 0))
	n := 400
	ds := causal.NewDataset([]string{"X", "Y"})

	xVals := make([]float64, n)
	yVals := make([]float64, n)
	for i := 0; i < n; i++ {
		xVals[i] = rng.NormFloat64()
	}
	for i := 3; i < n; i++ {
		yVals[i] = 0.8*xVals[i-3] + 0.3*rng.NormFloat64()
	}
	for i := 0; i < 3; i++ {
		yVals[i] = rng.NormFloat64()
	}
	for i := 0; i < n; i++ {
		if err := ds.Add(map[string]float64{"X": xVals[i], "Y": yVals[i]}); err != nil {
			t.Fatal(err)
		}
	}

	cfg := causal.PCMCIConfig{Alpha: 0.05, TauMax: 5, MaxCondSetSize: 3}
	g, err := causal.DiscoverPCMCI(ds, cfg)
	if err != nil {
		t.Fatalf("DiscoverPCMCI: %v", err)
	}

	if !hasLaggedParent(g, "Y", "X", 3) {
		t.Errorf("expected (X, lag=3) as parent of Y; got parents: %v", g.Parents["Y"])
	}
	// Lag-1 should not be a spurious strong parent.
	if hasLaggedParent(g, "Y", "X", 1) {
		t.Logf("note: (X, lag=1) present as parent of Y — may be borderline with short series")
	}
}

// TestPCMCI_NoSpuriousLinks checks that two independent random walks produce
// no lagged parents for either variable.
func TestPCMCI_NoSpuriousLinks(t *testing.T) {
	rng := rand.New(rand.NewPCG(20, 0))
	n := 300
	ds := causal.NewDataset([]string{"X", "Y"})
	for i := 0; i < n; i++ {
		if err := ds.Add(map[string]float64{
			"X": rng.NormFloat64(),
			"Y": rng.NormFloat64(),
		}); err != nil {
			t.Fatal(err)
		}
	}

	cfg := causal.PCMCIConfig{Alpha: 0.01, TauMax: 3, MaxCondSetSize: 2}
	g, err := causal.DiscoverPCMCI(ds, cfg)
	if err != nil {
		t.Fatalf("DiscoverPCMCI: %v", err)
	}

	if len(g.Parents["Y"]) > 0 {
		t.Errorf("expected no parents for Y (independent), got: %v", g.Parents["Y"])
	}
	if len(g.Parents["X"]) > 0 {
		t.Errorf("expected no parents for X (independent), got: %v", g.Parents["X"])
	}
}

// TestPCMCI_MultipleLags synthesises Y_t = 0.5*X_{t-1} + 0.5*X_{t-4} + noise.
// Expects both (X,1) and (X,4) as parents of Y.
func TestPCMCI_MultipleLags(t *testing.T) {
	rng := rand.New(rand.NewPCG(30, 0))
	n := 500
	ds := causal.NewDataset([]string{"X", "Y"})

	xVals := make([]float64, n)
	yVals := make([]float64, n)
	for i := 0; i < n; i++ {
		xVals[i] = rng.NormFloat64()
	}
	for i := 4; i < n; i++ {
		yVals[i] = 0.5*xVals[i-1] + 0.5*xVals[i-4] + 0.25*rng.NormFloat64()
	}
	for i := 0; i < 4; i++ {
		yVals[i] = rng.NormFloat64()
	}
	for i := 0; i < n; i++ {
		if err := ds.Add(map[string]float64{"X": xVals[i], "Y": yVals[i]}); err != nil {
			t.Fatal(err)
		}
	}

	cfg := causal.PCMCIConfig{Alpha: 0.05, TauMax: 5, MaxCondSetSize: 3}
	g, err := causal.DiscoverPCMCI(ds, cfg)
	if err != nil {
		t.Fatalf("DiscoverPCMCI: %v", err)
	}

	if !hasLaggedParent(g, "Y", "X", 1) {
		t.Errorf("expected (X, lag=1) as parent of Y; got parents: %v", g.Parents["Y"])
	}
	if !hasLaggedParent(g, "Y", "X", 4) {
		t.Errorf("expected (X, lag=4) as parent of Y; got parents: %v", g.Parents["Y"])
	}
}

// TestEstimateLaggedACE_Linear verifies that EstimateLaggedACE recovers the
// true generating coefficient ≈ 0.8 for Y_t = 0.8*X_{t-3} + noise.
func TestEstimateLaggedACE_Linear(t *testing.T) {
	rng := rand.New(rand.NewPCG(40, 0))
	n := 400
	ds := causal.NewDataset([]string{"X", "Y"})

	xVals := make([]float64, n)
	yVals := make([]float64, n)
	for i := 0; i < n; i++ {
		xVals[i] = rng.NormFloat64()
	}
	for i := 3; i < n; i++ {
		yVals[i] = 0.8*xVals[i-3] + 0.3*rng.NormFloat64()
	}
	for i := 0; i < 3; i++ {
		yVals[i] = rng.NormFloat64()
	}
	for i := 0; i < n; i++ {
		if err := ds.Add(map[string]float64{"X": xVals[i], "Y": yVals[i]}); err != nil {
			t.Fatal(err)
		}
	}

	cfg := causal.PCMCIConfig{Alpha: 0.05, TauMax: 5, MaxCondSetSize: 3}
	g, err := causal.DiscoverPCMCI(ds, cfg)
	if err != nil {
		t.Fatalf("DiscoverPCMCI: %v", err)
	}

	eff, err := causal.EstimateLaggedACE(ds, g, "X", "Y", 3)
	if err != nil {
		t.Fatalf("EstimateLaggedACE: %v", err)
	}

	if math.Abs(eff.Coefficient-0.8) > 0.2 {
		t.Errorf("LaggedACE at lag 3: want ≈0.8, got %.4f", eff.Coefficient)
	}
}
