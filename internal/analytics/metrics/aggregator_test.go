package metrics_test

import (
	"testing"

	"github.com/immortal-engine/immortal/internal/analytics/metrics"
)

func TestAggregatorRecordAndSummarize(t *testing.T) {
	a := metrics.New(1000)

	for i := 1; i <= 100; i++ {
		a.Record("response_time", float64(i))
	}

	s := a.Summarize("response_time")
	if s == nil {
		t.Fatal("expected summary")
	}
	if s.Count != 100 {
		t.Errorf("expected 100, got %d", s.Count)
	}
	if s.Min != 1 {
		t.Errorf("expected min 1, got %f", s.Min)
	}
	if s.Max != 100 {
		t.Errorf("expected max 100, got %f", s.Max)
	}
	if s.Mean != 50.5 {
		t.Errorf("expected mean 50.5, got %f", s.Mean)
	}
	if s.P95 < 90 {
		t.Errorf("P95 too low: %f", s.P95)
	}
	if s.P99 < 95 {
		t.Errorf("P99 too low: %f", s.P99)
	}
}

func TestAggregatorNonexistent(t *testing.T) {
	a := metrics.New(100)
	if a.Summarize("nope") != nil {
		t.Error("expected nil for nonexistent metric")
	}
}

func TestAggregatorNames(t *testing.T) {
	a := metrics.New(100)
	a.Record("cpu", 50)
	a.Record("memory", 60)
	a.Record("disk", 70)

	names := a.Names()
	if len(names) != 3 {
		t.Errorf("expected 3 names, got %d", len(names))
	}
}

func TestAggregatorMaxSize(t *testing.T) {
	a := metrics.New(5)

	for i := 0; i < 10; i++ {
		a.Record("test", float64(i))
	}

	s := a.Summarize("test")
	if s.Count != 5 {
		t.Errorf("expected 5 (max size), got %d", s.Count)
	}
	if s.Min != 5 {
		t.Errorf("oldest values should be evicted, min should be 5, got %f", s.Min)
	}
}
