package agentic

import "fmt"

// SafetyPolicy defines the guard rails for an agent run.
type SafetyPolicy struct {
	MaxDestructivePerRun int      // default 1 when zero
	MaxDisruptivePerRun  int      // default 3 when zero
	MaxBlastRadius       int      // default 5 when zero
	ForbiddenTools       []string // never allowed
	RequireDryRunFor     []string // must be preceded by a dry_run call
}

// SafetyViolation describes a blocked tool call.
type SafetyViolation struct {
	Step   int
	Tool   string
	Reason string
}

// defaults returns a policy with zero-value fields filled in.
func (p *SafetyPolicy) defaults() (maxDest, maxDisrupt, maxBlast int) {
	maxDest = p.MaxDestructivePerRun
	if maxDest <= 0 {
		maxDest = 1
	}
	maxDisrupt = p.MaxDisruptivePerRun
	if maxDisrupt <= 0 {
		maxDisrupt = 3
	}
	maxBlast = p.MaxBlastRadius
	if maxBlast <= 0 {
		maxBlast = 5
	}
	return
}

// Guard evaluates whether the proposed tool call is safe given execution
// history. Returns nil if safe, or a *SafetyViolation describing the block.
func (p *SafetyPolicy) Guard(tool Tool, args map[string]any, history []Step) *SafetyViolation {
	step := len(history)

	// 1. Forbidden list.
	for _, f := range p.ForbiddenTools {
		if f == tool.Name {
			return &SafetyViolation{
				Step:   step,
				Tool:   tool.Name,
				Reason: fmt.Sprintf("tool %q is in the forbidden list", tool.Name),
			}
		}
	}

	maxDest, maxDisrupt, maxBlast := p.defaults()

	// 2. Blast radius.
	if tool.BlastRadius > maxBlast {
		return &SafetyViolation{
			Step:   step,
			Tool:   tool.Name,
			Reason: fmt.Sprintf("blast radius %d exceeds max %d", tool.BlastRadius, maxBlast),
		}
	}

	// Count past destructive / disruptive calls (excluding blocked steps).
	destructiveCount := 0
	disruptiveCount := 0
	dryRunTargets := make(map[string]bool) // tools for which dry_run was executed

	for _, s := range history {
		if s.Error != "" && isDryRunViolationError(s.Error) {
			// skip: blocked step, don't count
			continue
		}
		// Count dry_run observations to know which tools were pre-cleared.
		if s.Tool == "dry_run" && s.Error == "" {
			if t, ok := s.ToolArgs["tool"].(string); ok {
				dryRunTargets[t] = true
			}
		}
		// We need the tool's cost tier; we look at the step tool name and
		// match it against what we know. Since we only have the Step (not
		// the Tool struct), we use the name pattern.
		switch s.Tool {
		case "failover", "canary":
			if s.Error == "" {
				destructiveCount++
			}
		case "restart_service", "scale_service", "scale":
			if s.Error == "" {
				disruptiveCount++
			}
		}
		// Also honour CostTier if we encounter a non-builtin tool name.
	}

	// 3. Max destructive calls.
	if tool.CostTier == CostDestructive && destructiveCount >= maxDest {
		return &SafetyViolation{
			Step:   step,
			Tool:   tool.Name,
			Reason: fmt.Sprintf("destructive tool limit %d reached (already ran %d)", maxDest, destructiveCount),
		}
	}

	// 4. Max disruptive calls.
	if tool.CostTier == CostDisruptive && disruptiveCount >= maxDisrupt {
		return &SafetyViolation{
			Step:   step,
			Tool:   tool.Name,
			Reason: fmt.Sprintf("disruptive tool limit %d reached (already ran %d)", maxDisrupt, disruptiveCount),
		}
	}

	// 5. RequireDryRunFor.
	for _, req := range p.RequireDryRunFor {
		if req == tool.Name && !dryRunTargets[tool.Name] {
			return &SafetyViolation{
				Step:   step,
				Tool:   tool.Name,
				Reason: fmt.Sprintf("tool %q requires a prior dry_run", tool.Name),
			}
		}
	}

	return nil
}

// isDryRunViolationError is a tiny helper to exclude safety-blocked steps
// from counters (they were never actually executed).
func isDryRunViolationError(errStr string) bool {
	return len(errStr) > 16 && errStr[:16] == "safety violation"
}
