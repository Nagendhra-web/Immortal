package dedup_test

import (
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/dedup"
	"github.com/immortal-engine/immortal/internal/event"
)

func TestFirstEventNotDuplicate(t *testing.T) {
	d := dedup.New(time.Second)
	e := event.New(event.TypeError, event.SeverityError, "crash").WithSource("api")
	if d.IsDuplicate(e) {
		t.Error("first should not be duplicate")
	}
}

func TestSameEventIsDuplicate(t *testing.T) {
	d := dedup.New(time.Second)
	e1 := event.New(event.TypeError, event.SeverityError, "crash").WithSource("api")
	e2 := event.New(event.TypeError, event.SeverityError, "crash").WithSource("api")
	d.IsDuplicate(e1)
	if !d.IsDuplicate(e2) {
		t.Error("same content should be duplicate")
	}
}

func TestDifferentEventsNotDuplicate(t *testing.T) {
	d := dedup.New(time.Second)
	e1 := event.New(event.TypeError, event.SeverityError, "crash1").WithSource("api")
	e2 := event.New(event.TypeError, event.SeverityError, "crash2").WithSource("api")
	d.IsDuplicate(e1)
	if d.IsDuplicate(e2) {
		t.Error("different messages should not be duplicate")
	}
}

func TestDuplicateExpiresAfterWindow(t *testing.T) {
	d := dedup.New(50 * time.Millisecond)
	e := event.New(event.TypeError, event.SeverityError, "crash").WithSource("api")
	d.IsDuplicate(e)
	time.Sleep(100 * time.Millisecond)
	if d.IsDuplicate(e) {
		t.Error("should not be duplicate after window")
	}
}

func TestCleanup(t *testing.T) {
	d := dedup.New(10 * time.Millisecond)
	d.IsDuplicate(event.New(event.TypeError, event.SeverityError, "old").WithSource("a"))
	time.Sleep(20 * time.Millisecond)
	d.Cleanup()
	if d.Size() != 0 {
		t.Error("should be empty after cleanup")
	}
}
