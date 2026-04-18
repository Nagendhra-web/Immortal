package federated

import (
	"math"
	"math/rand/v2"
	"sort"
	"testing"
)

// TestTDigest_PercentilesAccurate verifies p50, p95, p99 of 10k Normal(0,1)
// samples are within 0.1 of the true theoretical values.
func TestTDigest_PercentilesAccurate(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 0))
	td := NewTDigest(100)

	const n = 10_000
	for i := 0; i < n; i++ {
		td.Add(rng.NormFloat64())
	}

	if td.Count() != n {
		t.Errorf("Count=%d want %d", td.Count(), n)
	}

	cases := []struct {
		q, want, tol float64
		label        string
	}{
		{0.50, 0.0, 0.1, "p50"},
		{0.95, 1.645, 0.1, "p95"},
		{0.99, 2.326, 0.1, "p99"},
	}
	for _, tc := range cases {
		got := td.Quantile(tc.q)
		if math.Abs(got-tc.want) > tc.tol {
			t.Errorf("%s: Quantile(%v)=%v want %v ±%v", tc.label, tc.q, got, tc.want, tc.tol)
		}
	}
}

// TestTDigest_MergePreservesDistribution verifies that merging two 5k-sample
// digests gives a p95 within 0.15 of the combined sample's empirical p95.
func TestTDigest_MergePreservesDistribution(t *testing.T) {
	rng := rand.New(rand.NewPCG(99, 0))

	const half = 5_000
	all := make([]float64, 0, 2*half)

	td1 := NewTDigest(100)
	for i := 0; i < half; i++ {
		v := rng.NormFloat64()
		td1.Add(v)
		all = append(all, v)
	}

	td2 := NewTDigest(100)
	for i := 0; i < half; i++ {
		v := rng.NormFloat64()
		td2.Add(v)
		all = append(all, v)
	}

	td1.Merge(td2)

	// Empirical p95 from sorted slice.
	sort.Float64s(all)
	idx := int(0.95 * float64(len(all)))
	if idx >= len(all) {
		idx = len(all) - 1
	}
	empirical95 := all[idx]

	got := td1.Quantile(0.95)
	if math.Abs(got-empirical95) > 0.15 {
		t.Errorf("merged p95=%v empirical=%v diff=%v (want ≤0.15)", got, empirical95, math.Abs(got-empirical95))
	}
}
