// Package ebpf is the Immortal observer that derives hot-path signals
// directly from the host kernel.
//
// Platform support:
//
//	Linux:   real implementation reads /proc/net/netstat + /proc/stat +
//	         /proc/<pid>/status to derive TCP retransmit rate, fork rate,
//	         and per-process open-file pressure. Heuristic, not probe-level,
//	         but dependency-free and runs unprivileged.
//	other:   graceful no-op. Observer.Read() returns an empty Snapshot
//	         so the rest of the engine stays portable.
//
// A true eBPF probe implementation (cilium/ebpf, kprobes) is tracked in
// issue #36 as a follow-up; when it lands it will slot in behind the
// same Observer interface defined here.
package ebpf

import (
	"context"
	"sync"
	"time"
)

// Snapshot is the derived state of the host at an instant in time.
// Rates are computed over the interval between the previous Snapshot
// and this one.
type Snapshot struct {
	At                 time.Time
	TCPRetransmitsPer1k uint64 // retransmits per 1000 segments sent in the interval
	ForkRatePerSec      float64
	OpenFilesPressure   float64 // (open / max), system-wide, 0..1
	ContextSwitchesPerSec float64
	RunnableTasks      int
	Source             string // "linux/proc" | "nop"
}

// SignalsReport is the higher-level view derived from a Snapshot,
// suitable for feeding into evolve.SignalBag without the caller knowing
// anything about kernel internals.
type SignalsReport struct {
	RetryStormRisk     float64 // 0..1; high when retransmits/1k > 50
	FDExhaustionRisk   float64 // 0..1; high when OpenFilesPressure > 0.7
	RunawayWorkloadRisk float64 // 0..1; high when ForkRatePerSec > 100
	Source             string
}

// Observer is the contract the rest of the engine codes against. One
// live implementation per platform, plus a no-op for portability.
type Observer interface {
	// Read returns the current Snapshot. Computing rates requires calling
	// Read periodically; the implementation holds the previous sample.
	Read() (Snapshot, error)

	// Signals converts a Snapshot into risk estimates, clamped to 0..1.
	Signals(snap Snapshot) SignalsReport

	// Source describes the backing data source for logs / debugging.
	Source() string
}

// New returns the best Observer available on the current platform.
// On Linux, returns a real /proc-backed Observer. On macOS/Windows/etc.
// returns a Nop observer that reports zero-valued Snapshots.
func New() Observer {
	return newPlatformObserver()
}

// Nop is an Observer that reports zero state. Safe to use on any OS.
// The engine can wire this unconditionally and only branch when
// Snapshot.Source is non-"nop".
type Nop struct{}

func (Nop) Read() (Snapshot, error)          { return Snapshot{At: time.Now(), Source: "nop"}, nil }
func (Nop) Signals(_ Snapshot) SignalsReport { return SignalsReport{Source: "nop"} }
func (Nop) Source() string                    { return "nop" }

// Watcher runs an Observer in a goroutine and exposes the latest
// SignalsReport through a lock-guarded accessor. Callers typically spin
// one of these up at engine boot and poll Latest() on a timer.
type Watcher struct {
	obs      Observer
	interval time.Duration

	mu     sync.RWMutex
	latest SignalsReport
	lastErr error
}

// NewWatcher constructs a Watcher with the given interval.
// Zero interval defaults to 5 seconds.
func NewWatcher(obs Observer, interval time.Duration) *Watcher {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if obs == nil {
		obs = Nop{}
	}
	return &Watcher{obs: obs, interval: interval}
}

// Run blocks until ctx is cancelled, reading from the Observer on each
// interval tick and updating Latest().
func (w *Watcher) Run(ctx context.Context) error {
	tick := time.NewTicker(w.interval)
	defer tick.Stop()
	// Prime the observer so rate deltas are non-zero on second read.
	_, _ = w.obs.Read()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			snap, err := w.obs.Read()
			w.mu.Lock()
			if err != nil {
				w.lastErr = err
			} else {
				w.latest = w.obs.Signals(snap)
				w.lastErr = nil
			}
			w.mu.Unlock()
		}
	}
}

// Latest returns the most recent SignalsReport, or the zero value if
// Run has not observed anything yet.
func (w *Watcher) Latest() SignalsReport {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.latest
}

// LastError returns the last error returned by Observer.Read, if any.
func (w *Watcher) LastError() error {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastErr
}
