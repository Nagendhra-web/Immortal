// Package narrator turns structured incident data into human-readable
// explanations. Template-based and deterministic. No LLM required.
//
// The narrator produces three output shapes:
//
//	Brief(inc)     one-sentence summary ("Checkout latency doubled for 4 min.")
//	Explain(inc)   short paragraph suitable for a Slack message
//	Markdown(inc)  rich Markdown incident report with sections
//
// If an operator wants richer prose, Explain output can be fed to an LLM
// downstream. Everything the narrator produces is already deterministic
// enough to ship in a postmortem.
package narrator

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Event summarises the triggering symptom.
type Event struct {
	Service   string
	Kind      string    // "latency_spike", "error_storm", "cpu_anomaly", "queue_backpressure", etc.
	Metric    string    // human-readable metric name ("p99 latency", "error rate", "CPU")
	Baseline  float64   // normal value
	Peak      float64   // peak value during the incident
	Unit      string    // "ms", "req/s", "%", ""
	StartedAt time.Time
}

// RootCause is the engine's (causal) verdict on why the incident happened.
type RootCause struct {
	Driver string   // the underlying cause ("retry storm from service A")
	Chain  []string // cascading intermediate steps
	Score  float64  // 0.0-1.0 confidence
}

// Action is one healing action taken (or proposed).
type Action struct {
	At     time.Time
	Tool   string // "restart_service", "scale_out", "shed_load", etc.
	Target string
	Result string // "ok", "failed", "skipped"
}

// Incident bundles everything the narrator needs.
type Incident struct {
	ID         string
	Event      Event
	Root       RootCause
	Actions    []Action
	ResolvedAt *time.Time
}

// Narrator is stateless. Construct once and reuse.
type Narrator struct{}

// New returns a ready-to-use Narrator.
func New() *Narrator { return &Narrator{} }

// Brief returns a single-sentence summary, no punctuation-heavy detail.
func (n *Narrator) Brief(inc Incident) string {
	ev := inc.Event
	delta := changeWord(ev.Baseline, ev.Peak)
	dur := "ongoing"
	if inc.ResolvedAt != nil {
		dur = humanDuration(inc.ResolvedAt.Sub(ev.StartedAt))
	}
	return capitalize(fmt.Sprintf("%s on %s %s from %s to %s (%s).",
		friendlyKind(ev.Kind),
		safeService(ev.Service),
		delta,
		fmtVal(ev.Baseline, ev.Unit),
		fmtVal(ev.Peak, ev.Unit),
		dur,
	))
}

// Explain returns a short paragraph: symptom, cause, what was done,
// outcome. Suitable for a Slack message or an email opener.
func (n *Narrator) Explain(inc Incident) string {
	var b strings.Builder
	b.WriteString(n.Brief(inc))
	b.WriteString(" ")

	if inc.Root.Driver != "" {
		b.WriteString("Root cause: ")
		b.WriteString(inc.Root.Driver)
		if len(inc.Root.Chain) > 0 {
			b.WriteString(" (cascaded through ")
			b.WriteString(strings.Join(inc.Root.Chain, " -> "))
			b.WriteString(")")
		}
		b.WriteString(". ")
	}

	if len(inc.Actions) > 0 {
		okActs := filterActions(inc.Actions, "ok")
		failedActs := filterActions(inc.Actions, "failed")
		if len(okActs) > 0 {
			b.WriteString("Actions taken: ")
			b.WriteString(listActions(okActs))
			b.WriteString(". ")
		}
		if len(failedActs) > 0 {
			b.WriteString("Failed attempts: ")
			b.WriteString(listActions(failedActs))
			b.WriteString(". ")
		}
	}

	if inc.ResolvedAt != nil {
		b.WriteString("Resolved.")
	} else {
		b.WriteString("Still in progress.")
	}
	return strings.TrimSpace(b.String())
}

// Markdown returns a full-blown incident report in Markdown.
func (n *Narrator) Markdown(inc Incident) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Incident %s\n\n", incidentID(inc))
	fmt.Fprintf(&b, "> %s\n\n", n.Brief(inc))

	b.WriteString("## Symptom\n\n")
	fmt.Fprintf(&b, "- Service: **%s**\n", safeService(inc.Event.Service))
	fmt.Fprintf(&b, "- Metric: %s\n", safeMetric(inc.Event.Metric, inc.Event.Kind))
	fmt.Fprintf(&b, "- Normal: %s\n", fmtVal(inc.Event.Baseline, inc.Event.Unit))
	fmt.Fprintf(&b, "- Peak: %s\n", fmtVal(inc.Event.Peak, inc.Event.Unit))
	fmt.Fprintf(&b, "- Started: %s\n", inc.Event.StartedAt.UTC().Format(time.RFC3339))
	if inc.ResolvedAt != nil {
		fmt.Fprintf(&b, "- Resolved: %s (after %s)\n",
			inc.ResolvedAt.UTC().Format(time.RFC3339),
			humanDuration(inc.ResolvedAt.Sub(inc.Event.StartedAt)))
	} else {
		b.WriteString("- Resolved: pending\n")
	}
	b.WriteString("\n")

	b.WriteString("## Root cause\n\n")
	if inc.Root.Driver == "" {
		b.WriteString("_Not yet identified._\n\n")
	} else {
		fmt.Fprintf(&b, "**%s** (confidence %.0f%%)\n\n", inc.Root.Driver, inc.Root.Score*100)
		if len(inc.Root.Chain) > 0 {
			b.WriteString("Cascade:\n")
			for _, step := range inc.Root.Chain {
				fmt.Fprintf(&b, "1. %s\n", step)
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("## Healing actions\n\n")
	if len(inc.Actions) == 0 {
		b.WriteString("_No actions recorded._\n\n")
	} else {
		sorted := append([]Action(nil), inc.Actions...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].At.Before(sorted[j].At) })
		b.WriteString("| Time | Tool | Target | Result |\n| --- | --- | --- | --- |\n")
		for _, a := range sorted {
			fmt.Fprintf(&b, "| %s | `%s` | %s | %s |\n",
				a.At.UTC().Format("15:04:05"),
				a.Tool,
				a.Target,
				resultBadge(a.Result))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Outcome\n\n")
	if inc.ResolvedAt == nil {
		b.WriteString("Incident is still being processed.\n")
	} else {
		okCount := 0
		failCount := 0
		for _, a := range inc.Actions {
			switch a.Result {
			case "ok":
				okCount++
			case "failed":
				failCount++
			}
		}
		fmt.Fprintf(&b, "Resolved after %s. %d successful actions, %d failed. ",
			humanDuration(inc.ResolvedAt.Sub(inc.Event.StartedAt)), okCount, failCount)
		if failCount == 0 {
			b.WriteString("Clean recovery.\n")
		} else {
			b.WriteString("Partial degradation recovered.\n")
		}
	}
	return b.String()
}

// ── helpers ──────────────────────────────────────────────────────────────

func friendlyKind(k string) string {
	switch k {
	case "latency_spike":
		return "Latency spike"
	case "error_storm":
		return "Error storm"
	case "cpu_anomaly":
		return "CPU anomaly"
	case "memory_pressure":
		return "Memory pressure"
	case "queue_backpressure":
		return "Queue backpressure"
	case "cache_miss_spike":
		return "Cache miss spike"
	case "retry_storm":
		return "Retry storm"
	case "cascading_failure":
		return "Cascading failure"
	case "":
		return "Anomaly"
	default:
		// Replace underscores and title-case the first letter.
		s := strings.ReplaceAll(k, "_", " ")
		return capitalize(s)
	}
}

func changeWord(base, peak float64) string {
	if base == 0 {
		return "appeared"
	}
	ratio := peak / base
	switch {
	case ratio >= 5:
		return "surged over 5x"
	case ratio >= 2:
		return fmt.Sprintf("jumped %0.1fx", ratio)
	case ratio >= 1.25:
		return fmt.Sprintf("rose %0.0f%%", (ratio-1)*100)
	case ratio >= 1:
		return fmt.Sprintf("drifted up %0.0f%%", (ratio-1)*100)
	case ratio < 0.3:
		return fmt.Sprintf("collapsed to %0.0f%% of normal", ratio*100)
	default:
		return fmt.Sprintf("dropped %0.0f%%", (1-ratio)*100)
	}
}

func fmtVal(v float64, unit string) string {
	if unit == "" {
		return fmt.Sprintf("%.2f", v)
	}
	if unit == "%" {
		return fmt.Sprintf("%.1f%%", v*100)
	}
	return fmt.Sprintf("%.1f %s", v, unit)
}

func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

func listActions(actions []Action) string {
	parts := make([]string, 0, len(actions))
	for _, a := range actions {
		if a.Target != "" {
			parts = append(parts, fmt.Sprintf("%s(%s)", a.Tool, a.Target))
		} else {
			parts = append(parts, a.Tool)
		}
	}
	return strings.Join(parts, ", ")
}

func filterActions(actions []Action, result string) []Action {
	var out []Action
	for _, a := range actions {
		if a.Result == result {
			out = append(out, a)
		}
	}
	return out
}

func safeService(s string) string {
	if s == "" {
		return "the system"
	}
	return s
}

func safeMetric(m, kind string) string {
	if m != "" {
		return m
	}
	switch kind {
	case "latency_spike":
		return "p99 latency"
	case "error_storm":
		return "error rate"
	case "cpu_anomaly":
		return "CPU utilization"
	default:
		return "primary metric"
	}
}

func resultBadge(r string) string {
	switch r {
	case "ok":
		return "ok"
	case "failed":
		return "**failed**"
	case "skipped":
		return "_skipped_"
	default:
		if r == "" {
			return "-"
		}
		return r
	}
}

func incidentID(inc Incident) string {
	if inc.ID != "" {
		return inc.ID
	}
	return inc.Event.StartedAt.UTC().Format("20060102T150405Z")
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
