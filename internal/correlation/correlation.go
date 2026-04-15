package correlation

import (
	"math"
	"sort"
	"sync"
	"time"
)

type dataPoint struct {
	value     float64
	timestamp time.Time
}

type Correlation struct {
	MetricA     string        `json:"metric_a"`
	MetricB     string        `json:"metric_b"`
	Coefficient float64       `json:"coefficient"`
	Strength    string        `json:"strength"`
	LeadTime    time.Duration `json:"lead_time"`
	IsLeading   bool          `json:"is_leading"`
}

type Engine struct {
	mu        sync.RWMutex
	series    map[string][]dataPoint
	maxPoints int
}

func New() *Engine {
	return &Engine{
		series:    make(map[string][]dataPoint),
		maxPoints: 1000,
	}
}

func (e *Engine) Record(metric string, value float64) {
	e.recordAt(metric, value, time.Now())
}

func (e *Engine) recordAt(metric string, value float64, t time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.series[metric] = append(e.series[metric], dataPoint{value: value, timestamp: t})
	if len(e.series[metric]) > e.maxPoints {
		e.series[metric] = e.series[metric][len(e.series[metric])-e.maxPoints:]
	}
}

func (e *Engine) Correlate(metricA, metricB string) *Correlation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	a, okA := e.series[metricA]
	b, okB := e.series[metricB]
	if !okA || !okB || len(a) < 5 || len(b) < 5 {
		return nil
	}

	valsA, valsB := alignSeries(a, b)
	if len(valsA) < 5 {
		return nil
	}

	r := pearson(valsA, valsB)

	return &Correlation{
		MetricA:     metricA,
		MetricB:     metricB,
		Coefficient: r,
		Strength:    strengthLabel(r),
	}
}

func (e *Engine) AllCorrelations() []Correlation {
	e.mu.RLock()
	metrics := make([]string, 0, len(e.series))
	for m := range e.series {
		metrics = append(metrics, m)
	}
	e.mu.RUnlock()

	sort.Strings(metrics)

	var results []Correlation
	for i := 0; i < len(metrics); i++ {
		for j := i + 1; j < len(metrics); j++ {
			c := e.Correlate(metrics[i], metrics[j])
			if c != nil && math.Abs(c.Coefficient) > 0.3 {
				results = append(results, *c)
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return math.Abs(results[i].Coefficient) > math.Abs(results[j].Coefficient)
	})
	return results
}

func (e *Engine) LeadingIndicators(metric string) []Correlation {
	e.mu.RLock()
	target, ok := e.series[metric]
	if !ok || len(target) < 10 {
		e.mu.RUnlock()
		return nil
	}

	metrics := make([]string, 0, len(e.series))
	for m := range e.series {
		if m != metric {
			metrics = append(metrics, m)
		}
	}
	e.mu.RUnlock()

	var leaders []Correlation
	for _, m := range metrics {
		e.mu.RLock()
		other, ok := e.series[m]
		if !ok || len(other) < 10 {
			e.mu.RUnlock()
			continue
		}

		// Try shifts of 1-5 data points
		bestR := 0.0
		bestShift := 0
		baseValsA, baseValsB := alignSeries(other, target)
		if len(baseValsA) >= 5 {
			bestR = math.Abs(pearson(baseValsA, baseValsB))
		}

		for shift := 1; shift <= 5; shift++ {
			if len(other) <= shift || len(target) <= shift {
				break
			}
			// Shift other forward: other[:-shift] vs target[shift:]
			shifted := other[:len(other)-shift]
			tgt := target[shift:]
			vA, vB := alignSeries(shifted, tgt)
			if len(vA) < 5 {
				continue
			}
			r := math.Abs(pearson(vA, vB))
			if r > bestR+0.05 { // must be meaningfully better
				bestR = r
				bestShift = shift
			}
		}
		e.mu.RUnlock()

		if bestShift > 0 && bestR > 0.4 {
			// Estimate lead time from average interval
			e.mu.RLock()
			avgInterval := time.Duration(0)
			if len(other) > 1 {
				totalDur := other[len(other)-1].timestamp.Sub(other[0].timestamp)
				avgInterval = totalDur / time.Duration(len(other)-1)
			}
			e.mu.RUnlock()

			leaders = append(leaders, Correlation{
				MetricA:     m,
				MetricB:     metric,
				Coefficient: bestR,
				Strength:    strengthLabel(bestR),
				LeadTime:    avgInterval * time.Duration(bestShift),
				IsLeading:   true,
			})
		}
	}

	sort.Slice(leaders, func(i, j int) bool {
		return math.Abs(leaders[i].Coefficient) > math.Abs(leaders[j].Coefficient)
	})
	return leaders
}

func (e *Engine) StrongestCorrelation(metric string) *Correlation {
	e.mu.RLock()
	metrics := make([]string, 0, len(e.series))
	for m := range e.series {
		if m != metric {
			metrics = append(metrics, m)
		}
	}
	e.mu.RUnlock()

	var best *Correlation
	bestAbs := 0.0
	for _, m := range metrics {
		c := e.Correlate(metric, m)
		if c != nil && math.Abs(c.Coefficient) > bestAbs {
			best = c
			bestAbs = math.Abs(c.Coefficient)
		}
	}
	return best
}

func (e *Engine) Metrics() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	metrics := make([]string, 0, len(e.series))
	for m := range e.series {
		metrics = append(metrics, m)
	}
	sort.Strings(metrics)
	return metrics
}

func (e *Engine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.series = make(map[string][]dataPoint)
}

// alignSeries pairs up data points from two series by index (simplest approach).
func alignSeries(a, b []dataPoint) ([]float64, []float64) {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}

	valsA := make([]float64, n)
	valsB := make([]float64, n)
	for i := 0; i < n; i++ {
		valsA[i] = a[i].value
		valsB[i] = b[i].value
	}
	return valsA, valsB
}

func pearson(x, y []float64) float64 {
	n := float64(len(x))
	if n < 2 {
		return 0
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for i := range x {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	num := n*sumXY - sumX*sumY
	den := math.Sqrt((n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY))
	if den == 0 {
		return 0
	}
	return num / den
}

func strengthLabel(r float64) string {
	abs := math.Abs(r)
	switch {
	case abs > 0.7:
		return "strong"
	case abs > 0.4:
		return "moderate"
	case abs > 0.2:
		return "weak"
	default:
		return "none"
	}
}
