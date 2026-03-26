package retention

import (
	"database/sql"
	"fmt"
	"sync"
	"time"
)

type Policy struct {
	MaxAge    time.Duration
	MaxEvents int
}

type Cleaner struct {
	mu     sync.Mutex
	db     *sql.DB
	policy Policy
	done   chan struct{}
}

func New(db *sql.DB, policy Policy) *Cleaner {
	return &Cleaner{db: db, policy: policy, done: make(chan struct{})}
}

func (c *Cleaner) Clean() (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var total int64

	if c.policy.MaxAge > 0 {
		cutoff := time.Now().Add(-c.policy.MaxAge).UTC().Format("2006-01-02T15:04:05Z")
		result, err := c.db.Exec("DELETE FROM events WHERE timestamp < ?", cutoff)
		if err != nil {
			return 0, fmt.Errorf("retention: delete by age: %w", err)
		}
		n, _ := result.RowsAffected()
		total += n
	}

	if c.policy.MaxEvents > 0 {
		result, err := c.db.Exec(
			"DELETE FROM events WHERE id NOT IN (SELECT id FROM events ORDER BY timestamp DESC LIMIT ?)",
			c.policy.MaxEvents,
		)
		if err != nil {
			return total, fmt.Errorf("retention: delete by count: %w", err)
		}
		n, _ := result.RowsAffected()
		total += n
	}

	return total, nil
}

func (c *Cleaner) Start(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-c.done:
				return
			case <-ticker.C:
				c.Clean()
			}
		}
	}()
}

func (c *Cleaner) Stop() { close(c.done) }
