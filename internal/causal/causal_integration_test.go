package causal_test

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/causal"
)

// TestRealWorld_CascadingFailure_CausalRootCauseBeatsCorrelation simulates:
//
//	user_load ──► disk_full ──► db_latency ──► api_error_rate
//	         └─► marketing_traffic                           (red herring)
//
// marketing_traffic shares an observed confounder (user_load) with the real
// cause chain, so Pearson correlation makes it LOOK like a cause of errors.
// PC-algorithm causal discovery, conditioning on user_load, removes the
// spurious edge. RootCause ranks disk_full / db_latency highly and excludes
// marketing_traffic from the root-cause list.
func TestRealWorld_CascadingFailure_CausalRootCauseBeatsCorrelation(t *testing.T) {
	rng := rand.New(rand.NewPCG(99, 7))
	n := 1000

	names := []string{"user_load", "disk_full", "db_latency", "api_error_rate", "marketing_traffic"}
	ds := causal.NewDataset(names)

	for i := 0; i < n; i++ {
		userLoad := rng.NormFloat64()
		diskFull := 0.7*userLoad + 0.3*rng.NormFloat64()
		dbLatency := 0.85*diskFull + 0.15*rng.NormFloat64()
		apiErrorRate := 0.85*dbLatency + 0.15*rng.NormFloat64()
		marketingTraffic := 0.9*userLoad + 0.2*rng.NormFloat64() // correlated via user_load only

		_ = ds.Add(map[string]float64{
			"user_load":         userLoad,
			"disk_full":         diskFull,
			"db_latency":        dbLatency,
			"api_error_rate":    apiErrorRate,
			"marketing_traffic": marketingTraffic,
		})
	}

	// 1. Pearson correlation of marketing_traffic with api_error_rate should
	// be substantial — enough to fool a naive observer.
	pearsonR := pearsonManual(ds.Data["marketing_traffic"], ds.Data["api_error_rate"])
	if math.Abs(pearsonR) < 0.4 {
		t.Errorf("expected |Pearson r| > 0.4 for red-herring, got %.4f (check data generation)", pearsonR)
	}
	t.Logf("Pearson r(marketing_traffic, api_error_rate) = %.4f  [red-herring looks correlated]", pearsonR)

	// 2. Discover causal graph.
	cfg := causal.DiscoverConfig{Alpha: 0.05, MaxCondSetSize: 2}
	g, err := causal.Discover(ds, cfg)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	// marketing_traffic should have NO edge to api_error_rate after the PC
	// algorithm conditions on user_load.
	if g.HasEdge("marketing_traffic", "api_error_rate") || g.HasEdge("api_error_rate", "marketing_traffic") {
		t.Error("causal graph should NOT have marketing_traffic ↔ api_error_rate edge after conditioning on user_load")
	} else {
		t.Log("PASS: no marketing_traffic ↔ api_error_rate edge in causal graph")
	}

	// 3. RootCause should rank disk_full / db_latency / user_load highest;
	//    marketing_traffic should not appear (it isn't an ancestor) or have ACE ≈ 0.
	rc, err := causal.RootCause(ds, g, "api_error_rate")
	if err != nil {
		t.Fatalf("RootCause: %v", err)
	}

	t.Logf("RootCause ranking for api_error_rate:")
	for _, r := range rc.Ranked {
		t.Logf("  %-22s  ACE = %+.4f", r.Variable, r.ACE)
	}

	if len(rc.Ranked) == 0 {
		t.Fatal("expected at least one ranked ancestor")
	}

	// disk_full must appear with a non-trivial ACE.
	foundDisk := false
	for _, r := range rc.Ranked {
		if r.Variable == "disk_full" && math.Abs(r.ACE) > 0.1 {
			foundDisk = true
			break
		}
	}
	if !foundDisk {
		t.Errorf("expected disk_full with significant ACE in root causes; ranking: %+v", rc.Ranked)
	}

	// The #1 root cause must be a true causal ancestor — NOT marketing_traffic.
	// Pure correlation would put marketing_traffic near the top (Pearson r ≈ 0.84);
	// PC + adjustment-set regression correctly demotes it below the real chain.
	trueCauses := map[string]bool{"user_load": true, "disk_full": true, "db_latency": true}
	if !trueCauses[rc.Ranked[0].Variable] {
		t.Errorf("#1 root cause must be a true cause, got %q (ranking: %+v)", rc.Ranked[0].Variable, rc.Ranked)
	}

	// marketing_traffic must be ranked below disk_full (the true root of the chain).
	markRank, diskRank := -1, -1
	for i, r := range rc.Ranked {
		if r.Variable == "marketing_traffic" {
			markRank = i
		}
		if r.Variable == "disk_full" {
			diskRank = i
		}
	}
	if markRank != -1 && diskRank != -1 && markRank < diskRank {
		t.Errorf("marketing_traffic (rank %d) should be below disk_full (rank %d) — causal analysis failed to demote red herring",
			markRank, diskRank)
	}
	t.Log("PASS: causal root cause ranks true causes above the marketing_traffic red herring")
}

// pearsonManual computes Pearson r without relying on package internals.
func pearsonManual(x, y []float64) float64 {
	n := len(x)
	if n == 0 {
		return 0
	}
	var mx, my float64
	for i := range x {
		mx += x[i]
		my += y[i]
	}
	mx /= float64(n)
	my /= float64(n)
	var cov, vx, vy float64
	for i := range x {
		dx := x[i] - mx
		dy := y[i] - my
		cov += dx * dy
		vx += dx * dx
		vy += dy * dy
	}
	if vx < 1e-15 || vy < 1e-15 {
		return 0
	}
	return cov / math.Sqrt(vx*vy)
}
