package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
	_ "modernc.org/sqlite"
)

// Query defines filters for retrieving events from the store.
type Query struct {
	Type        event.Type
	Source      string
	MinSeverity event.Severity
	Since       time.Time
	Until       time.Time
	Limit       int
}

// Store is an embedded SQLite event store with async batch writes.
type Store struct {
	db      *sql.DB
	mu      sync.Mutex // serialize writes for SQLite
	batch   []*event.Event
	batchMu sync.Mutex
	done    chan struct{}
}

// New opens (or creates) a SQLite database at path and ensures the events table exists.
func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(10000)")
	if err != nil {
		return nil, fmt.Errorf("storage: open db: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("storage: set WAL mode: %w", err)
	}

	const createTable = `
CREATE TABLE IF NOT EXISTS events (
	id             TEXT PRIMARY KEY,
	type           TEXT NOT NULL,
	severity       TEXT NOT NULL,
	severity_level INTEGER NOT NULL,
	message        TEXT NOT NULL,
	source         TEXT NOT NULL DEFAULT '',
	timestamp      DATETIME NOT NULL,
	meta           TEXT NOT NULL DEFAULT '{}'
);`
	if _, err := db.Exec(createTable); err != nil {
		db.Close()
		return nil, fmt.Errorf("storage: create table: %w", err)
	}

	// Create indexes for query performance
	db.Exec("CREATE INDEX IF NOT EXISTS idx_events_type_ts ON events(type, timestamp)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_events_source ON events(source)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_events_severity ON events(severity_level)")

	s := &Store{db: db, done: make(chan struct{})}

	// Start batch flusher — writes in batches every 500ms for throughput
	go s.batchFlusher()

	return s, nil
}

// Save persists an event. Critical/fatal events are written synchronously
// (never lost on crash). Other events are batched for throughput.
func (s *Store) Save(e *event.Event) error {
	// Critical events: sync write — never lose these even on crash
	if e.Severity.Level() >= 4 { // critical or fatal
		return s.SaveSync(e)
	}

	// Non-critical: batch for throughput
	s.batchMu.Lock()
	s.batch = append(s.batch, e)
	shouldFlush := len(s.batch) >= 100
	s.batchMu.Unlock()

	if shouldFlush {
		s.flush()
	}
	return nil
}

// SaveSync persists an event immediately (synchronous).
func (s *Store) SaveSync(e *event.Event) error {
	metaJSON, err := json.Marshal(e.Meta)
	if err != nil {
		return fmt.Errorf("storage: marshal meta: %w", err)
	}

	const insert = `
INSERT INTO events (id, type, severity, severity_level, message, source, timestamp, meta)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);`

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err = s.db.Exec(insert,
		e.ID,
		string(e.Type),
		string(e.Severity),
		e.Severity.Level(),
		e.Message,
		e.Source,
		e.Timestamp.UTC().Format(time.RFC3339Nano),
		string(metaJSON),
	)
	if err != nil {
		return fmt.Errorf("storage: insert event: %w", err)
	}
	return nil
}

func (s *Store) batchFlusher() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.done:
			s.flush() // final flush on close
			return
		case <-ticker.C:
			s.flush()
		}
	}
}

// Flush forces all queued events to be written immediately.
func (s *Store) Flush() {
	s.flush()
}

func (s *Store) flush() {
	s.batchMu.Lock()
	if len(s.batch) == 0 {
		s.batchMu.Unlock()
		return
	}
	events := s.batch
	s.batch = nil
	s.batchMu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return
	}

	stmt, err := tx.Prepare(`INSERT INTO events (id, type, severity, severity_level, message, source, timestamp, meta) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return
	}
	defer stmt.Close()

	for _, e := range events {
		metaJSON, _ := json.Marshal(e.Meta)
		stmt.Exec(e.ID, string(e.Type), string(e.Severity), e.Severity.Level(),
			e.Message, e.Source, e.Timestamp.UTC().Format(time.RFC3339Nano), string(metaJSON))
	}

	tx.Commit()
}

// Query retrieves events matching the given filters.
func (s *Store) Query(q Query) ([]*event.Event, error) {
	where := "1=1"
	args := []interface{}{}

	if q.Type != "" {
		where += " AND type = ?"
		args = append(args, string(q.Type))
	}
	if q.Source != "" {
		where += " AND source = ?"
		args = append(args, q.Source)
	}
	if q.MinSeverity != "" {
		where += " AND severity_level >= ?"
		args = append(args, q.MinSeverity.Level())
	}
	if !q.Since.IsZero() {
		where += " AND timestamp >= ?"
		args = append(args, q.Since.UTC().Format(time.RFC3339Nano))
	}
	if !q.Until.IsZero() {
		where += " AND timestamp <= ?"
		args = append(args, q.Until.UTC().Format(time.RFC3339Nano))
	}

	query := fmt.Sprintf(
		"SELECT id, type, severity, message, source, timestamp, meta FROM events WHERE %s ORDER BY timestamp DESC",
		where,
	)

	if q.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", q.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("storage: query events: %w", err)
	}
	defer rows.Close()

	var events []*event.Event
	for rows.Next() {
		var (
			e         event.Event
			typ       string
			sev       string
			ts        string
			metaJSON  string
		)
		if err := rows.Scan(&e.ID, &typ, &sev, &e.Message, &e.Source, &ts, &metaJSON); err != nil {
			return nil, fmt.Errorf("storage: scan row: %w", err)
		}

		e.Type = event.Type(typ)
		e.Severity = event.Severity(sev)

		e.Timestamp, err = time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return nil, fmt.Errorf("storage: parse timestamp: %w", err)
		}

		e.Meta = make(map[string]interface{})
		if err := json.Unmarshal([]byte(metaJSON), &e.Meta); err != nil {
			return nil, fmt.Errorf("storage: unmarshal meta: %w", err)
		}

		events = append(events, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: rows iteration: %w", err)
	}

	return events, nil
}

// DB returns the underlying database connection for advanced operations.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	select {
	case s.done <- struct{}{}:
	default:
	}
	time.Sleep(100 * time.Millisecond) // let final flush complete
	return s.db.Close()
}
