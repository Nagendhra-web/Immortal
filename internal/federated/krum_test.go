package federated

import (
	"math"
	"testing"
)

// TestKrum_PicksRepresentativeClient verifies that with 6 honest clients
// (mean≈10) and 1 Byzantine client (mean=1000), Krum selects one of the
// honest cluster.
func TestKrum_PicksRepresentativeClient(t *testing.T) {
	const n = 7
	const f = 1

	agg := NewKrumAggregator(n, f)

	honestMeans := []float64{9.8, 10.0, 10.1, 9.9, 10.2, 10.05}
	for i, m := range honestMeans {
		c := NewClientWithSeed("h"+string(rune('0'+i)), uint64(i+1), 0)
		for j := 0; j < 100; j++ {
			c.Observe("cpu", m+float64(j%3)*0.01)
		}
		u := c.Snapshot(1, 0)
		if err := agg.Submit(u); err != nil {
			t.Fatalf("submit honest %d: %v", i, err)
		}
	}

	// Byzantine client
	bad := NewClientWithSeed("byzantine", 999, 0)
	for j := 0; j < 100; j++ {
		bad.Observe("cpu", 1000.0)
	}
	if err := agg.Submit(bad.Snapshot(1, 0)); err != nil {
		t.Fatalf("submit byzantine: %v", err)
	}

	gm, err := agg.Close(1)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	cpuMean := gm.Metrics["cpu"].Mean
	if cpuMean > 15 {
		t.Errorf("Krum picked Byzantine client: cpu mean=%v (expected ~10)", cpuMean)
	}
	t.Logf("Krum selected client %v with cpu mean=%v", gm.Contributors, cpuMean)
}

// TestTrimmedMean_OutlierDropped verifies that TrimmedMean(f=1) drops the
// malicious outlier at mean=1000 and keeps the result near the honest mean≈10.
func TestTrimmedMean_OutlierDropped(t *testing.T) {
	agg := NewTrimmedMeanAggregator(1)

	honestMeans := []float64{9.8, 10.0, 10.1, 9.9, 10.2, 10.05}
	for i, m := range honestMeans {
		c := NewClientWithSeed("h"+string(rune('0'+i)), uint64(i+1), 0)
		for j := 0; j < 100; j++ {
			c.Observe("cpu", m+float64(j%3)*0.01)
		}
		u := c.Snapshot(1, 0)
		if err := agg.Submit(u); err != nil {
			t.Fatalf("submit honest %d: %v", i, err)
		}
	}

	bad := NewClientWithSeed("byzantine", 999, 0)
	for j := 0; j < 100; j++ {
		bad.Observe("cpu", 1000.0)
	}
	if err := agg.Submit(bad.Snapshot(1, 0)); err != nil {
		t.Fatalf("submit byzantine: %v", err)
	}

	gm, err := agg.Close(1)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	cpuMean := gm.Metrics["cpu"].Mean
	if cpuMean > 15 {
		t.Errorf("TrimmedMean did not drop outlier: cpu mean=%v (expected ~10)", cpuMean)
	}
	t.Logf("TrimmedMean cpu mean=%v", cpuMean)
}

// TestKrumAggregator_DuplicateSubmit verifies that submitting the same client
// twice returns an error.
func TestKrumAggregator_DuplicateSubmit(t *testing.T) {
	agg := NewKrumAggregator(3, 0)
	c := NewClient("c1")
	c.Observe("x", 1.0)
	u := c.Snapshot(1, 0)
	if err := agg.Submit(u); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if err := agg.Submit(u); err == nil {
		t.Error("expected error on duplicate submit, got nil")
	}
}

// TestKrumScore_Basic verifies the internal score computation selects the
// centroid closest to the cluster.
func TestKrumScore_Basic(t *testing.T) {
	// 4 points: three near 0, one at 100. neighbors=2.
	vecs := [][]float64{
		{0.0}, {0.1}, {0.2}, {100.0},
	}
	scores := make([]float64, len(vecs))
	neighbors := 2
	for i := range vecs {
		dists := make([]float64, 0, len(vecs)-1)
		for j := range vecs {
			if i == j {
				continue
			}
			dists = append(dists, squaredL2(vecs[i], vecs[j]))
		}
		scores[i] = krumScore(dists, neighbors)
	}

	// The outlier (index 3) should have the highest score.
	maxScore := scores[0]
	maxIdx := 0
	minScore := scores[0]
	minIdx := 0
	for i, s := range scores {
		if s > maxScore {
			maxScore = s
			maxIdx = i
		}
		if s < minScore {
			minScore = s
			minIdx = i
		}
	}
	if maxIdx != 3 {
		t.Errorf("expected outlier (index 3) to have max Krum score, got index %d", maxIdx)
	}
	if minIdx == 3 {
		t.Errorf("outlier should not have min Krum score")
	}
	_ = math.MaxFloat64
}
