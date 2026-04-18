package causal

import (
	"sort"
)

// CausalGraph holds a partially directed acyclic graph produced by the PC algorithm.
// Directed edges represent oriented causal arrows; Undirected edges are skeleton
// edges the algorithm could not orient.
type CausalGraph struct {
	Nodes      []string
	Directed   map[string][]string // from → []to
	Undirected map[string][]string // stored symmetrically: both a→b and b→a present
}

// HasEdge returns true if there is any edge (directed or undirected) from→to.
func (g CausalGraph) HasEdge(from, to string) bool {
	for _, t := range g.Directed[from] {
		if t == to {
			return true
		}
	}
	for _, t := range g.Undirected[from] {
		if t == to {
			return true
		}
	}
	return false
}

// Ancestors returns all nodes that have a path to node, following directed
// edges backward and undirected skeleton edges (which represent possible
// causal links the algorithm could not orient). In the Markov equivalence
// class, any undirected neighbor is a candidate ancestor.
func (g CausalGraph) Ancestors(node string) []string {
	visited := map[string]bool{}
	var dfs func(n string)
	dfs = func(n string) {
		for src, targets := range g.Directed {
			for _, t := range targets {
				if t == n && !visited[src] {
					visited[src] = true
					dfs(src)
				}
			}
		}
		for _, u := range g.Undirected[n] {
			if !visited[u] && u != node {
				visited[u] = true
				dfs(u)
			}
		}
	}
	dfs(node)
	result := make([]string, 0, len(visited))
	for k := range visited {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

// DiscoverConfig controls the PC skeleton search.
type DiscoverConfig struct {
	Alpha          float64 // CI test significance threshold (e.g. 0.05)
	MaxCondSetSize int     // maximum conditioning set size (default 3)
}

// Discover runs the PC algorithm: skeleton phase then v-structure orientation.
func Discover(ds *Dataset, cfg DiscoverConfig) (CausalGraph, error) {
	if ds.Rows() < 10 {
		return CausalGraph{}, ErrInsufficientData
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

	// adjacency as symmetric boolean matrix (skeleton)
	adj := make([][]bool, p)
	for i := range adj {
		adj[i] = make([]bool, p)
		for j := range adj[i] {
			adj[i][j] = i != j
		}
	}

	// sep(i,j) = the separating set that made i⊥j
	sep := make([][]int, p)
	for i := range sep {
		sep[i] = make([]int, p)
		for j := range sep[i] {
			sep[i][j] = -1 // -1 means no sep set recorded yet
		}
	}
	// store actual sep sets as variable-name slices
	sepSet := make([][]string, p*p)

	// Skeleton phase: for each conditioning set size d = 0..MaxCondSetSize
	for d := 0; d <= cfg.MaxCondSetSize; d++ {
		changed := false
		for i := 0; i < p; i++ {
			for j := i + 1; j < p; j++ {
				if !adj[i][j] {
					continue
				}
				// neighbors of i excluding j
				nbrs := neighbors(adj, i, j, p)
				if len(nbrs) < d {
					continue
				}
				// iterate over subsets of nbrs of size d
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
						return true // stop subset search
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

	// Build directed / undirected maps; initially all edges are undirected.
	directed := make(map[string][]string)
	undirected := make(map[string][]string)

	for i := 0; i < p; i++ {
		for j := i + 1; j < p; j++ {
			if adj[i][j] {
				undirected[nodes[i]] = append(undirected[nodes[i]], nodes[j])
				undirected[nodes[j]] = append(undirected[nodes[j]], nodes[i])
			}
		}
	}

	// Orient v-structures: i - k - j, no edge i-j, k ∉ sep(i,j) → i→k←j
	for k := 0; k < p; k++ {
		// find all pairs (i,j) that are both adjacent to k but not to each other
		nbrsK := neighborsAll(adj, k, p)
		for ai := 0; ai < len(nbrsK); ai++ {
			for bi := ai + 1; bi < len(nbrsK); bi++ {
				i, j := nbrsK[ai], nbrsK[bi]
				if adj[i][j] {
					continue // i and j are adjacent — not a v-structure
				}
				ss := sepSet[i*p+j]
				if containsIdx(ss, nodes[k]) {
					continue // k is in the sep set — not a v-structure
				}
				// orient i→k and j→k
				orientEdge(undirected, directed, nodes[i], nodes[k])
				orientEdge(undirected, directed, nodes[j], nodes[k])
			}
		}
	}

	return CausalGraph{
		Nodes:      nodes,
		Directed:   directed,
		Undirected: undirected,
	}, nil
}

// neighbors returns indices of nodes adjacent to i (in the current skeleton),
// excluding j.
func neighbors(adj [][]bool, i, j, p int) []int {
	var nb []int
	for k := 0; k < p; k++ {
		if k != i && k != j && adj[i][k] {
			nb = append(nb, k)
		}
	}
	return nb
}

// neighborsAll returns all neighbors of node i.
func neighborsAll(adj [][]bool, i, p int) []int {
	var nb []int
	for k := 0; k < p; k++ {
		if k != i && adj[i][k] {
			nb = append(nb, k)
		}
	}
	return nb
}

// iterSubsets calls fn on each subset of elems of size k.
// Returns true if fn returned true (early exit).
func iterSubsets(elems []int, k int, fn func([]int) bool) bool {
	if k == 0 {
		return fn([]int{})
	}
	n := len(elems)
	if n < k {
		return false
	}
	sub := make([]int, k)
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

// containsIdx checks whether name appears in the list.
func containsIdx(list []string, name string) bool {
	for _, s := range list {
		if s == name {
			return true
		}
	}
	return false
}

// orientEdge moves the undirected edge from→to into the directed map.
// Safe to call when the edge is already directed (no-op).
func orientEdge(undirected, directed map[string][]string, from, to string) {
	// remove from undirected (both directions)
	undirected[from] = removeStr(undirected[from], to)
	undirected[to] = removeStr(undirected[to], from)
	// add directed if not already present
	if !containsIdx(directed[from], to) {
		directed[from] = append(directed[from], to)
	}
}

func removeStr(sl []string, s string) []string {
	out := sl[:0]
	for _, v := range sl {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}
