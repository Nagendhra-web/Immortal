package dna_test

import (
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/dna"
)

func TestDNARecordAndBaseline(t *testing.T) {
	d := dna.New("api-server")

	// Feed it normal metrics
	for i := 0; i < 100; i++ {
		d.Record("cpu_percent", 45.0+float64(i%10))
		d.Record("memory_percent", 60.0+float64(i%5))
		d.Record("response_time_ms", 120.0+float64(i%20))
	}

	baseline := d.Baseline()
	if baseline == nil {
		t.Fatal("expected non-nil baseline")
	}

	cpuBaseline, ok := baseline["cpu_percent"]
	if !ok {
		t.Fatal("expected cpu_percent in baseline")
	}
	if cpuBaseline.Mean < 44 || cpuBaseline.Mean > 55 {
		t.Errorf("cpu mean out of range: %f", cpuBaseline.Mean)
	}
	if cpuBaseline.StdDev <= 0 {
		t.Error("expected positive std dev")
	}
}

func TestDNADetectsAnomaly(t *testing.T) {
	d := dna.New("api-server")

	// Establish normal baseline
	for i := 0; i < 100; i++ {
		d.Record("cpu_percent", 45.0+float64(i%10))
	}

	// Normal value — should NOT be anomaly
	if d.IsAnomaly("cpu_percent", 48.0) {
		t.Error("48.0 should not be anomaly for cpu baseline ~45-55")
	}

	// Extreme value — SHOULD be anomaly
	if !d.IsAnomaly("cpu_percent", 99.0) {
		t.Error("99.0 should be anomaly for cpu baseline ~45-55")
	}
}

func TestDNAHealthScore(t *testing.T) {
	d := dna.New("api-server")

	for i := 0; i < 100; i++ {
		d.Record("cpu_percent", 45.0)
		d.Record("memory_percent", 60.0)
	}

	// All normal — health should be high
	score := d.HealthScore(map[string]float64{
		"cpu_percent":    46.0,
		"memory_percent": 61.0,
	})
	if score < 0.8 {
		t.Errorf("expected high health score for normal values, got %f", score)
	}

	// All anomalous — health should be low
	score = d.HealthScore(map[string]float64{
		"cpu_percent":    99.0,
		"memory_percent": 99.0,
	})
	if score > 0.5 {
		t.Errorf("expected low health score for anomalous values, got %f", score)
	}
}

func TestDNADecay(t *testing.T) {
	d := dna.NewWithWindow("api-server", 10)

	// Fill window
	for i := 0; i < 10; i++ {
		d.Record("cpu", 50.0)
	}

	// Now feed higher values — baseline should shift
	for i := 0; i < 10; i++ {
		d.Record("cpu", 90.0)
	}

	baseline := d.Baseline()
	if baseline["cpu"].Mean < 70 {
		t.Errorf("expected baseline to shift toward 90, got mean %f", baseline["cpu"].Mean)
	}
}
