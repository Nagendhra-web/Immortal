package agentic

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
)

// ToolFunc is the signature for a tool implementation.
type ToolFunc func(args map[string]any) (result string, err error)

// Tool describes an action the agent can invoke.
type Tool struct {
	Name        string
	Description string
	Schema      map[string]string // arg name -> type description
	Fn          ToolFunc
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

// Config controls agent behaviour.
type Config struct {
	MaxIterations  int
	StepTimeout    time.Duration
	Planner        Planner
	MetricProvider MetricProvider
	HealthChecker  HealthChecker
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

	for i := 0; i < a.cfg.MaxIterations; i++ {
		step := Step{Iteration: i, Timestamp: time.Now()}

		tool, args, thought, err := a.cfg.Planner.NextStep(incident, history)
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

		step.Observation, step.Error = a.runTool(tool, args)
		history = append(history, step)
	}

	trace.Steps = history
	trace.Duration = time.Since(start)
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
