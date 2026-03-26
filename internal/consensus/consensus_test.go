package consensus_test

import (
	"testing"

	"github.com/immortal-engine/immortal/internal/consensus"
	"github.com/immortal-engine/immortal/internal/event"
)

func TestConsensusAllAgree(t *testing.T) {
	c := consensus.New(consensus.Config{MinAgreement: 3})

	c.AddVerifier("rule-match", func(e *event.Event) bool { return true })
	c.AddVerifier("anomaly-detect", func(e *event.Event) bool { return true })
	c.AddVerifier("trend-analysis", func(e *event.Event) bool { return true })

	e := event.New(event.TypeError, event.SeverityCritical, "crash")
	result := c.Evaluate(e)

	if !result.Approved {
		t.Error("expected approved when all verifiers agree")
	}
	if result.Votes != 3 {
		t.Errorf("expected 3 votes, got %d", result.Votes)
	}
}

func TestConsensusDisagree(t *testing.T) {
	c := consensus.New(consensus.Config{MinAgreement: 3})

	c.AddVerifier("rule-match", func(e *event.Event) bool { return true })
	c.AddVerifier("anomaly-detect", func(e *event.Event) bool { return false })
	c.AddVerifier("trend-analysis", func(e *event.Event) bool { return true })

	e := event.New(event.TypeError, event.SeverityCritical, "crash")
	result := c.Evaluate(e)

	if result.Approved {
		t.Error("expected NOT approved when only 2/3 agree and min is 3")
	}
	if result.Votes != 2 {
		t.Errorf("expected 2 votes, got %d", result.Votes)
	}
}

func TestConsensusMajority(t *testing.T) {
	c := consensus.New(consensus.Config{MinAgreement: 2})

	c.AddVerifier("v1", func(e *event.Event) bool { return true })
	c.AddVerifier("v2", func(e *event.Event) bool { return true })
	c.AddVerifier("v3", func(e *event.Event) bool { return false })

	e := event.New(event.TypeError, event.SeverityError, "error")
	result := c.Evaluate(e)

	if !result.Approved {
		t.Error("expected approved with 2/3 when min is 2")
	}
}

func TestConsensusNoVerifiers(t *testing.T) {
	c := consensus.New(consensus.Config{MinAgreement: 1})

	e := event.New(event.TypeError, event.SeverityError, "error")
	result := c.Evaluate(e)

	if result.Approved {
		t.Error("expected NOT approved with no verifiers")
	}
}
