package agentic

import (
	"strings"
	"sync"
	"time"
)

// Outcome describes how an incident run ended.
type Outcome string

const (
	OutcomeResolved  Outcome = "resolved"
	OutcomeFailed    Outcome = "failed"
	OutcomeEscalated Outcome = "escalated"
)

// Incident is a distilled summary of an incident event (no pointer dependencies).
type Incident struct {
	Message  string
	Source   string
	Severity string
}

// MemoryEntry holds one recorded incident run.
type MemoryEntry struct {
	Timestamp time.Time
	Incident  Incident
	Trace     []Step
	Outcome   Outcome
}

// Memory is a fixed-capacity ring buffer of past incident traces.
// Safe for concurrent use.
type Memory struct {
	mu       sync.RWMutex
	capacity int
	entries  []MemoryEntry
	head     int // next write position
	size     int // current number of entries
}

// NewMemory returns a Memory with the given capacity (minimum 1).
func NewMemory(capacity int) *Memory {
	if capacity <= 0 {
		capacity = 100
	}
	return &Memory{
		capacity: capacity,
		entries:  make([]MemoryEntry, capacity),
	}
}

// Record stores a new incident trace in the ring buffer.
// If the buffer is full the oldest entry is overwritten.
func (m *Memory) Record(incident Incident, trace *Trace, outcome Outcome) {
	m.mu.Lock()
	defer m.mu.Unlock()

	steps := make([]Step, len(trace.Steps))
	copy(steps, trace.Steps)

	m.entries[m.head] = MemoryEntry{
		Timestamp: time.Now(),
		Incident:  incident,
		Trace:     steps,
		Outcome:   outcome,
	}
	m.head = (m.head + 1) % m.capacity
	if m.size < m.capacity {
		m.size++
	}
}

// Size returns the number of entries currently stored.
func (m *Memory) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.size
}

// Recall returns up to k entries ranked by similarity to the query incident.
// Similarity is a simple heuristic: exact source match (2 pts), exact severity
// match (1 pt), shared words in message (1 pt each). Ties broken by recency.
func (m *Memory) Recall(incident Incident, k int) []MemoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.size == 0 || k <= 0 {
		return nil
	}

	queryWords := tokenize(incident.Message)

	results := make([]scoredEntry, 0, m.size)
	for i := 0; i < m.size; i++ {
		// Walk entries in insertion order: oldest first.
		// The ring starts at head-size (mod capacity) for the oldest.
		pos := ((m.head - m.size + i) + m.capacity) % m.capacity
		e := m.entries[pos]

		score := 0
		if e.Incident.Source == incident.Source {
			score += 2
		}
		if e.Incident.Severity == incident.Severity {
			score += 1
		}
		entryWords := tokenize(e.Incident.Message)
		for w := range queryWords {
			if entryWords[w] {
				score++
			}
		}
		results = append(results, scoredEntry{entry: e, score: score, idx: i})
	}

	// Sort descending by score, then descending by idx (newer = higher idx).
	sortScored(results)

	if k > len(results) {
		k = len(results)
	}
	out := make([]MemoryEntry, k)
	for i := 0; i < k; i++ {
		out[i] = results[i].entry
	}
	return out
}

// scoredEntry is used internally by Recall for ranking.
type scoredEntry struct {
	entry MemoryEntry
	score int
	idx   int // insertion order for tie-breaking (higher = newer)
}

// tokenize splits a string into a lowercase word set.
func tokenize(s string) map[string]bool {
	words := strings.Fields(strings.ToLower(s))
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// sortScored performs an insertion sort (small N) descending by score then idx.
func sortScored(s []scoredEntry) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && (s[j].score < key.score || (s[j].score == key.score && s[j].idx < key.idx)) {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}
