package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/immortal-engine/immortal/internal/audit"
	"github.com/immortal-engine/immortal/internal/causality"
	"github.com/immortal-engine/immortal/internal/dependency"
	"github.com/immortal-engine/immortal/internal/dna"
	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/export"
	"github.com/immortal-engine/immortal/internal/healing"
	"github.com/immortal-engine/immortal/internal/health"
	"github.com/immortal-engine/immortal/internal/pattern"
	"github.com/immortal-engine/immortal/internal/predict"
	"github.com/immortal-engine/immortal/internal/selfmonitor"
	"github.com/immortal-engine/immortal/internal/sla"
	"github.com/immortal-engine/immortal/internal/storage"
	"github.com/immortal-engine/immortal/internal/stream"
	"github.com/immortal-engine/immortal/internal/timetravel"
)

type Server struct {
	store      *storage.Store
	registry   *health.Registry
	healer     *healing.Healer
	liveStream *stream.Stream

	// Advanced components
	dna             *dna.DNA
	causality       *causality.Graph
	timeTravel      *timetravel.Recorder
	monitor         *selfmonitor.Monitor
	exporter        *export.PrometheusExporter
	patternDet      *pattern.Detector
	predictor       *predict.Predictor
	slaTracker      *sla.Tracker
	auditLog        *audit.Logger
	depGraph        *dependency.Graph
	recommendations func() []healing.Recommendation

	mux *http.ServeMux
}

// ServerConfig holds all components for the full-featured server.
type ServerConfig struct {
	Store           *storage.Store
	Registry        *health.Registry
	Healer          *healing.Healer
	LiveStream      *stream.Stream
	DNA             *dna.DNA
	Causality       *causality.Graph
	TimeTravel      *timetravel.Recorder
	Monitor         *selfmonitor.Monitor
	Exporter        *export.PrometheusExporter
	PatternDet      *pattern.Detector
	Predictor       *predict.Predictor
	SLATracker      *sla.Tracker
	AuditLog        *audit.Logger
	DepGraph        *dependency.Graph
	Recommendations func() []healing.Recommendation
}

func New(store *storage.Store, registry *health.Registry, healer *healing.Healer) *Server {
	s := &Server{
		store:    store,
		registry: registry,
		healer:   healer,
		mux:      http.NewServeMux(),
	}
	s.routes()
	return s
}

func NewWithStream(store *storage.Store, registry *health.Registry, healer *healing.Healer, ls *stream.Stream) *Server {
	s := &Server{
		store:      store,
		registry:   registry,
		healer:     healer,
		liveStream: ls,
		mux:        http.NewServeMux(),
	}
	s.routes()
	return s
}

// NewFull creates a server with all advanced components wired in.
func NewFull(cfg ServerConfig) *Server {
	s := &Server{
		store:           cfg.Store,
		registry:        cfg.Registry,
		healer:          cfg.Healer,
		liveStream:      cfg.LiveStream,
		dna:             cfg.DNA,
		causality:       cfg.Causality,
		timeTravel:      cfg.TimeTravel,
		monitor:         cfg.Monitor,
		exporter:        cfg.Exporter,
		patternDet:      cfg.PatternDet,
		predictor:       cfg.Predictor,
		slaTracker:      cfg.SLATracker,
		auditLog:        cfg.AuditLog,
		depGraph:        cfg.DepGraph,
		recommendations: cfg.Recommendations,
		mux:             http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	// Core endpoints
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/events", s.handleEvents)
	s.mux.HandleFunc("/api/healing/history", s.handleHealingHistory)
	s.mux.HandleFunc("/api/status", s.handleStatus)
	s.mux.HandleFunc("/api/logs/stream", s.handleLogStream)
	s.mux.HandleFunc("/api/logs/history", s.handleLogHistory)

	// Advanced endpoints
	s.mux.HandleFunc("/api/recommendations", s.handleRecommendations)
	s.mux.HandleFunc("/api/metrics", s.handleMetrics)
	s.mux.HandleFunc("/api/dna/baseline", s.handleDNABaseline)
	s.mux.HandleFunc("/api/dna/health-score", s.handleDNAHealthScore)
	s.mux.HandleFunc("/api/dna/anomaly", s.handleDNAAnomaly)
	s.mux.HandleFunc("/api/causality/graph", s.handleCausalityGraph)
	s.mux.HandleFunc("/api/causality/root-cause", s.handleRootCause)
	s.mux.HandleFunc("/api/timetravel", s.handleTimeTravel)
	s.mux.HandleFunc("/api/patterns", s.handlePatterns)
	s.mux.HandleFunc("/api/predictions", s.handlePredictions)
	s.mux.HandleFunc("/api/sla", s.handleSLA)
	s.mux.HandleFunc("/api/audit", s.handleAudit)
	s.mux.HandleFunc("/api/dependencies", s.handleDependencies)
	s.mux.HandleFunc("/api/dependencies/impact", s.handleDependencyImpact)
	s.mux.HandleFunc("/api/monitor", s.handleMonitor)
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

// --- Core handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	services := s.registry.All()
	overall := s.registry.OverallStatus()

	resp := map[string]interface{}{
		"status":   overall,
		"services": services,
	}
	writeJSON(w, resp)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := storage.Query{Limit: 100}
	if typ := r.URL.Query().Get("type"); typ != "" {
		q.Type = event.Type(typ)
	}
	if src := r.URL.Query().Get("source"); src != "" {
		q.Source = src
	}

	events, err := s.store.Query(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, events)
}

func (s *Server) handleHealingHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.healer.History())
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp := map[string]interface{}{
		"engine":  "running",
		"version": "0.2.0",
	}
	if s.monitor != nil {
		stats := s.monitor.Stats()
		resp["uptime"] = stats.Uptime.String()
		resp["goroutines"] = stats.Goroutines
		resp["events_processed"] = stats.EventsProcessed
		resp["heals_executed"] = stats.HealsExecuted
	}
	if s.patternDet != nil {
		resp["active_patterns"] = len(s.patternDet.Patterns())
	}
	if s.predictor != nil {
		resp["active_predictions"] = len(s.predictor.AllPredictions())
	}
	if s.auditLog != nil {
		resp["audit_entries"] = s.auditLog.Count()
	}
	writeJSON(w, resp)
}

func (s *Server) handleLogStream(w http.ResponseWriter, r *http.Request) {
	if s.liveStream == nil {
		http.Error(w, "live stream not available", http.StatusServiceUnavailable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sub, cleanup := s.liveStream.Subscribe(fmt.Sprintf("sse-%s", r.RemoteAddr), 50, nil)
	defer cleanup()

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-sub.Ch:
			if !ok {
				return
			}
			data := stream.FormatJSON(entry)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *Server) handleLogHistory(w http.ResponseWriter, r *http.Request) {
	if s.liveStream == nil {
		writeJSON(w, []stream.LogEntry{})
		return
	}
	entries := s.liveStream.History(100)
	writeJSON(w, entries)
}

// --- Advanced handlers ---

func (s *Server) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.recommendations == nil {
		writeJSON(w, []healing.Recommendation{})
		return
	}
	recs := s.recommendations()
	if recs == nil {
		recs = []healing.Recommendation{}
	}
	writeJSON(w, recs)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.exporter == nil {
		http.Error(w, "metrics not available", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(s.exporter.Export()))
}

func (s *Server) handleDNABaseline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.dna == nil {
		http.Error(w, "DNA not available", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, s.dna.Baseline())
}

func (s *Server) handleDNAHealthScore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.dna == nil {
		http.Error(w, "DNA not available", http.StatusServiceUnavailable)
		return
	}
	score := s.dna.HealthScore(nil)
	writeJSON(w, map[string]interface{}{
		"health_score": score,
		"status":       healthLabel(score),
	})
}

func (s *Server) handleDNAAnomaly(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.dna == nil {
		http.Error(w, "DNA not available", http.StatusServiceUnavailable)
		return
	}
	metric := r.URL.Query().Get("metric")
	valueStr := r.URL.Query().Get("value")
	if metric == "" || valueStr == "" {
		http.Error(w, "metric and value query params required", http.StatusBadRequest)
		return
	}
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		http.Error(w, "invalid value", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]interface{}{
		"metric":     metric,
		"value":      value,
		"is_anomaly": s.dna.IsAnomaly(metric, value),
	})
}

func (s *Server) handleCausalityGraph(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.causality == nil {
		http.Error(w, "causality graph not available", http.StatusServiceUnavailable)
		return
	}
	s.causality.AutoCorrelate()
	writeJSON(w, map[string]interface{}{
		"status": "correlated",
	})
}

func (s *Server) handleRootCause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.causality == nil {
		http.Error(w, "causality graph not available", http.StatusServiceUnavailable)
		return
	}
	eventID := r.URL.Query().Get("event_id")
	if eventID == "" {
		http.Error(w, "event_id query param required", http.StatusBadRequest)
		return
	}
	chain := s.causality.RootCause(eventID)
	writeJSON(w, chain)
}

func (s *Server) handleTimeTravel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.timeTravel == nil {
		http.Error(w, "time-travel not available", http.StatusServiceUnavailable)
		return
	}
	countStr := r.URL.Query().Get("count")
	count := 10
	if countStr != "" {
		if n, err := strconv.Atoi(countStr); err == nil && n > 0 {
			count = n
		}
	}
	beforeStr := r.URL.Query().Get("before")
	before := time.Now()
	if beforeStr != "" {
		if t, err := time.Parse(time.RFC3339, beforeStr); err == nil {
			before = t
		}
	}
	events := s.timeTravel.RewindBefore(before, count)
	writeJSON(w, events)
}

func (s *Server) handlePatterns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.patternDet == nil {
		writeJSON(w, []interface{}{})
		return
	}
	writeJSON(w, s.patternDet.Patterns())
}

func (s *Server) handlePredictions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.predictor == nil {
		writeJSON(w, []interface{}{})
		return
	}
	preds := s.predictor.AllPredictions()
	if preds == nil {
		preds = []predict.Prediction{}
	}
	writeJSON(w, preds)
}

func (s *Server) handleSLA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.slaTracker == nil {
		writeJSON(w, []interface{}{})
		return
	}
	writeJSON(w, s.slaTracker.Report())
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.auditLog == nil {
		writeJSON(w, []interface{}{})
		return
	}
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	action := r.URL.Query().Get("action")
	if action != "" {
		writeJSON(w, s.auditLog.EntriesByAction(action))
		return
	}

	query := r.URL.Query().Get("q")
	if query != "" {
		writeJSON(w, s.auditLog.Search(query))
		return
	}

	writeJSON(w, s.auditLog.Entries(limit))
}

func (s *Server) handleDependencies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.depGraph == nil {
		writeJSON(w, map[string]interface{}{
			"nodes":         []interface{}{},
			"critical_path": []string{},
		})
		return
	}
	writeJSON(w, map[string]interface{}{
		"nodes":         s.depGraph.All(),
		"critical_path": s.depGraph.CriticalPath(),
		"roots":         s.depGraph.Roots(),
		"leaves":        s.depGraph.Leaves(),
		"has_cycle":     s.depGraph.HasCycle(),
	})
}

func (s *Server) handleDependencyImpact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.depGraph == nil {
		http.Error(w, "dependency graph not available", http.StatusServiceUnavailable)
		return
	}
	service := r.URL.Query().Get("service")
	if service == "" {
		http.Error(w, "service query param required", http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]interface{}{
		"service":      service,
		"impact_count": s.depGraph.ImpactOf(service),
		"affected":     s.depGraph.TransitiveDependents(service),
		"depends_on":   s.depGraph.TransitiveDependencies(service),
	})
}

func (s *Server) handleMonitor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.monitor == nil {
		http.Error(w, "monitor not available", http.StatusServiceUnavailable)
		return
	}
	stats := s.monitor.Stats()
	writeJSON(w, map[string]interface{}{
		"goroutines":       stats.Goroutines,
		"events_processed": stats.EventsProcessed,
		"heals_executed":   stats.HealsExecuted,
		"uptime":           stats.Uptime.String(),
		"uptime_seconds":   stats.Uptime.Seconds(),
	})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func healthLabel(score float64) string {
	switch {
	case score >= 0.9:
		return "healthy"
	case score >= 0.7:
		return "degraded"
	case score >= 0.4:
		return "warning"
	default:
		return "critical"
	}
}
