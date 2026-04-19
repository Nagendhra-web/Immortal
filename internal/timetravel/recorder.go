package timetravel

import (
	"fmt"
	"sync"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/event"
)

// Snapshot captures system state at a point in time.
type Snapshot struct {
	Name      string                 `json:"name"`
	Timestamp time.Time              `json:"timestamp"`
	State     map[string]interface{} `json:"state"`
}

// Diff represents a difference between two snapshots.
type Diff struct {
	Key    string      `json:"key"`
	Before interface{} `json:"before"`
	After  interface{} `json:"after"`
}

// Recorder captures events and snapshots for time-travel debugging.
type Recorder struct {
	mu        sync.RWMutex
	events    []*event.Event
	snapshots map[string]*Snapshot
	maxEvents int
}

// New creates a new time-travel recorder with a max event buffer.
func New(maxEvents int) *Recorder {
	return &Recorder{
		events:    make([]*event.Event, 0, maxEvents),
		snapshots: make(map[string]*Snapshot),
		maxEvents: maxEvents,
	}
}

// Record adds an event to the timeline.
func (r *Recorder) Record(e *event.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.events) >= r.maxEvents {
		r.events = r.events[1:]
	}
	r.events = append(r.events, e)
}

// Replay returns all events within a time range, ordered chronologically.
func (r *Recorder) Replay(from, to time.Time) []*event.Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*event.Event
	for _, e := range r.events {
		if (from.IsZero() || !e.Timestamp.Before(from)) &&
			(to.IsZero() || !e.Timestamp.After(to)) {
			result = append(result, e)
		}
	}
	return result
}

// RewindBefore returns the last N events before a given timestamp.
func (r *Recorder) RewindBefore(before time.Time, count int) []*event.Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var candidates []*event.Event
	for _, e := range r.events {
		if e.Timestamp.Before(before) {
			candidates = append(candidates, e)
		}
	}

	if len(candidates) <= count {
		return candidates
	}
	return candidates[len(candidates)-count:]
}

// TakeSnapshot captures the current state under a name.
func (r *Recorder) TakeSnapshot(name string, state map[string]interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.snapshots[name] = &Snapshot{
		Name:      name,
		Timestamp: time.Now(),
		State:     state,
	}
}

// Snapshots returns all snapshots.
func (r *Recorder) Snapshots() []*Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Snapshot
	for _, s := range r.snapshots {
		result = append(result, s)
	}
	return result
}

// DiffSnapshots compares two named snapshots and returns differences.
func (r *Recorder) DiffSnapshots(name1, name2 string) []Diff {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s1, ok1 := r.snapshots[name1]
	s2, ok2 := r.snapshots[name2]
	if !ok1 || !ok2 {
		return nil
	}

	var diffs []Diff

	// Check all keys in s1
	for k, v1 := range s1.State {
		v2, exists := s2.State[k]
		if !exists {
			diffs = append(diffs, Diff{Key: k, Before: v1, After: nil})
		} else if fmt.Sprintf("%v", v1) != fmt.Sprintf("%v", v2) {
			diffs = append(diffs, Diff{Key: k, Before: v1, After: v2})
		}
	}

	// Check keys only in s2
	for k, v2 := range s2.State {
		if _, exists := s1.State[k]; !exists {
			diffs = append(diffs, Diff{Key: k, Before: nil, After: v2})
		}
	}

	return diffs
}

// EventCount returns the number of recorded events.
func (r *Recorder) EventCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.events)
}
