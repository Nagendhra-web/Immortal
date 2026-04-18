package causal

import (
	"fmt"
	"sort"
)

// LaggedParent describes a causal parent of a variable at a given time lag.
// Variable is the name of the parent variable; Lag is the number of time steps
// in the past (>=1); PartialR is the signed partial correlation after MCI conditioning.
type LaggedParent struct {
	Variable string
	Lag      int     // positive, >=1
	PartialR float64 // signed partial correlation; sign indicates effect direction
}

// LaggedGraph holds the result of PCMCI: for each outcome variable, the list
// of confirmed lagged causal parents after PC_1 and MCI phases.
type LaggedGraph struct {
	TauMax  int
	Parents map[string][]LaggedParent
}

// PCMCIConfig controls the PCMCI algorithm.
type PCMCIConfig struct {
	Alpha          float64 // CI significance level; default 0.05
	TauMax         int     // maximum lag to consider; default 5
	MaxCondSetSize int     // maximum conditioning set size in PC_1; default 3
}

// lagKey uniquely identifies a (variable, lag) candidate parent.
type lagKey struct {
	variable string
	lag      int
}

// DiscoverPCMCI runs the PCMCI algorithm on a time-ordered Dataset.
// Each column in ds must be a time-ordered sequence of observations (row i = time step i).
// The algorithm has two phases:
//  1. PC_1: for each target Y, iteratively remove candidate lagged parents
//     {X_{t-τ} : X ∈ vars, τ ∈ [1,TauMax]} via conditional independence tests.
//  2. MCI: re-test each remaining parent (X,τ) conditioning on pa(Y)\{(X,τ)} and pa(X_{t-τ}).
func DiscoverPCMCI(ds *Dataset, cfg PCMCIConfig) (LaggedGraph, error) {
	if ds.Rows() < 10 {
		return LaggedGraph{}, ErrInsufficientData
	}
	if cfg.Alpha <= 0 {
		cfg.Alpha = 0.05
	}
	if cfg.TauMax <= 0 {
		cfg.TauMax = 5
	}
	if cfg.MaxCondSetSize <= 0 {
		cfg.MaxCondSetSize = 3
	}

	vars := ds.Names
	T := ds.Rows()
	tauMax := cfg.TauMax

	// Require enough rows to form lagged pairs.
	if T <= tauMax {
		return LaggedGraph{}, fmt.Errorf("causal: need more than TauMax=%d rows, got %d", tauMax, T)
	}

	// Build a lagged dataset: for each (variable, lag), extract the aligned
	// subsequence. If variable V has data V[0..T-1], then V at lag τ
	// contributes rows T[τ..T-1] as the "past" values aligned with target rows [τ..T-1].
	// We use τ_max to align everything: all sequences are clipped to length T-tauMax,
	// i.e., target uses rows [tauMax..T-1], and (V,τ) uses rows [tauMax-τ..T-1-τ].
	tLen := T - tauMax // effective time-series length after alignment

	// laggedData[variable][lag] = slice of length tLen
	laggedData := make(map[string]map[int][]float64, len(vars))
	for _, v := range vars {
		laggedData[v] = make(map[int][]float64, tauMax)
		for tau := 1; tau <= tauMax; tau++ {
			start := tauMax - tau
			seq := make([]float64, tLen)
			copy(seq, ds.Data[v][start:start+tLen])
			laggedData[v][tau] = seq
		}
	}

	// targetData[v] = target contemporaneous values (rows [tauMax..T-1])
	targetData := make(map[string][]float64, len(vars))
	for _, v := range vars {
		seq := make([]float64, tLen)
		copy(seq, ds.Data[v][tauMax:tauMax+tLen])
		targetData[v] = seq
	}

	// Build a temporary Dataset for all lagged columns + contemporaneous targets.
	// Column names: "<var>__lag<τ>" for lagged, "<var>__t0" for targets.
	colName := func(v string, tau int) string {
		return fmt.Sprintf("%s__lag%d", v, tau)
	}
	targetColName := func(v string) string {
		return fmt.Sprintf("%s__t0", v)
	}

	allCols := make([]string, 0, len(vars)*(tauMax+1))
	for _, v := range vars {
		allCols = append(allCols, targetColName(v))
		for tau := 1; tau <= tauMax; tau++ {
			allCols = append(allCols, colName(v, tau))
		}
	}

	lagDS := NewDataset(allCols)
	row := make(map[string]float64, len(allCols))
	for i := 0; i < tLen; i++ {
		for _, v := range vars {
			row[targetColName(v)] = targetData[v][i]
			for tau := 1; tau <= tauMax; tau++ {
				row[colName(v, tau)] = laggedData[v][tau][i]
			}
		}
		if err := lagDS.Add(row); err != nil {
			return LaggedGraph{}, fmt.Errorf("causal: building lagged dataset: %w", err)
		}
	}

	// ─── Phase 1: PC_1 ───────────────────────────────────────────────────────
	// For each target Y, start with all (X,τ) as candidate parents, then
	// iteratively remove via conditional independence at increasing conditioning depths.

	// parents[Y] = set of remaining candidate (variable, lag) pairs.
	parents := make(map[string][]lagKey, len(vars))
	for _, y := range vars {
		candidates := make([]lagKey, 0, len(vars)*tauMax)
		for _, x := range vars {
			for tau := 1; tau <= tauMax; tau++ {
				candidates = append(candidates, lagKey{x, tau})
			}
		}
		parents[y] = candidates
	}

	for _, y := range vars {
		yCol := targetColName(y)
		for d := 0; d <= cfg.MaxCondSetSize; d++ {
			removed := true
			for removed {
				removed = false
				candidates := parents[y]
				newCandidates := make([]lagKey, 0, len(candidates))
				for _, cand := range candidates {
					xCol := colName(cand.variable, cand.lag)
					// Build conditioning set candidates: all parents except current.
					condPool := make([]string, 0, len(candidates)-1)
					for _, other := range candidates {
						if other == cand {
							continue
						}
						condPool = append(condPool, colName(other.variable, other.lag))
					}
					if len(condPool) < d {
						newCandidates = append(newCandidates, cand)
						continue
					}
					// Try subsets of size d from condPool.
					indep := iterSubsetsStr(condPool, d, func(sub []string) bool {
						return independentAt(lagDS, xCol, yCol, sub, cfg.Alpha)
					})
					if indep {
						removed = true
						// cand is removed (not added to newCandidates)
					} else {
						newCandidates = append(newCandidates, cand)
					}
				}
				parents[y] = newCandidates
				if d == 0 {
					break // depth-0 only needs one pass
				}
			}
		}
	}

	// ─── Phase 2: MCI ────────────────────────────────────────────────────────
	// For each remaining parent (X,τ) of Y, re-test
	//   X_{t-τ} ⊥ Y_t | pa(Y_t)\{(X,τ)}, pa(X_{t-τ})
	// where pa(X_{t-τ}) are the lagged parents of X found in Phase 1.

	result := make(map[string][]LaggedParent, len(vars))

	for _, y := range vars {
		yCol := targetColName(y)
		yParents := parents[y]
		var confirmed []LaggedParent

		for _, cand := range yParents {
			xCol := colName(cand.variable, cand.lag)

			// pa(Y_t) \ {(X,τ)}: other parents of Y.
			pyMinusCand := make([]string, 0, len(yParents)-1)
			for _, p := range yParents {
				if p == cand {
					continue
				}
				pyMinusCand = append(pyMinusCand, colName(p.variable, p.lag))
			}

			// pa(X_{t-τ}): parents of X (from X's own Phase-1 parent set).
			// These are the lagged-parents of X, each shifted by an additional τ.
			// In our aligned dataset we only have lags up to tauMax; we include
			// X's parents that are within tauMax from the original time frame.
			pxCols := make([]string, 0)
			for _, xp := range parents[cand.variable] {
				totalLag := xp.lag + cand.lag
				if totalLag <= tauMax {
					pxCols = append(pxCols, colName(xp.variable, totalLag))
				}
			}

			mciCond := append(pyMinusCand, pxCols...)

			// Limit conditioning set size to avoid rank deficiency.
			if len(mciCond) > cfg.MaxCondSetSize+2 {
				mciCond = mciCond[:cfg.MaxCondSetSize+2]
			}

			indep := independentAt(lagDS, xCol, yCol, mciCond, cfg.Alpha)
			if indep {
				continue // remove this parent
			}

			// Compute signed partial correlation for this confirmed parent.
			pr, ok := partialCorr(lagDS, xCol, yCol, mciCond)
			if !ok {
				pr = 0
			}
			confirmed = append(confirmed, LaggedParent{
				Variable: cand.variable,
				Lag:      cand.lag,
				PartialR: pr,
			})
		}

		// Sort by lag, then variable name for determinism.
		sort.Slice(confirmed, func(i, j int) bool {
			if confirmed[i].Lag != confirmed[j].Lag {
				return confirmed[i].Lag < confirmed[j].Lag
			}
			return confirmed[i].Variable < confirmed[j].Variable
		})
		result[y] = confirmed
	}

	return LaggedGraph{
		TauMax:  tauMax,
		Parents: result,
	}, nil
}

// iterSubsetsStr calls fn on each subset of elems of size k (string version).
// Returns true if fn returned true (early exit).
func iterSubsetsStr(elems []string, k int, fn func([]string) bool) bool {
	if k == 0 {
		return fn([]string{})
	}
	n := len(elems)
	if n < k {
		return false
	}
	sub := make([]string, k)
	var recurse func(start, depth int) bool
	recurse = func(start, depth int) bool {
		if depth == k {
			return fn(sub)
		}
		for i := start; i <= n-k+depth; i++ {
			sub[depth] = elems[i]
			if recurse(i+1, depth+1) {
				return true
			}
		}
		return false
	}
	return recurse(0, 0)
}

// LaggedEffect holds the estimated total causal effect of cause at a given lag on outcome.
type LaggedEffect struct {
	Cause       string
	Outcome     string
	Lag         int
	Coefficient float64
	Adjustment  []LaggedParent
}

// EstimateLaggedACE estimates the average causal effect of cause at the given
// lag on outcome by regressing outcome_t on cause_{t-lag} controlling for
// all other lagged parents of outcome found in the LaggedGraph.
func EstimateLaggedACE(ds *Dataset, g LaggedGraph, cause, outcome string, lag int) (LaggedEffect, error) {
	if cause == outcome && lag == 0 {
		return LaggedEffect{}, fmt.Errorf("causal: cause and outcome are the same variable at lag 0")
	}
	T := ds.Rows()
	tauMax := g.TauMax
	if tauMax <= 0 {
		tauMax = 5
	}
	if T <= tauMax {
		return LaggedEffect{}, fmt.Errorf("causal: need more than TauMax=%d rows, got %d", tauMax, T)
	}
	if lag < 1 || lag > tauMax {
		return LaggedEffect{}, fmt.Errorf("causal: lag %d out of range [1, %d]", lag, tauMax)
	}

	tLen := T - tauMax

	// outcome target: rows [tauMax..T-1]
	yVals := ds.Data[outcome][tauMax : tauMax+tLen]

	// cause at requested lag: rows [tauMax-lag..T-1-lag]
	causeStart := tauMax - lag
	causeVals := ds.Data[cause][causeStart : causeStart+tLen]

	// adjustment = other parents of outcome in the graph.
	outParents := g.Parents[outcome]
	var adjustment []LaggedParent
	for _, p := range outParents {
		if p.Variable == cause && p.Lag == lag {
			continue
		}
		adjustment = append(adjustment, p)
	}

	// Build design matrix: [cause_{t-lag}, adjustment...]
	p := 1 + len(adjustment)
	X := NewMatrix(tLen, p)
	for i := 0; i < tLen; i++ {
		X.Set(i, 0, causeVals[i])
	}
	for ci, adj := range adjustment {
		adjStart := tauMax - adj.Lag
		adjVals := ds.Data[adj.Variable][adjStart : adjStart+tLen]
		for i := 0; i < tLen; i++ {
			X.Set(i, ci+1, adjVals[i])
		}
	}

	beta, err := OLS(yVals, X)
	if err != nil {
		return LaggedEffect{}, fmt.Errorf("causal: OLS for LaggedACE failed: %w", err)
	}
	// beta[0]=intercept, beta[1]=cause coefficient
	return LaggedEffect{
		Cause:       cause,
		Outcome:     outcome,
		Lag:         lag,
		Coefficient: beta[1],
		Adjustment:  adjustment,
	}, nil
}
