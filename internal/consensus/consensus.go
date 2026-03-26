package consensus

import (
	"sync"

	"github.com/immortal-engine/immortal/internal/event"
)

// VerifyFunc determines if a verifier agrees that action should be taken.
type VerifyFunc func(e *event.Event) bool

// Config controls consensus behavior.
type Config struct {
	MinAgreement int // Minimum number of verifiers that must agree
}

// Result captures the outcome of a consensus evaluation.
type Result struct {
	Approved   bool     `json:"approved"`
	Votes      int      `json:"votes"`
	Total      int      `json:"total"`
	Voters     []string `json:"voters"`     // Names of verifiers that agreed
	Dissenters []string `json:"dissenters"` // Names that disagreed
}

// Engine runs multiple verification strategies and requires consensus.
type Engine struct {
	mu        sync.RWMutex
	config    Config
	verifiers map[string]VerifyFunc
	order     []string
}

// New creates a new consensus engine.
func New(config Config) *Engine {
	if config.MinAgreement < 1 {
		config.MinAgreement = 1
	}
	return &Engine{
		config:    config,
		verifiers: make(map[string]VerifyFunc),
	}
}

// AddVerifier registers a named verification function.
func (e *Engine) AddVerifier(name string, fn VerifyFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.verifiers[name] = fn
	e.order = append(e.order, name)
}

// Evaluate runs all verifiers and determines if consensus is reached.
func (e *Engine) Evaluate(ev *event.Event) Result {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := Result{
		Total: len(e.verifiers),
	}

	for _, name := range e.order {
		fn := e.verifiers[name]
		if fn(ev) {
			result.Votes++
			result.Voters = append(result.Voters, name)
		} else {
			result.Dissenters = append(result.Dissenters, name)
		}
	}

	result.Approved = result.Votes >= e.config.MinAgreement
	return result
}

// VerifierCount returns the number of registered verifiers.
func (e *Engine) VerifierCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.verifiers)
}
