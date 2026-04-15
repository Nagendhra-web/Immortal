package capacity

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

type Forecast struct {
	Metric        string    `json:"metric"`
	CurrentValue  float64   `json:"current_value"`
	GrowthRate    float64   `json:"growth_rate_per_hour"`
	ExhaustionDate time.Time `json:"exhaustion_date"`
	DaysUntilFull float64   `json:"days_until_full"`
	Capacity      float64   `json:"capacity"`
	UsagePercent  float64   `json:"usage_percent"`
	Trend         string    `json:"trend"`
	Confidence    float64   `json:"confidence"`
}

type Planner struct {
	mu         sync.RWMutex
	series     map[string][]dataPoint
	capacities map[string]float64
	maxPoints  int
}

func New() *Planner {
	return &Planner{
		series:     make(map[string][]dataPoint),
		capacities: make(map[string]float64),
		maxPoints:  1000,
	}
}

func (p *Planner) SetCapacity(metric string, max float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.capacities[metric] = max
}

func (p *Planner) Record(metric string, value float64) {
	p.RecordAt(metric, value, time.Now())
}

func (p *Planner) RecordAt(metric string, value float64, t time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.series[metric] = append(p.series[metric], dataPoint{value: value, timestamp: t})
	if len(p.series[metric]) > p.maxPoints {
		p.series[metric] = p.series[metric][len(p.series[metric])-p.maxPoints:]
	}
}

func (p *Planner) Forecast(metric string) *Forecast {
	p.mu.RLock()
	defer p.mu.RUnlock()

	points, ok := p.series[metric]
	if !ok || len(points) < 3 {
		return nil
	}

	slope, intercept, rSquared := linearRegression(points)

	current := points[len(points)-1].value
	growthPerSecond := slope
	growthPerHour := growthPerSecond * 3600

	trend := "stable"
	if slope > 0.001 {
		trend = "growing"
	} else if slope < -0.001 {
		trend = "shrinking"
	}

	f := &Forecast{
		Metric:       metric,
		CurrentValue: current,
		GrowthRate:   growthPerHour,
		Trend:        trend,
		Confidence:   rSquared,
	}

	cap, hasCap := p.capacities[metric]
	if hasCap {
		f.Capacity = cap
		if cap > 0 {
			f.UsagePercent = (current / cap) * 100
		}

		if slope > 0 && current < cap {
			secondsToFull := (cap - intercept) / slope
			if secondsToFull > 0 {
				elapsed := points[len(points)-1].timestamp.Sub(points[0].timestamp).Seconds()
				remaining := secondsToFull - elapsed
				if remaining > 0 {
					f.ExhaustionDate = time.Now().Add(time.Duration(remaining) * time.Second)
					f.DaysUntilFull = remaining / 86400
				}
			}
		}
	}

	return f
}

func (p *Planner) AllForecasts() []Forecast {
	p.mu.RLock()
	metrics := make([]string, 0, len(p.capacities))
	for m := range p.capacities {
		metrics = append(metrics, m)
	}
	p.mu.RUnlock()

	var forecasts []Forecast
	for _, m := range metrics {
		f := p.Forecast(m)
		if f != nil {
			forecasts = append(forecasts, *f)
		}
	}

	sort.Slice(forecasts, func(i, j int) bool {
		if forecasts[i].DaysUntilFull == 0 && forecasts[j].DaysUntilFull == 0 {
			return forecasts[i].Metric < forecasts[j].Metric
		}
		if forecasts[i].DaysUntilFull == 0 {
			return false
		}
		if forecasts[j].DaysUntilFull == 0 {
			return true
		}
		return forecasts[i].DaysUntilFull < forecasts[j].DaysUntilFull
	})
	return forecasts
}

func (p *Planner) Critical(daysThreshold float64) []Forecast {
	all := p.AllForecasts()
	var critical []Forecast
	for _, f := range all {
		if f.DaysUntilFull > 0 && f.DaysUntilFull < daysThreshold {
			critical = append(critical, f)
		}
	}
	return critical
}

func (p *Planner) GrowthRate(metric string) (float64, bool) {
	f := p.Forecast(metric)
	if f == nil {
		return 0, false
	}
	return f.GrowthRate, true
}

func (p *Planner) Reset(metric string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.series, metric)
}

func linearRegression(points []dataPoint) (slope, intercept, rSquared float64) {
	n := float64(len(points))
	if n < 2 {
		return 0, 0, 0
	}

	t0 := points[0].timestamp

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for _, p := range points {
		x := p.timestamp.Sub(t0).Seconds()
		y := p.value
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

	// R-squared
	meanY := sumY / n
	var ssRes, ssTot float64
	for _, p := range points {
		x := p.timestamp.Sub(t0).Seconds()
		predicted := slope*x + intercept
		ssRes += (p.value - predicted) * (p.value - predicted)
		ssTot += (p.value - meanY) * (p.value - meanY)
	}

	if ssTot == 0 {
		rSquared = 1.0
	} else {
		rSquared = math.Max(0, 1.0-ssRes/ssTot)
	}

	return slope, intercept, rSquared
}
