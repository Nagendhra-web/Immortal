package narrator

import (
	"strings"
	"testing"
	"time"
)

func sampleIncident() Incident {
	start := time.Date(2026, 4, 19, 14, 3, 0, 0, time.UTC)
	end := start.Add(4*time.Minute + 12*time.Second)
	return Incident{
		ID: "inc-42",
		Event: Event{
			Service:   "checkout",
			Kind:      "latency_spike",
			Metric:    "p99 latency",
			Baseline:  80,
			Peak:      310,
			Unit:      "ms",
			StartedAt: start,
		},
		Root: RootCause{
			Driver: "retry storm from payments service",
			Chain:  []string{"payments retries", "postgres connection pool exhausted", "checkout requests queued"},
			Score:  0.87,
		},
		Actions: []Action{
			{At: start.Add(30 * time.Second), Tool: "throttle", Target: "payments", Result: "ok"},
			{At: start.Add(90 * time.Second), Tool: "scale_out", Target: "postgres", Result: "ok"},
			{At: start.Add(140 * time.Second), Tool: "warm_cache", Target: "catalog", Result: "failed"},
		},
		ResolvedAt: &end,
	}
}

func TestBrief_OneSentence(t *testing.T) {
	n := New()
	out := n.Brief(sampleIncident())
	if out == "" {
		t.Fatal("brief empty")
	}
	if strings.Count(out, ".") == 0 {
		t.Errorf("brief should end with a period: %q", out)
	}
	if !strings.Contains(out, "checkout") {
		t.Errorf("brief must mention service; got %q", out)
	}
	if !strings.Contains(out, "Latency spike") {
		t.Errorf("brief must describe the kind; got %q", out)
	}
}

func TestBrief_HandlesOngoing(t *testing.T) {
	inc := sampleIncident()
	inc.ResolvedAt = nil
	out := New().Brief(inc)
	if !strings.Contains(out, "ongoing") {
		t.Errorf("ongoing incident should be flagged; got %q", out)
	}
}

func TestExplain_IncludesRootCauseAndActions(t *testing.T) {
	out := New().Explain(sampleIncident())
	mustContain(t, out, "retry storm from payments service")
	mustContain(t, out, "postgres")
	mustContain(t, out, "throttle")
	mustContain(t, out, "scale_out")
	mustContain(t, out, "Failed attempts")
	mustContain(t, out, "warm_cache")
	mustContain(t, out, "Resolved.")
}

func TestExplain_NoActions(t *testing.T) {
	inc := sampleIncident()
	inc.Actions = nil
	out := New().Explain(inc)
	if strings.Contains(out, "Actions taken") {
		t.Errorf("should not mention actions when none exist: %q", out)
	}
}

func TestMarkdown_Structure(t *testing.T) {
	out := New().Markdown(sampleIncident())
	for _, section := range []string{"# Incident", "## Symptom", "## Root cause", "## Healing actions", "## Outcome"} {
		if !strings.Contains(out, section) {
			t.Errorf("missing section %q in markdown", section)
		}
	}
	if !strings.Contains(out, "| Time | Tool | Target | Result |") {
		t.Errorf("actions should render as a table")
	}
	if !strings.Contains(out, "**failed**") {
		t.Errorf("failed action should be bold")
	}
	if !strings.Contains(out, "confidence 87%") {
		t.Errorf("root cause confidence should be formatted; got:\n%s", out)
	}
}

func TestChangeWord(t *testing.T) {
	cases := []struct {
		base, peak float64
		want       string
	}{
		{80, 310, "jumped"},
		{10, 60, "surged"},
		{100, 110, "drifted up"},
		{100, 40, "dropped"},
		{100, 25, "collapsed"},
	}
	for _, c := range cases {
		got := changeWord(c.base, c.peak)
		if !strings.Contains(got, c.want) {
			t.Errorf("changeWord(%v, %v) = %q; want substring %q", c.base, c.peak, got, c.want)
		}
	}
}

func TestFriendlyKind_FallsBackToHumanize(t *testing.T) {
	if got := friendlyKind("disk_full"); got != "Disk full" {
		t.Errorf("unknown kind should be humanized; got %q", got)
	}
	if got := friendlyKind(""); got != "Anomaly" {
		t.Errorf("empty kind should be Anomaly; got %q", got)
	}
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("output missing %q:\n%s", needle, haystack)
	}
}
