package economic

import (
	"math"
	"testing"
)

const epsilon = 1e-9

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}

// baseCatalog returns a catalog suitable for most planner tests.
func baseCatalog() *Catalog {
	cat := NewCatalog()
	cat.SetServiceValue(ServiceValue{Service: "primary", USDPerMinute: 100, DependentFactor: 1.0})
	cat.SetActionCost(ActionCost{Type: "cheap-restart", InfraCostUSD: 5, EngineeringCostUSD: 5, ExpectedDowntimeSec: 0, SuccessProbability: 0.9})
	cat.SetActionCost(ActionCost{Type: "expensive-failover", InfraCostUSD: 80, EngineeringCostUSD: 20, ExpectedDowntimeSec: 0, SuccessProbability: 0.3})
	return cat
}

func incident(service string, durationMin, confidence float64, dependents ...string) IncidentProfile {
	return IncidentProfile{
		AffectedService:      service,
		AffectedDependents:   dependents,
		EstDurationMinutes:   durationMin,
		ConfidenceThisIsReal: confidence,
	}
}

func TestPlanner_PrefersHighSuccessLowCost(t *testing.T) {
	cat := baseCatalog()
	p := NewPlanner(cat)

	// Plan A: 0.9 success, $10 cost, restores in 1 min
	planA := CandidatePlan{ID: "A", Steps: []Step{{Action: "cheap-restart", Target: "primary"}}, AggregateSuccess: 0.9, ExpectedRestoreMin: 1}
	// Plan B: 0.3 success, $100 cost, restores in 1 min
	planB := CandidatePlan{ID: "B", Steps: []Step{{Action: "expensive-failover", Target: "primary"}}, AggregateSuccess: 0.3, ExpectedRestoreMin: 1}

	best := p.Pick(incident("primary", 10, 1.0), []CandidatePlan{planA, planB})
	if best == nil {
		t.Fatal("expected a plan to be picked")
	}
	if best.PlanID != "A" {
		t.Errorf("expected plan A to win, got %s", best.PlanID)
	}
}

func TestPlanner_DeclinesWhenNetNegative(t *testing.T) {
	cat := NewCatalog()
	// Very low-value service: $1/min for 2 minutes = $2 total potential loss
	cat.SetServiceValue(ServiceValue{Service: "low-value", USDPerMinute: 1, DependentFactor: 1.0})
	// Expensive action: $500 cost
	cat.SetActionCost(ActionCost{Type: "expensive-op", InfraCostUSD: 400, EngineeringCostUSD: 100, SuccessProbability: 0.9})

	p := NewPlanner(cat)
	plan := CandidatePlan{ID: "costly", Steps: []Step{{Action: "expensive-op", Target: "low-value"}}, AggregateSuccess: 0.9, ExpectedRestoreMin: 0}

	best := p.Pick(incident("low-value", 2, 1.0), []CandidatePlan{plan})
	if best != nil {
		t.Errorf("expected Pick to return nil for net-negative plan, got %+v", best)
	}
}

func TestPlanner_WeightsDependentsIntoValue(t *testing.T) {
	cat := NewCatalog()
	// Primary is very cheap on its own
	cat.SetServiceValue(ServiceValue{Service: "infra", USDPerMinute: 5, DependentFactor: 1.0})
	// Expensive action that would be net-negative without dependents
	cat.SetActionCost(ActionCost{Type: "failover", InfraCostUSD: 50, EngineeringCostUSD: 50, ExpectedDowntimeSec: 0, SuccessProbability: 1.0})

	// Without dependents: lossRate=$5/min * 10min * 1.0 success = $50 saved, cost=$100 → net=-$50 → nil
	p := NewPlanner(cat)
	planNoDepend := CandidatePlan{ID: "no-dep", Steps: []Step{{Action: "failover", Target: "infra"}}, AggregateSuccess: 1.0, ExpectedRestoreMin: 0}
	nilResult := p.Pick(incident("infra", 10, 1.0), []CandidatePlan{planNoDepend})
	if nilResult != nil {
		t.Errorf("expected nil without dependents, got %+v", nilResult)
	}

	// Add 3 dependents each worth $50/min DependentFactor=1.0 → total lossRate = 5 + 150 = $155/min
	cat.SetServiceValue(ServiceValue{Service: "dep1", USDPerMinute: 50, DependentFactor: 1.0})
	cat.SetServiceValue(ServiceValue{Service: "dep2", USDPerMinute: 50, DependentFactor: 1.0})
	cat.SetServiceValue(ServiceValue{Service: "dep3", USDPerMinute: 50, DependentFactor: 1.0})

	planWithDep := CandidatePlan{ID: "with-dep", Steps: []Step{{Action: "failover", Target: "infra"}}, AggregateSuccess: 1.0, ExpectedRestoreMin: 0}
	best := p.Pick(incident("infra", 10, 1.0, "dep1", "dep2", "dep3"), []CandidatePlan{planWithDep})
	if best == nil {
		t.Fatal("expected a plan to be picked when dependents raise value")
	}
	// bestCaseSaved = 155 * 10 = 1550, cost = 100, net = 1450 > 0
	if best.ExpectedNetValue <= 0 {
		t.Errorf("expected positive net value with dependents, got %f", best.ExpectedNetValue)
	}
}

func TestPlanner_ConfidenceScalesExpectedNetValue(t *testing.T) {
	cat := baseCatalog()
	p := NewPlanner(cat)
	plan := CandidatePlan{ID: "A", Steps: []Step{{Action: "cheap-restart", Target: "primary"}}, AggregateSuccess: 0.9, ExpectedRestoreMin: 1}

	r1 := p.Evaluate(incident("primary", 10, 1.0), []CandidatePlan{plan})
	r2 := p.Evaluate(incident("primary", 10, 0.5), []CandidatePlan{plan})

	if len(r1) == 0 || len(r2) == 0 {
		t.Fatal("expected evaluations")
	}
	// Both have same ExpectedNetValue (it doesn't include confidence); ConfidenceAdjusted should be halved
	if !approxEqual(r2[0].ConfidenceAdjusted, r1[0].ConfidenceAdjusted*0.5) {
		t.Errorf("expected halved confidence adjusted: got %f vs %f", r2[0].ConfidenceAdjusted, r1[0].ConfidenceAdjusted)
	}
	if r1[0].ExpectedNetValue != r2[0].ExpectedNetValue {
		t.Errorf("ExpectedNetValue should be independent of confidence: %f vs %f", r1[0].ExpectedNetValue, r2[0].ExpectedNetValue)
	}
}

func TestPlanner_TiebreakByLowerCost(t *testing.T) {
	cat := NewCatalog()
	cat.SetServiceValue(ServiceValue{Service: "svc", USDPerMinute: 1000, DependentFactor: 1.0})
	// Two actions with identical success probability
	cat.SetActionCost(ActionCost{Type: "cheap", InfraCostUSD: 10, EngineeringCostUSD: 0, SuccessProbability: 0.8})
	cat.SetActionCost(ActionCost{Type: "pricey", InfraCostUSD: 50, EngineeringCostUSD: 0, SuccessProbability: 0.8})

	p := NewPlanner(cat)
	planCheap := CandidatePlan{ID: "cheap-plan", Steps: []Step{{Action: "cheap", Target: "svc"}}, AggregateSuccess: 0.8, ExpectedRestoreMin: 1}
	planPricey := CandidatePlan{ID: "pricey-plan", Steps: []Step{{Action: "pricey", Target: "svc"}}, AggregateSuccess: 0.8, ExpectedRestoreMin: 1}

	best := p.Pick(incident("svc", 10, 1.0), []CandidatePlan{planPricey, planCheap})
	if best == nil {
		t.Fatal("expected a plan")
	}
	if best.PlanID != "cheap-plan" {
		t.Errorf("expected tie to be broken by lower cost, got %s", best.PlanID)
	}
}

func TestPlanner_Evaluate_ReturnsSorted(t *testing.T) {
	cat := baseCatalog()
	p := NewPlanner(cat)

	planA := CandidatePlan{ID: "A", Steps: []Step{{Action: "cheap-restart", Target: "primary"}}, AggregateSuccess: 0.9, ExpectedRestoreMin: 1}
	planB := CandidatePlan{ID: "B", Steps: []Step{{Action: "expensive-failover", Target: "primary"}}, AggregateSuccess: 0.3, ExpectedRestoreMin: 1}

	results := p.Evaluate(incident("primary", 10, 1.0), []CandidatePlan{planA, planB})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ExpectedNetValue < results[1].ExpectedNetValue {
		t.Errorf("results not sorted desc: %f < %f", results[0].ExpectedNetValue, results[1].ExpectedNetValue)
	}
}

func TestBudget_Spend_ExceededErrors(t *testing.T) {
	b := &Budget{MaxUSD: 100}
	plan := &ValuationBreakdown{PlanID: "X", PlanCostUSD: 150}

	if err := b.Spend(plan); err == nil {
		t.Fatal("expected error when budget exceeded")
	}
	// SpentUSD should not have been incremented on failure
	if b.SpentUSD != 0 {
		t.Errorf("expected SpentUSD to remain 0, got %f", b.SpentUSD)
	}
}

func TestBudget_Spend_TracksCumulative(t *testing.T) {
	b := &Budget{MaxUSD: 200}
	p1 := &ValuationBreakdown{PlanID: "X", PlanCostUSD: 80}
	p2 := &ValuationBreakdown{PlanID: "Y", PlanCostUSD: 80}
	p3 := &ValuationBreakdown{PlanID: "Z", PlanCostUSD: 80}

	if err := b.Spend(p1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := b.Spend(p2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.SpentUSD != 160 {
		t.Errorf("expected 160, got %f", b.SpentUSD)
	}
	if err := b.Spend(p3); err == nil {
		t.Fatal("expected budget exceeded error on third spend")
	}
	// Cumulative should remain at 160 (p3 was rejected)
	if b.SpentUSD != 160 {
		t.Errorf("expected spent to remain 160, got %f", b.SpentUSD)
	}
}

func TestCheckoutServiceExample(t *testing.T) {
	cat := NewCatalog()
	cat.SetServiceValue(ServiceValue{Service: "checkout", USDPerMinute: 2000, DependentFactor: 1.0})
	cat.SetServiceValue(ServiceValue{Service: "db", USDPerMinute: 500, DependentFactor: 1.0})
	cat.SetServiceValue(ServiceValue{Service: "analytics", USDPerMinute: 20, DependentFactor: 1.0})

	// restart: prob 0.9, $50 total cost, 30s self-inflicted downtime
	cat.SetActionCost(ActionCost{
		Type:                "restart",
		InfraCostUSD:        30,
		EngineeringCostUSD:  20,
		ExpectedDowntimeSec: 30,
		SuccessProbability:  0.9,
	})
	// noop: costs nothing but saves nothing either (zero success effectively)
	cat.SetActionCost(ActionCost{
		Type:               "noop",
		InfraCostUSD:       0,
		EngineeringCostUSD: 0,
		SuccessProbability: 0.0001, // nearly 0 — won't restore incident
	})

	p := NewPlanner(cat)

	// Incident on db; checkout is a dependent → totalLossRate = db(500) + checkout(2000*1.0) = 2500/min
	dbIncident := IncidentProfile{
		AffectedService:      "db",
		AffectedDependents:   []string{"checkout"},
		EstDurationMinutes:   10,
		ConfidenceThisIsReal: 0.9,
	}

	restartPlan := CandidatePlan{
		ID:                 "db-restart",
		Steps:              []Step{{Action: "restart", Target: "db"}},
		AggregateSuccess:   0.9,
		ExpectedRestoreMin: 2,
	}
	noopPlan := CandidatePlan{
		ID:                 "noop",
		Steps:              []Step{{Action: "noop", Target: "db"}},
		AggregateSuccess:   0.0001,
		ExpectedRestoreMin: 10,
	}

	best := p.Pick(dbIncident, []CandidatePlan{restartPlan, noopPlan})
	if best == nil {
		t.Fatal("expected restart plan to be picked for db incident")
	}
	if best.PlanID != "db-restart" {
		t.Errorf("expected db-restart to win, got %s", best.PlanID)
	}

	// Validate the math for the restart plan:
	// totalLossRate = 500 + 2000*1.0 = 2500/min
	// minutesSaved = 10 - 2 = 8
	// bestCaseSaved = 2500 * 8 = 20000
	// expectedLossSaved = 20000 * 0.9 = 18000
	// downtimeCost = (30/60) * 2500 = 1250
	// planCost = 30 + 20 + 1250 = 1300
	// expectedNetValue = 18000 - 1300 = 16700
	// confidenceAdjusted = 16700 * 0.9 = 15030
	if math.Abs(best.ExpectedLossSaved-18000) > 0.01 {
		t.Errorf("ExpectedLossSaved: want 18000, got %f", best.ExpectedLossSaved)
	}
	if math.Abs(best.PlanCostUSD-1300) > 0.01 {
		t.Errorf("PlanCostUSD: want 1300, got %f", best.PlanCostUSD)
	}
	if math.Abs(best.ExpectedNetValue-16700) > 0.01 {
		t.Errorf("ExpectedNetValue: want 16700, got %f", best.ExpectedNetValue)
	}
	if math.Abs(best.ConfidenceAdjusted-15030) > 0.01 {
		t.Errorf("ConfidenceAdjusted: want 15030, got %f", best.ConfidenceAdjusted)
	}

	// Analytics-only incident: $20/min, 5 min duration, restores in 1 min.
	// totalLossRate = 20/min (no dependents)
	// minutesSaved = 5 - 1 = 4; bestCaseSaved = 20 * 4 = 80; expectedLossSaved = 80 * 0.9 = 72
	// Use a dedicated expensive-analytics action with $200 fixed cost → planCost = 200
	// expectedNetValue = 72 - 200 = -128 < 0 → confidence_adjusted < 0 → nil
	cat.SetActionCost(ActionCost{
		Type:                "expensive-analytics-restart",
		InfraCostUSD:        180,
		EngineeringCostUSD:  20,
		ExpectedDowntimeSec: 0,
		SuccessProbability:  0.9,
	})
	analyticsIncident := IncidentProfile{
		AffectedService:      "analytics",
		AffectedDependents:   nil,
		EstDurationMinutes:   5,
		ConfidenceThisIsReal: 1.0,
	}
	analyticsRestartPlan := CandidatePlan{
		ID:                 "analytics-restart",
		Steps:              []Step{{Action: "expensive-analytics-restart", Target: "analytics"}},
		AggregateSuccess:   0.9,
		ExpectedRestoreMin: 1,
	}
	nilResult := p.Pick(analyticsIncident, []CandidatePlan{analyticsRestartPlan})
	if nilResult != nil {
		t.Errorf("expected Pick to return nil for low-value analytics incident, got %+v", nilResult)
	}
}
