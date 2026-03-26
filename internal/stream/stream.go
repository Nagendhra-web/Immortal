package stream

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// LogEntry represents a single log line from Immortal's activity.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`     // info, warn, error, heal, detect, ghost, alert
	Component string    `json:"component"` // engine, healer, collector, firewall, dna, etc.
	Message   string    `json:"message"`
	Source    string    `json:"source,omitempty"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
}

// Subscriber receives log entries in real-time.
type Subscriber struct {
	ID     string
	Ch     chan LogEntry
	Filter func(LogEntry) bool // nil = receive all
}

// Stream is a real-time log broadcasting system.
// Multiple subscribers can watch Immortal's activity live.
type Stream struct {
	mu          sync.RWMutex
	subscribers map[string]*Subscriber
	history     []LogEntry
	maxHistory  int
}

// New creates a new log stream.
func New(maxHistory int) *Stream {
	if maxHistory <= 0 {
		maxHistory = 1000
	}
	return &Stream{
		subscribers: make(map[string]*Subscriber),
		maxHistory:  maxHistory,
	}
}

// Emit broadcasts a log entry to all subscribers and stores in history.
func (s *Stream) Emit(entry LogEntry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Store in history
	s.mu.Lock()
	s.history = append(s.history, entry)
	if len(s.history) > s.maxHistory {
		s.history = s.history[len(s.history)-s.maxHistory:]
	}

	// Copy subscribers for safe iteration
	subs := make([]*Subscriber, 0, len(s.subscribers))
	for _, sub := range s.subscribers {
		subs = append(subs, sub)
	}
	s.mu.Unlock()

	// Broadcast to subscribers (non-blocking)
	for _, sub := range subs {
		if sub.Filter != nil && !sub.Filter(entry) {
			continue
		}
		select {
		case sub.Ch <- entry:
		default:
			// Subscriber too slow — drop this entry for them
		}
	}
}

// Subscribe creates a new subscriber. Returns the subscriber and a cleanup function.
func (s *Stream) Subscribe(id string, bufferSize int, filter func(LogEntry) bool) (*Subscriber, func()) {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	sub := &Subscriber{
		ID:     id,
		Ch:     make(chan LogEntry, bufferSize),
		Filter: filter,
	}

	s.mu.Lock()
	s.subscribers[id] = sub
	s.mu.Unlock()

	cleanup := func() {
		s.mu.Lock()
		delete(s.subscribers, id)
		s.mu.Unlock()
		close(sub.Ch)
	}

	return sub, cleanup
}

// History returns recent log entries.
func (s *Stream) History(limit int) []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}
	start := len(s.history) - limit
	if start < 0 {
		start = 0
	}
	out := make([]LogEntry, limit)
	copy(out, s.history[start:])
	return out
}

// SubscriberCount returns number of active subscribers.
func (s *Stream) SubscriberCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.subscribers)
}

// Convenience emitters

func (s *Stream) Info(component, message string) {
	s.Emit(LogEntry{Level: "info", Component: component, Message: message})
}

func (s *Stream) Warn(component, message string) {
	s.Emit(LogEntry{Level: "warn", Component: component, Message: message})
}

func (s *Stream) Error(component, message string) {
	s.Emit(LogEntry{Level: "error", Component: component, Message: message})
}

func (s *Stream) Heal(source, message string) {
	s.Emit(LogEntry{Level: "heal", Component: "healer", Message: message, Source: source})
}

func (s *Stream) Detect(component, source, message string) {
	s.Emit(LogEntry{Level: "detect", Component: component, Message: message, Source: source})
}

func (s *Stream) Ghost(message string) {
	s.Emit(LogEntry{Level: "ghost", Component: "engine", Message: message})
}

func (s *Stream) Alert(source, message string) {
	s.Emit(LogEntry{Level: "alert", Component: "alert", Message: message, Source: source})
}

func (s *Stream) Metric(source string, metrics map[string]interface{}) {
	s.Emit(LogEntry{Level: "info", Component: "metrics", Message: "metrics collected", Source: source, Meta: metrics})
}

// FormatCLI formats a log entry for terminal display.
func FormatCLI(e LogEntry) string {
	ts := e.Timestamp.Format("15:04:05")
	icon := levelIcon(e.Level)
	color := levelColor(e.Level)

	msg := fmt.Sprintf("%s %s%s %s│%s %s",
		ts, color, icon, e.Level, "\033[0m", e.Message)

	if e.Source != "" {
		msg += fmt.Sprintf(" \033[90m(%s)\033[0m", e.Source)
	}
	return msg
}

// FormatJSON formats a log entry as JSON string.
func FormatJSON(e LogEntry) string {
	data, _ := json.Marshal(e)
	return string(data)
}

func levelIcon(level string) string {
	switch level {
	case "heal":
		return "[FIX]"
	case "detect":
		return "[!!!]"
	case "ghost":
		return "[EYE]"
	case "alert":
		return "[ALR]"
	case "error":
		return "[ERR]"
	case "warn":
		return "[WRN]"
	default:
		return "[   ]"
	}
}

func levelColor(level string) string {
	switch level {
	case "heal":
		return "\033[32m" // green
	case "detect":
		return "\033[31m" // red
	case "ghost":
		return "\033[36m" // cyan
	case "alert":
		return "\033[33m" // yellow
	case "error":
		return "\033[31m" // red
	case "warn":
		return "\033[33m" // yellow
	default:
		return "\033[37m" // white
	}
}
