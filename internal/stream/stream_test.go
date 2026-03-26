package stream_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/stream"
)

func TestEmitAndSubscribe(t *testing.T) {
	s := stream.New(100)
	sub, cleanup := s.Subscribe("test", 10, nil)
	defer cleanup()

	s.Info("engine", "started")

	select {
	case entry := <-sub.Ch:
		if entry.Message != "started" {
			t.Errorf("wrong message: %s", entry.Message)
		}
		if entry.Level != "info" {
			t.Error("wrong level")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for entry")
	}
}

func TestMultipleSubscribers(t *testing.T) {
	s := stream.New(100)

	var count atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		sub, cleanup := s.Subscribe(string(rune('a'+i)), 10, nil)
		defer cleanup()
		wg.Add(1)
		go func(sub *stream.Subscriber) {
			defer wg.Done()
			<-sub.Ch
			count.Add(1)
		}(sub)
	}

	s.Info("engine", "broadcast")
	time.Sleep(100 * time.Millisecond)

	if count.Load() != 5 {
		t.Errorf("expected 5 subscribers to receive, got %d", count.Load())
	}
}

func TestFilteredSubscriber(t *testing.T) {
	s := stream.New(100)

	// Only receive heal events
	sub, cleanup := s.Subscribe("heals-only", 10, func(e stream.LogEntry) bool {
		return e.Level == "heal"
	})
	defer cleanup()

	s.Info("engine", "ignore this")
	s.Heal("nginx", "restarted nginx")
	s.Warn("engine", "ignore this too")

	select {
	case entry := <-sub.Ch:
		if entry.Level != "heal" {
			t.Errorf("filter should only pass heal events, got %s", entry.Level)
		}
		if entry.Message != "restarted nginx" {
			t.Error("wrong message")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	// Should NOT receive the info/warn events
	select {
	case e := <-sub.Ch:
		t.Errorf("should not receive non-heal event: %s %s", e.Level, e.Message)
	case <-time.After(100 * time.Millisecond):
		// expected — no more events
	}
}

func TestHistory(t *testing.T) {
	s := stream.New(100)

	for i := 0; i < 20; i++ {
		s.Info("engine", "event")
	}

	history := s.History(5)
	if len(history) != 5 {
		t.Errorf("expected 5 history entries, got %d", len(history))
	}

	allHistory := s.History(0)
	if len(allHistory) != 20 {
		t.Errorf("expected 20 total, got %d", len(allHistory))
	}
}

func TestHistoryMaxSize(t *testing.T) {
	s := stream.New(10)

	for i := 0; i < 50; i++ {
		s.Info("engine", "overflow")
	}

	history := s.History(0)
	if len(history) != 10 {
		t.Errorf("history should cap at 10, got %d", len(history))
	}
}

func TestConvenienceEmitters(t *testing.T) {
	s := stream.New(100)
	sub, cleanup := s.Subscribe("all", 20, nil)
	defer cleanup()

	s.Info("engine", "info msg")
	s.Warn("engine", "warn msg")
	s.Error("engine", "error msg")
	s.Heal("nginx", "healed")
	s.Detect("http", "api", "500 detected")
	s.Ghost("would heal")
	s.Alert("api", "critical alert")

	time.Sleep(100 * time.Millisecond)

	// Drain all
	received := map[string]bool{}
	for {
		select {
		case e := <-sub.Ch:
			received[e.Level] = true
		default:
			goto done
		}
	}
done:

	expected := []string{"info", "warn", "error", "heal", "detect", "ghost", "alert"}
	for _, level := range expected {
		if !received[level] {
			t.Errorf("missing level: %s", level)
		}
	}
}

func TestFormatCLI(t *testing.T) {
	entry := stream.LogEntry{
		Timestamp: time.Date(2026, 3, 26, 15, 30, 45, 0, time.UTC),
		Level:     "heal",
		Component: "healer",
		Message:   "restarted nginx",
		Source:    "process:nginx",
	}

	output := stream.FormatCLI(entry)
	if len(output) == 0 {
		t.Error("formatted output should not be empty")
	}
	t.Logf("  CLI format: %s", output)
}

func TestFormatJSON(t *testing.T) {
	entry := stream.LogEntry{
		Timestamp: time.Now(),
		Level:     "heal",
		Component: "healer",
		Message:   "restarted",
	}

	output := stream.FormatJSON(entry)
	if len(output) == 0 {
		t.Error("JSON output should not be empty")
	}
	if output[0] != '{' {
		t.Error("should be valid JSON")
	}
}

func TestSubscriberCount(t *testing.T) {
	s := stream.New(100)
	if s.SubscriberCount() != 0 {
		t.Error("should start at 0")
	}

	_, c1 := s.Subscribe("a", 10, nil)
	_, c2 := s.Subscribe("b", 10, nil)
	if s.SubscriberCount() != 2 {
		t.Error("should have 2")
	}

	c1()
	if s.SubscriberCount() != 1 {
		t.Error("should have 1 after unsubscribe")
	}
	c2()
}

func TestSlowSubscriberDoesntBlock(t *testing.T) {
	s := stream.New(100)

	// Tiny buffer — will fill fast
	_, cleanup := s.Subscribe("slow", 1, nil)
	defer cleanup()

	// Blast 100 events — should not block even though subscriber is slow
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			s.Info("engine", "blast")
		}
		done <- true
	}()

	select {
	case <-done:
		// good — didn't block
	case <-time.After(2 * time.Second):
		t.Fatal("Emit blocked on slow subscriber")
	}
}
