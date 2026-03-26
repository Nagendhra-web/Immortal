package timetravel_test

import (
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/timetravel"
)

func TestRecorderCaptureAndReplay(t *testing.T) {
	r := timetravel.New(100)

	// Record events over time
	e1 := event.New(event.TypeMetric, event.SeverityInfo, "cpu: 45%")
	time.Sleep(10 * time.Millisecond)
	e2 := event.New(event.TypeMetric, event.SeverityInfo, "cpu: 50%")
	time.Sleep(10 * time.Millisecond)
	e3 := event.New(event.TypeError, event.SeverityCritical, "crash!")

	r.Record(e1)
	r.Record(e2)
	r.Record(e3)

	// Replay all
	events := r.Replay(time.Time{}, time.Now().Add(time.Second))
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

func TestRecorderReplayTimeRange(t *testing.T) {
	r := timetravel.New(100)

	before := time.Now()
	time.Sleep(20 * time.Millisecond)

	e1 := event.New(event.TypeMetric, event.SeverityInfo, "in range")
	r.Record(e1)

	time.Sleep(20 * time.Millisecond)
	after := time.Now()
	time.Sleep(20 * time.Millisecond)

	e2 := event.New(event.TypeMetric, event.SeverityInfo, "out of range")
	r.Record(e2)

	events := r.Replay(before, after)
	if len(events) != 1 {
		t.Errorf("expected 1 event in range, got %d", len(events))
	}
}

func TestRecorderRewindToBeforeFailure(t *testing.T) {
	r := timetravel.New(100)

	// Normal period
	for i := 0; i < 5; i++ {
		r.Record(event.New(event.TypeMetric, event.SeverityInfo, "normal"))
		time.Sleep(5 * time.Millisecond)
	}

	failTime := time.Now()
	time.Sleep(5 * time.Millisecond)

	// Failure
	r.Record(event.New(event.TypeError, event.SeverityCritical, "CRASH"))

	// Rewind to before failure
	events := r.RewindBefore(failTime, 3)
	if len(events) == 0 {
		t.Error("expected events before failure")
	}
	for _, e := range events {
		if e.Severity == event.SeverityCritical {
			t.Error("should not include the crash event")
		}
	}
}

func TestRecorderSnapshot(t *testing.T) {
	r := timetravel.New(100)

	r.TakeSnapshot("deploy-v1", map[string]interface{}{
		"version": "1.0.0",
		"cpu":     45.0,
		"memory":  60.0,
	})

	r.TakeSnapshot("deploy-v2", map[string]interface{}{
		"version": "2.0.0",
		"cpu":     95.0,
		"memory":  88.0,
	})

	snapshots := r.Snapshots()
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snapshots))
	}

	diff := r.DiffSnapshots("deploy-v1", "deploy-v2")
	if len(diff) == 0 {
		t.Error("expected differences between snapshots")
	}
}
