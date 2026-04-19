package storage_test

import (
	"sync"
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/event"
	"github.com/Nagendhra-web/Immortal/internal/storage"
)

func TestStoreBulkInsert(t *testing.T) {
	store, err := storage.New(t.TempDir() + "/bulk.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for i := 0; i < 1000; i++ {
		err := store.Save(event.New(event.TypeError, event.SeverityError, "bulk event"))
		if err != nil {
			t.Fatalf("failed at insert %d: %v", i, err)
		}
	}

	store.Flush()

	events, err := store.Query(storage.Query{Type: event.TypeError, Limit: 2000})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1000 {
		t.Errorf("expected 1000 events, got %d", len(events))
	}
}

func TestStoreConcurrentWrites(t *testing.T) {
	store, err := storage.New(t.TempDir() + "/concurrent.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var wg sync.WaitGroup
	errors := make(chan error, 50)

	// SQLite handles sequential writes well; use 5 goroutines to test realistic concurrency
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				if err := store.Save(event.New(event.TypeError, event.SeverityError, "concurrent")); err != nil {
					errors <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent write error: %v", err)
	}
}

func TestStoreQueryEmpty(t *testing.T) {
	store, err := storage.New(t.TempDir() + "/empty.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	events, err := store.Query(storage.Query{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events from empty store, got %d", len(events))
	}
}

func TestStoreQueryAllFilters(t *testing.T) {
	store, err := storage.New(t.TempDir() + "/filters.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	store.Save(event.New(event.TypeError, event.SeverityCritical, "critical error").WithSource("api"))
	store.Save(event.New(event.TypeMetric, event.SeverityInfo, "metric").WithSource("system"))
	store.Save(event.New(event.TypeError, event.SeverityInfo, "info error").WithSource("api"))
	store.Flush()

	// Filter by type + source + severity
	events, err := store.Query(storage.Query{
		Type:        event.TypeError,
		Source:      "api",
		MinSeverity: event.SeverityCritical,
		Limit:       10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event matching all filters, got %d", len(events))
	}
}
