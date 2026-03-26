package sandbox_test

import (
	"errors"
	"testing"

	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/sandbox"
)

func TestSandboxRunSuccess(t *testing.T) {
	sb := sandbox.New()

	result := sb.Test(
		event.New(event.TypeError, event.SeverityCritical, "crash"),
		func(e *event.Event) error {
			// Simulated fix succeeds
			return nil
		},
	)

	if !result.Safe {
		t.Error("expected safe result for successful fix")
	}
	if result.Error != "" {
		t.Errorf("expected no error, got: %s", result.Error)
	}
}

func TestSandboxRunFailure(t *testing.T) {
	sb := sandbox.New()

	result := sb.Test(
		event.New(event.TypeError, event.SeverityCritical, "crash"),
		func(e *event.Event) error {
			return errors.New("fix made things worse")
		},
	)

	if result.Safe {
		t.Error("expected unsafe result for failing fix")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestSandboxRunPanic(t *testing.T) {
	sb := sandbox.New()

	result := sb.Test(
		event.New(event.TypeError, event.SeverityCritical, "crash"),
		func(e *event.Event) error {
			panic("something terrible")
		},
	)

	if result.Safe {
		t.Error("expected unsafe result for panicking fix")
	}
}

func TestSandboxHistory(t *testing.T) {
	sb := sandbox.New()

	sb.Test(event.New(event.TypeError, event.SeverityCritical, "fix1"),
		func(e *event.Event) error { return nil })
	sb.Test(event.New(event.TypeError, event.SeverityCritical, "fix2"),
		func(e *event.Event) error { return errors.New("fail") })

	history := sb.History()
	if len(history) != 2 {
		t.Fatalf("expected 2 results, got %d", len(history))
	}
	if !history[0].Safe {
		t.Error("first test should be safe")
	}
	if history[1].Safe {
		t.Error("second test should be unsafe")
	}
}
