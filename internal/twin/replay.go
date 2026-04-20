package twin

import (
	"fmt"
	"time"
)

// ReplayRequest describes a what-if replay of a historical incident.
// The caller supplies the state the system was in at the moment the
// incident began (Baseline) and a candidate Plan to apply against it.
// The twin simulates the plan and reports whether it would have improved
// the outcome vs. doing nothing.
type ReplayRequest struct {
	IncidentID string            // opaque id, surfaced in the response for audit
	Baseline   map[string]State  // state snapshot at incident start
	WithPlan   Plan              // candidate remediation
	Scorer     ScoreFunc         // optional; defaults to DefaultScore
}

// ReplayResult is the twin's verdict on the replay.
type ReplayResult struct {
	IncidentID       string        `json:"incident_id"`
	Accepted         bool          `json:"accepted"`           // true if the plan improves the score
	UnmitigatedScore float64       `json:"unmitigated_score"`  // what would have happened without intervention
	MitigatedScore   float64       `json:"mitigated_score"`    // score after the candidate plan
	ImprovementPct   float64       `json:"improvement_pct"`    // (mitigated - unmitigated) / max(|unmitigated|, epsilon)
	Counterexample   string        `json:"counterexample,omitempty"` // if rejected, which metric / service got worse
	Duration         time.Duration `json:"duration"`
}

// Replay simulates the incident's baseline state both WITH and WITHOUT
// the candidate plan, then compares scores. The score interpretation is
// delegated to the Scorer; DefaultScore() treats higher-healthier states
// as better.
//
// Replay is deterministic: given the same Baseline + Plan it returns the
// same ReplayResult. It is safe to run in CI as a pre-apply gate.
func (t *Twin) Replay(req ReplayRequest) ReplayResult {
	start := time.Now()
	scorer := req.Scorer
	if scorer == nil {
		scorer = t.cfg.Score
		if scorer == nil {
			scorer = DefaultScore
		}
	}

	// Unmitigated: do nothing, score the baseline as is.
	unmitigated := scorer(copyStates(req.Baseline))

	// Mitigated: apply the plan's actions against the baseline via the
	// twin's built-in effect models, then score the resulting state.
	mitigated := copyStates(req.Baseline)
	for _, action := range req.WithPlan.Actions {
		applyAction(mitigated, action, t.cfg.EffectModels)
	}
	mitigatedScore := scorer(mitigated)

	// Compute percent improvement, guard against div-by-zero.
	denom := unmitigated
	if denom == 0 {
		denom = 0.0001
	}
	improvementPct := ((mitigatedScore - unmitigated) / denom) * 100

	res := ReplayResult{
		IncidentID:       req.IncidentID,
		Accepted:         mitigatedScore > unmitigated,
		UnmitigatedScore: unmitigated,
		MitigatedScore:   mitigatedScore,
		ImprovementPct:   improvementPct,
		Duration:         time.Since(start),
	}
	if !res.Accepted {
		res.Counterexample = findCounterexample(req.Baseline, mitigated)
	}
	return res
}

// applyAction runs the appropriate effect model against the states map.
// Mirrors the private logic inside Simulate but exposed for Replay.
func applyAction(states map[string]State, a Action, models []EffectModel) {
	if st, ok := states[a.Target]; ok {
		for _, model := range models {
			if next, modeled := model(st, a); modeled {
				states[a.Target] = next
				break
			}
		}
	}
	// Propagate dependency effects (same policy Simulate uses).
	next := applyDependencyEffects(states, a.Target, a.Type)
	for k, v := range next {
		states[k] = v
	}
}

// findCounterexample returns a short human-readable explanation of which
// metric (in which service) moved in the wrong direction. Useful for
// explaining why a plan was rejected.
func findCounterexample(before, after map[string]State) string {
	for svc, aft := range after {
		bef, ok := before[svc]
		if !ok {
			continue
		}
		if aft.Healthy == false && bef.Healthy == true {
			return fmt.Sprintf("%s flipped from healthy to unhealthy", svc)
		}
		if aft.Latency > bef.Latency*1.5 && bef.Latency > 0 {
			return fmt.Sprintf("%s latency got 50%% worse (was %.1f ms, became %.1f ms)", svc, bef.Latency, aft.Latency)
		}
		if aft.ErrorRate > bef.ErrorRate+0.05 {
			return fmt.Sprintf("%s error rate increased by %.1f percentage points", svc, (aft.ErrorRate-bef.ErrorRate)*100)
		}
	}
	return "mitigated score <= unmitigated score (plan adds cost without benefit)"
}
