package scheduler

import (
	"sync"
	"time"
)

type Job struct {
	Name     string
	Interval time.Duration
	Fn       func()
}

type Scheduler struct {
	mu   sync.Mutex
	jobs []*runningJob
	done chan struct{}
}

type runningJob struct {
	job    Job
	ticker *time.Ticker
	stop   chan struct{}
}

func New() *Scheduler {
	return &Scheduler{done: make(chan struct{})}
}

func (s *Scheduler) Add(job Job) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rj := &runningJob{job: job, stop: make(chan struct{})}
	s.jobs = append(s.jobs, rj)
}

func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, rj := range s.jobs {
		rj.ticker = time.NewTicker(rj.job.Interval)
		go func(r *runningJob) {
			for {
				select {
				case <-r.stop:
					return
				case <-r.ticker.C:
					r.job.Fn()
				}
			}
		}(rj)
	}
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rj := range s.jobs {
		if rj.ticker != nil {
			rj.ticker.Stop()
		}
		close(rj.stop)
	}
}

func (s *Scheduler) JobCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.jobs)
}
