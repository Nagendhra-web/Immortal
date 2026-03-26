package tenant

import (
	"path/filepath"
	"sync"
	"time"
)

// Isolation manages per-tenant data partitioning.
// Each tenant gets its own SQLite database file for complete data isolation.
type Isolation struct {
	mu      sync.RWMutex
	baseDir string
	paths   map[string]string // tenantID → db path
}

// NewIsolation creates a data isolation manager.
func NewIsolation(baseDir string) *Isolation {
	return &Isolation{
		baseDir: baseDir,
		paths:   make(map[string]string),
	}
}

// DBPath returns the isolated database path for a tenant.
// Each tenant gets: baseDir/tenants/{tenantID}/immortal.db
func (iso *Isolation) DBPath(tenantID string) string {
	iso.mu.Lock()
	defer iso.mu.Unlock()

	if path, ok := iso.paths[tenantID]; ok {
		return path
	}

	path := filepath.Join(iso.baseDir, "tenants", tenantID, "immortal.db")
	iso.paths[tenantID] = path
	return path
}

// DataDir returns the isolated data directory for a tenant.
func (iso *Isolation) DataDir(tenantID string) string {
	return filepath.Join(iso.baseDir, "tenants", tenantID)
}

// AllPaths returns all tenant database paths.
func (iso *Isolation) AllPaths() map[string]string {
	iso.mu.RLock()
	defer iso.mu.RUnlock()
	out := make(map[string]string, len(iso.paths))
	for k, v := range iso.paths {
		out[k] = v
	}
	return out
}

// TenantRateLimiter provides per-tenant request rate limiting.
// Prevents one tenant from consuming all system resources.
type TenantRateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*tokenBucket
	defaults RateLimitConfig
}

// RateLimitConfig defines rate limit parameters.
type RateLimitConfig struct {
	RequestsPerSecond int
	BurstSize         int
}

type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewTenantRateLimiter creates a per-tenant rate limiter.
func NewTenantRateLimiter(defaults RateLimitConfig) *TenantRateLimiter {
	if defaults.RequestsPerSecond <= 0 {
		defaults.RequestsPerSecond = 100
	}
	if defaults.BurstSize <= 0 {
		defaults.BurstSize = defaults.RequestsPerSecond * 2
	}
	return &TenantRateLimiter{
		buckets:  make(map[string]*tokenBucket),
		defaults: defaults,
	}
}

// Allow checks if a tenant can make a request. Returns true if allowed.
func (r *TenantRateLimiter) Allow(tenantID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	b, ok := r.buckets[tenantID]
	if !ok {
		b = &tokenBucket{
			tokens:     float64(r.defaults.BurstSize),
			maxTokens:  float64(r.defaults.BurstSize),
			refillRate: float64(r.defaults.RequestsPerSecond),
			lastRefill: time.Now(),
		}
		r.buckets[tenantID] = b
	}

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now

	// Check if we have a token
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// SetLimit overrides rate limit for a specific tenant.
func (r *TenantRateLimiter) SetLimit(tenantID string, rps int, burst int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buckets[tenantID] = &tokenBucket{
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		refillRate: float64(rps),
		lastRefill: time.Now(),
	}
}

// Remaining returns approximate remaining tokens for a tenant.
func (r *TenantRateLimiter) Remaining(tenantID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.buckets[tenantID]
	if !ok {
		return r.defaults.BurstSize
	}
	return int(b.tokens)
}
