package twin

import (
	"math"
	"math/rand/v2"
)

// PreferencePair is a labeled "A is better than B" outcome from real ops.
// Both states must contain the SAME service set; the model is told that the
// healing path leading to Better.End was preferred.
type PreferencePair struct {
	Better map[string]State // post-healing observed state when this option was used
	Worse  map[string]State // post-healing observed state when this option was used
}

// RankNetConfig holds hyperparameters for the RankNet calibrator.
type RankNetConfig struct {
	LearningRate float64      // default 0.01
	Momentum     float64      // default 0.9
	L2           float64      // default 0.001
	Initial      ScoreWeights // starting weights; zero value → DefaultWeights()
	Seed         uint64
}

// RankNetCalibrator learns score weights via pairwise sigmoid cross-entropy
// (Burges et al. 2005, "Learning to Rank using Gradient Descent", ICML 2005).
//
// Feature vector (5 elements matching ScoreWeights):
//
//	f[0] = sum of Replicas for Healthy services
//	f[1] = sum of Replicas for Unhealthy services
//	f[2] = sum of ErrorRate across all services
//	f[3] = sum of Latency across all services
//	f[4] = count of services with CPU > HighCPUThreshold (fixed 90)
//
// Score(states) = w·f
type RankNetCalibrator struct {
	w        [5]float64 // learned weights
	velocity [5]float64 // momentum accumulator
	pairs    []PreferencePair

	lr   float64
	mom  float64
	l2   float64
	rng  *rand.Rand
	seed uint64
}

const highCPUThreshold = 90.0

// NewRankNetCalibrator constructs a calibrator with the given config.
// Zero-value LearningRate/Momentum/L2 trigger defaults.
// Zero-value Initial triggers DefaultWeights().
func NewRankNetCalibrator(cfg RankNetConfig) *RankNetCalibrator {
	lr := cfg.LearningRate
	if lr == 0 {
		lr = 0.01
	}
	mom := cfg.Momentum
	if mom == 0 {
		mom = 0.9
	}
	l2 := cfg.L2
	if l2 == 0 {
		l2 = 0.001
	}

	init := cfg.Initial
	// If all fields are zero, use DefaultWeights.
	if init == (ScoreWeights{}) {
		init = DefaultWeights()
	}

	w := weightsToVec(init)

	seed := cfg.Seed
	if seed == 0 {
		seed = 42
	}

	return &RankNetCalibrator{
		w:    w,
		lr:   lr,
		mom:  mom,
		l2:   l2,
		rng:  rand.New(rand.NewPCG(seed, seed^0xdeadbeef)),
		seed: seed,
	}
}

// AddPair stores a single preference pair (Better, Worse).
func (c *RankNetCalibrator) AddPair(p PreferencePair) {
	c.pairs = append(c.pairs, p)
}

// PairCount returns how many preference pairs have been added.
func (c *RankNetCalibrator) PairCount() int {
	return len(c.pairs)
}

// Train runs N epochs of mini-batch SGD over all stored pairs.
// BatchSize 0 → full batch. Returns the mean log-loss per epoch.
func (c *RankNetCalibrator) Train(epochs, batchSize int) []float64 {
	n := len(c.pairs)
	if n == 0 || epochs <= 0 {
		return nil
	}
	if batchSize <= 0 || batchSize > n {
		batchSize = n
	}

	losses := make([]float64, epochs)

	// Build index slice for shuffling.
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}

	for e := range epochs {
		// Shuffle indices for this epoch.
		c.rng.Shuffle(n, func(i, j int) { idx[i], idx[j] = idx[j], idx[i] })

		epochLoss := 0.0

		// Process mini-batches.
		for start := 0; start < n; start += batchSize {
			end := start + batchSize
			if end > n {
				end = n
			}
			batch := idx[start:end]

			// Accumulate gradient over batch.
			var grad [5]float64
			batchLoss := 0.0

			for _, pi := range batch {
				p := c.pairs[pi]
				fA := featureVec(p.Better)
				fB := featureVec(p.Worse)

				sA := dot(c.w, fA)
				sB := dot(c.w, fB)

				diff := sA - sB
				// sigma = sigmoid(sB - sA) = 1/(1+exp(sA-sB))
				sigma := sigmoid(-diff)

				// loss = log(1 + exp(-(sA - sB)))
				batchLoss += softplus(-diff)

				// ∂L/∂w_i = sigma * (fB[i] - fA[i])
				for i := range grad {
					grad[i] += sigma * (fB[i] - fA[i])
				}
			}

			bsz := float64(len(batch))
			epochLoss += batchLoss

			// Update with momentum + L2 regularisation.
			for i := range c.w {
				g := grad[i]/bsz + c.l2*c.w[i]
				c.velocity[i] = c.mom*c.velocity[i] + g
				c.w[i] -= c.lr * c.velocity[i]
			}
		}

		losses[e] = epochLoss / float64(n)
	}

	return losses
}

// Weights returns the current learned ScoreWeights.
func (c *RankNetCalibrator) Weights() ScoreWeights {
	return vecToWeights(c.w)
}

// Score returns the calibrated score for a single state map.
// Equivalent to WeightedScore(c.Weights())(states).
func (c *RankNetCalibrator) Score(states map[string]State) float64 {
	f := featureVec(states)
	return dot(c.w, f)
}

// Loss computes mean log-loss over all stored pairs at current weights.
func (c *RankNetCalibrator) Loss() float64 {
	if len(c.pairs) == 0 {
		return 0
	}
	total := 0.0
	for _, p := range c.pairs {
		fA := featureVec(p.Better)
		fB := featureVec(p.Worse)
		sA := dot(c.w, fA)
		sB := dot(c.w, fB)
		total += softplus(-(sA - sB))
	}
	return total / float64(len(c.pairs))
}

// ─── internal helpers ────────────────────────────────────────────────────────

// featureVec computes the 5-element feature vector for a state map.
func featureVec(states map[string]State) [5]float64 {
	var f [5]float64
	for _, s := range states {
		replicas := float64(s.Replicas)
		if replicas == 0 {
			replicas = 1
		}
		if s.Healthy {
			f[0] += replicas // healthy capacity
		} else {
			f[1] += replicas // unhealthy capacity
		}
		f[2] += s.ErrorRate
		f[3] += s.Latency
		if s.CPU > highCPUThreshold {
			f[4]++
		}
	}
	return f
}

// weightsToVec maps ScoreWeights → [5]float64.
// f[1] accumulates unhealthy replicas; the weight for it is UnhealthyReplica.
func weightsToVec(sw ScoreWeights) [5]float64 {
	return [5]float64{
		sw.HealthyReplica,
		sw.UnhealthyReplica,
		sw.ErrorRate,
		sw.Latency,
		sw.HighCPUPenalty,
	}
}

// vecToWeights maps [5]float64 → ScoreWeights, preserving HighCPUThreshold.
func vecToWeights(w [5]float64) ScoreWeights {
	return ScoreWeights{
		HealthyReplica:   w[0],
		UnhealthyReplica: w[1],
		ErrorRate:        w[2],
		Latency:          w[3],
		HighCPUPenalty:   w[4],
		HighCPUThreshold: highCPUThreshold,
	}
}

func dot(w, f [5]float64) float64 {
	s := 0.0
	for i := range w {
		s += w[i] * f[i]
	}
	return s
}

// sigmoid = 1/(1+exp(-x)), numerically stable.
func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

// softplus = log(1 + exp(x)), numerically stable for large |x|.
func softplus(x float64) float64 {
	if x > 30 {
		return x
	}
	if x < -30 {
		return math.Exp(x)
	}
	return math.Log1p(math.Exp(x))
}
