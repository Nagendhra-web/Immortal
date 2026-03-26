package rollback_test

import (
	"testing"

	"github.com/immortal-engine/immortal/internal/rollback"
)

func TestRecordAndRollback(t *testing.T) {
	m := rollback.New(10)
	undone := false
	id := m.Record("restart", func() error { undone = true; return nil })
	if err := m.Rollback(id); err != nil {
		t.Fatal(err)
	}
	if !undone {
		t.Error("undo should have been called")
	}
}

func TestRollbackLast(t *testing.T) {
	m := rollback.New(10)
	order := []string{}
	m.Record("a", func() error { order = append(order, "a"); return nil })
	m.Record("b", func() error { order = append(order, "b"); return nil })
	m.RollbackLast()
	if len(order) != 1 || order[0] != "b" {
		t.Errorf("should rollback last, got %v", order)
	}
}

func TestRollbackNotFound(t *testing.T) {
	m := rollback.New(10)
	if err := m.Rollback("fake"); err == nil {
		t.Error("should error")
	}
}

func TestRollbackEmpty(t *testing.T) {
	m := rollback.New(10)
	if err := m.RollbackLast(); err == nil {
		t.Error("should error on empty")
	}
}

func TestMaxSize(t *testing.T) {
	m := rollback.New(3)
	for i := 0; i < 5; i++ {
		m.Record("action", func() error { return nil })
	}
	if m.Size() != 3 {
		t.Errorf("expected 3, got %d", m.Size())
	}
}

func TestHistory(t *testing.T) {
	m := rollback.New(10)
	m.Record("a", func() error { return nil })
	m.Record("b", func() error { return nil })
	if len(m.History()) != 2 {
		t.Error("expected 2 history entries")
	}
}
