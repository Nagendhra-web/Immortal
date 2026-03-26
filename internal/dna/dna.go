package dna

import (
	"math"
	"sync"
)

// MetricStats holds rolling statistics for a single metric.
type MetricStats struct {
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"std_dev"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Count  int     `json:"count"`
}

// DNA represents the health fingerprint of an application.
// It learns what "normal" looks like from observed metrics.
type DNA struct {
	mu      sync.RWMutex
	source  string
	window  int
	metrics map[string]*rollingWindow
}

type rollingWindow struct {
	values []float64
	size   int
}

func newRollingWindow(size int) *rollingWindow {
	return &rollingWindow{
		values: make([]float64, 0, size),
		size:   size,
	}
}

func (rw *rollingWindow) Add(v float64) {
	if len(rw.values) >= rw.size {
		rw.values = rw.values[1:]
	}
	rw.values = append(rw.values, v)
}

func (rw *rollingWindow) Stats() MetricStats {
	n := len(rw.values)
	if n == 0 {
		return MetricStats{}
	}

	sum := 0.0
	min := rw.values[0]
	max := rw.values[0]
	for _, v := range rw.values {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	mean := sum / float64(n)

	variance := 0.0
	for _, v := range rw.values {
		diff := v - mean
		variance += diff * diff
	}
	if n > 1 {
		variance /= float64(n - 1)
	}

	return MetricStats{
		Mean:   mean,
		StdDev: math.Sqrt(variance),
		Min:    min,
		Max:    max,
		Count:  n,
	}
}

// New creates a new DNA with default window size of 1000.
func New(source string) *DNA {
	return NewWithWindow(source, 1000)
}

// NewWithWindow creates a new DNA with a custom rolling window size.
func NewWithWindow(source string, window int) *DNA {
	return &DNA{
		source:  source,
		window:  window,
		metrics: make(map[string]*rollingWindow),
	}
}

// Record adds a metric observation to the DNA.
func (d *DNA) Record(metric string, value float64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rw, ok := d.metrics[metric]
	if !ok {
		rw = newRollingWindow(d.window)
		d.metrics[metric] = rw
	}
	rw.Add(value)
}

// Baseline returns the current health baseline for all recorded metrics.
func (d *DNA) Baseline() map[string]MetricStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make(map[string]MetricStats)
	for name, rw := range d.metrics {
		result[name] = rw.Stats()
	}
	return result
}

// IsAnomaly checks if a value is anomalous for the given metric.
// Uses 3-sigma rule: anomaly if value is more than 3 standard deviations from mean.
func (d *DNA) IsAnomaly(metric string, value float64) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rw, ok := d.metrics[metric]
	if !ok || len(rw.values) < 10 {
		return false // Not enough data to determine
	}

	stats := rw.Stats()
	if stats.StdDev == 0 {
		return value != stats.Mean
	}

	zScore := math.Abs(value-stats.Mean) / stats.StdDev
	return zScore > 3.0
}

// HealthScore computes an overall health score (0.0 to 1.0) given current metric values.
// 1.0 = perfectly healthy, 0.0 = completely unhealthy.
func (d *DNA) HealthScore(current map[string]float64) float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(current) == 0 {
		return 1.0
	}

	totalScore := 0.0
	count := 0

	for metric, value := range current {
		rw, ok := d.metrics[metric]
		if !ok || len(rw.values) < 10 {
			continue
		}

		stats := rw.Stats()
		if stats.StdDev == 0 {
			// With zero deviation, use relative closeness to mean
			if stats.Mean == 0 {
				if value == 0 {
					totalScore += 1.0
				}
			} else {
				relDiff := math.Abs(value-stats.Mean) / math.Abs(stats.Mean)
				totalScore += math.Max(0, 1.0-relDiff)
			}
			count++
			continue
		}

		zScore := math.Abs(value-stats.Mean) / stats.StdDev
		// Convert z-score to health: z=0 → 1.0, z=3 → 0.0
		score := math.Max(0, 1.0-zScore/3.0)
		totalScore += score
		count++
	}

	if count == 0 {
		return 1.0
	}
	return totalScore / float64(count)
}

// Source returns the source name.
func (d *DNA) Source() string {
	return d.source
}
