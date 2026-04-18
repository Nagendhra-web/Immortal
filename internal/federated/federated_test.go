package federated

import (
	"math"
	"math/rand/v2"
	"sync"
	"testing"
)

// TestWelfordOnlineMatchesBatch verifies that Welford's online algorithm
// produces mean/variance within 1e-9 of the direct batch computation.
func TestWelfordOnlineMatchesBatch(t *testing.T) {
	const n = 1000
	rng := rand.New(rand.NewPCG(42, 0))

	values := make([]float64, n)
	for i := range values {
		values[i] = rng.Float64()*200 - 100
	}

	// Welford online
	var w welford
	for _, v := range values {
		w.update(v)
	}

	// Batch mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	batchMean := sum / float64(n)

	// Batch variance (sample)
	sumSq := 0.0
	for _, v := range values {
		d := v - batchMean
		sumSq += d * d
	}
	batchVar := sumSq / float64(n-1)

	if math.Abs(w.mean-batchMean) > 1e-9 {
		t.Errorf("mean mismatch: welford=%v batch=%v diff=%v", w.mean, batchMean, math.Abs(w.mean-batchMean))
	}
	welfordVar := w.variance()
	if math.Abs(welfordVar-batchVar) > 1e-9 {
		t.Errorf("variance mismatch: welford=%v batch=%v diff=%v", welfordVar, batchVar, math.Abs(welfordVar-batchVar))
	}
}

// TestFedAvgMatchesPooledStats verifies that 3 clients observing disjoint samples
// of a known distribution produce a GlobalModel whose mean/variance match pooling
// all samples together within 1e-6.
func TestFedAvgMatchesPooledStats(t *testing.T) {
	const samplesPerClient = 400
	const nClients = 3

	rng := rand.New(rand.NewPCG(7, 0))
	var all []float64

	agg := NewAggregator(AggregatorConfig{MinClients: nClients})

	for i := 0; i < nClients; i++ {
		c := NewClient("c" + string(rune('0'+i)))
		for j := 0; j < samplesPerClient; j++ {
			v := rng.NormFloat64()*10 + 50 // Normal(50, 10)
			c.Observe("cpu", v)
			all = append(all, v)
		}
		if err := agg.Submit(c.Snapshot(1, 0)); err != nil {
			t.Fatalf("submit error: %v", err)
		}
	}

	gm, err := agg.Close(1)
	if err != nil {
		t.Fatalf("close error: %v", err)
	}

	ms := gm.Metrics["cpu"]

	// Pooled mean
	sum := 0.0
	for _, v := range all {
		sum += v
	}
	pooledMean := sum / float64(len(all))

	// Pooled sample variance
	sumSq := 0.0
	for _, v := range all {
		d := v - pooledMean
		sumSq += d * d
	}
	pooledVar := sumSq / float64(len(all)-1)
	fedVar := 0.0
	if ms.Count > 1 {
		fedVar = ms.M2 / float64(ms.Count-1)
	}

	if math.Abs(ms.Mean-pooledMean) > 1e-6 {
		t.Errorf("mean mismatch: fed=%v pooled=%v", ms.Mean, pooledMean)
	}
	if math.Abs(fedVar-pooledVar) > 1e-6 {
		t.Errorf("variance mismatch: fed=%v pooled=%v", fedVar, pooledVar)
	}
	if ms.Count != samplesPerClient*nClients {
		t.Errorf("count mismatch: got %d want %d", ms.Count, samplesPerClient*nClients)
	}
}

// TestMinClientsNotMet verifies that Close returns an error when fewer than
// MinClients have submitted.
func TestMinClientsNotMet(t *testing.T) {
	agg := NewAggregator(AggregatorConfig{MinClients: 3})
	c := NewClient("only-one")
	c.Observe("mem", 1.0)
	if err := agg.Submit(c.Snapshot(1, 0)); err != nil {
		t.Fatalf("submit: %v", err)
	}
	_, err := agg.Close(1)
	if err == nil {
		t.Fatal("expected error when MinClients not met, got nil")
	}
}

// TestRobustTrimRatioDropsOutliers verifies that a malicious client with mean=1000
// is trimmed out when RobustTrimRatio=0.2, keeping GlobalMean near the honest mean.
func TestRobustTrimRatioDropsOutliers(t *testing.T) {
	honest := []float64{10, 11, 9, 10.5, 10.2}
	malicious := 1000.0

	agg := NewAggregator(AggregatorConfig{
		MinClients:      6,
		RobustTrimRatio: 0.2,
	})

	for i, mean := range honest {
		c := NewClientWithSeed("h"+string(rune('0'+i)), uint64(i+1), 0)
		for j := 0; j < 50; j++ {
			c.Observe("metric", mean+float64(j%3)*0.01)
		}
		if err := agg.Submit(c.Snapshot(1, 0)); err != nil {
			t.Fatalf("submit honest: %v", err)
		}
	}

	// Malicious client
	m := NewClientWithSeed("malicious", 999, 0)
	for j := 0; j < 50; j++ {
		m.Observe("metric", malicious)
	}
	if err := agg.Submit(m.Snapshot(1, 0)); err != nil {
		t.Fatalf("submit malicious: %v", err)
	}

	gm, err := agg.Close(1)
	if err != nil {
		t.Fatalf("close: %v", err)
	}

	globalMean := gm.Metrics["metric"].Mean
	if globalMean > 15 {
		t.Errorf("outlier not trimmed: GlobalMean=%v (expected ~10)", globalMean)
	}
}

// TestMaxClientWeight verifies that a high-count client is capped and its
// influence is bounded.
func TestMaxClientWeight(t *testing.T) {
	agg := NewAggregator(AggregatorConfig{
		MinClients:      2,
		MaxClientWeight: 1000,
	})

	// Big client: 1,000,000 observations around mean=100
	big := NewClient("big")
	for i := 0; i < 1_000_000; i++ {
		big.Observe("x", 100.0)
	}
	if err := agg.Submit(big.Snapshot(1, 0)); err != nil {
		t.Fatalf("submit big: %v", err)
	}

	// Small honest client: 100 observations around mean=10
	small := NewClient("small")
	for i := 0; i < 100; i++ {
		small.Observe("x", 10.0)
	}
	if err := agg.Submit(small.Snapshot(1, 0)); err != nil {
		t.Fatalf("submit small: %v", err)
	}

	gm, err := agg.Close(1)
	if err != nil {
		t.Fatalf("close: %v", err)
	}

	// Without cap, big client (1M vs 100) dominates → mean ≈ 100.
	// With cap at 1000, big=1000 and small=100 → mean = (1000*100 + 100*10)/1100 ≈ 91.8.
	// We just verify the mean is NOT essentially 100 (big uncapped).
	ms := gm.Metrics["x"]
	if ms.Mean > 99 {
		t.Errorf("MaxClientWeight not enforced: mean=%v (expected < 99)", ms.Mean)
	}
	// Also verify the effective count used is capped.
	if ms.Count > 1100+1 {
		t.Errorf("Count not capped: got %d", ms.Count)
	}
}

// TestClientAnomalyDetection verifies IsAnomaly with a synthetic GlobalModel.
func TestClientAnomalyDetection(t *testing.T) {
	c := NewClient("detector")

	// Build a global model: cpu mean=50, std=5 → var=25, M2 = var*(n-1)
	const n = 100
	mean := 50.0
	variance := 25.0 // std=5
	m2 := variance * float64(n-1)

	gm := GlobalModel{
		Round: 1,
		Metrics: map[string]MetricStats{
			"cpu": {Metric: "cpu", Count: n, Mean: mean, M2: m2},
		},
		Contributors: []string{"s1"},
	}
	c.ApplyGlobal(gm)

	// |80 - 50| = 30 > 3*5=15 → anomaly
	if !c.IsAnomaly("cpu", 80) {
		t.Error("expected 80 to be anomaly for cpu (mean=50, std=5)")
	}
	// |52 - 50| = 2 < 15 → not anomaly
	if c.IsAnomaly("cpu", 52) {
		t.Error("expected 52 to NOT be anomaly for cpu (mean=50, std=5)")
	}
}

// TestDifferentialNoiseAppliedNonZero verifies that Snapshot with epsilon > 0
// produces a different Mean from the raw local mean (using fixed seed).
func TestDifferentialNoiseAppliedNonZero(t *testing.T) {
	c := NewClientWithSeed("dp-test", 12345, 67890)
	for i := 0; i < 200; i++ {
		c.Observe("lat", 20.0)
	}

	rawStats := c.LocalModel()
	rawMean := rawStats["lat"].Mean

	// Snapshot with DP noise; epsilon=0.1 → scale=10 → large noise expected
	snap := c.Snapshot(1, 0.1)
	noisedMean := snap.Stats["lat"].Mean

	if rawMean == noisedMean {
		t.Error("DP noise not applied: noised mean equals raw mean exactly")
	}
}

// TestConcurrentObserve verifies that 100 goroutines calling Observe concurrently
// produce an exact final Count.
func TestConcurrentObserve(t *testing.T) {
	c := NewClient("concurrent")
	const goroutines = 100
	const perGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				c.Observe("metric", float64(j))
			}
		}()
	}
	wg.Wait()

	local := c.LocalModel()
	got := local["metric"].Count
	want := goroutines * perGoroutine
	if got != want {
		t.Errorf("concurrent observe: count=%d want=%d", got, want)
	}
}
