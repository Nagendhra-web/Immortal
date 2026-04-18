package plugin_test

import (
	"fmt"
	"strings"

	"github.com/immortal-engine/immortal/pkg/plugin"
)

// -------------------------------------------------------------------------
// BlueGreenSwap — EffectModel
// -------------------------------------------------------------------------

// blueGreenSwapModel teaches the digital twin how a blue/green swap changes
// service state: the target becomes healthy with reduced latency.
type blueGreenSwapModel struct{}

func (b *blueGreenSwapModel) Name() string { return "blue_green_swap" }
func (b *blueGreenSwapModel) Apply(s plugin.State, params map[string]string) (plugin.State, bool) {
	s.Healthy = true
	s.Latency *= 0.5   // new slot has lower latency
	s.ErrorRate = 0.0  // clean slate
	return s, true
}

// -------------------------------------------------------------------------
// MaxThreeUnhealthy — Invariant
// -------------------------------------------------------------------------

// maxThreeUnhealthyInvariant asserts that no more than three services may be
// unhealthy simultaneously — a cluster-wide safety property.
type maxThreeUnhealthyInvariant struct{}

func (m *maxThreeUnhealthyInvariant) Name() string { return "max_three_unhealthy" }
func (m *maxThreeUnhealthyInvariant) Description() string {
	return "at most three services may be unhealthy at the same time"
}
func (m *maxThreeUnhealthyInvariant) Holds(world map[string]plugin.ServiceState) bool {
	count := 0
	for _, s := range world {
		if !s.Healthy {
			count++
		}
	}
	return count <= 3
}

// -------------------------------------------------------------------------
// kubectl_rollout_undo — Tool
// -------------------------------------------------------------------------

// rolloutUndoTool wraps `kubectl rollout undo` for use in the agentic loop.
type rolloutUndoTool struct{}

func (r *rolloutUndoTool) Name() string        { return "kubectl_rollout_undo" }
func (r *rolloutUndoTool) Description() string { return "Undo the latest rollout for a deployment." }
func (r *rolloutUndoTool) Schema() map[string]string {
	return map[string]string{
		"namespace":  "string",
		"deployment": "string",
	}
}
func (r *rolloutUndoTool) CostTier() plugin.CostTier { return plugin.CostReversible }
func (r *rolloutUndoTool) Invoke(args map[string]any) (string, error) {
	ns, _ := args["namespace"].(string)
	dep, _ := args["deployment"].(string)
	if ns == "" || dep == "" {
		return "", fmt.Errorf("kubectl_rollout_undo: namespace and deployment are required")
	}
	// In production this would shell out to kubectl; here we simulate success.
	return fmt.Sprintf("rolled back %s/%s", ns, dep), nil
}

// -------------------------------------------------------------------------
// AcmePlugin — umbrella Plugin
// -------------------------------------------------------------------------

// AcmePlugin bundles the three contributions above into a single registerable
// unit. Third-party authors ship a type like this in their own module.
type AcmePlugin struct{}

func (a *AcmePlugin) PluginMeta() plugin.Meta {
	return plugin.Meta{
		Name:        "acme-healing-plugin",
		Version:     "1.0.0",
		Author:      "Acme Corp",
		Description: "Blue/green swap effect model, cluster invariant, and rollout-undo tool.",
		URL:         "https://github.com/acme/immortal-plugin",
	}
}

func (a *AcmePlugin) Register(r *plugin.Registry) {
	_ = r.RegisterEffectModel(&blueGreenSwapModel{})
	_ = r.RegisterInvariant(&maxThreeUnhealthyInvariant{})
	_ = r.RegisterTool(&rolloutUndoTool{})
}

// -------------------------------------------------------------------------
// Runnable Example (executed by `go test`)
// -------------------------------------------------------------------------

// ExampleRegistry_RegisterPlugin demonstrates the complete plugin authoring
// and registration workflow that a third-party developer would follow.
func ExampleRegistry_RegisterPlugin() {
	r := plugin.NewRegistry()

	if err := r.RegisterPlugin(&AcmePlugin{}); err != nil {
		fmt.Println("register error:", err)
		return
	}

	// Inspect what got registered.
	for _, m := range r.Plugins() {
		fmt.Println("plugin:", m.Name, m.Version)
	}
	for _, e := range r.EffectModels() {
		fmt.Println("effect:", e.Name())
	}
	for _, i := range r.Invariants() {
		fmt.Println("invariant:", i.Name())
	}
	for _, t := range r.Tools() {
		fmt.Println("tool:", t.Name())
	}

	// Exercise the effect model via the adapter.
	fn := plugin.AsEffectModelFn(&blueGreenSwapModel{})
	before := plugin.State{Service: "frontend", Healthy: false, Latency: 200, ErrorRate: 0.3}
	after, modeled := fn(before, "blue_green_swap", "frontend", nil)
	fmt.Println("modeled:", modeled)
	fmt.Println("healthy after swap:", after.Healthy)

	// Exercise the invariant via the adapter.
	world := map[string]plugin.ServiceState{
		"a": {Healthy: false},
		"b": {Healthy: false},
		"c": {Healthy: true},
	}
	holdsFn := plugin.AsInvariantFn(&maxThreeUnhealthyInvariant{})
	fmt.Println("invariant holds:", holdsFn(world))

	// Exercise the tool via the adapter.
	invoker := plugin.AsToolInvoker(&rolloutUndoTool{})
	result, err := invoker(map[string]any{"namespace": "prod", "deployment": "frontend"})
	if err != nil {
		fmt.Println("tool error:", err)
		return
	}
	fmt.Println("tool result:", strings.HasPrefix(result, "rolled back"))

	// Output:
	// plugin: acme-healing-plugin 1.0.0
	// effect: blue_green_swap
	// invariant: max_three_unhealthy
	// tool: kubectl_rollout_undo
	// modeled: true
	// healthy after swap: true
	// invariant holds: true
	// tool result: true
}
