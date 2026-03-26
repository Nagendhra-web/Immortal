package brain_test

import (
	"testing"

	"github.com/immortal-engine/immortal/internal/brain"
	"github.com/immortal-engine/immortal/internal/dna"
	"github.com/immortal-engine/immortal/internal/event"
)

func TestPredictiveHealerDetectsTrend(t *testing.T) {
	d := dna.New("api-server")

	// Establish normal baseline
	for i := 0; i < 100; i++ {
		d.Record("cpu_percent", 45.0)
		d.Record("memory_percent", 60.0)
	}

	ph := brain.NewPredictiveHealer(d)

	// Feed gradually increasing CPU — should detect upward trend
	for i := 0; i < 20; i++ {
		e := event.New(event.TypeMetric, event.SeverityInfo, "cpu metric").
			WithSource("api-server").
			WithMeta("cpu_percent", 45.0+float64(i)*3)
		ph.Observe(e)
	}

	predictions := ph.Predict()
	if len(predictions) == 0 {
		t.Error("expected predictions for rising CPU trend")
	}

	foundCPU := false
	for _, p := range predictions {
		if p.Metric == "cpu_percent" {
			foundCPU = true
			if p.Direction != brain.TrendUp {
				t.Errorf("expected upward trend, got %s", p.Direction)
			}
		}
	}
	if !foundCPU {
		t.Error("expected cpu_percent prediction")
	}
}

func TestPredictiveHealerStableNoAlert(t *testing.T) {
	d := dna.New("api-server")

	// Baseline with natural variation
	for i := 0; i < 100; i++ {
		d.Record("cpu_percent", 43.0+float64(i%5))
	}

	ph := brain.NewPredictiveHealer(d)

	// Feed stable values within baseline range — should NOT predict failure
	for i := 0; i < 20; i++ {
		e := event.New(event.TypeMetric, event.SeverityInfo, "cpu metric").
			WithSource("api-server").
			WithMeta("cpu_percent", 44.0+float64(i%3))
		ph.Observe(e)
	}

	predictions := ph.Predict()
	for _, p := range predictions {
		if p.Metric == "cpu_percent" && p.Risk > 0.5 {
			t.Errorf("stable metrics should have low risk, got %f", p.Risk)
		}
	}
}
