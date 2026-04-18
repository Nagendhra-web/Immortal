package twin

import (
	"testing"
)

// TestRealWorld_CascadeFailure_TwinRejectsBadPlan_AcceptsGoodPlan simulates a
// realistic cascade failure: db is down, causing api to degrade. The twin must
// reject a plan that does nothing meaningful and accept a plan that fixes db first.
func TestRealWorld_CascadeFailure_TwinRejectsBadPlan_AcceptsGoodPlan(t *testing.T) {
	tw := New(Config{})

	// Setup: api depends on db and cache; db is unhealthy.
	tw.Observe(State{
		Service:      "api",
		CPU:          70,
		Memory:       65,
		Replicas:     3,
		Healthy:      true,
		Latency:      500,
		ErrorRate:    0.3,
		Dependencies: []string{"db", "cache"},
	})
	tw.Observe(State{
		Service:   "db",
		CPU:       95,
		Memory:    90,
		Replicas:  1,
		Healthy:   false,
		Latency:   800,
		ErrorRate: 0.8,
	})
	tw.Observe(State{
		Service:   "cache",
		CPU:       30,
		Memory:    40,
		Replicas:  2,
		Healthy:   true,
		Latency:   5,
		ErrorRate: 0.0,
	})

	startScore := DefaultScore(tw.States())
	t.Logf("StartScore: %.4f", startScore)

	// BAD Plan: scale api down to 1 replica — CPU spikes, Latency doubles,
	// capacity drops. Doesn't address the db root cause. Twin must reject.
	badPlan := Plan{
		ID: "bad-plan",
		Actions: []Action{
			{Type: "noop", Target: "api"},
			{Type: "scale", Target: "api", Params: map[string]string{"replicas": "1"}},
			{Type: "noop", Target: "db"},
		},
	}
	badSim := tw.Simulate(badPlan)
	t.Logf("Bad plan EndScore: %.4f, Improvement: %.4f, Accepted: %v, Rejected: %v",
		badSim.EndScore, badSim.Improvement, badSim.Accepted, badSim.Rejected)

	if !badSim.Rejected {
		t.Errorf("bad plan should be Rejected, but was Accepted (EndScore=%.4f, StartScore=%.4f, reason=%s)",
			badSim.EndScore, badSim.StartScore, badSim.Reason)
	}

	// GOOD Plan: failover:db, restart:api — fixes root cause then restores api.
	goodPlan := Plan{
		ID: "good-plan",
		Actions: []Action{
			{Type: "failover", Target: "db"},
			{Type: "restart", Target: "api"},
		},
	}
	goodSim := tw.Simulate(goodPlan)
	t.Logf("Good plan EndScore: %.4f, Improvement: %.4f, Accepted: %v, Rejected: %v",
		goodSim.EndScore, goodSim.Improvement, goodSim.Accepted, goodSim.Rejected)

	if !goodSim.Accepted {
		t.Errorf("good plan should be Accepted, but was Rejected (EndScore=%.4f, StartScore=%.4f, reason=%s)",
			goodSim.EndScore, goodSim.StartScore, goodSim.Reason)
	}
	if goodSim.EndScore <= goodSim.StartScore {
		t.Errorf("good plan should improve score: EndScore=%.4f StartScore=%.4f",
			goodSim.EndScore, goodSim.StartScore)
	}

	// After failover:db, api's dependency on db should reduce api's Latency and ErrorRate.
	// Check that api's Latency improved in the good simulation.
	endAPI := goodSim.End["api"]
	startAPI := tw.States()["api"]
	if endAPI.Latency >= startAPI.Latency {
		t.Errorf("expected api Latency to improve after db failover: before=%.2f after=%.2f",
			startAPI.Latency, endAPI.Latency)
	}

	// Twin state must remain unchanged after both simulations.
	finalStates := tw.States()
	if finalStates["db"].Healthy != false {
		t.Error("twin state was mutated by Simulate — db should still be unhealthy")
	}
	if finalStates["api"].ErrorRate != 0.3 {
		t.Errorf("twin state was mutated by Simulate — api.ErrorRate should be 0.3, got %.2f",
			finalStates["api"].ErrorRate)
	}
}
