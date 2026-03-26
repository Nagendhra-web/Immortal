package sandbox

import (
	"fmt"
	"sync"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
)

// Result captures the outcome of a sandbox test.
type Result struct {
	EventID   string        `json:"event_id"`
	Safe      bool          `json:"safe"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
}

// ActionFunc is a healing action to test.
type ActionFunc func(e *event.Event) error

// Sandbox tests healing actions in isolation before applying to production.
type Sandbox struct {
	mu      sync.Mutex
	history []Result
}

// New creates a new simulation sandbox.
func New() *Sandbox {
	return &Sandbox{}
}

// Test runs a healing action in the sandbox and reports whether it's safe.
// Catches panics and errors.
func (s *Sandbox) Test(e *event.Event, action ActionFunc) Result {
	start := time.Now()

	result := Result{
		EventID:   e.ID,
		Timestamp: start,
	}

	// Run in protected context
	func() {
		defer func() {
			if r := recover(); r != nil {
				result.Safe = false
				result.Error = fmt.Sprintf("panic: %v", r)
			}
		}()

		err := action(e)
		if err != nil {
			result.Safe = false
			result.Error = err.Error()
		} else {
			result.Safe = true
		}
	}()

	result.Duration = time.Since(start)

	s.mu.Lock()
	s.history = append(s.history, result)
	s.mu.Unlock()

	return result
}

// History returns all sandbox test results.
func (s *Sandbox) History() []Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Result, len(s.history))
	copy(out, s.history)
	return out
}

// SuccessRate returns the percentage of sandbox tests that were safe.
func (s *Sandbox) SuccessRate() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.history) == 0 {
		return 1.0
	}

	safe := 0
	for _, r := range s.history {
		if r.Safe {
			safe++
		}
	}
	return float64(safe) / float64(len(s.history))
}
