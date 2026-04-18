package federated

import (
	"errors"
	"math"
	"math/rand/v2"
)

// DPMechanism selects which noise distribution to use.
type DPMechanism int

const (
	DPLaplace  DPMechanism = iota
	DPGaussian DPMechanism = iota
)

// DPBudget tracks the total ε budget and how much has been consumed.
type DPBudget struct {
	Epsilon float64
	Delta   float64
	spent   float64
}

// Consume deducts eps from the remaining budget. Returns an error if the
// deduction would exceed the total Epsilon.
func (b *DPBudget) Consume(eps float64) error {
	if b.spent+eps > b.Epsilon {
		return errors.New("federated: DP budget exhausted")
	}
	b.spent += eps
	return nil
}

// Remaining returns the ε not yet consumed.
func (b *DPBudget) Remaining() float64 {
	return b.Epsilon - b.spent
}

// AddNoise adds calibrated noise to value given the mechanism, sensitivity, ε,
// and δ (δ only used by Gaussian).
//
// Laplace:  scale = sensitivity / epsilon
// Gaussian: sigma = sensitivity * sqrt(2 * ln(1.25/delta)) / epsilon
func AddNoise(rng *rand.Rand, mech DPMechanism, value, sensitivity, epsilon, delta float64) float64 {
	switch mech {
	case DPGaussian:
		sigma := sensitivity * math.Sqrt(2*math.Log(1.25/delta)) / epsilon
		return value + rng.NormFloat64()*sigma
	default: // DPLaplace
		scale := sensitivity / epsilon
		return value + laplaceNoise(rng, scale)
	}
}

// Clip bounds value to the symmetric interval [-C, C], ensuring L1 sensitivity
// is bounded by 2C for sums or C for means with known range.
func Clip(value, C float64) float64 {
	return math.Max(-C, math.Min(C, value))
}
