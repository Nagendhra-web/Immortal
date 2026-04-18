package nlplan

import (
	"testing"
)

func svcSet(svcs ...string) map[string]bool {
	m := make(map[string]bool, len(svcs))
	for _, s := range svcs {
		m[s] = true
	}
	return m
}

func TestGrammar_RestartService(t *testing.T) {
	r := parseGrammar("restart api", svcSet("api"))
	if len(r.steps) != 1 {
		t.Fatalf("want 1 step, got %d", len(r.steps))
	}
	if len(r.warnings) != 0 {
		t.Fatalf("want no warnings, got %v", r.warnings)
	}
	if r.steps[0].Name != "restart(api)" {
		t.Errorf("unexpected step name %q", r.steps[0].Name)
	}
}

func TestGrammar_ScaleToN(t *testing.T) {
	r := parseGrammar("scale database to 5", svcSet("database"))
	if len(r.steps) != 1 {
		t.Fatalf("want 1 step, got %d", len(r.steps))
	}
	if r.steps[0].Name != "set_replicas(database,5)" {
		t.Errorf("unexpected step name %q", r.steps[0].Name)
	}
}

func TestGrammar_KeepAtLeastNHealthy(t *testing.T) {
	r := parseGrammar("keep at least 2 services healthy", nil)
	if len(r.invariants) != 1 {
		t.Fatalf("want 1 invariant, got %d", len(r.invariants))
	}
	if r.invariants[0].Name != "at-least-2-healthy" {
		t.Errorf("unexpected invariant name %q", r.invariants[0].Name)
	}
}

func TestGrammar_NeverLetDown(t *testing.T) {
	r := parseGrammar("never let api go down", svcSet("api"))
	if len(r.invariants) != 1 {
		t.Fatalf("want 1 invariant, got %d", len(r.invariants))
	}
	if r.invariants[0].Name != "service-api-always-healthy" {
		t.Errorf("unexpected invariant name %q", r.invariants[0].Name)
	}
}

func TestGrammar_MinReplicas(t *testing.T) {
	r := parseGrammar("api always has at least 3 replicas", svcSet("api"))
	if len(r.invariants) != 1 {
		t.Fatalf("want 1 invariant, got %d", len(r.invariants))
	}
	if r.invariants[0].Name != "min-replicas-api-3" {
		t.Errorf("unexpected invariant name %q", r.invariants[0].Name)
	}
}

func TestGrammar_MultipleSentences_AllParsed(t *testing.T) {
	text := "restart api. scale database to 3. keep at least 2 services healthy."
	r := parseGrammar(text, svcSet("api", "database"))
	if len(r.steps) != 2 {
		t.Fatalf("want 2 steps, got %d", len(r.steps))
	}
	if len(r.invariants) != 1 {
		t.Fatalf("want 1 invariant, got %d", len(r.invariants))
	}
	if len(r.warnings) != 0 {
		t.Errorf("want no warnings, got %v", r.warnings)
	}
}

func TestGrammar_UnknownSentence_RecordedAsWarning(t *testing.T) {
	r := parseGrammar("do something weird with the cluster", svcSet("api"))
	if len(r.steps) != 0 {
		t.Fatalf("want 0 steps, got %d", len(r.steps))
	}
	if len(r.warnings) != 1 {
		t.Fatalf("want 1 warning, got %d: %v", len(r.warnings), r.warnings)
	}
}

func TestGrammar_Synonyms(t *testing.T) {
	cases := []string{
		"reboot api",
		"cycle api",
		"restart api",
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			r := parseGrammar(tc, svcSet("api"))
			if len(r.steps) != 1 {
				t.Fatalf("want 1 step for %q, got %d", tc, len(r.steps))
			}
			// All should produce a restart-style action (set_healthy).
			if r.steps[0].Name != "restart(api)" {
				t.Errorf("want restart(api) for %q, got %q", tc, r.steps[0].Name)
			}
		})
	}
}
