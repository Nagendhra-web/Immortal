package intent

import (
	"regexp"
	"strconv"
	"strings"
)

// Compile takes a natural-language policy and returns zero or more
// Intents. Each sentence in the input is parsed independently; unknown
// patterns are skipped silently (callers get an Unknowns list for
// diagnostics).
//
// Supported phrasings (case-insensitive):
//
//	"protect checkout [and payments] at all costs"        -> ProtectCheckout(...)
//	"never drop jobs [in orders]"                          -> NeverDropJobs(...)
//	"never lose work [from queue]"                          -> NeverDropJobs(...)
//	"keep api available under degradation"                 -> AvailableUnderDegradation(...)
//	"keep checkout usable even when degraded"              -> AvailableUnderDegradation(...)
//	"keep cost under 12 dollars per hour"                   -> CostCeiling(12)
//	"do not go over 12$/hour"                               -> CostCeiling(12)
//	"cap spend at $12/hour"                                 -> CostCeiling(12)
//	"keep catalog latency under 500 ms"                    -> LowLatency(...) tightened to 500
//	"latency under 500ms on catalog"                       -> LowLatency(...)
//
// The return value is stable: inputs with the same semantic meaning produce
// equal Intent lists.
type CompileResult struct {
	Intents  []Intent
	Unknowns []string // sentences we could not parse
}

func Compile(input string) CompileResult {
	sentences := splitSentences(input)
	out := CompileResult{}
	for _, raw := range sentences {
		s := normalize(raw)
		if s == "" {
			continue
		}
		if it, ok := matchProtect(s); ok {
			out.Intents = append(out.Intents, it)
			continue
		}
		if it, ok := matchNeverDrop(s); ok {
			out.Intents = append(out.Intents, it)
			continue
		}
		if it, ok := matchAvailability(s); ok {
			out.Intents = append(out.Intents, it)
			continue
		}
		if it, ok := matchCost(s); ok {
			out.Intents = append(out.Intents, it)
			continue
		}
		if it, ok := matchLatency(s); ok {
			out.Intents = append(out.Intents, it)
			continue
		}
		out.Unknowns = append(out.Unknowns, raw)
	}
	return out
}

// ── grammar pieces ────────────────────────────────────────────────────────

var (
	reProtect = regexp.MustCompile(`^(?:protect|preserve|safeguard)\s+([a-z0-9,\s_\-/]+?)(?:\s+at all costs|\s+no matter what|\.|$)`)
	reDrop    = regexp.MustCompile(`^(?:never|do not|dont)\s+(?:drop|lose|discard)\s+(?:jobs|work|tasks|orders|anything|messages)(?:\s+(?:in|from|on)\s+([a-z0-9_\-]+))?`)
	reAvail   = regexp.MustCompile(`^(?:keep|ensure)\s+([a-z0-9,\s_\-]+?)\s+(?:available|usable|reachable|up|online)`)
	// reCostAmount extracts the $/hour number from the sentence.
	// Matches any of: "$25/hr", "12 dollars per hour", "40 usd per hour".
	reCostAmount  = regexp.MustCompile(`\$?\s*(\d+(?:\.\d+)?)\s*(?:dollars|usd|\$)?\s*(?:per hour|/hour|/hr|an hour)`)
	// reCostLimit acts as a gate — only treat the number as a ceiling if
	// the sentence has one of these cost-limiting verbs nearby. Prevents
	// "spending averaged 12 dollars per hour" (a report) from being parsed
	// as a cost ceiling declaration.
	reCostLimit = regexp.MustCompile(`\b(?:under|below|at most|cap|capped|ceiling|limit|stay under|keep (?:it )?under|do(?:\s+not|n't) (?:go over|exceed|spend)|not (?:over|above|to exceed)|less than|no more than)\b`)
	reLatA    = regexp.MustCompile(`^keep\s+([a-z0-9_\-]+)(?:\s+latency)?\s+(?:under|below|less than)\s+(\d+(?:\.\d+)?)\s*ms`)
	reLatB    = regexp.MustCompile(`^latency\s+(?:under|below|less than)\s+(\d+(?:\.\d+)?)\s*ms\s+on\s+([a-z0-9_\-]+)`)
)

func matchProtect(s string) (Intent, bool) {
	m := reProtect.FindStringSubmatch(s)
	if m == nil {
		return Intent{}, false
	}
	services := splitServiceList(m[1])
	if len(services) == 0 {
		return Intent{}, false
	}
	return ProtectCheckout(services...), true
}

func matchNeverDrop(s string) (Intent, bool) {
	m := reDrop.FindStringSubmatch(s)
	if m == nil {
		return Intent{}, false
	}
	queue := strings.TrimSpace(m[1])
	if queue == "" {
		queue = "queue"
	}
	return NeverDropJobs(queue), true
}

func matchAvailability(s string) (Intent, bool) {
	m := reAvail.FindStringSubmatch(s)
	if m == nil {
		return Intent{}, false
	}
	services := splitServiceList(m[1])
	if len(services) == 0 {
		return Intent{}, false
	}
	// Avoid collision with "keep X latency under Y ms" — if the matched
	// clause mentions latency or a number-ms, skip this branch and let the
	// latency matcher handle it.
	if strings.Contains(s, "latency") || strings.Contains(s, "ms") {
		return Intent{}, false
	}
	return AvailableUnderDegradation(services...), true
}

func matchCost(s string) (Intent, bool) {
	// Require both a cost-limiting phrase AND a $/hour number in the sentence.
	if !reCostLimit.MatchString(s) {
		return Intent{}, false
	}
	m := reCostAmount.FindStringSubmatch(s)
	if m == nil {
		return Intent{}, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return Intent{}, false
	}
	return CostCeiling(v), true
}

func matchLatency(s string) (Intent, bool) {
	if m := reLatA.FindStringSubmatch(s); m != nil {
		svc := m[1]
		ms, _ := strconv.ParseFloat(m[2], 64)
		return Intent{Name: "latency-" + svc, Goals: []Goal{{Kind: LatencyUnder, Service: svc, Target: ms, Priority: 5}}}, true
	}
	if m := reLatB.FindStringSubmatch(s); m != nil {
		ms, _ := strconv.ParseFloat(m[1], 64)
		svc := m[2]
		return Intent{Name: "latency-" + svc, Goals: []Goal{{Kind: LatencyUnder, Service: svc, Target: ms, Priority: 5}}}, true
	}
	return Intent{}, false
}

// ── helpers ───────────────────────────────────────────────────────────────

func normalize(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	// Collapse whitespace.
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	// Strip trailing period, comma.
	s = strings.TrimRight(s, ".,;!")
	return s
}

// splitSentences breaks the input on sentence boundaries. We keep it simple:
// period, question mark, exclamation, newline, or semicolon.
func splitSentences(s string) []string {
	re := regexp.MustCompile(`[.!?\n;]+`)
	parts := re.Split(s, -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// splitServiceList turns "checkout and payments, billing" into ["checkout","payments","billing"].
func splitServiceList(s string) []string {
	s = strings.ReplaceAll(s, " and ", ",")
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "'\"`")
		if p == "" || p == "the" || p == "our" {
			continue
		}
		out = append(out, p)
	}
	return out
}
