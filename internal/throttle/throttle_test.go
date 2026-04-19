package throttle_test

import (
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/event"
	"github.com/Nagendhra-web/Immortal/internal/throttle"
)

func TestThrottlerAllowsFirst(t *testing.T) {
	th := throttle.New(time.Second)
	e := event.New(event.TypeError, event.SeverityError, "crash").WithSource("api")
	if !th.Allow(e) {
		t.Error("first event should be allowed")
	}
}

func TestThrottlerBlocksDuplicate(t *testing.T) {
	th := throttle.New(time.Second)
	e := event.New(event.TypeError, event.SeverityError, "crash").WithSource("api")
	th.Allow(e)
	e2 := event.New(event.TypeError, event.SeverityError, "crash").WithSource("api")
	if th.Allow(e2) {
		t.Error("duplicate should be blocked within interval")
	}
}

func TestThrottlerAllowsAfterInterval(t *testing.T) {
	th := throttle.New(50 * time.Millisecond)
	e := event.New(event.TypeError, event.SeverityError, "crash").WithSource("api")
	th.Allow(e)
	time.Sleep(100 * time.Millisecond)
	e2 := event.New(event.TypeError, event.SeverityError, "crash").WithSource("api")
	if !th.Allow(e2) {
		t.Error("should allow after interval")
	}
}

func TestThrottlerDifferentEvents(t *testing.T) {
	th := throttle.New(time.Second)
	e1 := event.New(event.TypeError, event.SeverityError, "crash1").WithSource("api")
	e2 := event.New(event.TypeError, event.SeverityError, "crash2").WithSource("api")
	if !th.Allow(e1) {
		t.Error("first should be allowed")
	}
	if !th.Allow(e2) {
		t.Error("different message should be allowed")
	}
}

func TestThrottlerCleanup(t *testing.T) {
	th := throttle.New(10 * time.Millisecond)
	th.Allow(event.New(event.TypeError, event.SeverityError, "old").WithSource("a"))
	time.Sleep(30 * time.Millisecond)
	th.Cleanup()
	if th.Size() != 0 {
		t.Errorf("expected 0 after cleanup, got %d", th.Size())
	}
}
