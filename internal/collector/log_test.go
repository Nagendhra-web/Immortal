package collector_test

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/collector"
	"github.com/Nagendhra-web/Immortal/internal/event"
)

func TestLogCollectorDetectsErrors(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	path := tmpFile.Name()
	tmpFile.Close()

	var received []*event.Event
	var mu sync.Mutex

	lc := collector.NewLogCollector(path, func(e *event.Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	})

	lc.Start()
	defer lc.Stop()

	// Small delay to let collector open file and seek to end
	time.Sleep(300 * time.Millisecond)

	// Write log lines AFTER collector starts
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("2024-01-01 INFO: server started\n")
	f.WriteString("2024-01-01 ERROR: connection refused to database\n")
	f.WriteString("2024-01-01 FATAL: out of memory\n")
	f.Sync()
	f.Close()

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) < 2 {
		t.Errorf("expected at least 2 error events, got %d", len(received))
	}
}

func TestLogCollectorClassifiesSeverity(t *testing.T) {
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.log")
	if err != nil {
		t.Fatal(err)
	}
	path := tmpFile.Name()
	tmpFile.Close()

	var received []*event.Event
	var mu sync.Mutex

	lc := collector.NewLogCollector(path, func(e *event.Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	})

	lc.Start()
	defer lc.Stop()

	time.Sleep(300 * time.Millisecond)

	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("ERROR: something broke\n")
	f.Sync()
	f.Close()

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) == 0 {
		t.Fatal("expected at least 1 event")
	}
	if received[0].Severity != event.SeverityError {
		t.Errorf("expected severity error, got %s", received[0].Severity)
	}
}
