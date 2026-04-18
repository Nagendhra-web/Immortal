// Package causal provides causal discovery (PC algorithm) and causal effect
// estimation (do-calculus / adjustment-set approach) over tabular datasets.
// It distinguishes true causes from mere correlates — something Pearson
// correlation cannot do.
package causal

import (
	"errors"
	"fmt"
	"math"
	"sync"
)

// Dataset is a column-oriented table. Var[v] holds one observation per row.
// All columns must have the same length.
type Dataset struct {
	mu    sync.RWMutex
	Names []string
	Data  map[string][]float64
}

func NewDataset(names []string) *Dataset {
	d := &Dataset{
		Names: make([]string, len(names)),
		Data:  make(map[string][]float64, len(names)),
	}
	copy(d.Names, names)
	for _, n := range names {
		d.Data[n] = nil
	}
	return d
}

// Add appends one row. All names present in the Dataset must be supplied.
func (d *Dataset) Add(vals map[string]float64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, n := range d.Names {
		if _, ok := vals[n]; !ok {
			return fmt.Errorf("missing value for variable %q", n)
		}
	}
	for _, n := range d.Names {
		d.Data[n] = append(d.Data[n], vals[n])
	}
	return nil
}

// Rows returns the number of observations.
func (d *Dataset) Rows() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if len(d.Names) == 0 {
		return 0
	}
	return len(d.Data[d.Names[0]])
}

// pearson returns the Pearson correlation coefficient of two equal-length slices.
// Returns 0 if either slice is constant (zero variance).
func pearson(x, y []float64) float64 {
	n := len(x)
	if n == 0 {
		return 0
	}
	var mx, my float64
	for i := range x {
		mx += x[i]
		my += y[i]
	}
	mx /= float64(n)
	my /= float64(n)

	var cov, vx, vy float64
	for i := range x {
		dx := x[i] - mx
		dy := y[i] - my
		cov += dx * dy
		vx += dx * dx
		vy += dy * dy
	}
	if vx < 1e-15 || vy < 1e-15 {
		return 0
	}
	r := cov / math.Sqrt(vx*vy)
	// clamp for numerical safety
	return clamp(r, -0.9999, 0.9999)
}

// clamp constrains v to [lo, hi].
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// correlationMatrix builds the p×p Pearson correlation matrix for the listed variables.
func correlationMatrix(ds *Dataset, vars []string) *Matrix {
	p := len(vars)
	C := NewMatrix(p, p)
	for i, vi := range vars {
		C.Set(i, i, 1)
		for j := i + 1; j < p; j++ {
			r := pearson(ds.Data[vi], ds.Data[vars[j]])
			C.Set(i, j, r)
			C.Set(j, i, r)
		}
	}
	return C
}

// partialCorr computes ρ(X, Y | Z) using the inverse of the correlation submatrix
// of {X, Y} ∪ Z. Returns (r, ok); ok=false when the matrix is singular or
// the variables cannot be resolved.
func partialCorr(ds *Dataset, x, y string, z []string) (float64, bool) {
	vars := append([]string{x, y}, z...)
	C := correlationMatrix(ds, vars)
	P, err := Inverse(C)
	if err != nil {
		return 0, false
	}
	p00 := P.Get(0, 0)
	p11 := P.Get(1, 1)
	p01 := P.Get(0, 1)
	denom := math.Sqrt(p00 * p11)
	if denom < 1e-15 {
		return 0, false
	}
	r := -p01 / denom
	r = clamp(r, -0.9999, 0.9999)
	if math.IsNaN(r) {
		return 0, false
	}
	return r, true
}

// fisherZ converts a partial correlation to a z-statistic for the CI test.
// n = sample size, condSize = |Z|.
func fisherZ(r float64, n, condSize int) float64 {
	// avoid log(0) — correlations already clamped
	z := 0.5 * math.Log((1+r)/(1-r))
	scale := math.Sqrt(float64(n-condSize-3))
	if scale < 0 {
		return 0
	}
	return math.Abs(z * scale)
}

// independentAt returns true if X ⊥ Y | Z at significance level alpha.
// Uses alpha=0.05 → critical value 1.96.
func independentAt(ds *Dataset, x, y string, z []string, alpha float64) bool {
	r, ok := partialCorr(ds, x, y, z)
	if !ok {
		return false
	}
	stat := fisherZ(r, ds.Rows(), len(z))
	// approximate N(0,1) quantile for two-sided test
	crit := normalQuantile(1 - alpha/2)
	return stat < crit
}

// normalQuantile returns the z-quantile of the standard normal via rational approximation.
// Accurate to ~3e-4 for p in (0.5, 1).
func normalQuantile(p float64) float64 {
	// Beasley-Springer-Moro approximation
	if p >= 1 {
		return 8
	}
	if p <= 0 {
		return -8
	}
	// use symmetry
	sign := 1.0
	if p < 0.5 {
		p = 1 - p
		sign = -1
	}
	t := math.Sqrt(-2 * math.Log(1-p))
	// coefficients for rational approximation
	c := [3]float64{2.515517, 0.802853, 0.010328}
	d := [3]float64{1.432788, 0.189269, 0.001308}
	num := c[0] + c[1]*t + c[2]*t*t
	den := 1 + d[0]*t + d[1]*t*t + d[2]*t*t*t
	return sign * (t - num/den)
}

// ErrInsufficientData is returned when there are too few rows to run the algorithm.
var ErrInsufficientData = errors.New("causal: insufficient data (need ≥ 10 rows)")
