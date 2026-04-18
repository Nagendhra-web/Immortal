package twin

import (
	"math"
	"math/rand/v2"
	"sort"
)

// MCConfig controls stochastic simulation.
type MCConfig struct {
	Runs       int     // default 200
	NoiseScale float64 // relative noise injected into each metric per step (default 0.05)
	Seed       uint64  // for reproducibility
}

// MCSimulation is Simulation + uncertainty quantiles.
type MCSimulation struct {
	Simulation          // deterministic center
	ScoreP05  float64
	ScoreP50  float64  // median over MCRuns
	ScoreP95  float64
	Worst     Simulation // run with lowest EndScore
	Best      Simulation // run with highest EndScore
	Runs      int
}

func (cfg MCConfig) withDefaults() MCConfig {
	if cfg.Runs <= 0 {
		cfg.Runs = 200
	}
	if cfg.NoiseScale == 0 {
		cfg.NoiseScale = 0.05
	}
	return cfg
}

// SimulateMC runs the plan Runs times with noise on metric values; returns
// quantile bounds. Accepted is set based on P05 >= StartScore - Tolerance
// rather than the mean, to be conservative about noisy environments.
func (t *Twin) SimulateMC(p Plan, cfg MCConfig) MCSimulation {
	cfg = cfg.withDefaults()

	// Deterministic center run (no noise).
	center := t.Simulate(p)

	t.mu.RLock()
	baseStates := copyStates(t.states)
	tcfg := t.cfg
	t.mu.RUnlock()

	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed^0xdeadbeefcafe))

	scores := make([]float64, 0, cfg.Runs)
	sims := make([]Simulation, 0, cfg.Runs)

	for i := 0; i < cfg.Runs; i++ {
		noisy := applyNoise(copyStates(baseStates), rng, cfg.NoiseScale)
		sim := runSimOnStates(noisy, p, tcfg)
		scores = append(scores, sim.EndScore)
		sims = append(sims, sim)
	}

	sort.Float64s(scores)

	p05 := percentile(scores, 0.05)
	p50 := percentile(scores, 0.50)
	p95 := percentile(scores, 0.95)

	// Find best and worst runs.
	worst := sims[0]
	best := sims[0]
	for _, s := range sims[1:] {
		if s.EndScore < worst.EndScore {
			worst = s
		}
		if s.EndScore > best.EndScore {
			best = s
		}
	}

	// Override Accepted/Rejected based on P05 conservative criterion.
	threshold := center.StartScore - tcfg.Tolerance
	mc := MCSimulation{
		Simulation: center,
		ScoreP05:   p05,
		ScoreP50:   p50,
		ScoreP95:   p95,
		Worst:      worst,
		Best:       best,
		Runs:       cfg.Runs,
	}

	// Re-evaluate acceptance: use P05 for conservative gate.
	if p05 >= threshold {
		mc.Accepted = true
		mc.Rejected = false
	} else {
		mc.Accepted = false
		mc.Rejected = true
		mc.Reason = "MC P05 score below tolerance threshold"
	}

	return mc
}

// applyNoise multiplies CPU/Memory/Latency/ErrorRate by 1 + N(0, scale).
func applyNoise(states map[string]State, rng *rand.Rand, scale float64) map[string]State {
	for k, s := range states {
		s.CPU = clamp0(s.CPU * (1 + normalNoise(rng, scale)))
		s.Memory = clamp0(s.Memory * (1 + normalNoise(rng, scale)))
		s.Latency = clamp0(s.Latency * (1 + normalNoise(rng, scale)))
		s.ErrorRate = clampUnit(s.ErrorRate * (1 + normalNoise(rng, scale)))
		states[k] = s
	}
	return states
}

// normalNoise returns a sample from N(0, scale) using Box-Muller.
func normalNoise(rng *rand.Rand, scale float64) float64 {
	u1 := rng.Float64()
	u2 := rng.Float64()
	if u1 < 1e-10 {
		u1 = 1e-10
	}
	z := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
	return z * scale
}

func clamp0(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// runSimOnStates runs a plan simulation starting from a pre-built states map.
func runSimOnStates(start map[string]State, p Plan, cfg Config) Simulation {
	current := copyStates(start)
	startScore := cfg.Score(current)

	sim := Simulation{
		PlanID:     p.ID,
		Start:      copyStates(start),
		StartScore: startScore,
	}

	for _, action := range p.Actions {
		modeled := false
		for _, model := range cfg.EffectModels {
			target, exists := current[action.Target]
			if !exists {
				target = State{Service: action.Target}
			}
			prevHealthy := target.Healthy
			next, ok := model(target, action)
			if ok {
				next.Service = action.Target
				current[action.Target] = next
				if !prevHealthy && next.Healthy {
					current = applyDependencyEffects(current, action.Target, action.Type)
				}
				modeled = true
				break
			}
		}
		if modeled {
			sim.ModeledSteps++
		} else {
			sim.UnmodeledSteps++
		}
		sim.StepStates = append(sim.StepStates, copyStates(current))
	}

	endScore := cfg.Score(current)
	sim.End = copyStates(current)
	sim.EndScore = endScore
	sim.Improvement = endScore - startScore

	threshold := startScore - cfg.Tolerance
	if endScore >= threshold {
		sim.Accepted = true
	} else {
		sim.Rejected = true
	}

	return sim
}

// percentile returns the p-th percentile of a sorted slice (0.0–1.0).
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}
