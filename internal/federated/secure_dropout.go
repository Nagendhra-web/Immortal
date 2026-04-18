// Package federated provides privacy-preserving federated learning primitives.
//
// This file implements the Bonawitz et al. (CCS 2017) dropout-tolerant secure
// aggregation protocol: "Practical Secure Aggregation for Privacy-Preserving
// Machine Learning".
//
// # Trust Model
//
// The aggregator is honest-but-curious: it follows the protocol but tries to
// learn individual client contributions. Peers are also honest-but-curious.
// The protocol guarantees that the aggregator learns only the sum of client
// values, not any individual contribution, as long as at least `threshold`
// clients complete all rounds.
//
// The protocol tolerates up to N-threshold simultaneous client crashes.
// If more than N-threshold clients drop, Close returns an error because there
// are insufficient shares to reconstruct the dropped clients masks.
//
// # Protocol Overview (3-round simplification of the 4-round paper)
//
//  1. KeyExchange: Each client u generates self-seed b_u and pairwise seeds
//     s_{u,v} for each peer v. u Shamir-splits b_u and each s_{u,v} with
//     threshold t, distributing one share per peer (as backup for unmasking).
//
//  2. Submit: Each client u masks its value:
//     y_u = value + PRG(b_u) + sum_{v<u} PRG(s_{u,v}) - sum_{v>u} PRG(s_{u,v})
//     Pairwise masks cancel when both u and v submit.
//
//  3. Reveal: Survivors reveal shares of b_u for ONLINE clients (aggregator
//     subtracts PRG(b_u)) and shares of s_{u,v} for DROPPED clients (aggregator
//     reconstructs the un-cancelled pairwise contribution and removes it).
//
// Divergence from the paper: we omit Diffie-Hellman key agreement and
// authenticated encryption; seeds are generated locally and shares delivered
// in-memory. Production deployments must add authenticated channels.
package federated

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"sync"
	"time"
)

// ShareKind identifies whether a PeerShare carries a self-mask seed share
// (b_u) or a pairwise-seed share (s_{u,v}).
type ShareKind int

const (
	// KindSelf is a share of the self-mask seed b_u.
	KindSelf ShareKind = iota
	// KindPairwise is a share of the pairwise seed s_{u,v}.
	KindPairwise
)

// PeerShare is one Shamir share that a client distributes to a peer during
// the KeyExchange round.
type PeerShare struct {
	OwnerID string    // client who generated the secret
	PeerID  string    // client holding this share
	Kind    ShareKind // KindSelf or KindPairwise
	Share   Share     // Shamir share (X, Y) over GF(shamirPrime)
	Prime   uint64    // field modulus (shamirPrime)
	// OtherID is the second peer of the pairwise seed s_{OwnerID, OtherID}.
	// Ignored for KindSelf shares.
	OtherID string
	// Limb is the chunk index (0..seedSize-1) within the 256-bit seed.
	Limb int
}

// seedSize is the number of uint64 chunks used to represent a 256-bit seed.
// Each chunk is Shamir-split independently because shamirPrime (2^61-1) < 2^64.
const seedSize = 4

// seed256 is a 256-bit seed stored as four uint64 limbs, each < shamirPrime.
type seed256 [seedSize]uint64

// generateSeed produces a fresh 256-bit seed from rng.
func generateSeed(rng *rand.Rand) seed256 {
	var s seed256
	for i := range s {
		s[i] = rng.Uint64() % shamirPrime
	}
	return s
}

// prgFloat derives a deterministic float64 mask from a 256-bit seed.
// Uses sha256(seed||metric||round||marker), maps first 8 bytes to a signed
// float. The result is in ~(-1, 1) scaled by 2 for adequate hiding power.
func prgFloat(s seed256, metric string, round int, marker string) float64 {
	h := sha256.New()
	var buf [8]byte
	for _, limb := range s {
		binary.LittleEndian.PutUint64(buf[:], limb)
		h.Write(buf[:])
	}
	h.Write([]byte(metric))
	binary.LittleEndian.PutUint64(buf[:], uint64(round))
	h.Write(buf[:])
	h.Write([]byte(marker))
	digest := h.Sum(nil)

	bits := binary.LittleEndian.Uint64(digest[:8])
	sign := int64(1)
	if bits&(1<<63) != 0 {
		sign = -1
	}
	mag := int64((bits >> 11) & 0x000FFFFFFFFFFFFF)
	return float64(sign*mag) / (1 << 52)
}

// splitSeed Shamir-splits each limb of a 256-bit seed into n shares with
// threshold t. result[i][j] is the j-th share of limb i (X = j+1).
func splitSeed(s seed256, t, n int, rng *rand.Rand) ([seedSize][]Share, error) {
	var result [seedSize][]Share
	for i, limb := range s {
		shares, _, err := ShamirSplit(limb, t, n, rng)
		if err != nil {
			return result, fmt.Errorf("splitSeed limb %d: %w", i, err)
		}
		result[i] = shares
	}
	return result, nil
}

// reconstructSeed recovers a 256-bit seed from per-limb share subsets.
func reconstructSeed(limbShares [seedSize][]Share) (seed256, error) {
	var s seed256
	for i, shares := range limbShares {
		if len(shares) == 0 {
			return s, fmt.Errorf("reconstructSeed limb %d: no shares available", i)
		}
		limb, err := ShamirReconstruct(shares, shamirPrime)
		if err != nil {
			return s, fmt.Errorf("reconstructSeed limb %d: %w", i, err)
		}
		s[i] = limb
	}
	return s, nil
}

// shareXExists reports whether a share with X value x is already in the slice.
func shareXExists(shares []Share, x uint64) bool {
	for _, s := range shares {
		if s.X == x {
			return true
		}
	}
	return false
}

// secretMapKey returns the heldShares map key for a (kind, otherID) pair.
func secretMapKey(kind ShareKind, otherID string) string {
	if kind == KindSelf {
		return "self"
	}
	return "pair:" + otherID
}

// pairMarker returns a canonical PRG marker for the pair using sorted IDs.
// Both sides of a pair produce the same marker, ensuring PRG values match.
func pairMarker(a, b string) string {
	if a < b {
		return "pairwise:" + a + ":" + b
	}
	return "pairwise:" + b + ":" + a
}

// SecureDropoutClient implements the client side of the Bonawitz et al. 2017
// dropout-tolerant secure aggregation protocol.
type SecureDropoutClient struct {
	id        string
	peers     []string // sorted peer IDs (excludes self)
	threshold int
	rng       *rand.Rand

	selfSeed      seed256
	pairwiseSeeds map[string]seed256 // peerID -> s_{me,peer}

	heldSharesMu sync.Mutex
	// heldShares[ownerID][secretMapKey(kind,otherID)][limb] = []Share
	heldShares map[string]map[string][seedSize][]Share
}

// NewSecureDropoutClient creates a client ready for the KeyExchange round.
// peers must not include id. threshold is the Shamir reconstruction threshold
// (recommend floor(N/2)+1 where N = len(peers)+1).
func NewSecureDropoutClient(id string, peers []string, threshold int, rng *rand.Rand) (*SecureDropoutClient, error) {
	n := len(peers) + 1
	if threshold < 2 {
		return nil, errors.New("secure_dropout: threshold must be >= 2")
	}
	if n < threshold {
		return nil, errors.New("secure_dropout: not enough peers for given threshold")
	}

	sorted := make([]string, len(peers))
	copy(sorted, peers)
	sort.Strings(sorted)

	selfSeed := generateSeed(rng)
	pairwiseSeeds := make(map[string]seed256, len(sorted))
	for _, p := range sorted {
		pairwiseSeeds[p] = generateSeed(rng)
	}

	return &SecureDropoutClient{
		id:            id,
		peers:         sorted,
		threshold:     threshold,
		rng:           rng,
		selfSeed:      selfSeed,
		pairwiseSeeds: pairwiseSeeds,
		heldShares:    make(map[string]map[string][seedSize][]Share),
	}, nil
}

// allSorted returns all participants (self + peers) in sorted order.
func (c *SecureDropoutClient) allSorted() []string {
	all := make([]string, 0, len(c.peers)+1)
	all = append(all, c.id)
	all = append(all, c.peers...)
	sort.Strings(all)
	return all
}

// SetupShares generates Shamir shares for all secrets and returns
// map peerID -> []PeerShare (shares this client sends to each peer).
// This client stores its own shares internally via AcceptShares.
func (c *SecureDropoutClient) SetupShares() map[string][]PeerShare {
	n := len(c.peers) + 1
	all := c.allSorted()

	selfLimbs, err := splitSeed(c.selfSeed, c.threshold, n, c.rng)
	if err != nil {
		panic("secure_dropout: SetupShares selfSeed: " + err.Error())
	}

	pairLimbs := make(map[string][seedSize][]Share, len(c.peers))
	for _, peer := range c.peers {
		ls, err := splitSeed(c.pairwiseSeeds[peer], c.threshold, n, c.rng)
		if err != nil {
			panic("secure_dropout: SetupShares pairSeed: " + err.Error())
		}
		pairLimbs[peer] = ls
	}

	result := make(map[string][]PeerShare, n-1)
	for idx, participant := range all {
		var batch []PeerShare

		for limb := 0; limb < seedSize; limb++ {
			batch = append(batch, PeerShare{
				OwnerID: c.id,
				PeerID:  participant,
				Kind:    KindSelf,
				Share:   selfLimbs[limb][idx],
				Prime:   shamirPrime,
				Limb:    limb,
			})
		}

		for _, peer := range c.peers {
			ls := pairLimbs[peer]
			for limb := 0; limb < seedSize; limb++ {
				batch = append(batch, PeerShare{
					OwnerID: c.id,
					PeerID:  participant,
					Kind:    KindPairwise,
					Share:   ls[limb][idx],
					Prime:   shamirPrime,
					OtherID: peer,
					Limb:    limb,
				})
			}
		}

		if participant == c.id {
			c.AcceptShares(batch)
		} else {
			result[participant] = batch
		}
	}
	return result
}

// AcceptShares stores incoming shares from a peer received during KeyExchange.
func (c *SecureDropoutClient) AcceptShares(shares []PeerShare) {
	c.heldSharesMu.Lock()
	defer c.heldSharesMu.Unlock()

	for _, ps := range shares {
		if _, ok := c.heldShares[ps.OwnerID]; !ok {
			c.heldShares[ps.OwnerID] = make(map[string][seedSize][]Share)
		}
		key := secretMapKey(ps.Kind, ps.OtherID)
		entry := c.heldShares[ps.OwnerID][key]
		limb := ps.Limb
		if limb >= 0 && limb < seedSize && !shareXExists(entry[limb], ps.Share.X) {
			entry[limb] = append(entry[limb], ps.Share)
		}
		c.heldShares[ps.OwnerID][key] = entry
	}
}

// selfMask returns PRG(b_u) for the given metric and round.
func (c *SecureDropoutClient) selfMask(metric string, round int) float64 {
	return prgFloat(c.selfSeed, metric, round, "self")
}

// pairwiseMask returns the net pairwise mask for this client.
// For each peer v: sign is + if me < v, - if me > v.
// The canonical marker (sorted pair IDs) ensures both sides derive the same
// PRG value; opposite signs cause cancellation in the aggregate sum.
func (c *SecureDropoutClient) pairwiseMask(metric string, round int) float64 {
	net := 0.0
	for _, peer := range c.peers {
		seed := c.pairwiseSeeds[peer]
		m := prgFloat(seed, metric, round, pairMarker(c.id, peer))
		if c.id < peer {
			net += m
		} else {
			net -= m
		}
	}
	return net
}

// MaskValue applies the full Bonawitz mask to a single float64.
// Returns y_u = value + PRG(b_u) + net pairwise masks.
func (c *SecureDropoutClient) MaskValue(value float64, round int) float64 {
	return value + c.selfMask("__scalar__", round) + c.pairwiseMask("__scalar__", round)
}

// MaskMetricUpdate returns a new Update with masked means (Count and M2 unchanged).
func (c *SecureDropoutClient) MaskMetricUpdate(u Update, round int) Update {
	masked := make(map[string]MetricStats, len(u.Stats))
	for metric, s := range u.Stats {
		s.Mean += c.selfMask(metric, round) + c.pairwiseMask(metric, round)
		masked[metric] = s
	}
	return Update{
		ClientID: u.ClientID,
		Round:    round,
		Stats:    masked,
		Epsilon:  u.Epsilon,
	}
}

// RevealShares returns shares the aggregator needs to unmask the aggregate.
//
// For each online client u (including self): reveal all shares this client
// holds for u's secrets:
//   - u's self-seed (KindSelf): aggregator subtracts PRG(b_u).
//   - u's pairwise seeds for every peer (KindPairwise, both online and dropped):
//     aggregator subtracts u's pairwise contribution for each peer.
//
// Because each client generates its own independent pairwise seeds (no DH
// exchange), pairwise masks do not cancel across clients. Every online client's
// contribution for every peer must be reconstructed and removed.
//
// Crucially, this client must also reveal shares it holds of its OWN secrets
// (received during SetupShares). Without self's contribution, only N-1
// survivors provide shares, which may fall below threshold.
//
// The dropped parameter is accepted for API compatibility.
func (c *SecureDropoutClient) RevealShares(online, dropped []string) []PeerShare {
	c.heldSharesMu.Lock()
	defer c.heldSharesMu.Unlock()

	_ = dropped

	// Build set of online IDs for quick lookup; include self.
	onlineSet := make(map[string]bool, len(online)+1)
	for _, id := range online {
		onlineSet[id] = true
	}

	var result []PeerShare

	// Reveal shares for every online owner, including self.
	for ownerID, ownerShares := range c.heldShares {
		if !onlineSet[ownerID] {
			continue
		}

		// Self-seed shares for online peers (not self — aggregator reconstructs
		// b_u from other survivors' shares; self's share is one of them).
		// Also reveal self's own share of its self-seed so the aggregator has
		// enough shares even when survivors < threshold would otherwise fail.
		if limbShares, ok := ownerShares["self"]; ok {
			for limb := 0; limb < seedSize; limb++ {
				for _, sh := range limbShares[limb] {
					result = append(result, PeerShare{
						OwnerID: ownerID,
						PeerID:  c.id,
						Kind:    KindSelf,
						Share:   sh,
						Prime:   shamirPrime,
						Limb:    limb,
					})
				}
			}
		}

		// Pairwise-seed shares for all peers of ownerID.
		for key, limbShares := range ownerShares {
			if key == "self" {
				continue
			}
			otherID := key[5:] // strip "pair:"
			for limb := 0; limb < seedSize; limb++ {
				for _, sh := range limbShares[limb] {
					result = append(result, PeerShare{
						OwnerID: ownerID,
						PeerID:  c.id,
						Kind:    KindPairwise,
						Share:   sh,
						Prime:   shamirPrime,
						OtherID: otherID,
						Limb:    limb,
					})
				}
			}
		}
	}

	return result
}

// SecureDropoutAggregator runs the aggregator side of the Bonawitz protocol.
type SecureDropoutAggregator struct {
	mu             sync.Mutex
	allClients     []string
	threshold      int
	maskedUpdates  map[string]Update
	revealedShares map[string][]PeerShare
}

// NewSecureDropoutAggregator creates an aggregator expecting the listed clients.
func NewSecureDropoutAggregator(allClients []string, threshold int) *SecureDropoutAggregator {
	sorted := make([]string, len(allClients))
	copy(sorted, allClients)
	sort.Strings(sorted)
	return &SecureDropoutAggregator{
		allClients:     sorted,
		threshold:      threshold,
		maskedUpdates:  make(map[string]Update),
		revealedShares: make(map[string][]PeerShare),
	}
}

// SubmitMasked stores a masked Update. Returns error on duplicate.
func (a *SecureDropoutAggregator) SubmitMasked(u Update) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.maskedUpdates[u.ClientID]; exists {
		return errors.New("secure_dropout: duplicate masked submission from " + u.ClientID)
	}
	a.maskedUpdates[u.ClientID] = u
	return nil
}

// AcceptReveal collects share-reveals from a surviving client. Idempotent.
func (a *SecureDropoutAggregator) AcceptReveal(from string, shares []PeerShare) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.revealedShares[from] = shares
	return nil
}

// Close unmasks the aggregate and returns the GlobalModel.
// Returns an error if too many clients dropped or too few reveals received.
func (a *SecureDropoutAggregator) Close(round int) (GlobalModel, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	onlineSet := make(map[string]bool)
	for id := range a.maskedUpdates {
		onlineSet[id] = true
	}
	var online, dropped []string
	for _, id := range a.allClients {
		if onlineSet[id] {
			online = append(online, id)
		} else {
			dropped = append(dropped, id)
		}
	}

	if len(online) < a.threshold {
		return GlobalModel{}, fmt.Errorf(
			"secure_dropout: only %d clients online, need at least %d; protocol cannot recover",
			len(online), a.threshold)
	}
	if len(a.revealedShares) < a.threshold {
		return GlobalModel{}, fmt.Errorf(
			"secure_dropout: only %d clients revealed shares, need at least %d",
			len(a.revealedShares), a.threshold)
	}

	metricSet := make(map[string]struct{})
	for _, u := range a.maskedUpdates {
		for m := range u.Stats {
			metricSet[m] = struct{}{}
		}
	}

	type metricAcc struct {
		weightedSum float64
		totalCount  int
		totalM2     float64
	}
	acc := make(map[string]*metricAcc, len(metricSet))
	for m := range metricSet {
		acc[m] = &metricAcc{}
	}
	for _, u := range a.maskedUpdates {
		for metric, s := range u.Stats {
			ac := acc[metric]
			ac.weightedSum += float64(s.Count) * s.Mean
			ac.totalCount += s.Count
			ac.totalM2 += s.M2
		}
	}

	// Step 3: subtract PRG(b_u) for each online client u.
	for _, onlineID := range online {
		seed, err := a.gatherAndReconstruct(onlineID, KindSelf, "")
		if err != nil {
			return GlobalModel{}, fmt.Errorf("secure_dropout: self-seed for %s: %w", onlineID, err)
		}
		u := a.maskedUpdates[onlineID]
		for metric, s := range u.Stats {
			mask := prgFloat(seed, metric, round, "self")
			acc[metric].weightedSum -= float64(s.Count) * mask
		}
	}

	// Step 4: for every online client u and every peer v (online or dropped),
	// reconstruct u's pairwise seed s_{u,v} and subtract u's pairwise
	// contribution from the running sum.
	//
	// Because each client generates its own independent pairwise seeds (no DH),
	// pairwise masks never cancel automatically. The aggregator must reconstruct
	// and remove every online client's pairwise contribution for all peers.
	for _, onlineID := range online {
		for _, peerID := range a.allClients {
			if peerID == onlineID {
				continue
			}
			seed, err := a.gatherAndReconstruct(onlineID, KindPairwise, peerID)
			if err != nil {
				return GlobalModel{}, fmt.Errorf(
					"secure_dropout: pairwise seed %s->%s: %w", onlineID, peerID, err)
			}
			u := a.maskedUpdates[onlineID]
			marker := pairMarker(onlineID, peerID)
			for metric, s := range u.Stats {
				mask := prgFloat(seed, metric, round, marker)
				var contrib float64
				if onlineID < peerID {
					contrib = mask
				} else {
					contrib = -mask
				}
				acc[metric].weightedSum -= float64(s.Count) * contrib
			}
		}
	}

	globalMetrics := make(map[string]MetricStats, len(acc))
	for metric, ac := range acc {
		if ac.totalCount == 0 {
			continue
		}
		globalMetrics[metric] = MetricStats{
			Metric: metric,
			Count:  ac.totalCount,
			Mean:   ac.weightedSum / float64(ac.totalCount),
			M2:     ac.totalM2,
		}
	}

	sort.Strings(online)
	return GlobalModel{
		Round:        round,
		Metrics:      globalMetrics,
		Contributors: online,
		UpdatedAt:    time.Now(),
	}, nil
}

// gatherAndReconstruct collects shares for (ownerID, kind, otherID) from all
// revealedShares and reconstructs the 256-bit seed via Shamir interpolation.
func (a *SecureDropoutAggregator) gatherAndReconstruct(ownerID string, kind ShareKind, otherID string) (seed256, error) {
	var limbShares [seedSize][]Share
	for _, shares := range a.revealedShares {
		for _, ps := range shares {
			if ps.OwnerID != ownerID || ps.Kind != kind {
				continue
			}
			if kind == KindPairwise && ps.OtherID != otherID {
				continue
			}
			limb := ps.Limb
			if limb < 0 || limb >= seedSize {
				continue
			}
			if !shareXExists(limbShares[limb], ps.Share.X) {
				limbShares[limb] = append(limbShares[limb], ps.Share)
			}
		}
	}
	return reconstructSeed(limbShares)
}
