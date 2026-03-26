package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/security/ratelimit"
)

func TestLimiterAllowsWithinLimit(t *testing.T) {
	l := ratelimit.New(5, time.Second)

	for i := 0; i < 5; i++ {
		if !l.Allow("user1") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}
}

func TestLimiterBlocksOverLimit(t *testing.T) {
	l := ratelimit.New(3, time.Second)

	for i := 0; i < 3; i++ {
		l.Allow("user1")
	}

	if l.Allow("user1") {
		t.Error("4th request should be blocked")
	}
}

func TestLimiterResetsAfterWindow(t *testing.T) {
	l := ratelimit.New(2, 100*time.Millisecond)

	l.Allow("user1")
	l.Allow("user1")

	if l.Allow("user1") {
		t.Error("should be blocked")
	}

	time.Sleep(150 * time.Millisecond)

	if !l.Allow("user1") {
		t.Error("should be allowed after window reset")
	}
}

func TestLimiterPerKey(t *testing.T) {
	l := ratelimit.New(1, time.Second)

	if !l.Allow("user1") {
		t.Error("user1 first request should be allowed")
	}
	if !l.Allow("user2") {
		t.Error("user2 first request should be allowed")
	}

	if l.Allow("user1") {
		t.Error("user1 second request should be blocked")
	}
	if l.Allow("user2") {
		t.Error("user2 second request should be blocked")
	}
}

func TestLimiterRemaining(t *testing.T) {
	l := ratelimit.New(5, time.Second)

	if l.Remaining("user1") != 5 {
		t.Error("should have 5 remaining initially")
	}

	l.Allow("user1")
	l.Allow("user1")

	if l.Remaining("user1") != 3 {
		t.Errorf("expected 3 remaining, got %d", l.Remaining("user1"))
	}
}

func TestLimiterMiddleware(t *testing.T) {
	l := ratelimit.New(2, time.Second)

	handler := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	// First 2 should pass
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Errorf("request %d should pass, got %d", i+1, rec.Code)
		}
	}

	// 3rd should be rate limited
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 429 {
		t.Errorf("3rd request should be 429, got %d", rec.Code)
	}
}

func TestLimiterReset(t *testing.T) {
	l := ratelimit.New(1, time.Minute)

	l.Allow("user1")
	if l.Allow("user1") {
		t.Error("should be blocked")
	}

	l.Reset("user1")
	if !l.Allow("user1") {
		t.Error("should be allowed after reset")
	}
}
