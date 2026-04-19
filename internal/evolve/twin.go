package evolve

import (
	"fmt"
	"strings"
)

// Prediction summarises what the digital twin says about applying a
// suggestion. It is attached to a Suggestion to answer "if we do this,
// what should we expect?" before any real change touches prod.
type Prediction struct {
	MetricDeltas map[string]float64 // signed percent deltas, e.g. {"latency_p99": -0.63}
	RiskDeltas   map[string]float64 // signed percent deltas on risk surface, e.g. {"cost_per_hour": +0.15}
	Simulated    bool               // true when the values came from a real twin run
	Note         string             // one-sentence context ("Twin ran 3-min p99 scenario with +5x traffic")
}

// Describe renders the prediction into a short readable line.
// Example: "Predicted: latency_p99 -63%, cost_per_hour +15% (twin simulated)."
func (p Prediction) Describe() string {
	if len(p.MetricDeltas) == 0 && len(p.RiskDeltas) == 0 {
		return ""
	}
	var parts []string
	for k, v := range p.MetricDeltas {
		parts = append(parts, formatDelta(k, v))
	}
	for k, v := range p.RiskDeltas {
		parts = append(parts, formatDelta(k, v))
	}
	note := " (twin simulated)"
	if !p.Simulated {
		note = " (heuristic estimate)"
	}
	return "Predicted: " + strings.Join(parts, ", ") + note
}

// WithTwinPrediction attaches a Prediction to a suggestion. Returns a new
// Suggestion by value; original is unchanged.
func (s Suggestion) WithTwinPrediction(p Prediction) Suggestion {
	if p.Note != "" {
		s.Impact = p.Note + " " + s.Impact
	}
	deltas := p.Describe()
	if deltas != "" {
		s.Impact = strings.TrimSpace(s.Impact+" "+deltas) + "."
	}
	return s
}

// Rank is a score bucket that maps the 0.0-1.0 confidence onto an
// operator-friendly label (low / medium / high / critical). Useful in
// dashboards where raw decimals feel abstract.
func (s Suggestion) Rank() string {
	switch {
	case s.Score >= 0.85:
		return "critical"
	case s.Score >= 0.6:
		return "high"
	case s.Score >= 0.35:
		return "medium"
	default:
		return "low"
	}
}

func formatDelta(metric string, delta float64) string {
	sign := "+"
	if delta < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s %s%.0f%%", metric, sign, delta*100)
}
