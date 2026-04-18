package twin

import (
	"strconv"
)

// BuiltinEffectModels returns the default set of effect models for the Twin simulator.
// Models are tried in order; the first to return modeled=true wins.
func BuiltinEffectModels() []EffectModel {
	return []EffectModel{
		RestartEffect,
		ScaleEffect,
		FailoverEffect,
		RollbackEffect,
		NoopEffect,
	}
}

// RestartEffect models a service restart action.
// Result: Healthy=true, ErrorRate=0, CPU -= 10 (clamped >= 0).
// Dependency propagation (Latency *= 0.7, ErrorRate *= 0.5) is handled by
// Twin.applyDependencyEffects after the effect is applied.
func RestartEffect(s State, a Action) (State, bool) {
	if a.Type != "restart" {
		return s, false
	}
	s.Healthy = true
	s.ErrorRate = 0
	s.CPU -= 10
	if s.CPU < 0 {
		s.CPU = 0
	}
	return s, true
}

// ScaleEffect models a horizontal scale action.
// Requires params["replicas"] to be a valid integer.
// CPU scales inversely with replica count; Latency is reduced proportionally.
func ScaleEffect(s State, a Action) (State, bool) {
	if a.Type != "scale" {
		return s, false
	}
	replicasStr, ok := a.Params["replicas"]
	if !ok {
		return s, false
	}
	newReplicas, err := strconv.Atoi(replicasStr)
	if err != nil || newReplicas <= 0 {
		return s, false
	}
	oldReplicas := s.Replicas
	if oldReplicas <= 0 {
		oldReplicas = 1
	}
	factor := float64(newReplicas) / float64(oldReplicas)
	s.CPU = s.CPU / factor
	if s.CPU < 0 {
		s.CPU = 0
	}
	s.Latency = s.Latency / factor
	if s.Latency < 0 {
		s.Latency = 0
	}
	s.Replicas = newReplicas
	return s, true
}

// FailoverEffect models a failover to a fresh standby instance.
// Result: Healthy=true, CPU=30, ErrorRate=0.
func FailoverEffect(s State, a Action) (State, bool) {
	if a.Type != "failover" {
		return s, false
	}
	s.Healthy = true
	s.CPU = 30
	s.ErrorRate = 0
	return s, true
}

// RollbackEffect models rolling back a service to a previous known-good version.
// Result: ErrorRate=0, Latency reduced by 20% (clamped >= 0).
func RollbackEffect(s State, a Action) (State, bool) {
	if a.Type != "rollback" {
		return s, false
	}
	s.ErrorRate = 0
	s.Latency *= 0.8
	if s.Latency < 0 {
		s.Latency = 0
	}
	return s, true
}

// NoopEffect is the identity model — leaves state unchanged.
func NoopEffect(s State, a Action) (State, bool) {
	if a.Type != "noop" {
		return s, false
	}
	return s, true
}
