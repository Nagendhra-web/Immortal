package chaos

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
)

type Fault struct {
	ID       string    `json:"id"`
	Type     string    `json:"type"`
	Target   string    `json:"target"`
	Duration time.Duration `json:"duration"`
	Severity string    `json:"severity"`
	Message  string    `json:"message"`
	Injected time.Time `json:"injected"`
	Active   bool      `json:"active"`
}

type Result struct {
	Fault      Fault         `json:"fault"`
	Detected   bool          `json:"detected"`
	DetectTime time.Duration `json:"detect_time"`
	Healed     bool          `json:"healed"`
	HealTime   time.Duration `json:"heal_time"`
	Score      float64       `json:"score"`
}

type Engine struct {
	mu        sync.RWMutex
	faults    []Fault
	results   []Result
	callback  func(*event.Event)
	idCounter uint64
}

func New(callback func(*event.Event)) *Engine {
	return &Engine{
		callback: callback,
	}
}

func (e *Engine) inject(faultType, target, message, severity string) *Fault {
	id := fmt.Sprintf("chaos-%d", atomic.AddUint64(&e.idCounter, 1))
	f := Fault{
		ID:       id,
		Type:     faultType,
		Target:   target,
		Severity: severity,
		Message:  message,
		Injected: time.Now(),
		Active:   true,
	}

	e.mu.Lock()
	e.faults = append(e.faults, f)
	e.mu.Unlock()

	sev := event.SeverityCritical
	if severity == "warning" {
		sev = event.SeverityWarning
	} else if severity == "error" {
		sev = event.SeverityError
	}

	ev := event.New(event.TypeError, sev, message).WithSource(target)
	if e.callback != nil {
		e.callback(ev)
	}

	return &f
}

func (e *Engine) InjectHTTPError(target string, statusCode int) *Fault {
	return e.inject("http_error", target, fmt.Sprintf("HTTP %d — chaos test", statusCode), "critical")
}

func (e *Engine) InjectProcessCrash(processName string) *Fault {
	return e.inject("process_crash", processName, fmt.Sprintf("process %s crashed — chaos test", processName), "critical")
}

func (e *Engine) InjectCPUSpike(percent float64) *Fault {
	return e.inject("cpu_spike", "system", fmt.Sprintf("CPU at %.0f%% — chaos test", percent), "warning")
}

func (e *Engine) InjectMemoryPressure(percent float64) *Fault {
	return e.inject("memory_pressure", "system", fmt.Sprintf("memory at %.0f%% — chaos test", percent), "warning")
}

func (e *Engine) InjectLatency(target string, latencyMs int) *Fault {
	return e.inject("latency", target, fmt.Sprintf("latency %dms — chaos test", latencyMs), "warning")
}

func (e *Engine) InjectCustom(faultType, target, message string, severity event.Severity) *Fault {
	return e.inject(faultType, target, message, string(severity))
}

func (e *Engine) RecordResult(faultID string, detected bool, detectTime time.Duration, healed bool, healTime time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var fault Fault
	for _, f := range e.faults {
		if f.ID == faultID {
			fault = f
			break
		}
	}

	score := 0.0
	if detected {
		score += 0.4
	}
	if healed {
		score += 0.6
	}

	e.results = append(e.results, Result{
		Fault:      fault,
		Detected:   detected,
		DetectTime: detectTime,
		Healed:     healed,
		HealTime:   healTime,
		Score:      score,
	})
}

func (e *Engine) Results() []Result {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Result, len(e.results))
	copy(out, e.results)
	return out
}

func (e *Engine) Score() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.results) == 0 {
		return 0
	}

	detected := 0
	healed := 0
	for _, r := range e.results {
		if r.Detected {
			detected++
		}
		if r.Healed {
			healed++
		}
	}

	total := float64(len(e.results))
	return (float64(detected)*0.4 + float64(healed)*0.6) / total
}

func (e *Engine) ActiveFaults() []Fault {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var active []Fault
	for _, f := range e.faults {
		if f.Active {
			active = append(active, f)
		}
	}
	return active
}

func (e *Engine) ClearFault(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.faults {
		if e.faults[i].ID == id {
			e.faults[i].Active = false
			break
		}
	}
}

func (e *Engine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.faults = nil
	e.results = nil
}

func (e *Engine) Report() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	detected := 0
	healed := 0
	for _, r := range e.results {
		if r.Detected {
			detected++
		}
		if r.Healed {
			healed++
		}
	}

	return map[string]interface{}{
		"total_faults": len(e.faults),
		"detected":     detected,
		"healed":       healed,
		"score":        e.Score(),
		"results":      e.results,
	}
}
