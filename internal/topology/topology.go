// Package topology provides a persistent-homology-inspired tracker for service
// dependency graphs. It detects architectural drift, dependency cycles, and
// blast-radius changes over time — tracking not just what is broken but how
// the shape of the system is evolving.
package topology

import "sort"

// DiGraph is a directed dependency graph.
// An edge from src→dst means "src depends on dst".
type DiGraph struct {
	adj  map[string]map[string]bool // adjacency: adj[from][to]
	radj map[string]map[string]bool // reverse adjacency: radj[to][from]
}

// NewDiGraph creates an empty directed graph.
func NewDiGraph() *DiGraph {
	return &DiGraph{
		adj:  make(map[string]map[string]bool),
		radj: make(map[string]map[string]bool),
	}
}

// AddNode ensures node exists in the graph (idempotent).
func (g *DiGraph) AddNode(node string) {
	if _, ok := g.adj[node]; !ok {
		g.adj[node] = make(map[string]bool)
	}
	if _, ok := g.radj[node]; !ok {
		g.radj[node] = make(map[string]bool)
	}
}

// AddEdge adds a directed edge from→to, auto-registering both nodes.
func (g *DiGraph) AddEdge(from, to string) {
	g.AddNode(from)
	g.AddNode(to)
	g.adj[from][to] = true
	g.radj[to][from] = true
}

// RemoveEdge removes the directed edge from→to (no-op if absent).
func (g *DiGraph) RemoveEdge(from, to string) {
	if m, ok := g.adj[from]; ok {
		delete(m, to)
	}
	if m, ok := g.radj[to]; ok {
		delete(m, from)
	}
}

// RemoveNode removes a node and all its incident edges.
func (g *DiGraph) RemoveNode(node string) {
	// remove outgoing edges
	for to := range g.adj[node] {
		delete(g.radj[to], node)
	}
	// remove incoming edges
	for from := range g.radj[node] {
		delete(g.adj[from], node)
	}
	delete(g.adj, node)
	delete(g.radj, node)
}

// Nodes returns all nodes in sorted order.
func (g *DiGraph) Nodes() []string {
	out := make([]string, 0, len(g.adj))
	for n := range g.adj {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Edges returns all directed edges as [2]string{from, to}, sorted.
func (g *DiGraph) Edges() [][2]string {
	var out [][2]string
	for from, tos := range g.adj {
		for to := range tos {
			out = append(out, [2]string{from, to})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
	return out
}

// Neighbors returns direct out-neighbors (same as FanOut).
func (g *DiGraph) Neighbors(node string) []string {
	return g.FanOut(node)
}

// Clone returns a deep copy of the graph.
func (g *DiGraph) Clone() *DiGraph {
	c := NewDiGraph()
	for from, tos := range g.adj {
		c.AddNode(from)
		for to := range tos {
			c.AddEdge(from, to)
		}
	}
	return c
}

// FanOut returns the direct out-neighbors of node (sorted).
func (g *DiGraph) FanOut(node string) []string {
	m := g.adj[node]
	out := make([]string, 0, len(m))
	for n := range m {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// FanIn returns the direct in-neighbors of node (sorted).
func (g *DiGraph) FanIn(node string) []string {
	m := g.radj[node]
	out := make([]string, 0, len(m))
	for n := range m {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// ConnectedComponents returns the weakly-connected components of the graph,
// treating all edges as undirected. Uses union-find.
func (g *DiGraph) ConnectedComponents() [][]string {
	parent := make(map[string]string)
	var find func(x string) string
	find = func(x string) string {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(x, y string) {
		px, py := find(x), find(y)
		if px != py {
			parent[px] = py
		}
	}

	// initialise each node as its own root
	for n := range g.adj {
		parent[n] = n
	}
	// union connected nodes (undirected view)
	for from, tos := range g.adj {
		for to := range tos {
			union(from, to)
		}
	}

	// group by root
	groups := make(map[string][]string)
	for n := range g.adj {
		root := find(n)
		groups[root] = append(groups[root], n)
	}

	components := make([][]string, 0, len(groups))
	for _, members := range groups {
		sort.Strings(members)
		components = append(components, members)
	}
	// sort components by their first element for determinism
	sort.Slice(components, func(i, j int) bool {
		return components[i][0] < components[j][0]
	})
	return components
}

// StronglyConnectedComponents returns the SCCs using Tarjan's iterative algorithm.
// Any SCC of size > 1 contains a directed cycle.
func (g *DiGraph) StronglyConnectedComponents() [][]string {
	index := 0
	idx := make(map[string]int)
	low := make(map[string]int)
	onStack := make(map[string]bool)
	var stack []string
	var sccs [][]string

	// iterative Tarjan using an explicit call stack to avoid Go stack overflow
	// on large graphs. Each entry is (node, iterator-position).
	type frame struct {
		node     string
		children []string
		childIdx int
	}

	var strongconnect func(v string)
	strongconnect = func(v string) {
		// We use a worklist-based DFS.
		callStack := []*frame{{node: v, children: g.sortedNeighbors(v), childIdx: 0}}
		idx[v] = index
		low[v] = index
		index++
		stack = append(stack, v)
		onStack[v] = true

		for len(callStack) > 0 {
			top := callStack[len(callStack)-1]
			if top.childIdx < len(top.children) {
				w := top.children[top.childIdx]
				top.childIdx++
				if _, visited := idx[w]; !visited {
					idx[w] = index
					low[w] = index
					index++
					stack = append(stack, w)
					onStack[w] = true
					callStack = append(callStack, &frame{node: w, children: g.sortedNeighbors(w), childIdx: 0})
				} else if onStack[w] {
					if low[top.node] > idx[w] {
						low[top.node] = idx[w]
					}
				}
			} else {
				// pop frame, propagate low
				callStack = callStack[:len(callStack)-1]
				if len(callStack) > 0 {
					parent := callStack[len(callStack)-1]
					if low[parent.node] > low[top.node] {
						low[parent.node] = low[top.node]
					}
				}
				// if root of SCC, pop the stack
				if low[top.node] == idx[top.node] {
					var scc []string
					for {
						w := stack[len(stack)-1]
						stack = stack[:len(stack)-1]
						onStack[w] = false
						scc = append(scc, w)
						if w == top.node {
							break
						}
					}
					sort.Strings(scc)
					sccs = append(sccs, scc)
				}
			}
		}
	}

	for _, n := range g.Nodes() {
		if _, visited := idx[n]; !visited {
			strongconnect(n)
		}
	}

	sort.Slice(sccs, func(i, j int) bool {
		return sccs[i][0] < sccs[j][0]
	})
	return sccs
}

// Cycles returns all directed cycles as node lists.
// Each SCC of size > 1 is reported as a cycle (its members participate in a cycle).
func (g *DiGraph) Cycles() [][]string {
	var cycles [][]string
	for _, scc := range g.StronglyConnectedComponents() {
		if len(scc) > 1 {
			cycles = append(cycles, scc)
		}
	}
	return cycles
}

// BlastRadius returns all services that are transitively dependent on target —
// i.e., services that will be affected if target fails.
// It follows reverse edges (in-neighbors) from target outward via BFS.
func (g *DiGraph) BlastRadius(target string) []string {
	visited := make(map[string]bool)
	queue := []string{target}
	visited[target] = true

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for from := range g.radj[cur] {
			if !visited[from] {
				visited[from] = true
				queue = append(queue, from)
			}
		}
	}

	delete(visited, target)
	result := make([]string, 0, len(visited))
	for n := range visited {
		result = append(result, n)
	}
	sort.Strings(result)
	return result
}

// sortedNeighbors returns out-neighbors of node in sorted order (internal helper).
func (g *DiGraph) sortedNeighbors(node string) []string {
	m := g.adj[node]
	out := make([]string, 0, len(m))
	for n := range m {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
