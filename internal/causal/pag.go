package causal

import "sort"

// EdgeMark represents the endpoint mark on a PAG edge.
// PAG edges have two marks: one at each endpoint.
//
//	MarkCircle `o` — unknown (could be arrow or tail)
//	MarkArrow  `>` or `<` — definite arrowhead
//	MarkTail   `-` — definite tail (non-ancestor)
type EdgeMark int

const (
	MarkCircle EdgeMark = iota // `o`
	MarkArrow                  // `>` or `<`
	MarkTail                   // `-`
)

// PAGEdge is a single edge in a Partial Ancestral Graph.
// Semantics: A <FromMark>-<ToMark> B where FromMark is the endpoint mark
// at A (the From node) and ToMark is the endpoint mark at B (the To node).
//
// Examples:
//
//	o-o  : MarkCircle / MarkCircle  — completely undetermined
//	o->  : MarkCircle / MarkArrow   — arrowhead at B, uncertain at A
//	<->  : MarkArrow  / MarkArrow   — bidirected (latent common cause)
//	-->  : MarkTail   / MarkArrow   — definite directed edge A → B
type PAGEdge struct {
	From, To         string
	FromMark, ToMark EdgeMark
}

// PAG is a Partial Ancestral Graph, the output of the FCI algorithm.
// It represents the Markov equivalence class of DAGs that are consistent
// with the observed conditional independencies, allowing for latent variables.
type PAG struct {
	Nodes []string
	Edges []PAGEdge
}

// HasEdge returns true if there is any edge between from and to (in either
// direction).
func (p PAG) HasEdge(from, to string) bool {
	for _, e := range p.Edges {
		if (e.From == from && e.To == to) || (e.From == to && e.To == from) {
			return true
		}
	}
	return false
}

// DefiniteAncestors returns nodes that are definite ancestors of node,
// following only `-->` edges (MarkTail → MarkArrow).
func (p PAG) DefiniteAncestors(node string) []string {
	visited := map[string]bool{}
	var dfs func(n string)
	dfs = func(n string) {
		for _, e := range p.Edges {
			// follow --> edges: tail at From, arrow at To pointing to n
			if e.To == n && e.FromMark == MarkTail && e.ToMark == MarkArrow {
				if !visited[e.From] {
					visited[e.From] = true
					dfs(e.From)
				}
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

// PossibleAncestors returns nodes that are possible ancestors of node,
// following any edge that could represent an ancestral relationship
// (o-o, o->, <->, -->). Excludes definite non-ancestors (tail at target).
func (p PAG) PossibleAncestors(node string) []string {
	visited := map[string]bool{}
	var dfs func(n string)
	dfs = func(n string) {
		for _, e := range p.Edges {
			// An edge can represent ancestry toward n if:
			// - e.To == n and the ToMark is not MarkTail (i.e. arrowhead or circle at n)
			// - e.From == n and the FromMark is not MarkTail (circle endpoint)
			//   meaning the edge could go the other way
			if e.To == n && e.ToMark != MarkTail {
				if !visited[e.From] {
					visited[e.From] = true
					dfs(e.From)
				}
			}
			if e.From == n && e.FromMark != MarkTail {
				if !visited[e.To] {
					visited[e.To] = true
					dfs(e.To)
				}
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
