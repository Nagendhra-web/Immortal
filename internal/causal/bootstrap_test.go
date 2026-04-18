package causal_test

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/immortal-engine/immortal/internal/causal"
)

// makeLinearDataset creates a dataset where Y = coeff*X + noise*epsilon.
func makeLinearDataset(n int, coeff, noise float64, seed uint64) *causal.Dataset {
	rng := rand.New(rand.NewPCG(seed, 0))
	ds := causal.NewDataset([]string{"X", "Y"})
	for i := 0; i < n; i++ {
		x := rng.NormFloat64()
		y := coeff*x + noise*rng.NormFloat64()
		_ = ds.Add(map[string]float64{"X": x, "Y": y})
	}
	return ds
}

// makeSimpleGraph builds a CausalGraph with a single directed X → Y edge.
func makeSimpleGraph() causal.CausalGraph {
	return causal.CausalGraph{
		Nodes: []string{"X", "Y"},
		Directed: map[string][]string{
			"X": {"Y"},
		},
		Undirected: map[string][]string{},
	}
}

// TestBootstrapACE_ConvergesToPointEstimate_Linear checks that for Y=2X+noise,
// the 95% bootstrap CI contains 2.0.
func TestBootstrapACE_ConvergesToPointEstimate_Linear(t *testing.T) {
	ds := makeLinearDataset(400, 2.0, 0.5, 42)
	g := makeSimpleGraph()

	res, err := causal.BootstrapACEWithSeed(ds, g, "X", "Y", 200, 0.95, 1234)
	if err != nil {
		t.Fatalf("BootstrapACE: %v", err)
	}

	t.Logf("PointEstimate=%.4f Lower=%.4f Upper=%.4f StdErr=%.4f Resamples=%d",
		res.PointEstimate, res.Lower, res.Upper, res.StdErr, res.Resamples)

	// Point estimate should be close to 2.0
	if math.Abs(res.PointEstimate-2.0) > 0.3 {
		t.Errorf("PointEstimate: want ≈2.0, got %.4f", res.PointEstimate)
	}

	// CI must contain the true value 2.0
	if res.Lower > 2.0 || res.Upper < 2.0 {
		t.Errorf("95%% CI [%.4f, %.4f] does not contain true ACE=2.0", res.Lower, res.Upper)
	}

	// CI should be sensibly narrow for n=400
	width := res.Upper - res.Lower
	if width > 1.5 {
		t.Errorf("CI width %.4f is unexpectedly wide", width)
	}
}

// TestBootstrapACE_WiderCIForNoisyData checks that higher noise → wider CI.
func TestBootstrapACE_WiderCIForNoisyData(t *testing.T) {
	g := makeSimpleGraph()

	// Low noise
	dsLow := makeLinearDataset(300, 2.0, 0.2, 11)
	resLow, err := causal.BootstrapACEWithSeed(dsLow, g, "X", "Y", 200, 0.95, 999)
	if err != nil {
		t.Fatalf("BootstrapACE low noise: %v", err)
	}

	// High noise
	dsHigh := makeLinearDataset(300, 2.0, 3.0, 22)
	resHigh, err := causal.BootstrapACEWithSeed(dsHigh, g, "X", "Y", 200, 0.95, 999)
	if err != nil {
		t.Fatalf("BootstrapACE high noise: %v", err)
	}

	widthLow := resLow.Upper - resLow.Lower
	widthHigh := resHigh.Upper - resHigh.Lower

	t.Logf("Low noise CI width=%.4f  High noise CI width=%.4f", widthLow, widthHigh)

	if widthHigh <= widthLow {
		t.Errorf("expected CI to be wider for high-noise data: widthLow=%.4f widthHigh=%.4f",
			widthLow, widthHigh)
	}
}

// TestBootstrapACEWithSeed_Deterministic verifies that the same seed yields
// identical results across two calls.
func TestBootstrapACEWithSeed_Deterministic(t *testing.T) {
	ds := makeLinearDataset(200, 3.0, 1.0, 77)
	g := makeSimpleGraph()

	const seed = uint64(0xDEADBEEF)
	res1, err := causal.BootstrapACEWithSeed(ds, g, "X", "Y", 150, 0.95, seed)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	res2, err := causal.BootstrapACEWithSeed(ds, g, "X", "Y", 150, 0.95, seed)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if res1.PointEstimate != res2.PointEstimate {
		t.Errorf("PointEstimate differs: %v vs %v", res1.PointEstimate, res2.PointEstimate)
	}
	if res1.Lower != res2.Lower {
		t.Errorf("Lower differs: %v vs %v", res1.Lower, res2.Lower)
	}
	if res1.Upper != res2.Upper {
		t.Errorf("Upper differs: %v vs %v", res1.Upper, res2.Upper)
	}
	if res1.StdErr != res2.StdErr {
		t.Errorf("StdErr differs: %v vs %v", res1.StdErr, res2.StdErr)
	}
	if res1.Resamples != res2.Resamples {
		t.Errorf("Resamples differs: %v vs %v", res1.Resamples, res2.Resamples)
	}
}
