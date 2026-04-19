package learning_test

import (
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/learning"
)

func TestRecordAndFindPattern(t *testing.T) {
	s, err := learning.New(t.TempDir() + "/learn.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	err = s.RecordPattern(learning.Pattern{
		Type:        learning.PatternFailure,
		Source:      "api-server",
		Description: "OOM crash after sustained load",
		Action:      "restart + increase memory limit",
		Confidence:  0.7,
	})
	if err != nil {
		t.Fatal(err)
	}

	patterns, err := s.FindPatterns("api-server", learning.PatternFailure)
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}
	if patterns[0].Description != "OOM crash after sustained load" {
		t.Error("wrong description")
	}
}

func TestPatternConfidenceGrows(t *testing.T) {
	s, err := learning.New(t.TempDir() + "/learn.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Record same pattern 3 times
	for i := 0; i < 3; i++ {
		s.RecordPattern(learning.Pattern{
			Type:        learning.PatternFailure,
			Source:      "db",
			Description: "connection pool exhausted",
			Confidence:  0.5,
		})
	}

	patterns, _ := s.FindPatterns("db", "")
	if len(patterns) != 1 {
		t.Fatalf("should merge into 1 pattern, got %d", len(patterns))
	}
	if patterns[0].OccurrenceCount != 3 {
		t.Errorf("expected 3 occurrences, got %d", patterns[0].OccurrenceCount)
	}
	if patterns[0].Confidence <= 0.5 {
		t.Error("confidence should increase with repetition")
	}
}

func TestTopPatterns(t *testing.T) {
	s, err := learning.New(t.TempDir() + "/learn.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Record patterns with different frequencies
	for i := 0; i < 5; i++ {
		s.RecordPattern(learning.Pattern{Type: learning.PatternFailure, Source: "a", Description: "frequent"})
	}
	for i := 0; i < 2; i++ {
		s.RecordPattern(learning.Pattern{Type: learning.PatternFailure, Source: "b", Description: "rare"})
	}

	top, _ := s.TopPatterns(1)
	if len(top) != 1 {
		t.Fatal("expected 1")
	}
	if top[0].Description != "frequent" {
		t.Error("most frequent pattern should be first")
	}
}

func TestPatternCount(t *testing.T) {
	s, err := learning.New(t.TempDir() + "/learn.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.RecordPattern(learning.Pattern{Type: learning.PatternFailure, Source: "a", Description: "p1"})
	s.RecordPattern(learning.Pattern{Type: learning.PatternAnomaly, Source: "b", Description: "p2"})

	count, _ := s.PatternCount()
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestFindByType(t *testing.T) {
	s, err := learning.New(t.TempDir() + "/learn.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.RecordPattern(learning.Pattern{Type: learning.PatternFailure, Source: "a", Description: "fail"})
	s.RecordPattern(learning.Pattern{Type: learning.PatternRecovery, Source: "a", Description: "recover"})
	s.RecordPattern(learning.Pattern{Type: learning.PatternAnomaly, Source: "b", Description: "anomaly"})

	failures, _ := s.FindPatterns("", learning.PatternFailure)
	if len(failures) != 1 {
		t.Errorf("expected 1 failure, got %d", len(failures))
	}
}
