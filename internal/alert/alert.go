package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
)

type Level string

const (
	LevelInfo     Level = "info"
	LevelWarning  Level = "warning"
	LevelCritical Level = "critical"
)

type Alert struct {
	ID        string    `json:"id"`
	Level     Level     `json:"level"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
	Resolved  bool      `json:"resolved"`
}

type Channel interface {
	Send(alert *Alert) error
	Name() string
}

type LogChannel struct{}

func (l *LogChannel) Send(a *Alert) error {
	fmt.Printf("[IMMORTAL ALERT][%s] %s: %s — %s\n", a.Level, a.Source, a.Title, a.Message)
	return nil
}
func (l *LogChannel) Name() string { return "log" }

type WebhookChannel struct {
	URL string
}

func (w *WebhookChannel) Send(a *Alert) error {
	body, _ := json.Marshal(a)
	resp, err := http.Post(w.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}
func (w *WebhookChannel) Name() string { return "webhook:" + w.URL }

type CallbackChannel struct {
	Fn func(*Alert)
}

func (c *CallbackChannel) Send(a *Alert) error {
	c.Fn(a)
	return nil
}
func (c *CallbackChannel) Name() string { return "callback" }

type Manager struct {
	mu       sync.RWMutex
	channels []Channel
	history  []Alert
	rules    []AlertRule
}

type AlertRule struct {
	Name      string
	Match     func(e *event.Event) bool
	Level     Level
	Title     string
	Cooldown  time.Duration
	lastFired time.Time
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) AddChannel(ch Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels = append(m.channels, ch)
}

func (m *Manager) AddRule(rule AlertRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append(m.rules, rule)
}

func (m *Manager) Process(e *event.Event) []Alert {
	m.mu.Lock()
	defer m.mu.Unlock()

	var fired []Alert
	now := time.Now()

	for i := range m.rules {
		rule := &m.rules[i]
		if !rule.Match(e) {
			continue
		}
		if rule.Cooldown > 0 && now.Sub(rule.lastFired) < rule.Cooldown {
			continue
		}
		rule.lastFired = now

		a := Alert{
			ID:        e.ID,
			Level:     rule.Level,
			Title:     rule.Title,
			Message:   e.Message,
			Source:    e.Source,
			Timestamp: now,
		}

		for _, ch := range m.channels {
			ch.Send(&a)
		}

		m.history = append(m.history, a)
		fired = append(fired, a)
	}

	return fired
}

func (m *Manager) History() []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Alert, len(m.history))
	copy(out, m.history)
	return out
}
