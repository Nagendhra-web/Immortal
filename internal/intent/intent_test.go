package intent

import (
	"testing"
	"time"
)

type fakeMetrics map[string]float64

func (f fakeMetrics) Value(service, metric string) (float64, bool) {
	key := service + "::" + metric
	v, ok := f[key]
	return v, ok
}

func TestLatencyUnder_MetVsViolated(t *testing.T) {
	m := fakeMetrics{
		"rest::latency_p99": 100, // well under 200: met and safe (50% headroom)
		"auth::latency_p99": 260, // over 200: violated
	}
	e := New(m)
	e.now = func() time.Time { return time.Unix(1_000_000, 0) }
	e.AddIntent(Intent{
		Name: "checkout-fast",
		Goals: []Goal{
			{Kind: LatencyUnder, Service: "rest", Target: 200, Priority: 8},
			{Kind: LatencyUnder, Service: "auth", Target: 200, Priority: 8},
		},
	})
	statuses := e.Evaluate()
	if len(statuses) != 2 {
		t.Fatalf("want 2 statuses, got %d", len(statuses))
	}
	for _, s := range statuses {
		if s.Goal.Service == "rest" && (!s.Met || s.AtRisk) {
			t.Errorf("rest should be met+safe, got met=%v atRisk=%v current=%v", s.Met, s.AtRisk, s.Current)
		}
		if s.Goal.Service == "auth" && s.Met {
			t.Errorf("auth should be violated, got met=true current=%v", s.Current)
		}
	}
}

func TestAtRiskBand_LatencyUnder(t *testing.T) {
	m := fakeMetrics{"rest::latency_p99": 170} // within 20% of 200 -> at risk
	e := New(m)
	e.AddIntent(Intent{
		Name: "near-limit",
		Goals: []Goal{{Kind: LatencyUnder, Service: "rest", Target: 200, Priority: 5}},
	})
	s := e.Evaluate()[0]
	if !s.Met {
		t.Fatalf("latency 170 under 200 should be met")
	}
	if !s.AtRisk {
		t.Fatalf("latency 170 within 20%% of 200 should be at risk; got slack=%v", s.Slack)
	}
}

func TestAvailabilityOver_AtRisk(t *testing.T) {
	m := fakeMetrics{"api::availability": 0.9995}
	e := New(m)
	e.AddIntent(Intent{
		Name: "high-avail",
		Goals: []Goal{{Kind: AvailabilityOver, Service: "api", Target: 0.999, Priority: 10}},
	})
	s := e.Evaluate()[0]
	if !s.Met {
		t.Fatalf("availability 0.9995 > 0.999 should be met")
	}
	if !s.AtRisk {
		t.Errorf("near-threshold availability should be at risk")
	}
}

func TestJobsNoDrop_Violation(t *testing.T) {
	m := fakeMetrics{"queue::jobs_dropped": 3}
	e := New(m)
	e.AddIntent(Intent{
		Name:  "durable",
		Goals: []Goal{{Kind: JobsNoDrop, Service: "queue", Target: 0, Priority: 10}},
	})
	s := e.Evaluate()[0]
	if s.Met {
		t.Fatalf("3 dropped jobs should violate target 0")
	}
}

func TestSuggest_RanksByPriority(t *testing.T) {
	m := fakeMetrics{
		"queue::jobs_dropped": 5,
		"rest::latency_p99":   260,
	}
	e := New(m)
	e.AddIntent(Intent{
		Name: "mixed",
		Goals: []Goal{
			{Kind: LatencyUnder, Service: "rest", Target: 200, Priority: 3},
			{Kind: JobsNoDrop, Service: "queue", Target: 0, Priority: 10},
		},
	})
	sugs := e.Suggest()
	if len(sugs) == 0 {
		t.Fatal("expected suggestions for violated goals")
	}
	// First suggestion must be for the highest-priority goal (jobs no-drop).
	if sugs[0].Goal.Kind != JobsNoDrop {
		t.Errorf("want JobsNoDrop first by priority; got %v", sugs[0].Action)
	}
}

func TestSuggest_SkipsMetGoals(t *testing.T) {
	m := fakeMetrics{"rest::latency_p99": 50}
	e := New(m)
	e.AddIntent(Intent{
		Name:  "fast",
		Goals: []Goal{{Kind: LatencyUnder, Service: "rest", Target: 200, Priority: 5}},
	})
	if sugs := e.Suggest(); len(sugs) != 0 {
		t.Errorf("met goal should not produce suggestions; got %d", len(sugs))
	}
}

func TestDescribe(t *testing.T) {
	e := New(fakeMetrics{"rest::latency_p99": 180})
	e.AddIntent(Intent{
		Name:  "one",
		Goals: []Goal{{Kind: LatencyUnder, Service: "rest", Target: 200}},
	})
	got := e.Describe()
	if got == "" {
		t.Fatal("describe should not be empty")
	}
}

func TestAddRemoveIntent(t *testing.T) {
	e := New(fakeMetrics{})
	e.AddIntent(Intent{Name: "a", Goals: []Goal{{Kind: LatencyUnder, Target: 100}}})
	e.AddIntent(Intent{Name: "b", Goals: []Goal{{Kind: LatencyUnder, Target: 200}}})
	if got := len(e.List()); got != 2 {
		t.Errorf("want 2, got %d", got)
	}
	e.RemoveIntent("a")
	if got := len(e.List()); got != 1 {
		t.Errorf("after remove want 1, got %d", got)
	}
}
