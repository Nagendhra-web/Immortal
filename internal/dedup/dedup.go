package dedup

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
)

type Deduplicator struct {
	mu     sync.Mutex
	seen   map[string]time.Time
	window time.Duration
}

func New(window time.Duration) *Deduplicator {
	return &Deduplicator{seen: make(map[string]time.Time), window: window}
}

func (d *Deduplicator) IsDuplicate(e *event.Event) bool {
	key := fingerprint(e)
	d.mu.Lock()
	defer d.mu.Unlock()
	if last, ok := d.seen[key]; ok && time.Since(last) < d.window {
		return true
	}
	d.seen[key] = time.Now()
	return false
}

func (d *Deduplicator) Cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	for k, t := range d.seen {
		if now.Sub(t) > d.window {
			delete(d.seen, k)
		}
	}
}

func (d *Deduplicator) Size() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.seen)
}

func fingerprint(e *event.Event) string {
	data := string(e.Type) + "|" + string(e.Severity) + "|" + e.Source + "|" + e.Message
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:8])
}
