// Package nlplan provides a natural-language-to-formal-plan compiler.
// grammar.go implements a deterministic regex/tokenizer-based fallback parser
// that handles canonical English sentences without requiring an LLM.
package nlplan

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Nagendhra-web/Immortal/internal/formal"
)

// synonymTable maps variant verbs to their canonical form.
var synonymTable = map[string]string{
	"reboot":    "restart",
	"cycle":     "restart",
	"resize":    "scale",
	"fail-over": "failover",
	"revert":    "rollback",
	"roll-back": "rollback",
}

// Compiled regexes for each canonical sentence pattern.
// All are case-insensitive via (?i) flag.
var (
	reRestart   = regexp.MustCompile(`(?i)^(?:restart|reboot|cycle)\s+(?:the\s+)?(\S+?)[,.]?$`)
	reScale     = regexp.MustCompile(`(?i)^(?:scale|resize)\s+(?:the\s+)?(\S+?)\s+to\s+(\d+)[,.]?$`)
	reFailover  = regexp.MustCompile(`(?i)^(?:failover|fail-over)\s+(?:the\s+)?(\S+?)[,.]?$`)
	reRollback  = regexp.MustCompile(`(?i)^(?:rollback|roll-back|revert)\s+(?:the\s+)?(\S+?)[,.]?$`)
	reAtLeastN  = regexp.MustCompile(`(?i)^keep\s+at\s+least\s+(\d+)\s+services?\s+healthy[,.]?$`)
	reNeverDown = regexp.MustCompile(`(?i)^never\s+let\s+(?:the\s+)?(\S+?)\s+go\s+down[,.]?$`)
	reMinRep    = regexp.MustCompile(`(?i)^(\S+?)\s+always\s+has\s+at\s+least\s+(\d+)\s+replicas?[,.]?$`)
)

// grammarResult holds what the grammar parser extracted from one pass.
type grammarResult struct {
	steps      []formal.Action
	invariants []formal.Invariant
	trace      []Translation
	warnings   []string
}

// parseGrammar splits text on sentence boundaries and tries each pattern.
func parseGrammar(text string, services map[string]bool) grammarResult {
	sentences := splitSentences(text)
	result := grammarResult{}

	for _, raw := range sentences {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}

		// Normalise synonyms: replace variant verb at word boundary.
		normalised := normaliseSynonyms(s)

		// Try step patterns first.
		if m := reRestart.FindStringSubmatch(normalised); m != nil {
			svc := strings.ToLower(m[1])
			warnIfUnknown(svc, services, &result.warnings)
			step := makeSetHealthy(svc, true, "restart")
			result.steps = append(result.steps, step)
			result.trace = append(result.trace, Translation{
				Span:        s,
				Interpreted: fmt.Sprintf("set_healthy(%s, true)", svc),
			})
			continue
		}
		if m := reScale.FindStringSubmatch(normalised); m != nil {
			svc := strings.ToLower(m[1])
			n, _ := strconv.Atoi(m[2])
			warnIfUnknown(svc, services, &result.warnings)
			step := makeSetReplicas(svc, n)
			result.steps = append(result.steps, step)
			result.trace = append(result.trace, Translation{
				Span:        s,
				Interpreted: fmt.Sprintf("set_replicas(%s, %d)", svc, n),
			})
			continue
		}
		if m := reFailover.FindStringSubmatch(normalised); m != nil {
			svc := strings.ToLower(m[1])
			warnIfUnknown(svc, services, &result.warnings)
			step := makeSetHealthy(svc, true, "failover")
			result.steps = append(result.steps, step)
			result.trace = append(result.trace, Translation{
				Span:        s,
				Interpreted: fmt.Sprintf("set_healthy(%s, true) [failover]", svc),
			})
			continue
		}
		if m := reRollback.FindStringSubmatch(normalised); m != nil {
			svc := strings.ToLower(m[1])
			warnIfUnknown(svc, services, &result.warnings)
			step := makeSetHealthy(svc, true, "rollback")
			result.steps = append(result.steps, step)
			result.trace = append(result.trace, Translation{
				Span:        s,
				Interpreted: fmt.Sprintf("set_healthy(%s, true) [rollback]", svc),
			})
			continue
		}

		// Invariant patterns.
		if m := reAtLeastN.FindStringSubmatch(normalised); m != nil {
			n, _ := strconv.Atoi(m[1])
			inv := formal.AtLeastNHealthy(n)
			result.invariants = append(result.invariants, inv)
			result.trace = append(result.trace, Translation{
				Span:        s,
				Interpreted: fmt.Sprintf("AtLeastNHealthy(%d)", n),
			})
			continue
		}
		if m := reNeverDown.FindStringSubmatch(normalised); m != nil {
			svc := strings.ToLower(m[1])
			warnIfUnknown(svc, services, &result.warnings)
			inv := formal.ServiceAlwaysHealthy(svc)
			result.invariants = append(result.invariants, inv)
			result.trace = append(result.trace, Translation{
				Span:        s,
				Interpreted: fmt.Sprintf("ServiceAlwaysHealthy(%s)", svc),
			})
			continue
		}
		if m := reMinRep.FindStringSubmatch(normalised); m != nil {
			svc := strings.ToLower(m[1])
			n, _ := strconv.Atoi(m[2])
			warnIfUnknown(svc, services, &result.warnings)
			inv := formal.MinReplicas(svc, n)
			result.invariants = append(result.invariants, inv)
			result.trace = append(result.trace, Translation{
				Span:        s,
				Interpreted: fmt.Sprintf("MinReplicas(%s, %d)", svc, n),
			})
			continue
		}

		// Unmatched.
		result.warnings = append(result.warnings, fmt.Sprintf("unrecognised sentence: %q", s))
	}

	return result
}

// splitSentences splits on periods, semicolons, and newlines.
func splitSentences(text string) []string {
	// Replace sentence terminators with a common delimiter.
	r := strings.NewReplacer(
		".", ".|",
		";", ";|",
		"\n", "\n|",
	)
	parts := strings.Split(r.Replace(text), "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(strings.Trim(p, ".;"))
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// normaliseSynonyms replaces known synonym verbs at the start of a sentence.
func normaliseSynonyms(s string) string {
	lower := strings.ToLower(s)
	for variant, canonical := range synonymTable {
		prefix := variant + " "
		if strings.HasPrefix(lower, prefix) {
			return canonical + s[len(variant):]
		}
	}
	return s
}

// warnIfUnknown appends a warning when svc is not in the known service set.
func warnIfUnknown(svc string, services map[string]bool, warnings *[]string) {
	if len(services) > 0 && !services[svc] {
		*warnings = append(*warnings, fmt.Sprintf("service %q not in registry", svc))
	}
}

// makeSetHealthy creates an Action that sets a service's Healthy flag.
func makeSetHealthy(svc string, healthy bool, verb string) formal.Action {
	return formal.Action{
		Name: fmt.Sprintf("%s(%s)", verb, svc),
		Fn: func(w formal.World) formal.World {
			st := w[svc]
			st.Name = svc
			st.Healthy = healthy
			w[svc] = st
			return w
		},
	}
}

// makeSetReplicas creates an Action that sets a service's replica count.
func makeSetReplicas(svc string, n int) formal.Action {
	return formal.Action{
		Name: fmt.Sprintf("set_replicas(%s,%d)", svc, n),
		Fn: func(w formal.World) formal.World {
			st := w[svc]
			st.Name = svc
			st.Replicas = n
			w[svc] = st
			return w
		},
	}
}
