package causality_test

import (
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/causality"
	"github.com/immortal-engine/immortal/internal/event"
)

func TestGraphAddAndTraceChain(t *testing.T) {
	g := causality.New()

	// Simulate: bad deploy → slow DB → API timeout
	e1 := event.New(event.TypeError, event.SeverityWarning, "deployment started").
		WithSource("deploy-service")
	time.Sleep(10 * time.Millisecond)

	e2 := event.New(event.TypeError, event.SeverityError, "database query slow").
		WithSource("postgres")
	time.Sleep(10 * time.Millisecond)

	e3 := event.New(event.TypeError, event.SeverityCritical, "API timeout").
		WithSource("api-server")

	g.Add(e1)
	g.Add(e2)
	g.Add(e3)

	// Link them: e1 caused e2, e2 caused e3
	g.Link(e1.ID, e2.ID)
	g.Link(e2.ID, e3.ID)

	// Trace root cause from e3
	chain := g.RootCause(e3.ID)
	if len(chain) < 3 {
		t.Fatalf("expected chain of 3, got %d", len(chain))
	}
	if chain[0].ID != e1.ID {
		t.Errorf("expected root cause to be e1, got %s", chain[0].Source)
	}
}

func TestGraphAutoCorrelate(t *testing.T) {
	g := causality.NewWithWindow(2 * time.Second)

	// Events close in time from related sources
	e1 := event.New(event.TypeError, event.SeverityError, "disk full").
		WithSource("storage")
	e2 := event.New(event.TypeError, event.SeverityCritical, "write failed").
		WithSource("database")

	g.Add(e1)
	g.Add(e2)

	// Auto-correlate by time proximity
	g.AutoCorrelate()

	chain := g.RootCause(e2.ID)
	if len(chain) < 2 {
		t.Errorf("expected auto-correlated chain of at least 2, got %d", len(chain))
	}
}

func TestGraphImpactAnalysis(t *testing.T) {
	g := causality.New()

	root := event.New(event.TypeError, event.SeverityError, "root failure").WithSource("core")
	child1 := event.New(event.TypeError, event.SeverityError, "child 1").WithSource("svc-a")
	child2 := event.New(event.TypeError, event.SeverityError, "child 2").WithSource("svc-b")

	g.Add(root)
	g.Add(child1)
	g.Add(child2)
	g.Link(root.ID, child1.ID)
	g.Link(root.ID, child2.ID)

	impact := g.Impact(root.ID)
	if len(impact) != 2 {
		t.Errorf("expected 2 impacted events, got %d", len(impact))
	}
}
