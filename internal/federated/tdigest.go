package federated

import "sort"

// centroid is a compressed cluster of values in the t-digest.
type centroid struct {
	mean   float64
	weight float64
}

// TDigest is a compact sketch supporting percentile queries and lossless merges.
// This implements the merging t-digest variant: centroids are kept sorted by
// mean; when count exceeds compression*4 the sketch is compressed by merging
// adjacent centroids whose combined weight stays within the scale-function limit.
type TDigest struct {
	centroids   []centroid
	compression int
	total       float64
}

// NewTDigest creates a TDigest with the given compression factor (default 100).
// Higher compression → more centroids → more accurate but more memory.
func NewTDigest(compression int) *TDigest {
	if compression <= 0 {
		compression = 100
	}
	return &TDigest{compression: compression}
}

// Add inserts a single value with weight 1.
func (t *TDigest) Add(x float64) {
	t.centroids = append(t.centroids, centroid{mean: x, weight: 1})
	t.total++
	// Compress when buffer is full to avoid O(N) creep.
	if len(t.centroids) > t.compression*4 {
		t.compress()
	}
}

// Count returns the total number of values added.
func (t *TDigest) Count() int {
	return int(t.total)
}

// Quantile returns an estimate of the q-th quantile (q in [0,1]).
// Uses interpolation between centroids whose cumulative weight straddles q*total.
func (t *TDigest) Quantile(q float64) float64 {
	t.compress()
	if len(t.centroids) == 0 {
		return 0
	}
	if q <= 0 {
		return t.centroids[0].mean
	}
	if q >= 1 {
		return t.centroids[len(t.centroids)-1].mean
	}

	target := q * t.total
	cumulative := 0.0
	for i, c := range t.centroids {
		lower := cumulative
		upper := cumulative + c.weight
		mid := lower + c.weight/2

		if target <= mid {
			if i == 0 {
				return c.mean
			}
			// Interpolate between previous centroid mid and this centroid mid.
			prev := t.centroids[i-1]
			prevMid := lower - prev.weight/2
			thisMid := mid
			if thisMid == prevMid {
				return c.mean
			}
			frac := (target - prevMid) / (thisMid - prevMid)
			return prev.mean + frac*(c.mean-prev.mean)
		}
		cumulative = upper
		_ = lower
	}
	return t.centroids[len(t.centroids)-1].mean
}

// Merge incorporates another TDigest's centroids into this one.
func (t *TDigest) Merge(other *TDigest) {
	t.centroids = append(t.centroids, other.centroids...)
	t.total += other.total
	t.compress()
}

// compress sorts centroids and merges adjacent ones to keep count <= compression.
func (t *TDigest) compress() {
	if len(t.centroids) == 0 {
		return
	}
	sort.Slice(t.centroids, func(i, j int) bool {
		return t.centroids[i].mean < t.centroids[j].mean
	})

	// Recount total from centroids (handles Merge cases).
	total := 0.0
	for _, c := range t.centroids {
		total += c.weight
	}
	t.total = total

	if len(t.centroids) <= t.compression {
		return
	}

	merged := make([]centroid, 0, t.compression)
	merged = append(merged, t.centroids[0])

	cumulative := t.centroids[0].weight
	for i := 1; i < len(t.centroids); i++ {
		c := t.centroids[i]
		last := &merged[len(merged)-1]

		// k-size limit: centroid at quantile q can have weight at most
		// compression * q*(1-q) * (4/total). Simplified: cap each merged
		// centroid at total/compression to keep uniform resolution.
		limit := total / float64(t.compression)
		if last.weight+c.weight <= limit {
			// Merge into last centroid using weighted mean.
			combined := last.weight + c.weight
			last.mean = (last.mean*last.weight + c.mean*c.weight) / combined
			last.weight = combined
		} else {
			merged = append(merged, c)
		}
		cumulative += c.weight
		_ = cumulative
	}

	t.centroids = merged
}
