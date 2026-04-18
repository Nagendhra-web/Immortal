package twin

import (
	"strconv"
)

// AdvancedEffectModels returns all advanced effect models.
// Include these alongside BuiltinEffectModels in Config.EffectModels when enabling the advanced set.
func AdvancedEffectModels() []EffectModel {
	return []EffectModel{
		DeployEffect,
		CanaryEffect,
		TrafficShiftEffect,
		DBMigrationEffect,
		ConnectionPoolEffect,
		SecretRotationEffect,
	}
}

// DeployEffect models a new image deployment.
// params: {image: "v1.2.3"}
// Clean deploy assumption: Healthy=true, ErrorRate=0, brief CPU bump +10.
// MC noise injects failure probability via metric perturbation.
func DeployEffect(s State, a Action) (State, bool) {
	if a.Type != "deploy" {
		return s, false
	}
	// Record the image in state via no-op field manipulation (we track modeled=true).
	s.Healthy = true
	s.ErrorRate = 0
	s.CPU += 10
	if s.CPU > 100 {
		s.CPU = 100
	}
	return s, true
}

// CanaryEffect models splitting traffic to a canary instance.
// params: {percent: "10"}
// The primary target retains (100-percent)% load; a virtual "<target>-canary"
// state is returned as the next state of target with scaled load.
// Note: the canary split is encoded in the returned state, and the caller
// (Twin.Simulate) stores it under action.Target. To expose the canary state,
// CanaryEffect stores canary info in the caller state by convention:
// it SETS the primary target's CPU to (100-pct)% of original, and registers
// the canary via a side-channel stored in state; since State has no map field
// for sub-states, we return the primary-reduced state here. The test
// validates the twin's state map via the canary action's twin.states directly.
func CanaryEffect(s State, a Action) (State, bool) {
	if a.Type != "canary" {
		return s, false
	}
	pctStr, ok := a.Params["percent"]
	if !ok {
		return s, false
	}
	pct, err := strconv.ParseFloat(pctStr, 64)
	if err != nil || pct < 0 || pct > 100 {
		return s, false
	}

	fraction := pct / 100.0
	primaryFraction := 1.0 - fraction

	// Primary gets reduced load.
	primary := s
	primary.CPU = s.CPU * primaryFraction
	if primary.CPU < 0 {
		primary.CPU = 0
	}

	return primary, true
}

// canaryStateFor returns the canary State for a given source state and percent.
// Used by Simulate to register the canary side-state in the twin snapshot.
func canaryStateFor(src State, pct float64) State {
	fraction := pct / 100.0
	c := src
	c.Service = src.Service + "-canary"
	c.CPU = src.CPU * fraction
	if c.CPU < 0 {
		c.CPU = 0
	}
	c.Replicas = 1
	return c
}

// TrafficShiftEffect moves a traffic fraction from one service to another.
// params: {from, to, percent}
// CPU scales on both sides proportionally.
func TrafficShiftEffect(s State, a Action) (State, bool) {
	if a.Type != "traffic_shift" {
		return s, false
	}
	fromSvc, hasFr := a.Params["from"]
	toSvc, hasTo := a.Params["to"]
	pctStr, hasPct := a.Params["percent"]
	if !hasFr || !hasTo || !hasPct {
		return s, false
	}

	pct, err := strconv.ParseFloat(pctStr, 64)
	if err != nil || pct < 0 || pct > 100 {
		return s, false
	}
	fraction := pct / 100.0

	if a.Target == fromSvc {
		// Source loses traffic.
		s.CPU = s.CPU * (1 - fraction)
		if s.CPU < 0 {
			s.CPU = 0
		}
	} else if a.Target == toSvc {
		// Destination gains traffic.
		s.CPU = s.CPU * (1 + fraction)
		if s.CPU > 100 {
			s.CPU = 100
		}
	}

	return s, true
}

// DBMigrationEffect models a database schema migration.
// params: {duration_seconds: "30"}
// Latency spikes by 1.5x for the migration duration (best case modeled as steady state).
func DBMigrationEffect(s State, a Action) (State, bool) {
	if a.Type != "db_migration" {
		return s, false
	}
	s.Latency = s.Latency * 1.5
	if s.Latency < 0 {
		s.Latency = 0
	}
	return s, true
}

// ConnectionPoolEffect models adjusting a connection pool size.
// params: {size: "20"}
// If new size >= demand (proxy: CPU/20), ErrorRate goes to 0.
func ConnectionPoolEffect(s State, a Action) (State, bool) {
	if a.Type != "connection_pool" {
		return s, false
	}
	sizeStr, ok := a.Params["size"]
	if !ok {
		return s, false
	}
	size, err := strconv.Atoi(sizeStr)
	if err != nil || size <= 0 {
		return s, false
	}

	demand := s.CPU / 20.0
	if float64(size) >= demand {
		s.ErrorRate = 0
	}
	return s, true
}

// SecretRotationEffect models rotating secrets (e.g., DB passwords, API keys).
// params: {} (none required)
// If Healthy was true, briefly drops to Healthy=false (pool reconnect) then back.
// In deterministic mode, we model optimistic recovery (Healthy=true at end).
// MC noise injects small probability of staying unhealthy.
func SecretRotationEffect(s State, a Action) (State, bool) {
	if a.Type != "secret_rotation" {
		return s, false
	}
	// Model the reconnect: brief unhealthy period, then recovery.
	// In deterministic mode, assume recovery succeeds.
	if s.Healthy {
		// Simulate the brief drop and recovery: end state is Healthy=true.
		// (MC runs will inject noise that may flip this.)
		s.Healthy = true
	}
	return s, true
}
