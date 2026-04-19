package narrator

import (
	"strings"
	"testing"
)

func sampleVerdict() Verdict {
	return Verdict{
		Cause:    "A retry storm from the API exhausted Postgres connections.",
		Evidence: []string{"api retry rate 0.74/req", "postgres connection pool at 100%", "checkout p99 jumped from 80 ms to 310 ms"},
		Action: []string{
			"reduced the retry rate on api from 0.74 to 0.12",
			"added 5s exponential backoff",
			"cleared idle postgres connections",
		},
		Outcome:    "Error rate dropped from 18% to 0.3% in 40 seconds and checkout p99 returned to 95 ms.",
		Confidence: 0.92,
	}
}

func TestVerdict_Render_Shape(t *testing.T) {
	got := sampleVerdict().Render()
	mustContain(t, got, "retry storm from the API")
	mustContain(t, got, "I reduced")
	mustContain(t, got, ", and cleared idle")
	mustContain(t, got, "Error rate dropped from 18%")
	mustContain(t, got, "Confidence: 92%")
}

func TestVerdict_Markdown_Sections(t *testing.T) {
	got := sampleVerdict().Markdown()
	for _, section := range []string{
		"### What happened",
		"### Evidence",
		"### What I did",
		"### Outcome",
		"### Confidence",
	} {
		if !strings.Contains(got, section) {
			t.Errorf("missing section %q", section)
		}
	}
	if !strings.Contains(got, "- api retry rate 0.74/req") {
		t.Errorf("evidence should render as bullet list; got:\n%s", got)
	}
	if !strings.Contains(got, "1. Reduced") {
		t.Errorf("actions should render as numbered, capitalized list; got:\n%s", got)
	}
}

func TestVerdict_Brief_OneLine(t *testing.T) {
	got := sampleVerdict().Brief()
	if strings.Count(got, "\n") > 0 {
		t.Errorf("brief should be one line; got:\n%s", got)
	}
	mustContain(t, got, "retry storm")
	mustContain(t, got, "92% confident")
}

func TestVerdict_EmptyFieldsDegradeGracefully(t *testing.T) {
	v := Verdict{Confidence: 0.5}
	md := v.Markdown()
	mustContain(t, md, "_No confirmed cause yet._")
	mustContain(t, md, "_No supporting evidence recorded._")
	mustContain(t, md, "_No actions taken._")
	mustContain(t, md, "_Pending._")
	mustContain(t, md, "50%")
}
