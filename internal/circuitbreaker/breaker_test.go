package circuitbreaker_test

import (
	"errors"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/circuitbreaker"
)

func TestBreakerClosedOnSuccess(t *testing.T) {
	b := circuitbreaker.New(3, time.Second)
	err := b.Execute(func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if b.State() != circuitbreaker.StateClosed {
		t.Error("should be closed")
	}
}

func TestBreakerOpensOnFailures(t *testing.T) {
	b := circuitbreaker.New(3, time.Second)
	fail := errors.New("fail")
	for i := 0; i < 3; i++ {
		b.Execute(func() error { return fail })
	}
	if b.State() != circuitbreaker.StateOpen {
		t.Error("should be open after 3 failures")
	}
}

func TestBreakerRejectsWhenOpen(t *testing.T) {
	b := circuitbreaker.New(1, time.Second)
	b.Execute(func() error { return errors.New("fail") })
	err := b.Execute(func() error { return nil })
	if !errors.Is(err, circuitbreaker.ErrCircuitOpen) {
		t.Error("should reject when open")
	}
}

func TestBreakerHalfOpenAfterTimeout(t *testing.T) {
	b := circuitbreaker.New(1, 50*time.Millisecond)
	b.Execute(func() error { return errors.New("fail") })
	time.Sleep(100 * time.Millisecond)
	err := b.Execute(func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if b.State() != circuitbreaker.StateClosed {
		t.Errorf("should be closed after half-open success, got %s", b.State())
	}
}

func TestBreakerReset(t *testing.T) {
	b := circuitbreaker.New(1, time.Second)
	b.Execute(func() error { return errors.New("fail") })
	b.Reset()
	if b.State() != circuitbreaker.StateClosed {
		t.Error("should be closed after reset")
	}
}

func TestBreakerStateChangeCallback(t *testing.T) {
	b := circuitbreaker.New(1, time.Second)
	var changes []circuitbreaker.State
	b.OnStateChange(func(from, to circuitbreaker.State) { changes = append(changes, to) })
	b.Execute(func() error { return errors.New("fail") })
	time.Sleep(50 * time.Millisecond)
	if len(changes) == 0 {
		t.Error("expected state change callback")
	}
}
