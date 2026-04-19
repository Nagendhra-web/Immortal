package agentic

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/event"
)

// ToolFunc is the signature for a tool implementation.
type ToolFunc func(args map[string]any) (result string, err error)

// CostTier classifies the impact level of a tool.
type CostTier int

const (
	CostRead        CostTier = 0 // read-only: get_metric, check_health, list_dependencies
	CostReversible  CostTier = 1 // safe to undo: dry_run, wait, rollback
	CostDisruptive  CostTier = 2 // causes disruption: restart_service, scale
	CostDestructive CostTier = 3 // traffic-shifting, hard failover: canary, failover
)

// Tool describes an action the agent can invoke.
type Tool struct {
	Name          string
	Description   string
	Schema        map[string]string // arg name -> type description
	Fn            ToolFunc
	CostTier      CostTier // impact classification
	Reversible    bool     // whether the action can be undone
	BlastRadius   int      // estimated number of services impacted
	Prerequisites []string // tool names whose observations are needed first
}

// Step is one iteration of the agent reasoning loop.
type Step struct {
	Iteration   int
	Thought     string
	Tool        string
	ToolArgs    map[string]any
	Observation string
	Error       string
	Timestamp   time.Time
	Reflection  *Reflection // populated after each tool run when a Reflector is configured
}

// Trace is the full record of an agent run.
type Trace struct {
	IncidentID string
	Steps      []Step
	Resolved   bool
	Reason     string
	Duration   time.Duration
}

// Planner decides the next action given the incident and history.
// Returning tool == "finish" exits the loop.
type Planner interface {
	NextStep(incident *event.Event, history []Step) (tool string, args map[string]any, thought string, err error)
}

// MetricProvider returns the current value of a named metric.
type MetricProvider func(name string) (string, error)

// HealthChecker returns "healthy" or "unhealthy" for a target.
type HealthChecker func(target string) (string, error)

// ReflectingPlanner is an optional extension of Planner that receives the
// reflection history in addition to the step history.
type ReflectingPlanner interface {
	Planner
	NextStepWithReflection(incident *event.Event, history []Step, reflections []Reflection) (tool string, args map[string]any, thought string, err error)
}

// Config controls agent behaviour.
type Config struct {
	MaxIterations  int
	StepTimeout    time.Duration
	Planner        Planner
	MetricProvider MetricProvider
	HealthChecker  HealthChecker
	Reflector      Reflector      // optional; defaults to DefaultReflector when nil
	Memory         *Memory        // optional; records traces for future recall
	Safety         *SafetyPolicy  // optional; guards each tool call
}

// Agent runs the ReAct-style healing loop.
type Agent struct {
	cfg   Config
	mu    sync.RWMutex
	tools map[string]Tool
}

// New creates an Agent with safe defaults and registers built-in tools.
func New(cfg Config) *Agent {
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 8
	}
	if cfg.StepTimeout <= 0 {
		cfg.StepTimeout = 10 * time.Second
	}
	if cfg.MetricProvider == nil {
		cfg.MetricProvider = func(name string) (string, error) {
			return fmt.Sprintf("metric:%s=0", name), nil
		}
	}
	if cfg.HealthChecker == nil {
		cfg.HealthChecker = func(target string) (string, error) {
			return "healthy", nil
		}
	}
	if cfg.Reflector == nil {
		cfg.Reflector = DefaultReflector{}
	}

	a := &Agent{
		cfg:   cfg,
		tools: make(map[string]Tool),
	}
	registerBuiltins(a)
	return a
}

// RegisterTool adds or replaces a tool by name.
func (a *Agent) RegisterTool(t Tool) {
	a.mu.Lock()
	a.tools[t.Name] = t
	a.mu.Unlock()
}

// Tools returns a snapshot of all registered tools.
func (a *Agent) Tools() []Tool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]Tool, 0, len(a.tools))
	for _, t := range a.tools {
		out = append(out, t)
	}
	return out
}

// Run executes the agentic healing loop for the given incident and returns
// a full Trace regardless of whether healing succeeded.
func (a *Agent) Run(incident *event.Event) *Trace {
	start := time.Now()
	trace := &Trace{IncidentID: incident.ID}
	history := make([]Step, 0, a.cfg.MaxIterations)
	reflections := make([]Reflection, 0, a.cfg.MaxIterations)

	consecutiveViolations := 0

	for i := 0; i < a.cfg.MaxIterations; i++ {
		step := Step{Iteration: i, Timestamp: time.Now()}

		// Call either the reflecting or legacy planner variant.
		var tool string
		var args map[string]any
		var thought string
		var err error

		if rp, ok := a.cfg.Planner.(ReflectingPlanner); ok {
			tool, args, thought, err = rp.NextStepWithReflection(incident, history, reflections)
		} else {
			tool, args, thought, err = a.cfg.Planner.NextStep(incident, history)
		}

		step.Thought = thought
		step.Tool = tool
		step.ToolArgs = args

		if err != nil {
			step.Error = err.Error()
			history = append(history, step)
			break
		}

		if tool == "finish" {
			trace.Resolved = true
			if r, ok := args["reason"]; ok {
				trace.Reason = fmt.Sprintf("%v", r)
			} else {
				trace.Reason = thought
			}
			history = append(history, step)
			break
		}

		// Safety gate: check policy before executing.
		if a.cfg.Safety != nil {
			a.mu.RLock()
			toolDef, toolFound := a.tools[tool]
			a.mu.RUnlock()
			if toolFound {
				if violation := a.cfg.Safety.Guard(toolDef, args, history); violation != nil {
					step.Error = fmt.Sprintf("safety violation: %s", violation.Reason)
					history = append(history, step)
					consecutiveViolations++
					if consecutiveViolations >= 3 {
						trace.Resolved = false
						trace.Reason = "safety policy halted the loop"
						break
					}
					continue
				}
			}
		}
		consecutiveViolations = 0

		step.Observation, step.Error = a.runTool(tool, args)

		// Post-step reflection.
		ref := a.cfg.Reflector.Reflect(step, a.cfg.Planner)
		step.Reflection = &ref
		reflections = append(reflections, ref)

		history = append(history, step)
	}

	trace.Steps = history
	trace.Duration = time.Since(start)

	// Record into memory if configured.
	if a.cfg.Memory != nil {
		outcome := OutcomeFailed
		if trace.Resolved {
			outcome = OutcomeResolved
		}
		inc := Incident{
			Message:  incident.Message,
			Source:   incident.Source,
			Severity: string(incident.Severity),
		}
		a.cfg.Memory.Record(inc, trace, outcome)
	}

	return trace
}

// runTool executes the named tool under StepTimeout.
func (a *Agent) runTool(name string, args map[string]any) (result string, errStr string) {
	a.mu.RLock()
	t, ok := a.tools[name]
	a.mu.RUnlock()

	if !ok {
		return "", fmt.Sprintf("unknown tool: %s", name)
	}

	type outcome struct {
		result string
		err    error
	}
	ch := make(chan outcome, 1)

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.StepTimeout)
	defer cancel()

	go func() {
		r, e := t.Fn(args)
		ch <- outcome{r, e}
	}()

	select {
	case <-ctx.Done():
		return "", fmt.Sprintf("tool %s timed out", name)
	case o := <-ch:
		if o.err != nil {
			return "", o.err.Error()
		}
		return o.result, ""
	}
}
