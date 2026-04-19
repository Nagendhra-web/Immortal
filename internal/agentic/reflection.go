package agentic

import (
	"strings"

	"github.com/Nagendhra-web/Immortal/internal/event"
)

// Reflection captures the post-step self-critique for one iteration.
type Reflection struct {
	Step        int
	Observation string
	Expected    string  // what the planner said it expected (derived from Thought)
	Matched     bool    // did the observation match the expectation?
	Critique    string  // short self-critique fed into the next planner call
	Confidence  float64 // 0-1 estimate of resolution confidence
}

// Reflector evaluates a step's outcome and produces a Reflection.
type Reflector interface {
	Reflect(step Step, planner Planner) Reflection
}

// DefaultReflector applies deterministic heuristics to produce critiques.
// It does not call any LLM — results are stable and testable.
type DefaultReflector struct{}

// Reflect produces a Reflection for the given step.
//
// Heuristic rules (applied in order, first match wins):
//  1. Tool error → "tool failed; try a different approach"
//  2. restart_service / scale_service followed by "unhealthy" observation → "restart insufficient, try failover"
//  3. Observation contains "unhealthy" → "service still unhealthy; consider escalation"
//  4. Observation contains "healthy" → high confidence, positive critique
//  5. Default → neutral critique
func (d DefaultReflector) Reflect(step Step, _ Planner) Reflection {
	r := Reflection{
		Step:        step.Iteration,
		Observation: step.Observation,
		Expected:    step.Thought,
	}

	obs := strings.ToLower(step.Observation)
	tool := strings.ToLower(step.Tool)

	switch {
	case step.Error != "":
		r.Matched = false
		r.Critique = "tool failed; try a different approach"
		r.Confidence = 0.1

	case (tool == "restart_service" || tool == "scale_service" || tool == "scale") &&
		strings.Contains(obs, "unhealthy"):
		r.Matched = false
		r.Critique = "restart insufficient, try failover"
		r.Confidence = 0.2

	case strings.Contains(obs, "unhealthy"):
		r.Matched = false
		r.Critique = "service still unhealthy; consider escalation"
		r.Confidence = 0.3

	case strings.Contains(obs, "healthy"):
		r.Matched = true
		r.Critique = "service appears healthy; verify and resolve"
		r.Confidence = 0.9

	case strings.Contains(obs, "restarted") || strings.Contains(obs, "scaled") ||
		strings.Contains(obs, "rollback") || strings.Contains(obs, "failover"):
		r.Matched = true
		r.Critique = "action applied; check health next"
		r.Confidence = 0.6

	default:
		r.Matched = true
		r.Critique = "observation recorded; continue"
		r.Confidence = 0.5
	}

	return r
}

// ReflectingPlanner is an optional extension of Planner. Planners that
// implement it receive the full reflection history alongside the step history,
// enabling Reflexion-style verbal reinforcement.
//
// Defined in agentic.go to avoid circular reference; repeated here for docs.
// See: agentic.go → ReflectingPlanner interface.

// reflectingPlannerAdapter wraps a plain Planner so it satisfies
// ReflectingPlanner by ignoring reflections. Used in tests.
type reflectingPlannerAdapter struct {
	Planner
	fn func(incident *event.Event, history []Step, reflections []Reflection) (string, map[string]any, string, error)
}

func (r *reflectingPlannerAdapter) NextStepWithReflection(
	incident *event.Event,
	history []Step,
	reflections []Reflection,
) (string, map[string]any, string, error) {
	return r.fn(incident, history, reflections)
}
