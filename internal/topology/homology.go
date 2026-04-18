package topology

import (
	"fmt"
	"time"
)

// Snapshot captures topological invariants of a dependency graph at a point in time.
// The numbers correspond to Betti numbers of the graph's undirected skeleton:
//
//	β_0 = Components  (connected components)
//	β_1 = Cycles      (independent cycles; β_1 = |E_undirected| - |V| + β_0)
type Snapshot struct {
	Time       time.Time
	NodeCount  int
	EdgeCount  int
	Components int  // β_0: number of weakly-connected components
	Cycles     int  // β_1: number of independent cycles
	MaxBlast   int  // largest blast radius across all nodes
	CycleRank  int  // same as Cycles (kept for API compatibility)
	Degenerate bool // true if any self-loop or isolated node exists
}

// SnapshotOf computes topological statistics on the current state of g.
func SnapshotOf(g *DiGraph) Snapshot {
	nodes := g.Nodes()
	edges := g.Edges()
	v := len(nodes)
	e := len(edges)

	// count unique undirected edges (treat A→B and B→A as the same edge for β_1)
	undirectedSet := make(map[[2]string]bool)
	for _, edge := range edges {
		a, b := edge[0], edge[1]
		if a > b {
			a, b = b, a
		}
		undirectedSet[[2]string{a, b}] = true
	}
	eu := len(undirectedSet)

	// weakly-connected components
	comps := g.ConnectedComponents()
	c := len(comps)

	// β_1 = |E_undirected| - |V| + β_0
	beta1 := eu - v + c
	if beta1 < 0 {
		beta1 = 0
	}

	// max blast radius
	maxBlast := 0
	for _, n := range nodes {
		br := len(g.BlastRadius(n))
		if br > maxBlast {
			maxBlast = br
		}
	}

	// degenerate: any self-loop or isolated node (no edges at all)
	degenerate := false
	for _, edge := range edges {
		if edge[0] == edge[1] {
			degenerate = true
			break
		}
	}
	if !degenerate {
		for _, n := range nodes {
			if len(g.FanIn(n)) == 0 && len(g.FanOut(n)) == 0 {
				degenerate = true
				break
			}
		}
	}

	return Snapshot{
		Time:       time.Now(),
		NodeCount:  v,
		EdgeCount:  e,
		Components: c,
		Cycles:     beta1,
		MaxBlast:   maxBlast,
		CycleRank:  beta1,
		Degenerate: degenerate,
	}
}

// EventKind classifies a topological change detected between two snapshots.
type EventKind int

const (
	ComponentBirth EventKind = iota // a new component appeared (fragmentation)
	ComponentDeath                  // two components merged
	CycleBirth                      // a new cycle formed (potential dependency loop)
	CycleDeath                      // a cycle was broken
	BlastIncrease                   // max blast radius grew significantly
	Fragmentation                   // component count grew by more than 1
)

// Event describes a topological change between two consecutive snapshots.
type Event struct {
	Time        time.Time
	Kind        EventKind
	Description string
	Delta       int // signed count change
}

// Tracker stores snapshots over time and computes births/deaths of features.
type Tracker struct {
	maxSnapshots int
	snapshots    []Snapshot
	// cursor tracks which snapshot pairs have already been processed by Events()
	eventCursor int
	// cache of emitted events
	events []Event
	// cursor for Check: index into events already seen by Check
	checkCursor int
}

// NewTracker creates a new Tracker with a bounded history window.
// If maxSnapshots <= 0, it defaults to 500.
func NewTracker(maxSnapshots int) *Tracker {
	if maxSnapshots <= 0 {
		maxSnapshots = 500
	}
	return &Tracker{maxSnapshots: maxSnapshots}
}

// Record computes a snapshot of g and appends it to the history.
// If the history exceeds maxSnapshots, the oldest entry is dropped.
func (t *Tracker) Record(g *DiGraph) {
	snap := SnapshotOf(g)
	t.snapshots = append(t.snapshots, snap)
	if len(t.snapshots) > t.maxSnapshots {
		drop := len(t.snapshots) - t.maxSnapshots
		t.snapshots = t.snapshots[drop:]
		// adjust cursors
		if t.eventCursor > drop {
			t.eventCursor -= drop
		} else {
			t.eventCursor = 0
		}
	}
	// detect new events from newly added pairs
	t.detectEvents()
}

// Latest returns the most recent snapshot, or a zero Snapshot if none recorded.
func (t *Tracker) Latest() Snapshot {
	if len(t.snapshots) == 0 {
		return Snapshot{}
	}
	return t.snapshots[len(t.snapshots)-1]
}

// History returns a copy of all recorded snapshots in chronological order.
func (t *Tracker) History() []Snapshot {
	out := make([]Snapshot, len(t.snapshots))
	copy(out, t.snapshots)
	return out
}

// Events returns the topological events detected across all recorded snapshots.
func (t *Tracker) Events() []Event {
	out := make([]Event, len(t.events))
	copy(out, t.events)
	return out
}

// detectEvents processes any newly added snapshot pairs and appends events.
func (t *Tracker) detectEvents() {
	// we need at least 2 snapshots; eventCursor tracks the last processed "previous" index
	for t.eventCursor < len(t.snapshots)-1 {
		prev := t.snapshots[t.eventCursor]
		cur := t.snapshots[t.eventCursor+1]
		t.eventCursor++
		t.events = append(t.events, diffSnapshots(prev, cur)...)
	}
}

func diffSnapshots(prev, cur Snapshot) []Event {
	var events []Event
	now := cur.Time

	// Component changes
	deltaC := cur.Components - prev.Components
	if deltaC > 1 {
		events = append(events, Event{
			Time:        now,
			Kind:        Fragmentation,
			Description: fmt.Sprintf("fleet fragmented: components grew by %d", deltaC),
			Delta:       deltaC,
		})
	} else if deltaC == 1 {
		events = append(events, Event{
			Time:        now,
			Kind:        ComponentBirth,
			Description: "a new disconnected component appeared",
			Delta:       1,
		})
	} else if deltaC < 0 {
		events = append(events, Event{
			Time:        now,
			Kind:        ComponentDeath,
			Description: fmt.Sprintf("components merged: count dropped by %d", -deltaC),
			Delta:       deltaC,
		})
	}

	// Cycle changes
	deltaY := cur.Cycles - prev.Cycles
	if deltaY > 0 {
		events = append(events, Event{
			Time:        now,
			Kind:        CycleBirth,
			Description: fmt.Sprintf("dependency cycle(s) appeared: +%d independent cycle(s)", deltaY),
			Delta:       deltaY,
		})
	} else if deltaY < 0 {
		events = append(events, Event{
			Time:        now,
			Kind:        CycleDeath,
			Description: fmt.Sprintf("cycle(s) resolved: %d fewer independent cycle(s)", -deltaY),
			Delta:       deltaY,
		})
	}

	// Blast radius increase (10% threshold or absolute growth of ≥2)
	deltaB := cur.MaxBlast - prev.MaxBlast
	threshold := prev.MaxBlast/10 + 2
	if deltaB >= threshold {
		events = append(events, Event{
			Time:        now,
			Kind:        BlastIncrease,
			Description: fmt.Sprintf("max blast radius grew from %d to %d", prev.MaxBlast, cur.MaxBlast),
			Delta:       deltaB,
		})
	}

	return events
}
