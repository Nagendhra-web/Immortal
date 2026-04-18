package agentic

import (
	"testing"
)

func TestSimHash_IdenticalText_IdenticalHash(t *testing.T) {
	text := "database connection timeout on service-a"
	a := SimHash(text)
	b := SimHash(text)
	if a != b {
		t.Fatalf("identical text produced different hashes: %016x vs %016x", a, b)
	}
}

func TestSimHash_SimilarText_LowHamming(t *testing.T) {
	a := SimHash("db connection timeout on svc-a")
	b := SimHash("database connection timed out on svc-a")
	h := Hamming(a, b)
	if h >= 24 {
		t.Fatalf("expected Hamming < 24 for similar texts, got %d (a=%016x b=%016x)", h, a, b)
	}
}

func TestSimHash_DifferentText_HighHamming(t *testing.T) {
	a := SimHash("db connection timeout")
	b := SimHash("kafka consumer lag")
	h := Hamming(a, b)
	if h <= 20 {
		t.Fatalf("expected Hamming > 20 for very different texts, got %d (a=%016x b=%016x)", h, a, b)
	}
}

func TestHamming_SameFingerprint_Zero(t *testing.T) {
	fp := SimHash("some incident message")
	if Hamming(fp, fp) != 0 {
		t.Fatal("Hamming of identical fingerprints must be 0")
	}
}

func TestHamming_AllBitsDiffer(t *testing.T) {
	var a Fingerprint = 0x0000000000000000
	var b Fingerprint = 0xFFFFFFFFFFFFFFFF
	if Hamming(a, b) != 64 {
		t.Fatalf("expected 64 for all-bits-differ, got %d", Hamming(a, b))
	}
}

func TestSimHashWeighted_EmptyFeatures(t *testing.T) {
	fp := SimHashWeighted(map[string]int{})
	// All vec[i] == 0, so no bit set — result is 0.
	if fp != 0 {
		t.Fatalf("expected 0 for empty features, got %016x", fp)
	}
}

func TestSemanticMemory_RecallRanksByHamming(t *testing.T) {
	sm := NewSemanticMemory(20)

	// 4 near-duplicate incidents.
	nearDups := []string{
		"db connection timeout on svc-a",
		"database connection timed out on svc-a",
		"db conn timeout svc-a",
		"database connection timeout service-a",
	}
	// 1 dissimilar incident.
	dissimilar := "kafka consumer lag high watermark"

	emptyTrace := &Trace{}
	for _, msg := range nearDups {
		sm.Record(Incident{Message: msg, Source: "s", Severity: "critical"}, emptyTrace, OutcomeResolved)
	}
	sm.Record(Incident{Message: dissimilar, Source: "s", Severity: "critical"}, emptyTrace, OutcomeFailed)

	if sm.Size() != 5 {
		t.Fatalf("expected 5 entries, got %d", sm.Size())
	}

	query := Incident{Message: "db connection timed out on svc-a"}
	results := sm.Recall(query, 4)

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// The dissimilar entry should NOT appear in the top-4.
	for _, r := range results {
		if r.Incident.Message == dissimilar {
			t.Fatal("dissimilar incident appeared in top-4 recall")
		}
	}
}

func TestSemanticMemory_CapacityRingBuffer(t *testing.T) {
	capacity := 3
	sm := NewSemanticMemory(capacity)
	emptyTrace := &Trace{}

	messages := []string{"msg-a", "msg-b", "msg-c", "msg-d", "msg-e"}
	for i, msg := range messages {
		outcome := OutcomeResolved
		if i%2 == 0 {
			outcome = OutcomeFailed
		}
		sm.Record(Incident{Message: msg}, emptyTrace, outcome)
	}

	// Size capped at capacity.
	if sm.Size() != capacity {
		t.Fatalf("expected size %d, got %d", capacity, sm.Size())
	}

	// The 3 most-recently recorded messages should be present.
	results := sm.Recall(Incident{Message: "msg"}, capacity)
	present := make(map[string]bool)
	for _, r := range results {
		present[r.Incident.Message] = true
	}
	for _, expected := range []string{"msg-c", "msg-d", "msg-e"} {
		if !present[expected] {
			t.Errorf("expected %q in ring buffer, not found (got %v)", expected, present)
		}
	}
}

func TestSemanticMemory_RecallEmpty_ReturnsNil(t *testing.T) {
	sm := NewSemanticMemory(10)
	if sm.Recall(Incident{Message: "anything"}, 5) != nil {
		t.Fatal("expected nil recall on empty memory")
	}
}

func TestSemanticMemory_RecallKZero_ReturnsNil(t *testing.T) {
	sm := NewSemanticMemory(10)
	sm.Record(Incident{Message: "something"}, &Trace{}, OutcomeResolved)
	if sm.Recall(Incident{Message: "something"}, 0) != nil {
		t.Fatal("expected nil recall when k=0")
	}
}
