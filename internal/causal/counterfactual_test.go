package causal_test

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/causal"
)

// buildLinearDS builds a dataset where Y = slope*X + intercept + N(0, noise).
// Returns the dataset and the true CausalGraph (X → Y).
func buildLinearDS(n int, slope, intercept, noise float64, rng *rand.Rand) (*causal.Dataset, causal.CausalGraph) {
	ds := causal.NewDataset([]string{"X", "Y"})
	for i := 0; i < n; i++ {
		x := rng.NormFloat64() * 3
		y := slope*x + intercept + rng.NormFloat64()*noise
		_ = ds.Add(map[string]float64{"X": x, "Y": y})
	}
	g := causal.CausalGraph{
		Nodes:      []string{"X", "Y"},
		Directed:   map[string][]string{"X": {"Y"}},
		Undirected: map[string][]string{},
	}
	return ds, g
}

// TestFitSCM_RecoversTrueCoefs verifies that FitSCM recovers β_X ≈ 2 and intercept ≈ 3
// from Y = 2*X + 3 + N(0,1).
func TestFitSCM_RecoversTrueCoefs(t *testing.T) {
	rng := rand.New(rand.NewPCG(100, 0))
	ds, g := buildLinearDS(500, 2.0, 3.0, 1.0, rng)

	m, err := causal.FitSCM(ds, g)
	if err != nil {
		t.Fatalf("FitSCM: %v", err)
	}

	intercept := m.Intercept["Y"]
	coefX := m.Coefs["Y"]["X"]

	if math.Abs(intercept-3.0) > 0.1 {
		t.Errorf("intercept: want ≈3.0, got %.4f", intercept)
	}
	if math.Abs(coefX-2.0) > 0.1 {
		t.Errorf("coef[X]: want ≈2.0, got %.4f", coefX)
	}
}

// TestCounterfactual_MatchesGroundTruth verifies that do(X=5) shifts Y by
// approximately (5 - E[X]) * slope on average over 500 rows.
func TestCounterfactual_MatchesGroundTruth(t *testing.T) {
	rng := rand.New(rand.NewPCG(200, 0))
	n := 500
	slope := 2.0
	intercept := 1.0
	ds, g := buildLinearDS(n, slope, intercept, 0.5, rng)

	m, err := causal.FitSCM(ds, g)
	if err != nil {
		t.Fatalf("FitSCM: %v", err)
	}

	doVal := 5.0
	cfMean, err := causal.AverageCounterfactual(ds, m, "X", doVal, "Y")
	if err != nil {
		t.Fatalf("AverageCounterfactual: %v", err)
	}

	// Expected average CF outcome: slope*doVal + intercept (noise averages out,
	// and abducted noise exactly cancels the residual contribution).
	expected := slope*doVal + intercept
	if math.Abs(cfMean-expected) > 0.5 {
		t.Errorf("AverageCounterfactual do(X=5): want ≈%.2f, got %.4f", expected, cfMean)
	}
}

// TestAverageCounterfactual_StableAcrossSeed checks that two different seeds
// produce the same AverageCounterfactual (it is deterministic given the dataset).
func TestAverageCounterfactual_StableAcrossSeed(t *testing.T) {
	rng1 := rand.New(rand.NewPCG(300, 0))
	ds, g := buildLinearDS(300, 1.5, 0.5, 1.0, rng1)

	m, err := causal.FitSCM(ds, g)
	if err != nil {
		t.Fatalf("FitSCM: %v", err)
	}

	cf1, err := causal.AverageCounterfactual(ds, m, "X", 3.0, "Y")
	if err != nil {
		t.Fatalf("AverageCounterfactual (1): %v", err)
	}
	cf2, err := causal.AverageCounterfactual(ds, m, "X", 3.0, "Y")
	if err != nil {
		t.Fatalf("AverageCounterfactual (2): %v", err)
	}

	if math.Abs(cf1-cf2) > 1e-12 {
		t.Errorf("AverageCounterfactual not deterministic: %.10f vs %.10f", cf1, cf2)
	}
}

// TestCounterfactual_PreservesNoise verifies that do(X = observed_X) at any row
// gives CounterfactualOutcome == ObservedOutcome (up to float64 epsilon).
// This holds because the abducted noise term exactly reconstructs the observation.
func TestCounterfactual_PreservesNoise(t *testing.T) {
	rng := rand.New(rand.NewPCG(400, 0))
	n := 100
	ds, g := buildLinearDS(n, 2.5, -1.0, 1.0, rng)

	m, err := causal.FitSCM(ds, g)
	if err != nil {
		t.Fatalf("FitSCM: %v", err)
	}

	for i := 0; i < n; i++ {
		obsX := ds.Data["X"][i]
		res, err := causal.Counterfactual(ds, m, i, "X", obsX, "Y")
		if err != nil {
			t.Fatalf("Counterfactual row %d: %v", i, err)
		}
		if math.Abs(res.CounterfactualOutcome-res.ObservedOutcome) > 1e-9 {
			t.Errorf("row %d: do(X=observed) gives CF=%.10f, obs=%.10f (diff=%.2e)",
				i, res.CounterfactualOutcome, res.ObservedOutcome,
				res.CounterfactualOutcome-res.ObservedOutcome)
		}
	}
}
