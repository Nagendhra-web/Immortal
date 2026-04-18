package federated

import (
	"math"
	"math/rand/v2"
	"testing"
)

// TestAddNoise_Laplace_ScaleCorrect verifies that 10k Laplace samples with
// sensitivity=1, epsilon=1 have stddev ≈ sqrt(2) ≈ 1.414 within 10%.
func TestAddNoise_Laplace_ScaleCorrect(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	const n = 10_000
	samples := make([]float64, n)
	for i := range samples {
		samples[i] = AddNoise(rng, DPLaplace, 0, 1.0, 1.0, 0)
	}

	mean := 0.0
	for _, s := range samples {
		mean += s
	}
	mean /= n

	variance := 0.0
	for _, s := range samples {
		d := s - mean
		variance += d * d
	}
	variance /= float64(n - 1)
	stddev := math.Sqrt(variance)

	// Laplace(scale=1): variance = 2*scale^2 = 2 → stddev = sqrt(2) ≈ 1.414
	want := math.Sqrt2
	tol := want * 0.10 // 10%
	if math.Abs(stddev-want) > tol {
		t.Errorf("Laplace stddev=%v want %v ±10%%", stddev, want)
	}
}

// TestAddNoise_Gaussian_SigmaCorrect verifies that 10k Gaussian samples with
// sensitivity=1, epsilon=1, delta=1e-5 have stddev matching the formula within 10%.
func TestAddNoise_Gaussian_SigmaCorrect(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 0))
	const n = 10_000
	const sensitivity, epsilon, delta = 1.0, 1.0, 1e-5

	wantSigma := sensitivity * math.Sqrt(2*math.Log(1.25/delta)) / epsilon

	samples := make([]float64, n)
	for i := range samples {
		samples[i] = AddNoise(rng, DPGaussian, 0, sensitivity, epsilon, delta)
	}

	mean := 0.0
	for _, s := range samples {
		mean += s
	}
	mean /= n

	variance := 0.0
	for _, s := range samples {
		d := s - mean
		variance += d * d
	}
	variance /= float64(n - 1)
	stddev := math.Sqrt(variance)

	tol := wantSigma * 0.10
	if math.Abs(stddev-wantSigma) > tol {
		t.Errorf("Gaussian stddev=%v want %v ±10%%", stddev, wantSigma)
	}
}

// TestDPBudget_OverspendFails verifies that consuming more than the total
// epsilon returns an error, and that Remaining tracks correctly.
func TestDPBudget_OverspendFails(t *testing.T) {
	b := &DPBudget{Epsilon: 1.0}

	if err := b.Consume(0.4); err != nil {
		t.Fatalf("unexpected error on first consume: %v", err)
	}
	if got := b.Remaining(); math.Abs(got-0.6) > 1e-9 {
		t.Errorf("Remaining=%v want 0.6", got)
	}

	if err := b.Consume(0.4); err != nil {
		t.Fatalf("unexpected error on second consume: %v", err)
	}

	// Third consume would push to 1.2 > 1.0 → error expected.
	if err := b.Consume(0.3); err == nil {
		t.Error("expected error when budget exceeded, got nil")
	}

	// Remaining should still reflect the last successful consume.
	if got := b.Remaining(); math.Abs(got-0.2) > 1e-9 {
		t.Errorf("Remaining after failed consume=%v want 0.2", got)
	}
}

// TestClip_BoundsValue verifies Clip enforces symmetric bounds.
func TestClip_BoundsValue(t *testing.T) {
	cases := []struct {
		value, C, want float64
	}{
		{5.0, 3.0, 3.0},
		{-5.0, 3.0, -3.0},
		{2.0, 3.0, 2.0},
		{0.0, 1.0, 0.0},
		{3.0, 3.0, 3.0},  // exactly at boundary
		{-3.0, 3.0, -3.0},
	}
	for _, tc := range cases {
		got := Clip(tc.value, tc.C)
		if got != tc.want {
			t.Errorf("Clip(%v, %v)=%v want %v", tc.value, tc.C, got, tc.want)
		}
	}
}
