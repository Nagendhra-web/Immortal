package twin

// ScoreWeights defines pluggable weights for scoring service states.
type ScoreWeights struct {
	HealthyReplica   float64 // default +10
	UnhealthyReplica float64 // default -20
	ErrorRate        float64 // default -50
	Latency          float64 // default -0.01
	HighCPUPenalty   float64 // default -5 when CPU > HighCPUThreshold
	HighCPUThreshold float64 // default 90
}

// DefaultWeights returns the standard score weights matching DefaultScore behaviour.
func DefaultWeights() ScoreWeights {
	return ScoreWeights{
		HealthyReplica:   10,
		UnhealthyReplica: -20,
		ErrorRate:        -50,
		Latency:          -0.01,
		HighCPUPenalty:   -5,
		HighCPUThreshold: 90,
	}
}

// WeightedScore returns a ScoreFunc parameterized by the given weights.
func WeightedScore(w ScoreWeights) ScoreFunc {
	return func(states map[string]State) float64 {
		score := 0.0
		for _, s := range states {
			replicas := s.Replicas
			if replicas == 0 {
				replicas = 1
			}
			if s.Healthy {
				score += w.HealthyReplica * float64(replicas)
			} else {
				score += w.UnhealthyReplica * float64(replicas)
			}
			score += w.ErrorRate * s.ErrorRate
			score += w.Latency * s.Latency
			if s.CPU > w.HighCPUThreshold {
				score += w.HighCPUPenalty
			}
		}
		return score
	}
}

// Calibrator observes (before, action, after, labeledImprovement) tuples and
// adjusts weights via simple online regression (gradient descent scaffolding).
// Enterprise deployments replace with a full calibrator.
type Calibrator struct {
	weights ScoreWeights
	count   int
	lr      float64 // learning rate
}

// NewCalibrator creates a Calibrator starting from the given initial weights.
func NewCalibrator(initial ScoreWeights) *Calibrator {
	return &Calibrator{
		weights: initial,
		lr:      0.0001,
	}
}

// Observe records an observed (before, after, labeledImprovement) tuple and
// nudges weights toward explaining the labeled outcome.
// labeledImprovement > 0 means the action was good; < 0 means bad.
func (c *Calibrator) Observe(before, after map[string]State, labeledImprovement float64) {
	c.count++

	// Compute feature deltas between after and before (averaged across services).
	var deltaLatency, deltaErrorRate, deltaHealthy float64
	n := len(after)
	if n == 0 {
		return
	}

	for svc, a := range after {
		b, ok := before[svc]
		if !ok {
			continue
		}
		deltaLatency += a.Latency - b.Latency
		deltaErrorRate += a.ErrorRate - b.ErrorRate
		if a.Healthy && !b.Healthy {
			deltaHealthy += 1
		} else if !a.Healthy && b.Healthy {
			deltaHealthy -= 1
		}
	}
	deltaLatency /= float64(n)
	deltaErrorRate /= float64(n)
	deltaHealthy /= float64(n)

	// Simple gradient nudge: if labeledImprovement positive and latency went down,
	// increase the magnitude of the latency weight (it's negative, so decrease it).
	predicted := c.weights.Latency*deltaLatency +
		c.weights.ErrorRate*deltaErrorRate +
		c.weights.HealthyReplica*deltaHealthy

	err := labeledImprovement - predicted

	// Gradient step.
	c.weights.Latency += c.lr * err * deltaLatency
	c.weights.ErrorRate += c.lr * err * deltaErrorRate
	c.weights.HealthyReplica += c.lr * err * deltaHealthy

	// Keep HealthyReplica positive, ErrorRate/Latency negative.
	if c.weights.HealthyReplica < 0 {
		c.weights.HealthyReplica = 0
	}
	if c.weights.ErrorRate > 0 {
		c.weights.ErrorRate = 0
	}
	if c.weights.Latency > 0 {
		c.weights.Latency = 0
	}
}

// Weights returns the current calibrated weights.
func (c *Calibrator) Weights() ScoreWeights {
	return c.weights
}

// SampleCount returns the number of observations fed so far.
func (c *Calibrator) SampleCount() int {
	return c.count
}

// NewRankNetCalibratorFromWeights is a convenience wrapper that constructs a
// RankNetCalibrator seeded from the given ScoreWeights, using package defaults
// for learning-rate, momentum, and L2. It sits alongside NewCalibrator so
// callers can adopt RankNet without breaking existing gradient-descent code.
func NewRankNetCalibratorFromWeights(initial ScoreWeights) *RankNetCalibrator {
	return NewRankNetCalibrator(RankNetConfig{Initial: initial})
}
