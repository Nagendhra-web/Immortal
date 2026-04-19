package collector

import (
	"bufio"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/event"
)

type LogCollector struct {
	path     string
	callback EventCallback
	done     chan struct{}
}

func NewLogCollector(path string, callback EventCallback) *LogCollector {
	return &LogCollector{
		path:     path,
		callback: callback,
		done:     make(chan struct{}),
	}
}

func (l *LogCollector) Name() string { return "log:" + l.path }

func (l *LogCollector) Start() error {
	go l.run()
	return nil
}

func (l *LogCollector) Stop() error {
	close(l.done)
	return nil
}

func (l *LogCollector) run() {
	f, err := os.Open(l.path)
	if err != nil {
		return
	}
	defer f.Close()

	// Seek to end
	f.Seek(0, io.SeekEnd)
	reader := bufio.NewReader(f)

	for {
		select {
		case <-l.done:
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				time.Sleep(200 * time.Millisecond)
				continue
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			severity := classifyLine(line)
			if severity.Level() >= event.SeverityWarning.Level() {
				e := event.New(event.TypeLog, severity, line).
					WithSource(l.path)
				l.callback(e)
			}
		}
	}
}

func classifyLine(line string) event.Severity {
	upper := strings.ToUpper(line)
	switch {
	case strings.Contains(upper, "FATAL") || strings.Contains(upper, "PANIC"):
		return event.SeverityFatal
	case strings.Contains(upper, "CRITICAL"):
		return event.SeverityCritical
	case strings.Contains(upper, "ERROR") || strings.Contains(upper, "ERR"):
		return event.SeverityError
	case strings.Contains(upper, "WARN") || strings.Contains(upper, "WARNING"):
		return event.SeverityWarning
	default:
		return event.SeverityInfo
	}
}
