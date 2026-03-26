package lifecycle_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/lifecycle"
)

func TestShutdownRunsHooks(t *testing.T) {
	m := lifecycle.New(5 * time.Second)
	var order []string
	m.OnShutdown("db", func(ctx context.Context) error { order = append(order, "db"); return nil })
	m.OnShutdown("http", func(ctx context.Context) error { order = append(order, "http"); return nil })
	errs := m.Shutdown()
	if len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	// Should run in reverse order (LIFO)
	if len(order) != 2 || order[0] != "http" || order[1] != "db" {
		t.Errorf("wrong order: %v", order)
	}
}

func TestShutdownCollectsErrors(t *testing.T) {
	m := lifecycle.New(5 * time.Second)
	m.OnShutdown("fail", func(ctx context.Context) error { return errors.New("failed") })
	m.OnShutdown("ok", func(ctx context.Context) error { return nil })
	errs := m.Shutdown()
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
}

func TestHookCount(t *testing.T) {
	m := lifecycle.New(time.Second)
	if m.HookCount() != 0 {
		t.Error("expected 0")
	}
	m.OnShutdown("a", func(ctx context.Context) error { return nil })
	m.OnShutdown("b", func(ctx context.Context) error { return nil })
	if m.HookCount() != 2 {
		t.Error("expected 2")
	}
}
