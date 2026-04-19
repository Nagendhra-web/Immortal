package webhook_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/webhook"
)

func testPayload() webhook.Payload {
	return webhook.Payload{
		Event:     "error",
		Severity:  "critical",
		Source:    "api",
		Message:   "service down",
		Timestamp: time.Now(),
	}
}

func TestNewDefaults(t *testing.T) {
	s := webhook.New(webhook.Config{URL: "http://example.com"})
	if s == nil {
		t.Fatal("New returned nil")
	}
	// Verify defaults via Stats (zero history means it was constructed cleanly)
	total, succeeded, failed := s.Stats()
	if total != 0 || succeeded != 0 || failed != 0 {
		t.Errorf("expected zero stats, got total=%d succeeded=%d failed=%d", total, succeeded, failed)
	}
}

func TestSendSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
		if ua := r.Header.Get("User-Agent"); ua != "Immortal-Engine/0.2.0" {
			t.Errorf("unexpected User-Agent: %s", ua)
		}
		if r.Header.Get("X-Immortal-Delivery") == "" {
			t.Error("missing X-Immortal-Delivery header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := webhook.New(webhook.Config{URL: srv.URL, MaxRetries: 1})
	if err := s.Send(testPayload()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hist := s.History()
	if len(hist) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(hist))
	}
	if !hist[0].Success {
		t.Error("delivery should be marked success")
	}
	if hist[0].StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", hist[0].StatusCode)
	}
}

func TestSendWithSignature(t *testing.T) {
	secret := "mysecret"
	var capturedSig string
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSig = r.Header.Get("X-Immortal-Signature")
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := webhook.New(webhook.Config{URL: srv.URL, Secret: secret, MaxRetries: 1})
	if err := s.Send(testPayload()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedSig == "" {
		t.Fatal("X-Immortal-Signature header missing")
	}
	if !strings.HasPrefix(capturedSig, "sha256=") {
		t.Errorf("signature should start with sha256=, got %s", capturedSig)
	}

	// Verify the HMAC is correct
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(capturedBody)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if capturedSig != expected {
		t.Errorf("signature mismatch: got %s, want %s", capturedSig, expected)
	}
}

func TestSendRetryOn500(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := webhook.New(webhook.Config{URL: srv.URL, MaxRetries: 3})
	if err := s.Send(testPayload()); err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}

	if calls.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", calls.Load())
	}

	hist := s.History()
	if len(hist) != 1 {
		t.Fatalf("expected 1 record, got %d", len(hist))
	}
	if !hist[0].Success {
		t.Error("delivery should be marked success")
	}
	if hist[0].Attempts != 3 {
		t.Errorf("expected 3 attempts in record, got %d", hist[0].Attempts)
	}
}

func TestSendNoRetryOn400(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	s := webhook.New(webhook.Config{URL: srv.URL, MaxRetries: 3})
	err := s.Send(testPayload())
	if err == nil {
		t.Fatal("expected error on 400")
	}

	if calls.Load() != 1 {
		t.Errorf("expected 1 attempt (no retry on 4xx), got %d", calls.Load())
	}

	hist := s.History()
	if len(hist) != 1 {
		t.Fatalf("expected 1 record, got %d", len(hist))
	}
	if hist[0].Success {
		t.Error("delivery should not be marked success on 400")
	}
}

func TestSendAsync(t *testing.T) {
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		close(done)
	}))
	defer srv.Close()

	s := webhook.New(webhook.Config{URL: srv.URL, MaxRetries: 1})
	s.SendAsync(testPayload())

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("async delivery did not complete within 2s")
	}

	// Give the goroutine time to record history
	time.Sleep(10 * time.Millisecond)
	hist := s.History()
	if len(hist) != 1 {
		t.Fatalf("expected 1 history record after async, got %d", len(hist))
	}
	if !hist[0].Success {
		t.Error("async delivery should be marked success")
	}
}

func TestHistory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := webhook.New(webhook.Config{URL: srv.URL, MaxRetries: 1})

	for i := 0; i < 5; i++ {
		p := testPayload()
		p.Message = "msg"
		_ = s.Send(p)
	}

	hist := s.History()
	if len(hist) != 5 {
		t.Errorf("expected 5 history records, got %d", len(hist))
	}
}

func TestStats(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n%2 == 0 {
			// Even calls return 500 to force failure (MaxRetries=1)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	s := webhook.New(webhook.Config{URL: srv.URL, MaxRetries: 1})

	// 3 sends: call 1 -> 200 (success), call 2 -> 500 (fail), call 3 -> 200 (success)
	for i := 0; i < 3; i++ {
		_ = s.Send(testPayload())
	}

	total, succeeded, failed := s.Stats()
	if total != 3 {
		t.Errorf("expected total=3, got %d", total)
	}
	if succeeded != 2 {
		t.Errorf("expected succeeded=2, got %d", succeeded)
	}
	if failed != 1 {
		t.Errorf("expected failed=1, got %d", failed)
	}
}

// Ensure Payload marshals correctly.
func TestPayloadJSON(t *testing.T) {
	p := webhook.Payload{
		Event:     "error",
		Severity:  "critical",
		Source:    "db",
		Message:   "connection lost",
		Timestamp: time.Now(),
		Meta:      map[string]interface{}{"retry": 3},
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var out webhook.Payload
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out.Event != p.Event || out.Source != p.Source {
		t.Error("round-trip JSON mismatch")
	}
}
