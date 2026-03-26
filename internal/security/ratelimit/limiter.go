package ratelimit

import (
	"net/http"
	"sync"
	"time"
)

type entry struct {
	count       int
	windowStart time.Time
}

type Limiter struct {
	mu       sync.Mutex
	requests map[string]*entry
	limit    int
	window   time.Duration
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{
		requests: make(map[string]*entry),
		limit:    limit,
		window:   window,
	}
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	e, exists := l.requests[key]

	if !exists || now.Sub(e.windowStart) > l.window {
		l.requests[key] = &entry{count: 1, windowStart: now}
		return true
	}

	e.count++
	return e.count <= l.limit
}

func (l *Limiter) Remaining(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, exists := l.requests[key]
	if !exists {
		return l.limit
	}
	if time.Since(e.windowStart) > l.window {
		return l.limit
	}
	remaining := l.limit - e.count
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr
		if !l.Allow(key) {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.requests, key)
}
