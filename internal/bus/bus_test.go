package bus_test

import (
	"sync"
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/bus"
	"github.com/Nagendhra-web/Immortal/internal/event"
)

func TestBusPublishSubscribe(t *testing.T) {
	b := bus.New()

	var received *event.Event
	var wg sync.WaitGroup
	wg.Add(1)

	b.Subscribe("error", func(e *event.Event) {
		received = e
		wg.Done()
	})

	e := event.New(event.TypeError, event.SeverityCritical, "test crash")
	b.Publish(e)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}

	if received == nil {
		t.Fatal("expected to receive event")
	}
	if received.Message != "test crash" {
		t.Errorf("expected 'test crash', got '%s'", received.Message)
	}
}

func TestBusMultipleSubscribers(t *testing.T) {
	b := bus.New()

	var count int
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(3)

	for i := 0; i < 3; i++ {
		b.Subscribe("error", func(e *event.Event) {
			mu.Lock()
			count++
			mu.Unlock()
			wg.Done()
		})
	}

	b.Publish(event.New(event.TypeError, event.SeverityError, "test"))

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}

	mu.Lock()
	defer mu.Unlock()
	if count != 3 {
		t.Errorf("expected 3 subscribers notified, got %d", count)
	}
}

func TestBusSubscribeAll(t *testing.T) {
	b := bus.New()

	var received []*event.Event
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(2)

	b.Subscribe("*", func(e *event.Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
		wg.Done()
	})

	b.Publish(event.New(event.TypeError, event.SeverityError, "err"))
	b.Publish(event.New(event.TypeMetric, event.SeverityInfo, "metric"))

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Errorf("expected 2 events, got %d", len(received))
	}
}
