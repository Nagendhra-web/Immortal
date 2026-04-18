// Package formal provides a TLA+-style model checker for healing plans.
// Given a plan and a set of safety invariants (boolean predicates over service states),
// it exhaustively explores reachable states via BFS and either returns "plan is safe"
// or a concrete counterexample trace showing the first invariant violation.
package formal

import (
	"encoding/binary"
	"hash/fnv"
	"sort"
)

// ServiceState is a minimal struct — intentionally small so state hashing is cheap.
type ServiceState struct {
	Name     string
	Healthy  bool
	Replicas int
}

// World is the system state: map of service name → ServiceState.
type World map[string]ServiceState

// Clone returns a deep copy of the World.
func (w World) Clone() World {
	c := make(World, len(w))
	for k, v := range w {
		c[k] = v
	}
	return c
}

// Hash returns an FNV-64 digest over sorted (name, healthy, replicas) triples.
// Deterministic regardless of map iteration order.
func (w World) Hash() uint64 {
	// Collect and sort keys for determinism.
	keys := make([]string, 0, len(w))
	for k := range w {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := fnv.New64a()
	buf := make([]byte, 4)
	for _, k := range keys {
		s := w[k]
		_, _ = h.Write([]byte(k))
		if s.Healthy {
			_, _ = h.Write([]byte{1})
		} else {
			_, _ = h.Write([]byte{0})
		}
		binary.LittleEndian.PutUint32(buf, uint32(s.Replicas))
		_, _ = h.Write(buf)
	}
	return h.Sum64()
}

// Action is one step in a plan that transitions World → World.
type Action struct {
	Name string
	Fn   func(World) World // pure transition; receives a clone
}

// Plan is an ordered list of Actions.
type Plan struct {
	ID    string
	Steps []Action
}

// Invariant is a boolean predicate over World that must always hold.
type Invariant struct {
	Name        string
	Description string
	Fn          func(World) bool
}

// Violation reports the first invariant failure encountered.
type Violation struct {
	Invariant string
	Path      []string // action names leading to the violating state
	Trace     []World  // worlds at each step (including initial)
	Final     World    // the offending world
}

// Result is either (Safe: true) or (Safe: false, Violation: <trace>).
type Result struct {
	Safe          bool
	StatesVisited int
	Depth         int
	Violation     *Violation
}

// BoundedConfig controls CheckBounded exploration.
type BoundedConfig struct {
	MaxDepth       int  // default len(plan.Steps)
	MaxStates      int  // safety cap; default 100 000
	PermuteActions bool // if true, explore all orderings (exponential!)
}

// Check executes plan steps in order, checking every invariant after each step.
// Returns Safe=true iff no invariant ever fires.
//
// Algorithm:
//
//	current := initial
//	for each step: next = step.Fn(current.Clone()); check invariants; current = next
func Check(initial World, plan Plan, invariants []Invariant) Result {
	current := initial.Clone()
	trace := []World{current.Clone()}
	path := []string{}

	for _, step := range plan.Steps {
		next := step.Fn(current.Clone())
		path = append(path, step.Name)
		trace = append(trace, next.Clone())

		for _, inv := range invariants {
			if !inv.Fn(next) {
				return Result{
					Safe:          false,
					StatesVisited: len(trace),
					Depth:         len(path),
					Violation: &Violation{
						Invariant: inv.Name,
						Path:      append([]string(nil), path...),
						Trace:     trace,
						Final:     next.Clone(),
					},
				}
			}
		}
		current = next
	}

	return Result{
		Safe:          true,
		StatesVisited: len(trace),
		Depth:         len(plan.Steps),
	}
}

// CheckBounded explores reachable states via BFS with deduplication.
// When cfg.PermuteActions is true it explores all orderings of plan steps
// (each step applied at most once per path), modelling parallel / out-of-order
// execution. When false it behaves like Check but with cycle detection.
func CheckBounded(initial World, plan Plan, invariants []Invariant, cfg BoundedConfig) Result {
	if cfg.MaxDepth == 0 {
		cfg.MaxDepth = len(plan.Steps)
	}
	if cfg.MaxStates == 0 {
		cfg.MaxStates = 100_000
	}

	// successors: for a given world+path, which steps can be applied next?
	var successors func(w World, usedIndices map[int]bool) []indexedAction
	if cfg.PermuteActions {
		successors = func(w World, usedIndices map[int]bool) []indexedAction {
			var out []indexedAction
			for i, s := range plan.Steps {
				if !usedIndices[i] {
					out = append(out, indexedAction{idx: i, action: s})
				}
			}
			return out
		}
	} else {
		// Ordered: next step is the first unused index.
		successors = func(w World, usedIndices map[int]bool) []indexedAction {
			for i, s := range plan.Steps {
				if !usedIndices[i] {
					return []indexedAction{{idx: i, action: s}}
				}
			}
			return nil
		}
	}

	node, violation, visited := bfs(initial, successors, invariants, cfg)
	_ = node
	if violation != nil {
		return Result{
			Safe:          false,
			StatesVisited: visited,
			Depth:         len(violation.Path),
			Violation:     violation,
		}
	}
	return Result{
		Safe:          true,
		StatesVisited: visited,
		Depth:         cfg.MaxDepth,
	}
}

// indexedAction pairs a step with its original index so we can track "used".
type indexedAction struct {
	idx    int
	action Action
}
