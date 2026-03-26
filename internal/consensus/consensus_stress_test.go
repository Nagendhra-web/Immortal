package consensus_test

import (
	"sync"
	"testing"

	"github.com/immortal-engine/immortal/internal/consensus"
	"github.com/immortal-engine/immortal/internal/event"
)

func TestConsensusConcurrentEvaluate(t *testing.T) {
	c := consensus.New(consensus.Config{MinAgreement: 2})
	c.AddVerifier("v1", func(e *event.Event) bool { return true })
	c.AddVerifier("v2", func(e *event.Event) bool { return true })

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := c.Evaluate(event.New(event.TypeError, event.SeverityError, "test"))
			if !result.Approved {
				t.Error("should be approved")
			}
		}()
	}
	wg.Wait()
}

func TestConsensusMinAgreementZero(t *testing.T) {
	c := consensus.New(consensus.Config{MinAgreement: 0})
	c.AddVerifier("v1", func(e *event.Event) bool { return false })

	result := c.Evaluate(event.New(event.TypeError, event.SeverityError, "test"))
	// MinAgreement defaults to 1, so 0 votes should not approve
	if result.Approved {
		t.Error("0 votes should not approve with min 1")
	}
}

func TestConsensusVoterTracking(t *testing.T) {
	c := consensus.New(consensus.Config{MinAgreement: 1})
	c.AddVerifier("alice", func(e *event.Event) bool { return true })
	c.AddVerifier("bob", func(e *event.Event) bool { return false })
	c.AddVerifier("charlie", func(e *event.Event) bool { return true })

	result := c.Evaluate(event.New(event.TypeError, event.SeverityError, "test"))

	if len(result.Voters) != 2 {
		t.Errorf("expected 2 voters, got %d", len(result.Voters))
	}
	if len(result.Dissenters) != 1 {
		t.Errorf("expected 1 dissenter, got %d", len(result.Dissenters))
	}
}

func TestConsensusVerifierCount(t *testing.T) {
	c := consensus.New(consensus.Config{MinAgreement: 1})
	if c.VerifierCount() != 0 {
		t.Error("expected 0 verifiers initially")
	}
	c.AddVerifier("v1", func(e *event.Event) bool { return true })
	if c.VerifierCount() != 1 {
		t.Error("expected 1 verifier")
	}
}
