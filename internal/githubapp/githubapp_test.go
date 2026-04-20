package githubapp

import (
	"strings"
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/evolve"
)

func TestPropose_AddCache_GeneratesGoFile(t *testing.T) {
	p := Propose(evolve.Suggestion{
		Kind:      evolve.AddCache,
		Service:   "catalog",
		Rationale: "hot read path with low hit rate",
		Score:     0.9,
		Effort:    evolve.Medium,
	})
	if p.Draft != true {
		t.Errorf("MVP should always open drafts")
	}
	if !strings.Contains(p.Branch, "add-cache") || !strings.Contains(p.Branch, "catalog") {
		t.Errorf("unexpected branch: %q", p.Branch)
	}
	if len(p.Changes) != 1 {
		t.Fatalf("want 1 change, got %d", len(p.Changes))
	}
	change := p.Changes[0]
	if !strings.HasSuffix(change.Path, "cache.go") {
		t.Errorf("expected a cache.go file; got %q", change.Path)
	}
	if !strings.Contains(change.Content, "package catalog") {
		t.Errorf("package declaration wrong: %s", change.Content[:120])
	}
	if !strings.Contains(change.Content, "func NewCache") {
		t.Errorf("generated code should define NewCache")
	}
}

func TestPropose_AddCircuitBreaker_IncludesErrOpen(t *testing.T) {
	p := Propose(evolve.Suggestion{Kind: evolve.AddCircuitBreaker, Service: "edge-api", Rationale: "high error rate downstream", Score: 0.88, Effort: evolve.Small})
	if !strings.Contains(p.Changes[0].Content, "ErrOpen") {
		t.Errorf("breaker should expose ErrOpen")
	}
	if !strings.Contains(p.Body, "circuit breaker") {
		t.Errorf("body should describe the change; got %q", p.Body[:min(120, len(p.Body))])
	}
}

func TestPropose_TightenTimeout_UpdatesFile(t *testing.T) {
	p := Propose(evolve.Suggestion{Kind: evolve.TightenTimeout, Service: "payments", Rationale: "retry rate 0.4", Score: 0.7, Effort: evolve.Small})
	if p.Changes[0].Action != "update" {
		t.Errorf("tighten-timeout should modify existing file; got action=%q", p.Changes[0].Action)
	}
	if !strings.Contains(p.Changes[0].Content, "DialTimeout") {
		t.Errorf("should define DialTimeout constant")
	}
}

func TestPropose_UnknownKind_FallsBackToManual(t *testing.T) {
	p := Propose(evolve.Suggestion{Kind: evolve.Kind(9999), Service: "x", Score: 0.5})
	if len(p.Changes) != 0 {
		t.Errorf("manual proposals should not auto-generate files")
	}
	if !strings.Contains(p.Title, "manual review") {
		t.Errorf("title should indicate manual review; got %q", p.Title)
	}
}

func TestBody_IncludesRationaleEvidenceImpact(t *testing.T) {
	p := Propose(evolve.Suggestion{
		Kind:      evolve.AddCache,
		Service:   "x",
		Rationale: "RATIONALE_TOKEN",
		Evidence:  []string{"ev-1", "ev-2"},
		Impact:    "IMPACT_TOKEN",
		Score:     0.77,
		Effort:    evolve.Small,
	})
	for _, token := range []string{"RATIONALE_TOKEN", "ev-1", "ev-2", "IMPACT_TOKEN", "Effort", "Score", "Rank"} {
		if !strings.Contains(p.Body, token) {
			t.Errorf("body missing %q; body:\n%s", token, p.Body)
		}
	}
	if !strings.Contains(p.Body, "immortal:reject") {
		t.Errorf("body must tell operator how to reject the pattern")
	}
}

func TestBranchName_SanitizesServiceName(t *testing.T) {
	p := Propose(evolve.Suggestion{Kind: evolve.AddCache, Service: "my-service/with weird chars!", Rationale: "x"})
	// Strip the "immortal/" prefix; only the service chunk is sanitized.
	const prefix = "immortal/"
	if !strings.HasPrefix(p.Branch, prefix) {
		t.Fatalf("branch must start with %q; got %q", prefix, p.Branch)
	}
	serviceChunk := strings.TrimPrefix(p.Branch, prefix)
	// Git branch names allow a-z, 0-9, - and _. No spaces, no !, no /.
	if strings.ContainsAny(serviceChunk, " !/") {
		t.Errorf("sanitized chunk contains invalid chars: %q", serviceChunk)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
