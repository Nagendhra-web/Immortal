package backoff_test

import (
	"errors"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/backoff"
)

func TestBackoffIncreases(t *testing.T) {
	b := backoff.New(100*time.Millisecond, 10*time.Second)
	b.Jitter = false
	d1 := b.Next()
	d2 := b.Next()
	d3 := b.Next()
	if d2 <= d1 {
		t.Error("should increase")
	}
	if d3 <= d2 {
		t.Error("should keep increasing")
	}
}

func TestBackoffMaxCap(t *testing.T) {
	b := backoff.New(time.Second, 5*time.Second)
	b.Jitter = false
	for i := 0; i < 20; i++ {
		b.Next()
	}
	d := b.Next()
	if d > 5*time.Second {
		t.Errorf("should be capped at 5s, got %v", d)
	}
}

func TestBackoffReset(t *testing.T) {
	b := backoff.New(100*time.Millisecond, time.Second)
	b.Jitter = false
	b.Next()
	b.Next()
	b.Next()
	b.Reset()
	if b.Attempt() != 0 {
		t.Error("should be reset")
	}
	d := b.Next()
	if d != 100*time.Millisecond {
		t.Errorf("should be initial after reset, got %v", d)
	}
}

func TestRetrySuccess(t *testing.T) {
	attempts := 0
	b := backoff.New(time.Millisecond, 10*time.Millisecond)
	err := backoff.Retry(5, b, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("not yet")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryExhausted(t *testing.T) {
	b := backoff.New(time.Millisecond, 10*time.Millisecond)
	err := backoff.Retry(3, b, func() error { return errors.New("always fails") })
	if err == nil {
		t.Error("should fail after exhausted attempts")
	}
}
