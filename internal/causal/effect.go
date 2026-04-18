package causal

import (
	"fmt"
	"math"
	"sort"
)

// Effect holds the result of an average causal effect (ACE) estimation.
type Effect struct {
	Treatment     string
	Outcome       string
	Coefficient   float64  // ACE in the linear model
	AdjustmentSet []string // variables used to block back-door paths
	Confounders   []string // same as AdjustmentSet for linear Gaussian models
	Pseudocode    string   // human-readable explanation
}

// EstimateEffect estimates the ACE of do(treatment) on outcome using the
// back-door adjustment criterion. The adjustment set is the set of parents of
// treatment in the directed subgraph (sufficient for back-door in linear models).
func EstimateEffect(ds *Dataset, g CausalGraph, treatment, outcome string) (Effect, error) {
	if treatment == outcome {
		return Effect{}, fmt.Errorf("treatment and outcome must differ")
	}
	n := ds.Rows()
	if n < 5 {
		return Effect{}, ErrInsufficientData
	}

	// Adjustment set = parents of treatment (directed edges pointing to treatment).
	adj := parentsOf(g, treatment)
	sort.Strings(adj)

	// Build design matrix: columns = [treatment, adjustment vars...]
	p := 1 + len(adj)
	X := NewMatrix(n, p)
	tx := ds.Data[treatment]
	for i := 0; i < n; i++ {
		X.Set(i, 0, tx[i])
	}
	for ci, av := range adj {
		col := ds.Data[av]
		for i := 0; i < n; i++ {
			X.Set(i, ci+1, col[i])
		}
	}

	y := ds.Data[outcome]
	beta, err := OLS(y, X)
	if err != nil {
		return Effect{}, fmt.Errorf("OLS failed: %w", err)
	}
	// beta[0] = intercept, beta[1] = treatment coefficient
	ace := beta[1]

	pseudo := fmt.Sprintf(
		"Regress %q on [%q + adjustment set %v]; ACE = %.4f",
		outcome, treatment, adj, ace,
	)

	return Effect{
		Treatment:     treatment,
		Outcome:       outcome,
		Coefficient:   ace,
		AdjustmentSet: adj,
		Confounders:   adj,
		Pseudocode:    pseudo,
	}, nil
}

// parentsOf returns all nodes with a directed edge into node.
func parentsOf(g CausalGraph, node string) []string {
	var parents []string
	for src, targets := range g.Directed {
		for _, t := range targets {
			if t == node {
				parents = append(parents, src)
				break
			}
		}
	}
	return parents
}

// RootCauseResult ranks upstream variables by absolute ACE on the outcome.
type RootCauseResult struct {
	Outcome string
	Ranked  []RankedVar
}

// RankedVar is one entry in the ranked list.
type RankedVar struct {
	Variable string
	ACE      float64
}

// RootCause scans all ancestors of outcome, estimates each ancestor's ACE on
// outcome, and returns them ranked by descending |ACE|.
func RootCause(ds *Dataset, g CausalGraph, outcome string) (RootCauseResult, error) {
	ancestors := g.Ancestors(outcome)
	if len(ancestors) == 0 {
		return RootCauseResult{Outcome: outcome}, nil
	}

	ranked := make([]RankedVar, 0, len(ancestors))
	for _, anc := range ancestors {
		eff, err := EstimateEffect(ds, g, anc, outcome)
		if err != nil {
			// skip variables where estimation fails (e.g. collinearity)
			continue
		}
		ranked = append(ranked, RankedVar{Variable: anc, ACE: eff.Coefficient})
	}

	sort.Slice(ranked, func(i, j int) bool {
		return math.Abs(ranked[i].ACE) > math.Abs(ranked[j].ACE)
	})

	return RootCauseResult{Outcome: outcome, Ranked: ranked}, nil
}
