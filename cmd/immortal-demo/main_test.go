package main

import (
	"strings"
	"testing"
	"time"
)

// TestDemo_QuietScenario_Completes runs the quiet scenario and verifies:
//   - Run returns no error
//   - The audit chain is verified (no integrity issues)
func TestDemo_QuietScenario_Completes(t *testing.T) {
	cfg := Config{
		Duration: 2 * time.Second,
		Scenario: "quiet",
		Quiet:    true,
		NoColor:  true,
	}

	report, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if report == nil {
		t.Fatal("Run returned nil report")
	}
	if !report.AuditChainVerified {
		t.Errorf("audit chain not verified: label=%q", report.AuditChainLabel)
	}
	if !strings.Contains(report.AuditChainLabel, "verified") {
		t.Errorf("expected AuditChainLabel to contain 'verified', got %q", report.AuditChainLabel)
	}
	if len(report.ScenariosRun) == 0 {
		t.Error("expected at least one scenario in ScenariosRun")
	}
}
