package sla

import (
	"sort"
	"sync"
	"time"
)

type statusRecord struct {
	healthy   bool
	timestamp time.Time
}

// ServiceSLA holds computed SLA metrics for a single service.
type ServiceSLA struct {
	Name         string        `json:"name"`
	UptimePercent float64      `json:"uptime_percent"`
	Downtime     time.Duration `json:"downtime"`
	Uptime       time.Duration `json:"uptime"`
	TotalChecks  int           `json:"total_checks"`
	FailedChecks int           `json:"failed_checks"`
	LastStatus   string        `json:"last_status"`
	LastCheck    time.Time     `json:"last_check"`
	Violations   int           `json:"violations"`
	Target       float64       `json:"target"`
}

// Tracker records health status events per service and computes SLA metrics.
type Tracker struct {
	mu         sync.RWMutex
	records    map[string][]statusRecord
	targets    map[string]float64
	maxRecords int
}

// New returns a Tracker with a default cap of 100 000 records and a 99.9% default target.
func New() *Tracker {
	return &Tracker{
		records:    make(map[string][]statusRecord),
		targets:    make(map[string]float64),
		maxRecords: 100000,
	}
}

// SetTarget sets the SLA target uptime percentage for service (e.g. 99.9).
func (t *Tracker) SetTarget(service string, percent float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.targets[service] = percent
}

// RecordStatus appends a health status record for service using the current time.
func (t *Tracker) RecordStatus(service string, healthy bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	recs := t.records[service]
	recs = append(recs, statusRecord{healthy: healthy, timestamp: time.Now()})
	if len(recs) > t.maxRecords {
		recs = recs[len(recs)-t.maxRecords:]
	}
	t.records[service] = recs
}

// Report returns ServiceSLA for every known service, sorted by UptimePercent ascending (worst first).
func (t *Tracker) Report() []ServiceSLA {
	t.mu.RLock()
	services := make([]string, 0, len(t.records))
	for s := range t.records {
		services = append(services, s)
	}
	t.mu.RUnlock()

	out := make([]ServiceSLA, 0, len(services))
	for _, s := range services {
		out = append(out, t.computeSLA(s))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UptimePercent < out[j].UptimePercent
	})
	return out
}

// Uptime returns the uptime percentage for service.
func (t *Tracker) Uptime(service string) float64 {
	sla := t.computeSLA(service)
	return sla.UptimePercent
}

// IsViolating returns true if service's current uptime is below its target.
func (t *Tracker) IsViolating(service string) bool {
	sla := t.computeSLA(service)
	return sla.UptimePercent < sla.Target
}

// Worst returns the ServiceSLA with the lowest uptime, or nil if no records exist.
func (t *Tracker) Worst() *ServiceSLA {
	report := t.Report()
	if len(report) == 0 {
		return nil
	}
	w := report[0]
	return &w
}

// Reset clears all records for service.
func (t *Tracker) Reset(service string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.records, service)
}

// computeSLA calculates the ServiceSLA for a single service.
func (t *Tracker) computeSLA(service string) ServiceSLA {
	t.mu.RLock()
	recs := t.records[service]
	target, hasTarget := t.targets[service]
	t.mu.RUnlock()

	const defaultTarget = 99.9
	if !hasTarget {
		target = defaultTarget
	}

	sla := ServiceSLA{
		Name:   service,
		Target: target,
	}

	if len(recs) == 0 {
		return sla
	}

	total := len(recs)
	failed := 0
	for _, r := range recs {
		if !r.healthy {
			failed++
		}
	}

	sla.TotalChecks = total
	sla.FailedChecks = failed

	if total > 0 {
		sla.UptimePercent = float64(total-failed) / float64(total) * 100
	}

	// Compute downtime as sum of intervals between consecutive unhealthy records.
	var downtime time.Duration
	for i := 1; i < len(recs); i++ {
		if !recs[i].healthy && !recs[i-1].healthy {
			downtime += recs[i].timestamp.Sub(recs[i-1].timestamp)
		}
	}
	sla.Downtime = downtime

	// Compute uptime intervals (consecutive healthy pairs).
	var uptime time.Duration
	for i := 1; i < len(recs); i++ {
		if recs[i].healthy && recs[i-1].healthy {
			uptime += recs[i].timestamp.Sub(recs[i-1].timestamp)
		}
	}
	sla.Uptime = uptime

	last := recs[len(recs)-1]
	sla.LastCheck = last.timestamp
	if last.healthy {
		sla.LastStatus = "healthy"
	} else {
		sla.LastStatus = "unhealthy"
	}

	// Count violations: windows where rolling uptime dropped below target.
	// Use a simple sliding window of 10 records to detect violation periods.
	windowSize := 10
	if windowSize > total {
		windowSize = total
	}
	violations := 0
	prevViolating := false
	for i := windowSize; i <= total; i++ {
		window := recs[i-windowSize : i]
		wFailed := 0
		for _, r := range window {
			if !r.healthy {
				wFailed++
			}
		}
		wUptime := float64(windowSize-wFailed) / float64(windowSize) * 100
		violating := wUptime < target
		if violating && !prevViolating {
			violations++
		}
		prevViolating = violating
	}
	sla.Violations = violations

	return sla
}
