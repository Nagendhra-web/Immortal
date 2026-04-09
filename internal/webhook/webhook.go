package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Config holds the configuration for the webhook sender.
type Config struct {
	URL        string
	Secret     string
	Timeout    time.Duration
	MaxRetries int
	Headers    map[string]string
}

// Payload is the JSON body sent to the webhook endpoint.
type Payload struct {
	Event     string                 `json:"event"`
	Severity  string                 `json:"severity"`
	Source    string                 `json:"source"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
}

// DeliveryRecord captures the result of a single webhook delivery attempt.
type DeliveryRecord struct {
	Payload    Payload   `json:"payload"`
	StatusCode int       `json:"status_code"`
	Attempts   int       `json:"attempts"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// Sender sends webhook notifications with retry and history tracking.
type Sender struct {
	mu         sync.RWMutex
	cfg        Config
	client     http.Client
	history    []DeliveryRecord
	maxHistory int
}

// New creates a new Sender with defaults applied.
func New(cfg Config) *Sender {
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	return &Sender{
		cfg:        cfg,
		client:     http.Client{Timeout: cfg.Timeout},
		maxHistory: 1000,
	}
}

// Send POSTs the payload to the configured URL with retry on 5xx responses.
func (s *Sender) Send(p Payload) error {
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("webhook: marshal payload: %w", err)
	}

	rec := DeliveryRecord{
		Payload:   p,
		Timestamp: time.Now(),
	}

	var lastErr error
	for attempt := 1; attempt <= s.cfg.MaxRetries; attempt++ {
		rec.Attempts = attempt

		req, err := http.NewRequest(http.MethodPost, s.cfg.URL, bytes.NewReader(body))
		if err != nil {
			lastErr = fmt.Errorf("webhook: build request: %w", err)
			break
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Immortal-Engine/0.2.0")
		req.Header.Set("X-Immortal-Delivery", fmt.Sprintf("%d", time.Now().UnixNano()))
		if s.cfg.Secret != "" {
			req.Header.Set("X-Immortal-Signature", s.sign(body))
		}
		for k, v := range s.cfg.Headers {
			req.Header.Set(k, v)
		}

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("webhook: do request: %w", err)
			if attempt < s.cfg.MaxRetries {
				time.Sleep(time.Duration(attempt*attempt) * 100 * time.Millisecond)
			}
			continue
		}
		resp.Body.Close()

		rec.StatusCode = resp.StatusCode

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			rec.Success = true
			lastErr = nil
			break
		}

		// 4xx: do not retry
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			lastErr = fmt.Errorf("webhook: server returned %d", resp.StatusCode)
			break
		}

		// 5xx: retry with backoff
		lastErr = fmt.Errorf("webhook: server returned %d", resp.StatusCode)
		if attempt < s.cfg.MaxRetries {
			time.Sleep(time.Duration(attempt*attempt) * 100 * time.Millisecond)
		}
	}

	if lastErr != nil {
		rec.Error = lastErr.Error()
	}

	s.mu.Lock()
	s.history = append(s.history, rec)
	if len(s.history) > s.maxHistory {
		s.history = s.history[len(s.history)-s.maxHistory:]
	}
	s.mu.Unlock()

	return lastErr
}

// SendAsync sends the payload in a background goroutine (fire and forget).
func (s *Sender) SendAsync(p Payload) {
	go s.Send(p) //nolint:errcheck
}

// sign computes the HMAC-SHA256 signature of body using the configured secret.
func (s *Sender) sign(body []byte) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.Secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// History returns a copy of all delivery records.
func (s *Sender) History() []DeliveryRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DeliveryRecord, len(s.history))
	copy(out, s.history)
	return out
}

// Stats returns total, succeeded, and failed delivery counts.
func (s *Sender) Stats() (total, succeeded, failed int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total = len(s.history)
	for _, r := range s.history {
		if r.Success {
			succeeded++
		} else {
			failed++
		}
	}
	return
}
