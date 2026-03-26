package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
	LevelFatal Level = "fatal"
)

var levelOrder = map[Level]int{LevelDebug: 0, LevelInfo: 1, LevelWarn: 2, LevelError: 3, LevelFatal: 4}

type Entry struct {
	Timestamp string                 `json:"timestamp"`
	Level     Level                  `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

type Logger struct {
	mu     sync.Mutex
	level  Level
	output io.Writer
	fields map[string]interface{}
}

func New(level Level) *Logger {
	return &Logger{level: level, output: os.Stdout, fields: make(map[string]interface{})}
}

func (l *Logger) SetOutput(w io.Writer) { l.mu.Lock(); l.output = w; l.mu.Unlock() }

func (l *Logger) With(key string, value interface{}) *Logger {
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	newFields[key] = value
	return &Logger{level: l.level, output: l.output, fields: newFields}
}

func (l *Logger) log(level Level, msg string, args ...interface{}) {
	if levelOrder[level] < levelOrder[l.level] {
		return
	}
	entry := Entry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level,
		Message:   fmt.Sprintf(msg, args...),
		Fields:    l.fields,
	}
	data, _ := json.Marshal(entry)
	l.mu.Lock()
	fmt.Fprintln(l.output, string(data))
	l.mu.Unlock()
}

func (l *Logger) Debug(msg string, args ...interface{}) { l.log(LevelDebug, msg, args...) }
func (l *Logger) Info(msg string, args ...interface{})  { l.log(LevelInfo, msg, args...) }
func (l *Logger) Warn(msg string, args ...interface{})  { l.log(LevelWarn, msg, args...) }
func (l *Logger) Error(msg string, args ...interface{}) { l.log(LevelError, msg, args...) }
func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.log(LevelFatal, msg, args...)
	os.Exit(1)
}
