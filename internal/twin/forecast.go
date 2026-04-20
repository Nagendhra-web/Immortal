package twin

import (
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// Horizon and step sizes for long-horizon forecasting.
//
// The forecaster walks the twin state forward Horizon / StepSize times,
// applying a stochastic perturbation on each step. Paths parallel
// trajectories are aggregated into per-step (mean, p10, p90) bands so
// the dashboard can plot "where will we be tomorrow 9 AM, and what is
// the uncertainty".
//
// This ships without a Kalman correction step; drift is bounded only by
// the ar1Decay term. Adding a real Kalman filter + ARIMA residuals is a
// follow-up (tracked in #39 as the polish layer).
type ForecastRequest struct {
	Horizon  time.Duration // total forecast window, e.g. 24*time.Hour
	StepSize time.Duration // granularity, e.g. 5*time.Minute
	Paths    int           // Monte-Carlo paths; defaults to 64 if <= 0
	Metrics  []string      // which metric names to project: "latency", "cpu", "memory", "errorRate", "replicas"
	Baseline map[string]State // starting state; typically the twin's current snapshot
	Seed     int64         // RNG seed; 0 = time-based
}

// ForecastResult is a grid: [metric][step] -> (mean, p10, p90).
type ForecastResult struct {
	StartedAt time.Time
	StepSize  time.Duration
	Paths     int
	// Bands: metric -> service -> slice of ForecastBand (one per step).
	Bands      map[string]map[string][]ForecastBand
	Timestamps []time.Time // one entry per step, aligned with each Bands slice
}

// ForecastBand is the aggregated state at a single step of the forecast.
type ForecastBand struct {
	Mean float64 `json:"mean"`
	P10  float64 `json:"p10"`
	P90  float64 `json:"p90"`
}

// Forecast runs a Monte-Carlo projection of Baseline states forward.
func (t *Twin) Forecast(req ForecastRequest) ForecastResult {
	if req.Horizon <= 0 {
		req.Horizon = 24 * time.Hour
	}
	if req.StepSize <= 0 {
		req.StepSize = 5 * time.Minute
	}
	if req.Paths <= 0 {
		req.Paths = 64
	}
	if len(req.Metrics) == 0 {
		req.Metrics = []string{"latency", "cpu", "memory", "errorRate"}
	}
	if req.Baseline == nil {
		req.Baseline = t.States()
	}
	seed := req.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	steps := int(req.Horizon / req.StepSize)
	if steps < 1 {
		steps = 1
	}
	startedAt := time.Now()

	// Allocate output grid: metric -> service -> [step]Band.
	// We also need a temporary aggregation structure: per metric per
	// service per step, a slice of Paths samples to compute percentiles.
	pathsBuf := make(map[string]map[string][][]float64, len(req.Metrics))
	for _, m := range req.Metrics {
		pathsBuf[m] = make(map[string][][]float64)
		for svc := range req.Baseline {
			pathsBuf[m][svc] = make([][]float64, steps)
			for s := 0; s < steps; s++ {
				pathsBuf[m][svc][s] = make([]float64, 0, req.Paths)
			}
		}
	}

	// Run Paths in parallel (bounded to GOMAXPROCS implicitly via Go runtime).
	var wg sync.WaitGroup
	var mu sync.Mutex
	for p := 0; p < req.Paths; p++ {
		wg.Add(1)
		go func(pathIdx int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed + int64(pathIdx)))
			current := copyStates(req.Baseline)
			for stepIdx := 0; stepIdx < steps; stepIdx++ {
				evolve(current, rng)
				mu.Lock()
				for _, metric := range req.Metrics {
					for svc, st := range current {
						pathsBuf[metric][svc][stepIdx] = append(pathsBuf[metric][svc][stepIdx], metricValue(st, metric))
					}
				}
				mu.Unlock()
			}
		}(p)
	}
	wg.Wait()

	// Aggregate per step into (mean, p10, p90).
	bands := make(map[string]map[string][]ForecastBand, len(req.Metrics))
	for metric, services := range pathsBuf {
		bands[metric] = make(map[string][]ForecastBand, len(services))
		for svc, stepSlices := range services {
			row := make([]ForecastBand, steps)
			for s, samples := range stepSlices {
				row[s] = summarize(samples)
			}
			bands[metric][svc] = row
		}
	}

	// Timestamps grid.
	ts := make([]time.Time, steps)
	for i := 0; i < steps; i++ {
		ts[i] = startedAt.Add(time.Duration(i+1) * req.StepSize)
	}

	return ForecastResult{
		StartedAt:  startedAt,
		StepSize:   req.StepSize,
		Paths:      req.Paths,
		Bands:      bands,
		Timestamps: ts,
	}
}

// evolve applies one step of stochastic drift to every state in place.
// Latency + cpu + errorRate each get an AR(1) random walk: next = decay *
// current + (1 - decay) * equilibrium + noise. This produces mean-reverting
// paths with realistic spread.
//
// Services are walked in sorted-key order so that, given the same rng
// seed, the random draws are assigned to the same service in the same
// order across calls. Without this, Go's randomized map iteration would
// make forecasts non-deterministic.
func evolve(states map[string]State, rng *rand.Rand) {
	const (
		ar1Decay     = 0.95
		latencyEquil = 80.0
		cpuEquil     = 30.0
		memEquil     = 40.0
		errEquil     = 0.01
	)
	keys := make([]string, 0, len(states))
	for k := range states {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, svc := range keys {
		st := states[svc]
		st.Latency = ar1Decay*st.Latency + (1-ar1Decay)*latencyEquil + rng.NormFloat64()*st.Latency*0.05
		if st.Latency < 1 {
			st.Latency = 1
		}
		st.CPU = clampTo(ar1Decay*st.CPU+(1-ar1Decay)*cpuEquil+rng.NormFloat64()*5, 0, 100)
		st.Memory = clampTo(ar1Decay*st.Memory+(1-ar1Decay)*memEquil+rng.NormFloat64()*3, 0, 100)
		st.ErrorRate = clampTo(ar1Decay*st.ErrorRate+(1-ar1Decay)*errEquil+rng.NormFloat64()*0.002, 0, 1)
		// Replicas do not drift in the baseline model; autoscaling decisions
		// are made deterministically by the healing engine, not by the
		// forecast simulator.
		states[svc] = st
	}
}

func metricValue(s State, metric string) float64 {
	switch metric {
	case "latency":
		return s.Latency
	case "cpu":
		return s.CPU
	case "memory":
		return s.Memory
	case "errorRate":
		return s.ErrorRate
	case "replicas":
		return float64(s.Replicas)
	default:
		return 0
	}
}

func summarize(samples []float64) ForecastBand {
	if len(samples) == 0 {
		return ForecastBand{}
	}
	cp := append([]float64(nil), samples...)
	sort.Float64s(cp)
	var sum float64
	for _, v := range cp {
		sum += v
	}
	return ForecastBand{
		Mean: sum / float64(len(cp)),
		P10:  percentileForecast(cp, 0.10),
		P90:  percentileForecast(cp, 0.90),
	}
}

func percentileForecast(sorted []float64, p float64) float64 {
	idx := int(math.Round(float64(len(sorted)-1) * p))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func clampTo(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
