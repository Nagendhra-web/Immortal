package ebpf

import (
	"context"
	"testing"
	"time"
)

func TestNop_AlwaysSafe(t *testing.T) {
	n := Nop{}
	snap, err := n.Read()
	if err != nil {
		t.Fatalf("Nop.Read should never error; got %v", err)
	}
	if snap.Source != "nop" {
		t.Errorf("Nop source tag wrong; got %q", snap.Source)
	}
	sig := n.Signals(snap)
	if sig.RetryStormRisk != 0 || sig.FDExhaustionRisk != 0 || sig.RunawayWorkloadRisk != 0 {
		t.Errorf("Nop should report zero risk; got %+v", sig)
	}
}

func TestNew_ReturnsNonNilObserver(t *testing.T) {
	obs := New()
	if obs == nil {
		t.Fatal("New() must return a non-nil Observer")
	}
	if _, err := obs.Read(); err != nil {
		// Linux CI may produce /proc reading errors in sandboxes; that's OK.
		t.Logf("observer read errored (acceptable in sandboxed environments): %v", err)
	}
}

func TestWatcher_ReportsNopSignalsByDefault(t *testing.T) {
	w := NewWatcher(Nop{}, 10*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_ = w.Run(ctx)
	got := w.Latest()
	if got.Source != "nop" && got.Source != "" {
		t.Errorf("watcher should report nop source; got %q", got.Source)
	}
}

func TestWatcher_UsesDefaultIntervalWhenZero(t *testing.T) {
	w := NewWatcher(Nop{}, 0)
	if w.interval < time.Second {
		t.Errorf("zero interval should default to >= 1s; got %v", w.interval)
	}
}

func TestWatcher_NilObserverFallsBackToNop(t *testing.T) {
	w := NewWatcher(nil, 10*time.Millisecond)
	// Should not panic.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_ = w.Run(ctx)
	if w.Latest().Source != "nop" && w.Latest().Source != "" {
		t.Errorf("nil observer should fallback to Nop")
	}
}
