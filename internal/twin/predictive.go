package twin

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"sync"
	"time"
)

// ForecastConfig controls the forward predictor.
type ForecastConfig struct {
	Horizon      int           // default 30 — number of ticks to predict ahead
	TickDuration time.Duration // default 10s — how long each tick represents
	Alpha        float64       // level smoothing, default 0.4
	Beta         float64       // trend smoothing, default 0.2
	NoiseScale   float64       // MC noise per step, default 0.05
	Runs         int           // MC runs, default 200
	Seed         uint64
}

func (c ForecastConfig) withDefaults() ForecastConfig {
	if c.Horizon <= 0 {
		c.Horizon = 30
	}
	if c.TickDuration <= 0 {
		c.TickDuration = 10 * time.Second
	}
	if c.Alpha <= 0 {
		c.Alpha = 0.4
	}
	if c.Beta <= 0 {
		c.Beta = 0.2
	}
	if c.NoiseScale <= 0 {
		c.NoiseScale = 0.05
	}
	if c.Runs <= 0 {
		c.Runs = 200
	}
	return c
}

// Forecast is the predicted trajectory + MC envelope for a single service.
type Forecast struct {
	Service           string
	AtTick            []int     // [0..Horizon] tick indices
	ExpectedCPU       []float64 // point forecast per tick
	ExpectedLatency   []float64
	ExpectedErrorRate []float64
	P95CPU            []float64 // 95th-percentile MC outcome per tick (upper bound)
	P05CPU            []float64 // 5th percentile (lower bound)
	WillBreachCPU     bool      // did any tick's P95 exceed a configured threshold?
	BreachTick        int       // first tick where breach predicted (-1 if none)
	Confidence        float64   // 0..1: share of MC runs that agree on the breach
}

// holtState tracks Holt linear smoothing state for one metric.
type holtState struct {
	level float64
	trend float64
	init  bool
}

// update applies one observation to the Holt state and returns the new state.
func (h holtState) update(x, alpha, beta float64) holtState {
	if !h.init {
		return holtState{level: x, trend: 0, init: true}
	}
	prevLevel := h.level
	newLevel := alpha*x + (1-alpha)*(h.level+h.trend)
	newTrend := beta*(newLevel-prevLevel) + (1-beta)*h.trend
	return holtState{level: newLevel, trend: newTrend, init: true}
}

// forecast returns the point estimate k steps ahead.
func (h holtState) forecast(k int) float64 {
	return h.level + float64(k)*h.trend
}

// maxObs is the rolling window of observations kept per (service, metric).
const maxObs = 100

// serviceHistory stores Holt state + raw history for one service.
type serviceHistory struct {
	cpuHolt     holtState
	latHolt     holtState
	errHolt     holtState
	cpuObs      []float64
	latObs      []float64
	errObs      []float64
}

// Predictor observes a time-series of State snapshots and forecasts the next Horizon ticks.
type Predictor struct {
	mu      sync.Mutex
	cfg     ForecastConfig
	history map[string]*serviceHistory
}

// NewPredictor creates a Predictor with the given config (defaults applied).
func NewPredictor(cfg ForecastConfig) *Predictor {
	return &Predictor{
		cfg:     cfg.withDefaults(),
		history: make(map[string]*serviceHistory),
	}
}

// Observe records a state for a service at the current time.
func (p *Predictor) Observe(service string, s State) {
	p.mu.Lock()
	defer p.mu.Unlock()

	h, ok := p.history[service]
	if !ok {
		h = &serviceHistory{}
		p.history[service] = h
	}

	alpha := p.cfg.Alpha
	beta := p.cfg.Beta

	h.cpuHolt = h.cpuHolt.update(s.CPU, alpha, beta)
	h.latHolt = h.latHolt.update(s.Latency, alpha, beta)
	h.errHolt = h.errHolt.update(s.ErrorRate, alpha, beta)

	h.cpuObs = appendCapped(h.cpuObs, s.CPU)
	h.latObs = appendCapped(h.latObs, s.Latency)
	h.errObs = appendCapped(h.errObs, s.ErrorRate)
}

func appendCapped(sl []float64, v float64) []float64 {
	sl = append(sl, v)
	if len(sl) > maxObs {
		sl = sl[len(sl)-maxObs:]
	}
	return sl
}

// Forecast runs the prediction for service. breachThresholdCPU is the CPU value
// above which P95CPU[i] triggers a "will breach" flag.
func (p *Predictor) Forecast(service string, breachThresholdCPU float64) (Forecast, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	h, ok := p.history[service]
	if !ok {
		return Forecast{}, fmt.Errorf("predictor: no observations for service %q", service)
	}
	if !h.cpuHolt.init {
		return Forecast{}, fmt.Errorf("predictor: insufficient observations for service %q", service)
	}

	cfg := p.cfg
	horizon := cfg.Horizon

	// Point forecasts via Holt.
	atTick := make([]int, horizon)
	expCPU := make([]float64, horizon)
	expLat := make([]float64, horizon)
	expErr := make([]float64, horizon)

	for k := 0; k < horizon; k++ {
		atTick[k] = k + 1
		expCPU[k] = math.Max(0, h.cpuHolt.forecast(k+1))
		expLat[k] = math.Max(0, h.latHolt.forecast(k+1))
		expErr[k] = clampUnit(h.errHolt.forecast(k + 1))
	}

	// MC envelope for CPU.
	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^0xdeadbeefcafe))

	// cpuRuns[run][tick]
	cpuRuns := make([][]float64, cfg.Runs)
	for i := 0; i < cfg.Runs; i++ {
		traj := make([]float64, horizon)
		for k := 0; k < horizon; k++ {
			base := expCPU[k]
			noise := normalNoise(rng, cfg.NoiseScale) * base
			traj[k] = math.Max(0, base+noise)
		}
		cpuRuns[i] = traj
	}

	p05CPU := make([]float64, horizon)
	p95CPU := make([]float64, horizon)
	col := make([]float64, cfg.Runs)
	for k := 0; k < horizon; k++ {
		for i := 0; i < cfg.Runs; i++ {
			col[i] = cpuRuns[i][k]
		}
		sort.Float64s(col)
		p05CPU[k] = percentile(col, 0.05)
		p95CPU[k] = percentile(col, 0.95)
	}

	// Breach detection.
	breachTick := -1
	breachCount := 0

	for k := 0; k < horizon; k++ {
		if p95CPU[k] > breachThresholdCPU {
			if breachTick == -1 {
				breachTick = atTick[k]
				// Count how many runs breach at this tick.
				for i := 0; i < cfg.Runs; i++ {
					if cpuRuns[i][k] > breachThresholdCPU {
						breachCount++
					}
				}
			}
			break
		}
	}

	willBreach := breachTick != -1
	confidence := 0.0
	if willBreach {
		confidence = float64(breachCount) / float64(cfg.Runs)
	}

	return Forecast{
		Service:           service,
		AtTick:            atTick,
		ExpectedCPU:       expCPU,
		ExpectedLatency:   expLat,
		ExpectedErrorRate: expErr,
		P95CPU:            p95CPU,
		P05CPU:            p05CPU,
		WillBreachCPU:     willBreach,
		BreachTick:        breachTick,
		Confidence:        confidence,
	}, nil
}

// ForecastAll returns Forecast for every known service.
func (p *Predictor) ForecastAll(breachThresholdCPU float64) []Forecast {
	p.mu.Lock()
	services := make([]string, 0, len(p.history))
	for svc := range p.history {
		services = append(services, svc)
	}
	p.mu.Unlock()

	out := make([]Forecast, 0, len(services))
	for _, svc := range services {
		f, err := p.Forecast(svc, breachThresholdCPU)
		if err == nil {
			out = append(out, f)
		}
	}
	return out
}

// PredictedFailures returns services whose forecast predicts a breach, sorted by
// earliest predicted breach time (closest first).
func (p *Predictor) PredictedFailures(breachThresholdCPU float64) []Forecast {
	all := p.ForecastAll(breachThresholdCPU)
	var failing []Forecast
	for _, f := range all {
		if f.WillBreachCPU {
			failing = append(failing, f)
		}
	}
	sort.Slice(failing, func(i, j int) bool {
		return failing[i].BreachTick < failing[j].BreachTick
	})
	return failing
}
