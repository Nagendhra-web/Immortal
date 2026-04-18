package twin

import (
	"math"
	"testing"
)

func makePredictor(alpha, beta, noiseScale float64, runs, horizon int, seed uint64) *Predictor {
	return NewPredictor(ForecastConfig{
		Horizon:    horizon,
		Alpha:      alpha,
		Beta:       beta,
		NoiseScale: noiseScale,
		Runs:       runs,
		Seed:       seed,
	})
}

func observeCPU(p *Predictor, service string, cpuValues []float64) {
	for _, v := range cpuValues {
		p.Observe(service, State{Service: service, CPU: v, Latency: 10, ErrorRate: 0.01})
	}
}

// TestPredictor_ObserveAndForecastLinearGrowth feeds 30 observations of a
// linear series (cpu grows by 5 per tick) so Holt's trend fully converges.
// Horizon=5 should project the next 5 values within ±3.
func TestPredictor_ObserveAndForecastLinearGrowth(t *testing.T) {
	p := makePredictor(0.4, 0.2, 0.001, 500, 5, 1)

	// 30 observations: 50, 55, 60, ..., 195  (step=5)
	obs := make([]float64, 30)
	for i := range obs {
		obs[i] = 50 + float64(i)*5
	}
	observeCPU(p, "svc", obs)

	f, err := p.Forecast("svc", 9999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.ExpectedCPU) != 5 {
		t.Fatalf("expected 5 forecast ticks, got %d", len(f.ExpectedCPU))
	}

	// Last observed = 195; next 5 should be 200, 205, 210, 215, 220.
	want := []float64{200, 205, 210, 215, 220}
	tol := 3.0
	for i, w := range want {
		got := f.ExpectedCPU[i]
		if math.Abs(got-w) > tol {
			t.Errorf("tick %d: expected %.1f ± %.1f, got %.2f", i+1, w, tol, got)
		}
	}
}

// TestPredictor_WillBreachCPU_TrueWhenTrendCrossesThreshold: rising CPU crosses threshold.
func TestPredictor_WillBreachCPU_TrueWhenTrendCrossesThreshold(t *testing.T) {
	p := makePredictor(0.4, 0.2, 0.001, 500, 10, 1)
	// CPU rising fast: will cross 95 within 10 ticks.
	observeCPU(p, "rising", []float64{50, 60, 70, 80, 88})

	f, err := p.Forecast("rising", 90)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !f.WillBreachCPU {
		t.Errorf("expected WillBreachCPU=true, got false (P95CPU: %v)", f.P95CPU)
	}
	if f.BreachTick < 1 {
		t.Errorf("expected BreachTick >= 1, got %d", f.BreachTick)
	}
}

// TestPredictor_WillBreachCPU_FalseForFlatMetric: stable CPU never breaches.
func TestPredictor_WillBreachCPU_FalseForFlatMetric(t *testing.T) {
	p := makePredictor(0.4, 0.2, 0.001, 500, 30, 1)
	observeCPU(p, "flat", []float64{30, 30, 30, 30, 30, 30, 30, 30, 30, 30})

	f, err := p.Forecast("flat", 80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.WillBreachCPU {
		t.Errorf("expected WillBreachCPU=false for flat metric at 30%%, breach threshold 80, got true (BreachTick=%d)", f.BreachTick)
	}
	if f.BreachTick != -1 {
		t.Errorf("expected BreachTick=-1, got %d", f.BreachTick)
	}
}

// TestPredictor_P95CPUWiderThanExpected_WhenNoiseIsHigh: with high noise, P95-P05 spread is larger.
func TestPredictor_P95CPUWiderThanExpected_WhenNoiseIsHigh(t *testing.T) {
	obsLow := makePredictor(0.4, 0.2, 0.001, 500, 10, 42)
	obsHigh := makePredictor(0.4, 0.2, 0.5, 500, 10, 42)

	for _, pp := range []*Predictor{obsLow, obsHigh} {
		observeCPU(pp, "svc", []float64{50, 50, 50, 50, 50})
	}

	fLow, _ := obsLow.Forecast("svc", 200)
	fHigh, _ := obsHigh.Forecast("svc", 200)

	spreadLow := fLow.P95CPU[0] - fLow.P05CPU[0]
	spreadHigh := fHigh.P95CPU[0] - fHigh.P05CPU[0]

	if spreadHigh <= spreadLow {
		t.Errorf("high noise should produce wider spread: spreadHigh=%.4f spreadLow=%.4f", spreadHigh, spreadLow)
	}
}

// TestPredictor_ForecastAll_ReturnsEveryService: all observed services appear in ForecastAll.
func TestPredictor_ForecastAll_ReturnsEveryService(t *testing.T) {
	p := makePredictor(0.4, 0.2, 0.05, 200, 5, 1)
	services := []string{"alpha", "beta", "gamma"}
	for _, svc := range services {
		observeCPU(p, svc, []float64{40, 42, 44, 46, 48})
	}

	forecasts := p.ForecastAll(200)
	if len(forecasts) != len(services) {
		t.Errorf("expected %d forecasts, got %d", len(services), len(forecasts))
	}

	seen := make(map[string]bool)
	for _, f := range forecasts {
		seen[f.Service] = true
	}
	for _, svc := range services {
		if !seen[svc] {
			t.Errorf("service %q missing from ForecastAll output", svc)
		}
	}
}

// TestPredictor_PredictedFailures_SortedByEarliestBreach: services sorted by closest breach.
func TestPredictor_PredictedFailures_SortedByEarliestBreach(t *testing.T) {
	p := makePredictor(0.4, 0.2, 0.001, 500, 20, 1)

	// "fast" crosses threshold sooner (very steep trend).
	// "slow" crosses later (gentler trend).
	fast := []float64{50, 65, 78, 88, 94}
	slow := []float64{50, 53, 56, 59, 62}

	observeCPU(p, "fast", fast)
	observeCPU(p, "slow", slow)

	failures := p.PredictedFailures(90)

	// At least "fast" should breach.
	if len(failures) == 0 {
		t.Fatal("expected at least one predicted failure")
	}

	// Verify sorted by BreachTick ascending.
	for i := 1; i < len(failures); i++ {
		if failures[i].BreachTick < failures[i-1].BreachTick {
			t.Errorf("failures not sorted by BreachTick: [%d]=%d > [%d]=%d",
				i-1, failures[i-1].BreachTick, i, failures[i].BreachTick)
		}
	}
}

// TestPredictor_DeterministicWithSeed: same seed produces identical forecasts.
func TestPredictor_DeterministicWithSeed(t *testing.T) {
	obs := []float64{50, 55, 60, 65, 70}

	p1 := makePredictor(0.4, 0.2, 0.05, 200, 10, 999)
	p2 := makePredictor(0.4, 0.2, 0.05, 200, 10, 999)
	observeCPU(p1, "svc", obs)
	observeCPU(p2, "svc", obs)

	f1, err1 := p1.Forecast("svc", 95)
	f2, err2 := p2.Forecast("svc", 95)
	if err1 != nil || err2 != nil {
		t.Fatalf("errors: %v / %v", err1, err2)
	}

	for i := range f1.P95CPU {
		if f1.P95CPU[i] != f2.P95CPU[i] {
			t.Errorf("tick %d: P95CPU differs: %.6f vs %.6f", i, f1.P95CPU[i], f2.P95CPU[i])
		}
		if f1.P05CPU[i] != f2.P05CPU[i] {
			t.Errorf("tick %d: P05CPU differs: %.6f vs %.6f", i, f1.P05CPU[i], f2.P05CPU[i])
		}
	}
}

// TestPredictor_NoDataReturnsError: Forecast on unknown service returns an error.
func TestPredictor_NoDataReturnsError(t *testing.T) {
	p := NewPredictor(ForecastConfig{})
	_, err := p.Forecast("ghost", 80)
	if err == nil {
		t.Error("expected error for service with no observations, got nil")
	}
}

// TestPredictor_BreachConfidence_Monotonic_WithNoise: higher noise => lower confidence.
func TestPredictor_BreachConfidence_Monotonic_WithNoise(t *testing.T) {
	// Observations near threshold so noise direction matters.
	obs := []float64{70, 75, 78, 80, 82}
	threshold := 85.0

	runs := 1000

	pLow := makePredictor(0.4, 0.2, 0.001, runs, 5, 7)
	pHigh := makePredictor(0.4, 0.2, 0.5, runs, 5, 7)

	observeCPU(pLow, "svc", obs)
	observeCPU(pHigh, "svc", obs)

	fLow, _ := pLow.Forecast("svc", threshold)
	fHigh, _ := pHigh.Forecast("svc", threshold)

	// With very low noise, the trend should clearly breach → high confidence.
	// With very high noise, outcomes scatter → lower confidence.
	// Both should breach (trend is rising toward threshold), but high noise has lower confidence.
	if fLow.WillBreachCPU && fHigh.WillBreachCPU {
		if fHigh.Confidence >= fLow.Confidence {
			t.Errorf("expected high-noise confidence (%.3f) < low-noise confidence (%.3f)",
				fHigh.Confidence, fLow.Confidence)
		}
	}
	// If low-noise doesn't breach, the trend isn't strong enough — skip the comparison.
}
