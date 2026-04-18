package federated

import (
	"errors"
	"math"
	"sort"
	"sync"
	"time"
)

// KrumAggregator picks the single most-representative client update using the
// Krum algorithm (Blanchard et al., 2017). It tolerates up to F Byzantine
// (malicious or faulty) clients out of N total by selecting the client whose
// update is closest to its N-F-2 nearest neighbors.
type KrumAggregator struct {
	mu      sync.Mutex
	N, F    int
	updates []Update
}

// NewKrumAggregator creates a KrumAggregator expecting n total clients with f
// assumed malicious. Requires n > 2*f+2.
func NewKrumAggregator(n, f int) *KrumAggregator {
	return &KrumAggregator{N: n, F: f}
}

// Submit stores an update. Returns error on duplicate ClientID.
func (k *KrumAggregator) Submit(u Update) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, existing := range k.updates {
		if existing.ClientID == u.ClientID {
			return errors.New("federated: duplicate submission from client " + u.ClientID)
		}
	}
	k.updates = append(k.updates, u)
	return nil
}

// Close selects the Krum winner and returns its stats as the GlobalModel.
// Requires len(updates) == N and N > 2*F+2.
func (k *KrumAggregator) Close(round int) (GlobalModel, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	n := len(k.updates)
	if n == 0 {
		return GlobalModel{}, errors.New("federated: no updates submitted")
	}
	if n <= 2*k.F+2 {
		return GlobalModel{}, errors.New("federated: not enough clients for Krum (need N > 2F+2)")
	}

	// Build a vector per client from all metrics (sorted for determinism).
	allMetrics := metricUnion(k.updates)
	vecs := make([][]float64, n)
	for i, u := range k.updates {
		vecs[i] = updateVector(u, allMetrics)
	}

	// Score each client: sum of squared L2 distances to its N-F-2 closest peers.
	neighbors := n - k.F - 2
	scores := make([]float64, n)
	for i := 0; i < n; i++ {
		dists := make([]float64, 0, n-1)
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}
			dists = append(dists, squaredL2(vecs[i], vecs[j]))
		}
		sort.Float64s(dists)
		sum := 0.0
		for _, d := range dists[:neighbors] {
			sum += d
		}
		scores[i] = sum
	}

	// Pick the client with the minimum score.
	best := 0
	for i := 1; i < n; i++ {
		if scores[i] < scores[best] {
			best = i
		}
	}

	winner := k.updates[best]
	metrics := make(map[string]MetricStats, len(winner.Stats))
	for k, v := range winner.Stats {
		metrics[k] = v
	}

	k.updates = nil
	return GlobalModel{
		Round:        round,
		Metrics:      metrics,
		Contributors: []string{winner.ClientID},
		UpdatedAt:    time.Now(),
	}, nil
}

// TrimmedMeanAggregator computes per-metric means after dropping the F highest
// and F lowest values, providing robustness against Byzantine outliers.
type TrimmedMeanAggregator struct {
	mu      sync.Mutex
	F       int
	updates []Update
}

// NewTrimmedMeanAggregator creates a TrimmedMeanAggregator that drops f
// clients from each tail per metric.
func NewTrimmedMeanAggregator(f int) *TrimmedMeanAggregator {
	return &TrimmedMeanAggregator{F: f}
}

// Submit stores an update. Returns error on duplicate ClientID.
func (t *TrimmedMeanAggregator) Submit(u Update) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, existing := range t.updates {
		if existing.ClientID == u.ClientID {
			return errors.New("federated: duplicate submission from client " + u.ClientID)
		}
	}
	t.updates = append(t.updates, u)
	return nil
}

// Close computes per-metric trimmed means. Requires len(updates) > 2*F.
func (t *TrimmedMeanAggregator) Close(round int) (GlobalModel, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	n := len(t.updates)
	if n == 0 {
		return GlobalModel{}, errors.New("federated: no updates submitted")
	}
	if n <= 2*t.F {
		return GlobalModel{}, errors.New("federated: not enough clients for trimmed mean (need N > 2F)")
	}

	allMetrics := metricUnion(t.updates)
	globalMetrics := make(map[string]MetricStats, len(allMetrics))
	contributorSet := make(map[string]bool)

	for _, metric := range allMetrics {
		type entry struct {
			clientID string
			mean     float64
			count    int
		}
		var entries []entry
		for _, u := range t.updates {
			s, ok := u.Stats[metric]
			if !ok {
				continue
			}
			entries = append(entries, entry{clientID: u.ClientID, mean: s.Mean, count: s.Count})
		}
		if len(entries) <= 2*t.F {
			continue
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].mean < entries[j].mean
		})
		trimmed := entries[t.F : len(entries)-t.F]

		totalCount := 0
		weightedSum := 0.0
		for _, e := range trimmed {
			totalCount += e.count
			weightedSum += float64(e.count) * e.mean
			contributorSet[e.clientID] = true
		}
		if totalCount == 0 {
			continue
		}
		globalMean := weightedSum / float64(totalCount)
		globalMetrics[metric] = MetricStats{
			Metric: metric,
			Count:  totalCount,
			Mean:   globalMean,
		}
	}

	contributors := make([]string, 0, len(contributorSet))
	for id := range contributorSet {
		contributors = append(contributors, id)
	}
	sort.Strings(contributors)

	t.updates = nil
	return GlobalModel{
		Round:        round,
		Metrics:      globalMetrics,
		Contributors: contributors,
		UpdatedAt:    time.Now(),
	}, nil
}

// metricUnion returns a sorted slice of all metric names across all updates.
func metricUnion(updates []Update) []string {
	seen := make(map[string]struct{})
	for _, u := range updates {
		for m := range u.Stats {
			seen[m] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for m := range seen {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// updateVector builds a flat float64 vector from an update's metric means,
// using 0 for missing metrics to ensure consistent dimensionality.
func updateVector(u Update, metrics []string) []float64 {
	v := make([]float64, len(metrics))
	for i, m := range metrics {
		if s, ok := u.Stats[m]; ok {
			v[i] = s.Mean
		}
	}
	return v
}

// squaredL2 computes the squared Euclidean distance between two equal-length vectors.
func squaredL2(a, b []float64) float64 {
	sum := 0.0
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return sum
}

// krumScore computes the Krum score for client i given all distance vectors.
// Exported as a helper for tests; not part of public API surface.
func krumScore(dists []float64, neighbors int) float64 {
	sorted := make([]float64, len(dists))
	copy(sorted, dists)
	sort.Float64s(sorted)
	sum := 0.0
	for _, d := range sorted[:neighbors] {
		sum += d
	}
	return sum
}

// l2Distance computes the Euclidean distance between two vectors.
func l2Distance(a, b []float64) float64 {
	return math.Sqrt(squaredL2(a, b))
}

var _ = l2Distance // suppress unused warning; exposed for potential test use
