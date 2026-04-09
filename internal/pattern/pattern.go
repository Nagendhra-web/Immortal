package pattern

import (
	"sort"
	"sync"
	"time"
)

// Pattern describes a recurring failure pattern detected within a time window.
type Pattern struct {
	Key       string        `json:"key"`
	Count     int           `json:"count"`
	Window    time.Duration `json:"window"`
	FirstSeen time.Time     `json:"first_seen"`
	LastSeen  time.Time     `json:"last_seen"`
	Severity  string        `json:"severity"`
}

// entry is an unexported timestamped record of a single occurrence.
type entry struct {
	key       string
	timestamp time.Time
	severity  string
}

// Detector tracks occurrences by key within a sliding time window and surfaces
// recurring patterns once a configurable threshold is reached.
type Detector struct {
	mu         sync.RWMutex
	entries    []entry
	windowSize time.Duration
	threshold  int
	maxEntries int
}

// New creates a Detector with the given window size and threshold.
func New(windowSize time.Duration, threshold int) *Detector {
	return &Detector{
		windowSize: windowSize,
		threshold:  threshold,
		maxEntries: 50000,
	}
}

// Record adds an occurrence of key with the given severity and prunes stale entries.
func (d *Detector) Record(key string, severity string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.entries = append(d.entries, entry{key: key, timestamp: time.Now(), severity: severity})
	if len(d.entries) > d.maxEntries {
		d.entries = d.entries[len(d.entries)-d.maxEntries:]
	}
	d.prune()
}

// Patterns returns all keys whose count within the window meets or exceeds the
// threshold, sorted by count descending.
func (d *Detector) Patterns() []Pattern {
	d.mu.RLock()
	defer d.mu.RUnlock()

	type agg struct {
		count     int
		firstSeen time.Time
		lastSeen  time.Time
		severity  string
	}

	now := time.Now()
	cutoff := now.Add(-d.windowSize)
	m := make(map[string]*agg)

	for _, e := range d.entries {
		if e.timestamp.Before(cutoff) {
			continue
		}
		a, ok := m[e.key]
		if !ok {
			m[e.key] = &agg{count: 1, firstSeen: e.timestamp, lastSeen: e.timestamp, severity: e.severity}
			continue
		}
		a.count++
		if e.timestamp.Before(a.firstSeen) {
			a.firstSeen = e.timestamp
		}
		if e.timestamp.After(a.lastSeen) {
			a.lastSeen = e.timestamp
			a.severity = e.severity
		}
	}

	var out []Pattern
	for key, a := range m {
		if a.count >= d.threshold {
			out = append(out, Pattern{
				Key:       key,
				Count:     a.count,
				Window:    d.windowSize,
				FirstSeen: a.firstSeen,
				LastSeen:  a.lastSeen,
				Severity:  a.severity,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Count > out[j].Count
	})
	return out
}

// IsRepeating returns true if key has appeared at least threshold times within
// the current window.
func (d *Detector) IsRepeating(key string) bool {
	return d.Count(key) >= d.threshold
}

// Count returns the number of times key has appeared within the current window.
func (d *Detector) Count(key string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	cutoff := time.Now().Add(-d.windowSize)
	n := 0
	for _, e := range d.entries {
		if e.key == key && !e.timestamp.Before(cutoff) {
			n++
		}
	}
	return n
}

// Reset clears all recorded entries.
func (d *Detector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.entries = d.entries[:0]
}

// prune removes entries older than the window. Must be called with mu held (write).
func (d *Detector) prune() {
	cutoff := time.Now().Add(-d.windowSize)
	i := 0
	for _, e := range d.entries {
		if !e.timestamp.Before(cutoff) {
			d.entries[i] = e
			i++
		}
	}
	d.entries = d.entries[:i]
}
