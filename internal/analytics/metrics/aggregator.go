package metrics

import (
	"math"
	"sort"
	"sync"
	"time"
)

type DataPoint struct {
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

type Summary struct {
	Count  int     `json:"count"`
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	StdDev float64 `json:"std_dev"`
	P95    float64 `json:"p95"`
	P99    float64 `json:"p99"`
	Sum    float64 `json:"sum"`
}

type Aggregator struct {
	mu      sync.RWMutex
	series  map[string][]DataPoint
	maxSize int
}

func New(maxSize int) *Aggregator {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &Aggregator{
		series:  make(map[string][]DataPoint),
		maxSize: maxSize,
	}
}

func (a *Aggregator) Record(name string, value float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	points := a.series[name]
	if len(points) >= a.maxSize {
		points = points[1:]
	}
	a.series[name] = append(points, DataPoint{Value: value, Timestamp: time.Now()})
}

func (a *Aggregator) Summarize(name string) *Summary {
	a.mu.RLock()
	defer a.mu.RUnlock()

	points, ok := a.series[name]
	if !ok || len(points) == 0 {
		return nil
	}

	values := make([]float64, len(points))
	for i, p := range points {
		values[i] = p.Value
	}
	sort.Float64s(values)

	n := len(values)
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(n)

	variance := 0.0
	for _, v := range values {
		d := v - mean
		variance += d * d
	}
	if n > 1 {
		variance /= float64(n - 1)
	}

	return &Summary{
		Count:  n,
		Mean:   mean,
		Median: percentile(values, 50),
		Min:    values[0],
		Max:    values[n-1],
		StdDev: math.Sqrt(variance),
		P95:    percentile(values, 95),
		P99:    percentile(values, 99),
		Sum:    sum,
	}
}

func (a *Aggregator) Series(name string, since time.Time) []DataPoint {
	a.mu.RLock()
	defer a.mu.RUnlock()

	points := a.series[name]
	if since.IsZero() {
		out := make([]DataPoint, len(points))
		copy(out, points)
		return out
	}

	var result []DataPoint
	for _, p := range points {
		if !p.Timestamp.Before(since) {
			result = append(result, p)
		}
	}
	return result
}

func (a *Aggregator) Names() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var names []string
	for name := range a.series {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func percentile(sorted []float64, pct float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(pct/100.0*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
