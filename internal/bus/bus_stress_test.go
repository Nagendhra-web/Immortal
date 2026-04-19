package bus_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/bus"
	"github.com/Nagendhra-web/Immortal/internal/event"
)

func TestBusHighThroughput(t *testing.T) {
	b := bus.New()
	var count atomic.Int64
	var wg sync.WaitGroup

	b.Subscribe("*", func(e *event.Event) {
		count.Add(1)
		wg.Done()
	})

	total := 10000
	wg.Add(total)

	for i := 0; i < total; i++ {
		b.Publish(event.New(event.TypeError, event.SeverityError, "high throughput"))
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out — only received %d of %d events", count.Load(), total)
	}

	if count.Load() != int64(total) {
		t.Errorf("expected %d events, got %d", total, count.Load())
	}
}

func TestBusConcurrentPublishSubscribe(t *testing.T) {
	b := bus.New()
	var count atomic.Int64

	// Subscribe concurrently
	for i := 0; i < 10; i++ {
		b.Subscribe("error", func(e *event.Event) {
			count.Add(1)
		})
	}

	// Publish concurrently
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Publish(event.New(event.TypeError, event.SeverityError, "concurrent"))
		}()
	}
	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	// 100 publishes x 10 subscribers = 1000
	if count.Load() != 1000 {
		t.Errorf("expected 1000 events, got %d", count.Load())
	}
}

func TestBusNoSubscribers(t *testing.T) {
	b := bus.New()
	// Should not panic with no subscribers
	b.Publish(event.New(event.TypeError, event.SeverityError, "nobody listening"))
}

func TestBusMultipleTopics(t *testing.T) {
	b := bus.New()
	var errorCount, metricCount atomic.Int64
	var wg sync.WaitGroup
	wg.Add(2)

	b.Subscribe("error", func(e *event.Event) {
		errorCount.Add(1)
		wg.Done()
	})
	b.Subscribe("metric", func(e *event.Event) {
		metricCount.Add(1)
		wg.Done()
	})

	b.Publish(event.New(event.TypeError, event.SeverityError, "err"))
	b.Publish(event.New(event.TypeMetric, event.SeverityInfo, "met"))

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}

	if errorCount.Load() != 1 {
		t.Errorf("expected 1 error event, got %d", errorCount.Load())
	}
	if metricCount.Load() != 1 {
		t.Errorf("expected 1 metric event, got %d", metricCount.Load())
	}
}
