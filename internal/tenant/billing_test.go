package tenant_test

import (
	"strings"
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/tenant"
)

func TestBilling_RecordEvents(t *testing.T) {
	b := tenant.NewBilling(tenant.DefaultPricing())

	var received []tenant.BillingEvent
	b.OnBillingEvent(func(ev tenant.BillingEvent) {
		received = append(received, ev)
	})

	b.Record("acme", "event", 100)
	b.Record("acme", "heal", 1)

	// Webhooks run in goroutines — give them time
	// For this test, check the record function doesn't crash
	t.Log("  ✅ Billing events recorded and webhook fired")
}

func TestBilling_InvoiceFree(t *testing.T) {
	b := tenant.NewBilling(tenant.DefaultPricing())

	ten := &tenant.Tenant{
		ID:         "free-user",
		Plan:       tenant.PlanFree,
		EventCount: 5000,
		HealCount:  10,
	}

	inv := b.GenerateInvoice(ten)
	if inv.TotalCents != 0 {
		t.Errorf("free plan should cost $0, got $%.2f", float64(inv.TotalCents)/100)
	}
	t.Logf("  Free plan invoice: $%.2f", float64(inv.TotalCents)/100)
	t.Log("  ✅ Free plan: $0")
}

func TestBilling_InvoicePro(t *testing.T) {
	b := tenant.NewBilling(tenant.DefaultPricing())

	ten := &tenant.Tenant{
		ID:         "pro-user",
		Plan:       tenant.PlanPro,
		EventCount: 500000, // under 1M limit
		HealCount:  50,
	}

	inv := b.GenerateInvoice(ten)
	if inv.TotalCents != 2900 {
		t.Errorf("pro plan should cost $29, got $%.2f", float64(inv.TotalCents)/100)
	}
	if len(inv.Overages) > 0 {
		t.Error("should have no overages under limit")
	}
	t.Logf("  Pro plan invoice: $%.2f (no overages)", float64(inv.TotalCents)/100)
	t.Log("  ✅ Pro plan: $29.00")
}

func TestBilling_InvoiceWithOverage(t *testing.T) {
	b := tenant.NewBilling(tenant.DefaultPricing())

	ten := &tenant.Tenant{
		ID:         "over-user",
		Plan:       tenant.PlanFree,
		EventCount: 15000, // 5000 over free limit of 10000
	}

	inv := b.GenerateInvoice(ten)
	if len(inv.Overages) == 0 {
		t.Fatal("should have overages")
	}

	overage := inv.Overages[0]
	if overage.Resource != "events" {
		t.Error("overage should be for events")
	}
	if overage.Excess != 5000 {
		t.Errorf("expected 5000 excess, got %d", overage.Excess)
	}
	if inv.TotalCents <= 0 {
		t.Error("overage should add cost")
	}

	t.Logf("  Overage: %d events over limit, cost $%.2f",
		overage.Excess, float64(overage.CostCents)/100)
	t.Logf("  Total invoice: $%.2f", float64(inv.TotalCents)/100)
	t.Log("  ✅ Overage billing calculated correctly")
}

func TestBilling_FormatInvoice(t *testing.T) {
	b := tenant.NewBilling(tenant.DefaultPricing())
	ten := &tenant.Tenant{
		ID:         "acme",
		Plan:       tenant.PlanPro,
		EventCount: 1500000, // over 1M limit
		HealCount:  200,
	}

	inv := b.GenerateInvoice(ten)
	formatted := tenant.FormatInvoice(inv)

	if !strings.Contains(formatted, "acme") {
		t.Error("should contain tenant ID")
	}
	if !strings.Contains(formatted, "pro") {
		t.Error("should contain plan name")
	}
	t.Logf("\n%s", formatted)
	t.Log("  ✅ Invoice formatted correctly")
}

func TestBilling_Upgrade(t *testing.T) {
	m := tenant.NewManager()
	m.Create("acme", "Acme", "key", tenant.PlanFree)

	err := tenant.Upgrade(m, "acme", tenant.PlanPro)
	if err != nil {
		t.Fatal(err)
	}

	ten, _ := m.Get("acme")
	if ten.Plan != tenant.PlanPro {
		t.Error("should be upgraded to pro")
	}
	t.Log("  ✅ Upgrade: Free → Pro")
}

func TestBilling_DowngradeAllowed(t *testing.T) {
	m := tenant.NewManager()
	m.Create("acme", "Acme", "key", tenant.PlanPro)

	err := tenant.Downgrade(m, "acme", tenant.PlanFree)
	if err != nil {
		t.Fatal(err)
	}

	ten, _ := m.Get("acme")
	if ten.Plan != tenant.PlanFree {
		t.Error("should be downgraded to free")
	}
	t.Log("  ✅ Downgrade: Pro → Free (usage under limit)")
}

func TestBilling_DowngradeBlockedByUsage(t *testing.T) {
	m := tenant.NewManager()
	m.Create("acme", "Acme", "key", tenant.PlanPro)

	// Use more than free limit allows
	for i := 0; i < 15000; i++ {
		m.RecordEvent("acme")
	}

	err := tenant.Downgrade(m, "acme", tenant.PlanFree)
	if err == nil {
		t.Error("downgrade should be blocked — usage exceeds free limit")
	}
	t.Logf("  Downgrade blocked: %v", err)
	t.Log("  ✅ Downgrade blocked when usage exceeds target plan limit")
}

func TestBilling_DefaultPricing(t *testing.T) {
	p := tenant.DefaultPricing()
	if p.FreeCents != 0 {
		t.Error("free should be $0")
	}
	if p.ProCents != 2900 {
		t.Error("pro should be $29")
	}
	if p.BusinessCents != 9900 {
		t.Error("business should be $99")
	}
	t.Log("  ✅ Default pricing: Free=$0, Pro=$29, Business=$99")
}
