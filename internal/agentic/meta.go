package agentic

import (
	"fmt"
	"sync"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
)

// Hypothesis is a single investigation branch with a name and its own Planner.
type Hypothesis struct {
	Name        string
	Description string
	Planner     Planner
	MaxSteps    int
}

// MetaResult is the aggregated outcome of parallel investigation.
type MetaResult struct {
	Incident  *event.Event
	Branches  []*Trace // one per hypothesis, in dispatch order
	Winner    int      // index of the branch that Resolved=true fastest; -1 if none did
	Duration  time.Duration
}

// MetaConfig controls MetaAgent behaviour.
type MetaConfig struct {
	ToolConfigurator    func(a *Agent) // called per sub-agent to register tools
	Safety              *SafetyPolicy
	ParallelLimit       int  // max concurrent hypotheses (0 = len(hypotheses))
	StopOnFirstResolve  bool // cancel remaining branches once any branch resolves
}

// MetaAgent runs multiple Hypotheses in parallel, each with its own sub-Agent,
// and returns the fastest Resolved branch. If none resolve, returns all traces
// with Winner=-1.
type MetaAgent struct {
	cfg MetaConfig
}

// NewMetaAgent returns a MetaAgent with the given configuration.
func NewMetaAgent(cfg MetaConfig) *MetaAgent {
	return &MetaAgent{cfg: cfg}
}

// indexedTrace pairs a branch index with its completed trace.
type indexedTrace struct {
	idx   int
	trace *Trace
}

// Investigate dispatches hypotheses in parallel and collects traces.
// When StopOnFirstResolve is set, collection stops as soon as one branch resolves;
// other goroutines still run to completion but their results are discarded.
func (m *MetaAgent) Investigate(ev *event.Event, hypotheses []Hypothesis) *MetaResult {
	start := time.Now()
	n := len(hypotheses)
	result := &MetaResult{
		Incident: ev,
		Branches: make([]*Trace, n),
		Winner:   -1,
	}
	if n == 0 {
		result.Duration = time.Since(start)
		return result
	}

	limit := m.cfg.ParallelLimit
	if limit <= 0 || limit > n {
		limit = n
	}

	// Buffered channel large enough to hold all traces without blocking goroutines.
	ch := make(chan indexedTrace, n)

	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup

	for i, h := range hypotheses {
		wg.Add(1)
		i, h := i, h
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			maxSteps := h.MaxSteps
			if maxSteps <= 0 {
				maxSteps = 8
			}
			a := New(Config{
				Planner:       h.Planner,
				Safety:        m.cfg.Safety,
				MaxIterations: maxSteps,
			})
			if m.cfg.ToolConfigurator != nil {
				m.cfg.ToolConfigurator(a)
			}
			trace := a.Run(ev)
			ch <- indexedTrace{idx: i, trace: trace}
		}()
	}

	// Close channel once all goroutines finish so the collector loop exits.
	go func() {
		wg.Wait()
		close(ch)
	}()

	// Collect results; stop early if StopOnFirstResolve.
	collected := 0
	for it := range ch {
		result.Branches[it.idx] = it.trace
		collected++
		if it.trace.Resolved && result.Winner == -1 {
			result.Winner = it.idx
			if m.cfg.StopOnFirstResolve {
				break
			}
		}
		if collected == n {
			break
		}
	}

	// Drain remaining goroutines into a discard channel so they don't block.
	go func() {
		for range ch { //nolint:revive
		}
	}()

	result.Duration = time.Since(start)
	return result
}

// ---- Built-in hypothesis generators ----

// HypothesisResourceExhaustion returns a Hypothesis that checks cpu, memory,
// and disk metrics using the provided checker function.
// checker(target, metricName) returns (value, error).
func HypothesisResourceExhaustion(target string, metricChecker func(string, string) (float64, error)) Hypothesis {
	return Hypothesis{
		Name:        "resource_exhaustion",
		Description: "Check whether CPU, memory, or disk is exhausted on " + target,
		MaxSteps:    5,
		Planner: &resourceExhaustionPlanner{
			target:        target,
			metricChecker: metricChecker,
		},
	}
}

type resourceExhaustionPlanner struct {
	mu            sync.Mutex
	pos           int
	target        string
	metricChecker func(string, string) (float64, error)
	verdict       string
}

func (p *resourceExhaustionPlanner) NextStep(_ *event.Event, history []Step) (string, map[string]any, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	metrics := []string{"cpu", "memory", "disk"}
	if p.pos < len(metrics) {
		metric := metrics[p.pos]
		p.pos++

		val, err := p.metricChecker(p.target, metric)
		obs := ""
		if err != nil {
			obs = fmt.Sprintf("error checking %s: %v", metric, err)
		} else {
			obs = fmt.Sprintf("%s=%.2f", metric, val)
			if val > 90.0 {
				p.verdict = fmt.Sprintf("%s exhausted on %s (%.2f%%)", metric, p.target, val)
			}
		}
		_ = obs
		_ = history
		return "get_metric", map[string]any{"name": metric, "target": p.target},
			fmt.Sprintf("checking %s on %s", metric, p.target), nil
	}

	reason := p.verdict
	if reason == "" {
		reason = fmt.Sprintf("no resource exhaustion detected on %s", p.target)
	}
	return "finish", map[string]any{"reason": reason}, "resource check complete", nil
}

// HypothesisDependencyFailure returns a Hypothesis that lists dependencies of
// target and checks whether any have failed.
// depChecker(target) returns ([]string of failed deps, error).
func HypothesisDependencyFailure(target string, depChecker func(string) ([]string, error)) Hypothesis {
	return Hypothesis{
		Name:        "dependency_failure",
		Description: "Check whether any dependency of " + target + " has failed",
		MaxSteps:    4,
		Planner: &dependencyFailurePlanner{
			target:     target,
			depChecker: depChecker,
		},
	}
}

type dependencyFailurePlanner struct {
	mu         sync.Mutex
	done       bool
	target     string
	depChecker func(string) ([]string, error)
}

func (p *dependencyFailurePlanner) NextStep(_ *event.Event, _ []Step) (string, map[string]any, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.done {
		p.done = true
		failed, err := p.depChecker(p.target)
		reason := ""
		if err != nil {
			reason = fmt.Sprintf("dependency check error: %v", err)
		} else if len(failed) > 0 {
			reason = fmt.Sprintf("failed dependencies: %v", failed)
		} else {
			reason = "all dependencies healthy"
		}
		return "finish", map[string]any{"reason": reason},
			fmt.Sprintf("checking dependencies of %s", p.target), nil
	}
	return "finish", map[string]any{"reason": "already checked"}, "done", nil
}

// HypothesisRecentDeployment returns a Hypothesis that fetches deployment history
// and checks whether a recent deploy could be the cause.
// historyChecker(target) returns ([]string of recent deploy events, error).
func HypothesisRecentDeployment(target string, historyChecker func(string) ([]string, error)) Hypothesis {
	return Hypothesis{
		Name:        "recent_deployment",
		Description: "Check whether a recent deployment to " + target + " caused the incident",
		MaxSteps:    4,
		Planner: &recentDeploymentPlanner{
			target:         target,
			historyChecker: historyChecker,
		},
	}
}

type recentDeploymentPlanner struct {
	mu             sync.Mutex
	done           bool
	target         string
	historyChecker func(string) ([]string, error)
}

func (p *recentDeploymentPlanner) NextStep(_ *event.Event, _ []Step) (string, map[string]any, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.done {
		p.done = true
		events, err := p.historyChecker(p.target)
		reason := ""
		if err != nil {
			reason = fmt.Sprintf("deployment history error: %v", err)
		} else if len(events) > 0 {
			reason = fmt.Sprintf("recent deployments found: %v", events)
		} else {
			reason = "no recent deployments found"
		}
		return "finish", map[string]any{"reason": reason},
			fmt.Sprintf("checking deployment history of %s", p.target), nil
	}
	return "finish", map[string]any{"reason": "already checked"}, "done", nil
}
