package distributed_test

import (
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/distributed"
)

func TestStateStorePutGet(t *testing.T) {
	s := distributed.NewStateStore()
	s.Put("service:api", map[string]string{"status": "healthy"}, distributed.EntryConfig, "node-1")

	entry, ok := s.Get("service:api")
	if !ok {
		t.Fatal("expected entry")
	}
	if entry.NodeID != "node-1" {
		t.Error("wrong node ID")
	}
	if entry.Version != 1 {
		t.Errorf("expected version 1, got %d", entry.Version)
	}
}

func TestStateStoreDelete(t *testing.T) {
	s := distributed.NewStateStore()
	s.Put("key1", "val1", distributed.EntryConfig, "n1")
	s.Delete("key1")
	_, ok := s.Get("key1")
	if ok {
		t.Error("should be deleted")
	}
}

func TestStateStoreByType(t *testing.T) {
	s := distributed.NewStateStore()
	s.Put("h1", "heal1", distributed.EntryHealing, "n1")
	s.Put("h2", "heal2", distributed.EntryHealing, "n1")
	s.Put("c1", "conf1", distributed.EntryConfig, "n1")

	heals := s.ByType(distributed.EntryHealing)
	if len(heals) != 2 {
		t.Errorf("expected 2 healing entries, got %d", len(heals))
	}

	configs := s.ByType(distributed.EntryConfig)
	if len(configs) != 1 {
		t.Errorf("expected 1 config entry, got %d", len(configs))
	}
}

func TestDistributedLock(t *testing.T) {
	s := distributed.NewStateStore()

	// Node 1 acquires lock
	if !s.TryLock("heal:api", "node-1", time.Second) {
		t.Error("should acquire lock")
	}

	// Node 2 cannot acquire same lock
	if s.TryLock("heal:api", "node-2", time.Second) {
		t.Error("should NOT acquire lock held by node-1")
	}

	// Node 1 can re-acquire (idempotent)
	if !s.TryLock("heal:api", "node-1", time.Second) {
		t.Error("node-1 should be able to re-acquire own lock")
	}

	// Unlock
	s.Unlock("heal:api", "node-1")

	// Now node 2 can acquire
	if !s.TryLock("heal:api", "node-2", time.Second) {
		t.Error("node-2 should acquire after unlock")
	}
}

func TestLockExpiry(t *testing.T) {
	s := distributed.NewStateStore()
	s.TryLock("key", "node-1", 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond)

	// Lock expired — another node can acquire
	if !s.TryLock("key", "node-2", time.Second) {
		t.Error("should acquire expired lock")
	}
}

func TestVersionIncrement(t *testing.T) {
	s := distributed.NewStateStore()
	s.Put("a", 1, distributed.EntryConfig, "n1")
	s.Put("b", 2, distributed.EntryConfig, "n1")
	s.Put("c", 3, distributed.EntryConfig, "n1")
	if s.CurrentVersion() != 3 {
		t.Errorf("expected version 3, got %d", s.CurrentVersion())
	}
}
