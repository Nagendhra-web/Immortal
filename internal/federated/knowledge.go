// Package federated provides a privacy-preserving federated incident knowledge graph.
//
// # Threat Model
//
// Patterns contain an irreversible hash of an incident message plus the action taken.
// Given a fingerprint, the original message is unrecoverable without a dictionary of
// plausible messages + brute-force comparison — not zero-knowledge, but a significant
// privacy gradient. Specifically:
//
//   - What IS shared: 64-bit SimHash of the incident text, action string, outcome enum,
//     healing duration, contribution timestamp, and an optional node ID.
//   - What is NOT shared: raw log lines, service names, customer data, host names,
//     stack traces, or any text that could re-identify the incident.
//   - Attack surface: an adversary with a corpus of known incident messages can
//     compute their fingerprints and match against the graph — equivalent to a
//     rainbow-table attack. Mitigation: salt per deployment (not yet implemented).
//   - Scaling note: linear scan is acceptable up to ~100 k patterns (microseconds per
//     query on modern hardware). Beyond ~10 M patterns, replace the slice with LSH
//     buckets (e.g., random-projection bands over 64-bit fingerprints) to reduce the
//     candidate set before exact Hamming scoring.
package federated

import (
	"encoding/json"
	"hash/fnv"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

// ---------------------------------------------------------------------------
// Core types
// ---------------------------------------------------------------------------

// Fingerprint is a 64-bit SimHash of an incident description.
// Compatible with (but not importing) the internal/agentic.Fingerprint type.
type Fingerprint uint64

// Outcome is the observed result of a healing attempt.
type Outcome string

const (
	OutcomeResolved  Outcome = "resolved"
	OutcomeFailed    Outcome = "failed"
	OutcomeEscalated Outcome = "escalated"
	OutcomeUnknown   Outcome = "unknown"
)

// outcomeWeight maps outcomes to a score multiplier used when computing Match.Score.
var outcomeWeight = map[Outcome]float64{
	OutcomeResolved:  1.0,
	OutcomeEscalated: 0.5,
	OutcomeUnknown:   0.25,
	OutcomeFailed:    0.0,
}

// Pattern is what a node contributes to the knowledge graph:
//   - A fingerprint of the incident (NOT the raw message)
//   - Which healing action was attempted
//   - What happened (resolved / failed / escalated)
//   - Duration until resolved (or 0 if unresolved)
//
// No raw logs, no service names, no customer data.
type Pattern struct {
	Fingerprint Fingerprint   `json:"fingerprint"`
	Action      string        `json:"action"`
	Outcome     Outcome       `json:"outcome"`
	Duration    time.Duration `json:"duration_ns"`
	Contributed time.Time     `json:"contributed"`
	NodeID      string        `json:"node_id,omitempty"` // blank for full anonymity
}

// Query searches the knowledge graph for patterns whose fingerprint is within
// Hamming distance k of the given incident fingerprint.
type Query struct {
	Fingerprint Fingerprint
	MaxHamming  int     // default 16
	MinOutcome  Outcome // default OutcomeResolved — only return known-good patterns
	Limit       int     // default 10
}

// Match is one nearest-neighbor hit plus its Hamming distance.
type Match struct {
	Pattern  Pattern
	Distance int
	// Score is 1 - distance/64, scaled by the outcome weight.
	Score float64
}

// ---------------------------------------------------------------------------
// Graph
// ---------------------------------------------------------------------------

// GraphConfig controls the in-memory knowledge graph.
type GraphConfig struct {
	// MaxPatterns is the maximum number of patterns stored before oldest are evicted.
	// Default 100_000.
	MaxPatterns int
	// MinK is the minimum number of matching bits required (i.e., MaxHamming = 64-MinK).
	// Not enforced by Graph directly — callers set Query.MaxHamming.
	// Default 48 out of 64.
	MinK int
}

// Graph is an in-memory knowledge graph that maintains patterns and answers
// nearest-neighbor queries. It is thread-safe and bounded.
//
// Storage: a circular slice bounded by GraphConfig.MaxPatterns.
// Query complexity: O(n) linear scan — acceptable up to ~100 k patterns.
// For larger deployments replace with LSH bands over the 64-bit fingerprint space.
type Graph struct {
	mu       sync.RWMutex
	cfg      GraphConfig
	patterns []Pattern // ring buffer
	head     int       // next write position
	size     int       // current fill
}

// NewGraph returns an initialised Graph.
func NewGraph(cfg GraphConfig) *Graph {
	if cfg.MaxPatterns <= 0 {
		cfg.MaxPatterns = 100_000
	}
	if cfg.MinK <= 0 {
		cfg.MinK = 48
	}
	return &Graph{
		cfg:      cfg,
		patterns: make([]Pattern, cfg.MaxPatterns),
	}
}

// Contribute adds a pattern to the graph. When the ring buffer is full the
// oldest pattern is evicted (FIFO).
func (g *Graph) Contribute(p Pattern) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.patterns[g.head] = p
	g.head = (g.head + 1) % g.cfg.MaxPatterns
	if g.size < g.cfg.MaxPatterns {
		g.size++
	}
}

// Query returns at most q.Limit patterns whose Hamming distance to q.Fingerprint
// is <= q.MaxHamming and whose Outcome is at least as good as q.MinOutcome.
// Results are sorted by descending Score.
func (g *Graph) Query(q Query) []Match {
	// Apply defaults.
	if q.MaxHamming <= 0 {
		q.MaxHamming = 16
	}
	if q.MinOutcome == "" {
		q.MinOutcome = OutcomeResolved
	}
	if q.Limit <= 0 {
		q.Limit = 10
	}

	minWeight := outcomeWeight[q.MinOutcome]

	g.mu.RLock()
	defer g.mu.RUnlock()

	matches := make([]Match, 0, q.Limit*2)

	for i := 0; i < g.size; i++ {
		pos := ((g.head - g.size + i) + g.cfg.MaxPatterns) % g.cfg.MaxPatterns
		p := g.patterns[pos]

		// Filter by outcome quality.
		if outcomeWeight[p.Outcome] < minWeight {
			continue
		}

		d := Hamming(q.Fingerprint, p.Fingerprint)
		if d > q.MaxHamming {
			continue
		}

		score := (1.0 - float64(d)/64.0) * outcomeWeight[p.Outcome]
		matches = append(matches, Match{Pattern: p, Distance: d, Score: score})
	}

	// Sort descending by score; ties by ascending distance.
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].Distance < matches[j].Distance
	})

	if len(matches) > q.Limit {
		matches = matches[:q.Limit]
	}
	return matches
}

// Size returns the number of patterns currently stored.
func (g *Graph) Size() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.size
}

// ExportJSON serialises the graph's current patterns to w as a JSON array.
func (g *Graph) ExportJSON(w io.Writer) error {
	g.mu.RLock()
	snap := make([]Pattern, g.size)
	for i := 0; i < g.size; i++ {
		pos := ((g.head - g.size + i) + g.cfg.MaxPatterns) % g.cfg.MaxPatterns
		snap[i] = g.patterns[pos]
	}
	g.mu.RUnlock()

	return json.NewEncoder(w).Encode(snap)
}

// ImportJSON reads a JSON array of patterns from r and contributes each one to
// the graph. Existing patterns are preserved; overflow evicts oldest.
func (g *Graph) ImportJSON(r io.Reader) error {
	var patterns []Pattern
	if err := json.NewDecoder(r).Decode(&patterns); err != nil {
		return err
	}
	for _, p := range patterns {
		g.Contribute(p)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Primitive functions
// ---------------------------------------------------------------------------

// ComputeFingerprint produces a 64-bit Charikar SimHash of text.
// Tokenization: split on whitespace and punctuation, lowercase, skip tokens < 2 chars.
// Per-token hash: FNV-64a.
// This is intentionally self-contained and does NOT import internal/agentic.
func ComputeFingerprint(text string) Fingerprint {
	lower := strings.ToLower(text)
	tokens := strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	var vec [64]int
	h := fnv.New64a()
	for _, tok := range tokens {
		if len(tok) < 2 {
			continue
		}
		h.Reset()
		h.Write([]byte(tok))
		bits := h.Sum64()
		for i := 0; i < 64; i++ {
			if bits&(1<<uint(i)) != 0 {
				vec[i]++
			} else {
				vec[i]--
			}
		}
	}

	var fp Fingerprint
	for i := 0; i < 64; i++ {
		if vec[i] > 0 {
			fp |= Fingerprint(1) << uint(i)
		}
	}
	return fp
}

// Hamming returns the number of bit positions that differ between two fingerprints.
func Hamming(a, b Fingerprint) int {
	x := uint64(a ^ b)
	count := 0
	for x != 0 {
		x &= x - 1
		count++
	}
	return count
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

// KnowledgeClient wraps a local Graph with incident fingerprinting and graph push/pull.
// It is intentionally named KnowledgeClient to avoid collision with the federated.Client
// type defined in federated.go (which handles metric aggregation).
type KnowledgeClient struct {
	mu      sync.Mutex
	nodeID  string
	graph   *Graph
	pending []Pattern
}

// NewKnowledgeClient returns a KnowledgeClient associated with the given nodeID and backing Graph.
func NewKnowledgeClient(nodeID string, g *Graph) *KnowledgeClient {
	return &KnowledgeClient{nodeID: nodeID, graph: g}
}

// Record fingerprints the incident message, stores the pattern locally in the
// graph, and stages it for the next Push.
func (c *KnowledgeClient) Record(message string, action string, outcome Outcome, duration time.Duration) {
	p := Pattern{
		Fingerprint: ComputeFingerprint(message),
		Action:      action,
		Outcome:     outcome,
		Duration:    duration,
		Contributed: time.Now(),
		NodeID:      c.nodeID,
	}
	c.graph.Contribute(p)

	c.mu.Lock()
	c.pending = append(c.pending, p)
	c.mu.Unlock()
}

// Push returns all pending patterns for submission to an aggregator and clears
// the staging buffer. Callers typically invoke this once per round.
func (c *KnowledgeClient) Push() []Pattern {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Pattern, len(c.pending))
	copy(out, c.pending)
	c.pending = c.pending[:0]
	return out
}

// Ingest accepts patterns broadcast by an aggregator and adds them to the local
// graph. Duplicate-suppression is left to the caller (graph simply appends).
func (c *KnowledgeClient) Ingest(patterns []Pattern) {
	for _, p := range patterns {
		c.graph.Contribute(p)
	}
}

// Recommendation is a ranked healing suggestion derived from similar resolved incidents.
type Recommendation struct {
	Action      string
	Frequency   int
	SuccessRate float64
	AvgHamming  float64
	Score       float64
}

// Recommend returns healing actions that resolved similar incidents elsewhere,
// ranked by similarity * success rate.
//
// Algorithm: query the graph with MaxHamming=32, MinOutcome="" (include all),
// then group by Action. SuccessRate = resolved / (resolved + failed).
// Score = freq * successRate / (1 + avgHamming/64).
func (c *KnowledgeClient) Recommend(message string, topN int) []Recommendation {
	if topN <= 0 {
		topN = 5
	}

	fp := ComputeFingerprint(message)
	matches := c.graph.Query(Query{
		Fingerprint: fp,
		MaxHamming:  32,
		MinOutcome:  "",  // include all outcomes for grouping
		Limit:       200, // gather enough to aggregate
	})

	if len(matches) == 0 {
		return nil
	}

	type actionStats struct {
		resolved   int
		failed     int
		total      int
		hammingSum int
	}
	byAction := make(map[string]*actionStats)

	for _, m := range matches {
		a := m.Pattern.Action
		if _, ok := byAction[a]; !ok {
			byAction[a] = &actionStats{}
		}
		s := byAction[a]
		s.total++
		s.hammingSum += m.Distance
		switch m.Pattern.Outcome {
		case OutcomeResolved:
			s.resolved++
		case OutcomeFailed:
			s.failed++
		}
	}

	recs := make([]Recommendation, 0, len(byAction))
	for action, s := range byAction {
		sr := 0.0
		denom := s.resolved + s.failed
		if denom > 0 {
			sr = float64(s.resolved) / float64(denom)
		}
		avgH := float64(s.hammingSum) / float64(s.total)
		score := float64(s.total) * sr / (1.0 + avgH/64.0)
		recs = append(recs, Recommendation{
			Action:      action,
			Frequency:   s.total,
			SuccessRate: sr,
			AvgHamming:  avgH,
			Score:       score,
		})
	}

	sort.Slice(recs, func(i, j int) bool {
		return recs[i].Score > recs[j].Score
	})

	if len(recs) > topN {
		recs = recs[:topN]
	}
	return recs
}
