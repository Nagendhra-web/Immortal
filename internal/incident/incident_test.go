package incident_test

import (
	"strings"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/incident"
)

func TestNew(t *testing.T) {
	m := incident.New()
	if m == nil {
		t.Fatal("expected manager")
	}
}

func TestOpen(t *testing.T) {
	m := incident.New()
	r := m.Open("API Down", "critical")
	if r.ID != "INC-1" {
		t.Errorf("expected INC-1, got %s", r.ID)
	}
	if r.Status != "open" {
		t.Errorf("expected open, got %s", r.Status)
	}
	if r.Title != "API Down" {
		t.Errorf("expected API Down, got %s", r.Title)
	}
}

func TestAddEvent(t *testing.T) {
	m := incident.New()
	r := m.Open("API Down", "critical")
	m.AddEvent(r.ID, "api-server", "critical", "HTTP 500")
	m.AddEvent(r.ID, "database", "error", "connection timeout")

	got := m.Get(r.ID)
	if len(got.Timeline) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got.Timeline))
	}
}

func TestSetRootCause(t *testing.T) {
	m := incident.New()
	r := m.Open("API Down", "critical")
	m.SetRootCause(r.ID, "disk full on database server")

	got := m.Get(r.ID)
	if got.RootCause != "disk full on database server" {
		t.Errorf("expected root cause set, got %s", got.RootCause)
	}
}

func TestAddAffectedServiceDedup(t *testing.T) {
	m := incident.New()
	r := m.Open("API Down", "critical")
	m.AddAffectedService(r.ID, "api-server")
	m.AddAffectedService(r.ID, "api-server") // duplicate
	m.AddAffectedService(r.ID, "database")

	got := m.Get(r.ID)
	if len(got.AffectedServices) != 2 {
		t.Errorf("expected 2 services (deduped), got %d", len(got.AffectedServices))
	}
}

func TestAddHealingAction(t *testing.T) {
	m := incident.New()
	r := m.Open("API Down", "critical")
	m.AddHealingAction(r.ID, "restarted api-server")
	m.AddHealingAction(r.ID, "cleared disk cache")

	got := m.Get(r.ID)
	if len(got.HealingActions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(got.HealingActions))
	}
}

func TestResolve(t *testing.T) {
	m := incident.New()
	r := m.Open("API Down", "critical")
	time.Sleep(5 * time.Millisecond)
	m.Resolve(r.ID, "Fixed by restarting the service")

	got := m.Get(r.ID)
	if got.Status != "resolved" {
		t.Errorf("expected resolved, got %s", got.Status)
	}
	if got.Duration <= 0 {
		t.Error("expected positive duration")
	}
	if got.Summary != "Fixed by restarting the service" {
		t.Errorf("expected summary set, got %s", got.Summary)
	}
}

func TestGet(t *testing.T) {
	m := incident.New()
	r := m.Open("Test", "info")
	got := m.Get(r.ID)
	if got == nil {
		t.Fatal("expected report")
	}

	none := m.Get("nonexistent")
	if none != nil {
		t.Error("expected nil for nonexistent")
	}
}

func TestActive(t *testing.T) {
	m := incident.New()
	r1 := m.Open("Incident 1", "critical")
	m.Open("Incident 2", "warning")
	m.Resolve(r1.ID, "fixed")

	active := m.Active()
	if len(active) != 1 {
		t.Errorf("expected 1 active, got %d", len(active))
	}
}

func TestAll(t *testing.T) {
	m := incident.New()
	m.Open("Inc 1", "critical")
	m.Open("Inc 2", "warning")
	m.Open("Inc 3", "info")

	all := m.All()
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}
}

func TestSearch(t *testing.T) {
	m := incident.New()
	r := m.Open("Database Outage", "critical")
	m.Open("API Latency Spike", "warning")
	m.Resolve(r.ID, "disk was full")

	results := m.Search("database")
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	results = m.Search("disk")
	if len(results) != 1 {
		t.Errorf("expected 1 result for summary search, got %d", len(results))
	}
}

func TestGenerateMarkdown(t *testing.T) {
	m := incident.New()
	r := m.Open("API Down", "critical")
	m.AddEvent(r.ID, "api-server", "critical", "HTTP 500")
	m.SetRootCause(r.ID, "disk full")
	m.AddAffectedService(r.ID, "api-server")
	m.AddHealingAction(r.ID, "restarted service")
	m.Resolve(r.ID, "Service restored")

	md := m.GenerateMarkdown(r.ID)
	if !strings.Contains(md, "# Incident Report: API Down") {
		t.Error("expected title in markdown")
	}
	if !strings.Contains(md, "HTTP 500") {
		t.Error("expected timeline event in markdown")
	}
	if !strings.Contains(md, "disk full") {
		t.Error("expected root cause in markdown")
	}
	if !strings.Contains(md, "api-server") {
		t.Error("expected affected service in markdown")
	}
}

func TestStats(t *testing.T) {
	m := incident.New()
	r1 := m.Open("Inc 1", "critical")
	m.Open("Inc 2", "warning")
	m.Resolve(r1.ID, "fixed")

	stats := m.Stats()
	if stats["total"].(int) != 2 {
		t.Errorf("expected total 2, got %v", stats["total"])
	}
	if stats["open"].(int) != 1 {
		t.Errorf("expected 1 open, got %v", stats["open"])
	}
	if stats["resolved"].(int) != 1 {
		t.Errorf("expected 1 resolved, got %v", stats["resolved"])
	}
}
