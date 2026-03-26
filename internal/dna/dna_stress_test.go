package dna_test

import (
	"sync"
	"testing"

	"github.com/immortal-engine/immortal/internal/dna"
)

func TestDNAConcurrentRecords(t *testing.T) {
	d := dna.New("test")
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				d.Record("cpu", float64(n+j))
				d.Record("mem", float64(n*2+j))
			}
		}(i)
	}
	wg.Wait()

	baseline := d.Baseline()
	if len(baseline) != 2 {
		t.Errorf("expected 2 metrics, got %d", len(baseline))
	}
}

func TestDNAEmptyBaseline(t *testing.T) {
	d := dna.New("test")
	baseline := d.Baseline()
	if len(baseline) != 0 {
		t.Errorf("expected empty baseline, got %d metrics", len(baseline))
	}
}

func TestDNAIsAnomalyInsufficientData(t *testing.T) {
	d := dna.New("test")
	// Only 5 records — below threshold of 10
	for i := 0; i < 5; i++ {
		d.Record("cpu", 50.0)
	}
	// Should not flag anomaly with insufficient data
	if d.IsAnomaly("cpu", 99.0) {
		t.Error("should not detect anomaly with insufficient data")
	}
}

func TestDNAIsAnomalyUnknownMetric(t *testing.T) {
	d := dna.New("test")
	if d.IsAnomaly("nonexistent", 50.0) {
		t.Error("should not detect anomaly for unknown metric")
	}
}

func TestDNAHealthScoreEmpty(t *testing.T) {
	d := dna.New("test")
	score := d.HealthScore(map[string]float64{})
	if score != 1.0 {
		t.Errorf("expected 1.0 for empty input, got %f", score)
	}
}

func TestDNAHealthScoreUnknownMetrics(t *testing.T) {
	d := dna.New("test")
	score := d.HealthScore(map[string]float64{"unknown": 50.0})
	if score != 1.0 {
		t.Errorf("expected 1.0 for unknown metrics, got %f", score)
	}
}

func TestDNAWindowSize1(t *testing.T) {
	d := dna.NewWithWindow("test", 1)
	d.Record("cpu", 50.0)
	d.Record("cpu", 90.0)
	baseline := d.Baseline()
	if baseline["cpu"].Mean != 90.0 {
		t.Errorf("window 1 should only keep latest, got mean %f", baseline["cpu"].Mean)
	}
}

func TestDNASource(t *testing.T) {
	d := dna.New("my-service")
	if d.Source() != "my-service" {
		t.Errorf("expected source 'my-service', got '%s'", d.Source())
	}
}
