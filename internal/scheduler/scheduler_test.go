package scheduler_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/scheduler"
)

func TestSchedulerRunsJobs(t *testing.T) {
	s := scheduler.New()
	var count atomic.Int64
	s.Add(scheduler.Job{Name: "counter", Interval: 50 * time.Millisecond, Fn: func() { count.Add(1) }})
	s.Start()
	time.Sleep(180 * time.Millisecond)
	s.Stop()
	if count.Load() < 2 {
		t.Errorf("expected at least 2 runs, got %d", count.Load())
	}
}

func TestSchedulerMultipleJobs(t *testing.T) {
	s := scheduler.New()
	var c1, c2 atomic.Int64
	s.Add(scheduler.Job{Name: "j1", Interval: 50 * time.Millisecond, Fn: func() { c1.Add(1) }})
	s.Add(scheduler.Job{Name: "j2", Interval: 50 * time.Millisecond, Fn: func() { c2.Add(1) }})
	s.Start()
	time.Sleep(120 * time.Millisecond)
	s.Stop()
	if c1.Load() == 0 || c2.Load() == 0 {
		t.Error("both jobs should have run")
	}
}

func TestSchedulerJobCount(t *testing.T) {
	s := scheduler.New()
	s.Add(scheduler.Job{Name: "a", Interval: time.Second, Fn: func() {}})
	s.Add(scheduler.Job{Name: "b", Interval: time.Second, Fn: func() {}})
	if s.JobCount() != 2 {
		t.Error("expected 2 jobs")
	}
}
