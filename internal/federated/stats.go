package federated

import "math"

// welford tracks running mean and M2 (sum of squared deviations) using
// Welford's online algorithm, which is numerically stable for large streams.
type welford struct {
	count int
	mean  float64
	m2    float64
}

// update incorporates a new value into the running statistics.
func (w *welford) update(x float64) {
	w.count++
	delta := x - w.mean
	w.mean += delta / float64(w.count)
	delta2 := x - w.mean
	w.m2 += delta * delta2
}

// variance returns the sample variance (M2 / (count-1)).
// Returns 0 if count < 2.
func (w *welford) variance() float64 {
	if w.count < 2 {
		return 0
	}
	return w.m2 / float64(w.count-1)
}

// stddev returns the sample standard deviation.
func (w *welford) stddev() float64 {
	return math.Sqrt(w.variance())
}

// snapshot returns a MetricStats from the current welford state.
func (w *welford) snapshot(metric string) MetricStats {
	return MetricStats{
		Metric: metric,
		Count:  w.count,
		Mean:   w.mean,
		M2:     w.m2,
	}
}

// combineStats merges two MetricStats using Chan et al. parallel Welford combination.
// This is the core math used in FedAvg aggregation.
func combineStats(a, b MetricStats) MetricStats {
	if a.Count == 0 {
		return b
	}
	if b.Count == 0 {
		return a
	}
	combinedCount := a.Count + b.Count
	delta := b.Mean - a.Mean
	combinedMean := (float64(a.Count)*a.Mean + float64(b.Count)*b.Mean) / float64(combinedCount)
	combinedM2 := a.M2 + b.M2 + delta*delta*float64(a.Count)*float64(b.Count)/float64(combinedCount)
	return MetricStats{
		Metric: a.Metric,
		Count:  combinedCount,
		Mean:   combinedMean,
		M2:     combinedM2,
	}
}
