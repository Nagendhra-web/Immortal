package nlplan

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Nagendhra-web/Immortal/internal/formal"
)

// CompileRequest asks the compiler to turn English into a plan.
type CompileRequest struct {
	Text     string
	Services []string // available services; compiler validates targets are known
	MaxSteps int      // caps plan size; default 10
}

// CompiledPlan bundles a formal.Plan + the invariants the user stated.
type CompiledPlan struct {
	Plan       formal.Plan
	Invariants []formal.Invariant
	Trace      []Translation
	Warnings   []string
	Source     string
}

// Translation documents one span of English -> action.
type Translation struct {
	SpanStart  int
	SpanEnd    int
	Span       string
	Interpreted string
}

// LLMFunc is the minimal interface the compiler needs from an LLM.
type LLMFunc func(systemPrompt, userPrompt string) (responseJSON string, err error)

// Config holds optional configuration for the Compiler.
type Config struct {
	LLM LLMFunc // may be nil; grammar fallback used when nil
}

// Compiler is the public compiler.
type Compiler struct {
	cfg Config
}

// NewCompiler creates a new Compiler with the given Config.
func NewCompiler(cfg Config) *Compiler {
	return &Compiler{cfg: cfg}
}

// Compile turns r.Text into a typed CompiledPlan.
func (c *Compiler) Compile(r CompileRequest) (*CompiledPlan, error) {
	if r.MaxSteps == 0 {
		r.MaxSteps = 10
	}

	svcSet := make(map[string]bool, len(r.Services))
	for _, s := range r.Services {
		svcSet[strings.ToLower(s)] = true
	}

	// Always try grammar first.
	gr := parseGrammar(r.Text, svcSet)

	var steps []formal.Action
	var invariants []formal.Invariant
	var trace []Translation
	var warnings []string

	if len(gr.steps) > 0 || len(gr.invariants) > 0 {
		steps = gr.steps
		invariants = gr.invariants
		trace = gr.trace
		warnings = gr.warnings
	} else if c.cfg.LLM != nil {
		// Grammar produced nothing; fall through to LLM.
		llmSteps, llmInvariants, llmTrace, llmWarnings, err := c.compileLLM(r.Text, svcSet)
		if err != nil {
			return nil, fmt.Errorf("nlplan: LLM compilation failed: %w", err)
		}
		steps = llmSteps
		invariants = llmInvariants
		trace = llmTrace
		warnings = llmWarnings
	} else {
		return nil, fmt.Errorf("nlplan: could not parse %q (no LLM configured)", r.Text)
	}

	// Enforce MaxSteps.
	if len(steps) > r.MaxSteps {
		warnings = append(warnings, fmt.Sprintf("plan truncated to %d steps (MaxSteps)", r.MaxSteps))
		steps = steps[:r.MaxSteps]
	}

	if len(steps) == 0 && len(invariants) == 0 {
		return nil, fmt.Errorf("nlplan: compilation produced empty plan and no invariants")
	}

	plan := formal.Plan{
		ID:    planID(r.Text),
		Steps: steps,
	}

	return &CompiledPlan{
		Plan:       plan,
		Invariants: invariants,
		Trace:      trace,
		Warnings:   warnings,
		Source:     r.Text,
	}, nil
}

// CompileAndVerify compiles r and then runs formal.Check.
func (c *Compiler) CompileAndVerify(r CompileRequest, initial formal.World) (*CompiledPlan, formal.Result, error) {
	cp, err := c.Compile(r)
	if err != nil {
		return nil, formal.Result{}, err
	}
	result := formal.Check(initial, cp.Plan, cp.Invariants)
	return cp, result, nil
}

// ---- LLM path ---------------------------------------------------------------

// llmResponse is the strict JSON schema we ask the LLM to emit.
type llmResponse struct {
	PlanID     string        `json:"plan_id"`
	Steps      []llmStep     `json:"steps"`
	Invariants []llmInvariant `json:"invariants"`
	Trace      []llmTrace    `json:"trace"`
	Warnings   []string      `json:"warnings"`
}

type llmStep struct {
	Action string  `json:"action"` // "restart" | "set_replicas" | "noop"
	Target string  `json:"target"`
	Value  *int    `json:"value"`
}

type llmInvariant struct {
	Kind    string `json:"kind"`    // "at_least_n_healthy" | "min_replicas" | "service_always_healthy"
	N       int    `json:"n"`
	Service string `json:"service"`
}

type llmTrace struct {
	Span        string `json:"span"`
	Interpreted string `json:"interpreted"`
}

const llmSystemPrompt = `You are a healing-plan compiler. Given a description of a system healing plan in English, output ONLY valid JSON matching this schema (no markdown, no explanation):

{
  "plan_id": "<short identifier>",
  "steps": [
    {"action": "restart|set_replicas|noop", "target": "<service>", "value": <integer or null>}
  ],
  "invariants": [
    {"kind": "at_least_n_healthy", "n": <integer>},
    {"kind": "min_replicas", "service": "<service>", "n": <integer>},
    {"kind": "service_always_healthy", "service": "<service>"}
  ],
  "trace": [{"span": "<original text fragment>", "interpreted": "<what you understood>"}],
  "warnings": ["<any ambiguity or unknown service>"]
}

Rules:
- "restart" means set the service healthy=true.
- "set_replicas" means set replica count; "value" must be a non-null integer.
- "noop" is a no-operation step.
- Emit only the invariant kinds listed above.
- Respond with ONLY the JSON object. No markdown fences.`

func (c *Compiler) compileLLM(text string, svcSet map[string]bool) (
	[]formal.Action, []formal.Invariant, []Translation, []string, error,
) {
	raw, err := c.cfg.LLM(llmSystemPrompt, text)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	var resp llmResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("LLM returned invalid JSON: %w", err)
	}

	var steps []formal.Action
	var warnings []string

	for _, s := range resp.Steps {
		svc := strings.ToLower(s.Target)
		warnIfUnknown(svc, svcSet, &warnings)
		switch strings.ToLower(s.Action) {
		case "restart":
			steps = append(steps, makeSetHealthy(svc, true, "restart"))
		case "set_replicas":
			if s.Value == nil {
				warnings = append(warnings, fmt.Sprintf("set_replicas for %q missing value; skipping", svc))
				continue
			}
			steps = append(steps, makeSetReplicas(svc, *s.Value))
		case "noop":
			steps = append(steps, formal.Action{Name: "noop", Fn: func(w formal.World) formal.World { return w }})
		default:
			warnings = append(warnings, fmt.Sprintf("unknown LLM action %q for %q; skipping", s.Action, svc))
		}
	}

	var invariants []formal.Invariant
	for _, inv := range resp.Invariants {
		switch inv.Kind {
		case "at_least_n_healthy":
			invariants = append(invariants, formal.AtLeastNHealthy(inv.N))
		case "min_replicas":
			invariants = append(invariants, formal.MinReplicas(strings.ToLower(inv.Service), inv.N))
		case "service_always_healthy":
			invariants = append(invariants, formal.ServiceAlwaysHealthy(strings.ToLower(inv.Service)))
		default:
			warnings = append(warnings, fmt.Sprintf("unsupported invariant kind %q; skipping", inv.Kind))
		}
	}

	warnings = append(warnings, resp.Warnings...)

	var trace []Translation
	for _, t := range resp.Trace {
		trace = append(trace, Translation{Span: t.Span, Interpreted: t.Interpreted})
	}

	return steps, invariants, trace, warnings, nil
}

// planID generates a short deterministic ID from the input text.
func planID(text string) string {
	words := strings.Fields(text)
	if len(words) > 4 {
		words = words[:4]
	}
	slug := strings.ToLower(strings.Join(words, "-"))
	// strip non-alphanumeric except dash
	var b strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return "plan-" + b.String()
}
