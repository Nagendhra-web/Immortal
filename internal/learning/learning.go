package learning

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type PatternType string

const (
	PatternFailure     PatternType = "failure"
	PatternRecovery    PatternType = "recovery"
	PatternAnomaly     PatternType = "anomaly"
	PatternCorrelation PatternType = "correlation"
)

type Pattern struct {
	ID              string      `json:"id"`
	Type            PatternType `json:"type"`
	Source          string      `json:"source"`
	Description     string      `json:"description"`
	Conditions      string      `json:"conditions"`
	Action          string      `json:"action"`
	Confidence      float64     `json:"confidence"`
	OccurrenceCount int         `json:"occurrence_count"`
	LastSeen        time.Time   `json:"last_seen"`
	CreatedAt       time.Time   `json:"created_at"`
}

type Store struct {
	mu sync.Mutex
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("learning: open db: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS patterns (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL,
			conditions TEXT NOT NULL DEFAULT '{}',
			action TEXT NOT NULL DEFAULT '',
			confidence REAL NOT NULL DEFAULT 0.5,
			occurrence_count INTEGER NOT NULL DEFAULT 1,
			last_seen DATETIME NOT NULL,
			created_at DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_patterns_type ON patterns(type);
		CREATE INDEX IF NOT EXISTS idx_patterns_source ON patterns(source);
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("learning: create table: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) RecordPattern(p Pattern) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if similar pattern exists
	var existingID string
	var count int
	err := s.db.QueryRow(
		"SELECT id, occurrence_count FROM patterns WHERE type = ? AND source = ? AND description = ?",
		string(p.Type), p.Source, p.Description,
	).Scan(&existingID, &count)

	if err == nil {
		// Update existing pattern
		newConfidence := p.Confidence
		if count >= 1 {
			newConfidence = min(1.0, p.Confidence+0.05) // Increase confidence with repetition
		}
		_, err = s.db.Exec(
			"UPDATE patterns SET occurrence_count = ?, confidence = ?, last_seen = ? WHERE id = ?",
			count+1, newConfidence, time.Now().UTC().Format(time.RFC3339), existingID,
		)
		return err
	}

	// Insert new pattern
	if p.ID == "" {
		p.ID = fmt.Sprintf("pat_%d", time.Now().UnixNano())
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	if p.LastSeen.IsZero() {
		p.LastSeen = time.Now()
	}
	if p.OccurrenceCount <= 0 {
		p.OccurrenceCount = 1
	}

	_, err = s.db.Exec(
		`INSERT INTO patterns (id, type, source, description, conditions, action, confidence, occurrence_count, last_seen, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, string(p.Type), p.Source, p.Description, p.Conditions, p.Action,
		p.Confidence, p.OccurrenceCount, p.LastSeen.UTC().Format(time.RFC3339),
		p.CreatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) FindPatterns(source string, patternType PatternType) ([]Pattern, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := "SELECT id, type, source, description, conditions, action, confidence, occurrence_count, last_seen, created_at FROM patterns WHERE 1=1"
	args := []interface{}{}

	if source != "" {
		query += " AND source = ?"
		args = append(args, source)
	}
	if patternType != "" {
		query += " AND type = ?"
		args = append(args, string(patternType))
	}
	query += " ORDER BY confidence DESC, occurrence_count DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patterns []Pattern
	for rows.Next() {
		var p Pattern
		var lastSeen, createdAt string
		err := rows.Scan(&p.ID, &p.Type, &p.Source, &p.Description, &p.Conditions, &p.Action,
			&p.Confidence, &p.OccurrenceCount, &lastSeen, &createdAt)
		if err != nil {
			return nil, err
		}
		p.LastSeen, _ = time.Parse(time.RFC3339, lastSeen)
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		patterns = append(patterns, p)
	}
	return patterns, rows.Err()
}

func (s *Store) TopPatterns(limit int) ([]Pattern, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(
		"SELECT id, type, source, description, conditions, action, confidence, occurrence_count, last_seen, created_at FROM patterns ORDER BY occurrence_count DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patterns []Pattern
	for rows.Next() {
		var p Pattern
		var lastSeen, createdAt string
		err := rows.Scan(&p.ID, &p.Type, &p.Source, &p.Description, &p.Conditions, &p.Action,
			&p.Confidence, &p.OccurrenceCount, &lastSeen, &createdAt)
		if err != nil {
			return nil, err
		}
		p.LastSeen, _ = time.Parse(time.RFC3339, lastSeen)
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		patterns = append(patterns, p)
	}
	return patterns, rows.Err()
}

func (s *Store) PatternCount() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM patterns").Scan(&count)
	return count, err
}

func (s *Store) Close() error {
	return s.db.Close()
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
