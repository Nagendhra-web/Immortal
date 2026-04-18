package federated

import (
	"math/rand/v2"
	"testing"
)

// TestRealWorld_FleetLearnsCPUBaseline simulates 5 honest clients observing
// CPU ~ Normal(50,5) and 1 malicious client observing CPU ~ Normal(200,20).
// After federated averaging with RobustTrimRatio=0.15, the global mean should
// stay within 1.5 of 50. All clients then apply the global model and verify
// anomaly detection works correctly.
func TestRealWorld_FleetLearnsCPUBaseline(t *testing.T) {
	const (
		honestClients  = 5
		samples        = 500
		honestMean     = 50.0
		honestStd      = 5.0
		maliciousMean  = 200.0
		maliciousStd   = 20.0
		trimRatio      = 0.15
	)

	agg := NewAggregator(AggregatorConfig{
		MinClients:      6,
		RobustTrimRatio: trimRatio,
	})

	clients := make([]*Client, 0, honestClients+1)

	// Honest clients: fixed seeds per client for determinism.
	for i := 0; i < honestClients; i++ {
		rng := rand.New(rand.NewPCG(uint64(i+1)*1000, 0))
		c := NewClientWithSeed("honest-"+string(rune('A'+i)), uint64(i+1)*1000, 0)
		for j := 0; j < samples; j++ {
			v := rng.NormFloat64()*honestStd + honestMean
			c.Observe("cpu", v)
		}
		clients = append(clients, c)
		if err := agg.Submit(c.Snapshot(1, 0)); err != nil {
			t.Fatalf("submit honest client %d: %v", i, err)
		}
	}

	// Malicious client: observes a very different distribution.
	rngMal := rand.New(rand.NewPCG(9999, 0))
	malClient := NewClientWithSeed("malicious", 9999, 0)
	for j := 0; j < samples; j++ {
		v := rngMal.NormFloat64()*maliciousStd + maliciousMean
		malClient.Observe("cpu", v)
	}
	clients = append(clients, malClient)
	if err := agg.Submit(malClient.Snapshot(1, 0)); err != nil {
		t.Fatalf("submit malicious client: %v", err)
	}

	gm, err := agg.Close(1)
	if err != nil {
		t.Fatalf("close round 1: %v", err)
	}

	cpuStats, ok := gm.Metrics["cpu"]
	if !ok {
		t.Fatal("cpu metric missing from GlobalModel")
	}

	// Global mean should be within 1.5 of honest mean (50).
	if cpuStats.Mean < honestMean-1.5 || cpuStats.Mean > honestMean+1.5 {
		t.Errorf("GlobalModel mean=%v is not within 1.5 of %v (malicious outlier not rejected)", cpuStats.Mean, honestMean)
	}

	// All clients apply the global model and test anomaly detection.
	for _, c := range clients {
		c.ApplyGlobal(gm)

		// 90 is far from mean=50 (>3 sigma at std~5): should be anomaly.
		if !c.IsAnomaly("cpu", 90) {
			t.Errorf("client %s: expected 90 to be anomaly (mean~50, std~5)", c.id)
		}

		// 52 is within 1 sigma: should NOT be anomaly.
		if c.IsAnomaly("cpu", 52) {
			t.Errorf("client %s: expected 52 to NOT be anomaly (mean~50, std~5)", c.id)
		}
	}

	// Verify contributors list is populated.
	if len(gm.Contributors) == 0 {
		t.Error("Contributors list is empty")
	}

	// Verify history is recorded.
	history := agg.History()
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
}
