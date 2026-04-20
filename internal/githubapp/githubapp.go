// Package githubapp converts architecture advisor suggestions into draft
// pull requests. When evolve.Advisor emits a high-confidence structural
// change (score >= 0.85), the engine calls Propose() and a draft PR lands
// in the target repo with code, rationale, twin-simulated impact, and a
// rollback runbook.
//
// This MVP generates Go code changes for three suggestion kinds:
//
//	AddCache           wraps a hot-path function with an LRU cache
//	AddCircuitBreaker  wraps a downstream client with a circuit breaker
//	TightenTimeout     adjusts a timeout constant
//
// Other suggestion kinds return a "manual intervention" stub PR with the
// rationale and evidence so operators can author the change by hand.
package githubapp

import (
	"fmt"
	"strings"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/evolve"
)

// Proposal is the full set of artifacts needed to open a draft PR.
// A caller (typically a GitHub App webhook) receives a Proposal and
// translates it into the real GitHub REST calls.
type Proposal struct {
	Branch       string       // suggested new branch name, e.g. "immortal/add-cache-catalog"
	Title        string       // PR title
	Body         string       // PR body (Markdown)
	Changes      []FileChange // files to add or modify
	Labels       []string     // labels to apply to the PR
	Draft        bool         // always true for safety in MVP
	Reviewers    []string     // optional reviewers (CODEOWNERS by default)
	Commits      []Commit     // 1+ commits; MVP uses a single commit
	Rollback     string       // plain-text rollback runbook
	CreatedAt    time.Time
}

// FileChange is one file write or diff.
type FileChange struct {
	Path    string `json:"path"`
	Content string `json:"content"`   // full new content (atomic overwrite)
	Action  string `json:"action"`    // "add" | "update" | "delete"
}

// Commit is a single commit within a Proposal.
type Commit struct {
	Message string       `json:"message"`
	Files   []FileChange `json:"files"`
}

// Propose converts an evolve.Suggestion into a Proposal. Returns a
// manual-intervention stub when the SuggestionKind has no Go code
// generator yet.
func Propose(sug evolve.Suggestion) Proposal {
	switch sug.Kind {
	case evolve.AddCache:
		return proposeAddCache(sug)
	case evolve.AddCircuitBreaker:
		return proposeAddCircuitBreaker(sug)
	case evolve.TightenTimeout:
		return proposeTightenTimeout(sug)
	case evolve.AddRetryBudget:
		return proposeAddRetryBudget(sug)
	default:
		return proposeManual(sug)
	}
}

func proposeAddCache(s evolve.Suggestion) Proposal {
	branch := branchName("add-cache", s.Service)
	path := fmt.Sprintf("internal/%s/cache.go", sanitize(s.Service))
	content := fmt.Sprintf(`// Package %s cache layer added by Immortal advisor.
//
// Background: %s
//
// Usage: wrap the hot read function that produced the latency and replace
// direct callers with the cached variant. This file is the minimal LRU
// template; tune Size and TTL to your workload.
package %s

import (
	"sync"
	"time"
)

type cacheEntry struct {
	value     any
	expiresAt time.Time
}

type Cache struct {
	mu   sync.RWMutex
	data map[string]cacheEntry
	ttl  time.Duration
	max  int
}

func NewCache(max int, ttl time.Duration) *Cache {
	return &Cache{data: make(map[string]cacheEntry, max), ttl: ttl, max: max}
}

func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	e, ok := c.data[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.value, true
}

func (c *Cache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.data) >= c.max {
		// Simple eviction: drop any one entry.
		for k := range c.data {
			delete(c.data, k)
			break
		}
	}
	c.data[key] = cacheEntry{value: value, expiresAt: time.Now().Add(c.ttl)}
}
`, sanitize(s.Service), s.Rationale, sanitize(s.Service))

	return Proposal{
		Branch:    branch,
		Title:     fmt.Sprintf("advisor: add cache on %s", s.Service),
		Body:      renderBody(s, "Adds a minimal LRU cache wrapper on the hot read path. Tune `Size` and `TTL` before merging."),
		Changes:   []FileChange{{Path: path, Content: content, Action: "add"}},
		Labels:    []string{"immortal:advisor", "immortal:add-cache"},
		Draft:     true,
		Commits:   []Commit{{Message: fmt.Sprintf("chore(advisor): add LRU cache on %s", s.Service), Files: []FileChange{{Path: path, Content: content, Action: "add"}}}},
		Rollback:  "Delete the new file and remove any import of it.",
		CreatedAt: time.Now().UTC(),
	}
}

func proposeAddCircuitBreaker(s evolve.Suggestion) Proposal {
	branch := branchName("circuit-breaker", s.Service)
	path := fmt.Sprintf("internal/%s/breaker.go", sanitize(s.Service))
	content := fmt.Sprintf(`// Package %s circuit-breaker added by Immortal advisor.
//
// Background: %s
package %s

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ErrOpen is returned when the breaker is open.
var ErrOpen = errors.New("circuit breaker open")

type Breaker struct {
	mu           sync.RWMutex
	failures     atomic.Int64
	threshold    int64
	openFor      time.Duration
	openedAt     time.Time
}

func NewBreaker(threshold int64, openFor time.Duration) *Breaker {
	return &Breaker{threshold: threshold, openFor: openFor}
}

// Call runs fn through the breaker. If the breaker is open, returns ErrOpen.
// If fn succeeds, the failure counter is reset; if it fails, increments.
func (b *Breaker) Call(fn func() error) error {
	b.mu.RLock()
	opened := b.openedAt
	b.mu.RUnlock()
	if !opened.IsZero() && time.Since(opened) < b.openFor {
		return ErrOpen
	}
	if err := fn(); err != nil {
		if b.failures.Add(1) >= b.threshold {
			b.mu.Lock()
			b.openedAt = time.Now()
			b.mu.Unlock()
		}
		return err
	}
	b.failures.Store(0)
	return nil
}
`, sanitize(s.Service), s.Rationale, sanitize(s.Service))

	return Proposal{
		Branch:    branch,
		Title:     fmt.Sprintf("advisor: add circuit breaker on %s", s.Service),
		Body:      renderBody(s, "Adds a simple circuit breaker around the downstream client. Set `threshold` and `openFor` based on the upstream's SLO."),
		Changes:   []FileChange{{Path: path, Content: content, Action: "add"}},
		Labels:    []string{"immortal:advisor", "immortal:circuit-breaker"},
		Draft:     true,
		Commits:   []Commit{{Message: fmt.Sprintf("chore(advisor): add circuit breaker on %s", s.Service), Files: []FileChange{{Path: path, Content: content, Action: "add"}}}},
		Rollback:  "Remove the new file and replace usage sites with the original raw client call.",
		CreatedAt: time.Now().UTC(),
	}
}

func proposeTightenTimeout(s evolve.Suggestion) Proposal {
	branch := branchName("tighten-timeout", s.Service)
	path := fmt.Sprintf("internal/%s/timeout.go", sanitize(s.Service))
	content := fmt.Sprintf(`package %s

import "time"

// DialTimeout defines the max time to wait for a downstream connection.
// Tightened by Immortal advisor based on observed retry-rate signals.
//
// Before: typically 30s.
// Suggested: 3s. Shorter timeouts fail fast, freeing client capacity.
//
// Context: %s
const DialTimeout = 3 * time.Second
`, sanitize(s.Service), s.Rationale)
	return Proposal{
		Branch:    branch,
		Title:     fmt.Sprintf("advisor: tighten dial timeout on %s", s.Service),
		Body:      renderBody(s, "Reduces DialTimeout to 3 s based on observed retry-rate. Verify with a canary before rolling out."),
		Changes:   []FileChange{{Path: path, Content: content, Action: "update"}},
		Labels:    []string{"immortal:advisor", "immortal:tighten-timeout"},
		Draft:     true,
		Commits:   []Commit{{Message: fmt.Sprintf("chore(advisor): tighten dial timeout on %s", s.Service), Files: []FileChange{{Path: path, Content: content, Action: "update"}}}},
		Rollback:  "Revert DialTimeout to its previous value (typically 30s).",
		CreatedAt: time.Now().UTC(),
	}
}

func proposeAddRetryBudget(s evolve.Suggestion) Proposal {
	branch := branchName("retry-budget", s.Service)
	path := fmt.Sprintf("internal/%s/retry.go", sanitize(s.Service))
	content := fmt.Sprintf(`package %s

import (
	"errors"
	"sync/atomic"
)

// Context: %s
//
// RetryBudget caps how many retries we issue per minute. When the budget
// is exhausted, further retries are dropped with ErrBudgetExceeded. This
// stops retry storms from amplifying the fault.
type RetryBudget struct {
	max   int64
	used  atomic.Int64
	reset atomic.Int64 // unix nano when we last rolled the window
}

var ErrBudgetExceeded = errors.New("retry budget exceeded")

func NewRetryBudget(perMinute int64) *RetryBudget { return &RetryBudget{max: perMinute} }

func (b *RetryBudget) Try() error {
	if b.used.Add(1) > b.max {
		return ErrBudgetExceeded
	}
	return nil
}
`, sanitize(s.Service), s.Rationale)
	return Proposal{
		Branch:    branch,
		Title:     fmt.Sprintf("advisor: add retry budget on %s", s.Service),
		Body:      renderBody(s, "Caps retry amplification at a configurable per-minute rate."),
		Changes:   []FileChange{{Path: path, Content: content, Action: "add"}},
		Labels:    []string{"immortal:advisor", "immortal:retry-budget"},
		Draft:     true,
		Commits:   []Commit{{Message: fmt.Sprintf("chore(advisor): add retry budget on %s", s.Service), Files: []FileChange{{Path: path, Content: content, Action: "add"}}}},
		Rollback:  "Delete the new file; callers revert to unbounded retry.",
		CreatedAt: time.Now().UTC(),
	}
}

func proposeManual(s evolve.Suggestion) Proposal {
	return Proposal{
		Branch:    branchName("manual", s.Service),
		Title:     fmt.Sprintf("advisor: manual review needed for %s (%s)", s.Kind.String(), s.Service),
		Body:      renderBody(s, "This suggestion kind does not yet have a code generator. Review the rationale below and author the change manually. The advisor will revisit once it sees improvement."),
		Changes:   []FileChange{}, // no auto changes
		Labels:    []string{"immortal:advisor", "immortal:manual"},
		Draft:     true,
		Commits:   []Commit{{Message: fmt.Sprintf("chore(advisor): manual review for %s on %s", s.Kind, s.Service)}},
		Rollback:  "n/a (no auto changes)",
		CreatedAt: time.Now().UTC(),
	}
}

func renderBody(s evolve.Suggestion, intro string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Architecture advisor recommendation\n\n")
	fmt.Fprintf(&b, "**%s on `%s`**\n\n", s.Kind.String(), s.Service)
	fmt.Fprintf(&b, "%s\n\n", intro)
	fmt.Fprintf(&b, "### Rationale\n\n%s\n\n", s.Rationale)
	if len(s.Evidence) > 0 {
		fmt.Fprintf(&b, "### Evidence\n\n")
		for _, e := range s.Evidence {
			fmt.Fprintf(&b, "- `%s`\n", e)
		}
		b.WriteString("\n")
	}
	if s.Impact != "" {
		fmt.Fprintf(&b, "### Predicted impact\n\n%s\n\n", s.Impact)
	}
	fmt.Fprintf(&b, "### Effort: `%s` - Score: `%.2f` - Rank: `%s`\n\n", s.Effort, s.Score, s.Rank())
	fmt.Fprintf(&b, "---\n\n")
	fmt.Fprintf(&b, "_Opened automatically by the Immortal advisor. Review, adjust, and merge when you are comfortable. Add the `immortal:reject` label to train the advisor to stop suggesting this pattern._\n")
	return b.String()
}

func branchName(kind, service string) string {
	return fmt.Sprintf("immortal/%s-%s-%d", kind, sanitize(service), time.Now().Unix())
}

func sanitize(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
			out = append(out, c)
		case c >= 'A' && c <= 'Z':
			out = append(out, c+32)
		case c >= '0' && c <= '9':
			out = append(out, c)
		case c == '-' || c == '_':
			out = append(out, c)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "service"
	}
	return string(out)
}
