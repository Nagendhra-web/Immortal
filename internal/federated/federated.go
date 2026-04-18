package federated

import (
	"errors"
	"math"
	"math/rand/v2"
	"sort"
	"sync"
	"time"
)

// MetricStats summarizes a client's local observations of one metric.
type MetricStats struct {
	Metric string
	Count  int
	Mean   float64
	M2     float64 // sum of squared diffs from mean (Welford); Var = M2 / (Count-1)
}

// Update is what a client sends to the aggregator — NEVER raw metrics.
type Update struct {
	ClientID string
	Round    int
	Stats    map[string]MetricStats
	Epsilon  float64 // DP noise parameter (0 = no noise)
}

// GlobalModel is the aggregated result for a completed round.
type GlobalModel struct {
	Round        int
	Metrics      map[string]MetricStats
	Contributors []string
	UpdatedAt    time.Time
}

// Client observes raw metrics locally and produces Updates.
type Client struct {
	mu      sync.RWMutex
	id      string
	local   map[string]*welford
	global  GlobalModel
	rng     *rand.Rand
}

// NewClient creates a Client with a random seed.
func NewClient(id string) *Client {
	src := rand.NewPCG(uint64(time.Now().UnixNano()), 0)
	return &Client{
		id:    id,
		local: make(map[string]*welford),
		rng:   rand.New(src),
	}
}

// NewClientWithSeed creates a Client with a fixed seed for deterministic tests.
func NewClientWithSeed(id string, seed1, seed2 uint64) *Client {
	src := rand.NewPCG(seed1, seed2)
	return &Client{
		id:    id,
		local: make(map[string]*welford),
		rng:   rand.New(src),
	}
}

// Observe records a raw metric value locally. Thread-safe.
func (c *Client) Observe(metric string, value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	w, ok := c.local[metric]
	if !ok {
		w = &welford{}
		c.local[metric] = w
	}
	w.update(value)
}

// Snapshot produces an Update for the given round. If epsilon > 0, Laplace
// noise scaled by 1/epsilon is added to each metric Mean before sending.
// WHY: simplified Laplace; production DP needs sensitivity calibration per query budget.
func (c *Client) Snapshot(round int, epsilon float64) Update {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := make(map[string]MetricStats, len(c.local))
	for metric, w := range c.local {
		s := w.snapshot(metric)
		if epsilon > 0 {
			s.Mean += laplaceNoise(c.rng, 1.0/epsilon)
		}
		stats[metric] = s
	}
	return Update{
		ClientID: c.id,
		Round:    round,
		Stats:    stats,
		Epsilon:  epsilon,
	}
}

// ApplyGlobal stores the latest global model so the client can use it for anomaly detection.
func (c *Client) ApplyGlobal(m GlobalModel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.global = m
}

// LocalModel returns a copy of the client's local metric statistics.
func (c *Client) LocalModel() map[string]MetricStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]MetricStats, len(c.local))
	for k, w := range c.local {
		out[k] = w.snapshot(k)
	}
	return out
}

// IsAnomaly returns true if |value - mean| > 3*stddev using the last known GlobalModel.
// Returns false when the global model has fewer than 30 observations for the metric,
// or the metric is not yet in the global model.
func (c *Client) IsAnomaly(metric string, value float64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.global.Metrics[metric]
	if !ok || s.Count < 30 {
		return false
	}
	variance := 0.0
	if s.Count > 1 {
		variance = s.M2 / float64(s.Count-1)
	}
	stddev := math.Sqrt(variance)
	return math.Abs(value-s.Mean) > 3*stddev
}

// laplaceNoise draws a sample from Laplace(0, scale) using the inverse-CDF method.
func laplaceNoise(rng *rand.Rand, scale float64) float64 {
	u := rng.Float64() - 0.5
	if u == 0 {
		return 0
	}
	return -scale * math.Copysign(math.Log(1-2*math.Abs(u)), u)
}

// AggregatorConfig controls round closure and robustness.
type AggregatorConfig struct {
	MinClients      int     // round doesn't close until this many updates received
	RobustTrimRatio float64 // 0.1 = drop top 10% + bottom 10% of per-metric means
	MaxClientWeight float64 // 0 = no cap; cap each client's Count contribution
}

// Aggregator receives updates from N clients per round and computes the global model.
type Aggregator struct {
	mu      sync.RWMutex
	cfg     AggregatorConfig
	round   int
	updates []Update
	history []GlobalModel
}

// NewAggregator creates an Aggregator with the given config.
func NewAggregator(cfg AggregatorConfig) *Aggregator {
	if cfg.MinClients <= 0 {
		cfg.MinClients = 1
	}
	return &Aggregator{cfg: cfg}
}

// Submit stores an update for the current round. Returns error on duplicate ClientID.
func (a *Aggregator) Submit(u Update) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, existing := range a.updates {
		if existing.ClientID == u.ClientID {
			return errors.New("federated: duplicate submission from client " + u.ClientID)
		}
	}
	a.updates = append(a.updates, u)
	return nil
}

// Close computes the FedAvg global model for the given round and advances to the next round.
// Returns an error if fewer than MinClients have submitted.
func (a *Aggregator) Close(round int) (GlobalModel, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.updates) < a.cfg.MinClients {
		return GlobalModel{}, errors.New("federated: not enough clients for round closure")
	}

	// Collect all metric names across all updates.
	metricSet := make(map[string]struct{})
	for _, u := range a.updates {
		for m := range u.Stats {
			metricSet[m] = struct{}{}
		}
	}

	globalMetrics := make(map[string]MetricStats)
	contributorSet := make(map[string]bool)

	for metric := range metricSet {
		// Gather per-client contributions for this metric.
		type clientEntry struct {
			clientID string
			s        MetricStats
		}
		var entries []clientEntry
		for _, u := range a.updates {
			s, ok := u.Stats[metric]
			if !ok {
				continue
			}
			entries = append(entries, clientEntry{clientID: u.ClientID, s: s})
		}

		if len(entries) < a.cfg.MinClients {
			// Metric not present in enough clients; skip.
			continue
		}

		// Robust trim: sort by Mean, drop RobustTrimRatio from each end.
		if a.cfg.RobustTrimRatio > 0 && len(entries) > 2 {
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].s.Mean < entries[j].s.Mean
			})
			drop := int(math.Round(float64(len(entries)) * a.cfg.RobustTrimRatio))
			if drop > 0 && 2*drop < len(entries) {
				entries = entries[drop : len(entries)-drop]
			}
		}

		// Apply MaxClientWeight cap.
		for i := range entries {
			if a.cfg.MaxClientWeight > 0 && float64(entries[i].s.Count) > a.cfg.MaxClientWeight {
				ratio := a.cfg.MaxClientWeight / float64(entries[i].s.Count)
				entries[i].s.Count = int(a.cfg.MaxClientWeight)
				entries[i].s.M2 *= ratio
			}
		}

		// Compute FedAvg: weighted mean + combined M2 (Chan et al.).
		totalCount := 0
		for _, e := range entries {
			totalCount += e.s.Count
		}
		if totalCount == 0 {
			continue
		}

		globalMean := 0.0
		for _, e := range entries {
			globalMean += float64(e.s.Count) * e.s.Mean
		}
		globalMean /= float64(totalCount)

		globalM2 := 0.0
		for _, e := range entries {
			globalM2 += e.s.M2 + float64(e.s.Count)*math.Pow(e.s.Mean-globalMean, 2)
		}

		globalMetrics[metric] = MetricStats{
			Metric: metric,
			Count:  totalCount,
			Mean:   globalMean,
			M2:     globalM2,
		}

		for _, e := range entries {
			contributorSet[e.clientID] = true
		}
	}

	contributors := make([]string, 0, len(contributorSet))
	for id := range contributorSet {
		contributors = append(contributors, id)
	}
	sort.Strings(contributors)

	gm := GlobalModel{
		Round:        round,
		Metrics:      globalMetrics,
		Contributors: contributors,
		UpdatedAt:    time.Now(),
	}

	a.history = append(a.history, gm)
	a.updates = nil
	a.round = round + 1

	return gm, nil
}

// CurrentRound returns the current round number.
func (a *Aggregator) CurrentRound() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.round
}

// History returns all completed GlobalModels.
func (a *Aggregator) History() []GlobalModel {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]GlobalModel, len(a.history))
	copy(out, a.history)
	return out
}
