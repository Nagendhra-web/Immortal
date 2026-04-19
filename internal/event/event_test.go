package event_test

import (
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/event"
)

func TestNewEvent(t *testing.T) {
	e := event.New(event.TypeError, event.SeverityCritical, "service crashed")

	if e.Type != event.TypeError {
		t.Errorf("expected type %s, got %s", event.TypeError, e.Type)
	}
	if e.Severity != event.SeverityCritical {
		t.Errorf("expected severity %s, got %s", event.SeverityCritical, e.Severity)
	}
	if e.Message != "service crashed" {
		t.Errorf("expected message 'service crashed', got '%s'", e.Message)
	}
	if e.ID == "" {
		t.Error("expected non-empty ID")
	}
	if e.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestEventWithMetadata(t *testing.T) {
	e := event.New(event.TypeMetric, event.SeverityWarning, "high cpu").
		WithSource("node-app").
		WithMeta("cpu_percent", 95.5).
		WithMeta("pid", 1234)

	if e.Source != "node-app" {
		t.Errorf("expected source 'node-app', got '%s'", e.Source)
	}
	if e.Meta["cpu_percent"] != 95.5 {
		t.Errorf("expected cpu_percent 95.5, got %v", e.Meta["cpu_percent"])
	}
	if e.Meta["pid"] != 1234 {
		t.Errorf("expected pid 1234, got %v", e.Meta["pid"])
	}
}

func TestSeverityOrdering(t *testing.T) {
	if event.SeverityCritical.Level() <= event.SeverityWarning.Level() {
		t.Error("critical should have higher level than warning")
	}
	if event.SeverityWarning.Level() <= event.SeverityInfo.Level() {
		t.Error("warning should have higher level than info")
	}
}
