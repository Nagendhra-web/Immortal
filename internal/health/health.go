package health

import (
	"sync"
	"time"
)

type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
	StatusUnknown   Status = "unknown"
)

type ServiceHealth struct {
	Name      string                 `json:"name"`
	Status    Status                 `json:"status"`
	Message   string                 `json:"message"`
	LastCheck time.Time              `json:"last_check"`
	Uptime    time.Duration          `json:"uptime"`
	StartedAt time.Time             `json:"started_at"`
	Checks    int64                  `json:"checks"`
	Failures  int64                  `json:"failures"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
}

type Registry struct {
	mu       sync.RWMutex
	services map[string]*ServiceHealth
}

func NewRegistry() *Registry {
	return &Registry{services: make(map[string]*ServiceHealth)}
}

func (r *Registry) Register(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services[name] = &ServiceHealth{
		Name:      name,
		Status:    StatusUnknown,
		StartedAt: time.Now(),
		Meta:      make(map[string]interface{}),
	}
}

func (r *Registry) Update(name string, status Status, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	svc, ok := r.services[name]
	if !ok {
		return
	}
	svc.Status = status
	svc.Message = message
	svc.LastCheck = time.Now()
	svc.Checks++
	if status == StatusUnhealthy {
		svc.Failures++
	}
	svc.Uptime = time.Since(svc.StartedAt)
}

func (r *Registry) Get(name string) *ServiceHealth {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if svc, ok := r.services[name]; ok {
		copy := *svc
		return &copy
	}
	return nil
}

func (r *Registry) All() []*ServiceHealth {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*ServiceHealth
	for _, svc := range r.services {
		copy := *svc
		result = append(result, &copy)
	}
	return result
}

func (r *Registry) OverallStatus() Status {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.services) == 0 {
		return StatusUnknown
	}
	hasUnhealthy := false
	hasDegraded := false
	for _, svc := range r.services {
		switch svc.Status {
		case StatusUnhealthy:
			hasUnhealthy = true
		case StatusDegraded:
			hasDegraded = true
		}
	}
	if hasUnhealthy {
		return StatusUnhealthy
	}
	if hasDegraded {
		return StatusDegraded
	}
	return StatusHealthy
}

func (r *Registry) UptimePercent(name string) float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	svc, ok := r.services[name]
	if !ok || svc.Checks == 0 {
		return 100.0
	}
	return float64(svc.Checks-svc.Failures) / float64(svc.Checks) * 100.0
}
