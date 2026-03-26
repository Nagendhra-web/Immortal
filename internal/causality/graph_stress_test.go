package causality_test

import (
	"fmt"
	"testing"

	"github.com/immortal-engine/immortal/internal/causality"
	"github.com/immortal-engine/immortal/internal/event"
)

func TestGraphLongChain(t *testing.T) {
	g := causality.New()

	events := make([]*event.Event, 50)
	for i := 0; i < 50; i++ {
		events[i] = event.New(event.TypeError, event.SeverityError, fmt.Sprintf("event-%d", i))
		g.Add(events[i])
	}

	for i := 0; i < 49; i++ {
		g.Link(events[i].ID, events[i+1].ID)
	}

	chain := g.RootCause(events[49].ID)
	if len(chain) != 50 {
		t.Errorf("expected chain of 50, got %d", len(chain))
	}
	if chain[0].ID != events[0].ID {
		t.Error("root cause should be first event")
	}
}

func TestGraphCyclePrevention(t *testing.T) {
	g := causality.New()

	e1 := event.New(event.TypeError, event.SeverityError, "e1")
	e2 := event.New(event.TypeError, event.SeverityError, "e2")

	g.Add(e1)
	g.Add(e2)
	g.Link(e1.ID, e2.ID)
	g.Link(e2.ID, e1.ID) // Cycle!

	// Should not hang/infinite loop
	chain := g.RootCause(e1.ID)
	if len(chain) == 0 {
		t.Error("should return at least 1 event even with cycle")
	}
}

func TestGraphNoLinks(t *testing.T) {
	g := causality.New()
	e := event.New(event.TypeError, event.SeverityError, "isolated")
	g.Add(e)

	chain := g.RootCause(e.ID)
	if len(chain) != 1 {
		t.Errorf("expected chain of 1 for isolated event, got %d", len(chain))
	}

	impact := g.Impact(e.ID)
	if len(impact) != 0 {
		t.Errorf("expected 0 impact for isolated event, got %d", len(impact))
	}
}

func TestGraphNonExistentEvent(t *testing.T) {
	g := causality.New()
	chain := g.RootCause("nonexistent")
	if len(chain) != 0 {
		t.Error("expected empty chain for nonexistent event")
	}
}

func TestGraphWideImpact(t *testing.T) {
	g := causality.New()
	root := event.New(event.TypeError, event.SeverityError, "root")
	g.Add(root)

	for i := 0; i < 20; i++ {
		child := event.New(event.TypeError, event.SeverityError, fmt.Sprintf("child-%d", i))
		g.Add(child)
		g.Link(root.ID, child.ID)
	}

	impact := g.Impact(root.ID)
	if len(impact) != 20 {
		t.Errorf("expected 20 impacted, got %d", len(impact))
	}
}
