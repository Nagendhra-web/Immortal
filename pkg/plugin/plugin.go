// Package plugin provides the public SDK for extending the Immortal engine
// without modifying or importing internal packages. Third-party authors
// implement one or more of the four extension-point interfaces (EffectModel,
// HealingAction, Tool, Invariant), bundle them into a Plugin, and register
// them with a Registry. Adapter helpers let internal consumers call plugin
// contributions through lightweight function closures.
package plugin

import (
	"errors"
	"time"
)

// -------------------------------------------------------------------------
// Sentinel errors
// -------------------------------------------------------------------------

// ErrDuplicateName is returned when a contribution with the same Name() is
// registered more than once in the same Registry.
var ErrDuplicateName = errors.New("plugin: duplicate name")

// ErrEmptyName is returned when a contribution's Name() is the empty string.
var ErrEmptyName = errors.New("plugin: empty name")

// ErrNilContribution is returned when a nil interface value is registered.
var ErrNilContribution = errors.New("plugin: nil contribution")

// -------------------------------------------------------------------------
// State — public mirror of internal/twin.State
// -------------------------------------------------------------------------

// State is a snapshot of a single service's runtime condition. It is the
// public counterpart of internal/twin.State and is bridged by adapter
// helpers; plugin code never imports internal packages.
type State struct {
	// Service is the unique name of the service this state belongs to.
	Service string
	// CPU utilisation in the range [0, 100].
	CPU float64
	// Memory utilisation in the range [0, 100].
	Memory float64
	// Replicas is the current replica count.
	Replicas int
	// Healthy reports whether the service is passing health checks.
	Healthy bool
	// Latency is the observed request latency in milliseconds.
	Latency float64
	// ErrorRate is the fraction of requests that result in errors, in [0, 1].
	ErrorRate float64
	// Dependencies lists the names of services this service depends on.
	Dependencies []string
}

// -------------------------------------------------------------------------
// EffectModel — teach the digital-twin how a new action changes state
// -------------------------------------------------------------------------

// EffectModel is the public counterpart of internal/twin.EffectModel.
// Implement this interface to register a new healing action with the
// digital-twin simulator. The simulator calls Apply for every action whose
// type string equals Name(); return (next, true) when you handle it, or
// (s, false) to pass through to the next registered model.
type EffectModel interface {
	// Name returns the unique action type this model handles, e.g. "blue_green_swap".
	Name() string
	// Apply predicts the post-action state. params mirrors twin.Action.Params.
	// Return modeled=false when actionType does not match Name().
	Apply(s State, params map[string]string) (next State, modeled bool)
}

// -------------------------------------------------------------------------
// HealingAction — match and respond to events
// -------------------------------------------------------------------------

// EventCtx is a read-only view of an event delivered to HealingAction plugin
// code. It is intentionally value-typed so plugin authors cannot mutate the
// engine's internal event.
type EventCtx struct {
	// ID is the globally unique event identifier.
	ID string
	// Source identifies the system that emitted the event, e.g. "prometheus".
	Source string
	// Severity is one of "info", "warning", "error", "critical", or "fatal".
	Severity string
	// Message is the human-readable event description.
	Message string
	// Meta carries arbitrary key-value metadata attached to the event.
	Meta map[string]any
	// Timestamp records when the event was generated.
	Timestamp time.Time
}

// HealingAction is the public counterpart of internal/healing.Rule.
// Implement this interface to add a healing rule that the engine evaluates
// against every incoming event.
type HealingAction interface {
	// Name returns a human-readable rule identifier.
	Name() string
	// Match returns true when this rule should fire for the given event.
	Match(ctx EventCtx) bool
	// Execute performs the healing action. Return a non-nil error to signal
	// that the action failed; the engine will record the failure.
	Execute(ctx EventCtx) error
}

// -------------------------------------------------------------------------
// Tool — agentic tool callable by the ReAct loop
// -------------------------------------------------------------------------

// CostTier classifies the operational impact of a Tool invocation, mirroring
// internal/agentic.CostTier. The engine's safety policy uses this to gate
// high-impact actions.
type CostTier int

const (
	// CostRead indicates a read-only operation (e.g. fetching a metric).
	CostRead CostTier = iota
	// CostReversible indicates an action that can be undone (e.g. dry-run).
	CostReversible
	// CostDisruptive indicates an action that causes service disruption (e.g. restart).
	CostDisruptive
	// CostDestructive indicates an action with wide blast radius (e.g. failover).
	CostDestructive
)

// Tool is the public counterpart of internal/agentic.Tool. Implement this
// interface to add a callable tool to the engine's agentic healing loop.
type Tool interface {
	// Name returns the unique tool identifier, e.g. "kubectl_rollout_undo".
	Name() string
	// Description explains what the tool does; used by the planner LLM.
	Description() string
	// Schema maps argument names to their type descriptions, e.g. {"namespace": "string"}.
	Schema() map[string]string
	// Invoke executes the tool and returns a string result or an error.
	Invoke(args map[string]any) (result string, err error)
	// CostTier classifies the operational impact of this tool.
	CostTier() CostTier
}

// -------------------------------------------------------------------------
// Invariant — formal safety predicate over the world state
// -------------------------------------------------------------------------

// ServiceState is the public counterpart of internal/formal.ServiceState.
// It represents the minimal state of a service used during formal verification.
type ServiceState struct {
	// Name is the service identifier.
	Name string
	// Healthy reports whether the service is currently healthy.
	Healthy bool
	// Replicas is the current replica count.
	Replicas int
}

// Invariant is the public counterpart of internal/formal.Invariant.
// Implement this interface to add a safety property that the formal model
// checker evaluates after every plan step.
type Invariant interface {
	// Name returns the invariant identifier, e.g. "at_least_one_healthy".
	Name() string
	// Description explains what property this invariant enforces.
	Description() string
	// Holds returns true iff the invariant is satisfied in the given world.
	// world maps service name to its current ServiceState.
	Holds(world map[string]ServiceState) bool
}

// -------------------------------------------------------------------------
// Plugin — umbrella interface
// -------------------------------------------------------------------------

// Meta carries metadata about a plugin, populated via PluginMeta().
type Meta struct {
	// Name is the unique plugin identifier.
	Name string
	// Version follows semantic versioning, e.g. "1.0.0".
	Version string
	// Author is the plugin author's name or organisation.
	Author string
	// Description summarises what the plugin provides.
	Description string
	// URL points to documentation or source repository.
	URL string
}

// Plugin is the top-level interface that a third-party plugin author implements.
// At minimum, a plugin declares its metadata and registers its contributions
// with the provided Registry.
type Plugin interface {
	// PluginMeta returns descriptive information about this plugin.
	PluginMeta() Meta
	// Register is called by the engine to collect the plugin's contributions.
	// Implementations should call r.RegisterEffectModel, r.RegisterHealingAction,
	// r.RegisterTool, and/or r.RegisterInvariant for each provided extension.
	Register(r *Registry)
}

// -------------------------------------------------------------------------
// Adapter helpers
// -------------------------------------------------------------------------

// AsEffectModelFn returns a plain function closure that wraps an EffectModel.
// Internal packages that cannot import this package can receive the closure via
// dependency injection and call it without depending on the plugin SDK types.
// The function signature mirrors what internal/twin.EffectModel expects: the
// caller supplies the action type, action target (unused by the closure but
// available for routing), and params; it returns the next State and whether
// the action was modeled.
func AsEffectModelFn(e EffectModel) func(s State, actionType, actionTarget string, params map[string]string) (State, bool) {
	return func(s State, actionType, actionTarget string, params map[string]string) (State, bool) {
		if actionType != e.Name() {
			return s, false
		}
		return e.Apply(s, params)
	}
}

// AsToolInvoker extracts the bare Invoke function from a Tool. Callers that
// build their own tool wrapper (e.g. internal/agentic.Tool) can use this to
// obtain the invoke function without importing the full SDK interface.
func AsToolInvoker(t Tool) func(args map[string]any) (string, error) {
	return t.Invoke
}

// AsInvariantFn returns the world predicate from an Invariant. Internal
// packages can use this to embed plugin invariants into formal.Invariant
// without importing the SDK.
func AsInvariantFn(i Invariant) func(world map[string]ServiceState) bool {
	return i.Holds
}
