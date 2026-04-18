package economic

import (
	"fmt"
	"sync"
)

// ServiceValue captures the dollar value of a service being up.
type ServiceValue struct {
	Service         string
	USDPerMinute    float64 // e.g. checkout: 2000, analytics: 20
	DependentFactor float64 // 1.0 = fully dependent; higher = damaged beyond its own value
}

// ActionCost captures the dollar cost of executing an action.
type ActionCost struct {
	Type                string  // "restart", "failover", "scale", "rollback", ...
	InfraCostUSD        float64 // cloud spend incurred, e.g. extra replicas
	EngineeringCostUSD  float64 // human time if action requires review
	ExpectedDowntimeSec float64 // any self-inflicted outage caused by the action
	SuccessProbability  float64 // 0..1 — estimated success of this action type
}

// IncidentProfile describes the current incident in economic terms.
type IncidentProfile struct {
	AffectedService      string
	AffectedDependents   []string // transitive dependents
	EstDurationMinutes   float64  // if we do nothing
	ConfidenceThisIsReal float64  // 0..1 — e.g. probability it's not a false alarm
}

// Catalog is the registry of service values + action costs.
type Catalog struct {
	mu       sync.RWMutex
	services map[string]ServiceValue
	actions  map[string]ActionCost
	warnings []string
}

// NewCatalog returns an empty Catalog.
func NewCatalog() *Catalog {
	return &Catalog{
		services: make(map[string]ServiceValue),
		actions:  make(map[string]ActionCost),
	}
}

// SetServiceValue registers or updates a service's value.
func (c *Catalog) SetServiceValue(s ServiceValue) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services[s.Service] = s
}

// SetActionCost registers or updates an action cost. If SuccessProbability is
// zero (the zero value), it defaults to 1.0.
func (c *Catalog) SetActionCost(a ActionCost) {
	if a.SuccessProbability == 0 {
		a.SuccessProbability = 1.0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.actions[a.Type] = a
}

// ServiceValue returns the registered value for a service.
func (c *Catalog) ServiceValue(service string) (ServiceValue, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	sv, ok := c.services[service]
	if !ok {
		c.mu.RUnlock()
		c.mu.Lock()
		c.warnings = append(c.warnings, fmt.Sprintf("ServiceValue not registered for %q; defaulting to 0", service))
		c.mu.Unlock()
		c.mu.RLock()
	}
	return sv, ok
}

// ActionCost returns the registered cost for an action type.
func (c *Catalog) ActionCost(actionType string) (ActionCost, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ac, ok := c.actions[actionType]
	return ac, ok
}

// Warnings returns accumulated debug warnings (e.g. missing service registrations).
func (c *Catalog) Warnings() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]string, len(c.warnings))
	copy(out, c.warnings)
	return out
}

// CandidatePlan is a sequence of (actionType, target) steps with an
// estimated aggregate success probability and expected restore time.
type CandidatePlan struct {
	ID                 string
	Steps              []Step
	AggregateSuccess   float64 // 0..1 — product of per-action success probabilities
	ExpectedRestoreMin float64 // estimated minutes until full recovery if plan works
}

// Step is a single (action, target) pair within a plan.
type Step struct {
	Action string
	Target string
}

// ValuationBreakdown is the expected value analysis for one plan.
type ValuationBreakdown struct {
	PlanID             string
	ExpectedLossSaved  float64 // USD; service_value * minutes_saved * dependent_factor
	PlanCostUSD        float64 // sum of action infra + engineering + self-inflicted downtime
	ExpectedNetValue   float64 // LossSaved * AggregateSuccess - PlanCost
	WorstCaseLoss      float64 // if plan fails and incident continues full duration
	BestCaseSaved      float64
	ConfidenceAdjusted float64 // ExpectedNetValue * IncidentProfile.ConfidenceThisIsReal
}
