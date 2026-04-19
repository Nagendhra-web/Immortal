package intent

import (
	"strings"
	"testing"
)

func TestProtectCheckout_DefaultService(t *testing.T) {
	i := ProtectCheckout()
	if i.Name != "protect-checkout" {
		t.Errorf("unexpected name %q", i.Name)
	}
	if len(i.Goals) != 3 {
		t.Fatalf("want 3 goals for 1 service; got %d", len(i.Goals))
	}
	for _, g := range i.Goals {
		if g.Service != "checkout" {
			t.Errorf("default service should be checkout; got %q", g.Service)
		}
		if g.Priority != 10 {
			t.Errorf("checkout goals should be priority 10; got %d", g.Priority)
		}
	}
}

func TestProtectCheckout_MultipleServices(t *testing.T) {
	i := ProtectCheckout("checkout", "payments", "billing")
	if len(i.Goals) != 9 {
		t.Fatalf("want 3 goals per service (3*3=9); got %d", len(i.Goals))
	}
	var hasProtect, hasLatency, hasErrors int
	for _, g := range i.Goals {
		switch g.Kind {
		case ProtectService:
			hasProtect++
		case LatencyUnder:
			hasLatency++
		case ErrorRateUnder:
			hasErrors++
		}
	}
	if hasProtect != 3 || hasLatency != 3 || hasErrors != 3 {
		t.Errorf("each service should get all three goal kinds; got protect=%d latency=%d errors=%d", hasProtect, hasLatency, hasErrors)
	}
}

func TestNeverDropJobs(t *testing.T) {
	i := NeverDropJobs("orders", "webhooks")
	if len(i.Goals) != 2 {
		t.Fatalf("want one goal per queue; got %d", len(i.Goals))
	}
	for _, g := range i.Goals {
		if g.Kind != JobsNoDrop {
			t.Errorf("kind must be JobsNoDrop; got %v", g.Kind)
		}
		if g.Target != 0 {
			t.Errorf("target should be 0 drops; got %v", g.Target)
		}
		if g.Priority != 10 {
			t.Errorf("should be high priority")
		}
	}
}

func TestAvailableUnderDegradation(t *testing.T) {
	i := AvailableUnderDegradation("api")
	if len(i.Goals) != 2 {
		t.Fatalf("want availability + error-rate; got %d", len(i.Goals))
	}
	var hasAvail, hasErr bool
	for _, g := range i.Goals {
		if g.Kind == AvailabilityOver && g.Target == 0.99 {
			hasAvail = true
		}
		if g.Kind == ErrorRateUnder && g.Target == 0.05 {
			hasErr = true
		}
	}
	if !hasAvail || !hasErr {
		t.Errorf("missing availability or error-rate goal")
	}
}

func TestCostCeiling(t *testing.T) {
	i := CostCeiling(12.00)
	if len(i.Goals) != 1 {
		t.Fatalf("want 1 goal; got %d", len(i.Goals))
	}
	g := i.Goals[0]
	if g.Kind != CostCap {
		t.Errorf("kind must be CostCap")
	}
	if g.Target != 12.00 {
		t.Errorf("target should be 12.00; got %v", g.Target)
	}
	if g.Priority > 5 {
		t.Errorf("cost ceiling should be low priority (<5); got %d", g.Priority)
	}
}

func TestSummary_HumanReadable(t *testing.T) {
	cases := []struct {
		in   Intent
		want string
	}{
		{ProtectCheckout(), "Protect checkout"},
		{NeverDropJobs(), "Never drop jobs"},
		{AvailableUnderDegradation(), "Available under degradation"},
		{CostCeiling(5), "Cost ceiling"},
		{LowLatency(), "Low latency"},
	}
	for _, c := range cases {
		got := Summary(c.in)
		if !strings.Contains(got, c.want) {
			t.Errorf("Summary(%q) = %q; want to contain %q", c.in.Name, got, c.want)
		}
	}
}
