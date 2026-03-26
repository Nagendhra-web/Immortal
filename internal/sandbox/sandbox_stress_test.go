package sandbox_test

import (
	"sync"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/sandbox"
)

func TestSandboxConcurrentTests(t *testing.T) {
	sb := sandbox.New()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sb.Test(event.New(event.TypeError, event.SeverityCritical, "test"),
				func(e *event.Event) error { return nil })
		}()
	}
	wg.Wait()

	if len(sb.History()) != 100 {
		t.Errorf("expected 100 results, got %d", len(sb.History()))
	}
}

func TestSandboxSuccessRate(t *testing.T) {
	sb := sandbox.New()

	// 3 success, 1 fail
	sb.Test(event.New(event.TypeError, event.SeverityError, "1"), func(e *event.Event) error { return nil })
	sb.Test(event.New(event.TypeError, event.SeverityError, "2"), func(e *event.Event) error { return nil })
	sb.Test(event.New(event.TypeError, event.SeverityError, "3"), func(e *event.Event) error { return nil })
	sb.Test(event.New(event.TypeError, event.SeverityError, "4"), func(e *event.Event) error { panic("boom") })

	rate := sb.SuccessRate()
	if rate != 0.75 {
		t.Errorf("expected 0.75 success rate, got %f", rate)
	}
}

func TestSandboxEmptySuccessRate(t *testing.T) {
	sb := sandbox.New()
	if sb.SuccessRate() != 1.0 {
		t.Error("empty sandbox should have 1.0 success rate")
	}
}

func TestSandboxActionTimeout(t *testing.T) {
	sb := sandbox.New()
	result := sb.Test(event.New(event.TypeError, event.SeverityError, "slow"),
		func(e *event.Event) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})

	if !result.Safe {
		t.Error("slow but successful action should be safe")
	}
	if result.Duration < 10*time.Millisecond {
		t.Error("duration should reflect actual execution time")
	}
}
