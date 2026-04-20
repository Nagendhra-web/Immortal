package otel

import (
	"encoding/json"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/evolve"
)

// Signals maintains rolling per-service statistics computed from OTLP
// spans. It is designed to be plugged into the Receiver as an additional
// consumer: register a trace sink that calls Ingest on every decoded
// tracesPayload.
//
// The Aggregator keeps at most Window duration of history per service.
// Calls to SignalBag() return a snapshot of the current state suitable
// for feeding into evolve.Advisor.Analyze().
type Signals struct {
	mu     sync.RWMutex
	window time.Duration
	// services keyed by service.name attribute on the resource.
	services   map[string]*serviceStats
	// dependency graph: parent->child edges observed in the trace.
	parents    map[string]map[string]struct{} // service -> set of callers
	children   map[string]map[string]struct{} // service -> set of callees
	nowFn      func() time.Time
	maxSamples int
}

type serviceStats struct {
	samples     []spanSample // ring of recent durations
	errors      []eventTs    // per-error timestamps
	retries     []eventTs    // per-retry timestamps
	lastSpanAt  time.Time
}

type spanSample struct {
	ts      time.Time
	durMS   float64
	errored bool
}

type eventTs struct {
	ts time.Time
}

// NewSignals returns an aggregator retaining the last `window` of spans
// per service. A reasonable default is 5 minutes.
func NewSignals(window time.Duration) *Signals {
	if window <= 0 {
		window = 5 * time.Minute
	}
	return &Signals{
		window:     window,
		services:   make(map[string]*serviceStats),
		parents:    make(map[string]map[string]struct{}),
		children:   make(map[string]map[string]struct{}),
		nowFn:      time.Now,
		maxSamples: 2048,
	}
}

// Ingest decodes an OTLP traces JSON payload and updates the statistics.
// It is safe to call concurrently; a mutex guards the service map.
func (s *Signals) Ingest(payload []byte) error {
	var p tracesPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowFn()
	for _, rs := range p.ResourceSpans {
		service := attrString(rs.Resource.Attributes, "service.name")
		if service == "" {
			continue
		}
		stats, ok := s.services[service]
		if !ok {
			stats = &serviceStats{}
			s.services[service] = stats
		}
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				dur := durationMs(span.StartTimeUnixNano, span.EndTimeUnixNano)
				errored := span.Status.Code == 2 // STATUS_CODE_ERROR
				ts := now
				stats.samples = append(stats.samples, spanSample{ts: ts, durMS: dur, errored: errored})
				if errored {
					stats.errors = append(stats.errors, eventTs{ts})
				}
				if isRetry(span.Attributes) {
					stats.retries = append(stats.retries, eventTs{ts})
				}
				stats.lastSpanAt = ts

				// Dependency inference via parent span attribute. We do not
				// do full trace reassembly here; we rely on
				// `rpc.service` / `peer.service` attributes to identify the
				// downstream.
				if peer := attrString(span.Attributes, "peer.service"); peer != "" {
					addEdge(s.children, service, peer)
					addEdge(s.parents, peer, service)
				}
			}
		}
		// Trim to maxSamples / window.
		s.trim(stats, now)
	}
	return nil
}

// SignalBag returns an evolve.SignalBag computed from the current state.
// The result is a snapshot; subsequent Ingest calls do not mutate it.
func (s *Signals) SignalBag() evolve.SignalBag {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bag := evolve.SignalBag{
		LatencyP99:      map[string]float64{},
		LatencyCoeffVar: map[string]float64{},
		ErrorRate:       map[string]float64{},
		RetryRate:       map[string]float64{},
		DependentCount:  map[string]int{},
		DependencyCount: map[string]int{},
	}
	for svc, stats := range s.services {
		if len(stats.samples) == 0 {
			continue
		}
		durations := make([]float64, 0, len(stats.samples))
		for _, s := range stats.samples {
			durations = append(durations, s.durMS)
		}
		bag.LatencyP99[svc] = percentile(durations, 0.99)
		bag.LatencyCoeffVar[svc] = coeffVar(durations)
		total := float64(len(stats.samples))
		if total > 0 {
			bag.ErrorRate[svc] = float64(len(stats.errors)) / total
			bag.RetryRate[svc] = float64(len(stats.retries)) / total
		}
	}
	for svc, callers := range s.parents {
		bag.DependentCount[svc] = len(callers)
	}
	for svc, callees := range s.children {
		bag.DependencyCount[svc] = len(callees)
	}
	return bag
}

// Services returns the names of services with at least one observed span.
func (s *Signals) Services() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.services))
	for k := range s.services {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ── helpers ───────────────────────────────────────────────────────────────

func (s *Signals) trim(stats *serviceStats, now time.Time) {
	cutoff := now.Add(-s.window)
	stats.samples = trimSamples(stats.samples, cutoff, s.maxSamples)
	stats.errors = trimEvents(stats.errors, cutoff, s.maxSamples)
	stats.retries = trimEvents(stats.retries, cutoff, s.maxSamples)
}

func trimSamples(in []spanSample, cutoff time.Time, max int) []spanSample {
	// Drop anything older than cutoff.
	i := 0
	for i < len(in) && in[i].ts.Before(cutoff) {
		i++
	}
	in = in[i:]
	if len(in) > max {
		in = in[len(in)-max:]
	}
	return in
}

func trimEvents(in []eventTs, cutoff time.Time, max int) []eventTs {
	i := 0
	for i < len(in) && in[i].ts.Before(cutoff) {
		i++
	}
	in = in[i:]
	if len(in) > max {
		in = in[len(in)-max:]
	}
	return in
}

func percentile(durations []float64, p float64) float64 {
	if len(durations) == 0 {
		return 0
	}
	cp := append([]float64(nil), durations...)
	sort.Float64s(cp)
	idx := int(float64(len(cp)-1) * p)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}

func coeffVar(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum, sumSq float64
	for _, v := range vals {
		sum += v
		sumSq += v * v
	}
	n := float64(len(vals))
	mean := sum / n
	variance := (sumSq / n) - (mean * mean)
	if variance < 0 {
		variance = 0
	}
	if mean == 0 {
		return 0
	}
	return sqrt(variance) / mean
}

// sqrt avoids importing math for a single call; uses fast Newton iterations.
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 8; i++ {
		z = (z + x/z) / 2
	}
	return z
}

func addEdge(m map[string]map[string]struct{}, key, value string) {
	set, ok := m[key]
	if !ok {
		set = make(map[string]struct{})
		m[key] = set
	}
	set[value] = struct{}{}
}

func attrString(attrs []otlpKeyValue, key string) string {
	for _, a := range attrs {
		if a.Key != key || a.Value == nil {
			continue
		}
		if v, ok := a.Value["stringValue"].(string); ok {
			return v
		}
	}
	return ""
}

// durationMs parses the OTLP nano-timestamps and returns the span
// duration in milliseconds. If either timestamp is missing or invalid,
// returns zero (treated as a zero-length span, counted but with 0 ms).
func durationMs(startNs, endNs string) float64 {
	start, err1 := strconv.ParseInt(startNs, 10, 64)
	end, err2 := strconv.ParseInt(endNs, 10, 64)
	if err1 != nil || err2 != nil || end < start {
		return 0
	}
	return float64(end-start) / 1_000_000.0
}

// isRetry inspects span attributes for signals that the span is a retry.
// Convention: either `http.retry_count` attribute > 0, or a boolean
// `retry` attribute, or `rpc.retry` > 0. This is a heuristic but covers
// the major OTel semantic-conventions in the wild.
func isRetry(attrs []otlpKeyValue) bool {
	for _, a := range attrs {
		if a.Value == nil {
			continue
		}
		switch a.Key {
		case "http.retry_count", "rpc.retry":
			if v, ok := a.Value["intValue"].(string); ok && v != "" && v != "0" {
				return true
			}
			if v, ok := a.Value["asInt"].(string); ok && v != "" && v != "0" {
				return true
			}
			if v, ok := a.Value["asDouble"].(float64); ok && v > 0 {
				return true
			}
		case "retry":
			if v, ok := a.Value["boolValue"].(bool); ok && v {
				return true
			}
		}
	}
	return false
}
