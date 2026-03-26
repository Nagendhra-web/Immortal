package tenant

import (
	"fmt"
	"sync"
	"time"
)

// Tenant represents an isolated customer/team within Immortal.
type Tenant struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	APIKey    string            `json:"api_key"`
	Plan      Plan              `json:"plan"`
	Config    TenantConfig      `json:"config"`
	CreatedAt time.Time         `json:"created_at"`
	Active    bool              `json:"active"`
	Meta      map[string]string `json:"meta,omitempty"`

	// Usage tracking
	EventCount   int64 `json:"event_count"`
	HealCount    int64 `json:"heal_count"`
	StorageBytes int64 `json:"storage_bytes"`
}

// Plan defines usage limits per tenant.
type Plan string

const (
	PlanFree       Plan = "free"
	PlanPro        Plan = "pro"
	PlanBusiness   Plan = "business"
	PlanEnterprise Plan = "enterprise"
)

// PlanLimits returns the limits for a plan.
func PlanLimits(p Plan) Limits {
	switch p {
	case PlanFree:
		return Limits{MaxEvents: 10000, MaxRules: 5, MaxServices: 3, MaxStorageMB: 100, RateLimit: 10}
	case PlanPro:
		return Limits{MaxEvents: 1000000, MaxRules: 50, MaxServices: 25, MaxStorageMB: 5000, RateLimit: 100}
	case PlanBusiness:
		return Limits{MaxEvents: 10000000, MaxRules: 500, MaxServices: 100, MaxStorageMB: 50000, RateLimit: 1000}
	case PlanEnterprise:
		return Limits{MaxEvents: -1, MaxRules: -1, MaxServices: -1, MaxStorageMB: -1, RateLimit: -1} // unlimited
	default:
		return PlanLimits(PlanFree)
	}
}

// Limits defines resource caps per tenant.
type Limits struct {
	MaxEvents    int64 `json:"max_events"`    // -1 = unlimited
	MaxRules     int   `json:"max_rules"`
	MaxServices  int   `json:"max_services"`
	MaxStorageMB int64 `json:"max_storage_mb"`
	RateLimit    int   `json:"rate_limit"`    // events per second
}

// TenantConfig holds per-tenant configuration.
type TenantConfig struct {
	GhostMode  bool     `json:"ghost_mode"`
	RulesJSON  string   `json:"rules_json,omitempty"`
	WatchURLs  []string `json:"watch_urls,omitempty"`
	WatchProcs []string `json:"watch_procs,omitempty"`
	WatchLogs  []string `json:"watch_logs,omitempty"`
}

// Manager handles multi-tenant operations.
type Manager struct {
	mu      sync.RWMutex
	tenants map[string]*Tenant  // by ID
	byKey   map[string]*Tenant  // by API key
}

// NewManager creates a new tenant manager.
func NewManager() *Manager {
	return &Manager{
		tenants: make(map[string]*Tenant),
		byKey:   make(map[string]*Tenant),
	}
}

// Create registers a new tenant.
func (m *Manager) Create(id, name, apiKey string, plan Plan) (*Tenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tenants[id]; exists {
		return nil, fmt.Errorf("tenant '%s' already exists", id)
	}
	if _, exists := m.byKey[apiKey]; exists {
		return nil, fmt.Errorf("API key already in use")
	}

	t := &Tenant{
		ID:        id,
		Name:      name,
		APIKey:    apiKey,
		Plan:      plan,
		CreatedAt: time.Now(),
		Active:    true,
		Meta:      make(map[string]string),
	}

	m.tenants[id] = t
	m.byKey[apiKey] = t
	return t, nil
}

// Get returns a tenant by ID.
func (m *Manager) Get(id string) (*Tenant, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tenants[id]
	if !ok {
		return nil, false
	}
	copy := *t
	return &copy, true
}

// GetByAPIKey returns a tenant by API key (for request authentication).
func (m *Manager) GetByAPIKey(key string) (*Tenant, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.byKey[key]
	if !ok {
		return nil, false
	}
	copy := *t
	return &copy, true
}

// Authenticate validates an API key and returns the tenant.
func (m *Manager) Authenticate(apiKey string) (*Tenant, error) {
	t, ok := m.GetByAPIKey(apiKey)
	if !ok {
		return nil, fmt.Errorf("invalid API key")
	}
	if !t.Active {
		return nil, fmt.Errorf("tenant '%s' is suspended", t.ID)
	}
	return t, nil
}

// CheckLimit verifies a tenant hasn't exceeded their plan limits.
func (m *Manager) CheckLimit(id string, resource string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tenants[id]
	if !ok {
		return fmt.Errorf("tenant not found")
	}

	limits := PlanLimits(t.Plan)

	switch resource {
	case "events":
		if limits.MaxEvents > 0 && t.EventCount >= limits.MaxEvents {
			return fmt.Errorf("tenant '%s' exceeded event limit (%d/%d) — upgrade plan", t.ID, t.EventCount, limits.MaxEvents)
		}
	case "rules":
		if limits.MaxRules > 0 {
			return nil // checked at rule creation time
		}
	}
	return nil
}

// RecordEvent increments the event counter for a tenant.
func (m *Manager) RecordEvent(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tenants[id]; ok {
		t.EventCount++
	}
}

// RecordHeal increments the heal counter for a tenant.
func (m *Manager) RecordHeal(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tenants[id]; ok {
		t.HealCount++
	}
}

// Suspend disables a tenant.
func (m *Manager) Suspend(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tenants[id]
	if !ok {
		return fmt.Errorf("tenant not found")
	}
	t.Active = false
	return nil
}

// Activate enables a tenant.
func (m *Manager) Activate(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tenants[id]
	if !ok {
		return fmt.Errorf("tenant not found")
	}
	t.Active = true
	return nil
}

// Delete removes a tenant.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tenants[id]
	if !ok {
		return fmt.Errorf("tenant not found")
	}
	delete(m.byKey, t.APIKey)
	delete(m.tenants, id)
	return nil
}

// All returns all tenants.
func (m *Manager) All() []Tenant {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Tenant
	for _, t := range m.tenants {
		result = append(result, *t)
	}
	return result
}

// Count returns number of tenants.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tenants)
}

// Usage returns usage stats for a tenant.
func (m *Manager) Usage(id string) (events, heals, storageMB int64, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tenants[id]
	if !ok {
		return 0, 0, 0, fmt.Errorf("tenant not found")
	}
	return t.EventCount, t.HealCount, t.StorageBytes / 1024 / 1024, nil
}
