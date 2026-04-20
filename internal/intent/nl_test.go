package intent

import (
	"testing"
)

func TestCompile_ProtectCheckout(t *testing.T) {
	r := Compile("Protect checkout and payments at all costs.")
	if len(r.Intents) != 1 {
		t.Fatalf("want 1 intent, got %d (unknowns: %v)", len(r.Intents), r.Unknowns)
	}
	it := r.Intents[0]
	if it.Name != "protect-checkout" {
		t.Errorf("wrong preset used; got %q", it.Name)
	}
	services := map[string]bool{}
	for _, g := range it.Goals {
		services[g.Service] = true
	}
	if !services["checkout"] || !services["payments"] {
		t.Errorf("want checkout+payments in goals; got %v", services)
	}
}

func TestCompile_NeverDropJobs(t *testing.T) {
	cases := []string{
		"Never drop jobs.",
		"Never drop anything in orders",
		"Do not lose work from queue",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			r := Compile(in)
			if len(r.Intents) != 1 {
				t.Fatalf("%q: want 1 intent, got %d (unknowns: %v)", in, len(r.Intents), r.Unknowns)
			}
			if r.Intents[0].Name != "never-drop-jobs" {
				t.Errorf("%q: wrong preset; got %q", in, r.Intents[0].Name)
			}
		})
	}
}

func TestCompile_Availability(t *testing.T) {
	r := Compile("Keep api available under degradation.")
	if len(r.Intents) != 1 {
		t.Fatalf("want 1 intent, got %d (unknowns: %v)", len(r.Intents), r.Unknowns)
	}
	if r.Intents[0].Name != "available-under-degradation" {
		t.Errorf("wrong preset; got %q", r.Intents[0].Name)
	}
}

func TestCompile_CostCeiling(t *testing.T) {
	cases := map[string]float64{
		"Keep cost under 12 dollars per hour":           12,
		"cap spend at $25/hr":                           25,
		"do not go over 7 per hour":                     7,
		"cost below 40 usd per hour":                    40,
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			r := Compile(in)
			if len(r.Intents) != 1 {
				t.Fatalf("%q: want 1 intent, got %d (unknowns: %v)", in, len(r.Intents), r.Unknowns)
			}
			it := r.Intents[0]
			if it.Name != "cost-ceiling" {
				t.Errorf("wrong preset; got %q", it.Name)
			}
			if it.Goals[0].Target != want {
				t.Errorf("want target %v, got %v", want, it.Goals[0].Target)
			}
		})
	}
}

func TestCompile_Latency(t *testing.T) {
	cases := []struct {
		in      string
		service string
		target  float64
	}{
		{"Keep catalog latency under 500 ms", "catalog", 500},
		{"latency under 300ms on api", "api", 300},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			r := Compile(c.in)
			if len(r.Intents) != 1 {
				t.Fatalf("%q: want 1 intent, got %d (unknowns: %v)", c.in, len(r.Intents), r.Unknowns)
			}
			g := r.Intents[0].Goals[0]
			if g.Service != c.service || g.Target != c.target || g.Kind != LatencyUnder {
				t.Errorf("wrong goal: %+v", g)
			}
		})
	}
}

func TestCompile_MultiSentence(t *testing.T) {
	input := `
		Protect checkout and payments at all costs.
		Never drop jobs in orders.
		Keep cost under 12 dollars per hour.
	`
	r := Compile(input)
	if len(r.Intents) != 3 {
		t.Fatalf("want 3 intents, got %d (unknowns: %v)", len(r.Intents), r.Unknowns)
	}
	names := map[string]bool{}
	for _, it := range r.Intents {
		names[it.Name] = true
	}
	for _, want := range []string{"protect-checkout", "never-drop-jobs", "cost-ceiling"} {
		if !names[want] {
			t.Errorf("missing %q in output; got %v", want, names)
		}
	}
}

func TestCompile_UnknownGoesToUnknownsBucket(t *testing.T) {
	r := Compile("Make the system faster somehow.")
	if len(r.Intents) != 0 {
		t.Errorf("want 0 intents for ambiguous input; got %d", len(r.Intents))
	}
	if len(r.Unknowns) != 1 {
		t.Errorf("want 1 unknown; got %d", len(r.Unknowns))
	}
}

func TestCompile_EmptyInput(t *testing.T) {
	r := Compile("")
	if len(r.Intents) != 0 || len(r.Unknowns) != 0 {
		t.Errorf("empty input should yield empty result; got %+v", r)
	}
}
