package federated

import (
	"math"
	"math/rand/v2"
	"strings"
	"testing"
)

// newDropoutRng creates a deterministic RNG for tests.
func newDropoutRng(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed^0xcafebabe))
}

// setupProtocol runs the KeyExchange round for all clients, distributes shares,
// and returns clients + aggregator ready for Submit/Reveal rounds.
func setupProtocol(ids []string, threshold int, seeds []uint64) ([]*SecureDropoutClient, *SecureDropoutAggregator, error) {
	clients := make([]*SecureDropoutClient, len(ids))
	for i, id := range ids {
		peers := make([]string, 0, len(ids)-1)
		for _, other := range ids {
			if other != id {
				peers = append(peers, other)
			}
		}
		rng := newDropoutRng(seeds[i])
		c, err := NewSecureDropoutClient(id, peers, threshold, rng)
		if err != nil {
			return nil, nil, err
		}
		clients[i] = c
	}

	// KeyExchange: collect shares from each client, distribute to peers.
	allShares := make(map[string]map[string][]PeerShare) // sender -> recipient -> shares
	for _, c := range clients {
		allShares[c.id] = c.SetupShares()
	}

	// Deliver shares: for each (sender, recipient) pair.
	for _, recipient := range clients {
		for _, sender := range clients {
			if sender.id == recipient.id {
				continue
			}
			batch, ok := allShares[sender.id][recipient.id]
			if !ok {
				continue
			}
			recipient.AcceptShares(batch)
		}
	}

	agg := NewSecureDropoutAggregator(ids, threshold)
	return clients, agg, nil
}

// submitAndReveal runs the Submit+Reveal rounds for the given set of active
// clients (dropping the ones in dropped). Returns the GlobalModel.
func submitAndReveal(
	t *testing.T,
	allClients []*SecureDropoutClient,
	agg *SecureDropoutAggregator,
	dropped map[string]bool,
	values map[string]float64,
	metric string,
	round int,
) (GlobalModel, error) {
	t.Helper()

	var online []string
	var droppedList []string
	for _, c := range allClients {
		if dropped[c.id] {
			droppedList = append(droppedList, c.id)
		} else {
			online = append(online, c.id)
		}
	}

	// Submit masked updates.
	for _, c := range allClients {
		if dropped[c.id] {
			continue
		}
		v := values[c.id]
		u := Update{
			ClientID: c.id,
			Round:    round,
			Stats: map[string]MetricStats{
				metric: {Metric: metric, Count: 1, Mean: v},
			},
		}
		masked := c.MaskMetricUpdate(u, round)
		if err := agg.SubmitMasked(masked); err != nil {
			t.Fatalf("SubmitMasked %s: %v", c.id, err)
		}
	}

	// Reveal shares from surviving clients.
	for _, c := range allClients {
		if dropped[c.id] {
			continue
		}
		revealed := c.RevealShares(online, droppedList)
		if err := agg.AcceptReveal(c.id, revealed); err != nil {
			t.Fatalf("AcceptReveal %s: %v", c.id, err)
		}
	}

	return agg.Close(round)
}

// TestBonawitz_AllClientsOnline_MatchesPlainSum verifies that with all 5
// clients online the unmasked aggregate equals the plain sum of values.
func TestBonawitz_AllClientsOnline_MatchesPlainSum(t *testing.T) {
	ids := []string{"a", "b", "c", "d", "e"}
	threshold := 3
	seeds := []uint64{1, 2, 3, 4, 5}
	values := map[string]float64{"a": 10, "b": 20, "c": 30, "d": 40, "e": 50}
	const metric = "loss"
	const round = 1

	clients, agg, err := setupProtocol(ids, threshold, seeds)
	if err != nil {
		t.Fatalf("setupProtocol: %v", err)
	}

	gm, err := submitAndReveal(t, clients, agg, nil, values, metric, round)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	// With Count=1 per client, weighted mean = sum/N.
	wantMean := (10.0 + 20.0 + 30.0 + 40.0 + 50.0) / 5.0
	gotMean := gm.Metrics[metric].Mean
	if math.Abs(gotMean-wantMean) > 1e-9 {
		t.Errorf("mean=%v want=%v diff=%v", gotMean, wantMean, math.Abs(gotMean-wantMean))
	}
	if gm.Metrics[metric].Count != 5 {
		t.Errorf("count=%d want=5", gm.Metrics[metric].Count)
	}
}

// TestBonawitz_OneClientDrops_Recoverable verifies that 1 drop with threshold=3
// still produces the correct aggregate over the 4 remaining clients.
func TestBonawitz_OneClientDrops_Recoverable(t *testing.T) {
	ids := []string{"a", "b", "c", "d", "e"}
	threshold := 3
	seeds := []uint64{10, 20, 30, 40, 50}
	values := map[string]float64{"a": 1, "b": 2, "c": 3, "d": 4, "e": 5}
	const metric = "acc"
	const round = 2

	clients, agg, err := setupProtocol(ids, threshold, seeds)
	if err != nil {
		t.Fatalf("setupProtocol: %v", err)
	}

	dropped := map[string]bool{"e": true}
	gm, err := submitAndReveal(t, clients, agg, dropped, values, metric, round)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Only a,b,c,d contributed; sum=10, mean=10/4=2.5.
	wantMean := (1.0 + 2.0 + 3.0 + 4.0) / 4.0
	gotMean := gm.Metrics[metric].Mean
	if math.Abs(gotMean-wantMean) > 1e-9 {
		t.Errorf("mean=%v want=%v diff=%v", gotMean, wantMean, math.Abs(gotMean-wantMean))
	}
	if gm.Metrics[metric].Count != 4 {
		t.Errorf("count=%d want=4", gm.Metrics[metric].Count)
	}
}

// TestBonawitz_TwoClientsDrop_Recoverable verifies threshold=3 tolerates 2 drops.
func TestBonawitz_TwoClientsDrop_Recoverable(t *testing.T) {
	ids := []string{"a", "b", "c", "d", "e"}
	threshold := 3
	seeds := []uint64{100, 200, 300, 400, 500}
	values := map[string]float64{"a": 5, "b": 10, "c": 15, "d": 20, "e": 25}
	const metric = "grad"
	const round = 3

	clients, agg, err := setupProtocol(ids, threshold, seeds)
	if err != nil {
		t.Fatalf("setupProtocol: %v", err)
	}

	dropped := map[string]bool{"d": true, "e": true}
	gm, err := submitAndReveal(t, clients, agg, dropped, values, metric, round)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	wantMean := (5.0 + 10.0 + 15.0) / 3.0
	gotMean := gm.Metrics[metric].Mean
	if math.Abs(gotMean-wantMean) > 1e-9 {
		t.Errorf("mean=%v want=%v diff=%v", gotMean, wantMean, math.Abs(gotMean-wantMean))
	}
	if gm.Metrics[metric].Count != 3 {
		t.Errorf("count=%d want=3", gm.Metrics[metric].Count)
	}
}

// TestBonawitz_TooManyDrop_Errors verifies that dropping 3 clients when
// threshold=3 makes Close return an error (N-threshold = 2, but 3 drop).
func TestBonawitz_TooManyDrop_Errors(t *testing.T) {
	ids := []string{"a", "b", "c", "d", "e"}
	threshold := 3
	seeds := []uint64{11, 22, 33, 44, 55}
	values := map[string]float64{"a": 1, "b": 2, "c": 3, "d": 4, "e": 5}
	const metric = "loss"
	const round = 1

	clients, agg, err := setupProtocol(ids, threshold, seeds)
	if err != nil {
		t.Fatalf("setupProtocol: %v", err)
	}

	// Drop 3 clients — only 2 remain, below threshold=3.
	dropped := map[string]bool{"c": true, "d": true, "e": true}
	_, err = submitAndReveal(t, clients, agg, dropped, values, metric, round)
	if err == nil {
		t.Error("expected error when too many clients drop, got nil")
	}
	if !strings.Contains(err.Error(), "cannot recover") && !strings.Contains(err.Error(), "need at least") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestBonawitz_MetricUpdate_PreservesCounts verifies that Count and M2 are
// unchanged after masking, and the mean is correctly unmasked on Close.
func TestBonawitz_MetricUpdate_PreservesCounts(t *testing.T) {
	ids := []string{"x", "y", "z"}
	threshold := 2
	seeds := []uint64{7, 8, 9}
	const metric = "rtt"
	const round = 5

	clients, agg, err := setupProtocol(ids, threshold, seeds)
	if err != nil {
		t.Fatalf("setupProtocol: %v", err)
	}

	rawUpdates := map[string]Update{
		"x": {ClientID: "x", Round: round, Stats: map[string]MetricStats{metric: {Metric: metric, Count: 100, Mean: 3.14, M2: 5.0}}},
		"y": {ClientID: "y", Round: round, Stats: map[string]MetricStats{metric: {Metric: metric, Count: 200, Mean: 2.72, M2: 8.0}}},
		"z": {ClientID: "z", Round: round, Stats: map[string]MetricStats{metric: {Metric: metric, Count: 150, Mean: 1.41, M2: 3.5}}},
	}

	var online []string
	for _, id := range ids {
		online = append(online, id)
	}

	for _, c := range clients {
		u := rawUpdates[c.id]
		masked := c.MaskMetricUpdate(u, round)
		// Count and M2 must be unchanged.
		orig := rawUpdates[c.id].Stats[metric]
		got := masked.Stats[metric]
		if got.Count != orig.Count {
			t.Errorf("client %s: Count changed %d -> %d", c.id, orig.Count, got.Count)
		}
		if got.M2 != orig.M2 {
			t.Errorf("client %s: M2 changed %v -> %v", c.id, orig.M2, got.M2)
		}
		if err := agg.SubmitMasked(masked); err != nil {
			t.Fatalf("SubmitMasked %s: %v", c.id, err)
		}
	}
	for _, c := range clients {
		revealed := c.RevealShares(online, nil)
		if err := agg.AcceptReveal(c.id, revealed); err != nil {
			t.Fatalf("AcceptReveal %s: %v", c.id, err)
		}
	}

	gm, err := agg.Close(round)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Expected weighted mean.
	totalCount := 100 + 200 + 150
	wantMean := (100*3.14 + 200*2.72 + 150*1.41) / float64(totalCount)
	gotMean := gm.Metrics[metric].Mean
	if math.Abs(gotMean-wantMean) > 1e-9 {
		t.Errorf("mean=%v want=%v diff=%v", gotMean, wantMean, math.Abs(gotMean-wantMean))
	}
	if gm.Metrics[metric].Count != totalCount {
		t.Errorf("count=%d want=%d", gm.Metrics[metric].Count, totalCount)
	}
}

// TestBonawitz_DroppedClientCannotPoisonResult verifies that a client which
// dropped (never submitted) does not contribute its value to the aggregate.
func TestBonawitz_DroppedClientCannotPoisonResult(t *testing.T) {
	ids := []string{"a", "b", "c", "d", "e"}
	threshold := 3
	seeds := []uint64{91, 92, 93, 94, 95}
	const metric = "err"
	const round = 4

	clients, agg, err := setupProtocol(ids, threshold, seeds)
	if err != nil {
		t.Fatalf("setupProtocol: %v", err)
	}

	// "e" drops with a huge value — its value must NOT appear in the result.
	values := map[string]float64{"a": 1, "b": 1, "c": 1, "d": 1, "e": 1e12}
	dropped := map[string]bool{"e": true}

	gm, err := submitAndReveal(t, clients, agg, dropped, values, metric, round)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Result should be mean of a,b,c,d = 1.0, not anywhere near 1e12.
	gotMean := gm.Metrics[metric].Mean
	if math.Abs(gotMean-1.0) > 1e-9 {
		t.Errorf("dropped client value leaked: mean=%v want=1.0", gotMean)
	}
	// Contributors must not include "e".
	for _, id := range gm.Contributors {
		if id == "e" {
			t.Errorf("dropped client e appears in contributors: %v", gm.Contributors)
		}
	}
}

// TestBonawitz_AggregatorSeesNoIndividualValue verifies that the masked value
// stored by the aggregator differs meaningfully from the raw value.
func TestBonawitz_AggregatorSeesNoIndividualValue(t *testing.T) {
	ids := []string{"p", "q", "r"}
	threshold := 2
	seeds := []uint64{42, 43, 44}
	const metric = "cpu"
	const round = 7
	const trueValue = 99.0

	clients, _, err := setupProtocol(ids, threshold, seeds)
	if err != nil {
		t.Fatalf("setupProtocol: %v", err)
	}

	u := Update{
		ClientID: "p",
		Round:    round,
		Stats:    map[string]MetricStats{metric: {Metric: metric, Count: 1, Mean: trueValue}},
	}
	masked := clients[0].MaskMetricUpdate(u, round)
	maskedMean := masked.Stats[metric].Mean

	// The masked value must differ from the raw value by more than float epsilon.
	// With a PRG mask in ~(-2, 2) range, this should be large.
	if math.Abs(maskedMean-trueValue) < 1e-6 {
		t.Errorf("masked value too close to raw value: masked=%v raw=%v", maskedMean, trueValue)
	}
}

// TestBonawitz_DeterministicWithFixedSeed verifies that the same RNG seeds
// produce identical masked outputs, confirming the PRG is deterministic.
func TestBonawitz_DeterministicWithFixedSeed(t *testing.T) {
	ids := []string{"u1", "u2", "u3"}
	threshold := 2
	seeds := []uint64{1000, 2000, 3000}
	const metric = "latency"
	const round = 1

	run := func() float64 {
		clients, agg, err := setupProtocol(ids, threshold, seeds)
		if err != nil {
			t.Fatalf("setupProtocol: %v", err)
		}
		values := map[string]float64{"u1": 5, "u2": 10, "u3": 15}
		var online []string
		for _, id := range ids {
			online = append(online, id)
		}
		for _, c := range clients {
			v := values[c.id]
			u := Update{
				ClientID: c.id,
				Round:    round,
				Stats:    map[string]MetricStats{metric: {Metric: metric, Count: 1, Mean: v}},
			}
			masked := c.MaskMetricUpdate(u, round)
			if err := agg.SubmitMasked(masked); err != nil {
				t.Fatalf("SubmitMasked: %v", err)
			}
		}
		for _, c := range clients {
			revealed := c.RevealShares(online, nil)
			if err := agg.AcceptReveal(c.id, revealed); err != nil {
				t.Fatalf("AcceptReveal: %v", err)
			}
		}
		gm, err := agg.Close(round)
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
		return gm.Metrics[metric].Mean
	}

	mean1 := run()
	mean2 := run()
	if mean1 != mean2 {
		t.Errorf("runs differ: %v vs %v", mean1, mean2)
	}
	// Also verify the result equals the plain mean.
	wantMean := (5.0 + 10.0 + 15.0) / 3.0
	if math.Abs(mean1-wantMean) > 1e-9 {
		t.Errorf("mean=%v want=%v", mean1, wantMean)
	}
}
