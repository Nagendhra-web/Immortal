package twin

import (
	"math"
	"testing"
)

func TestDefaultWeights_MatchDefaultScore(t *testing.T) {
	states := map[string]State{
		"api": {Service: "api", CPU: 70, Replicas: 2, Healthy: true, ErrorRate: 0.1, Latency: 200},
		"db":  {Service: "db", CPU: 95, Replicas: 1, Healthy: false, ErrorRate: 0.5, Latency: 500},
	}

	want := DefaultScore(states)
	got := WeightedScore(DefaultWeights())(states)

	if math.Abs(want-got) > 1e-9 {
		t.Errorf("WeightedScore(DefaultWeights()) = %.6f, DefaultScore = %.6f; expected equal", got, want)
	}
}

func TestCalibrator_AdjustsWeightsTowardObserved(t *testing.T) {
	initial := DefaultWeights()
	cal := NewCalibrator(initial)

	if cal.SampleCount() != 0 {
		t.Errorf("expected SampleCount=0, got %d", cal.SampleCount())
	}

	// Feed 50 observations: latency decreased by 100ms each time, labeled as +2 improvement.
	// This signals that latency reduction is worth more than the current weight implies.
	for i := 0; i < 50; i++ {
		before := map[string]State{
			"api": {Service: "api", Latency: 300, ErrorRate: 0.1, Healthy: true},
		}
		after := map[string]State{
			"api": {Service: "api", Latency: 200, ErrorRate: 0.1, Healthy: true},
		}
		cal.Observe(before, after, 2.0)
	}

	if cal.SampleCount() != 50 {
		t.Errorf("expected SampleCount=50, got %d", cal.SampleCount())
	}

	w := cal.Weights()
	// Latency weight should have grown in magnitude (become more negative) relative to initial.
	if math.Abs(w.Latency) <= math.Abs(initial.Latency) {
		t.Errorf("expected |Latency| weight to grow after latency-reduction observations; initial=%.6f got=%.6f",
			initial.Latency, w.Latency)
	}
}

func TestCalibrator_WeightsRemainsNegativeForLatency(t *testing.T) {
	cal := NewCalibrator(DefaultWeights())

	// Feed observations where latency increased and outcome was bad.
	for i := 0; i < 20; i++ {
		before := map[string]State{"svc": {Latency: 100, ErrorRate: 0.0, Healthy: true}}
		after := map[string]State{"svc": {Latency: 300, ErrorRate: 0.0, Healthy: true}}
		cal.Observe(before, after, -1.0)
	}

	w := cal.Weights()
	if w.Latency > 0 {
		t.Errorf("Latency weight should remain <= 0, got %.6f", w.Latency)
	}
}

func TestWeightedScore_ZeroState(t *testing.T) {
	states := map[string]State{}
	score := WeightedScore(DefaultWeights())(states)
	if score != 0 {
		t.Errorf("expected score=0 for empty states, got %.4f", score)
	}
}

func TestWeightedScore_CustomWeights(t *testing.T) {
	// With custom weights where latency matters a lot.
	w := ScoreWeights{
		HealthyReplica:   5,
		UnhealthyReplica: -10,
		ErrorRate:        -100,
		Latency:          -1.0,
		HighCPUPenalty:   -50,
		HighCPUThreshold: 80,
	}
	sf := WeightedScore(w)

	highLatency := map[string]State{
		"api": {Service: "api", CPU: 50, Replicas: 1, Healthy: true, Latency: 500, ErrorRate: 0},
	}
	lowLatency := map[string]State{
		"api": {Service: "api", CPU: 50, Replicas: 1, Healthy: true, Latency: 10, ErrorRate: 0},
	}

	hs := sf(highLatency)
	ls := sf(lowLatency)
	if ls <= hs {
		t.Errorf("low latency score (%.2f) should beat high latency score (%.2f) with Latency weight -1.0", ls, hs)
	}
}
