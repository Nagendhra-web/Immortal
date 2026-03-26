package alert_test

import (
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/alert"
	"github.com/immortal-engine/immortal/internal/event"
)

func TestAlertFires(t *testing.T) {
	m := alert.NewManager()

	var received []alert.Alert
	m.AddChannel(&alert.CallbackChannel{Fn: func(a *alert.Alert) {
		received = append(received, *a)
	}})

	m.AddRule(alert.AlertRule{
		Name:  "crash-alert",
		Match: func(e *event.Event) bool { return e.Severity == event.SeverityCritical },
		Level: alert.LevelCritical,
		Title: "Service Crashed",
	})

	e := event.New(event.TypeError, event.SeverityCritical, "process died").WithSource("api")
	fired := m.Process(e)

	if len(fired) != 1 {
		t.Errorf("expected 1 alert, got %d", len(fired))
	}
	if len(received) != 1 {
		t.Errorf("expected 1 callback, got %d", len(received))
	}
}

func TestAlertCooldown(t *testing.T) {
	m := alert.NewManager()
	m.AddChannel(&alert.LogChannel{})
	m.AddRule(alert.AlertRule{
		Name:     "crash",
		Match:    func(e *event.Event) bool { return true },
		Level:    alert.LevelCritical,
		Title:    "Crash",
		Cooldown: time.Second,
	})

	e := event.New(event.TypeError, event.SeverityCritical, "crash")

	fired1 := m.Process(e)
	fired2 := m.Process(e) // Within cooldown

	if len(fired1) != 1 {
		t.Error("first alert should fire")
	}
	if len(fired2) != 0 {
		t.Error("second alert should be suppressed by cooldown")
	}
}

func TestAlertNoMatch(t *testing.T) {
	m := alert.NewManager()
	m.AddChannel(&alert.LogChannel{})
	m.AddRule(alert.AlertRule{
		Name:  "critical-only",
		Match: func(e *event.Event) bool { return e.Severity == event.SeverityCritical },
		Level: alert.LevelCritical,
		Title: "Critical",
	})

	e := event.New(event.TypeError, event.SeverityInfo, "all good")
	fired := m.Process(e)

	if len(fired) != 0 {
		t.Error("info event should not trigger critical alert")
	}
}

func TestAlertHistory(t *testing.T) {
	m := alert.NewManager()
	m.AddChannel(&alert.LogChannel{})
	m.AddRule(alert.AlertRule{
		Name:  "all",
		Match: func(e *event.Event) bool { return true },
		Level: alert.LevelInfo,
		Title: "Event",
	})

	m.Process(event.New(event.TypeError, event.SeverityError, "err1"))
	m.Process(event.New(event.TypeError, event.SeverityError, "err2"))

	if len(m.History()) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(m.History()))
	}
}
