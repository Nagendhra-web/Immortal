package tenant_test

import (
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/tenant"
)

func TestCreateAndGet(t *testing.T) {
	m := tenant.NewManager()
	ten, err := m.Create("acme", "Acme Corp", "key-acme-123", tenant.PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	if ten.ID != "acme" {
		t.Error("wrong ID")
	}
	if ten.Plan != tenant.PlanPro {
		t.Error("wrong plan")
	}

	got, ok := m.Get("acme")
	if !ok {
		t.Fatal("should find tenant")
	}
	if got.Name != "Acme Corp" {
		t.Error("wrong name")
	}
}

func TestDuplicateID(t *testing.T) {
	m := tenant.NewManager()
	m.Create("dup", "First", "key-1", tenant.PlanFree)
	_, err := m.Create("dup", "Second", "key-2", tenant.PlanFree)
	if err == nil {
		t.Error("duplicate ID should fail")
	}
}

func TestDuplicateAPIKey(t *testing.T) {
	m := tenant.NewManager()
	m.Create("t1", "First", "same-key", tenant.PlanFree)
	_, err := m.Create("t2", "Second", "same-key", tenant.PlanFree)
	if err == nil {
		t.Error("duplicate API key should fail")
	}
}

func TestAuthenticate(t *testing.T) {
	m := tenant.NewManager()
	m.Create("acme", "Acme", "secret-key", tenant.PlanPro)

	// Valid key
	ten, err := m.Authenticate("secret-key")
	if err != nil {
		t.Fatal(err)
	}
	if ten.ID != "acme" {
		t.Error("wrong tenant")
	}

	// Invalid key
	_, err = m.Authenticate("bad-key")
	if err == nil {
		t.Error("bad key should fail")
	}
}

func TestSuspendedTenantCantAuth(t *testing.T) {
	m := tenant.NewManager()
	m.Create("acme", "Acme", "key-1", tenant.PlanFree)
	m.Suspend("acme")

	_, err := m.Authenticate("key-1")
	if err == nil {
		t.Error("suspended tenant should not authenticate")
	}
}

func TestActivateSuspended(t *testing.T) {
	m := tenant.NewManager()
	m.Create("acme", "Acme", "key-1", tenant.PlanFree)
	m.Suspend("acme")
	m.Activate("acme")

	_, err := m.Authenticate("key-1")
	if err != nil {
		t.Error("activated tenant should authenticate")
	}
}

func TestPlanLimits(t *testing.T) {
	free := tenant.PlanLimits(tenant.PlanFree)
	if free.MaxEvents != 10000 {
		t.Errorf("free plan expected 10000 events, got %d", free.MaxEvents)
	}
	if free.MaxRules != 5 {
		t.Errorf("free plan expected 5 rules, got %d", free.MaxRules)
	}

	enterprise := tenant.PlanLimits(tenant.PlanEnterprise)
	if enterprise.MaxEvents != -1 {
		t.Error("enterprise should be unlimited")
	}
}

func TestCheckLimit(t *testing.T) {
	m := tenant.NewManager()
	m.Create("acme", "Acme", "key", tenant.PlanFree)

	// Under limit
	err := m.CheckLimit("acme", "events")
	if err != nil {
		t.Error("should be under limit")
	}

	// Exceed limit
	for i := 0; i < 10001; i++ {
		m.RecordEvent("acme")
	}
	err = m.CheckLimit("acme", "events")
	if err == nil {
		t.Error("should exceed event limit")
	}
}

func TestRecordUsage(t *testing.T) {
	m := tenant.NewManager()
	m.Create("acme", "Acme", "key", tenant.PlanFree)

	m.RecordEvent("acme")
	m.RecordEvent("acme")
	m.RecordHeal("acme")

	events, heals, _, _ := m.Usage("acme")
	if events != 2 {
		t.Errorf("expected 2 events, got %d", events)
	}
	if heals != 1 {
		t.Errorf("expected 1 heal, got %d", heals)
	}
}

func TestDelete(t *testing.T) {
	m := tenant.NewManager()
	m.Create("acme", "Acme", "key", tenant.PlanFree)
	m.Delete("acme")

	_, ok := m.Get("acme")
	if ok {
		t.Error("deleted tenant should not be found")
	}
	_, err := m.Authenticate("key")
	if err == nil {
		t.Error("deleted tenant key should not authenticate")
	}
}

func TestAllAndCount(t *testing.T) {
	m := tenant.NewManager()
	m.Create("t1", "One", "k1", tenant.PlanFree)
	m.Create("t2", "Two", "k2", tenant.PlanPro)
	m.Create("t3", "Three", "k3", tenant.PlanBusiness)

	if m.Count() != 3 {
		t.Errorf("expected 3, got %d", m.Count())
	}

	all := m.All()
	if len(all) != 3 {
		t.Errorf("expected 3 tenants, got %d", len(all))
	}
}

func TestMultiTenantIsolation(t *testing.T) {
	m := tenant.NewManager()
	m.Create("company-a", "Company A", "key-a", tenant.PlanPro)
	m.Create("company-b", "Company B", "key-b", tenant.PlanFree)

	// Each tenant has independent counters
	m.RecordEvent("company-a")
	m.RecordEvent("company-a")
	m.RecordEvent("company-a")
	m.RecordEvent("company-b")

	evA, _, _, _ := m.Usage("company-a")
	evB, _, _, _ := m.Usage("company-b")

	if evA != 3 {
		t.Errorf("Company A should have 3 events, got %d", evA)
	}
	if evB != 1 {
		t.Errorf("Company B should have 1 event, got %d", evB)
	}

	// Suspending A doesn't affect B
	m.Suspend("company-a")
	_, err := m.Authenticate("key-b")
	if err != nil {
		t.Error("Company B should still work when A is suspended")
	}
}

func TestGetNonexistent(t *testing.T) {
	m := tenant.NewManager()
	_, ok := m.Get("nobody")
	if ok {
		t.Error("nonexistent tenant should return false")
	}
}

func TestUsageNonexistent(t *testing.T) {
	m := tenant.NewManager()
	_, _, _, err := m.Usage("nobody")
	if err == nil {
		t.Error("nonexistent tenant should error")
	}
}
