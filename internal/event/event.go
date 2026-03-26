package event

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Type categorizes events.
type Type string

const (
	TypeError  Type = "error"
	TypeMetric Type = "metric"
	TypeLog    Type = "log"
	TypeTrace  Type = "trace"
	TypeHealth Type = "health"
)

// Event is the universal event format for Immortal.
// Every signal (log, metric, error, trace) is normalized into this format.
type Event struct {
	ID        string                 `json:"id"`
	Type      Type                   `json:"type"`
	Severity  Severity               `json:"severity"`
	Message   string                 `json:"message"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
	mu        sync.RWMutex
}

// New creates a new Event with a unique ID and current timestamp.
func New(typ Type, severity Severity, message string) *Event {
	return &Event{
		ID:        generateID(),
		Type:      typ,
		Severity:  severity,
		Message:   message,
		Timestamp: time.Now(),
		Meta:      make(map[string]interface{}),
	}
}

func (e *Event) WithSource(source string) *Event {
	e.mu.Lock()
	e.Source = source
	e.mu.Unlock()
	return e
}

func (e *Event) WithMeta(key string, value interface{}) *Event {
	e.mu.Lock()
	e.Meta[key] = value
	e.mu.Unlock()
	return e
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
