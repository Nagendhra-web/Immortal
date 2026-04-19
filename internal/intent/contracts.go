package intent

// Contract presets turn intent declarations into business-language contracts:
//
//	intent.ProtectCheckout("checkout", "payments")
//	intent.NeverDropJobs("queue")
//	intent.AvailableUnderDegradation("api")
//	intent.CostCeiling(12.00)
//
// Each returns a ready-to-register Intent with sensible thresholds and high
// priorities. Operators can still build raw Intents by hand for one-off
// goals; contracts exist because the five or six most common ones deserve
// a readable shorthand.

// ProtectCheckout declares that the named services are business-critical.
// Their latency must stay under 200 ms and their error rate under 0.5%.
// They are marked as protected so, when the evaluator resolves conflicts,
// it will degrade other services before harming checkout.
func ProtectCheckout(services ...string) Intent {
	if len(services) == 0 {
		services = []string{"checkout"}
	}
	goals := make([]Goal, 0, len(services)*3)
	for _, svc := range services {
		goals = append(goals,
			Goal{Kind: ProtectService, Service: svc, Priority: 10},
			Goal{Kind: LatencyUnder, Service: svc, Target: 200, Priority: 10},
			Goal{Kind: ErrorRateUnder, Service: svc, Target: 0.005, Priority: 10},
		)
	}
	return Intent{Name: "protect-checkout", Goals: goals}
}

// NeverDropJobs declares that the given queues must not drop work under
// any circumstances. If the queues are saturating the evaluator will
// suggest scaling or spilling to disk before dropping.
func NeverDropJobs(queues ...string) Intent {
	if len(queues) == 0 {
		queues = []string{"queue"}
	}
	goals := make([]Goal, 0, len(queues))
	for _, q := range queues {
		goals = append(goals, Goal{
			Kind:     JobsNoDrop,
			Service:  q,
			Target:   0,
			Priority: 10,
		})
	}
	return Intent{Name: "never-drop-jobs", Goals: goals}
}

// AvailableUnderDegradation says: keep the service usable even when the
// system is under stress. This is weaker than ProtectCheckout. It allows
// latency to rise and non-critical features to shed so the core path works.
func AvailableUnderDegradation(services ...string) Intent {
	if len(services) == 0 {
		services = []string{"api"}
	}
	goals := make([]Goal, 0, len(services)*2)
	for _, svc := range services {
		goals = append(goals,
			Goal{Kind: AvailabilityOver, Service: svc, Target: 0.99, Priority: 7},
			Goal{Kind: ErrorRateUnder, Service: svc, Target: 0.05, Priority: 7},
		)
	}
	return Intent{Name: "available-under-degradation", Goals: goals}
}

// CostCeiling caps the aggregate $/hour the engine is allowed to spend.
// Lower priority than user-facing contracts; breached only after the engine
// has already tried to scale-in non-critical services.
func CostCeiling(dollarsPerHour float64) Intent {
	return Intent{
		Name: "cost-ceiling",
		Goals: []Goal{
			{Kind: CostCap, Service: "", Target: dollarsPerHour, Priority: 4},
		},
	}
}

// LowLatency is a milder latency goal for non-critical services. Higher
// threshold, lower priority than ProtectCheckout.
func LowLatency(services ...string) Intent {
	if len(services) == 0 {
		services = []string{"*"}
	}
	goals := make([]Goal, 0, len(services))
	for _, svc := range services {
		goals = append(goals, Goal{
			Kind:     LatencyUnder,
			Service:  svc,
			Target:   500,
			Priority: 5,
		})
	}
	return Intent{Name: "low-latency", Goals: goals}
}

// Summary returns a short business-language description of an intent that
// came from one of the preset constructors.
func Summary(i Intent) string {
	switch i.Name {
	case "protect-checkout":
		return "Protect checkout at all costs: latency <200 ms, errors <0.5%"
	case "never-drop-jobs":
		return "Never drop jobs: queue must retain every accepted unit of work"
	case "available-under-degradation":
		return "Available under degradation: keep the core path usable"
	case "cost-ceiling":
		return "Cost ceiling: cap spend before letting bills balloon"
	case "low-latency":
		return "Low latency: keep non-critical paths under 500 ms"
	default:
		return i.Name
	}
}
