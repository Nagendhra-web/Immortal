package connector

import (
	"fmt"
	"net/http"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
)

type HTTPConfig struct {
	URL      string
	Interval time.Duration
	Timeout  time.Duration
	Callback func(e *event.Event)
}

type HTTPConnector struct {
	config HTTPConfig
	client *http.Client
	done   chan struct{}
}

func NewHTTPConnector(config HTTPConfig) *HTTPConnector {
	if config.Interval == 0 {
		config.Interval = 10 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}
	return &HTTPConnector{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
		done:   make(chan struct{}),
	}
}

func (h *HTTPConnector) Start() error {
	go h.run()
	return nil
}

func (h *HTTPConnector) Stop() error {
	close(h.done)
	return nil
}

func (h *HTTPConnector) run() {
	ticker := time.NewTicker(h.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.done:
			return
		case <-ticker.C:
			h.check()
		}
	}
}

func (h *HTTPConnector) check() {
	start := time.Now()
	resp, err := h.client.Get(h.config.URL)
	latency := time.Since(start)

	if err != nil {
		h.config.Callback(
			event.New(event.TypeHealth, event.SeverityCritical,
				fmt.Sprintf("HTTP check failed: %s — %v", h.config.URL, err)).
				WithSource("http:" + h.config.URL).
				WithMeta("latency_ms", latency.Milliseconds()).
				WithMeta("status", "unreachable"),
		)
		return
	}
	defer resp.Body.Close()

	severity := event.SeverityInfo
	msg := fmt.Sprintf("HTTP %d — %s (%.0fms)", resp.StatusCode, h.config.URL, float64(latency.Milliseconds()))

	if resp.StatusCode >= 500 {
		severity = event.SeverityCritical
	} else if resp.StatusCode >= 400 {
		severity = event.SeverityWarning
	} else if latency > 2*time.Second {
		severity = event.SeverityWarning
		msg = fmt.Sprintf("HTTP %d — %s SLOW (%.0fms)", resp.StatusCode, h.config.URL, float64(latency.Milliseconds()))
	}

	h.config.Callback(
		event.New(event.TypeHealth, severity, msg).
			WithSource("http:" + h.config.URL).
			WithMeta("status_code", resp.StatusCode).
			WithMeta("latency_ms", latency.Milliseconds()),
	)
}
