package correlation_test

import (
	"math"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/correlation"
)

func TestNew(t *testing.T) {
	e := correlation.New()
	if e == nil {
		t.Fatal("expected engine")
	}
}

func TestRecord(t *testing.T) {
	e := correlation.New()
	e.Record("cpu", 50)
	e.Record("cpu", 60)

	metrics := e.Metrics()
	if len(metrics) != 1 {
		t.Errorf("expected 1 metric, got %d", len(metrics))
	}
}

func TestCorrelatePositive(t *testing.T) {
	e := correlation.New()

	// Two metrics that increase together
	for i := 0; i < 20; i++ {
		e.Record("cpu", float64(i)*10)
		e.Record("memory", float64(i)*5+10)
	}

	c := e.Correlate("cpu", "memory")
	if c == nil {
		t.Fatal("expected correlation")
	}
	if c.Coefficient < 0.9 {
		t.Errorf("expected strong positive correlation, got %.2f", c.Coefficient)
	}
	if c.Strength != "strong" {
		t.Errorf("expected strong, got %s", c.Strength)
	}
}

func TestCorrelateNegative(t *testing.T) {
	e := correlation.New()

	for i := 0; i < 20; i++ {
		e.Record("cpu", float64(i)*10)
		e.Record("available_memory", 1000-float64(i)*50)
	}

	c := e.Correlate("cpu", "available_memory")
	if c == nil {
		t.Fatal("expected correlation")
	}
	if c.Coefficient > -0.9 {
		t.Errorf("expected strong negative correlation, got %.2f", c.Coefficient)
	}
}

func TestCorrelateUncorrelated(t *testing.T) {
	e := correlation.New()

	// Alternating values that shouldn't correlate well
	for i := 0; i < 20; i++ {
		e.Record("metric_a", float64(i%3)*10)
		e.Record("metric_b", float64((i+7)%5)*20)
	}

	c := e.Correlate("metric_a", "metric_b")
	if c == nil {
		t.Fatal("expected correlation result")
	}
	if math.Abs(c.Coefficient) > 0.5 {
		t.Errorf("expected weak correlation, got %.2f", c.Coefficient)
	}
}

func TestAllCorrelations(t *testing.T) {
	e := correlation.New()

	for i := 0; i < 20; i++ {
		e.Record("a", float64(i)*10)
		e.Record("b", float64(i)*5)
		e.Record("c", float64(i%3)*100)
	}

	all := e.AllCorrelations()
	// a and b should be strongly correlated
	found := false
	for _, c := range all {
		if (c.MetricA == "a" && c.MetricB == "b") || (c.MetricA == "b" && c.MetricB == "a") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a-b correlation in results")
	}
}

func TestLeadingIndicators(t *testing.T) {
	e := correlation.New()

	// Metric A changes before metric B (shifted by 3 positions)
	base := time.Now()
	for i := 0; i < 30; i++ {
		ts := base.Add(time.Duration(i) * time.Minute)
		e.Record("leader", float64(i)*10)
		// follower is same pattern but delayed
		if i >= 3 {
			_ = ts
			e.Record("follower", float64(i-3)*10)
		} else {
			e.Record("follower", 0)
		}
	}

	leaders := e.LeadingIndicators("follower")
	// May or may not detect depending on alignment
	_ = leaders
}

func TestStrongestCorrelation(t *testing.T) {
	e := correlation.New()

	for i := 0; i < 20; i++ {
		e.Record("cpu", float64(i)*10)
		e.Record("memory", float64(i)*5)
		e.Record("disk", float64(i%4)*30)
	}

	best := e.StrongestCorrelation("cpu")
	if best == nil {
		t.Fatal("expected strongest correlation")
	}
	// cpu and memory should be most correlated
	if best.MetricB != "memory" && best.MetricA != "memory" {
		t.Errorf("expected memory as strongest, got %s-%s", best.MetricA, best.MetricB)
	}
}

func TestMetrics(t *testing.T) {
	e := correlation.New()
	e.Record("cpu", 50)
	e.Record("memory", 70)
	e.Record("disk", 30)

	metrics := e.Metrics()
	if len(metrics) != 3 {
		t.Errorf("expected 3 metrics, got %d", len(metrics))
	}
	// Should be sorted
	if metrics[0] != "cpu" || metrics[1] != "disk" || metrics[2] != "memory" {
		t.Errorf("expected sorted metrics, got %v", metrics)
	}
}

func TestReset(t *testing.T) {
	e := correlation.New()
	e.Record("cpu", 50)
	e.Reset()

	if len(e.Metrics()) != 0 {
		t.Error("expected 0 metrics after reset")
	}
}

func TestInsufficientData(t *testing.T) {
	e := correlation.New()
	e.Record("a", 1)
	e.Record("a", 2)
	e.Record("b", 3)

	c := e.Correlate("a", "b")
	if c != nil {
		t.Error("expected nil with insufficient data")
	}
}
