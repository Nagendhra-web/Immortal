package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Nagendhra-web/Immortal/internal/agentic"
	"github.com/Nagendhra-web/Immortal/internal/event"
	"github.com/Nagendhra-web/Immortal/internal/llm"
)

// heuristicPlanner is the default Planner when no LLM is configured.
// It follows a deterministic health-first flow:
//
//	iter 0: check_health(target)
//	iter 1 (if unhealthy): restart_service(target)
//	iter 2: check_health(target)
//	iter 3+: finish
//
// This keeps the agentic loop useful even for operators who haven't wired
// an LLM yet — the work is real, just rule-driven.
type heuristicPlanner struct{}

func (h *heuristicPlanner) NextStep(ev *event.Event, history []agentic.Step) (tool string, args map[string]any, thought string, err error) {
	target := ev.Source
	if target == "" {
		target = "unknown"
	}
	switch len(history) {
	case 0:
		return "check_health", map[string]any{"target": target},
			"initial triage — check current health of " + target, nil
	case 1:
		last := history[0].Observation
		if strings.Contains(strings.ToLower(last), "unhealthy") {
			return "restart_service", map[string]any{"name": target},
				"target reports unhealthy, attempting restart", nil
		}
		return "finish", map[string]any{"reason": "target was already healthy"},
			"no action needed", nil
	case 2:
		return "check_health", map[string]any{"target": target},
			"verify restart actually restored health", nil
	default:
		reason := "completed healing loop"
		if last := history[len(history)-1].Observation; last != "" {
			reason = reason + " — final state: " + last
		}
		return "finish", map[string]any{"reason": reason}, "done", nil
	}
}

// llmPlanner asks a configured llm.Client what tool to call next. The client
// is expected to return strict JSON with the fields {tool, args, thought}.
type llmPlanner struct {
	client *llm.Client
}

func newLLMPlanner(c *llm.Client) agentic.Planner {
	return &llmPlanner{client: c}
}

func (p *llmPlanner) NextStep(ev *event.Event, history []agentic.Step) (tool string, args map[string]any, thought string, err error) {
	if p.client == nil || !p.client.IsEnabled() {
		return "", nil, "", errors.New("llm client not configured")
	}

	system := "You are Immortal, an autonomous self-healing engine operating a ReAct loop. " +
		"At each step decide ONE tool to call. Available tools: check_health(target), " +
		"restart_service(name), scale_service(name, replicas), get_metric(name), finish(reason). " +
		"Respond ONLY with compact JSON: {\"tool\":\"...\",\"args\":{...},\"thought\":\"...\"}. " +
		"When the incident is resolved, call finish."

	user := fmt.Sprintf("Incident: source=%q severity=%q message=%q\nMeta: %v\nSteps so far:\n%s",
		ev.Source, ev.Severity, ev.Message, ev.Meta, formatHistory(history))

	resp, err := p.client.Analyze(system, user)
	if err != nil {
		return "", nil, "", fmt.Errorf("llm: %w", err)
	}

	var parsed struct {
		Tool    string         `json:"tool"`
		Args    map[string]any `json:"args"`
		Thought string         `json:"thought"`
	}
	if err := json.Unmarshal([]byte(resp.Content), &parsed); err != nil {
		return "finish", map[string]any{"reason": "llm response unparseable: " + resp.Content},
			"llm returned non-JSON; terminating", nil
	}
	if parsed.Tool == "" {
		return "finish", map[string]any{"reason": "llm omitted tool field"}, parsed.Thought, nil
	}
	if parsed.Args == nil {
		parsed.Args = map[string]any{}
	}
	return parsed.Tool, parsed.Args, parsed.Thought, nil
}

func formatHistory(steps []agentic.Step) string {
	if len(steps) == 0 {
		return "(no prior steps)"
	}
	var b strings.Builder
	for _, s := range steps {
		fmt.Fprintf(&b, "  #%d tool=%s args=%v observation=%q\n",
			s.Iteration, s.Tool, s.ToolArgs, s.Observation)
	}
	return b.String()
}
