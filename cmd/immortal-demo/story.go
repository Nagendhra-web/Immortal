package main

import (
	"time"

	"github.com/Nagendhra-web/Immortal/internal/evolve"
	"github.com/Nagendhra-web/Immortal/internal/narrator"
)

// TimelineEvent is one entry in the dramatic timeline shown in the report.
type TimelineEvent struct {
	At       time.Time
	Kind     string // "observe" | "detect" | "contract" | "heal" | "verdict" | "advisor"
	Source   string // service or actor
	Message  string
	Detail   string // optional sub-line (rationale, evidence, etc.)
	Emphasis bool   // true = highlight as a decision taken by Immortal
}

// CounterfactualMetric pairs "without Immortal" and "with Immortal" values
// for the same metric. Rendered as a side-by-side bar comparison.
type CounterfactualMetric struct {
	Label     string  // "Error rate", "Checkout p99", "Failures / min"
	Unit      string  // "%", "ms", "/min"
	Without   float64 // what would have happened
	With      float64 // what actually happened
	HigherBad bool    // true = higher is worse; false = higher is better
}

// CausalNode is a node in the cascade DAG.
type CausalNode struct {
	ID       string
	Label    string
	Role     string // "root_cause" | "relay" | "victim" | "action"
	Delta    string // optional "p99 310ms" / "retry 0.42" annotation
}

// CausalEdge connects two nodes in the DAG.
type CausalEdge struct {
	From    string
	To      string
	Kind    string // "caused" | "amplified" | "healed_by"
	Label   string // optional "retry storm" / "pool exhaustion"
}

// Story is the complete record of a demo run, collected as the scenario
// executes, then rendered to terminal (brief) and HTML (rich).
type Story struct {
	Scenario      string
	StartedAt     time.Time
	EndedAt       time.Time
	Headline      string // the ONE dramatic line
	Tagline       string // shorter subtitle
	Contract      string // human-language contract text
	Timeline      []TimelineEvent
	Counterfact   []CounterfactualMetric
	Causal        struct {
		Nodes []CausalNode
		Edges []CausalEdge
	}
	Verdict       narrator.Verdict
	TopSuggestion *evolve.Suggestion // optional advisor recommendation with twin prediction
}

// Add pushes an event onto the timeline with the current wall clock.
func (s *Story) Add(kind, source, msg string) {
	s.Timeline = append(s.Timeline, TimelineEvent{
		At: time.Now(), Kind: kind, Source: source, Message: msg,
	})
}

// AddWithDetail is Add plus a sub-line used for rationales / evidence.
func (s *Story) AddWithDetail(kind, source, msg, detail string, emphasis bool) {
	s.Timeline = append(s.Timeline, TimelineEvent{
		At: time.Now(), Kind: kind, Source: source, Message: msg,
		Detail: detail, Emphasis: emphasis,
	})
}

// Duration returns how long the scenario ran.
func (s *Story) Duration() time.Duration {
	if s.EndedAt.IsZero() {
		return time.Since(s.StartedAt)
	}
	return s.EndedAt.Sub(s.StartedAt)
}
