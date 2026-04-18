package twin

import (
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
)

func TestObserveEvent_AutoRegistersUnknownService(t *testing.T) {
	tw := New(Config{})

	ev := event.New(event.TypeError, event.SeverityError, "something went wrong").
		WithSource("new-svc")

	tw.ObserveEvent(ev)

	s, ok := tw.Get("new-svc")
	if !ok {
		t.Fatal("expected new-svc to be registered in twin after ObserveEvent")
	}
	if s.Service != "new-svc" {
		t.Errorf("expected Service='new-svc', got %q", s.Service)
	}

	discovered := tw.AutoDiscover()
	found := false
	for _, name := range discovered {
		if name == "new-svc" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("AutoDiscover did not include 'new-svc'; got %v", discovered)
	}
}

func TestObserveEvent_CriticalError_MarksUnhealthy(t *testing.T) {
	tw := New(Config{})
	// Pre-register as healthy.
	tw.Observe(State{Service: "worker", Healthy: true, Replicas: 2})

	ev := event.New(event.TypeError, event.SeverityCritical, "OOM killed").
		WithSource("worker")
	tw.ObserveEvent(ev)

	s, ok := tw.Get("worker")
	if !ok {
		t.Fatal("expected worker state to exist")
	}
	if s.Healthy {
		t.Error("expected Healthy=false after critical error event")
	}
}

func TestObserveEvent_HealthEvent_UpdatesLastHealthCheck(t *testing.T) {
	tw := New(Config{})

	before := time.Now()
	ev := event.New(event.TypeHealth, event.SeverityInfo, "health check OK").
		WithSource("api")
	tw.ObserveEvent(ev)
	after := time.Now()

	s, ok := tw.Get("api")
	if !ok {
		t.Fatal("expected api state after health event")
	}
	if s.LastHealthCheck.Before(before) || s.LastHealthCheck.After(after) {
		t.Errorf("LastHealthCheck %v not in [%v, %v]", s.LastHealthCheck, before, after)
	}
	if !s.Healthy {
		t.Error("expected Healthy=true for info-severity health event")
	}

	// Warning severity should mark unhealthy.
	ev2 := event.New(event.TypeHealth, event.SeverityWarning, "health check degraded").
		WithSource("api")
	tw.ObserveEvent(ev2)

	s2, _ := tw.Get("api")
	if s2.Healthy {
		t.Error("expected Healthy=false for warning-severity health event")
	}
}

func TestObserveEvent_DepsFromMeta(t *testing.T) {
	tw := New(Config{})

	ev := event.New(event.TypeMetric, event.SeverityInfo, "metrics").
		WithSource("frontend").
		WithMeta("depends_on", []string{"api", "cache"})

	tw.ObserveEvent(ev)

	s, ok := tw.Get("frontend")
	if !ok {
		t.Fatal("expected frontend state")
	}
	if len(s.Dependencies) != 2 {
		t.Fatalf("expected 2 dependencies, got %v", s.Dependencies)
	}
	depMap := map[string]bool{}
	for _, d := range s.Dependencies {
		depMap[d] = true
	}
	if !depMap["api"] || !depMap["cache"] {
		t.Errorf("expected dependencies [api cache], got %v", s.Dependencies)
	}
}

func TestObserveEvent_MetricUpdatesFields(t *testing.T) {
	tw := New(Config{})

	ev := event.New(event.TypeMetric, event.SeverityInfo, "metrics").
		WithSource("db").
		WithMeta("cpu", 75.5).
		WithMeta("memory", 60.0).
		WithMeta("latency", 120.0).
		WithMeta("error_rate", 0.02).
		WithMeta("replicas", 3)

	tw.ObserveEvent(ev)

	s, ok := tw.Get("db")
	if !ok {
		t.Fatal("expected db state after metric event")
	}
	if s.CPU != 75.5 {
		t.Errorf("expected CPU=75.5, got %.2f", s.CPU)
	}
	if s.Memory != 60.0 {
		t.Errorf("expected Memory=60.0, got %.2f", s.Memory)
	}
	if s.Latency != 120.0 {
		t.Errorf("expected Latency=120.0, got %.2f", s.Latency)
	}
	if s.ErrorRate != 0.02 {
		t.Errorf("expected ErrorRate=0.02, got %.4f", s.ErrorRate)
	}
	if s.Replicas != 3 {
		t.Errorf("expected Replicas=3, got %d", s.Replicas)
	}
}
