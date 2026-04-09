package sla

import (
	"testing"
	"time"
)

// recordStatuses injects status records directly without time.Now() dependency.
func recordStatuses(t *Tracker, service string, statuses []bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	base := time.Now()
	for i, healthy := range statuses {
		rec := statusRecord{
			healthy:   healthy,
			timestamp: base.Add(time.Duration(i) * time.Second),
		}
		t.records[service] = append(t.records[service], rec)
	}
}

func TestNewDefaults(t *testing.T) {
	tr := New()
	if tr == nil {
		t.Fatal("New() returned nil")
	}
	if tr.maxRecords != 100000 {
		t.Errorf("expected maxRecords 100000, got %d", tr.maxRecords)
	}
	// Default target applied on computeSLA when no target set.
	sla := tr.computeSLA("nonexistent")
	if sla.Target != 99.9 {
		t.Errorf("expected default target 99.9, got %f", sla.Target)
	}
}

func TestRecordAndUptime(t *testing.T) {
	tr := New()
	statuses := make([]bool, 100)
	for i := 0; i < 90; i++ {
		statuses[i] = true
	}
	// last 10 unhealthy (already false by zero value)
	recordStatuses(tr, "api", statuses)

	uptime := tr.Uptime("api")
	if uptime < 89.0 || uptime > 91.0 {
		t.Errorf("expected ~90%% uptime, got %f", uptime)
	}
}

func TestSetTarget(t *testing.T) {
	tr := New()
	tr.SetTarget("api", 95.0)
	sla := tr.computeSLA("api")
	if sla.Target != 95.0 {
		t.Errorf("expected target 95.0, got %f", sla.Target)
	}
}

func TestReport(t *testing.T) {
	tr := New()
	// svc-a: 100% uptime
	statuses100 := make([]bool, 10)
	for i := range statuses100 {
		statuses100[i] = true
	}
	recordStatuses(tr, "svc-a", statuses100)

	// svc-b: 80% uptime
	statuses80 := make([]bool, 10)
	for i := 0; i < 8; i++ {
		statuses80[i] = true
	}
	recordStatuses(tr, "svc-b", statuses80)

	// svc-c: 50% uptime
	statuses50 := make([]bool, 10)
	for i := 0; i < 5; i++ {
		statuses50[i] = true
	}
	recordStatuses(tr, "svc-c", statuses50)

	report := tr.Report()
	if len(report) != 3 {
		t.Fatalf("expected 3 services in report, got %d", len(report))
	}
	// sorted ascending by uptime — worst first
	if report[0].UptimePercent > report[1].UptimePercent {
		t.Errorf("report not sorted ascending: %f > %f", report[0].UptimePercent, report[1].UptimePercent)
	}
	if report[1].UptimePercent > report[2].UptimePercent {
		t.Errorf("report not sorted ascending: %f > %f", report[1].UptimePercent, report[2].UptimePercent)
	}
	// worst should be ~50%
	if report[0].UptimePercent > 55.0 {
		t.Errorf("expected worst ~50%%, got %f", report[0].UptimePercent)
	}
}

func TestIsViolating(t *testing.T) {
	tr := New()
	tr.SetTarget("api", 99.9)
	// Record 95% uptime: 95 healthy, 5 unhealthy
	statuses := make([]bool, 100)
	for i := 0; i < 95; i++ {
		statuses[i] = true
	}
	recordStatuses(tr, "api", statuses)

	if !tr.IsViolating("api") {
		t.Error("expected IsViolating=true for 95% uptime with 99.9% target")
	}
}

func TestNotViolating(t *testing.T) {
	tr := New()
	tr.SetTarget("api", 90.0)
	// Record 95% uptime
	statuses := make([]bool, 100)
	for i := 0; i < 95; i++ {
		statuses[i] = true
	}
	recordStatuses(tr, "api", statuses)

	if tr.IsViolating("api") {
		t.Error("expected IsViolating=false for 95% uptime with 90% target")
	}
}

func TestWorst(t *testing.T) {
	tr := New()
	// svc-a: 90%
	s90 := make([]bool, 10)
	for i := 0; i < 9; i++ {
		s90[i] = true
	}
	recordStatuses(tr, "svc-a", s90)

	// svc-b: 60%
	s60 := make([]bool, 10)
	for i := 0; i < 6; i++ {
		s60[i] = true
	}
	recordStatuses(tr, "svc-b", s60)

	worst := tr.Worst()
	if worst == nil {
		t.Fatal("expected Worst() to return a service")
	}
	if worst.Name != "svc-b" {
		t.Errorf("expected worst=svc-b, got %s", worst.Name)
	}
	if worst.UptimePercent > 65.0 {
		t.Errorf("expected worst uptime ~60%%, got %f", worst.UptimePercent)
	}
}

func TestReset(t *testing.T) {
	tr := New()
	statuses := make([]bool, 5)
	for i := range statuses {
		statuses[i] = true
	}
	recordStatuses(tr, "api", statuses)

	tr.Reset("api")

	tr.mu.RLock()
	_, exists := tr.records["api"]
	tr.mu.RUnlock()
	if exists {
		t.Error("expected records cleared after Reset")
	}

	sla := tr.computeSLA("api")
	if sla.TotalChecks != 0 {
		t.Errorf("expected 0 total checks after reset, got %d", sla.TotalChecks)
	}
}

func TestAllHealthy(t *testing.T) {
	tr := New()
	statuses := make([]bool, 20)
	for i := range statuses {
		statuses[i] = true
	}
	recordStatuses(tr, "api", statuses)

	uptime := tr.Uptime("api")
	if uptime != 100.0 {
		t.Errorf("expected 100%% uptime, got %f", uptime)
	}
}

func TestAllUnhealthy(t *testing.T) {
	tr := New()
	statuses := make([]bool, 20) // all false
	recordStatuses(tr, "api", statuses)

	uptime := tr.Uptime("api")
	if uptime != 0.0 {
		t.Errorf("expected 0%% uptime, got %f", uptime)
	}
}
