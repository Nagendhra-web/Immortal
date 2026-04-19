package bus

import (
	"sync"
	"sync/atomic"

	"github.com/Nagendhra-web/Immortal/internal/event"
)

// Handler processes an event.
type Handler func(e *event.Event)

const (
	defaultBufferSize = 10000
	defaultWorkers    = 16
)

type work struct {
	handler Handler
	event   *event.Event
}

// Bus is an in-process event bus with bounded worker pool and backpressure.
type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	queue    chan work
	wg       sync.WaitGroup
	running  atomic.Bool

	// Stats
	published atomic.Int64
	processed atomic.Int64
	dropped   atomic.Int64
}

// New creates a new event bus with default buffer (10K) and workers (16).
func New() *Bus {
	return NewWithConfig(defaultBufferSize, defaultWorkers)
}

// NewWithConfig creates a bus with custom buffer size and worker count.
func NewWithConfig(bufferSize, workers int) *Bus {
	if bufferSize <= 0 {
		bufferSize = defaultBufferSize
	}
	if workers <= 0 {
		workers = defaultWorkers
	}

	b := &Bus{
		handlers: make(map[string][]Handler),
		queue:    make(chan work, bufferSize),
	}

	// Start worker pool
	b.running.Store(true)
	for i := 0; i < workers; i++ {
		b.wg.Add(1)
		go b.worker()
	}

	return b
}

func (b *Bus) worker() {
	defer b.wg.Done()
	for w := range b.queue {
		w.handler(w.event)
		b.processed.Add(1)
	}
}

// Subscribe registers a handler for a topic.
// Use "*" to subscribe to all events.
func (b *Bus) Subscribe(topic string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[topic] = append(b.handlers[topic], handler)
}

// Publish sends an event to all matching subscribers.
// If the queue is full, the event is dropped (backpressure).
func (b *Bus) Publish(e *event.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	topic := string(e.Type)
	b.published.Add(1)

	// Collect all matching handlers
	var matched []Handler
	matched = append(matched, b.handlers[topic]...)
	if topic != "*" {
		matched = append(matched, b.handlers["*"]...)
	}

	// Dispatch to worker pool via buffered channel
	// Priority-aware: critical/fatal events NEVER get dropped
	isCritical := e.Severity.Level() >= 4 // critical or fatal

	for _, h := range matched {
		if isCritical {
			// Critical events block until space — never dropped
			b.queue <- work{handler: h, event: e}
		} else {
			// Non-critical events drop gracefully under backpressure
			select {
			case b.queue <- work{handler: h, event: e}:
				// Queued
			default:
				b.dropped.Add(1)
			}
		}
	}
}

// Close shuts down the worker pool and waits for in-flight work.
func (b *Bus) Close() {
	if b.running.CompareAndSwap(true, false) {
		close(b.queue)
		b.wg.Wait()
	}
}

// Stats returns bus statistics.
func (b *Bus) Stats() (published, processed, dropped int64) {
	return b.published.Load(), b.processed.Load(), b.dropped.Load()
}

// QueueLen returns current items waiting in the queue.
func (b *Bus) QueueLen() int {
	return len(b.queue)
}
