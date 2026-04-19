package evolve

import (
	"strings"
	"testing"
)

func TestPrediction_Describe_Simulated(t *testing.T) {
	p := Prediction{
		MetricDeltas: map[string]float64{"latency_p99": -0.63},
		RiskDeltas:   map[string]float64{"cost_per_hour": 0.15},
		Simulated:    true,
	}
	got := p.Describe()
	if !strings.Contains(got, "latency_p99 -63%") {
		t.Errorf("missing latency delta: %q", got)
	}
	if !strings.Contains(got, "cost_per_hour +15%") {
		t.Errorf("missing cost delta: %q", got)
	}
	if !strings.Contains(got, "(twin simulated)") {
		t.Errorf("should be marked simulated: %q", got)
	}
}

func TestPrediction_Describe_Heuristic(t *testing.T) {
	p := Prediction{
		MetricDeltas: map[string]float64{"error_rate": -0.8},
	}
	got := p.Describe()
	if !strings.Contains(got, "(heuristic estimate)") {
		t.Errorf("should be marked heuristic when not simulated: %q", got)
	}
}

func TestPrediction_Describe_Empty(t *testing.T) {
	if got := (Prediction{}).Describe(); got != "" {
		t.Errorf("empty prediction should render empty string; got %q", got)
	}
}

func TestSuggestion_WithTwinPrediction_AppendsImpact(t *testing.T) {
	s := Suggestion{Kind: AddCache, Service: "catalog", Score: 0.7, Impact: "Typically reduces p99 by 30-60%."}
	p := Prediction{
		MetricDeltas: map[string]float64{"latency_p99": -0.63},
		Simulated:    true,
		Note:         "Twin ran 3-min p99 scenario.",
	}
	got := s.WithTwinPrediction(p)
	if !strings.Contains(got.Impact, "Twin ran 3-min") {
		t.Errorf("twin note should prefix impact: %q", got.Impact)
	}
	if !strings.Contains(got.Impact, "latency_p99 -63%") {
		t.Errorf("prediction deltas should appear in impact: %q", got.Impact)
	}
	// Immutability check: the original Suggestion was not modified.
	if strings.Contains(s.Impact, "Twin ran") {
		t.Errorf("original suggestion should not be mutated")
	}
}

func TestSuggestion_Rank_Buckets(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{0.95, "critical"},
		{0.72, "high"},
		{0.45, "medium"},
		{0.1, "low"},
	}
	for _, c := range cases {
		got := Suggestion{Score: c.score}.Rank()
		if got != c.want {
			t.Errorf("Rank(%v) = %q; want %q", c.score, got, c.want)
		}
	}
}
