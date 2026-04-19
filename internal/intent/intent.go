// Package intent implements intent-based healing: operators declare goals
// (for example "keep API latency under 200 ms", "never drop jobs", "protect
// checkout flow at all costs") and the engine compiles those goals into
// concrete actions whenever they are at risk.
//
// Intent replaces "if X then Y" healing rules with "achieve outcome Z" and
// lets the engine pick the cheapest set of actions that keep all goals met.
package intent

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ConstraintKind defines the shape of a goal.
type ConstraintKind int

const (
	LatencyUnder    ConstraintKind = iota // service latency metric must stay under Target ms
	ErrorRateUnder                        // error rate must stay under Target (0.0-1.0)
	AvailabilityOver                      // availability must stay over Target (0.0-1.0)
	JobsNoDrop                            // boolean invariant: queue depth cannot exceed capacity
	ProtectService                        // service is business-critical and must be preserved first
	CostCap                               // aggregate $/hour must stay under Target
	Saturation                            // resource saturation must stay under Target (0.0-1.0)
)

// Goal is a single declarative objective.
type Goal struct {
	Kind     ConstraintKind
	Service  string  // empty = applies to the whole system
	Metric   string  // the metric this goal reads (e.g. "latency_p99")
	Target   float64 // numeric threshold; unit depends on Kind
	Priority int     // 0 = lowest, 10 = highest; used to resolve conflicts
}

// Intent is a named collection of goals.
type Intent struct {
	Name  string
	Goals []Goal
}

// Status is the evaluation result for a single goal.
type Status struct {
	Goal      Goal
	Current   float64
	Target    float64
	Met       bool
	AtRisk    bool    // true when Current is within 20% of Target
	Slack     float64 // distance between Current and Target in the "safe" direction
	CheckedAt time.Time
}

// Suggestion is a compiled action that would restore a goal.
type Suggestion struct {
	Intent    string
	Goal      Goal
	Action    string             // e.g. "scale:rest:+2", "shed_load:non_critical", "warm_cache:catalog"
	Rationale string             // human-readable why
	Priority  int                // inherits from Goal.Priority
	Impact    map[string]float64 // predicted metric deltas if applied
}

// MetricProvider returns the current value of a metric for a service.
// Pass empty service for system-wide metrics.
type MetricProvider interface {
	Value(service, metric string) (float64, bool)
}

// Evaluator watches intents and reports which goals are at risk.
type Evaluator struct {
	mu      sync.RWMutex
	intents map[string]Intent
	metrics MetricProvider
	now     func() time.Time
}

// New creates an Evaluator backed by a metric provider.
func New(metrics MetricProvider) *Evaluator {
	return &Evaluator{
		intents: make(map[string]Intent),
		metrics: metrics,
		now:     time.Now,
	}
}

// AddIntent registers or replaces an intent by name.
func (e *Evaluator) AddIntent(i Intent) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.intents[i.Name] = i
}

// RemoveIntent drops an intent by name. No-op if absent.
func (e *Evaluator) RemoveIntent(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.intents, name)
}

// List returns all registered intents in a stable order.
func (e *Evaluator) List() []Intent {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Intent, 0, len(e.intents))
	for _, i := range e.intents {
		out = append(out, i)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Evaluate reports the status of every goal across every intent.
func (e *Evaluator) Evaluate() []Status {
	e.mu.RLock()
	defer e.mu.RUnlock()
	now := e.now()
	var out []Status
	for _, intent := range e.intents {
		for _, g := range intent.Goals {
			out = append(out, e.evalGoal(g, now))
		}
	}
	return out
}

// Suggest returns ranked actions that would restore at-risk or violated goals.
// Higher-priority goals appear first.
func (e *Evaluator) Suggest() []Suggestion {
	statuses := e.Evaluate()
	var out []Suggestion
	for _, s := range statuses {
		if s.Met && !s.AtRisk {
			continue
		}
		for _, sug := range e.compile(s) {
			out = append(out, sug)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Priority > out[j].Priority
	})
	return out
}

// Describe returns a human-readable summary of the current intent state.
func (e *Evaluator) Describe() string {
	statuses := e.Evaluate()
	if len(statuses) == 0 {
		return "no intents registered"
	}
	met, risk, violated := 0, 0, 0
	for _, s := range statuses {
		switch {
		case !s.Met:
			violated++
		case s.AtRisk:
			risk++
		default:
			met++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d goals: %d met, %d at risk, %d violated", len(statuses), met, risk, violated)
	return b.String()
}

// ── internal helpers ──────────────────────────────────────────────────────

func (e *Evaluator) evalGoal(g Goal, now time.Time) Status {
	v, ok := e.metrics.Value(g.Service, metricFor(g))
	if !ok {
		return Status{Goal: g, Target: g.Target, CheckedAt: now}
	}
	met, atRisk, slack := judge(g, v)
	return Status{
		Goal:      g,
		Current:   v,
		Target:    g.Target,
		Met:       met,
		AtRisk:    atRisk,
		Slack:     slack,
		CheckedAt: now,
	}
}

func judge(g Goal, current float64) (met, atRisk bool, slack float64) {
	const riskBand = 0.20 // within 20% of threshold = at risk
	switch g.Kind {
	case LatencyUnder, ErrorRateUnder, CostCap, Saturation:
		// "under" constraints: current must stay below target.
		// At-risk when current is inside the top 20% of the band below target.
		met = current < g.Target
		slack = g.Target - current
		if met && current >= g.Target*(1-riskBand) {
			atRisk = true
		}
	case AvailabilityOver:
		// "over" constraint: current must stay above target.
		// At-risk when slack is inside the bottom 60% of the possible safe
		// band (target..1.0). 60% rather than 50% because availability
		// targets are often close to 1.0 where floating-point headroom is tiny.
		met = current > g.Target
		slack = current - g.Target
		if met && slack < (1-g.Target)*0.6 {
			atRisk = true
		}
	case JobsNoDrop:
		// current is the drop count; target is 0.
		met = current <= g.Target
		slack = g.Target - current
	case ProtectService:
		// Protect is always "met" in evaluation terms; it boosts priority of
		// other suggestions that touch this service.
		met = true
	}
	return
}

func metricFor(g Goal) string {
	if g.Metric != "" {
		return g.Metric
	}
	switch g.Kind {
	case LatencyUnder:
		return "latency_p99"
	case ErrorRateUnder:
		return "error_rate"
	case AvailabilityOver:
		return "availability"
	case JobsNoDrop:
		return "jobs_dropped"
	case CostCap:
		return "cost_per_hour"
	case Saturation:
		return "saturation"
	default:
		return ""
	}
}

// compile maps a violated or at-risk status into one or more concrete actions.
// Actions are strings the engine's action executor can recognize; the concrete
// implementation is decoupled from the intent package.
func (e *Evaluator) compile(s Status) []Suggestion {
	base := Suggestion{
		Intent:   "",
		Goal:     s.Goal,
		Priority: s.Goal.Priority,
		Impact:   map[string]float64{},
	}
	svc := s.Goal.Service
	if svc == "" {
		svc = "*"
	}
	switch s.Goal.Kind {
	case LatencyUnder:
		return []Suggestion{
			fill(base, fmt.Sprintf("scale:%s:+1", svc), fmt.Sprintf("latency %0.1f is approaching cap %0.1f on %s; scale out one replica", s.Current, s.Target, svc), map[string]float64{"latency_p99": -15}),
			fill(base, fmt.Sprintf("warm_cache:%s", svc), "pre-warm the read-set to absorb the load peak", map[string]float64{"cache_miss": -0.3, "latency_p99": -20}),
			fill(base, "shed_load:non_critical", "drop non-critical traffic until latency recovers", map[string]float64{"latency_p99": -35, "throughput_non_critical": -0.4}),
		}
	case ErrorRateUnder:
		return []Suggestion{
			fill(base, fmt.Sprintf("throttle:%s", svc), fmt.Sprintf("error rate %0.3f exceeds target %0.3f on %s; apply retry backoff", s.Current, s.Target, svc), map[string]float64{"error_rate": -0.4, "retry_storm_risk": -0.7}),
			fill(base, fmt.Sprintf("circuit_break:%s", svc), "open the circuit breaker to stop cascading failures", map[string]float64{"error_rate": -0.6}),
		}
	case AvailabilityOver:
		return []Suggestion{
			fill(base, fmt.Sprintf("failover:%s", svc), fmt.Sprintf("availability %0.4f is below target %0.4f; fail over to secondary", s.Current, s.Target), map[string]float64{"availability": 0.003}),
			fill(base, fmt.Sprintf("scale:%s:+2", svc), "add redundancy to avoid single-instance risk", map[string]float64{"availability": 0.002}),
		}
	case JobsNoDrop:
		return []Suggestion{
			fill(base, fmt.Sprintf("scale:%s:+2", svc), fmt.Sprintf("queue dropping jobs (%0.0f dropped); add workers", s.Current), map[string]float64{"jobs_dropped": -1.0, "queue_depth": -0.5}),
			fill(base, fmt.Sprintf("spill_to_disk:%s", svc), "buffer excess jobs on disk instead of dropping", map[string]float64{"jobs_dropped": -1.0, "memory": 0.1}),
		}
	case CostCap:
		return []Suggestion{
			fill(base, "scale_in:non_critical:-1", fmt.Sprintf("cost %0.2f exceeds target %0.2f; scale in non-critical services", s.Current, s.Target), map[string]float64{"cost_per_hour": -0.15}),
			fill(base, "shift_to_spot", "move eligible workloads to spot instances", map[string]float64{"cost_per_hour": -0.3, "availability_risk": 0.02}),
		}
	case Saturation:
		return []Suggestion{
			fill(base, fmt.Sprintf("scale:%s:+1", svc), fmt.Sprintf("saturation %0.2f near cap %0.2f on %s", s.Current, s.Target, svc), map[string]float64{"saturation": -0.25}),
		}
	case ProtectService:
		// no direct suggestion; priority-only goal
	}
	return nil
}

func fill(base Suggestion, action, rationale string, impact map[string]float64) Suggestion {
	base.Action = action
	base.Rationale = rationale
	base.Impact = impact
	return base
}
