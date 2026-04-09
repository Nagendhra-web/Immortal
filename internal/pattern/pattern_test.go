package pattern_test

import (
	"sync"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/pattern"
)

func TestNewDefaults(t *testing.T) {
	d := pattern.New(time.Minute, 5)
	if d == nil {
		t.Fatal("New returned nil")
	}
	// A fresh detector has no patterns
	if p := d.Patterns(); len(p) != 0 {
		t.Errorf("expected no patterns, got %d", len(p))
	}
}

func TestRecordAndPatterns(t *testing.T) {
	d := pattern.New(time.Minute, 5)
	for i := 0; i < 5; i++ {
		d.Record("db.connection_lost", "error")
	}
	patterns := d.Patterns()
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}
	if patterns[0].Key != "db.connection_lost" {
		t.Errorf("unexpected key: %s", patterns[0].Key)
	}
	if patterns[0].Count != 5 {
		t.Errorf("expected count=5, got %d", patterns[0].Count)
	}
}

func TestBelowThreshold(t *testing.T) {
	d := pattern.New(time.Minute, 5)
	for i := 0; i < 4; i++ {
		d.Record("db.connection_lost", "error")
	}
	if p := d.Patterns(); len(p) != 0 {
		t.Errorf("expected no patterns below threshold, got %d", len(p))
	}
}

func TestMultiplePatterns(t *testing.T) {
	d := pattern.New(time.Minute, 3)

	// "alpha" appears 7 times, "beta" 4 times, "gamma" 2 times (below threshold)
	for i := 0; i < 7; i++ {
		d.Record("alpha", "error")
	}
	for i := 0; i < 4; i++ {
		d.Record("beta", "warning")
	}
	for i := 0; i < 2; i++ {
		d.Record("gamma", "info")
	}

	patterns := d.Patterns()
	if len(patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(patterns))
	}
	// Sorted by count desc: alpha first, then beta
	if patterns[0].Key != "alpha" {
		t.Errorf("expected alpha first, got %s", patterns[0].Key)
	}
	if patterns[0].Count != 7 {
		t.Errorf("expected alpha count=7, got %d", patterns[0].Count)
	}
	if patterns[1].Key != "beta" {
		t.Errorf("expected beta second, got %s", patterns[1].Key)
	}
	if patterns[1].Count != 4 {
		t.Errorf("expected beta count=4, got %d", patterns[1].Count)
	}
}

func TestIsRepeating(t *testing.T) {
	d := pattern.New(time.Minute, 3)

	d.Record("crash", "critical")
	d.Record("crash", "critical")
	if d.IsRepeating("crash") {
		t.Error("should not be repeating with only 2 occurrences (threshold=3)")
	}

	d.Record("crash", "critical")
	if !d.IsRepeating("crash") {
		t.Error("should be repeating with 3 occurrences (threshold=3)")
	}

	if d.IsRepeating("unknown") {
		t.Error("unknown key should not be repeating")
	}
}

func TestCount(t *testing.T) {
	d := pattern.New(time.Minute, 5)

	if d.Count("x") != 0 {
		t.Error("count of unseen key should be 0")
	}

	d.Record("x", "info")
	d.Record("x", "info")
	d.Record("x", "info")

	if d.Count("x") != 3 {
		t.Errorf("expected count=3, got %d", d.Count("x"))
	}
}

func TestPruneOldEntries(t *testing.T) {
	window := 50 * time.Millisecond
	d := pattern.New(window, 2)

	d.Record("stale", "error")
	d.Record("stale", "error")
	d.Record("stale", "error")

	// Entries should be visible now
	if d.Count("stale") < 2 {
		t.Fatal("entries should be counted before window expires")
	}

	time.Sleep(window + 20*time.Millisecond)

	// After window expires, a new Record triggers prune
	d.Record("fresh", "info")

	if d.Count("stale") != 0 {
		t.Errorf("stale entries should be pruned, got count=%d", d.Count("stale"))
	}
}

func TestReset(t *testing.T) {
	d := pattern.New(time.Minute, 2)
	d.Record("key", "error")
	d.Record("key", "error")
	d.Record("key", "error")

	d.Reset()

	if d.Count("key") != 0 {
		t.Errorf("after reset, count should be 0, got %d", d.Count("key"))
	}
	if p := d.Patterns(); len(p) != 0 {
		t.Errorf("after reset, patterns should be empty, got %d", len(p))
	}
}

func TestConcurrentAccess(t *testing.T) {
	d := pattern.New(time.Minute, 10)
	const goroutines = 50
	const recsEach = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < recsEach; j++ {
				d.Record("concurrent-key", "error")
				_ = d.Count("concurrent-key")
				_ = d.IsRepeating("concurrent-key")
			}
		}(i)
	}
	wg.Wait()

	// After all goroutines, total count must be goroutines * recsEach
	total := d.Count("concurrent-key")
	expected := goroutines * recsEach
	if total != expected {
		t.Errorf("expected count=%d after concurrent writes, got %d", expected, total)
	}
}
