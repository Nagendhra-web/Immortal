package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Channel interface {
	Send(title, message, level string) error
	Name() string
}

type SlackChannel struct{ WebhookURL string }

func (s *SlackChannel) Send(title, message, level string) error {
	payload := map[string]interface{}{
		"text": fmt.Sprintf("*[%s] %s*\n%s", level, title, message),
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(s.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
func (s *SlackChannel) Name() string { return "slack" }

type DiscordChannel struct{ WebhookURL string }

func (d *DiscordChannel) Send(title, message, level string) error {
	payload := map[string]interface{}{
		"content": fmt.Sprintf("**[%s] %s**\n%s", level, title, message),
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(d.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
func (d *DiscordChannel) Name() string { return "discord" }

type ConsoleChannel struct{}

func (c *ConsoleChannel) Send(title, message, level string) error {
	fmt.Printf("[IMMORTAL][%s] %s: %s\n", level, title, message)
	return nil
}
func (c *ConsoleChannel) Name() string { return "console" }

type CallbackChannel struct{ Fn func(title, message, level string) }

func (c *CallbackChannel) Send(title, message, level string) error {
	c.Fn(title, message, level)
	return nil
}
func (c *CallbackChannel) Name() string { return "callback" }

type Dispatcher struct {
	mu       sync.RWMutex
	channels []Channel
	history  []Notification
}

type Notification struct {
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Level     string    `json:"level"`
	Channel   string    `json:"channel"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
}

func NewDispatcher() *Dispatcher { return &Dispatcher{} }

func (d *Dispatcher) AddChannel(ch Channel) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.channels = append(d.channels, ch)
}

func (d *Dispatcher) Send(title, message, level string) []error {
	d.mu.RLock()
	channels := make([]Channel, len(d.channels))
	copy(channels, d.channels)
	d.mu.RUnlock()

	var errs []error
	for _, ch := range channels {
		n := Notification{Title: title, Message: message, Level: level, Channel: ch.Name(), Timestamp: time.Now()}
		if err := ch.Send(title, message, level); err != nil {
			n.Error = err.Error()
			errs = append(errs, err)
		}
		d.mu.Lock()
		d.history = append(d.history, n)
		d.mu.Unlock()
	}
	return errs
}

func (d *Dispatcher) History() []Notification {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]Notification, len(d.history))
	copy(out, d.history)
	return out
}
