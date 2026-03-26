package brain

import (
	"sync"

	"github.com/immortal-engine/immortal/internal/dna"
	"github.com/immortal-engine/immortal/internal/event"
)

// TrendDirection indicates the direction of a metric trend.
type TrendDirection string

const (
	TrendUp     TrendDirection = "up"
	TrendDown   TrendDirection = "down"
	TrendStable TrendDirection = "stable"
)

// Prediction represents a predicted future state.
type Prediction struct {
	Metric    string         `json:"metric"`
	Direction TrendDirection `json:"direction"`
	Risk      float64        `json:"risk"` // 0.0 = no risk, 1.0 = certain failure
	Message   string         `json:"message"`
}

// PredictiveHealer observes metric events and predicts failures using trend analysis.
type PredictiveHealer struct {
	mu      sync.RWMutex
	dna     *dna.DNA
	history map[string][]float64 // recent values per metric
	window  int
}

// NewPredictiveHealer creates a predictive healer backed by a DNA baseline.
func NewPredictiveHealer(d *dna.DNA) *PredictiveHealer {
	return &PredictiveHealer{
		dna:     d,
		history: make(map[string][]float64),
		window:  20,
	}
}

// Observe records a metric event for trend analysis.
func (ph *PredictiveHealer) Observe(e *event.Event) {
	if e.Type != event.TypeMetric {
		return
	}

	ph.mu.Lock()
	defer ph.mu.Unlock()

	for key, val := range e.Meta {
		fval, ok := toFloat64(val)
		if !ok {
			continue
		}

		history := ph.history[key]
		if len(history) >= ph.window {
			history = history[1:]
		}
		ph.history[key] = append(history, fval)
	}
}

// Predict analyzes recent trends and returns predictions.
func (ph *PredictiveHealer) Predict() []Prediction {
	ph.mu.RLock()
	defer ph.mu.RUnlock()

	var predictions []Prediction

	for metric, values := range ph.history {
		if len(values) < 5 {
			continue
		}

		direction, slope := detectTrend(values)
		risk := ph.assessRisk(metric, values, slope)

		if risk > 0.3 {
			predictions = append(predictions, Prediction{
				Metric:    metric,
				Direction: direction,
				Risk:      risk,
				Message:   formatPrediction(metric, direction, risk),
			})
		}
	}

	return predictions
}

func detectTrend(values []float64) (TrendDirection, float64) {
	n := len(values)
	if n < 3 {
		return TrendStable, 0
	}

	// Simple linear regression slope
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i, v := range values {
		x := float64(i)
		sumX += x
		sumY += v
		sumXY += x * v
		sumX2 += x * x
	}

	nf := float64(n)
	slope := (nf*sumXY - sumX*sumY) / (nf*sumX2 - sumX*sumX)

	// Normalize slope relative to mean
	mean := sumY / nf
	if mean == 0 {
		mean = 1
	}
	normalizedSlope := slope / mean

	if normalizedSlope > 0.02 {
		return TrendUp, normalizedSlope
	} else if normalizedSlope < -0.02 {
		return TrendDown, normalizedSlope
	}
	return TrendStable, normalizedSlope
}

func (ph *PredictiveHealer) assessRisk(metric string, values []float64, slope float64) float64 {
	if len(values) == 0 {
		return 0
	}

	latest := values[len(values)-1]
	isAnomaly := ph.dna.IsAnomaly(metric, latest)

	risk := 0.0

	// Rising trend increases risk
	if slope > 0 {
		risk += slope * 5 // Scale slope to risk contribution
		if risk > 0.5 {
			risk = 0.5
		}
	}

	// Already anomalous adds significant risk
	if isAnomaly {
		risk += 0.5
	}

	if risk > 1.0 {
		risk = 1.0
	}
	return risk
}

func formatPrediction(metric string, direction TrendDirection, risk float64) string {
	severity := "low"
	if risk > 0.7 {
		severity = "high"
	} else if risk > 0.4 {
		severity = "medium"
	}

	switch direction {
	case TrendUp:
		return metric + " is trending upward — " + severity + " risk of threshold breach"
	case TrendDown:
		return metric + " is trending downward — " + severity + " risk of degradation"
	default:
		return metric + " — " + severity + " risk detected"
	}
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}
