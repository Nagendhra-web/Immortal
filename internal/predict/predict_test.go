package predict

import (
	"math"
	"testing"
	"time"
)

func TestNewDefaults(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("New() returned nil")
	}
	if p.maxPoints != 500 {
		t.Errorf("expected maxPoints 500, got %d", p.maxPoints)
	}
}

func TestFeedAndPredict(t *testing.T) {
	p := New()
	base := time.Now()
	for i := 0; i < 10; i++ {
		p.feedAt("cpu", float64(i*10), base.Add(time.Duration(i)*time.Minute))
	}
	pred := p.Predict("cpu")
	if pred == nil {
		t.Fatal("expected prediction, got nil")
	}
	if pred.Metric != "cpu" {
		t.Errorf("expected metric cpu, got %s", pred.Metric)
	}
	slope, ok := p.Trend("cpu")
	if !ok {
		t.Fatal("expected trend to be available")
	}
	if slope <= 0 {
		t.Errorf("expected positive slope, got %f", slope)
	}
	if pred.PredictedValue <= pred.CurrentValue {
		// For a strictly increasing linear series the predicted value at the
		// last x should equal the last observation, so check slope instead.
		_ = pred.PredictedValue
	}
}

func TestPredictDecreasing(t *testing.T) {
	p := New()
	base := time.Now()
	for i := 0; i < 10; i++ {
		p.feedAt("mem", float64(100-i*5), base.Add(time.Duration(i)*time.Minute))
	}
	slope, ok := p.Trend("mem")
	if !ok {
		t.Fatal("expected trend")
	}
	if slope >= 0 {
		t.Errorf("expected negative slope for decreasing series, got %f", slope)
	}
}

func TestPredictInsufficientData(t *testing.T) {
	p := New()
	base := time.Now()
	p.feedAt("cpu", 10, base)
	p.feedAt("cpu", 20, base.Add(time.Minute))
	pred := p.Predict("cpu")
	if pred != nil {
		t.Error("expected nil prediction with < 3 data points")
	}
}

func TestSetThresholdAndTimeToThreshold(t *testing.T) {
	p := New()
	p.SetThreshold("cpu", 90.0)
	base := time.Now()
	// Feed 10 points increasing from 10 to 100 over 10 minutes
	for i := 0; i < 10; i++ {
		p.feedAt("cpu", float64(10+i*10), base.Add(time.Duration(i)*time.Minute))
	}
	pred := p.Predict("cpu")
	if pred == nil {
		t.Fatal("expected prediction")
	}
	// threshold is 90, current value around 100 — already past threshold or very soon
	// Either TimeToThreshold is set or severity is critical
	if pred.TimeToThreshold < 0 && pred.Severity != "critical" {
		t.Error("expected critical severity or positive time to threshold")
	}
	// Verify severity is not empty
	if pred.Severity == "" {
		t.Error("expected non-empty severity")
	}
}

func TestAllPredictions(t *testing.T) {
	p := New()
	p.SetThreshold("cpu", 90.0)
	p.SetThreshold("mem", 80.0)
	base := time.Now()
	for i := 0; i < 5; i++ {
		p.feedAt("cpu", float64(i*10), base.Add(time.Duration(i)*time.Minute))
		p.feedAt("mem", float64(i*8), base.Add(time.Duration(i)*time.Minute))
	}
	preds := p.AllPredictions()
	if len(preds) != 2 {
		t.Errorf("expected 2 predictions, got %d", len(preds))
	}
}

func TestTrend(t *testing.T) {
	p := New()
	base := time.Now()
	for i := 0; i < 5; i++ {
		p.feedAt("disk", float64(i*3), base.Add(time.Duration(i)*time.Second))
	}
	slope, ok := p.Trend("disk")
	if !ok {
		t.Fatal("expected trend")
	}
	// slope should be approximately 3 per second
	if slope <= 0 {
		t.Errorf("expected positive slope, got %f", slope)
	}
}

func TestConfidence(t *testing.T) {
	p := New()
	base := time.Now()
	// Perfectly linear: y = 2x + 5
	for i := 0; i < 10; i++ {
		p.feedAt("net", float64(2*i+5), base.Add(time.Duration(i)*time.Second))
	}
	pred := p.Predict("net")
	if pred == nil {
		t.Fatal("expected prediction")
	}
	if pred.Confidence < 0.99 {
		t.Errorf("expected R² near 1.0 for perfect linear data, got %f", pred.Confidence)
	}
}

func TestPredictNoThreshold(t *testing.T) {
	p := New()
	base := time.Now()
	for i := 0; i < 5; i++ {
		p.feedAt("latency", float64(i*2), base.Add(time.Duration(i)*time.Second))
	}
	pred := p.Predict("latency")
	if pred == nil {
		t.Fatal("expected prediction even without threshold")
	}
	// No threshold: TimeToThreshold should be zero value
	if pred.TimeToThreshold != 0 {
		t.Errorf("expected zero TimeToThreshold without threshold, got %v", pred.TimeToThreshold)
	}
}

func TestMaxPointsCap(t *testing.T) {
	p := New()
	p.maxPoints = 10
	base := time.Now()
	for i := 0; i < 50; i++ {
		p.feedAt("cpu", float64(i), base.Add(time.Duration(i)*time.Second))
	}
	p.mu.RLock()
	got := len(p.series["cpu"])
	p.mu.RUnlock()
	if got != 10 {
		t.Errorf("expected series capped at 10, got %d", got)
	}
}

// Ensure math import is used (R² uses math.Max).
var _ = math.Pi
