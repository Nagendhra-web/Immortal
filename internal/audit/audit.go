package audit

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Entry represents a single immutable audit log record.
type Entry struct {
	ID        string        `json:"id"`
	Timestamp time.Time     `json:"timestamp"`
	Action    string        `json:"action"`
	Actor     string        `json:"actor"`
	Target    string        `json:"target"`
	Detail    string        `json:"detail"`
	Success   bool          `json:"success"`
	Duration  time.Duration `json:"duration"`
}

// Logger is a thread-safe immutable audit log.
type Logger struct {
	mu         sync.RWMutex
	entries    []Entry
	maxEntries int
	counter    uint64
}

// New creates a new Logger. If maxEntries <= 0, defaults to 10000.
func New(maxEntries int) *Logger {
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	return &Logger{
		entries:    make([]Entry, 0),
		maxEntries: maxEntries,
	}
}

// Log records an audit entry and returns it.
func (l *Logger) Log(action, actor, target, detail string, success bool) *Entry {
	return l.LogWithDuration(action, actor, target, detail, success, 0)
}

// LogWithDuration records an audit entry with an explicit duration and returns it.
func (l *Logger) LogWithDuration(action, actor, target, detail string, success bool, duration time.Duration) *Entry {
	id := fmt.Sprintf("audit-%d", atomic.AddUint64(&l.counter, 1))
	e := Entry{
		ID:        id,
		Timestamp: time.Now(),
		Action:    action,
		Actor:     actor,
		Target:    target,
		Detail:    detail,
		Success:   success,
		Duration:  duration,
	}
	l.mu.Lock()
	l.entries = append(l.entries, e)
	if len(l.entries) > l.maxEntries {
		l.entries = l.entries[len(l.entries)-l.maxEntries:]
	}
	l.mu.Unlock()
	return &e
}

// snapshot returns a safe copy of all entries under read lock.
func (l *Logger) snapshot() []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	cp := make([]Entry, len(l.entries))
	copy(cp, l.entries)
	return cp
}

// Entries returns up to limit entries, newest first. If limit <= 0, returns all.
func (l *Logger) Entries(limit int) []Entry {
	result := reversed(l.snapshot())
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return result
}

// EntriesByAction returns all entries matching action, newest first.
func (l *Logger) EntriesByAction(action string) []Entry {
	src := l.snapshot()
	var filtered []Entry
	for _, e := range src {
		if e.Action == action {
			filtered = append(filtered, e)
		}
	}
	return reversed(filtered)
}

// EntriesByActor returns all entries matching actor, newest first.
func (l *Logger) EntriesByActor(actor string) []Entry {
	src := l.snapshot()
	var filtered []Entry
	for _, e := range src {
		if e.Actor == actor {
			filtered = append(filtered, e)
		}
	}
	return reversed(filtered)
}

// EntriesByTarget returns all entries matching target, newest first.
func (l *Logger) EntriesByTarget(target string) []Entry {
	src := l.snapshot()
	var filtered []Entry
	for _, e := range src {
		if e.Target == target {
			filtered = append(filtered, e)
		}
	}
	return reversed(filtered)
}

// Count returns the total number of stored entries.
func (l *Logger) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// Since returns entries recorded at or after t, newest first.
func (l *Logger) Since(t time.Time) []Entry {
	src := l.snapshot()
	var filtered []Entry
	for _, e := range src {
		if !e.Timestamp.Before(t) {
			filtered = append(filtered, e)
		}
	}
	return reversed(filtered)
}

// Search returns entries where action, actor, target, or detail contains query
// (case-insensitive), newest first.
func (l *Logger) Search(query string) []Entry {
	src := l.snapshot()
	q := strings.ToLower(query)
	var filtered []Entry
	for _, e := range src {
		if strings.Contains(strings.ToLower(e.Action), q) ||
			strings.Contains(strings.ToLower(e.Actor), q) ||
			strings.Contains(strings.ToLower(e.Target), q) ||
			strings.Contains(strings.ToLower(e.Detail), q) {
			filtered = append(filtered, e)
		}
	}
	return reversed(filtered)
}

// reversed returns a copy of the slice in reverse order.
func reversed(src []Entry) []Entry {
	n := len(src)
	out := make([]Entry, n)
	for i, e := range src {
		out[n-1-i] = e
	}
	return out
}
