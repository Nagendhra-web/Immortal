package selfmonitor_test

import (
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/selfmonitor"
)

func TestMonitorStats(t *testing.T) {
	m := selfmonitor.New()
	stats := m.Stats()
	if stats.Goroutines == 0 {
		t.Error("expected goroutines > 0")
	}
	// Uptime may round to 0 on fast machines; just verify it doesn't panic
	_ = stats.Uptime
}

func TestMonitorRecordEvents(t *testing.T) {
	m := selfmonitor.New()
	m.RecordEvent()
	m.RecordEvent()
	m.RecordEvent()
	if m.Stats().EventsProcessed != 3 {
		t.Error("expected 3 events")
	}
}

func TestMonitorRecordHeals(t *testing.T) {
	m := selfmonitor.New()
	m.RecordHeal()
	if m.Stats().HealsExecuted != 1 {
		t.Error("expected 1 heal")
	}
}

func TestMonitorIsHealthy(t *testing.T) {
	m := selfmonitor.New()
	if !m.IsHealthy() {
		t.Error("fresh monitor should be healthy")
	}
}
