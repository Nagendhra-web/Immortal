package throttle

import (
	"sync"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/event"
)

type Throttler struct {
	mu       sync.Mutex
	seen     map[string]time.Time
	interval time.Duration
}

func New(interval time.Duration) *Throttler {
	return &Throttler{seen: make(map[string]time.Time), interval: interval}
}

func (t *Throttler) Allow(e *event.Event) bool {
	key := e.Source + ":" + string(e.Severity) + ":" + e.Message
	t.mu.Lock()
	defer t.mu.Unlock()
	last, ok := t.seen[key]
	if ok && time.Since(last) < t.interval {
		return false
	}
	t.seen[key] = time.Now()
	return true
}

func (t *Throttler) Cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	for k, v := range t.seen {
		if now.Sub(v) > t.interval*2 {
			delete(t.seen, k)
		}
	}
}

func (t *Throttler) Size() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.seen)
}
