package storage_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/storage"
)

func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestStoreAndRetrieve(t *testing.T) {
	s := newTestStore(t)

	e := event.New(event.TypeError, event.SeverityCritical, "service crashed").
		WithSource("api-gateway")

	if err := s.Save(e); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	s.Flush()

	results, err := s.Query(storage.Query{Type: event.TypeError})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	got := results[0]
	if got.ID != e.ID {
		t.Errorf("expected ID %s, got %s", e.ID, got.ID)
	}
	if got.Type != event.TypeError {
		t.Errorf("expected type %s, got %s", event.TypeError, got.Type)
	}
	if got.Severity != event.SeverityCritical {
		t.Errorf("expected severity %s, got %s", event.SeverityCritical, got.Severity)
	}
	if got.Message != "service crashed" {
		t.Errorf("expected message 'service crashed', got '%s'", got.Message)
	}
	if got.Source != "api-gateway" {
		t.Errorf("expected source 'api-gateway', got '%s'", got.Source)
	}
}

func TestStoreQueryBySeverity(t *testing.T) {
	s := newTestStore(t)

	events := []*event.Event{
		event.New(event.TypeLog, event.SeverityInfo, "info message"),
		event.New(event.TypeLog, event.SeverityWarning, "warning message"),
		event.New(event.TypeLog, event.SeverityCritical, "critical message"),
	}

	for _, e := range events {
		if err := s.Save(e); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}
	s.Flush()

	results, err := s.Query(storage.Query{MinSeverity: event.SeverityCritical})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result for MinSeverity critical, got %d", len(results))
	}

	if results[0].Severity != event.SeverityCritical {
		t.Errorf("expected severity %s, got %s", event.SeverityCritical, results[0].Severity)
	}
}

func TestStoreQueryByTimeRange(t *testing.T) {
	s := newTestStore(t)

	e := event.New(event.TypeHealth, event.SeverityInfo, "heartbeat").
		WithSource("monitor")

	if err := s.Save(e); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	s.Flush()

	since := time.Now().Add(-1 * time.Minute)
	until := time.Now().Add(1 * time.Minute)

	results, err := s.Query(storage.Query{
		Since: since,
		Until: until,
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result in time range, got %d", len(results))
	}

	if results[0].Message != "heartbeat" {
		t.Errorf("expected message 'heartbeat', got '%s'", results[0].Message)
	}
}
