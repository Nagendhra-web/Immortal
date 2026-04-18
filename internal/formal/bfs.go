package formal

// stateNode is used internally by the BFS queue.
type stateNode struct {
	W           World
	Path        []string
	Trace       []World
	Depth       int
	UsedIndices map[int]bool // which plan step indices have been applied
}

// bfs explores states level by level, deduplicated by World.Hash().
// Returns (zero node, *Violation, statesVisited) on a violation,
// or (zero node, nil, statesVisited) when exploration is exhausted safely.
//
// successors returns the set of applicable actions given the current world
// and the set of already-used step indices.
func bfs(
	initial World,
	successors func(w World, usedIndices map[int]bool) []indexedAction,
	invariants []Invariant,
	cfg BoundedConfig,
) (stateNode, *Violation, int) {
	visited := make(map[uint64]bool)
	visited[initial.Hash()] = true

	queue := []stateNode{
		{
			W:           initial.Clone(),
			Path:        nil,
			Trace:       []World{initial.Clone()},
			Depth:       0,
			UsedIndices: make(map[int]bool),
		},
	}

	statesVisited := 1

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		if node.Depth >= cfg.MaxDepth {
			continue
		}

		for _, ia := range successors(node.W, node.UsedIndices) {
			next := ia.action.Fn(node.W.Clone())

			// Check invariants before deduplication so we always report a violation
			// even if the same world was reached safely before.
			for _, inv := range invariants {
				if !inv.Fn(next) {
					newPath := append(append([]string(nil), node.Path...), ia.action.Name)
					newTrace := append(append([]World(nil), node.Trace...), next.Clone())
					v := &Violation{
						Invariant: inv.Name,
						Path:      newPath,
						Trace:     newTrace,
						Final:     next.Clone(),
					}
					return stateNode{}, v, statesVisited
				}
			}

			h := next.Hash()
			if visited[h] {
				continue
			}
			if statesVisited >= cfg.MaxStates {
				// Cap reached — return safe (inconclusive but bounded).
				continue
			}
			visited[h] = true
			statesVisited++

			newUsed := make(map[int]bool, len(node.UsedIndices)+1)
			for k, v := range node.UsedIndices {
				newUsed[k] = v
			}
			newUsed[ia.idx] = true

			newPath := append(append([]string(nil), node.Path...), ia.action.Name)
			newTrace := append(append([]World(nil), node.Trace...), next.Clone())

			queue = append(queue, stateNode{
				W:           next,
				Path:        newPath,
				Trace:       newTrace,
				Depth:       node.Depth + 1,
				UsedIndices: newUsed,
			})
		}
	}

	return stateNode{}, nil, statesVisited
}
