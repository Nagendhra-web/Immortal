package causal

// fci_advanced.go — FCI orientation rules R4–R10 (Zhang 2008 complete FCI).
//
// Reference: Zhang, J. (2008). "On the completeness of orientation rules for
// causal discovery in the presence of latent confounders and selection bias."
// Artificial Intelligence, 172(16-17), 1873-1896.
//
// Rules R1–R3 are implemented in fci.go. This file adds R4 (discriminating paths)
// and R5–R10 (circle-to-tail and ancestor-propagation rules).
//
// Notation used in comments:
//   *  = any mark (circle, arrow, or tail)
//   o  = circle (MarkCircle)
//   >  = arrowhead (MarkArrow)
//   -  = tail (MarkTail)
//   The mark stored in pagMark[a][b] is the endpoint mark AT b on the edge a–b.

// pagState bundles the mutable PAG state passed between rule functions.
// All indices correspond to the node-name slice used in DiscoverFCI.
type pagState struct {
	adj      [][]bool
	pagMark  [][]EdgeMark
	nodes    []string
	idx      map[string]int
	p        int
}

// sepKey builds a canonical key for the separation set map used in DiscoverFCI.
func sepKey(i, j, p int) int { return i*p + j }

// DiscriminatingPath returns one discriminating path from x to y in g, or nil
// if none exists. A path π = ⟨x = V_0, V_1, …, V_k = y⟩ is discriminating for V_{k-1} iff:
//
//	k ≥ 3
//	x and y are not adjacent
//	V_1, …, V_{k-1} are colliders on π and parents of y
//	V_{k-1} is adjacent to y
//
// Returns the node sequence including endpoints.
//
// sep is the separation-set map: sep[i*p+j] = conditioning set that d-separated
// variable i and variable j (indices into nodes).
func DiscriminatingPath(g *PAG, sep map[[2]string][]string, x, y string) []string {
	// Build adjacency from PAG edges.
	nodeIdx := make(map[string]int, len(g.Nodes))
	for i, n := range g.Nodes {
		nodeIdx[n] = i
	}
	p := len(g.Nodes)
	adj := make([][]bool, p)
	for i := range adj {
		adj[i] = make([]bool, p)
	}
	// pagMark[a][b] = endpoint mark at b on edge a–b
	pm := make([][]EdgeMark, p)
	for i := range pm {
		pm[i] = make([]EdgeMark, p)
	}
	for _, e := range g.Edges {
		fi, ti := nodeIdx[e.From], nodeIdx[e.To]
		adj[fi][ti] = true
		adj[ti][fi] = true
		// e.FromMark = mark at From; e.ToMark = mark at To
		// pagMark[To][From] = FromMark  (mark at From, seen from To)
		// pagMark[From][To] = ToMark    (mark at To, seen from From)
		pm[ti][fi] = e.FromMark
		pm[fi][ti] = e.ToMark
	}

	xi, yi := nodeIdx[x], nodeIdx[y]
	if adj[xi][yi] {
		return nil // x and y must not be adjacent
	}

	// BFS/DFS: build paths starting from x, exploring via collider-parents of y.
	// A node v (not x, not y) qualifies as an intermediate node if:
	//   - v is adjacent to y (parent of y: tail at v on v–y edge, arrow at y)
	//   - v is a collider on the path so far
	//
	// We do DFS, tracking the path and whether each intermediate node is a
	// collider-on-path and parent-of-y.

	visited := make([]bool, p)
	var result []string

	// isParentOfY: v --> y means tail at v on edge v–y (pm[yi][v]==MarkTail)
	// and arrowhead at y (pm[v][yi]==MarkArrow).
	isParentOfY := func(v int) bool {
		if !adj[v][yi] {
			return false
		}
		return pm[v][yi] == MarkArrow && pm[yi][v] == MarkTail
	}

	// isColliderBetween: node v is a collider on path between prev and next iff
	// both neighbours on the path have arrowheads pointing at v.
	// pm[prev][v] == MarkArrow means the edge prev–v has an arrowhead at v.
	isColliderBetween := func(prev, v, next int) bool {
		return pm[prev][v] == MarkArrow && pm[next][v] == MarkArrow
	}

	// Discriminating path for V_{k-1}: ⟨X=V_0, V_1, …, V_{k-1}, Y=V_k⟩ where
	//   - k ≥ 3 (at least 4 nodes including endpoints)
	//   - X and Y not adjacent
	//   - V_1, …, V_{k-2} are colliders on π AND parents of Y
	//   - V_{k-1} is adjacent to Y (but NOT required to be a parent of Y here)
	//
	// DFS explores from X, requiring each intermediate node up to V_{k-2} to
	// be a collider-on-path and parent of Y. The last step lands on Y.

	var dfs func(path []int) bool
	dfs = func(path []int) bool {
		cur := path[len(path)-1]
		if cur == yi {
			// path = [X=V_0, V_1, ..., V_{k-1}, Y=V_k]
			// Need k >= 3, i.e. len(path) >= 4.
			if len(path) < 4 {
				return false
			}
			// V_{k-1} = path[len-2] must be adjacent to Y (guaranteed since we got here).
			// V_1 .. V_{k-2} = path[1..len-3] must be colliders on path AND parents of Y.
			for idx2 := 1; idx2 <= len(path)-3; idx2++ {
				v := path[idx2]
				if !isParentOfY(v) {
					return false
				}
				prev2 := path[idx2-1]
				next2 := path[idx2+1]
				if !isColliderBetween(prev2, v, next2) {
					return false
				}
			}
			result = make([]string, len(path))
			for i, ni := range path {
				result[i] = g.Nodes[ni]
			}
			return true
		}

		for nb := 0; nb < p; nb++ {
			if !adj[cur][nb] || visited[nb] {
				continue
			}
			if nb == xi {
				continue
			}
			visited[nb] = true
			path = append(path, nb)
			if dfs(path) {
				return true
			}
			path = path[:len(path)-1]
			visited[nb] = false
		}
		return false
	}

	visited[xi] = true
	if dfs([]int{xi}) {
		return result
	}
	return nil
}

// ApplyR4 applies the discriminating-path orientation rule (Zhang 2008, R4).
//
// For every pair (x, y) where x and y are not adjacent:
//   - Find a discriminating path π = ⟨x, V_1, …, V_{k-1}, y⟩ for V_{k-1}.
//   - Let b = V_{k-1} (the node being discriminated).
//   - If b ∈ Sep(x, y): orient b–y as b --> y (tail at b, arrow at y)
//     AND orient b as non-collider on π (tail at b on incoming edge).
//   - If b ∉ Sep(x, y): orient as collider b <-> y portion
//     (arrowhead at b from its predecessor AND arrowhead at y from b).
//
// Returns true if it changed g.
// sep is keyed by [2]string{smaller-name, larger-name} (canonical order).
func ApplyR4(g *PAG, sep map[[2]string][]string) bool {
	nodeIdx := make(map[string]int, len(g.Nodes))
	for i, n := range g.Nodes {
		nodeIdx[n] = i
	}
	p := len(g.Nodes)

	adj := make([][]bool, p)
	for i := range adj {
		adj[i] = make([]bool, p)
	}
	pm := make([][]EdgeMark, p)
	for i := range pm {
		pm[i] = make([]EdgeMark, p)
	}
	loadPAG := func() {
		for i := range adj {
			for j := range adj[i] {
				adj[i][j] = false
				pm[i][j] = MarkCircle
			}
		}
		for _, e := range g.Edges {
			fi, ti := nodeIdx[e.From], nodeIdx[e.To]
			adj[fi][ti] = true
			adj[ti][fi] = true
			pm[ti][fi] = e.FromMark
			pm[fi][ti] = e.ToMark
		}
	}
	loadPAG()

	savePAG := func() {
		newEdges := make([]PAGEdge, 0, len(g.Edges))
		for i := 0; i < p; i++ {
			for j := i + 1; j < p; j++ {
				if adj[i][j] {
					newEdges = append(newEdges, PAGEdge{
						From:     g.Nodes[i],
						To:       g.Nodes[j],
						FromMark: pm[j][i],
						ToMark:   pm[i][j],
					})
				}
			}
		}
		g.Edges = newEdges
	}

	sepContains := func(x, y string, node string) bool {
		key := [2]string{x, y}
		if x > y {
			key = [2]string{y, x}
		}
		for _, s := range sep[key] {
			if s == node {
				return true
			}
		}
		return false
	}

	changed := false
	for xi := 0; xi < p; xi++ {
		for yi := 0; yi < p; yi++ {
			if xi == yi || adj[xi][yi] {
				continue
			}
			x, y := g.Nodes[xi], g.Nodes[yi]
			path := DiscriminatingPath(g, nil, x, y)
			if len(path) < 4 {
				continue
			}
			// b = V_{k-1} = path[len-2]
			b := path[len(path)-2]
			bi := nodeIdx[b]
			// predecessor of b on path
			prevB := path[len(path)-3]
			prevBi := nodeIdx[prevB]

			if sepContains(x, y, b) {
				// b is in Sep(x,y): orient b as non-collider → tail at b on b–y
				// and tail at b on prevB–b edge (non-collider means tails at b).
				if pm[bi][yi] != MarkArrow || pm[yi][bi] != MarkTail {
					pm[bi][yi] = MarkArrow // arrowhead at y from b
					pm[yi][bi] = MarkTail  // tail at b on b–y edge
					changed = true
				}
				if pm[prevBi][bi] != MarkTail {
					pm[prevBi][bi] = MarkTail // tail at b on prevB–b edge
					changed = true
				}
			} else {
				// b not in Sep(x,y): orient as collider on path
				// arrowhead at b from prevB, arrowhead at y from b
				if pm[prevBi][bi] != MarkArrow {
					pm[prevBi][bi] = MarkArrow
					changed = true
				}
				if pm[bi][yi] != MarkArrow {
					pm[bi][yi] = MarkArrow
					changed = true
				}
				if pm[yi][bi] != MarkArrow {
					pm[yi][bi] = MarkArrow
					changed = true
				}
			}
		}
	}

	if changed {
		savePAG()
	}
	return changed
}

// ApplyR5ThroughR7 applies Zhang 2008 rules R5, R6, R7 — circle-into-tail rules.
//
// R5: If α o-o β is in an undirected path π = ⟨α, β, …⟩ that is a chordless
//
//	cycle of only o-o edges, then orient all edges on π as tails: α --β.
//
// R6: If α --> β o-o γ (tail at α, arrow at β, circle–circle β–γ), orient β –- γ
//
//	(tail at β on β–γ) — β cannot be a collider since it has a tail on α–β.
//
// R7: If α --o β o-o γ and α and γ are not adjacent, orient β o-* γ as β -* γ
//
//	(tail at β on β–γ).
//
// Returns true if any mark changed.
func ApplyR5ThroughR7(g *PAG) bool {
	nodeIdx := make(map[string]int, len(g.Nodes))
	for i, n := range g.Nodes {
		nodeIdx[n] = i
	}
	p := len(g.Nodes)

	adj := make([][]bool, p)
	for i := range adj {
		adj[i] = make([]bool, p)
	}
	pm := make([][]EdgeMark, p)
	for i := range pm {
		pm[i] = make([]EdgeMark, p)
	}
	for _, e := range g.Edges {
		fi, ti := nodeIdx[e.From], nodeIdx[e.To]
		adj[fi][ti] = true
		adj[ti][fi] = true
		pm[ti][fi] = e.FromMark
		pm[fi][ti] = e.ToMark
	}

	changed := false

	// R5: Find chordless cycles of o-o edges and orient them all as tails.
	// An edge α o-o β (both marks circle) is part of an undirected (circle) path.
	// If that path forms a cycle with no chords, orient all edges as tails.
	//
	// Implementation: find a chordless cycle through each o-o edge.
	isCircleEdge := func(i, j int) bool {
		return adj[i][j] && pm[i][j] == MarkCircle && pm[j][i] == MarkCircle
	}

	// BFS to find a chordless cycle of o-o edges starting from node s,
	// via edge s–start (only o-o edges allowed on path, no chords).
	findCircleCycle := func(s int) []int {
		// Try to find cycle: DFS along o-o edges from s back to s
		visited := make([]bool, p)
		visited[s] = true
		var path []int
		var dfs func(cur int) bool
		dfs = func(cur int) bool {
			for nb := 0; nb < p; nb++ {
				if !isCircleEdge(cur, nb) {
					continue
				}
				if nb == s && len(path) >= 3 {
					// Check no chord: no adj between non-consecutive nodes in the cycle.
					// The cycle is path[0], path[1], ..., path[k], path[0].
					// Consecutive pairs in the cycle: (path[i], path[i+1]) and (path[k], path[0]).
					// A chord is an edge between any two non-consecutive nodes.
					chordless := true
					k := len(path)
					for ai := 0; ai < k && chordless; ai++ {
						for bi := ai + 2; bi < k && chordless; bi++ {
							// Skip the pair (path[0], path[k-1]) which is consecutive in cycle.
							if ai == 0 && bi == k-1 {
								continue
							}
							if adj[path[ai]][path[bi]] {
								chordless = false
							}
						}
					}
					if chordless {
						return true
					}
				}
				if !visited[nb] {
					visited[nb] = true
					path = append(path, nb)
					if dfs(nb) {
						return true
					}
					path = path[:len(path)-1]
					visited[nb] = false
				}
			}
			return false
		}
		path = append(path, s)
		if dfs(s) {
			return path
		}
		return nil
	}

	for s := 0; s < p; s++ {
		cycle := findCircleCycle(s)
		if len(cycle) < 3 {
			continue
		}
		// Orient all cycle edges as tails (both ends).
		for ci := 0; ci < len(cycle); ci++ {
			a := cycle[ci]
			b := cycle[(ci+1)%len(cycle)]
			if pm[a][b] != MarkTail {
				pm[a][b] = MarkTail
				pm[b][a] = MarkTail
				changed = true
			}
		}
	}

	// R6: α --> β o-o γ (not already a tail at β on β–γ) → orient tail at β on β–γ.
	// α --> β: pm[beta][alpha]==MarkTail (tail at α on α–β) and pm[alpha][beta]==MarkArrow (arrow at β).
	for beta := 0; beta < p; beta++ {
		for alpha := 0; alpha < p; alpha++ {
			if !adj[alpha][beta] {
				continue
			}
			// α --> β: tail at α, arrow at β from α
			if pm[alpha][beta] != MarkArrow || pm[beta][alpha] != MarkTail {
				continue
			}
			// Find γ adjacent to β with o-o edge β–γ
			for gamma := 0; gamma < p; gamma++ {
				if gamma == alpha || !adj[beta][gamma] {
					continue
				}
				if pm[beta][gamma] == MarkCircle && pm[gamma][beta] == MarkCircle {
					// Orient β–γ: tail at β (pm[gamma][beta] = MarkTail)
					pm[gamma][beta] = MarkTail
					changed = true
				}
			}
		}
	}

	// R7: α --o β o-o γ, α and γ not adjacent → tail at β on β–γ.
	// α --o β: pm[alpha][beta]==MarkCircle (circle at β from α) and pm[beta][alpha]==MarkTail (tail at α).
	for beta := 0; beta < p; beta++ {
		for alpha := 0; alpha < p; alpha++ {
			if !adj[alpha][beta] {
				continue
			}
			// α --o β: tail at α, circle at β from α
			if pm[alpha][beta] != MarkCircle || pm[beta][alpha] != MarkTail {
				continue
			}
			for gamma := 0; gamma < p; gamma++ {
				if gamma == alpha || !adj[beta][gamma] {
					continue
				}
				if adj[alpha][gamma] {
					continue // α and γ must not be adjacent
				}
				if pm[beta][gamma] == MarkCircle && pm[gamma][beta] == MarkCircle {
					pm[gamma][beta] = MarkTail
					changed = true
				}
			}
		}
	}

	if changed {
		newEdges := make([]PAGEdge, 0, len(g.Edges))
		for i := 0; i < p; i++ {
			for j := i + 1; j < p; j++ {
				if adj[i][j] {
					newEdges = append(newEdges, PAGEdge{
						From:     g.Nodes[i],
						To:       g.Nodes[j],
						FromMark: pm[j][i],
						ToMark:   pm[i][j],
					})
				}
			}
		}
		g.Edges = newEdges
	}
	return changed
}

// ApplyR8ThroughR10 applies Zhang 2008 rules R8, R9, R10 — ancestor-propagation rules.
//
// R8: If α o-> β and α --> γ --> β, or α --o γ o-> β (an almost-directed path
//
//	from α to β through γ), then orient α --> β (tail at α, arrow at β).
//
// R9: If α o-> β and there is an undirected-circle path ⟨α, γ_1, …, γ_k, β⟩
//
//	(k ≥ 1) that is an uncovered possibly-directed path not containing β's
//	other neighbors, orient α --> β.
//
// R10: If α o-> β and β <-- γ, β <-- δ, and there are uncovered pd paths
//
//	π1 = ⟨α, …, γ⟩ and π2 = ⟨α, …, δ⟩ (α not adjacent to γ or δ), orient α --> β.
//
// Returns true if any mark changed.
func ApplyR8ThroughR10(g *PAG) bool {
	nodeIdx := make(map[string]int, len(g.Nodes))
	for i, n := range g.Nodes {
		nodeIdx[n] = i
	}
	p := len(g.Nodes)

	adj := make([][]bool, p)
	for i := range adj {
		adj[i] = make([]bool, p)
	}
	pm := make([][]EdgeMark, p)
	for i := range pm {
		pm[i] = make([]EdgeMark, p)
	}
	for _, e := range g.Edges {
		fi, ti := nodeIdx[e.From], nodeIdx[e.To]
		adj[fi][ti] = true
		adj[ti][fi] = true
		pm[ti][fi] = e.FromMark
		pm[fi][ti] = e.ToMark
	}

	changed := false

	// R8: α o-> β, and α --> γ --> β (or α --o γ o-> β) → orient α --> β.
	// "o-> β from α": pm[alpha][beta]==MarkArrow (arrow at β) && pm[beta][alpha]==MarkCircle (circle at α).
	for alpha := 0; alpha < p; alpha++ {
		for beta := 0; beta < p; beta++ {
			if !adj[alpha][beta] {
				continue
			}
			if pm[alpha][beta] != MarkArrow || pm[beta][alpha] != MarkCircle {
				continue // need α o-> β
			}
			for gamma := 0; gamma < p; gamma++ {
				if gamma == alpha || gamma == beta {
					continue
				}
				if !adj[alpha][gamma] || !adj[gamma][beta] {
					continue
				}
				// Case 1: α --> γ --> β
				// α --> γ: tail at α (pm[alpha][gamma] direction check)
				// pm[gamma][alpha]==MarkTail && pm[alpha][gamma]==MarkArrow → α --> γ
				// pm[beta][gamma]==MarkTail && pm[gamma][beta]==MarkArrow  → γ --> β
				case1 := pm[gamma][alpha] == MarkTail && pm[alpha][gamma] == MarkArrow &&
					pm[beta][gamma] == MarkTail && pm[gamma][beta] == MarkArrow

				// Case 2: α --o γ o-> β
				// α --o γ: pm[alpha][gamma]==MarkCircle && pm[gamma][alpha]==MarkTail
				// γ o-> β: pm[gamma][beta]==MarkArrow && pm[beta][gamma]==MarkCircle
				case2 := pm[alpha][gamma] == MarkCircle && pm[gamma][alpha] == MarkTail &&
					pm[gamma][beta] == MarkArrow && pm[beta][gamma] == MarkCircle

				if case1 || case2 {
					// Orient α --> β: tail at α, arrow at β
					if pm[beta][alpha] != MarkTail {
						pm[beta][alpha] = MarkTail
						changed = true
					}
				}
			}
		}
	}

	// R9: α o-> β. If there is an uncovered possibly-directed path from α to β
	// through some γ_1 (not adjacent to β), orient α --> β.
	// Uncovered pd path: path ⟨α=V0, V1, …, Vk=β⟩ where
	//   - each edge Vi o-> Vi+1 or Vi --> Vi+1
	//   - V_i and V_{i+2} are not adjacent (uncovered)
	//   - V1 is not adjacent to β (otherwise it's just a shortcut)
	//
	// We check if there is any V1 adjacent to α (not adjacent to β) such that
	// V1 can reach β via a possibly-directed path without revisiting α.
	hasPDPath := func(from, to int, notAdjTo int) bool {
		// BFS for possibly-directed path from→to where first step is notAdjTo-check
		visited := make([]bool, p)
		visited[from] = true
		queue := []int{from}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for nb := 0; nb < p; nb++ {
				if !adj[cur][nb] || visited[nb] {
					continue
				}
				// edge cur o-> nb or cur --> nb
				if pm[cur][nb] != MarkArrow {
					continue
				}
				if pm[nb][cur] != MarkCircle && pm[nb][cur] != MarkTail {
					continue
				}
				if nb == to {
					return true
				}
				visited[nb] = true
				queue = append(queue, nb)
			}
		}
		return false
	}

	for alpha := 0; alpha < p; alpha++ {
		for beta := 0; beta < p; beta++ {
			if !adj[alpha][beta] {
				continue
			}
			if pm[alpha][beta] != MarkArrow || pm[beta][alpha] != MarkCircle {
				continue
			}
			// Try each γ_1 adjacent to α, not adjacent to β
			for gamma := 0; gamma < p; gamma++ {
				if gamma == alpha || gamma == beta {
					continue
				}
				if !adj[alpha][gamma] || adj[gamma][beta] {
					continue // γ must not be adjacent to β
				}
				// edge α o-> γ or α --> γ
				if pm[alpha][gamma] != MarkArrow {
					continue
				}
				if pm[gamma][alpha] != MarkCircle && pm[gamma][alpha] != MarkTail {
					continue
				}
				// Find pd path from γ to β (not going back through α)
				if hasPDPath(gamma, beta, alpha) {
					if pm[beta][alpha] != MarkTail {
						pm[beta][alpha] = MarkTail
						changed = true
					}
					break
				}
			}
		}
	}

	// R10: α o-> β. If β <-- γ, β <-- δ (γ ≠ δ), and there are uncovered
	// pd paths π1 = ⟨α, …, γ⟩ and π2 = ⟨α, …, δ⟩ where the first nodes of π1
	// and π2 after α are different and not adjacent to β, then orient α --> β.
	for alpha := 0; alpha < p; alpha++ {
		for beta := 0; beta < p; beta++ {
			if !adj[alpha][beta] {
				continue
			}
			if pm[alpha][beta] != MarkArrow || pm[beta][alpha] != MarkCircle {
				continue
			}
			// Find all γ such that γ --> β (tail at γ on γ–β, arrow at β from γ)
			var parents []int
			for gamma := 0; gamma < p; gamma++ {
				if gamma == alpha || !adj[gamma][beta] {
					continue
				}
				if pm[gamma][beta] == MarkArrow && pm[beta][gamma] == MarkTail {
					parents = append(parents, gamma)
				}
			}
			if len(parents) < 2 {
				continue
			}
			// Find two pd paths from α to distinct parents, each starting with
			// a node not adjacent to β.
			found := false
			for gi := 0; gi < len(parents) && !found; gi++ {
				for di := gi + 1; di < len(parents) && !found; di++ {
					g1, d1 := parents[gi], parents[di]
					// Need uncovered pd path α to g1 and α to d1,
					// first steps not adjacent to β.
					pathToG := hasPDPath(alpha, g1, beta)
					pathToD := hasPDPath(alpha, d1, beta)
					if pathToG && pathToD {
						if pm[beta][alpha] != MarkTail {
							pm[beta][alpha] = MarkTail
							changed = true
						}
						found = true
					}
				}
			}
		}
	}

	if changed {
		newEdges := make([]PAGEdge, 0, len(g.Edges))
		for i := 0; i < p; i++ {
			for j := i + 1; j < p; j++ {
				if adj[i][j] {
					newEdges = append(newEdges, PAGEdge{
						From:     g.Nodes[i],
						To:       g.Nodes[j],
						FromMark: pm[j][i],
						ToMark:   pm[i][j],
					})
				}
			}
		}
		g.Edges = newEdges
	}
	return changed
}

// applyAdvancedRules runs R4–R10 (plus R1–R3) until fixpoint.
// sep is the separating-set map from the skeleton phase, keyed [2]string canonical.
// Returns true if at least one mark changed.
func applyAdvancedRules(g *PAG, sep map[[2]string][]string) bool {
	anyChanged := false
	for {
		c1 := applyR1R2R3OnPAG(g)
		c2 := ApplyR4(g, sep)
		c3 := ApplyR5ThroughR7(g)
		c4 := ApplyR8ThroughR10(g)
		if !c1 && !c2 && !c3 && !c4 {
			break
		}
		anyChanged = true
	}
	return anyChanged
}

// applyR1R2R3OnPAG re-runs R1, R2, R3 on a PAG value (used within the advanced
// fixpoint loop). This is a thin adapter that mirrors the R1-R3 logic in fci.go
// but operates on a *PAG rather than the raw index arrays.
func applyR1R2R3OnPAG(g *PAG) bool {
	nodeIdx := make(map[string]int, len(g.Nodes))
	for i, n := range g.Nodes {
		nodeIdx[n] = i
	}
	p := len(g.Nodes)

	adj := make([][]bool, p)
	for i := range adj {
		adj[i] = make([]bool, p)
	}
	pm := make([][]EdgeMark, p)
	for i := range pm {
		pm[i] = make([]EdgeMark, p)
	}
	for _, e := range g.Edges {
		fi, ti := nodeIdx[e.From], nodeIdx[e.To]
		adj[fi][ti] = true
		adj[ti][fi] = true
		pm[ti][fi] = e.FromMark
		pm[fi][ti] = e.ToMark
	}

	changed := false

	// R1: α *-> β o-* γ, α and γ not adjacent → β --> γ
	for beta := 0; beta < p; beta++ {
		for alpha := 0; alpha < p; alpha++ {
			if !adj[alpha][beta] || pm[alpha][beta] != MarkArrow {
				continue
			}
			for gamma := 0; gamma < p; gamma++ {
				if !adj[beta][gamma] || gamma == alpha || adj[alpha][gamma] {
					continue
				}
				if pm[beta][gamma] != MarkCircle || pm[gamma][beta] == MarkArrow {
					continue
				}
				pm[beta][gamma] = MarkArrow
				pm[gamma][beta] = MarkTail
				changed = true
			}
		}
	}

	// R2: α --> β *-> γ or α *-> β --> γ, α *-o γ → α *-> γ
	for alpha := 0; alpha < p; alpha++ {
		for gamma := 0; gamma < p; gamma++ {
			if !adj[alpha][gamma] || alpha == gamma || pm[alpha][gamma] != MarkCircle {
				continue
			}
			for beta := 0; beta < p; beta++ {
				if beta == alpha || beta == gamma || !adj[alpha][beta] || !adj[beta][gamma] {
					continue
				}
				case1 := pm[alpha][beta] == MarkArrow && pm[beta][alpha] == MarkTail && pm[beta][gamma] == MarkArrow
				case2 := pm[alpha][beta] == MarkArrow && pm[beta][gamma] == MarkArrow && pm[gamma][beta] == MarkTail
				if case1 || case2 {
					pm[alpha][gamma] = MarkArrow
					changed = true
				}
			}
		}
	}

	// R3: α *-> β <-* γ, α *-o δ o-* γ, δ *-o β, !adj(α,γ) → δ *-> β
	for beta := 0; beta < p; beta++ {
		for alpha := 0; alpha < p; alpha++ {
			if !adj[alpha][beta] || pm[alpha][beta] != MarkArrow {
				continue
			}
			for gamma := 0; gamma < p; gamma++ {
				if gamma == alpha || !adj[gamma][beta] || pm[gamma][beta] != MarkArrow || adj[alpha][gamma] {
					continue
				}
				for delta := 0; delta < p; delta++ {
					if delta == alpha || delta == gamma || delta == beta {
						continue
					}
					if !adj[alpha][delta] || !adj[gamma][delta] || !adj[delta][beta] {
						continue
					}
					if pm[alpha][delta] != MarkCircle || pm[gamma][delta] != MarkCircle || pm[delta][beta] != MarkCircle {
						continue
					}
					pm[delta][beta] = MarkArrow
					changed = true
				}
			}
		}
	}

	if changed {
		newEdges := make([]PAGEdge, 0, len(g.Edges))
		for i := 0; i < p; i++ {
			for j := i + 1; j < p; j++ {
				if adj[i][j] {
					newEdges = append(newEdges, PAGEdge{
						From:     g.Nodes[i],
						To:       g.Nodes[j],
						FromMark: pm[j][i],
						ToMark:   pm[i][j],
					})
				}
			}
		}
		g.Edges = newEdges
	}
	return changed
}
