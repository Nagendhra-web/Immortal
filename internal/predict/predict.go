package predict

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

// Prediction holds the result of a linear regression forecast for a metric.
type Prediction struct {
	Metric          string        `json:"metric"`
	CurrentValue    float64       `json:"current_value"`
	PredictedValue  float64       `json:"predicted_value"`
	TimeToThreshold time.Duration `json:"time_to_threshold"`
	Confidence      float64       `json:"confidence"`
	Severity        string        `json:"severity"`
	Timestamp       time.Time     `json:"timestamp"`
}

// Predictor maintains time series data per metric and produces linear regression forecasts.
type Predictor struct {
	mu         sync.RWMutex
	series     map[string][]dataPoint
	maxPoints  int
	thresholds map[string]float64
}

// New returns a Predictor with a default cap of 500 data points per metric.
func New() *Predictor {
	return &Predictor{
		series:     make(map[string][]dataPoint),
		maxPoints:  500,
		thresholds: make(map[string]float64),
	}
}

// SetThreshold sets the upper threshold for a metric (e.g. cpu_percent -> 90.0).
func (p *Predictor) SetThreshold(metric string, value float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.thresholds[metric] = value
}

// Feed adds a data point for metric using the current time.
func (p *Predictor) Feed(metric string, value float64) {
	p.feedAt(metric, value, time.Now())
}

// feedAt adds a data point with an explicit timestamp (used in tests).
func (p *Predictor) feedAt(metric string, value float64, t time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	pts := p.series[metric]
	pts = append(pts, dataPoint{value: value, timestamp: t})
	if len(pts) > p.maxPoints {
		pts = pts[len(pts)-p.maxPoints:]
	}
	p.series[metric] = pts
}

// Predict runs linear regression on the time series for metric and returns a forecast.
// Returns nil when fewer than 3 data points are available.
func (p *Predictor) Predict(metric string) *Prediction {
	p.mu.RLock()
	pts := p.series[metric]
	threshold, hasThreshold := p.thresholds[metric]
	p.mu.RUnlock()

	if len(pts) < 3 {
		return nil
	}

	slope, intercept, r2 := linreg(pts)

	current := pts[len(pts)-1].value
	origin := pts[0].timestamp
	nowSec := pts[len(pts)-1].timestamp.Sub(origin).Seconds()
	predicted := slope*nowSec + intercept

	pred := &Prediction{
		Metric:         metric,
		CurrentValue:   current,
		PredictedValue: predicted,
		Confidence:     r2,
		Severity:       "info",
		Timestamp:      time.Now(),
	}

	if hasThreshold && slope > 0 {
		// seconds until value reaches threshold: threshold = slope*t + intercept => t = (threshold - intercept) / slope
		tSec := (threshold - intercept) / slope
		remaining := tSec - nowSec
		if remaining > 0 {
			pred.TimeToThreshold = time.Duration(remaining * float64(time.Second))
			switch {
			case pred.TimeToThreshold < 5*time.Minute:
				pred.Severity = "critical"
			case pred.TimeToThreshold < 30*time.Minute:
				pred.Severity = "warning"
			}
		} else {
			// already past threshold
			pred.Severity = "critical"
		}
	}

	return pred
}

// AllPredictions returns predictions for every metric that has a threshold set.
func (p *Predictor) AllPredictions() []Prediction {
	p.mu.RLock()
	metrics := make([]string, 0, len(p.thresholds))
	for m := range p.thresholds {
		metrics = append(metrics, m)
	}
	p.mu.RUnlock()

	sort.Strings(metrics)
	out := make([]Prediction, 0, len(metrics))
	for _, m := range metrics {
		if pr := p.Predict(m); pr != nil {
			out = append(out, *pr)
		}
	}
	return out
}

// Trend returns the linear regression slope for metric (positive = increasing).
func (p *Predictor) Trend(metric string) (slope float64, ok bool) {
	p.mu.RLock()
	pts := p.series[metric]
	p.mu.RUnlock()

	if len(pts) < 3 {
		return 0, false
	}
	s, _, _ := linreg(pts)
	return s, true
}

// linreg performs ordinary least-squares linear regression on the data points.
// x is seconds since the first data point, y is value.
// Returns slope, intercept, and R² (coefficient of determination).
func linreg(pts []dataPoint) (slope, intercept, r2 float64) {
	n := float64(len(pts))
	origin := pts[0].timestamp

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for _, pt := range pts {
		x := pt.timestamp.Sub(origin).Seconds()
		y := pt.value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
		sumY2 += y * y
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, sumY / n, 0
	}

	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n

	// R² = 1 - SS_res / SS_tot
	meanY := sumY / n
	var ssTot, ssRes float64
	for _, pt := range pts {
		x := pt.timestamp.Sub(origin).Seconds()
		yHat := slope*x + intercept
		diff := pt.value - meanY
		ssTot += diff * diff
		res := pt.value - yHat
		ssRes += res * res
	}
	if ssTot == 0 {
		r2 = 1
	} else {
		r2 = 1 - ssRes/ssTot
	}
	r2 = math.Max(0, r2)
	return slope, intercept, r2
}
