package economic

import (
	"errors"
	"sort"
)

// Planner picks the best plan by expected net value under an incident.
type Planner struct {
	cat *Catalog
}

// NewPlanner returns a Planner backed by the given Catalog.
func NewPlanner(cat *Catalog) *Planner {
	return &Planner{cat: cat}
}

// Evaluate returns a ValuationBreakdown for every candidate plan, sorted by
// ExpectedNetValue descending.
func (p *Planner) Evaluate(incident IncidentProfile, plans []CandidatePlan) []ValuationBreakdown {
	totalLossRate := p.totalLossRate(incident)

	out := make([]ValuationBreakdown, 0, len(plans))
	for _, plan := range plans {
		out = append(out, p.valuate(incident, plan, totalLossRate))
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].ExpectedNetValue != out[j].ExpectedNetValue {
			return out[i].ExpectedNetValue > out[j].ExpectedNetValue
		}
		// tie-break: lower cost first
		return out[i].PlanCostUSD < out[j].PlanCostUSD
	})
	return out
}

// Pick returns the highest-value plan, or nil if none has positive net value.
// Breaks ties by lower PlanCostUSD.
func (p *Planner) Pick(incident IncidentProfile, plans []CandidatePlan) *ValuationBreakdown {
	ranked := p.Evaluate(incident, plans)
	if len(ranked) == 0 {
		return nil
	}
	best := ranked[0]
	if best.ConfidenceAdjusted <= 0 {
		return nil
	}
	return &best
}

// totalLossRate computes USD/min lost while the incident persists.
func (p *Planner) totalLossRate(incident IncidentProfile) float64 {
	primary, _ := p.cat.ServiceValue(incident.AffectedService)
	rate := primary.USDPerMinute

	for _, dep := range incident.AffectedDependents {
		sv, ok := p.cat.ServiceValue(dep)
		if !ok {
			continue
		}
		rate += sv.USDPerMinute * sv.DependentFactor
	}
	return rate
}

// valuate computes the ValuationBreakdown for a single plan.
func (p *Planner) valuate(incident IncidentProfile, plan CandidatePlan, totalLossRate float64) ValuationBreakdown {
	// Minutes saved if plan succeeds and restores before natural end.
	minutesSaved := incident.EstDurationMinutes - plan.ExpectedRestoreMin
	if minutesSaved < 0 {
		minutesSaved = 0
	}

	bestCaseSaved := totalLossRate * minutesSaved
	worstCaseLoss := totalLossRate * incident.EstDurationMinutes
	expectedLossSaved := bestCaseSaved * plan.AggregateSuccess

	// Plan execution cost: sum over steps.
	planCost := 0.0
	for _, step := range plan.Steps {
		ac, ok := p.cat.ActionCost(step.Action)
		if !ok {
			continue
		}
		// Self-inflicted downtime cost: convert seconds to minutes then multiply by loss rate.
		downtimeCost := (ac.ExpectedDowntimeSec / 60.0) * totalLossRate
		planCost += ac.InfraCostUSD + ac.EngineeringCostUSD + downtimeCost
	}

	expectedNetValue := expectedLossSaved - planCost
	confidenceAdjusted := expectedNetValue * incident.ConfidenceThisIsReal

	return ValuationBreakdown{
		PlanID:             plan.ID,
		ExpectedLossSaved:  expectedLossSaved,
		PlanCostUSD:        planCost,
		ExpectedNetValue:   expectedNetValue,
		WorstCaseLoss:      worstCaseLoss,
		BestCaseSaved:      bestCaseSaved,
		ConfidenceAdjusted: confidenceAdjusted,
	}
}

// Budget enforces a spending cap per incident.
type Budget struct {
	MaxUSD   float64
	SpentUSD float64
}

// Spend checks if the budget allows the plan cost, records the spend, and
// returns an error if the cap would be exceeded.
func (b *Budget) Spend(plan *ValuationBreakdown) error {
	if plan == nil {
		return nil
	}
	if b.SpentUSD+plan.PlanCostUSD > b.MaxUSD {
		return errors.New("economic: budget exceeded")
	}
	b.SpentUSD += plan.PlanCostUSD
	return nil
}
