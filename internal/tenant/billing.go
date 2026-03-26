package tenant

import (
	"fmt"
	"sync"
	"time"
)

// BillingEvent represents a usage event for billing.
type BillingEvent struct {
	TenantID  string    `json:"tenant_id"`
	Type      string    `json:"type"` // "event", "heal", "storage", "overage"
	Quantity  int64     `json:"quantity"`
	Timestamp time.Time `json:"timestamp"`
	Meta      string    `json:"meta,omitempty"`
}

// Invoice represents a billing period summary.
type Invoice struct {
	TenantID    string    `json:"tenant_id"`
	Period      string    `json:"period"` // "2026-03"
	Plan        Plan      `json:"plan"`
	EventCount  int64     `json:"event_count"`
	HealCount   int64     `json:"heal_count"`
	StorageMB   int64     `json:"storage_mb"`
	Overages    []Overage `json:"overages,omitempty"`
	TotalCents  int64     `json:"total_cents"`
	GeneratedAt time.Time `json:"generated_at"`
}

// Overage represents usage beyond plan limits.
type Overage struct {
	Resource string `json:"resource"` // "events", "heals", "storage"
	Used     int64  `json:"used"`
	Limit    int64  `json:"limit"`
	Excess   int64  `json:"excess"`
	CostCents int64 `json:"cost_cents"`
}

// BillingWebhook is called when billing events occur.
type BillingWebhook func(event BillingEvent)

// Billing manages usage tracking and invoice generation.
type Billing struct {
	mu       sync.Mutex
	events   []BillingEvent
	webhooks []BillingWebhook
	prices   PlanPricing
}

// PlanPricing defines costs per plan.
type PlanPricing struct {
	FreeCents       int64 // 0
	ProCents        int64 // 2900 ($29)
	BusinessCents   int64 // 9900 ($99)
	EnterpriseCents int64 // custom

	// Overage pricing per unit
	EventOveragePer1K int64 // cents per 1000 events over limit
	HealOveragePer1   int64 // cents per heal over limit
	StorageOveragePerGB int64 // cents per GB over limit
}

// DefaultPricing returns the standard pricing.
func DefaultPricing() PlanPricing {
	return PlanPricing{
		FreeCents:          0,
		ProCents:           2900,
		BusinessCents:      9900,
		EnterpriseCents:    0, // custom
		EventOveragePer1K:  50,  // $0.50 per 1K events
		HealOveragePer1:    10,  // $0.10 per heal
		StorageOveragePerGB: 100, // $1.00 per GB
	}
}

// NewBilling creates a billing manager.
func NewBilling(prices PlanPricing) *Billing {
	return &Billing{
		prices: prices,
	}
}

// OnBillingEvent registers a webhook for billing events.
func (b *Billing) OnBillingEvent(fn BillingWebhook) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.webhooks = append(b.webhooks, fn)
}

// Record tracks a billing event.
func (b *Billing) Record(tenantID, eventType string, quantity int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ev := BillingEvent{
		TenantID:  tenantID,
		Type:      eventType,
		Quantity:  quantity,
		Timestamp: time.Now(),
	}
	b.events = append(b.events, ev)

	for _, fn := range b.webhooks {
		go fn(ev)
	}
}

// GenerateInvoice creates an invoice for a tenant's current usage.
func (b *Billing) GenerateInvoice(t *Tenant) *Invoice {
	b.mu.Lock()
	defer b.mu.Unlock()

	limits := PlanLimits(t.Plan)
	inv := &Invoice{
		TenantID:    t.ID,
		Period:      time.Now().Format("2006-01"),
		Plan:        t.Plan,
		EventCount:  t.EventCount,
		HealCount:   t.HealCount,
		StorageMB:   t.StorageBytes / 1024 / 1024,
		GeneratedAt: time.Now(),
	}

	// Base plan cost
	switch t.Plan {
	case PlanFree:
		inv.TotalCents = b.prices.FreeCents
	case PlanPro:
		inv.TotalCents = b.prices.ProCents
	case PlanBusiness:
		inv.TotalCents = b.prices.BusinessCents
	case PlanEnterprise:
		inv.TotalCents = b.prices.EnterpriseCents
	}

	// Calculate overages
	if limits.MaxEvents > 0 && t.EventCount > limits.MaxEvents {
		excess := t.EventCount - limits.MaxEvents
		cost := (excess / 1000) * b.prices.EventOveragePer1K
		inv.Overages = append(inv.Overages, Overage{
			Resource:  "events",
			Used:      t.EventCount,
			Limit:     limits.MaxEvents,
			Excess:    excess,
			CostCents: cost,
		})
		inv.TotalCents += cost
	}

	return inv
}

// FormatInvoice returns a human-readable invoice string.
func FormatInvoice(inv *Invoice) string {
	s := fmt.Sprintf("Invoice: %s (%s plan)\n", inv.TenantID, inv.Plan)
	s += fmt.Sprintf("Period: %s\n", inv.Period)
	s += fmt.Sprintf("Events: %d | Heals: %d | Storage: %d MB\n", inv.EventCount, inv.HealCount, inv.StorageMB)
	if len(inv.Overages) > 0 {
		s += "Overages:\n"
		for _, o := range inv.Overages {
			s += fmt.Sprintf("  %s: %d used / %d limit = %d excess ($%.2f)\n",
				o.Resource, o.Used, o.Limit, o.Excess, float64(o.CostCents)/100)
		}
	}
	s += fmt.Sprintf("Total: $%.2f\n", float64(inv.TotalCents)/100)
	return s
}

// Upgrade changes a tenant's plan.
func Upgrade(m *Manager, tenantID string, newPlan Plan) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tenants[tenantID]
	if !ok {
		return fmt.Errorf("tenant not found")
	}
	t.Plan = newPlan
	return nil
}

// Downgrade changes a tenant's plan (with limit check).
func Downgrade(m *Manager, tenantID string, newPlan Plan) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tenants[tenantID]
	if !ok {
		return fmt.Errorf("tenant not found")
	}

	newLimits := PlanLimits(newPlan)
	if newLimits.MaxEvents > 0 && t.EventCount > newLimits.MaxEvents {
		return fmt.Errorf("cannot downgrade: current usage (%d events) exceeds %s limit (%d)",
			t.EventCount, newPlan, newLimits.MaxEvents)
	}

	t.Plan = newPlan
	return nil
}
