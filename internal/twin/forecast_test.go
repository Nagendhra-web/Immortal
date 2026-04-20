package twin

import (
	"testing"
	"time"
)

func baseline() map[string]State {
	return map[string]State{
		"api": {Service: "api", Latency: 80, CPU: 30, Memory: 40, ErrorRate: 0.01, Replicas: 3, Healthy: true},
		"db":  {Service: "db", Latency: 5, CPU: 20, Memory: 50, ErrorRate: 0.001, Replicas: 1, Healthy: true},
	}
}

func TestForecast_ShapeMatchesRequest(t *testing.T) {
	twn := New(Config{})
	res := twn.Forecast(ForecastRequest{
		Horizon:  1 * time.Hour,
		StepSize: 5 * time.Minute,
		Paths:    16,
		Metrics:  []string{"latency", "cpu"},
		Baseline: baseline(),
		Seed:     42,
	})
	wantSteps := 12
	if len(res.Timestamps) != wantSteps {
		t.Errorf("expected %d timestamps, got %d", wantSteps, len(res.Timestamps))
	}
	for _, metric := range []string{"latency", "cpu"} {
		svcMap, ok := res.Bands[metric]
		if !ok {
			t.Fatalf("metric %q missing from result", metric)
		}
		for _, svc := range []string{"api", "db"} {
			band, ok := svcMap[svc]
			if !ok {
				t.Fatalf("service %q missing for metric %q", svc, metric)
			}
			if len(band) != wantSteps {
				t.Errorf("%s/%s: want %d steps, got %d", metric, svc, wantSteps, len(band))
			}
		}
	}
}

func TestForecast_DeterministicUnderSameSeed(t *testing.T) {
	twn := New(Config{})
	req := ForecastRequest{
		Horizon:  10 * time.Minute,
		StepSize: time.Minute,
		Paths:    8,
		Metrics:  []string{"latency"},
		Baseline: baseline(),
		Seed:     1234,
	}
	a := twn.Forecast(req)
	b := twn.Forecast(req)
	for svc, bands := range a.Bands["latency"] {
		for i := range bands {
			if !nearlyEqual(bands[i].Mean, b.Bands["latency"][svc][i].Mean, 1e-9) {
				t.Fatalf("non-deterministic: svc=%s step=%d A=%v B=%v", svc, i, bands[i].Mean, b.Bands["latency"][svc][i].Mean)
			}
		}
	}
}

func TestForecast_P90GreaterOrEqualToP10(t *testing.T) {
	twn := New(Config{})
	res := twn.Forecast(ForecastRequest{
		Horizon:  20 * time.Minute,
		StepSize: 5 * time.Minute,
		Paths:    64,
		Baseline: baseline(),
		Seed:     7,
	})
	for metric, services := range res.Bands {
		for svc, bands := range services {
			for i, b := range bands {
				if b.P90 < b.P10-1e-9 {
					t.Errorf("%s/%s step=%d: p90 %v < p10 %v", metric, svc, i, b.P90, b.P10)
				}
				if b.Mean < b.P10-1e-9 || b.Mean > b.P90+1e-9 {
					t.Errorf("%s/%s step=%d: mean %v outside [p10 %v, p90 %v]", metric, svc, i, b.Mean, b.P10, b.P90)
				}
			}
		}
	}
}

func TestForecast_DefaultMetricsWhenEmpty(t *testing.T) {
	twn := New(Config{})
	res := twn.Forecast(ForecastRequest{
		Horizon:  10 * time.Minute,
		StepSize: 5 * time.Minute,
		Baseline: baseline(),
	})
	for _, expected := range []string{"latency", "cpu", "memory", "errorRate"} {
		if _, ok := res.Bands[expected]; !ok {
			t.Errorf("default metric %q missing", expected)
		}
	}
}

func TestForecast_UsesTwinStatesWhenBaselineNil(t *testing.T) {
	twn := New(Config{})
	twn.Observe(State{Service: "api", Latency: 100, Healthy: true})
	res := twn.Forecast(ForecastRequest{
		Horizon:  5 * time.Minute,
		StepSize: time.Minute,
		Paths:    4,
		Metrics:  []string{"latency"},
		Seed:     9,
	})
	if _, ok := res.Bands["latency"]["api"]; !ok {
		t.Errorf("forecast should default Baseline to twin.States(); api missing")
	}
}

func TestForecast_BoundedValues(t *testing.T) {
	twn := New(Config{})
	res := twn.Forecast(ForecastRequest{
		Horizon:  1 * time.Hour,
		StepSize: 5 * time.Minute,
		Paths:    32,
		Metrics:  []string{"cpu", "errorRate"},
		Baseline: baseline(),
		Seed:     3,
	})
	for svc, bands := range res.Bands["cpu"] {
		for i, b := range bands {
			if b.Mean < 0 || b.Mean > 100 {
				t.Errorf("%s cpu step=%d mean=%v outside [0,100]", svc, i, b.Mean)
			}
		}
	}
	for svc, bands := range res.Bands["errorRate"] {
		for i, b := range bands {
			if b.Mean < 0 || b.Mean > 1 {
				t.Errorf("%s errorRate step=%d mean=%v outside [0,1]", svc, i, b.Mean)
			}
		}
	}
}

func nearlyEqual(a, b, eps float64) bool {
	if a > b {
		return a-b <= eps
	}
	return b-a <= eps
}
