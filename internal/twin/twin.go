package twin

import (
	"fmt"
	"strconv"
	"sync"
	"time"
)

// State represents the observed state of a service.
type State struct {
	Service      string
	CPU          float64  // 0-100
	Memory       float64  // 0-100
	Replicas     int
	Healthy      bool
	Latency      float64  // ms
	ErrorRate    float64  // 0-1
	Dependencies []string // service names this one depends on

	// Extended fields populated by ObserveEvent.
	LastHealthCheck time.Time
	LastSeen        time.Time
	ErrorCount      int64 // running count since Twin start
}

// Action is a single step in a healing plan.
type Action struct {
	Type   string            // "restart", "scale", "failover", "rollback", "noop"
	Target string            // service name
	Params map[string]string
}

// Plan is a sequence of Actions.
type Plan struct {
	ID      string
	Actions []Action
}

// EffectModel predicts how State changes after applying Action.
// If it cannot model this action type, returns (state, false).
type EffectModel func(s State, a Action) (next State, modeled bool)

// ScoreFunc rates a state: higher = better. Used to compare plan outcomes.
type ScoreFunc func(states map[string]State) float64

// Simulation result.
type Simulation struct {
	PlanID         string
	Start          map[string]State   // deep copy of twin state before
	End            map[string]State   // state after applying plan steps
	StepStates     []map[string]State // snapshot after each action
	StartScore     float64
	EndScore       float64
	Improvement    float64 // EndScore - StartScore
	Accepted       bool    // EndScore >= StartScore (or within tolerance)
	Rejected       bool    // EndScore < StartScore - tolerance
	Reason         string
	ModeledSteps   int
	UnmodeledSteps int
}

// Config controls Twin behaviour.
type Config struct {
	Tolerance    float64       // how much score degradation is acceptable (default 0.0)
	Score        ScoreFunc     // defaults to DefaultScore
	EffectModels []EffectModel // defaults to built-ins
}

// Twin is a thread-safe shadow state machine for service states.
type Twin struct {
	mu     sync.RWMutex
	states map[string]State
	cfg    Config
}

// New creates a new Twin with the given configuration.
// Zero-value Config fields are filled with defaults.
func New(cfg Config) *Twin {
	if cfg.Score == nil {
		cfg.Score = DefaultScore
	}
	if len(cfg.EffectModels) == 0 {
		cfg.EffectModels = BuiltinEffectModels()
	}
	return &Twin{
		states: make(map[string]State),
		cfg:    cfg,
	}
}

// Observe upserts state for s.Service.
func (t *Twin) Observe(s State) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.states[s.Service] = s
}

// States returns a deep copy snapshot of all service states.
func (t *Twin) States() map[string]State {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return copyStates(t.states)
}

// Get returns the state for a given service name.
func (t *Twin) Get(service string) (State, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	s, ok := t.states[service]
	return s, ok
}

// ApplyObserved mutates the state for service via delta function.
func (t *Twin) ApplyObserved(service string, delta func(s *State)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.states[service]
	delta(&s)
	s.Service = service
	t.states[service] = s
}

// Simulate runs the plan against a copy of the twin state and returns a Simulation.
// It does NOT mutate the twin's internal state.
func (t *Twin) Simulate(p Plan) Simulation {
	t.mu.RLock()
	start := copyStates(t.states)
	cfg := t.cfg
	t.mu.RUnlock()

	current := copyStates(start)
	startScore := cfg.Score(current)

	sim := Simulation{
		PlanID:     p.ID,
		Start:      copyStates(start),
		StartScore: startScore,
	}

	for _, action := range p.Actions {
		modeled := false

		// For traffic_shift, apply the model to both from and to services.
		if action.Type == "traffic_shift" {
			fromSvc := action.Params["from"]
			toSvc := action.Params["to"]
			for _, svcName := range []string{fromSvc, toSvc} {
				if svcName == "" {
					continue
				}
				svcAction := action
				svcAction.Target = svcName
				svc, exists := current[svcName]
				if !exists {
					svc = State{Service: svcName}
				}
				for _, model := range cfg.EffectModels {
					next, ok := model(svc, svcAction)
					if ok {
						next.Service = svcName
						current[svcName] = next
						modeled = true
						break
					}
				}
			}
			if modeled {
				sim.ModeledSteps++
			} else {
				sim.UnmodeledSteps++
			}
			sim.StepStates = append(sim.StepStates, copyStates(current))
			continue
		}

		for _, model := range cfg.EffectModels {
			target, exists := current[action.Target]
			if !exists {
				// create a zero-value state for unknown services
				target = State{Service: action.Target}
			}
			prevHealthy := target.Healthy
			next, ok := model(target, action)
			if ok {
				next.Service = action.Target
				current[action.Target] = next

				// For canary actions, also register the canary sub-state.
				if action.Type == "canary" {
					if pctStr, hasPct := action.Params["percent"]; hasPct {
						if pct, err := strconv.ParseFloat(pctStr, 64); err == nil {
							cs := canaryStateFor(target, pct)
							current[cs.Service] = cs
						}
					}
				}

				// Only propagate dependency recovery when the action actually
				// restored an unhealthy target — restarting a healthy service
				// is a disturbance, not a benefit to its dependents.
				if !prevHealthy && next.Healthy {
					current = applyDependencyEffects(current, action.Target, action.Type)
				}
				modeled = true
				break
			}
		}
		if modeled {
			sim.ModeledSteps++
		} else {
			sim.UnmodeledSteps++
		}
		sim.StepStates = append(sim.StepStates, copyStates(current))
	}

	endScore := cfg.Score(current)
	sim.End = copyStates(current)
	sim.EndScore = endScore
	sim.Improvement = endScore - startScore

	threshold := startScore - cfg.Tolerance
	if endScore >= threshold {
		sim.Accepted = true
		sim.Reason = fmt.Sprintf("score improved by %.4f (%.4f → %.4f)", sim.Improvement, startScore, endScore)
	} else {
		sim.Rejected = true
		sim.Reason = fmt.Sprintf("score worsened by %.4f (%.4f → %.4f), below tolerance threshold %.4f",
			-sim.Improvement, startScore, endScore, threshold)
	}

	return sim
}

// DefaultScore computes a composite health score across all service states.
// Higher is better.
//
//	per service: (Healthy ? 10 : -20) * Replicas - ErrorRate*50 - Latency*0.01 - (CPU>90 ? 5 : 0)
func DefaultScore(states map[string]State) float64 {
	score := 0.0
	for _, s := range states {
		replicas := s.Replicas
		if replicas == 0 {
			replicas = 1
		}
		if s.Healthy {
			score += 10 * float64(replicas)
		} else {
			score += -20 * float64(replicas)
		}
		score -= s.ErrorRate * 50
		score -= s.Latency * 0.01
		if s.CPU > 90 {
			score -= 5
		}
	}
	return score
}

// copyStates returns a deep copy of a states map.
func copyStates(src map[string]State) map[string]State {
	dst := make(map[string]State, len(src))
	for k, v := range src {
		// copy the Dependencies slice
		if v.Dependencies != nil {
			deps := make([]string, len(v.Dependencies))
			copy(deps, v.Dependencies)
			v.Dependencies = deps
		}
		dst[k] = v
	}
	return dst
}

// applyDependencyEffects propagates recovery signals to services that depend on target.
// Called after restart or failover actions.
func applyDependencyEffects(states map[string]State, target, actionType string) map[string]State {
	if actionType != "restart" && actionType != "failover" {
		return states
	}
	for svc, s := range states {
		if svc == target {
			continue
		}
		for _, dep := range s.Dependencies {
			if dep == target {
				s.Latency *= 0.7
				s.ErrorRate *= 0.5
				if s.Latency < 0 {
					s.Latency = 0
				}
				if s.ErrorRate < 0 {
					s.ErrorRate = 0
				}
				states[svc] = s
				break
			}
		}
	}
	return states
}
