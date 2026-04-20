package rest

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/evolve"
	"github.com/Nagendhra-web/Immortal/internal/intent"
	"github.com/Nagendhra-web/Immortal/internal/narrator"
)

// v0.6.x REST surface: intent-based healing, narrator, architecture advisor.
// All handlers return 503 when the optional dependency is not configured.

// ── /api/v6/intent ───────────────────────────────────────────────────────
//
//	GET  lists all intents with live goal Status.
//	POST adds or replaces an Intent (JSON body).
func (s *Server) handleV6Intent(w http.ResponseWriter, r *http.Request) {
	if s.intentEval == nil {
		http.Error(w, "intent evaluator not configured", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		intents := s.intentEval.List()
		statuses := s.intentEval.Evaluate()
		// Group statuses by intent.Name via a lookup map; intents are a flat list.
		type byIntent struct {
			Intent   intent.Intent    `json:"intent"`
			Summary  string           `json:"summary"`
			Statuses []intent.Status  `json:"statuses"`
		}
		out := make([]byIntent, 0, len(intents))
		for _, it := range intents {
			row := byIntent{Intent: it, Summary: intent.Summary(it)}
			for _, st := range statuses {
				for _, g := range it.Goals {
					if goalEq(st.Goal, g) {
						row.Statuses = append(row.Statuses, st)
						break
					}
				}
			}
			out = append(out, row)
		}
		writeJSON(w, out)

	case http.MethodPost:
		var body intent.Intent
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if body.Name == "" {
			http.Error(w, "intent.name is required", http.StatusBadRequest)
			return
		}
		s.intentEval.AddIntent(body)
		writeJSON(w, map[string]any{"ok": true, "name": body.Name, "goals": len(body.Goals)})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// /api/v6/intent/:name — DELETE removes a named intent.
func (s *Server) handleV6IntentByName(w http.ResponseWriter, r *http.Request) {
	if s.intentEval == nil {
		http.Error(w, "intent evaluator not configured", http.StatusServiceUnavailable)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/v6/intent/")
	name = strings.TrimSuffix(name, "/")
	if name == "" || name == "suggest" {
		// the "suggest" collision is handled by its own handler; this fall-through
		// is only reached for a POST/GET to that path which we do not support here.
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.intentEval.RemoveIntent(name)
	writeJSON(w, map[string]any{"ok": true, "removed": name})
}

// /api/v6/intent/suggest — GET returns ranked Suggestions compiled from
// currently at-risk or violated goals.
func (s *Server) handleV6IntentSuggest(w http.ResponseWriter, r *http.Request) {
	if s.intentEval == nil {
		http.Error(w, "intent evaluator not configured", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.intentEval.Suggest())
}

// /api/v6/narrator/explain — POST takes an Incident JSON and returns a
// Verdict with three renderings (brief/paragraph/markdown).
func (s *Server) handleV6NarratorExplain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	n := s.narrator
	if n == nil {
		n = narrator.New()
	}
	// Accept one of two shapes:
	//   { "incident": {...} }  -> use narrator.Explain/Brief/Markdown
	//   { "verdict":  {...} }  -> use Verdict.Render/Markdown/Brief
	var body struct {
		Incident *narrator.Incident `json:"incident"`
		Verdict  *narrator.Verdict  `json:"verdict"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if body.Incident != nil {
		writeJSON(w, map[string]any{
			"brief":    n.Brief(*body.Incident),
			"explain":  n.Explain(*body.Incident),
			"markdown": n.Markdown(*body.Incident),
		})
		return
	}
	if body.Verdict != nil {
		writeJSON(w, map[string]any{
			"brief":    body.Verdict.Brief(),
			"render":   body.Verdict.Render(),
			"markdown": body.Verdict.Markdown(),
		})
		return
	}
	http.Error(w, `body must contain "incident" or "verdict"`, http.StatusBadRequest)
}

// /api/v6/evolve/suggest — GET returns ranked architecture suggestions.
// Accepts SignalBag fields as query parameters for ad-hoc calls, or uses
// the EvolveSignals callback if configured.
func (s *Server) handleV6EvolveSuggest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	adv := s.evolveAdv
	if adv == nil {
		adv = evolve.New()
	}
	var bag evolve.SignalBag
	switch {
	case s.evolveSignals != nil:
		bag = s.evolveSignals()
	default:
		// Small default demo bag so the endpoint is never empty in dev mode.
		bag = evolve.SignalBag{
			LatencyP99:   map[string]float64{"checkout": 310},
			CacheHitRate: map[string]float64{"checkout": 0.45},
			RetryRate:    map[string]float64{"api": 0.42},
		}
	}
	suggestions := adv.Analyze(bag)
	// Shape for display: include rank + format string per suggestion.
	type row struct {
		evolve.Suggestion
		Rank      string `json:"rank"`
		Formatted string `json:"formatted"`
	}
	out := make([]row, 0, len(suggestions))
	for _, sg := range suggestions {
		out = append(out, row{Suggestion: sg, Rank: sg.Rank(), Formatted: sg.Format()})
	}
	writeJSON(w, map[string]any{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"count":        len(out),
		"suggestions":  out,
	})
}

// goalEq compares two Goals structurally for the intent grouping.
func goalEq(a, b intent.Goal) bool {
	return a.Kind == b.Kind && a.Service == b.Service && a.Metric == b.Metric &&
		a.Target == b.Target && a.Priority == b.Priority
}

// /api/v6/intent/compile — POST compiles natural-language policy text into
// Intent declarations. Optional query parameter ?register=1 also installs
// each compiled Intent into the evaluator.
func (s *Server) handleV6IntentCompile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}
	res := intent.Compile(body.Text)
	// Optionally register the parsed intents.
	register := r.URL.Query().Get("register") == "1"
	if register && s.intentEval != nil {
		for _, it := range res.Intents {
			s.intentEval.AddIntent(it)
		}
	}
	writeJSON(w, map[string]any{
		"intents":    res.Intents,
		"unknowns":   res.Unknowns,
		"registered": register && s.intentEval != nil,
	})
}
