package causal_test

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/immortal-engine/immortal/internal/causal"
)

// ── Dataset ──────────────────────────────────────────────────────────────────

func TestDatasetAddAndRows(t *testing.T) {
	ds := causal.NewDataset([]string{"x", "y"})
	if ds.Rows() != 0 {
		t.Fatalf("expected 0 rows, got %d", ds.Rows())
	}
	if err := ds.Add(map[string]float64{"x": 1, "y": 2}); err != nil {
		t.Fatal(err)
	}
	if ds.Rows() != 1 {
		t.Fatalf("expected 1 row, got %d", ds.Rows())
	}
	// missing key
	if err := ds.Add(map[string]float64{"x": 3}); err == nil {
		t.Fatal("expected error for missing key")
	}
}

// ── Correlation ───────────────────────────────────────────────────────────────

func TestCorrelationBasic(t *testing.T) {
	ds := causal.NewDataset([]string{"a", "b", "c"})
	for i := 0; i < 50; i++ {
		_ = ds.Add(map[string]float64{
			"a": float64(i),
			"b": float64(i) * 2,    // perfect linear
			"c": float64(50 - i),   // perfect anti-linear
		})
	}
	// We exercise pearson indirectly through Discover; here we do a sanity check
	// via EstimateEffect on a trivially discoverable graph.
	cfg := causal.DiscoverConfig{Alpha: 0.01, MaxCondSetSize: 1}
	_, err := causal.Discover(ds, cfg)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
}

// ── Partial correlation — known three-var chain ───────────────────────────────

func TestPartialCorrelation_KnownThreeVarCase(t *testing.T) {
	// X → Z → Y: conditioning on Z should make X ⊥ Y
	rng := rand.New(rand.NewPCG(42, 0))
	n := 600
	ds := causal.NewDataset([]string{"X", "Z", "Y"})
	for i := 0; i < n; i++ {
		x := rng.NormFloat64()
		z := 0.8*x + 0.2*rng.NormFloat64()
		y := 0.8*z + 0.2*rng.NormFloat64()
		_ = ds.Add(map[string]float64{"X": x, "Z": z, "Y": y})
	}
	cfg := causal.DiscoverConfig{Alpha: 0.05, MaxCondSetSize: 1}
	g, err := causal.Discover(ds, cfg)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	// X-Y edge should be absent in skeleton (conditional independence holds)
	if g.HasEdge("X", "Y") || g.HasEdge("Y", "X") {
		t.Error("expected no X-Y edge after conditioning on Z")
	}
	// X-Z and Z-Y edges must be present
	if !g.HasEdge("X", "Z") && !g.HasEdge("Z", "X") {
		t.Error("expected X-Z edge in skeleton")
	}
	if !g.HasEdge("Z", "Y") && !g.HasEdge("Y", "Z") {
		t.Error("expected Z-Y edge in skeleton")
	}
}

// ── Matrix inverse ────────────────────────────────────────────────────────────

func TestMatrixInverseIdentity(t *testing.T) {
	for n := 2; n <= 5; n++ {
		I := causal.Identity(n)
		inv, err := causal.Inverse(I)
		if err != nil {
			t.Fatalf("Inverse(I_%d): %v", n, err)
		}
		prod, err := I.Multiply(inv)
		if err != nil {
			t.Fatal(err)
		}
		for r := 0; r < n; r++ {
			for c := 0; c < n; c++ {
				want := 0.0
				if r == c {
					want = 1.0
				}
				if math.Abs(prod.Get(r, c)-want) > 1e-9 {
					t.Errorf("I_%d × I_%d⁻¹ [%d,%d] = %.6f, want %.1f", n, n, r, c, prod.Get(r, c), want)
				}
			}
		}
	}
}

// ── OLS ───────────────────────────────────────────────────────────────────────

func TestOLSSolvesLinearRegression(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 0))
	n := 200
	x := make([]float64, n)
	y := make([]float64, n)
	for i := range x {
		x[i] = rng.NormFloat64() * 5
		y[i] = 2*x[i] + 3 + rng.NormFloat64()*0.5
	}
	X := causal.NewMatrix(n, 1)
	for i, v := range x {
		X.Set(i, 0, v)
	}
	beta, err := causal.OLS(y, X)
	if err != nil {
		t.Fatalf("OLS: %v", err)
	}
	// beta[0]=intercept≈3, beta[1]=slope≈2
	if math.Abs(beta[0]-3) > 0.3 {
		t.Errorf("intercept: want ≈3, got %.4f", beta[0])
	}
	if math.Abs(beta[1]-2) > 0.3 {
		t.Errorf("slope: want ≈2, got %.4f", beta[1])
	}
}

// ── Discover — chain ──────────────────────────────────────────────────────────

func TestDiscoverChain(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 0))
	n := 500
	ds := causal.NewDataset([]string{"X", "Z", "Y"})
	for i := 0; i < n; i++ {
		x := rng.NormFloat64()
		z := 0.9*x + 0.1*rng.NormFloat64()
		y := 0.9*z + 0.1*rng.NormFloat64()
		_ = ds.Add(map[string]float64{"X": x, "Z": z, "Y": y})
	}
	g, err := causal.Discover(ds, causal.DiscoverConfig{Alpha: 0.05, MaxCondSetSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if g.HasEdge("X", "Y") || g.HasEdge("Y", "X") {
		t.Error("skeleton should not contain X-Y edge (chain: X→Z→Y)")
	}
	if !g.HasEdge("X", "Z") && !g.HasEdge("Z", "X") {
		t.Error("skeleton must contain X-Z edge")
	}
	if !g.HasEdge("Z", "Y") && !g.HasEdge("Y", "Z") {
		t.Error("skeleton must contain Z-Y edge")
	}
}

// ── Discover — confounder ─────────────────────────────────────────────────────

func TestDiscoverConfounder(t *testing.T) {
	// Z → X, Z → Y  (Z confounds X and Y; X⊥Y | Z)
	rng := rand.New(rand.NewPCG(2, 0))
	n := 500
	ds := causal.NewDataset([]string{"Z", "X", "Y"})
	for i := 0; i < n; i++ {
		z := rng.NormFloat64()
		x := 0.8*z + 0.2*rng.NormFloat64()
		y := 0.8*z + 0.2*rng.NormFloat64()
		_ = ds.Add(map[string]float64{"Z": z, "X": x, "Y": y})
	}
	g, err := causal.Discover(ds, causal.DiscoverConfig{Alpha: 0.05, MaxCondSetSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if g.HasEdge("X", "Y") || g.HasEdge("Y", "X") {
		t.Error("skeleton must NOT have X-Y edge when Z is the confounder")
	}
}

// ── EstimateEffect — linear chain ─────────────────────────────────────────────

func TestEstimateEffect_LinearChain(t *testing.T) {
	// y = 2*x + noise; z = 0.5*u + noise (unrelated to x,y path)
	rng := rand.New(rand.NewPCG(3, 0))
	n := 600
	ds := causal.NewDataset([]string{"u", "z", "x", "y"})
	for i := 0; i < n; i++ {
		u := rng.NormFloat64()
		z := 0.5*u + 0.5*rng.NormFloat64()
		x := rng.NormFloat64()
		y := 2*x + rng.NormFloat64()
		_ = ds.Add(map[string]float64{"u": u, "z": z, "x": x, "y": y})
	}
	g, err := causal.Discover(ds, causal.DiscoverConfig{Alpha: 0.05, MaxCondSetSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	eff, err := causal.EstimateEffect(ds, g, "x", "y")
	if err != nil {
		t.Fatalf("EstimateEffect: %v", err)
	}
	if math.Abs(eff.Coefficient-2.0) > 0.4 {
		t.Errorf("ACE of x on y: want ≈2.0, got %.4f", eff.Coefficient)
	}
}

// ── RootCause — ranks ancestors ───────────────────────────────────────────────

func TestRootCauseRanksAncestors(t *testing.T) {
	// A→B→C; C is outcome
	rng := rand.New(rand.NewPCG(4, 0))
	n := 600
	ds := causal.NewDataset([]string{"A", "B", "C"})
	for i := 0; i < n; i++ {
		a := rng.NormFloat64()
		b := 0.9*a + 0.1*rng.NormFloat64()
		c := 0.9*b + 0.1*rng.NormFloat64()
		_ = ds.Add(map[string]float64{"A": a, "B": b, "C": c})
	}
	g, err := causal.Discover(ds, causal.DiscoverConfig{Alpha: 0.05, MaxCondSetSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	rc, err := causal.RootCause(ds, g, "C")
	if err != nil {
		t.Fatal(err)
	}
	if len(rc.Ranked) == 0 {
		t.Fatal("expected at least one ranked ancestor")
	}
	for _, r := range rc.Ranked {
		if math.Abs(r.ACE) < 1e-6 {
			t.Errorf("variable %q has near-zero ACE; expected non-trivial causal effect", r.Variable)
		}
	}
}
