package dependency

import (
	"sort"
	"sync"
)

// edge represents a directed dependency relationship: from depends on to.
type edge struct {
	from string
	to   string
}

// Node describes a service and its relationships in the graph.
type Node struct {
	Name         string   `json:"name"`
	Dependencies []string `json:"dependencies"`
	Dependents   []string `json:"dependents"`
	Depth        int      `json:"depth"`
}

// Graph is a thread-safe directed dependency graph.
type Graph struct {
	mu    sync.RWMutex
	edges []edge
	nodes map[string]bool
}

// New creates an empty dependency graph.
func New() *Graph {
	return &Graph{
		nodes: make(map[string]bool),
	}
}

// AddNode registers a service node.
func (g *Graph) AddNode(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nodes[name] = true
}

// AddDependency records that from depends on to. Both nodes are auto-registered.
func (g *Graph) AddDependency(from, to string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nodes[from] = true
	g.nodes[to] = true
	// avoid duplicate edges
	for _, e := range g.edges {
		if e.from == from && e.to == to {
			return
		}
	}
	g.edges = append(g.edges, edge{from: from, to: to})
}

// RemoveDependency removes the dependency edge from -> to.
func (g *Graph) RemoveDependency(from, to string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	filtered := g.edges[:0]
	for _, e := range g.edges {
		if e.from == from && e.to == to {
			continue
		}
		filtered = append(filtered, e)
	}
	g.edges = filtered
}

// Dependencies returns the direct dependencies of service (sorted).
func (g *Graph) Dependencies(service string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.directDeps(service)
}

// Dependents returns services that directly depend on service (sorted).
func (g *Graph) Dependents(service string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.directDependents(service)
}

// TransitiveDependencies returns all dependencies recursively (sorted, no duplicates).
func (g *Graph) TransitiveDependencies(service string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	visited := make(map[string]bool)
	g.collectDeps(service, visited)
	delete(visited, service)
	return sortedKeys(visited)
}

// TransitiveDependents returns all services that (transitively) depend on service (sorted, no duplicates).
func (g *Graph) TransitiveDependents(service string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	visited := make(map[string]bool)
	g.collectDependents(service, visited)
	delete(visited, service)
	return sortedKeys(visited)
}

// ImpactOf returns the count of transitive dependents of service.
func (g *Graph) ImpactOf(service string) int {
	return len(g.TransitiveDependents(service))
}

// CriticalPath returns all services sorted by impact descending (most impacted first).
func (g *Graph) CriticalPath() []string {
	g.mu.RLock()
	all := make([]string, 0, len(g.nodes))
	for n := range g.nodes {
		all = append(all, n)
	}
	g.mu.RUnlock()

	sort.Slice(all, func(i, j int) bool {
		ii := g.ImpactOf(all[i])
		jj := g.ImpactOf(all[j])
		if ii != jj {
			return ii > jj
		}
		return all[i] < all[j]
	})
	return all
}

// All returns all nodes with their relationships filled in, sorted by name.
func (g *Graph) All() []Node {
	g.mu.RLock()
	names := make([]string, 0, len(g.nodes))
	for n := range g.nodes {
		names = append(names, n)
	}
	g.mu.RUnlock()

	sort.Strings(names)
	result := make([]Node, 0, len(names))
	for _, name := range names {
		deps := g.Dependencies(name)
		dependents := g.Dependents(name)
		if deps == nil {
			deps = []string{}
		}
		if dependents == nil {
			dependents = []string{}
		}
		result = append(result, Node{
			Name:         name,
			Dependencies: deps,
			Dependents:   dependents,
			Depth:        g.depth(name),
		})
	}
	return result
}

// HasCycle returns true if the graph contains a circular dependency.
func (g *Graph) HasCycle() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// 0=unvisited, 1=in-stack, 2=done
	state := make(map[string]int)
	for n := range g.nodes {
		if state[n] == 0 {
			if g.dfsCycle(n, state) {
				return true
			}
		}
	}
	return false
}

// Roots returns nodes with no dependents (top-level services), sorted.
func (g *Graph) Roots() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var result []string
	for n := range g.nodes {
		if len(g.directDependents(n)) == 0 {
			result = append(result, n)
		}
	}
	sort.Strings(result)
	return result
}

// Leaves returns nodes with no dependencies (bottom-level services), sorted.
func (g *Graph) Leaves() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var result []string
	for n := range g.nodes {
		if len(g.directDeps(n)) == 0 {
			result = append(result, n)
		}
	}
	sort.Strings(result)
	return result
}

// --- internal helpers (caller must hold appropriate lock) ---

func (g *Graph) directDeps(service string) []string {
	var result []string
	for _, e := range g.edges {
		if e.from == service {
			result = append(result, e.to)
		}
	}
	sort.Strings(result)
	return result
}

func (g *Graph) directDependents(service string) []string {
	var result []string
	for _, e := range g.edges {
		if e.to == service {
			result = append(result, e.from)
		}
	}
	sort.Strings(result)
	return result
}

func (g *Graph) collectDeps(service string, visited map[string]bool) {
	if visited[service] {
		return
	}
	visited[service] = true
	for _, e := range g.edges {
		if e.from == service {
			g.collectDeps(e.to, visited)
		}
	}
}

func (g *Graph) collectDependents(service string, visited map[string]bool) {
	if visited[service] {
		return
	}
	visited[service] = true
	for _, e := range g.edges {
		if e.to == service {
			g.collectDependents(e.from, visited)
		}
	}
}

func (g *Graph) dfsCycle(node string, state map[string]int) bool {
	state[node] = 1
	for _, e := range g.edges {
		if e.from == node {
			if state[e.to] == 1 {
				return true
			}
			if state[e.to] == 0 && g.dfsCycle(e.to, state) {
				return true
			}
		}
	}
	state[node] = 2
	return false
}

// depth returns the longest path from any root (node with no dependents) to this node.
func (g *Graph) depth(service string) int {
	// BFS/DFS from dependents upward: longest incoming path length
	memo := make(map[string]int)
	return g.depthMemo(service, memo)
}

func (g *Graph) depthMemo(service string, memo map[string]int) int {
	if v, ok := memo[service]; ok {
		return v
	}
	// Cycle guard: install a sentinel BEFORE recursing so a cyclic dependency
	// (e.g. api↔db) terminates instead of overflowing the stack.
	memo[service] = 0
	max := 0
	for _, e := range g.edges {
		if e.to == service {
			d := g.depthMemo(e.from, memo) + 1
			if d > max {
				max = d
			}
		}
	}
	memo[service] = max
	return max
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
