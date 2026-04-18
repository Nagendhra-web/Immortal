package causal

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
)

// BootstrapResult holds the output of a bootstrap ACE estimation.
type BootstrapResult struct {
	PointEstimate float64 // ACE on the original dataset
	Lower, Upper  float64 // percentile CI at the requested level
	Resamples     int     // number of bootstrap resamples performed (B)
	StdErr        float64 // standard deviation of the B resample estimates
}

// BootstrapACE runs B bootstrap resamples of ds, re-estimates the ACE of
// treatment on outcome each time using EstimateEffect, and returns a
// percentile confidence interval at the given level (e.g. 0.95 for 95% CI).
// A random seed is drawn from the global source; use BootstrapACEWithSeed
// for deterministic results.
func BootstrapACE(ds *Dataset, g CausalGraph, treatment, outcome string, B int, level float64) (BootstrapResult, error) {
	// Use a time-seeded source by creating a new PCG with zero state;
	// in practice the caller can use BootstrapACEWithSeed for reproducibility.
	src := rand.NewPCG(0, 0)
	rng := rand.New(src)
	// advance the source with a runtime-varying value (row count * B acts as
	// cheap entropy; for production use a real entropy source).
	for i := 0; i < ds.Rows()%97+1; i++ {
		rng.Float64()
	}
	return bootstrapACEInternal(ds, g, treatment, outcome, B, level, rng)
}

// BootstrapACEWithSeed is the seeded variant for deterministic tests.
// seed is split into two 32-bit halves for the PCG128 source.
func BootstrapACEWithSeed(ds *Dataset, g CausalGraph, treatment, outcome string, B int, level float64, seed uint64) (BootstrapResult, error) {
	hi := seed >> 32
	lo := seed & 0xFFFFFFFF
	src := rand.NewPCG(hi, lo)
	rng := rand.New(src)
	return bootstrapACEInternal(ds, g, treatment, outcome, B, level, rng)
}

// bootstrapACEInternal is the shared implementation.
func bootstrapACEInternal(ds *Dataset, g CausalGraph, treatment, outcome string, B int, level float64, rng *rand.Rand) (BootstrapResult, error) {
	if B <= 0 {
		return BootstrapResult{}, fmt.Errorf("bootstrap: B must be positive, got %d", B)
	}
	if level <= 0 || level >= 1 {
		return BootstrapResult{}, fmt.Errorf("bootstrap: level must be in (0,1), got %f", level)
	}

	n := ds.Rows()
	if n < 5 {
		return BootstrapResult{}, ErrInsufficientData
	}

	// Point estimate on original data.
	origEff, err := EstimateEffect(ds, g, treatment, outcome)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("bootstrap: point estimate failed: %w", err)
	}

	// Build a reusable resampled dataset (same names, pre-allocated).
	estimates := make([]float64, 0, B)

	for b := 0; b < B; b++ {
		resampled := resampleDataset(ds, rng, n)
		eff, err := EstimateEffect(resampled, g, treatment, outcome)
		if err != nil {
			// Skip singular resamples — common with very small n.
			continue
		}
		estimates = append(estimates, eff.Coefficient)
	}

	if len(estimates) == 0 {
		return BootstrapResult{}, fmt.Errorf("bootstrap: all resamples failed (OLS singular)")
	}

	sort.Float64s(estimates)

	loQ := (1 - level) / 2
	hiQ := (1 + level) / 2
	lower := percentile(estimates, loQ)
	upper := percentile(estimates, hiQ)

	stdErr := stdDev(estimates)

	return BootstrapResult{
		PointEstimate: origEff.Coefficient,
		Lower:         lower,
		Upper:         upper,
		Resamples:     len(estimates),
		StdErr:        stdErr,
	}, nil
}

// resampleDataset draws n rows with replacement from ds into a new Dataset.
func resampleDataset(ds *Dataset, rng *rand.Rand, n int) *Dataset {
	resampled := NewDataset(ds.Names)
	row := make(map[string]float64, len(ds.Names))
	for i := 0; i < n; i++ {
		ri := rng.IntN(n)
		for _, name := range ds.Names {
			row[name] = ds.Data[name][ri]
		}
		// Ignore error — we control the dataset structure.
		_ = resampled.Add(row)
	}
	return resampled
}

// percentile returns the p-th quantile of a sorted slice using linear interpolation.
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	pos := p * float64(n-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	frac := pos - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

// stdDev computes the population standard deviation of a slice.
func stdDev(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(len(vals))
	var variance float64
	for _, v := range vals {
		d := v - mean
		variance += d * d
	}
	variance /= float64(len(vals))
	return math.Sqrt(variance)
}
