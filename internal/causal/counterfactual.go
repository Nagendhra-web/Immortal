package causal

import (
	"fmt"
	"sort"
)

// StructuralModel captures a linear-Gaussian Structural Causal Model (SCM)
// derived from observed data and a CausalGraph. For each variable V with
// parents, it stores OLS regression coefficients and empirical residuals
// (the exogenous noise U_V used in the abduction step of counterfactual inference).
type StructuralModel struct {
	Variables []string
	Parents   map[string][]string
	// Intercept[V] = β₀ in V = β₀ + Σ β_i·parent_i + U_V
	Intercept map[string]float64
	// Coefs[V][parent] = β_i for that parent.
	Coefs map[string]map[string]float64
	// Residuals[V][i] = U_V at observation i (V[i] - predicted(V[i]|parents)).
	Residuals map[string][]float64
}

// FitSCM fits a linear-Gaussian SCM to ds using the directed + undirected edges
// in g. Undirected neighbors are treated as possible parents (conservative).
// For root nodes (no parents), the intercept is the variable mean and residuals
// are mean-centred values.
func FitSCM(ds *Dataset, g CausalGraph) (*StructuralModel, error) {
	if ds.Rows() < 5 {
		return nil, ErrInsufficientData
	}

	n := ds.Rows()
	m := &StructuralModel{
		Variables: make([]string, len(g.Nodes)),
		Parents:   make(map[string][]string, len(g.Nodes)),
		Intercept: make(map[string]float64, len(g.Nodes)),
		Coefs:     make(map[string]map[string]float64, len(g.Nodes)),
		Residuals: make(map[string][]float64, len(g.Nodes)),
	}
	copy(m.Variables, g.Nodes)

	for _, v := range g.Nodes {
		// Collect parents: directed parents + undirected neighbors (possible parents).
		pset := map[string]bool{}
		for src, targets := range g.Directed {
			for _, t := range targets {
				if t == v {
					pset[src] = true
				}
			}
		}
		for _, nb := range g.Undirected[v] {
			pset[nb] = true
		}
		pars := make([]string, 0, len(pset))
		for p := range pset {
			pars = append(pars, p)
		}
		sort.Strings(pars)
		m.Parents[v] = pars

		resids := make([]float64, n)

		if len(pars) == 0 {
			// Root node: intercept = mean, residuals = demeaned values.
			var sum float64
			for _, val := range ds.Data[v] {
				sum += val
			}
			mu := sum / float64(n)
			m.Intercept[v] = mu
			m.Coefs[v] = map[string]float64{}
			for i, val := range ds.Data[v] {
				resids[i] = val - mu
			}
		} else {
			// Regress V on its parents.
			X := NewMatrix(n, len(pars))
			for ci, par := range pars {
				col := ds.Data[par]
				for i := 0; i < n; i++ {
					X.Set(i, ci, col[i])
				}
			}
			beta, err := OLS(ds.Data[v], X)
			if err != nil {
				return nil, fmt.Errorf("causal: FitSCM OLS for %q failed: %w", v, err)
			}
			// beta[0] = intercept, beta[1..] = parent coefficients
			m.Intercept[v] = beta[0]
			coefs := make(map[string]float64, len(pars))
			for ci, par := range pars {
				coefs[par] = beta[ci+1]
			}
			m.Coefs[v] = coefs

			// Compute residuals.
			yVals := ds.Data[v]
			for i := 0; i < n; i++ {
				pred := beta[0]
				for ci, par := range pars {
					pred += beta[ci+1] * ds.Data[par][i]
				}
				resids[i] = yVals[i] - pred
			}
		}
		m.Residuals[v] = resids
	}

	return m, nil
}

// CounterfactualResult holds the output of a single-row counterfactual query.
type CounterfactualResult struct {
	RowIndex              int
	Cause, Outcome        string
	ObservedCause         float64
	ObservedOutcome       float64
	DoValue               float64
	CounterfactualOutcome float64
	Difference            float64 // CounterfactualOutcome - ObservedOutcome
}

// Counterfactual answers "given observed row rowIndex, what would outcome have
// been had we intervened do(cause = doValue)?"
//
// Pearl's three-step procedure (linear-Gaussian case):
//  1. Abduction: infer U_V at rowIndex for every variable V as
//     U_V = V[rowIndex] - (intercept + Σ coef·parent[rowIndex]).
//  2. Action: override cause's value to doValue.
//  3. Prediction: topologically propagate through the DAG using the abducted
//     noise terms to obtain the counterfactual value of outcome.
func Counterfactual(ds *Dataset, m *StructuralModel, rowIndex int, cause string, doValue float64, outcome string) (CounterfactualResult, error) {
	n := ds.Rows()
	if rowIndex < 0 || rowIndex >= n {
		return CounterfactualResult{}, fmt.Errorf("causal: rowIndex %d out of range [0,%d)", rowIndex, n)
	}
	if _, ok := ds.Data[cause]; !ok {
		return CounterfactualResult{}, fmt.Errorf("causal: cause %q not in dataset", cause)
	}
	if _, ok := ds.Data[outcome]; !ok {
		return CounterfactualResult{}, fmt.Errorf("causal: outcome %q not in dataset", outcome)
	}

	// ── Step 1: Abduction ─────────────────────────────────────────────────
	// U_V = observed - predicted (using observed parent values at rowIndex).
	noise := make(map[string]float64, len(m.Variables))
	for _, v := range m.Variables {
		obs := ds.Data[v][rowIndex]
		pred := m.Intercept[v]
		for par, coef := range m.Coefs[v] {
			pred += coef * ds.Data[par][rowIndex]
		}
		noise[v] = obs - pred
	}

	// ── Step 2: Action ────────────────────────────────────────────────────
	// We record which variable is intervened on; downstream computation uses
	// doValue as the fixed value for cause.

	// ── Step 3: Prediction ────────────────────────────────────────────────
	// Topologically sort variables so parents are computed before children.
	order, err := topoSort(m)
	if err != nil {
		return CounterfactualResult{}, fmt.Errorf("causal: topological sort failed: %w", err)
	}

	cfVals := make(map[string]float64, len(m.Variables))
	for _, v := range order {
		if v == cause {
			cfVals[v] = doValue
			continue
		}
		val := m.Intercept[v] + noise[v]
		for par, coef := range m.Coefs[v] {
			val += coef * cfVals[par]
		}
		cfVals[v] = val
	}

	cfOutcome := cfVals[outcome]
	obsOutcome := ds.Data[outcome][rowIndex]

	return CounterfactualResult{
		RowIndex:              rowIndex,
		Cause:                 cause,
		Outcome:               outcome,
		ObservedCause:         ds.Data[cause][rowIndex],
		ObservedOutcome:       obsOutcome,
		DoValue:               doValue,
		CounterfactualOutcome: cfOutcome,
		Difference:            cfOutcome - obsOutcome,
	}, nil
}

// AverageCounterfactual computes the mean counterfactual outcome over all rows
// when intervening do(cause = doValue).
func AverageCounterfactual(ds *Dataset, m *StructuralModel, cause string, doValue float64, outcome string) (float64, error) {
	n := ds.Rows()
	if n == 0 {
		return 0, ErrInsufficientData
	}
	var sum float64
	for i := 0; i < n; i++ {
		res, err := Counterfactual(ds, m, i, cause, doValue, outcome)
		if err != nil {
			return 0, err
		}
		sum += res.CounterfactualOutcome
	}
	return sum / float64(n), nil
}

// topoSort returns a topological ordering of m.Variables using Kahn's algorithm.
// Edges are defined by m.Parents (parent → child).
func topoSort(m *StructuralModel) ([]string, error) {
	// Build in-degree and adjacency list.
	inDeg := make(map[string]int, len(m.Variables))
	children := make(map[string][]string, len(m.Variables))
	for _, v := range m.Variables {
		if _, ok := inDeg[v]; !ok {
			inDeg[v] = 0
		}
		for _, par := range m.Parents[v] {
			children[par] = append(children[par], v)
			inDeg[v]++
		}
	}

	// Collect roots (in-degree 0), sorted for determinism.
	queue := make([]string, 0, len(m.Variables))
	for _, v := range m.Variables {
		if inDeg[v] == 0 {
			queue = append(queue, v)
		}
	}
	sort.Strings(queue)

	order := make([]string, 0, len(m.Variables))
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		order = append(order, v)
		next := make([]string, 0, len(children[v]))
		for _, child := range children[v] {
			inDeg[child]--
			if inDeg[child] == 0 {
				next = append(next, child)
			}
		}
		sort.Strings(next)
		queue = append(queue, next...)
	}

	if len(order) != len(m.Variables) {
		return nil, fmt.Errorf("causal: cycle detected in structural model")
	}
	return order, nil
}
