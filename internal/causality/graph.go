package causality

import (
	"sync"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/event"
)

// Graph tracks causal relationships between events.
type Graph struct {
	mu       sync.RWMutex
	events   map[string]*event.Event
	parents  map[string]string   // child → parent
	children map[string][]string // parent → children
	order    []string            // insertion order
	window   time.Duration       // time window for auto-correlation
}

// New creates a new causality graph with default 5-second correlation window.
func New() *Graph {
	return NewWithWindow(5 * time.Second)
}

// NewWithWindow creates a graph with a custom time window.
func NewWithWindow(window time.Duration) *Graph {
	return &Graph{
		events:   make(map[string]*event.Event),
		parents:  make(map[string]string),
		children: make(map[string][]string),
		window:   window,
	}
}

// Add records an event in the graph.
func (g *Graph) Add(e *event.Event) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.events[e.ID] = e
	g.order = append(g.order, e.ID)
}

// Link establishes a causal relationship: cause → effect.
func (g *Graph) Link(causeID, effectID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.parents[effectID] = causeID
	g.children[causeID] = append(g.children[causeID], effectID)
}

// RootCause traces the causal chain from an event back to its root cause.
// Returns events from root to the given event.
func (g *Graph) RootCause(eventID string) []*event.Event {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var chain []*event.Event
	current := eventID
	visited := make(map[string]bool)

	for {
		if visited[current] {
			break // Prevent cycles
		}
		visited[current] = true

		e, ok := g.events[current]
		if !ok {
			break
		}
		chain = append([]*event.Event{e}, chain...)

		parent, ok := g.parents[current]
		if !ok {
			break
		}
		current = parent
	}

	return chain
}

// Impact returns all events caused (directly or indirectly) by the given event.
func (g *Graph) Impact(eventID string) []*event.Event {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []*event.Event
	visited := make(map[string]bool)

	var walk func(id string)
	walk = func(id string) {
		for _, childID := range g.children[id] {
			if visited[childID] {
				continue
			}
			visited[childID] = true
			if e, ok := g.events[childID]; ok {
				result = append(result, e)
			}
			walk(childID)
		}
	}

	walk(eventID)
	return result
}

// AutoCorrelate links events that are close in time and likely related.
// Events within the time window are linked by temporal proximity.
func (g *Graph) AutoCorrelate() {
	g.mu.Lock()
	defer g.mu.Unlock()

	for i := 0; i < len(g.order)-1; i++ {
		e1, ok1 := g.events[g.order[i]]
		e2, ok2 := g.events[g.order[i+1]]
		if !ok1 || !ok2 {
			continue
		}

		timeDiff := e2.Timestamp.Sub(e1.Timestamp)
		if timeDiff >= 0 && timeDiff <= g.window {
			// Link if not already linked
			if _, exists := g.parents[e2.ID]; !exists {
				g.parents[e2.ID] = e1.ID
				g.children[e1.ID] = append(g.children[e1.ID], e2.ID)
			}
		}
	}
}
