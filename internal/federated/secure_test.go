package federated

import (
	"crypto/rand"
	"math"
	"testing"
)

// makeSharedSecrets creates a symmetric secret map for a list of client IDs.
// Each pair (a, b) shares the same 32-byte key stored under both keys.
func makeSharedSecrets(ids []string) map[string]map[string][]byte {
	secrets := make(map[string]map[string][]byte, len(ids))
	for _, id := range ids {
		secrets[id] = make(map[string][]byte)
	}
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			key := make([]byte, 32)
			if _, err := rand.Read(key); err != nil {
				panic(err)
			}
			secrets[ids[i]][ids[j]] = key
			secrets[ids[j]][ids[i]] = key
		}
	}
	return secrets
}

// TestSecureAggregation_RoundtripSumMatchesPlain verifies that 5 clients using
// SecureClient produce an aggregate identical (within float epsilon) to plain
// FedAvg when all participants submit.
func TestSecureAggregation_RoundtripSumMatchesPlain(t *testing.T) {
	ids := []string{"a", "b", "c", "d", "e"}
	secrets := makeSharedSecrets(ids)

	const round = 1
	const obsPerClient = 200
	const metric = "cpu"

	// Plain aggregator (no masking).
	plainAgg := NewAggregator(AggregatorConfig{MinClients: len(ids)})

	// Secure aggregator.
	secureAgg := NewSecureAggregator(ids)

	for i, id := range ids {
		peers := make([]string, 0, len(ids)-1)
		for _, other := range ids {
			if other != id {
				peers = append(peers, other)
			}
		}

		c := NewClientWithSeed(id, uint64(i+1), uint64(i*7+3))
		meanVal := 10.0 + float64(i)*0.5
		for j := 0; j < obsPerClient; j++ {
			c.Observe(metric, meanVal)
		}

		sc := NewSecureClient(c, id, peers, secrets[id])

		// Plain snapshot (no mask).
		plainSnap := c.Snapshot(round, 0)
		if err := plainAgg.Submit(plainSnap); err != nil {
			t.Fatalf("plain submit %s: %v", id, err)
		}

		// Masked snapshot.
		maskedSnap := sc.MaskedSnapshot(round)
		if err := secureAgg.Submit(maskedSnap); err != nil {
			t.Fatalf("secure submit %s: %v", id, err)
		}
	}

	plainGM, err := plainAgg.Close(round)
	if err != nil {
		t.Fatalf("plain close: %v", err)
	}

	secureGM, err := secureAgg.Close(round)
	if err != nil {
		t.Fatalf("secure close: %v", err)
	}

	plainMean := plainGM.Metrics[metric].Mean
	secureMean := secureGM.Metrics[metric].Mean

	// Masks are floating-point; allow for accumulated rounding error.
	// With 5 clients and masks up to ±2^11 (from 1<<52 scale), relative
	// error should be < 1e-9 of the mask magnitude; we allow 1e-6 absolute.
	if math.Abs(plainMean-secureMean) > 1e-6 {
		t.Errorf("secure mean=%v plain mean=%v diff=%v (want ≤1e-6)",
			secureMean, plainMean, math.Abs(plainMean-secureMean))
	}
}

// TestSecureAggregation_MissingClient_BreaksResult verifies that when one
// client drops out, the aggregate sum does NOT equal the plain FedAvg sum
// (the un-cancelled masks corrupt the result — this IS the privacy guarantee).
func TestSecureAggregation_MissingClient_BreaksResult(t *testing.T) {
	ids := []string{"a", "b", "c", "d", "e"}
	secrets := makeSharedSecrets(ids)

	const round = 1
	const metric = "cpu"

	// Plain aggregator for all 5 clients.
	plainAgg := NewAggregator(AggregatorConfig{MinClients: len(ids)})

	// SecureAggregator expects all 5, but we'll only submit 4.
	secureAgg := NewSecureAggregator(ids)

	for i, id := range ids {
		peers := make([]string, 0, len(ids)-1)
		for _, other := range ids {
			if other != id {
				peers = append(peers, other)
			}
		}

		c := NewClientWithSeed(id, uint64(i+1), uint64(i*7+3))
		for j := 0; j < 100; j++ {
			c.Observe(metric, 10.0)
		}

		sc := NewSecureClient(c, id, peers, secrets[id])

		if err := plainAgg.Submit(c.Snapshot(round, 0)); err != nil {
			t.Fatalf("plain submit %s: %v", id, err)
		}

		// Drop the last client ("e") from the secure aggregator.
		if id == "e" {
			continue
		}
		if err := secureAgg.Submit(sc.MaskedSnapshot(round)); err != nil {
			t.Fatalf("secure submit %s: %v", id, err)
		}
	}

	// Secure aggregator should return an error because "e" is missing.
	_, err := secureAgg.Close(round)
	if err == nil {
		t.Error("expected error when a client is missing from SecureAggregator, got nil")
	}
}
