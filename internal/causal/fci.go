package causal

// FCIConfig controls the FCI skeleton search and orientation.
type FCIConfig struct {
	Alpha          float64 // CI test significance threshold (default 0.05)
	MaxCondSetSize int     // maximum conditioning set size (default 3)
}

// DiscoverFCI runs the FCI algorithm over ds, producing a PAG that correctly
// represents the Markov equivalence class even when latent confounders exist.
//
// Procedure:
//  1. Skeleton phase — same conditional independence tests as PC.
//  2. Initialize PAG — all skeleton edges start as o-o (MarkCircle/MarkCircle).
//  3. R0 — orient v-structures: i *-o k o-* j, no edge i-j, k ∉ Sep(i,j) → i *-> k <-* j.
//  4. Rules R1-R3 iterated until stable.
func DiscoverFCI(ds *Dataset, cfg FCIConfig) (PAG, error) {
	if ds.Rows() < 10 {
		return PAG{}, ErrInsufficientData
	}
	if cfg.Alpha <= 0 {
		cfg.Alpha = 0.05
	}
	if cfg.MaxCondSetSize <= 0 {
		cfg.MaxCondSetSize = 3
	}

	nodes := ds.Names
	p := len(nodes)
	idx := make(map[string]int, p)
	for i, n := range nodes {
		idx[n] = i
	}

	// ── Phase 1: Skeleton (identical to PC) ──────────────────────────────────

	adj := make([][]bool, p)
	for i := range adj {
		adj[i] = make([]bool, p)
		for j := range adj[i] {
			adj[i][j] = i != j
		}
	}

	// sepSet[i*p+j] = conditioning set that d-separated i and j
	sepSet := make([][]string, p*p)

	for d := 0; d <= cfg.MaxCondSetSize; d++ {
		changed := false
		for i := 0; i < p; i++ {
			for j := i + 1; j < p; j++ {
				if !adj[i][j] {
					continue
				}
				nbrs := neighbors(adj, i, j, p)
				if len(nbrs) < d {
					continue
				}
				found := iterSubsets(nbrs, d, func(sub []int) bool {
					condNames := make([]string, len(sub))
					for k, si := range sub {
						condNames[k] = nodes[si]
					}
					if independentAt(ds, nodes[i], nodes[j], condNames, cfg.Alpha) {
						adj[i][j] = false
						adj[j][i] = false
						sepSet[i*p+j] = condNames
						sepSet[j*p+i] = condNames
						changed = true
						return true
					}
					return false
				})
				_ = found
			}
		}
		if !changed && d > 0 {
			break
		}
	}

	// ── Phase 2: Initialize PAG — all edges as o-o ───────────────────────────

	// pagMark[i][j] = the endpoint mark on the i-side of edge (i,j).
	// Only defined when adj[i][j] == true.
	// Initially all circles.
	pagMark := make([][]EdgeMark, p)
	for i := range pagMark {
		pagMark[i] = make([]EdgeMark, p)
		for j := range pagMark[i] {
			pagMark[i][j] = MarkCircle
		}
	}

	// ── Phase 3: R0 — orient v-structures / colliders ────────────────────────
	//
	// For every unshielded triple i - k - j (adj[i][k], adj[k][j], !adj[i][j]):
	//   if k ∉ Sep(i,j) → orient as i *-> k <-* j
	//   (set endpoint at k to MarkArrow on both edges i-k and j-k)

	for k := 0; k < p; k++ {
		nbrsK := neighborsAll(adj, k, p)
		for ai := 0; ai < len(nbrsK); ai++ {
			for bi := ai + 1; bi < len(nbrsK); bi++ {
				i, j := nbrsK[ai], nbrsK[bi]
				if adj[i][j] {
					continue // shielded triple — skip
				}
				ss := sepSet[i*p+j]
				if containsIdx(ss, nodes[k]) {
					continue // k in sep set — not a collider
				}
				// Orient arrowheads at k
				pagMark[i][k] = MarkArrow // endpoint on k-side of edge i-k
				pagMark[j][k] = MarkArrow // endpoint on k-side of edge j-k
			}
		}
	}

	// ── Phase 4: Orientation rules R1-R3, iterate until stable ───────────────

	for {
		changed := false

		// R1: α *-> β o-* γ, α and γ not adjacent → orient β --> γ
		// Conditions:
		//   pagMark[α][β] == MarkArrow  (arrowhead at β from α)
		//   pagMark[β][γ] == MarkCircle (circle at γ from β — undecided endpoint)
		//   pagMark[γ][β] == MarkCircle (circle at β from γ — β is not yet a collider on β-γ)
		//   !adj[α][γ]
		// The check pagMark[γ][β] != MarkArrow prevents R1 from firing when β is
		// already a collider on the α-β-γ triple (which would destroy the v-structure).
		for beta := 0; beta < p; beta++ {
			for alpha := 0; alpha < p; alpha++ {
				if !adj[alpha][beta] {
					continue
				}
				if pagMark[alpha][beta] != MarkArrow {
					continue
				}
				for gamma := 0; gamma < p; gamma++ {
					if !adj[beta][gamma] || gamma == alpha {
						continue
					}
					if adj[alpha][gamma] {
						continue // α and γ adjacent — R1 doesn't apply
					}
					// β-γ edge must be o-* at β side (circle at γ from β)
					if pagMark[beta][gamma] != MarkCircle {
						continue
					}
					// β must NOT already have an arrowhead from γ (would be a collider)
					if pagMark[gamma][beta] == MarkArrow {
						continue
					}
					// Orient β --> γ: tail at β on β-γ edge, arrowhead at γ
					pagMark[beta][gamma] = MarkArrow // arrowhead at γ from β
					pagMark[gamma][beta] = MarkTail  // tail at β on β-γ edge
					changed = true
				}
			}
		}

		// R2: α --> β *-> γ or α *-> β --> γ, and α *-o γ → orient α *-> γ
		// i.e. if there is a directed path through β with α-γ undetermined, sharpen γ endpoint
		for alpha := 0; alpha < p; alpha++ {
			for gamma := 0; gamma < p; gamma++ {
				if !adj[alpha][gamma] || alpha == gamma {
					continue
				}
				if pagMark[alpha][gamma] != MarkCircle {
					continue // α *-o γ required
				}
				// Look for a β that is adjacent to both α and γ
				for beta := 0; beta < p; beta++ {
					if beta == alpha || beta == gamma {
						continue
					}
					if !adj[alpha][beta] || !adj[beta][gamma] {
						continue
					}
					// Case 1: α --> β *-> γ
					// α --> β: tail at α (pagMark[beta][alpha]==MarkTail), arrow at β from α (pagMark[alpha][beta]==MarkArrow)
					// *-> γ: arrow at γ from β (pagMark[beta][gamma]==MarkArrow)
					case1 := pagMark[alpha][beta] == MarkArrow &&
						pagMark[beta][alpha] == MarkTail &&
						pagMark[beta][gamma] == MarkArrow
					// Case 2: α *-> β --> γ
					// *-> β: arrow at β from α (pagMark[alpha][beta]==MarkArrow)
					// --> γ: tail at β (pagMark[gamma][beta]==MarkTail), arrow at γ (pagMark[beta][gamma]==MarkArrow)
					case2 := pagMark[alpha][beta] == MarkArrow &&
						pagMark[beta][gamma] == MarkArrow &&
						pagMark[gamma][beta] == MarkTail
					if case1 || case2 {
						pagMark[alpha][gamma] = MarkArrow
						changed = true
					}
				}
			}
		}

		// R3: α *-> β <-* γ, α *-o δ o-* γ, δ *-o β, α and γ not adjacent → δ *-> β
		for beta := 0; beta < p; beta++ {
			for alpha := 0; alpha < p; alpha++ {
				if !adj[alpha][beta] {
					continue
				}
				if pagMark[alpha][beta] != MarkArrow {
					continue // need arrowhead at β from α
				}
				for gamma := 0; gamma < p; gamma++ {
					if gamma == alpha || !adj[gamma][beta] {
						continue
					}
					if pagMark[gamma][beta] != MarkArrow {
						continue // need arrowhead at β from γ
					}
					if adj[alpha][gamma] {
						continue // α and γ must not be adjacent
					}
					// Find δ adjacent to both α and γ (via circle endpoints) and to β
					for delta := 0; delta < p; delta++ {
						if delta == alpha || delta == gamma || delta == beta {
							continue
						}
						if !adj[alpha][delta] || !adj[gamma][delta] || !adj[delta][beta] {
							continue
						}
						// α *-o δ: circle at δ on α-δ edge
						if pagMark[alpha][delta] != MarkCircle {
							continue
						}
						// o-* γ: circle at δ on γ-δ edge
						if pagMark[gamma][delta] != MarkCircle {
							continue
						}
						// δ *-o β: circle at β on δ-β edge
						if pagMark[delta][beta] != MarkCircle {
							continue
						}
						// Orient δ *-> β (arrowhead at β from δ)
						pagMark[delta][beta] = MarkArrow
						changed = true
					}
				}
			}
		}

		if !changed {
			break
		}
	}

	// ── Phase 5: Build PAG from pagMark ──────────────────────────────────────
	//
	// For each undirected pair (i < j) with adj[i][j]:
	//   FromMark = pagMark[j][i]  (endpoint mark at i, stored as "j sees i as")
	//   ToMark   = pagMark[i][j]  (endpoint mark at j, stored as "i sees j as")
	//
	// Convention: pagMark[a][b] = the mark at b on the edge a-b.
	// So for edge i-j:
	//   mark at i = pagMark[j][i]
	//   mark at j = pagMark[i][j]

	var edges []PAGEdge
	for i := 0; i < p; i++ {
		for j := i + 1; j < p; j++ {
			if !adj[i][j] {
				continue
			}
			edges = append(edges, PAGEdge{
				From:     nodes[i],
				To:       nodes[j],
				FromMark: pagMark[j][i], // mark at i (the From node)
				ToMark:   pagMark[i][j], // mark at j (the To node)
			})
		}
	}

	return PAG{
		Nodes: nodes,
		Edges: edges,
	}, nil
}
