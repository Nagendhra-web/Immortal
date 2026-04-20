package rest

import (
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/version"
)

// handleConfig — GET /api/config
//
// Returns the current engine configuration plus runtime metadata suitable
// for operator inspection and for compliance audits (SOC 2, FedRAMP).
// Secrets are never included: the engine.Config snapshot does not hold
// API keys or tokens directly; LLM API keys go through internal/llm which
// masks them in its Config dump.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	out := map[string]any{
		"version": map[string]any{
			"tag":        version.Short(),
			"full":       version.Full(),
			"go":         runtime.Version(),
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
			"pid":        os.Getpid(),
			"hostname":   hostname(),
			"started_at": time.Now().UTC().Format(time.RFC3339),
		},
		"features": map[string]any{
			"pqaudit":    s.pqLedger != nil,
			"twin":       s.twinSvc != nil,
			"agentic":    s.agentSvc != nil,
			"federated":  s.fedClient != nil,
			"topology":   s.topoTracker != nil,
			"formal":     s.formalOn,
			"intent":     s.intentEval != nil,
			"narrator":   s.narrator != nil,
			"evolve":     s.evolveAdv != nil,
			"chaos":      s.chaosEng != nil,
			"autolearn":  s.autoLearner != nil,
			"incidents":  s.incidents != nil,
			"capacity":   s.capacityPln != nil,
			"playbook":   s.playbookRun != nil,
			"exporter":   s.exporter != nil,
			"livestream": s.liveStream != nil,
		},
	}
	if s.engineConfig != nil {
		out["engine"] = s.engineConfig()
	}
	writeJSON(w, out)
}

// hostname returns the hostname or "unknown" on error. Not cached to
// stay correct across hostname changes (rare but possible in cloud envs).
func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}
