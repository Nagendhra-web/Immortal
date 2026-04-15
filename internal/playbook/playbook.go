package playbook

import (
	"fmt"
	"sync"
	"time"
)

type Step struct {
	Name      string
	Action    func() error
	Rollback  func() error
	Condition func() bool
	Timeout   time.Duration
	Retries   int
}

type StepResult struct {
	Name     string        `json:"name"`
	Status   string        `json:"status"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error"`
	Attempt  int           `json:"attempt"`
}

type Playbook struct {
	Name  string
	Steps []Step
}

type Execution struct {
	PlaybookName string        `json:"playbook_name"`
	Status       string        `json:"status"`
	StartTime    time.Time     `json:"start_time"`
	EndTime      time.Time     `json:"end_time"`
	Duration     time.Duration `json:"duration"`
	Results      []StepResult  `json:"results"`
}

type Runner struct {
	mu         sync.RWMutex
	playbooks  map[string]*Playbook
	history    []Execution
	maxHistory int
}

func New() *Runner {
	return &Runner{
		playbooks:  make(map[string]*Playbook),
		maxHistory: 1000,
	}
}

func (r *Runner) Register(name string, steps []Step) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.playbooks[name] = &Playbook{
		Name:  name,
		Steps: steps,
	}
}

func (r *Runner) Run(name string) (*Execution, error) {
	r.mu.RLock()
	pb, ok := r.playbooks[name]
	if !ok {
		r.mu.RUnlock()
		return nil, fmt.Errorf("playbook %q not found", name)
	}
	// Copy steps to avoid holding lock during execution
	steps := make([]Step, len(pb.Steps))
	copy(steps, pb.Steps)
	r.mu.RUnlock()

	exec := &Execution{
		PlaybookName: name,
		Status:       "running",
		StartTime:    time.Now(),
	}

	var completedSteps []int

	for i, step := range steps {
		// Check condition
		if step.Condition != nil && !step.Condition() {
			exec.Results = append(exec.Results, StepResult{
				Name:   step.Name,
				Status: "skipped",
			})
			continue
		}

		retries := step.Retries
		if retries <= 0 {
			retries = 1
		}

		var lastErr error
		succeeded := false
		start := time.Now()

		for attempt := 1; attempt <= retries; attempt++ {
			err := step.Action()
			if err == nil {
				exec.Results = append(exec.Results, StepResult{
					Name:     step.Name,
					Status:   "success",
					Duration: time.Since(start),
					Attempt:  attempt,
				})
				completedSteps = append(completedSteps, i)
				succeeded = true
				break
			}
			lastErr = err
		}

		if !succeeded {
			exec.Results = append(exec.Results, StepResult{
				Name:     step.Name,
				Status:   "failed",
				Duration: time.Since(start),
				Error:    lastErr.Error(),
				Attempt:  retries,
			})

			// Rollback completed steps in reverse order
			for j := len(completedSteps) - 1; j >= 0; j-- {
				idx := completedSteps[j]
				if steps[idx].Rollback != nil {
					rbErr := steps[idx].Rollback()
					// Update status of rolled-back step
					for k := range exec.Results {
						if exec.Results[k].Name == steps[idx].Name && exec.Results[k].Status == "success" {
							exec.Results[k].Status = "rolled_back"
							if rbErr != nil {
								exec.Results[k].Error = "rollback error: " + rbErr.Error()
							}
							break
						}
					}
				}
			}

			exec.Status = "failed"
			exec.EndTime = time.Now()
			exec.Duration = exec.EndTime.Sub(exec.StartTime)

			r.mu.Lock()
			r.history = append(r.history, *exec)
			if len(r.history) > r.maxHistory {
				r.history = r.history[len(r.history)-r.maxHistory:]
			}
			r.mu.Unlock()

			return exec, lastErr
		}
	}

	exec.Status = "success"
	exec.EndTime = time.Now()
	exec.Duration = exec.EndTime.Sub(exec.StartTime)

	r.mu.Lock()
	r.history = append(r.history, *exec)
	if len(r.history) > r.maxHistory {
		r.history = r.history[len(r.history)-r.maxHistory:]
	}
	r.mu.Unlock()

	return exec, nil
}

func (r *Runner) DryRun(name string) (*Execution, error) {
	r.mu.RLock()
	pb, ok := r.playbooks[name]
	if !ok {
		r.mu.RUnlock()
		return nil, fmt.Errorf("playbook %q not found", name)
	}
	steps := make([]Step, len(pb.Steps))
	copy(steps, pb.Steps)
	r.mu.RUnlock()

	exec := &Execution{
		PlaybookName: name,
		Status:       "dry_run",
		StartTime:    time.Now(),
	}

	for _, step := range steps {
		status := "success"
		if step.Condition != nil && !step.Condition() {
			status = "skipped"
		}
		exec.Results = append(exec.Results, StepResult{
			Name:   step.Name,
			Status: status,
		})
	}

	exec.EndTime = time.Now()
	exec.Duration = exec.EndTime.Sub(exec.StartTime)
	return exec, nil
}

func (r *Runner) History() []Execution {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Execution, len(r.history))
	copy(out, r.history)
	return out
}

func (r *Runner) Get(name string) *Playbook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.playbooks[name]
}

func (r *Runner) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for name := range r.playbooks {
		names = append(names, name)
	}
	return names
}

func (r *Runner) LastExecution(name string) *Execution {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := len(r.history) - 1; i >= 0; i-- {
		if r.history[i].PlaybookName == name {
			e := r.history[i]
			return &e
		}
	}
	return nil
}
