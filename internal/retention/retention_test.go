package retention_test

import (
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/event"
	"github.com/Nagendhra-web/Immortal/internal/retention"
	"github.com/Nagendhra-web/Immortal/internal/storage"
)

func TestCleanByMaxEvents(t *testing.T) {
	store, _ := storage.New(t.TempDir() + "/ret.db")
	defer store.Close()

	for i := 0; i < 20; i++ {
		store.Save(event.New(event.TypeError, event.SeverityError, "test"))
	}
	store.Flush()

	cleaner := retention.New(store.DB(), retention.Policy{MaxEvents: 10})
	deleted, err := cleaner.Clean()
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 10 {
		t.Errorf("expected 10 deleted, got %d", deleted)
	}

	events, _ := store.Query(storage.Query{Limit: 100})
	if len(events) != 10 {
		t.Errorf("expected 10 remaining, got %d", len(events))
	}
}

func TestCleanByAge(t *testing.T) {
	store, _ := storage.New(t.TempDir() + "/ret2.db")
	defer store.Close()

	store.Save(event.New(event.TypeError, event.SeverityError, "recent"))
	store.Flush()

	cleaner := retention.New(store.DB(), retention.Policy{MaxAge: time.Hour})
	deleted, err := cleaner.Clean()
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 0 {
		t.Errorf("recent events should not be deleted, got %d", deleted)
	}
}
