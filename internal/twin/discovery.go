package twin

import (
	"sync/atomic"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/event"
)

// ObserveEvent projects a generic event.Event into twin state.
// Unlike metric-only observation, this accepts error/log/trace/health events too.
//
// Rules:
//   - ev.Source is the service name (auto-register if unknown).
//   - TypeMetric events: update CPU/Memory/Latency/ErrorRate from meta fields.
//   - TypeError / TypeLog with severity >= Critical: Healthy = false, increment ErrorCount.
//   - TypeHealth: Healthy = severity < Warning, update LastHealthCheck.
//   - Dependencies updated from ev.Meta["depends_on"] if present ([]string or []interface{}).
func (t *Twin) ObserveEvent(ev *event.Event) {
	if ev == nil || ev.Source == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	s, exists := t.states[ev.Source]
	if !exists {
		s = State{Service: ev.Source, Healthy: true}
	}

	now := time.Now()
	s.LastSeen = now

	switch ev.Type {
	case event.TypeMetric:
		if v, ok := metaFloat(ev.Meta, "cpu"); ok {
			s.CPU = v
		}
		if v, ok := metaFloat(ev.Meta, "memory"); ok {
			s.Memory = v
		}
		if v, ok := metaFloat(ev.Meta, "latency"); ok {
			s.Latency = v
		}
		if v, ok := metaFloat(ev.Meta, "error_rate"); ok {
			s.ErrorRate = v
		}
		if v, ok := metaInt(ev.Meta, "replicas"); ok {
			s.Replicas = v
		}

	case event.TypeError, event.TypeLog:
		if ev.Severity.Level() >= event.SeverityCritical.Level() {
			s.Healthy = false
			atomic.AddInt64(&s.ErrorCount, 1)
		}

	case event.TypeHealth:
		s.LastHealthCheck = now
		s.Healthy = ev.Severity.Level() < event.SeverityWarning.Level()
	}

	// Update dependencies from meta if present.
	if deps, ok := metaStringSlice(ev.Meta, "depends_on"); ok {
		s.Dependencies = deps
	}

	t.states[ev.Source] = s
}

// AutoDiscover returns the list of services the twin has inferred from events.
func (t *Twin) AutoDiscover() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	svcs := make([]string, 0, len(t.states))
	for k := range t.states {
		svcs = append(svcs, k)
	}
	return svcs
}

// metaFloat extracts a float64 from a meta map by key.
func metaFloat(meta map[string]interface{}, key string) (float64, bool) {
	v, ok := meta[key]
	if !ok {
		return 0, false
	}
	switch f := v.(type) {
	case float64:
		return f, true
	case float32:
		return float64(f), true
	case int:
		return float64(f), true
	case int64:
		return float64(f), true
	}
	return 0, false
}

// metaInt extracts an int from a meta map by key.
func metaInt(meta map[string]interface{}, key string) (int, bool) {
	v, ok := meta[key]
	if !ok {
		return 0, false
	}
	switch i := v.(type) {
	case int:
		return i, true
	case float64:
		return int(i), true
	case int64:
		return int(i), true
	}
	return 0, false
}

// metaStringSlice extracts a []string from a meta map by key.
// Accepts both []string and []interface{} (JSON-decoded arrays).
func metaStringSlice(meta map[string]interface{}, key string) ([]string, bool) {
	v, ok := meta[key]
	if !ok {
		return nil, false
	}
	switch s := v.(type) {
	case []string:
		out := make([]string, len(s))
		copy(out, s)
		return out, true
	case []interface{}:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out, true
	}
	return nil, false
}
