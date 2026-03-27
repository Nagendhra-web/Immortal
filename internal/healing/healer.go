package healing

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
)

const defaultActionTimeout = 30 * time.Second

// HealPolicy controls when and how healing actions fire.
type HealPolicy struct {
	ConsecutiveFailures int           // How many consecutive failures before healing (default: 3)
	MaxRetries          int           // Max heal attempts before giving up (default: 3)
	InitialBackoff      time.Duration // First retry delay (default: 10s)
	MaxBackoff          time.Duration // Max retry delay (default: 5m)
	CooldownAfterHeal   time.Duration // Wait after healing before monitoring again (default: 30s)
}

// DefaultPolicy returns backward-compatible defaults (heal on first match).
// Use ProductionPolicy() for real deployments.
func DefaultPolicy() HealPolicy {
	return HealPolicy{
		ConsecutiveFailures: 1,
		MaxRetries:          100,
		InitialBackoff:      0,
		MaxBackoff:          5 * time.Minute,
		CooldownAfterHeal:   0,
	}
}

// ProductionPolicy returns production-grade defaults (requires consecutive failures).
func ProductionPolicy() HealPolicy {
	return HealPolicy{
		ConsecutiveFailures: 3,
		MaxRetries:          3,
		InitialBackoff:      10 * time.Second,
		MaxBackoff:          5 * time.Minute,
		CooldownAfterHeal:   30 * time.Second,
	}
}

// sourceState tracks consecutive failures and heal attempts per source.
type sourceState struct {
	consecutiveFailures int
	healAttempts        int
	lastHealTime        time.Time
	lastFailureTime     time.Time
	inCooldown          bool
}

type Healer struct {
	mu        sync.RWMutex
	rules     []Rule
	ghostMode bool
	history   []HealRecord
	policy    HealPolicy
	sources   map[string]*sourceState // per-source failure tracking
}

type HealRecord struct {
	RuleName  string    `json:"rule_name"`
	EventID   string    `json:"event_id"`
	Source    string    `json:"source"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	Attempt   int       `json:"attempt"`
	Timestamp time.Time `json:"timestamp"`
}

func NewHealer() *Healer {
	return NewHealerWithPolicy(DefaultPolicy())
}

func NewHealerWithPolicy(policy HealPolicy) *Healer {
	if policy.ConsecutiveFailures <= 0 {
		policy.ConsecutiveFailures = 1
	}
	if policy.MaxRetries <= 0 {
		policy.MaxRetries = 3
	}
	if policy.InitialBackoff <= 0 {
		policy.InitialBackoff = 10 * time.Second
	}
	if policy.MaxBackoff <= 0 {
		policy.MaxBackoff = 5 * time.Minute
	}
	return &Healer{
		policy:  policy,
		sources: make(map[string]*sourceState),
	}
}

func (h *Healer) AddRule(rule Rule) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.rules = append(h.rules, rule)
}

func (h *Healer) SetGhostMode(enabled bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ghostMode = enabled
}

func (h *Healer) Policy() HealPolicy {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.policy
}

func (h *Healer) Handle(e *event.Event) []Recommendation {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var recommendations []Recommendation

	for _, rule := range h.rules {
		if rule.Match(e) {
			rec := Recommendation{
				RuleName: rule.Name,
				Event:    e,
				Message:  fmt.Sprintf("rule '%s' matched event: %s", rule.Name, e.Message),
			}
			recommendations = append(recommendations, rec)

			if !h.ghostMode {
				go h.executeWithPolicy(rule, e)
			}
		}
	}

	return recommendations
}

func (h *Healer) executeWithPolicy(r Rule, e *event.Event) {
	source := e.Source
	if source == "" {
		source = "_default"
	}

	h.mu.Lock()
	state, ok := h.sources[source]
	if !ok {
		state = &sourceState{}
		h.sources[source] = state
	}

	// Track consecutive failure
	if e.Severity.Level() >= event.SeverityError.Level() {
		state.consecutiveFailures++
		state.lastFailureTime = time.Now()
	}

	// Check if in cooldown after a recent heal
	if state.inCooldown && time.Since(state.lastHealTime) < h.policy.CooldownAfterHeal {
		h.mu.Unlock()
		log.Printf("[immortal] source '%s' in cooldown (%.0fs remaining), skipping heal",
			source, (h.policy.CooldownAfterHeal - time.Since(state.lastHealTime)).Seconds())
		return
	}
	state.inCooldown = false

	// Check consecutive failure threshold
	if state.consecutiveFailures < h.policy.ConsecutiveFailures {
		h.mu.Unlock()
		log.Printf("[immortal] source '%s' has %d/%d consecutive failures, not healing yet",
			source, state.consecutiveFailures, h.policy.ConsecutiveFailures)
		return
	}

	// Check max retries (give up and alert human)
	if state.healAttempts >= h.policy.MaxRetries {
		h.mu.Unlock()
		log.Printf("[immortal] source '%s' exceeded max heal attempts (%d), giving up — ALERT HUMAN",
			source, h.policy.MaxRetries)
		h.recordHeal(r.Name, e, false, fmt.Sprintf("gave up after %d attempts", state.healAttempts), state.healAttempts)
		return
	}

	// Calculate backoff delay
	attempt := state.healAttempts
	backoffDuration := time.Duration(0)
	if attempt > 0 {
		backoff := float64(h.policy.InitialBackoff) * math.Pow(2, float64(attempt-1))
		if backoff > float64(h.policy.MaxBackoff) {
			backoff = float64(h.policy.MaxBackoff)
		}
		backoffDuration = time.Duration(backoff)

		timeSinceLastHeal := time.Since(state.lastHealTime)
		if timeSinceLastHeal < backoffDuration {
			h.mu.Unlock()
			log.Printf("[immortal] source '%s' backing off (attempt %d, wait %.0fs)",
				source, attempt+1, (backoffDuration - timeSinceLastHeal).Seconds())
			return
		}
	}

	// Execute heal
	state.healAttempts++
	state.lastHealTime = time.Now()
	state.inCooldown = true
	state.consecutiveFailures = 0 // reset after heal attempt
	currentAttempt := state.healAttempts
	h.mu.Unlock()

	log.Printf("[immortal] healing source '%s' (attempt %d/%d)", source, currentAttempt, h.policy.MaxRetries)

	// Run action with timeout + panic recovery
	done := make(chan error, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				done <- fmt.Errorf("panic: %v", p)
			}
		}()
		done <- r.Action(e)
	}()

	var healErr error
	select {
	case err := <-done:
		healErr = err
	case <-time.After(defaultActionTimeout):
		healErr = fmt.Errorf("timeout after %s", defaultActionTimeout)
	}

	success := healErr == nil
	errMsg := ""
	if healErr != nil {
		errMsg = healErr.Error()
		log.Printf("[immortal] healing action '%s' failed (attempt %d): %v", r.Name, currentAttempt, healErr)
	} else {
		log.Printf("[immortal] healing action '%s' succeeded (attempt %d)", r.Name, currentAttempt)
		// Reset attempts on success
		h.mu.Lock()
		if s, ok := h.sources[source]; ok {
			s.healAttempts = 0
		}
		h.mu.Unlock()
	}

	h.recordHeal(r.Name, e, success, errMsg, currentAttempt)
}

func (h *Healer) recordHeal(ruleName string, e *event.Event, success bool, errMsg string, attempt int) {
	record := HealRecord{
		RuleName:  ruleName,
		EventID:   e.ID,
		Source:    e.Source,
		Success:   success,
		Error:     errMsg,
		Attempt:   attempt,
		Timestamp: time.Now(),
	}

	h.mu.Lock()
	h.history = append(h.history, record)
	if len(h.history) > 10000 {
		h.history = h.history[len(h.history)-5000:]
	}
	h.mu.Unlock()
}

// ResetSource resets failure tracking for a source (e.g., after manual recovery).
func (h *Healer) ResetSource(source string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sources, source)
}

// SourceState returns the current failure state for a source.
func (h *Healer) SourceState(source string) (consecutiveFailures, healAttempts int, inCooldown bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if s, ok := h.sources[source]; ok {
		return s.consecutiveFailures, s.healAttempts, s.inCooldown
	}
	return 0, 0, false
}

func (h *Healer) History() []HealRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]HealRecord, len(h.history))
	copy(out, h.history)
	return out
}
