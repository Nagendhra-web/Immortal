package logger_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/immortal-engine/immortal/internal/logger"
)

func TestLoggerOutput(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.LevelInfo)
	l.SetOutput(&buf)
	l.Info("hello %s", "world")

	var entry logger.Entry
	json.NewDecoder(&buf).Decode(&entry)
	if entry.Message != "hello world" {
		t.Errorf("got '%s'", entry.Message)
	}
	if entry.Level != logger.LevelInfo {
		t.Errorf("got '%s'", entry.Level)
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.LevelWarn)
	l.SetOutput(&buf)
	l.Debug("hidden")
	l.Info("hidden")
	l.Warn("shown")
	if strings.Count(buf.String(), "\n") != 1 {
		t.Error("only warn should be logged")
	}
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.LevelInfo)
	l.SetOutput(&buf)
	l.With("service", "api").With("version", "1.0").Info("started")

	var entry logger.Entry
	json.NewDecoder(&buf).Decode(&entry)
	if entry.Fields["service"] != "api" {
		t.Error("missing service field")
	}
	if entry.Fields["version"] != "1.0" {
		t.Error("missing version field")
	}
}

func TestLoggerConcurrent(t *testing.T) {
	var buf bytes.Buffer
	l := logger.New(logger.LevelInfo)
	l.SetOutput(&buf)
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func(n int) { l.Info("msg %d", n); done <- true }(i)
	}
	for i := 0; i < 100; i++ {
		<-done
	}
	lines := strings.Count(buf.String(), "\n")
	if lines != 100 {
		t.Errorf("expected 100 lines, got %d", lines)
	}
}
