package audit_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/audit"
)

func TestNewDefaults(t *testing.T) {
	l := audit.New(0)
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
	if l.Count() != 0 {
		t.Errorf("expected 0 entries, got %d", l.Count())
	}
}

func TestLog(t *testing.T) {
	l := audit.New(100)
	before := time.Now()
	e := l.Log("create", "admin", "service-a", "created service", true)
	after := time.Now()

	if e == nil {
		t.Fatal("expected non-nil entry")
	}
	if e.Action != "create" {
		t.Errorf("expected action 'create', got %q", e.Action)
	}
	if e.Actor != "admin" {
		t.Errorf("expected actor 'admin', got %q", e.Actor)
	}
	if e.Target != "service-a" {
		t.Errorf("expected target 'service-a', got %q", e.Target)
	}
	if e.Detail != "created service" {
		t.Errorf("expected detail 'created service', got %q", e.Detail)
	}
	if !e.Success {
		t.Error("expected success=true")
	}
	if e.Timestamp.Before(before) || e.Timestamp.After(after) {
		t.Error("timestamp out of expected range")
	}
	if e.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestLogWithDuration(t *testing.T) {
	l := audit.New(100)
	d := 42 * time.Millisecond
	e := l.LogWithDuration("deploy", "ci", "service-b", "deployed", true, d)
	if e.Duration != d {
		t.Errorf("expected duration %v, got %v", d, e.Duration)
	}
}

func TestAutoID(t *testing.T) {
	l := audit.New(100)
	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		e := l.Log("action", "actor", "target", "detail", true)
		if ids[e.ID] {
			t.Fatalf("duplicate ID: %s", e.ID)
		}
		ids[e.ID] = true
		if !strings.HasPrefix(e.ID, "audit-") {
			t.Errorf("expected ID prefix 'audit-', got %q", e.ID)
		}
	}
}

func TestEntries(t *testing.T) {
	l := audit.New(100)
	for i := 0; i < 10; i++ {
		l.Log("action", "actor", "target", "detail", true)
	}
	entries := l.Entries(5)
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}
	// newest first: IDs should be descending (audit-10, audit-9, ...)
	for i := 1; i < len(entries); i++ {
		if entries[i-1].Timestamp.Before(entries[i].Timestamp) {
			t.Error("entries not in newest-first order")
		}
	}
}

func TestEntriesAll(t *testing.T) {
	l := audit.New(100)
	for i := 0; i < 5; i++ {
		l.Log("action", "actor", "target", "detail", true)
	}
	entries := l.Entries(0)
	if len(entries) != 5 {
		t.Errorf("expected 5 entries with limit=0, got %d", len(entries))
	}
}

func TestEntriesByAction(t *testing.T) {
	l := audit.New(100)
	l.Log("create", "admin", "svc-a", "d", true)
	l.Log("delete", "admin", "svc-b", "d", true)
	l.Log("create", "admin", "svc-c", "d", true)

	results := l.EntriesByAction("create")
	if len(results) != 2 {
		t.Fatalf("expected 2 'create' entries, got %d", len(results))
	}
	for _, e := range results {
		if e.Action != "create" {
			t.Errorf("expected action 'create', got %q", e.Action)
		}
	}
}

func TestEntriesByActor(t *testing.T) {
	l := audit.New(100)
	l.Log("action", "alice", "t", "d", true)
	l.Log("action", "bob", "t", "d", true)
	l.Log("action", "alice", "t", "d", true)

	results := l.EntriesByActor("alice")
	if len(results) != 2 {
		t.Fatalf("expected 2 entries for alice, got %d", len(results))
	}
	for _, e := range results {
		if e.Actor != "alice" {
			t.Errorf("expected actor 'alice', got %q", e.Actor)
		}
	}
}

func TestEntriesByTarget(t *testing.T) {
	l := audit.New(100)
	l.Log("action", "actor", "db", "d", true)
	l.Log("action", "actor", "api", "d", true)
	l.Log("action", "actor", "db", "d", true)

	results := l.EntriesByTarget("db")
	if len(results) != 2 {
		t.Fatalf("expected 2 entries for target 'db', got %d", len(results))
	}
	for _, e := range results {
		if e.Target != "db" {
			t.Errorf("expected target 'db', got %q", e.Target)
		}
	}
}

func TestCount(t *testing.T) {
	l := audit.New(100)
	for i := 0; i < 7; i++ {
		l.Log("a", "b", "c", "d", true)
	}
	if l.Count() != 7 {
		t.Errorf("expected count 7, got %d", l.Count())
	}
}

func TestSince(t *testing.T) {
	l := audit.New(100)
	l.Log("action", "actor", "target", "before", true)
	time.Sleep(10 * time.Millisecond)
	mark := time.Now()
	time.Sleep(10 * time.Millisecond)
	l.Log("action", "actor", "target", "after1", true)
	l.Log("action", "actor", "target", "after2", true)

	results := l.Since(mark)
	if len(results) != 2 {
		t.Fatalf("expected 2 entries since mark, got %d", len(results))
	}
	for _, e := range results {
		if e.Timestamp.Before(mark) {
			t.Error("entry timestamp is before mark")
		}
	}
}

func TestSearch(t *testing.T) {
	l := audit.New(100)
	l.Log("deploy", "ci-bot", "api-gateway", "deployed v1.2", true)
	l.Log("restart", "admin", "worker", "manual restart", true)
	l.Log("scale", "autoscaler", "db-primary", "scale up", true)
	l.Log("resize", "autoscaler", "cache", "scale down", true)

	results := l.Search("deploy")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'deploy', got %d", len(results))
	}

	// case-insensitive: matches entry 3 (action "scale", detail "scale up")
	// and entry 4 (detail "scale down")
	results = l.Search("SCALE")
	if len(results) != 2 {
		t.Fatalf("expected 2 results for 'SCALE', got %d", len(results))
	}

	results = l.Search("nonexistent-xyz")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestMaxEntriesCap(t *testing.T) {
	max := 5
	l := audit.New(max)
	for i := 0; i < 10; i++ {
		l.Log("action", "actor", "target", "detail", true)
	}
	if l.Count() != max {
		t.Errorf("expected count capped at %d, got %d", max, l.Count())
	}
	// most recent entries should be kept
	entries := l.Entries(0)
	if len(entries) != max {
		t.Errorf("expected %d entries returned, got %d", max, len(entries))
	}
}

func TestConcurrentLog(t *testing.T) {
	l := audit.New(10000)
	n := 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			l.Log("concurrent", "goroutine", "target", "detail", true)
		}()
	}
	wg.Wait()
	if l.Count() != n {
		t.Errorf("expected %d entries after concurrent log, got %d", n, l.Count())
	}
}
