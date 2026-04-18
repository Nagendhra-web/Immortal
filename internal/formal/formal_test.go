package formal

import (
	"testing"
)

func initialWorld() World {
	return World{
		"api": {Name: "api", Healthy: true, Replicas: 3},
		"db":  {Name: "db", Healthy: true, Replicas: 1},
	}
}

func TestCheck_LinearPlanSafe(t *testing.T) {
	plan := Plan{
		ID: "noop-restart",
		Steps: []Action{
			{Name: "restart-api", Fn: func(w World) World {
				s := w["api"]
				s.Healthy = true
				w["api"] = s
				return w
			}},
		},
	}
	r := Check(initialWorld(), plan, []Invariant{AtLeastNHealthy(2)})
	if !r.Safe {
		t.Errorf("expected safe, got violation: %+v", r.Violation)
	}
}

func TestCheck_PlanViolatesInvariant(t *testing.T) {
	plan := Plan{
		ID: "scale-to-zero",
		Steps: []Action{
			{Name: "scale-api-to-zero", Fn: func(w World) World {
				s := w["api"]
				s.Replicas = 0
				w["api"] = s
				return w
			}},
		},
	}
	r := Check(initialWorld(), plan, []Invariant{MinReplicas("api", 1)})
	if r.Safe {
		t.Errorf("expected violation, got safe")
	}
	if r.Violation == nil {
		t.Fatal("violation is nil")
	}
	if r.Violation.Invariant == "" {
		t.Errorf("violation invariant name should be set")
	}
}

func TestCheck_ServiceAlwaysHealthy_Trips(t *testing.T) {
	plan := Plan{
		ID: "kill-db",
		Steps: []Action{
			{Name: "stop-db", Fn: func(w World) World {
				s := w["db"]
				s.Healthy = false
				w["db"] = s
				return w
			}},
		},
	}
	r := Check(initialWorld(), plan, []Invariant{ServiceAlwaysHealthy("db")})
	if r.Safe {
		t.Errorf("expected violation when killing db, got safe")
	}
}

func TestWorld_HashStable(t *testing.T) {
	w := initialWorld()
	h1 := w.Hash()
	h2 := w.Clone().Hash()
	if h1 != h2 {
		t.Errorf("clone hash differs: %d vs %d", h1, h2)
	}
}

func TestWorld_HashChangesOnMutation(t *testing.T) {
	w := initialWorld()
	h1 := w.Hash()
	s := w["api"]
	s.Healthy = false
	w["api"] = s
	h2 := w.Hash()
	if h1 == h2 {
		t.Errorf("hash should differ after mutation")
	}
}

func TestInvariants_AtLeastNHealthy(t *testing.T) {
	w := initialWorld()
	if !AtLeastNHealthy(2).Fn(w) {
		t.Error("AtLeastNHealthy(2) should pass")
	}
	if AtLeastNHealthy(5).Fn(w) {
		t.Error("AtLeastNHealthy(5) should fail with only 2 services")
	}
}

func TestInvariants_NoMoreThanKUnhealthy(t *testing.T) {
	w := initialWorld()
	if !NoMoreThanKUnhealthy(0).Fn(w) {
		t.Error("0 unhealthy should pass NoMoreThanKUnhealthy(0)")
	}
	s := w["api"]
	s.Healthy = false
	w["api"] = s
	if NoMoreThanKUnhealthy(0).Fn(w) {
		t.Error("1 unhealthy should fail NoMoreThanKUnhealthy(0)")
	}
}

func TestInvariants_Conjunction(t *testing.T) {
	w := initialWorld()
	c := Conjunction("both", AtLeastNHealthy(2), MinReplicas("api", 1))
	if !c.Fn(w) {
		t.Error("conjunction of two true invariants should hold")
	}
}

func TestInvariants_Negation(t *testing.T) {
	w := initialWorld()
	if Negation(AtLeastNHealthy(2)).Fn(w) {
		t.Error("negation of true invariant should be false")
	}
}
