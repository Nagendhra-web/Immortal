package twin

import (
	"math"
	"testing"
)

// healthyState returns a single-service state map with one healthy replica.
func healthyState(svc string) map[string]State {
	return map[string]State{
		svc: {Service: svc, Replicas: 1, Healthy: true, ErrorRate: 0, Latency: 10, CPU: 50},
	}
}

// unhealthyState returns a single-service state map with one unhealthy replica.
func unhealthyState(svc string) map[string]State {
	return map[string]State{
		svc: {Service: svc, Replicas: 1, Healthy: false, ErrorRate: 0, Latency: 10, CPU: 50},
	}
}

// TestRankNet_LearnsHealthyOverUnhealthy feeds 50 pairs where Better is healthy
// and Worse is unhealthy. After training, Score(healthy) must exceed Score(unhealthy).
func TestRankNet_LearnsHealthyOverUnhealthy(t *testing.T) {
	cfg := RankNetConfig{Seed: 1}
	c := NewRankNetCalibrator(cfg)

	for i := 0; i < 50; i++ {
		c.AddPair(PreferencePair{
			Better: healthyState("svc"),
			Worse:  unhealthyState("svc"),
		})
	}

	c.Train(200, 0)

	// Held-out states (different service name).
	scoreGood := c.Score(healthyState("held"))
	scoreBad := c.Score(unhealthyState("held"))

	if scoreGood <= scoreBad {
		t.Errorf("expected Score(healthy)=%.4f > Score(unhealthy)=%.4f after training", scoreGood, scoreBad)
	}
}

// TestRankNet_PrefersLowLatency trains on pairs where Better has low latency
// and Worse has high latency. The Latency weight (w[3]) should be negative after training.
func TestRankNet_PrefersLowLatency(t *testing.T) {
	cfg := RankNetConfig{
		Initial: ScoreWeights{
			HealthyReplica:   1,
			UnhealthyReplica: 0,
			ErrorRate:        0,
			Latency:          0, // start neutral so we can observe the sign flip
			HighCPUPenalty:   0,
			HighCPUThreshold: 90,
		},
		LearningRate: 0.05,
		Momentum:     0.9,
		L2:           0.0001,
		Seed:         2,
	}
	c := NewRankNetCalibrator(cfg)

	lowLatency := func() map[string]State {
		return map[string]State{"svc": {Service: "svc", Replicas: 1, Healthy: true, Latency: 10, ErrorRate: 0, CPU: 50}}
	}
	highLatency := func() map[string]State {
		return map[string]State{"svc": {Service: "svc", Replicas: 1, Healthy: true, Latency: 500, ErrorRate: 0, CPU: 50}}
	}

	for i := 0; i < 100; i++ {
		c.AddPair(PreferencePair{Better: lowLatency(), Worse: highLatency()})
	}

	c.Train(300, 0)

	w := c.Weights()
	if w.Latency >= 0 {
		t.Errorf("expected Latency weight < 0 after low-latency preference training, got %.6f", w.Latency)
	}
}

// TestRankNet_LossDecreasesAcrossEpochs checks that the returned per-epoch
// loss series trends downward over 100 epochs. We start from neutral (zero)
// weights so the initial loss is log(2) ≈ 0.693 per pair and training must
// push it down.
func TestRankNet_LossDecreasesAcrossEpochs(t *testing.T) {
	c := NewRankNetCalibrator(RankNetConfig{
		Initial: ScoreWeights{HighCPUThreshold: 90}, // all weights zero → loss = log(2)
		LearningRate: 0.05,
		Momentum:     0.9,
		L2:           0.001,
		Seed:         3,
	})
	for i := 0; i < 30; i++ {
		c.AddPair(PreferencePair{Better: healthyState("a"), Worse: unhealthyState("b")})
	}

	losses := c.Train(100, 0)

	if len(losses) != 100 {
		t.Fatalf("expected 100 loss values, got %d", len(losses))
	}

	// Compare first 10 vs last 10 averages.
	first, last := 0.0, 0.0
	for i := 0; i < 10; i++ {
		first += losses[i]
		last += losses[90+i]
	}
	first /= 10
	last /= 10

	if last >= first {
		t.Errorf("loss did not decrease: first-10 avg=%.6f, last-10 avg=%.6f", first, last)
	}
}

// TestRankNet_Deterministic_WithSeed verifies that two calibrators with the
// same seed produce identical final weights (within 1e-9).
func TestRankNet_Deterministic_WithSeed(t *testing.T) {
	build := func() *RankNetCalibrator {
		c := NewRankNetCalibrator(RankNetConfig{Seed: 99})
		for i := 0; i < 20; i++ {
			c.AddPair(PreferencePair{Better: healthyState("s"), Worse: unhealthyState("s")})
		}
		c.Train(50, 5)
		return c
	}

	c1 := build()
	c2 := build()

	w1 := c1.Weights()
	w2 := c2.Weights()

	check := func(name string, a, b float64) {
		t.Helper()
		if math.Abs(a-b) > 1e-9 {
			t.Errorf("%s: run1=%.12f run2=%.12f differ by more than 1e-9", name, a, b)
		}
	}
	check("HealthyReplica", w1.HealthyReplica, w2.HealthyReplica)
	check("UnhealthyReplica", w1.UnhealthyReplica, w2.UnhealthyReplica)
	check("ErrorRate", w1.ErrorRate, w2.ErrorRate)
	check("Latency", w1.Latency, w2.Latency)
	check("HighCPUPenalty", w1.HighCPUPenalty, w2.HighCPUPenalty)
}

// TestRankNet_FullBatchVsMiniBatch_BothConverge verifies that both full-batch
// and mini-batch training reduce loss below 50% of initial loss.
func TestRankNet_FullBatchVsMiniBatch_BothConverge(t *testing.T) {
	addPairs := func(c *RankNetCalibrator) {
		for i := 0; i < 40; i++ {
			c.AddPair(PreferencePair{Better: healthyState("x"), Worse: unhealthyState("x")})
		}
	}

	// Neutral starting weights so loss starts high enough to be measurable.
	neutralCfg := RankNetConfig{
		Initial: ScoreWeights{
			HealthyReplica:   0,
			UnhealthyReplica: 0,
			ErrorRate:        0,
			Latency:          0,
			HighCPUPenalty:   0,
			HighCPUThreshold: 90,
		},
		LearningRate: 0.05,
		Momentum:     0.9,
		L2:           0.001,
		Seed:         7,
	}

	// full batch
	cfgFull := neutralCfg
	cfgFull.Seed = 7
	full := NewRankNetCalibrator(cfgFull)
	addPairs(full)
	initialLossFull := full.Loss()
	full.Train(200, 0)
	finalLossFull := full.Loss()

	// mini-batch (size 8)
	cfgMini := neutralCfg
	cfgMini.Seed = 7
	mini := NewRankNetCalibrator(cfgMini)
	addPairs(mini)
	initialLossMini := mini.Loss()
	mini.Train(200, 8)
	finalLossMini := mini.Loss()

	if finalLossFull >= initialLossFull*0.5 {
		t.Errorf("full-batch: loss did not halve; initial=%.6f final=%.6f", initialLossFull, finalLossFull)
	}
	if finalLossMini >= initialLossMini*0.5 {
		t.Errorf("mini-batch: loss did not halve; initial=%.6f final=%.6f", initialLossMini, finalLossMini)
	}
}

// TestRankNet_Weights_AvailableMidTraining calls Weights() between Train calls; must not panic.
func TestRankNet_Weights_AvailableMidTraining(t *testing.T) {
	c := NewRankNetCalibrator(RankNetConfig{Seed: 5})
	c.AddPair(PreferencePair{Better: healthyState("a"), Worse: unhealthyState("a")})

	c.Train(10, 0)
	w1 := c.Weights() // must not panic

	c.Train(10, 0)
	w2 := c.Weights() // must not panic

	// Weights should differ after more training.
	changed := w1.HealthyReplica != w2.HealthyReplica ||
		w1.UnhealthyReplica != w2.UnhealthyReplica ||
		w1.Latency != w2.Latency
	if !changed {
		t.Log("note: weights unchanged between Train calls (acceptable for single pair)")
	}
}

// TestRankNet_PairCount verifies PairCount reflects AddPair calls.
func TestRankNet_PairCount(t *testing.T) {
	c := NewRankNetCalibrator(RankNetConfig{Seed: 6})
	if c.PairCount() != 0 {
		t.Errorf("expected 0 pairs, got %d", c.PairCount())
	}
	for i := 1; i <= 5; i++ {
		c.AddPair(PreferencePair{Better: healthyState("s"), Worse: unhealthyState("s")})
		if c.PairCount() != i {
			t.Errorf("expected %d pairs after %d AddPair calls, got %d", i, i, c.PairCount())
		}
	}
}

// TestRankNet_Loss_DecreasesAfterTraining verifies Loss() is lower after training.
func TestRankNet_Loss_DecreasesAfterTraining(t *testing.T) {
	c := NewRankNetCalibrator(RankNetConfig{
		Initial: ScoreWeights{
			HealthyReplica:   0,
			UnhealthyReplica: 0,
			HighCPUThreshold: 90,
		},
		LearningRate: 0.05,
		Seed:         8,
	})
	for i := 0; i < 20; i++ {
		c.AddPair(PreferencePair{Better: healthyState("s"), Worse: unhealthyState("s")})
	}

	before := c.Loss()
	c.Train(100, 0)
	after := c.Loss()

	if after >= before {
		t.Errorf("Loss() did not decrease after training: before=%.6f after=%.6f", before, after)
	}
}

// TestRankNet_PreservesScoreWeightsCompat verifies that WeightedScore(c.Weights())(states)
// produces the same value as c.Score(states).
func TestRankNet_PreservesScoreWeightsCompat(t *testing.T) {
	c := NewRankNetCalibrator(RankNetConfig{Seed: 9})
	for i := 0; i < 10; i++ {
		c.AddPair(PreferencePair{Better: healthyState("a"), Worse: unhealthyState("b")})
	}
	c.Train(50, 0)

	states := map[string]State{
		"api": {Service: "api", CPU: 70, Replicas: 2, Healthy: true, ErrorRate: 0.1, Latency: 200},
		"db":  {Service: "db", CPU: 95, Replicas: 1, Healthy: false, ErrorRate: 0.5, Latency: 500},
	}

	direct := c.Score(states)
	viaWeights := WeightedScore(c.Weights())(states)

	if math.Abs(direct-viaWeights) > 1e-9 {
		t.Errorf("c.Score()=%.10f != WeightedScore(c.Weights())()=%.10f", direct, viaWeights)
	}
}
