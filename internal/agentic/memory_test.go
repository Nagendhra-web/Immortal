package agentic

import (
	"fmt"
	"testing"
)

func makeTrace(resolved bool) *Trace {
	return &Trace{
		IncidentID: "test",
		Steps:      []Step{{Iteration: 0, Tool: "check_health", Observation: "healthy"}},
		Resolved:   resolved,
	}
}

// TestMemoryRecordAndRecall_ByMessage verifies that a recorded entry can be
// recalled when the query matches its message words.
func TestMemoryRecordAndRecall_ByMessage(t *testing.T) {
	m := NewMemory(10)

	inc := Incident{Message: "database connection refused", Source: "db-monitor", Severity: "critical"}
	m.Record(inc, makeTrace(true), OutcomeResolved)

	if m.Size() != 1 {
		t.Fatalf("expected size 1, got %d", m.Size())
	}

	results := m.Recall(Incident{Message: "database connection refused", Source: "db-monitor", Severity: "critical"}, 5)
	if len(results) != 1 {
		t.Fatalf("expected 1 recall result, got %d", len(results))
	}
	if results[0].Outcome != OutcomeResolved {
		t.Errorf("expected OutcomeResolved, got %s", results[0].Outcome)
	}
	if results[0].Incident.Message != inc.Message {
		t.Errorf("message mismatch: got %q", results[0].Incident.Message)
	}
}

// TestMemoryCapacityRingBuffer verifies that adding more entries than capacity
// evicts the oldest ones.
func TestMemoryCapacityRingBuffer(t *testing.T) {
	capacity := 3
	m := NewMemory(capacity)

	for i := 0; i < 5; i++ {
		inc := Incident{Message: fmt.Sprintf("incident %d", i), Source: "src", Severity: "low"}
		m.Record(inc, makeTrace(i%2 == 0), OutcomeResolved)
	}

	if m.Size() != capacity {
		t.Fatalf("expected size %d (ring buffer), got %d", capacity, m.Size())
	}

	// The oldest entries (0, 1) should have been evicted; only 2, 3, 4 remain.
	all := m.Recall(Incident{Message: "", Source: "", Severity: ""}, capacity+10)
	if len(all) != capacity {
		t.Fatalf("expected %d recalled entries, got %d", capacity, len(all))
	}

	messages := make(map[string]bool)
	for _, e := range all {
		messages[e.Incident.Message] = true
	}
	for _, evicted := range []string{"incident 0", "incident 1"} {
		if messages[evicted] {
			t.Errorf("evicted entry %q should not be in recall results", evicted)
		}
	}
	for _, kept := range []string{"incident 2", "incident 3", "incident 4"} {
		if !messages[kept] {
			t.Errorf("entry %q should be present in recall results", kept)
		}
	}
}

// TestMemoryRecallRanksBySimilarity verifies that entries with a higher
// overlap in source/severity/message words rank above less-similar entries.
func TestMemoryRecallRanksBySimilarity(t *testing.T) {
	m := NewMemory(10)

	// Low similarity: different source, different severity, unrelated message.
	m.Record(
		Incident{Message: "cpu spike on worker node", Source: "cpu-monitor", Severity: "warning"},
		makeTrace(false),
		OutcomeFailed,
	)

	// High similarity: same source, same severity, overlapping message words.
	m.Record(
		Incident{Message: "database connection refused on primary", Source: "db-monitor", Severity: "critical"},
		makeTrace(true),
		OutcomeResolved,
	)

	query := Incident{Message: "database connection timeout", Source: "db-monitor", Severity: "critical"}
	results := m.Recall(query, 2)

	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// The database entry should rank first.
	if results[0].Incident.Source != "db-monitor" {
		t.Errorf("expected db-monitor entry to rank first, got source=%q", results[0].Incident.Source)
	}
}

// TestMemoryRecallEmpty verifies Recall on empty memory returns nil safely.
func TestMemoryRecallEmpty(t *testing.T) {
	m := NewMemory(10)
	results := m.Recall(Incident{Message: "anything"}, 5)
	if results != nil {
		t.Errorf("expected nil from empty memory, got %v", results)
	}
}
