package economic

import (
	"testing"
)

func TestCatalog_SetAndGet(t *testing.T) {
	cat := NewCatalog()
	cat.SetServiceValue(ServiceValue{Service: "checkout", USDPerMinute: 2000, DependentFactor: 1.0})
	cat.SetActionCost(ActionCost{Type: "restart", InfraCostUSD: 5, EngineeringCostUSD: 10, ExpectedDowntimeSec: 30, SuccessProbability: 0.9})

	sv, ok := cat.ServiceValue("checkout")
	if !ok {
		t.Fatal("expected checkout to be found")
	}
	if sv.USDPerMinute != 2000 {
		t.Errorf("expected 2000, got %f", sv.USDPerMinute)
	}

	ac, ok := cat.ActionCost("restart")
	if !ok {
		t.Fatal("expected restart to be found")
	}
	if ac.SuccessProbability != 0.9 {
		t.Errorf("expected 0.9, got %f", ac.SuccessProbability)
	}
}

func TestCatalog_MissingServiceReturnsFalse(t *testing.T) {
	cat := NewCatalog()
	_, ok := cat.ServiceValue("nonexistent")
	if ok {
		t.Fatal("expected false for missing service")
	}
	warnings := cat.Warnings()
	if len(warnings) == 0 {
		t.Fatal("expected a warning for missing service lookup")
	}
}

func TestCatalog_DefaultSuccessProbability(t *testing.T) {
	cat := NewCatalog()
	// SuccessProbability left at zero — should default to 1.0
	cat.SetActionCost(ActionCost{Type: "scale", InfraCostUSD: 50})

	ac, ok := cat.ActionCost("scale")
	if !ok {
		t.Fatal("expected scale to be found")
	}
	if ac.SuccessProbability != 1.0 {
		t.Errorf("expected default SuccessProbability 1.0, got %f", ac.SuccessProbability)
	}
}
