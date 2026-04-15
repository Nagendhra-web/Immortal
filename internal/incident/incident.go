package incident

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Event struct {
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
	Severity string    `json:"severity"`
	Message  string    `json:"message"`
}

type Report struct {
	ID               string        `json:"id"`
	Title            string        `json:"title"`
	Status           string        `json:"status"`
	Severity         string        `json:"severity"`
	StartTime        time.Time     `json:"start_time"`
	EndTime          time.Time     `json:"end_time"`
	Duration         time.Duration `json:"duration"`
	Timeline         []Event       `json:"timeline"`
	RootCause        string        `json:"root_cause"`
	AffectedServices []string      `json:"affected_services"`
	HealingActions   []string      `json:"healing_actions"`
	Impact           string        `json:"impact"`
	Summary          string        `json:"summary"`
}

type Manager struct {
	mu        sync.RWMutex
	incidents map[string]*Report
	idCounter uint64
}

func New() *Manager {
	return &Manager{
		incidents: make(map[string]*Report),
	}
}

func (m *Manager) Open(title, severity string) *Report {
	id := fmt.Sprintf("INC-%d", atomic.AddUint64(&m.idCounter, 1))
	r := &Report{
		ID:        id,
		Title:     title,
		Status:    "open",
		Severity:  severity,
		StartTime: time.Now(),
	}

	m.mu.Lock()
	m.incidents[id] = r
	m.mu.Unlock()

	return r
}

func (m *Manager) AddEvent(incidentID, source, severity, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.incidents[incidentID]
	if !ok {
		return
	}
	r.Timeline = append(r.Timeline, Event{
		Timestamp: time.Now(),
		Source:    source,
		Severity: severity,
		Message:  message,
	})
}

func (m *Manager) SetRootCause(incidentID, cause string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if r, ok := m.incidents[incidentID]; ok {
		r.RootCause = cause
	}
}

func (m *Manager) AddAffectedService(incidentID, service string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.incidents[incidentID]
	if !ok {
		return
	}
	for _, s := range r.AffectedServices {
		if s == service {
			return
		}
	}
	r.AffectedServices = append(r.AffectedServices, service)
}

func (m *Manager) AddHealingAction(incidentID, action string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if r, ok := m.incidents[incidentID]; ok {
		r.HealingActions = append(r.HealingActions, action)
	}
}

func (m *Manager) Resolve(incidentID, summary string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.incidents[incidentID]
	if !ok {
		return
	}
	r.Status = "resolved"
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime)
	r.Summary = summary
}

func (m *Manager) Get(incidentID string) *Report {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r, ok := m.incidents[incidentID]
	if !ok {
		return nil
	}
	copy := *r
	return &copy
}

func (m *Manager) Active() []Report {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var active []Report
	for _, r := range m.incidents {
		if r.Status == "open" || r.Status == "investigating" {
			active = append(active, *r)
		}
	}
	return active
}

func (m *Manager) All() []Report {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var all []Report
	for _, r := range m.incidents {
		all = append(all, *r)
	}
	// newest first
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	return all
}

func (m *Manager) Search(query string) []Report {
	m.mu.RLock()
	defer m.mu.RUnlock()

	q := strings.ToLower(query)
	var results []Report
	for _, r := range m.incidents {
		if strings.Contains(strings.ToLower(r.Title), q) ||
			strings.Contains(strings.ToLower(r.Summary), q) ||
			strings.Contains(strings.ToLower(r.RootCause), q) {
			results = append(results, *r)
		}
	}
	return results
}

func (m *Manager) GenerateMarkdown(incidentID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	r, ok := m.incidents[incidentID]
	if !ok {
		return ""
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Incident Report: %s\n\n", r.Title))
	b.WriteString(fmt.Sprintf("**ID:** %s\n", r.ID))
	b.WriteString(fmt.Sprintf("**Severity:** %s\n", r.Severity))
	b.WriteString(fmt.Sprintf("**Status:** %s\n", r.Status))
	b.WriteString(fmt.Sprintf("**Started:** %s\n", r.StartTime.Format("2006-01-02 15:04:05")))
	if !r.EndTime.IsZero() {
		b.WriteString(fmt.Sprintf("**Resolved:** %s\n", r.EndTime.Format("2006-01-02 15:04:05")))
		b.WriteString(fmt.Sprintf("**Duration:** %s\n", r.Duration.Round(time.Second)))
	}

	if len(r.Timeline) > 0 {
		b.WriteString("\n## Timeline\n\n")
		for _, e := range r.Timeline {
			b.WriteString(fmt.Sprintf("- **%s** [%s] %s: %s\n", e.Timestamp.Format("15:04:05"), e.Severity, e.Source, e.Message))
		}
	}

	if r.RootCause != "" {
		b.WriteString(fmt.Sprintf("\n## Root Cause\n\n%s\n", r.RootCause))
	}

	if len(r.AffectedServices) > 0 {
		b.WriteString("\n## Affected Services\n\n")
		for _, s := range r.AffectedServices {
			b.WriteString(fmt.Sprintf("- %s\n", s))
		}
	}

	if len(r.HealingActions) > 0 {
		b.WriteString("\n## Healing Actions Taken\n\n")
		for _, a := range r.HealingActions {
			b.WriteString(fmt.Sprintf("- %s\n", a))
		}
	}

	if r.Summary != "" {
		b.WriteString(fmt.Sprintf("\n## Summary\n\n%s\n", r.Summary))
	}

	return b.String()
}

func (m *Manager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	open := 0
	resolved := 0
	var totalDuration time.Duration
	for _, r := range m.incidents {
		if r.Status == "resolved" {
			resolved++
			totalDuration += r.Duration
		} else {
			open++
		}
	}

	meanDuration := time.Duration(0)
	if resolved > 0 {
		meanDuration = totalDuration / time.Duration(resolved)
	}

	return map[string]interface{}{
		"total":         len(m.incidents),
		"open":          open,
		"resolved":      resolved,
		"mean_duration": meanDuration.String(),
	}
}
