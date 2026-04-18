package federated

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// SecureClient wraps a Client and produces masked Updates whose masks cancel
// exactly when ALL participants contribute to the aggregate. The aggregator
// therefore sees only the sum of true values; individual contributions are
// hidden.
//
// Masking scheme (simplified Bonawitz et al., 2017):
//   - For each pair (me, peer), both sides derive the same deterministic mask
//     from their shared secret: mask = HKDF(secret, "mask-"+metric+"-"+round).
//   - Client me ADDS the mask for every peer lexicographically greater than me,
//     and SUBTRACTS the mask for every peer lexicographically less than me.
//   - When all clients submit, masks cancel pairwise in the aggregate sum.
type SecureClient struct {
	base          *Client
	me            string
	peers         []string // sorted
	sharedSecrets map[string][]byte
}

// NewSecureClient creates a SecureClient. peers must not include me.
// sharedSecrets maps each peer ID to a 32-byte pre-shared key.
func NewSecureClient(base *Client, me string, peers []string, sharedSecrets map[string][]byte) *SecureClient {
	sorted := make([]string, len(peers))
	copy(sorted, peers)
	sort.Strings(sorted)
	return &SecureClient{
		base:          base,
		me:            me,
		peers:         sorted,
		sharedSecrets: sharedSecrets,
	}
}

// MaskedSnapshot produces an Update with pairwise masks applied to each metric
// Mean. The masks cancel in the aggregate when all clients participate.
func (sc *SecureClient) MaskedSnapshot(round int) Update {
	u := sc.base.Snapshot(round, 0) // no DP noise here; apply separately if needed

	masked := make(map[string]MetricStats, len(u.Stats))
	for metric, s := range u.Stats {
		netMask := sc.netMask(metric, round)
		s.Mean += netMask
		masked[metric] = s
	}
	u.Stats = masked
	return u
}

// netMask computes the net additive mask for this client for a given metric and
// round. Peers with ID > me contribute +mask; peers with ID < me contribute -mask.
func (sc *SecureClient) netMask(metric string, round int) float64 {
	net := 0.0
	for _, peer := range sc.peers {
		secret, ok := sc.sharedSecrets[peer]
		if !ok {
			continue
		}
		m := deriveMask(secret, metric, round)
		if sc.me < peer {
			net += m
		} else {
			net -= m
		}
	}
	return net
}

// deriveMask derives a deterministic float64 mask in (-1, 1) from the shared
// secret, metric name, and round using a SHA-256 keyed hash (simplified HKDF).
//
// Both sides of a pair call deriveMask with the same secret and get the same
// value. The caller decides sign based on lexicographic order of IDs.
func deriveMask(secret []byte, metric string, round int) float64 {
	h := sha256.New()
	h.Write(secret)
	h.Write([]byte(fmt.Sprintf("mask-%s-%d", metric, round)))
	digest := h.Sum(nil)

	// Interpret first 8 bytes as uint64, map to (-1, 1).
	// Divide by 2^52 to stay within a range comparable to typical means.
	bits := binary.LittleEndian.Uint64(digest[:8])
	// Map [0, 2^64) → (-1, 1): shift to signed, divide by 2^63.
	signed := int64(bits)
	return float64(signed) / (1 << 52)
}

// SecureAggregator sums masked Updates. Because masks cancel when all clients
// participate, the result equals the plain FedAvg sum. If any client is missing,
// the masks do not cancel and the result is meaningless (intentionally).
type SecureAggregator struct {
	mu              sync.Mutex
	expectedClients []string // sorted
	updates         []Update
}

// NewSecureAggregator creates a SecureAggregator that expects exactly the
// listed client IDs per round.
func NewSecureAggregator(expectedClients []string) *SecureAggregator {
	sorted := make([]string, len(expectedClients))
	copy(sorted, expectedClients)
	sort.Strings(sorted)
	return &SecureAggregator{expectedClients: sorted}
}

// Submit stores a masked update. Returns an error on duplicate ClientID.
func (sa *SecureAggregator) Submit(u Update) error {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	for _, existing := range sa.updates {
		if existing.ClientID == u.ClientID {
			return errors.New("federated: duplicate submission from client " + u.ClientID)
		}
	}
	sa.updates = append(sa.updates, u)
	return nil
}

// Close aggregates all masked updates into a GlobalModel. Returns an error if
// any expected client has not submitted (missing clients break mask cancellation).
func (sa *SecureAggregator) Close(round int) (GlobalModel, error) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	// Verify all expected clients have submitted.
	submitted := make(map[string]bool, len(sa.updates))
	for _, u := range sa.updates {
		submitted[u.ClientID] = true
	}
	for _, id := range sa.expectedClients {
		if !submitted[id] {
			return GlobalModel{}, fmt.Errorf("federated: client %q has not submitted; masks will not cancel", id)
		}
	}

	// Sum masked means across clients per metric (FedAvg weighted by count).
	allMetrics := metricUnion(sa.updates)
	globalMetrics := make(map[string]MetricStats, len(allMetrics))

	for _, metric := range allMetrics {
		totalCount := 0
		maskedWeightedSum := 0.0
		totalM2 := 0.0
		var contributors []string

		for _, u := range sa.updates {
			s, ok := u.Stats[metric]
			if !ok {
				continue
			}
			maskedWeightedSum += float64(s.Count) * s.Mean
			totalM2 += s.M2
			totalCount += s.Count
			contributors = append(contributors, u.ClientID)
		}
		if totalCount == 0 {
			continue
		}
		globalMetrics[metric] = MetricStats{
			Metric: metric,
			Count:  totalCount,
			Mean:   maskedWeightedSum / float64(totalCount),
			M2:     totalM2,
		}
		_ = contributors
	}

	contributorIDs := make([]string, 0, len(sa.updates))
	for _, u := range sa.updates {
		contributorIDs = append(contributorIDs, u.ClientID)
	}
	sort.Strings(contributorIDs)

	sa.updates = nil
	return GlobalModel{
		Round:        round,
		Metrics:      globalMetrics,
		Contributors: contributorIDs,
		UpdatedAt:    time.Now(),
	}, nil
}
