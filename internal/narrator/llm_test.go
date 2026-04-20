package narrator

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeAnalyzer struct {
	enabled bool
	resp    *AnalyzerResponse
	err     error
	called  int
}

func (f *fakeAnalyzer) IsEnabled() bool { return f.enabled }
func (f *fakeAnalyzer) Analyze(sys, user string) (*AnalyzerResponse, error) {
	f.called++
	return f.resp, f.err
}

func sampleIncForLLM() Incident {
	inc := sampleIncident()
	return inc
}
func sampleVerdictForLLM() Verdict { return sampleVerdict() }

func TestEnrich_NilAnalyzer_FallsBackToDeterministic(t *testing.T) {
	p := Enrich(context.Background(), sampleIncForLLM(), sampleVerdictForLLM(), nil)
	if p.GeneratedBy != "deterministic" {
		t.Errorf("want deterministic marker; got %q", p.GeneratedBy)
	}
	if p.TLDR == "" {
		t.Errorf("TLDR should be populated from Verdict.Brief")
	}
}

func TestEnrich_DisabledAnalyzer_FallsBack(t *testing.T) {
	az := &fakeAnalyzer{enabled: false}
	p := Enrich(context.Background(), sampleIncForLLM(), sampleVerdictForLLM(), az)
	if az.called != 0 {
		t.Fatalf("Analyze should not be called when disabled")
	}
	if p.GeneratedBy != "deterministic" {
		t.Errorf("want deterministic marker; got %q", p.GeneratedBy)
	}
}

func TestEnrich_AnalyzerError_FallsBack(t *testing.T) {
	az := &fakeAnalyzer{enabled: true, err: errors.New("network down")}
	p := Enrich(context.Background(), sampleIncForLLM(), sampleVerdictForLLM(), az)
	if p.GeneratedBy != "deterministic" {
		t.Errorf("want deterministic marker on error; got %q", p.GeneratedBy)
	}
}

func TestEnrich_ValidJSONResponse_ParsesAndMarksLLM(t *testing.T) {
	json := `{
		"incident_id": "inc-42",
		"tldr": "A retry storm took down checkout for 4 minutes.",
		"executive_summary": "At 14:03 UTC, checkout latency jumped 4x...",
		"what_happened": "Between 14:03 and 14:07, retries...",
		"why_it_happened": "API retry rate crossed 0.3 request threshold...",
		"how_it_was_fixed": "Throttled payments retries, scaled postgres, warmed catalog cache.",
		"prevention": ["add retry budget", "cap postgres pool", "pre-warm catalog on deploy"]
	}`
	az := &fakeAnalyzer{
		enabled: true,
		resp:    &AnalyzerResponse{Content: json, Model: "gpt-oss-20b", TokensUsed: 187},
	}
	p := Enrich(context.Background(), sampleIncForLLM(), sampleVerdictForLLM(), az)
	if !strings.HasPrefix(p.GeneratedBy, "llm:") {
		t.Fatalf("want llm marker; got %q", p.GeneratedBy)
	}
	if !strings.Contains(p.GeneratedBy, "gpt-oss-20b") {
		t.Errorf("llm marker should include model; got %q", p.GeneratedBy)
	}
	if p.TokensUsed != 187 {
		t.Errorf("tokens should be preserved; got %d", p.TokensUsed)
	}
	if !strings.Contains(p.TLDR, "retry storm") {
		t.Errorf("TLDR not parsed; got %q", p.TLDR)
	}
	if len(p.Prevention) != 3 {
		t.Errorf("prevention should have 3 items; got %d", len(p.Prevention))
	}
}

func TestEnrich_ConfidenceInheritedFromVerdictWhenZero(t *testing.T) {
	json := `{"tldr":"x","executive_summary":"y"}` // no confidence field
	az := &fakeAnalyzer{enabled: true, resp: &AnalyzerResponse{Content: json, Model: "m"}}
	v := sampleVerdictForLLM()
	v.Confidence = 0.77
	p := Enrich(context.Background(), sampleIncForLLM(), v, az)
	if p.Confidence != 0.77 {
		t.Errorf("confidence should inherit from verdict; got %v", p.Confidence)
	}
}

func TestEnrich_MalformedJSON_FallsBackButMarks(t *testing.T) {
	az := &fakeAnalyzer{enabled: true, resp: &AnalyzerResponse{Content: "not json at all"}}
	p := Enrich(context.Background(), sampleIncForLLM(), sampleVerdictForLLM(), az)
	if !strings.Contains(p.GeneratedBy, "unparseable") {
		t.Errorf("should mark unparseable fallback; got %q", p.GeneratedBy)
	}
}

func TestEnrich_StripsMarkdownFence(t *testing.T) {
	json := "```json\n{\"tldr\":\"wrapped in fence\"}\n```"
	az := &fakeAnalyzer{enabled: true, resp: &AnalyzerResponse{Content: json, Model: "m"}}
	p := Enrich(context.Background(), sampleIncForLLM(), sampleVerdictForLLM(), az)
	if p.TLDR != "wrapped in fence" {
		t.Errorf("fence stripping failed; got TLDR=%q", p.TLDR)
	}
}

func TestPostmortem_Markdown_Structure(t *testing.T) {
	p := Postmortem{
		IncidentID:       "inc-1",
		TLDR:             "x",
		ExecutiveSummary: "a",
		WhatHappened:     "b",
		WhyItHappened:    "c",
		HowItWasFixed:    "d",
		Prevention:       []string{"one", "two"},
		Confidence:       0.9,
		GeneratedBy:      "llm:gpt-oss-20b",
		TokensUsed:       150,
	}
	md := p.Markdown()
	for _, section := range []string{"# Postmortem", "## Executive summary", "## What happened", "## Why it happened", "## How it was fixed", "## Prevention", "- one", "- two", "90%", "Tokens used: 150"} {
		if !strings.Contains(md, section) {
			t.Errorf("missing %q in markdown:\n%s", section, md)
		}
	}
}
