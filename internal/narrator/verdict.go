package narrator

import (
	"fmt"
	"strings"
)

// Verdict is the "holy shit moment" rendering. It packages five fields an
// operator cares about in the exact order a reader's brain wants them:
//
//	Cause      why did this happen
//	Evidence   what do you observe that supports that
//	Action     what did you do about it
//	Outcome    what changed as a result
//	Confidence how sure are you
//
// This is the structure users cite when they ask "why would I trust this
// system?". It is the single most important UX asset the narrator produces.
type Verdict struct {
	Cause      string   // one sentence: the underlying driver
	Evidence   []string // observable facts that support the cause
	Action     []string // what the engine did (in order)
	Outcome    string   // the measurable result
	Confidence float64  // 0.0 - 1.0
}

// Render returns a human-readable block formatted for Slack, a terminal,
// or a Markdown card. Fits in a message preview.
func (v Verdict) Render() string {
	var b strings.Builder
	if v.Cause != "" {
		b.WriteString(v.Cause)
		b.WriteString("\n")
	}
	for i, a := range v.Action {
		if i == 0 {
			b.WriteString("I ")
		} else if i == len(v.Action)-1 && len(v.Action) > 1 {
			b.WriteString(", and ")
		} else {
			b.WriteString(", ")
		}
		b.WriteString(a)
	}
	if len(v.Action) > 0 {
		b.WriteString(".\n")
	}
	if v.Outcome != "" {
		b.WriteString(v.Outcome)
		b.WriteString("\n")
	}
	if v.Confidence > 0 {
		fmt.Fprintf(&b, "Confidence: %.0f%% this resolves the root cause.", v.Confidence*100)
	}
	return strings.TrimSpace(b.String())
}

// Markdown formats the Verdict as a structured Markdown block with the five
// labelled sections. Useful for the dashboard's incident detail sheet.
func (v Verdict) Markdown() string {
	var b strings.Builder
	b.WriteString("### What happened\n\n")
	if v.Cause == "" {
		b.WriteString("_No confirmed cause yet._\n\n")
	} else {
		fmt.Fprintf(&b, "%s\n\n", v.Cause)
	}

	b.WriteString("### Evidence\n\n")
	if len(v.Evidence) == 0 {
		b.WriteString("_No supporting evidence recorded._\n\n")
	} else {
		for _, e := range v.Evidence {
			fmt.Fprintf(&b, "- %s\n", e)
		}
		b.WriteString("\n")
	}

	b.WriteString("### What I did\n\n")
	if len(v.Action) == 0 {
		b.WriteString("_No actions taken._\n\n")
	} else {
		for i, a := range v.Action {
			fmt.Fprintf(&b, "%d. %s\n", i+1, capitalize(a))
		}
		b.WriteString("\n")
	}

	b.WriteString("### Outcome\n\n")
	if v.Outcome == "" {
		b.WriteString("_Pending._\n\n")
	} else {
		fmt.Fprintf(&b, "%s\n\n", v.Outcome)
	}

	b.WriteString("### Confidence\n\n")
	fmt.Fprintf(&b, "%.0f%% this resolves the root cause.\n", v.Confidence*100)
	return b.String()
}

// Brief returns a one-line summary suitable for a chat notification.
func (v Verdict) Brief() string {
	parts := []string{}
	if v.Cause != "" {
		parts = append(parts, strings.TrimSuffix(v.Cause, "."))
	}
	if v.Outcome != "" {
		parts = append(parts, strings.TrimSuffix(v.Outcome, "."))
	}
	if v.Confidence > 0 {
		parts = append(parts, fmt.Sprintf("%.0f%% confident", v.Confidence*100))
	}
	return strings.Join(parts, ". ") + "."
}
