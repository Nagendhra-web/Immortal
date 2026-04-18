package rest

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/immortal-engine/immortal/internal/agentic"
	"github.com/immortal-engine/immortal/internal/audit"
	"github.com/immortal-engine/immortal/internal/autolearn"
	"github.com/immortal-engine/immortal/internal/capacity"
	"github.com/immortal-engine/immortal/internal/causal"
	"github.com/immortal-engine/immortal/internal/causality"
	"github.com/immortal-engine/immortal/internal/chaos"
	"github.com/immortal-engine/immortal/internal/correlation"
	"github.com/immortal-engine/immortal/internal/dependency"
	"github.com/immortal-engine/immortal/internal/dna"
	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/export"
	"github.com/immortal-engine/immortal/internal/federated"
	"github.com/immortal-engine/immortal/internal/formal"
	"github.com/immortal-engine/immortal/internal/healing"
	"github.com/immortal-engine/immortal/internal/health"
	"github.com/immortal-engine/immortal/internal/incident"
	"github.com/immortal-engine/immortal/internal/pattern"
	"github.com/immortal-engine/immortal/internal/playbook"
	"github.com/immortal-engine/immortal/internal/pqaudit"
	"github.com/immortal-engine/immortal/internal/predict"
	"github.com/immortal-engine/immortal/internal/selfmonitor"
	"github.com/immortal-engine/immortal/internal/sla"
	"github.com/immortal-engine/immortal/internal/storage"
	"github.com/immortal-engine/immortal/internal/stream"
	"github.com/immortal-engine/immortal/internal/timetravel"
	"github.com/immortal-engine/immortal/internal/topology"
	"github.com/immortal-engine/immortal/internal/twin"
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

	// v0.3.0 components
	chaosEng    *chaos.Engine
	autoLearner *autolearn.Learner
	incidents   *incident.Manager
	capacityPln *capacity.Planner
	correlator  *correlation.Engine
	playbookRun *playbook.Runner

	// v0.4.0 components
	pqLedger  *pqaudit.Ledger
	twinSvc   *twin.Twin
	agentSvc  *agentic.Agent
	fedClient *federated.Client
	causalFn  func(outcome string, vars []string) (*causal.RootCauseResult, error)

	// v0.5.0 components
	topoTracker        *topology.Tracker
	formalOn           bool
	pcmciFn            func(ds *causal.Dataset, cfg causal.PCMCIConfig) (causal.LaggedGraph, error)
	counterfactualFn   func(ds *causal.Dataset, m *causal.StructuralModel, rowIdx int, cause string, do float64, outcome string) (causal.CounterfactualResult, error)
	semanticMemory     *agentic.SemanticMemory
	metaAgent          *agentic.MetaAgent
	aggregatorAdvanced *federated.Aggregator

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
	ChaosEng        *chaos.Engine
	AutoLearner     *autolearn.Learner
	Incidents       *incident.Manager
	CapacityPln     *capacity.Planner
	Correlator      *correlation.Engine
	PlaybookRun     *playbook.Runner

	// v0.4.0 components
	PQLedger  *pqaudit.Ledger
	Twin      *twin.Twin
	Agent     *agentic.Agent
	FedClient *federated.Client
	CausalFn  func(outcome string, vars []string) (*causal.RootCauseResult, error)

	// v0.5.0 — novel observability primitives + advanced agentic/causal/federated.
	Topology           *topology.Tracker  // optional; nil disables topology endpoints
	FormalOn           bool               // mirrors engine.FormalEnabled — gates formal endpoint
	PCMCIFn            func(ds *causal.Dataset, cfg causal.PCMCIConfig) (causal.LaggedGraph, error)
	CounterfactualFn   func(ds *causal.Dataset, m *causal.StructuralModel, rowIdx int, cause string, do float64, outcome string) (causal.CounterfactualResult, error)
	SemanticMemory     *agentic.SemanticMemory
	MetaAgent          *agentic.MetaAgent
	AggregatorAdvanced *federated.Aggregator // for /api/v5/federated/close
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
		chaosEng:        cfg.ChaosEng,
		autoLearner:     cfg.AutoLearner,
		incidents:       cfg.Incidents,
		capacityPln:     cfg.CapacityPln,
		correlator:      cfg.Correlator,
		playbookRun:     cfg.PlaybookRun,
		pqLedger:           cfg.PQLedger,
		twinSvc:            cfg.Twin,
		agentSvc:           cfg.Agent,
		fedClient:          cfg.FedClient,
		causalFn:           cfg.CausalFn,
		topoTracker:        cfg.Topology,
		formalOn:           cfg.FormalOn,
		pcmciFn:            cfg.PCMCIFn,
		counterfactualFn:   cfg.CounterfactualFn,
		semanticMemory:     cfg.SemanticMemory,
		metaAgent:          cfg.MetaAgent,
		aggregatorAdvanced: cfg.AggregatorAdvanced,
		mux:                http.NewServeMux(),
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

	// v0.3.0 endpoints
	s.mux.HandleFunc("/api/chaos/report", s.handleChaosReport)
	s.mux.HandleFunc("/api/autolearn/rules", s.handleAutoLearnRules)
	s.mux.HandleFunc("/api/autolearn/stats", s.handleAutoLearnStats)
	s.mux.HandleFunc("/api/incidents", s.handleIncidents)
	s.mux.HandleFunc("/api/incidents/active", s.handleIncidentsActive)
	s.mux.HandleFunc("/api/capacity", s.handleCapacity)
	s.mux.HandleFunc("/api/capacity/critical", s.handleCapacityCritical)
	s.mux.HandleFunc("/api/correlations", s.handleCorrelations)
	s.mux.HandleFunc("/api/playbooks", s.handlePlaybooks)
	s.mux.HandleFunc("/api/playbooks/history", s.handlePlaybookHistory)

	// v0.4.0 endpoints
	s.mux.HandleFunc("/api/v4/audit/verify", s.handleV4AuditVerify)
	s.mux.HandleFunc("/api/v4/audit/merkle-root", s.handleV4AuditMerkleRoot)
	s.mux.HandleFunc("/api/v4/audit/entries", s.handleV4AuditEntries)
	s.mux.HandleFunc("/api/v4/twin/simulate", s.handleV4TwinSimulate)
	s.mux.HandleFunc("/api/v4/twin/states", s.handleV4TwinStates)
	s.mux.HandleFunc("/api/v4/twin/state/", s.handleV4TwinStateByService)
	s.mux.HandleFunc("/api/v4/agentic/run", s.handleV4AgenticRun)
	s.mux.HandleFunc("/api/v4/causal/root-cause", s.handleV4CausalRootCause)
	s.mux.HandleFunc("/api/v4/federated/snapshot", s.handleV4FederatedSnapshot)

	// v0.5.0 endpoints
	s.mux.HandleFunc("/api/v5/topology/snapshot", s.handleV5TopologySnapshot)
	s.mux.HandleFunc("/api/v5/topology/events", s.handleV5TopologyEvents)
	s.mux.HandleFunc("/api/v5/formal/check", s.handleV5FormalCheck)
	s.mux.HandleFunc("/api/v5/causal/pcmci", s.handleV5CausalPCMCI)
	s.mux.HandleFunc("/api/v5/causal/counterfactual", s.handleV5CausalCounterfactual)
	s.mux.HandleFunc("/api/v5/agentic/memory/recall", s.handleV5AgenticMemoryRecall)
	s.mux.HandleFunc("/api/v5/agentic/meta-investigate", s.handleV5AgenticMetaInvestigate)
	s.mux.HandleFunc("/api/v5/federated/close", s.handleV5FederatedClose)
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
		"version": "0.3.0",
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

// --- v0.3.0 handlers ---

func (s *Server) handleChaosReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.chaosEng == nil {
		writeJSON(w, map[string]interface{}{"total_faults": 0, "score": 0})
		return
	}
	writeJSON(w, s.chaosEng.Report())
}

func (s *Server) handleAutoLearnRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.autoLearner == nil {
		writeJSON(w, []interface{}{})
		return
	}
	suggested := r.URL.Query().Get("suggested")
	if suggested == "true" {
		writeJSON(w, s.autoLearner.SuggestedRules())
		return
	}
	writeJSON(w, s.autoLearner.AllRules())
}

func (s *Server) handleAutoLearnStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.autoLearner == nil {
		writeJSON(w, map[string]interface{}{})
		return
	}
	writeJSON(w, s.autoLearner.Stats())
}

func (s *Server) handleIncidents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.incidents == nil {
		writeJSON(w, []interface{}{})
		return
	}
	query := r.URL.Query().Get("q")
	if query != "" {
		writeJSON(w, s.incidents.Search(query))
		return
	}
	writeJSON(w, s.incidents.All())
}

func (s *Server) handleIncidentsActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.incidents == nil {
		writeJSON(w, []interface{}{})
		return
	}
	writeJSON(w, s.incidents.Active())
}

func (s *Server) handleCapacity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.capacityPln == nil {
		writeJSON(w, []interface{}{})
		return
	}
	writeJSON(w, s.capacityPln.AllForecasts())
}

func (s *Server) handleCapacityCritical(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.capacityPln == nil {
		writeJSON(w, []interface{}{})
		return
	}
	daysStr := r.URL.Query().Get("days")
	days := 7.0
	if daysStr != "" {
		if d, err := strconv.ParseFloat(daysStr, 64); err == nil && d > 0 {
			days = d
		}
	}
	writeJSON(w, s.capacityPln.Critical(days))
}

func (s *Server) handleCorrelations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.correlator == nil {
		writeJSON(w, []interface{}{})
		return
	}
	metric := r.URL.Query().Get("metric")
	if metric != "" {
		writeJSON(w, map[string]interface{}{
			"strongest":          s.correlator.StrongestCorrelation(metric),
			"leading_indicators": s.correlator.LeadingIndicators(metric),
		})
		return
	}
	writeJSON(w, s.correlator.AllCorrelations())
}

func (s *Server) handlePlaybooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.playbookRun == nil {
		writeJSON(w, []string{})
		return
	}
	writeJSON(w, s.playbookRun.List())
}

func (s *Server) handlePlaybookHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.playbookRun == nil {
		writeJSON(w, []interface{}{})
		return
	}
	writeJSON(w, s.playbookRun.History())
}

// --- v0.4.0 handlers ---

func (s *Server) handleV4AuditVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.pqLedger == nil {
		writeJSON(w, map[string]interface{}{"ok": true, "issues": []interface{}{}, "count": 0})
		return
	}
	ok, issues := s.pqLedger.Verify()
	type issueJSON struct {
		Seq    uint64 `json:"seq"`
		Reason string `json:"reason"`
	}
	out := make([]issueJSON, len(issues))
	for i, iss := range issues {
		out[i] = issueJSON{Seq: iss.Seq, Reason: iss.Reason}
	}
	writeJSON(w, map[string]interface{}{
		"ok":     ok,
		"issues": out,
		"count":  len(issues),
	})
}

func (s *Server) handleV4AuditMerkleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.pqLedger == nil {
		http.Error(w, "pqaudit ledger not available", http.StatusNotFound)
		return
	}
	root := s.pqLedger.MerkleRoot()
	writeJSON(w, map[string]interface{}{
		"root":  hex.EncodeToString(root),
		"count": s.pqLedger.Count(),
	})
}

func (s *Server) handleV4AuditEntries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.pqLedger == nil {
		http.Error(w, "pqaudit ledger not available", http.StatusNotFound)
		return
	}
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}
	all := s.pqLedger.Entries()
	if limit < len(all) {
		all = all[len(all)-limit:]
	}
	writeJSON(w, map[string]interface{}{"entries": all})
}

func (s *Server) handleV4TwinSimulate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.twinSvc == nil {
		http.Error(w, "digital twin not available", http.StatusNotFound)
		return
	}
	var body struct {
		ID      string `json:"id"`
		Actions []struct {
			Type   string            `json:"type"`
			Target string            `json:"target"`
			Params map[string]string `json:"params"`
		} `json:"actions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	actions := make([]twin.Action, len(body.Actions))
	for i, a := range body.Actions {
		actions[i] = twin.Action{Type: a.Type, Target: a.Target, Params: a.Params}
	}
	plan := twin.Plan{ID: body.ID, Actions: actions}
	sim := s.twinSvc.Simulate(plan)
	writeJSON(w, sim)
}

func (s *Server) handleV4TwinStates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.twinSvc == nil {
		http.Error(w, "digital twin not available", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]interface{}{"states": s.twinSvc.States()})
}

func (s *Server) handleV4TwinStateByService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.twinSvc == nil {
		http.Error(w, "digital twin not available", http.StatusNotFound)
		return
	}
	// Extract service name from path: /api/v4/twin/state/<service>
	service := strings.TrimPrefix(r.URL.Path, "/api/v4/twin/state/")
	if service == "" {
		http.Error(w, "service name required", http.StatusBadRequest)
		return
	}
	state, ok := s.twinSvc.Get(service)
	if !ok {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}
	writeJSON(w, state)
}

func (s *Server) handleV4AgenticRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.agentSvc == nil {
		http.Error(w, "agentic agent not available", http.StatusNotFound)
		return
	}
	var body struct {
		Type     string                 `json:"type"`
		Severity string                 `json:"severity"`
		Message  string                 `json:"message"`
		Source   string                 `json:"source"`
		Meta     map[string]interface{} `json:"meta"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	ev := event.New(event.Type(body.Type), event.Severity(body.Severity), body.Message)
	ev.Source = body.Source
	for k, v := range body.Meta {
		ev.Meta[k] = v
	}
	trace := s.agentSvc.Run(ev)
	writeJSON(w, trace)
}

func (s *Server) handleV4CausalRootCause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.causalFn == nil {
		http.Error(w, "causal inference not available", http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Outcome   string   `json:"outcome"`
		Variables []string `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if body.Outcome == "" {
		http.Error(w, "outcome is required", http.StatusBadRequest)
		return
	}
	result, err := s.causalFn(body.Outcome, body.Variables)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleV4FederatedSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.fedClient == nil {
		http.Error(w, "federated client not available", http.StatusNotFound)
		return
	}
	roundStr := r.URL.Query().Get("round")
	round := 0
	if roundStr != "" {
		if n, err := strconv.Atoi(roundStr); err == nil {
			round = n
		}
	}
	epsilonStr := r.URL.Query().Get("epsilon")
	epsilon := 0.0
	if epsilonStr != "" {
		if f, err := strconv.ParseFloat(epsilonStr, 64); err == nil {
			epsilon = f
		}
	}
	update := s.fedClient.Snapshot(round, epsilon)
	writeJSON(w, update)
}

// --- v0.5.0 handlers ---

func (s *Server) handleV5TopologySnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.topoTracker == nil {
		http.Error(w, "topology tracker not available", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]interface{}{"snapshot": s.topoTracker.Latest()})
}

func (s *Server) handleV5TopologyEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.topoTracker == nil {
		http.Error(w, "topology tracker not available", http.StatusNotFound)
		return
	}
	events := s.topoTracker.Events()
	if events == nil {
		events = []topology.Event{}
	}
	writeJSON(w, map[string]interface{}{"events": events})
}

func (s *Server) handleV5FormalCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.formalOn {
		http.Error(w, "formal checker not enabled", http.StatusServiceUnavailable)
		return
	}

	var body struct {
		World map[string]struct {
			Healthy  bool `json:"healthy"`
			Replicas int  `json:"replicas"`
		} `json:"world"`
		Plan struct {
			ID    string `json:"id"`
			Steps []struct {
				Name       string                 `json:"name"`
				ActionType string                 `json:"action_type"`
				Target     string                 `json:"target"`
				Params     map[string]interface{} `json:"params"`
			} `json:"steps"`
		} `json:"plan"`
		Invariants []struct {
			Kind string                 `json:"kind"`
			Args map[string]interface{} `json:"args"`
		} `json:"invariants"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Build World
	world := make(formal.World, len(body.World))
	for svc, st := range body.World {
		world[svc] = formal.ServiceState{Name: svc, Healthy: st.Healthy, Replicas: st.Replicas}
	}

	// Build Plan steps
	steps := make([]formal.Action, 0, len(body.Plan.Steps))
	for _, step := range body.Plan.Steps {
		s := step // capture
		var fn func(formal.World) formal.World
		switch s.ActionType {
		case "set_healthy":
			target, _ := s.Params["target"].(string)
			value, _ := s.Params["value"].(bool)
			fn = func(w formal.World) formal.World {
				st := w[target]
				st.Healthy = value
				w[target] = st
				return w
			}
		case "set_replicas":
			target, _ := s.Params["target"].(string)
			var n int
			switch v := s.Params["value"].(type) {
			case float64:
				n = int(v)
			case int:
				n = v
			}
			fn = func(w formal.World) formal.World {
				st := w[target]
				st.Replicas = n
				w[target] = st
				return w
			}
		default: // "noop" or unknown
			fn = func(w formal.World) formal.World { return w }
		}
		steps = append(steps, formal.Action{Name: s.Name, Fn: fn})
	}

	plan := formal.Plan{ID: body.Plan.ID, Steps: steps}

	// Build Invariants
	invs := make([]formal.Invariant, 0, len(body.Invariants))
	for _, inv := range body.Invariants {
		switch inv.Kind {
		case "at_least_n_healthy":
			n := 0
			if v, ok := inv.Args["n"].(float64); ok {
				n = int(v)
			}
			invs = append(invs, formal.AtLeastNHealthy(n))
		case "min_replicas":
			svc, _ := inv.Args["service"].(string)
			n := 0
			if v, ok := inv.Args["n"].(float64); ok {
				n = int(v)
			}
			invs = append(invs, formal.MinReplicas(svc, n))
		case "service_always_healthy":
			svc, _ := inv.Args["service"].(string)
			invs = append(invs, formal.ServiceAlwaysHealthy(svc))
		}
	}

	result := formal.Check(world, plan, invs)
	writeJSON(w, result)
}

func (s *Server) handleV5CausalPCMCI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.pcmciFn == nil {
		http.Error(w, "PCMCI not available", http.StatusServiceUnavailable)
		return
	}

	var body struct {
		Names          []string    `json:"names"`
		Rows           [][]float64 `json:"rows"`
		Alpha          float64     `json:"alpha"`
		TauMax         int         `json:"tau_max"`
		MaxCondSetSize int         `json:"max_cond_set_size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	ds := causal.NewDataset(body.Names)
	for _, row := range body.Rows {
		m := make(map[string]float64, len(body.Names))
		for i, name := range body.Names {
			if i < len(row) {
				m[name] = row[i]
			}
		}
		if err := ds.Add(m); err != nil {
			http.Error(w, "dataset error: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	cfg := causal.PCMCIConfig{
		Alpha:          body.Alpha,
		TauMax:         body.TauMax,
		MaxCondSetSize: body.MaxCondSetSize,
	}
	graph, err := s.pcmciFn(ds, cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, graph)
}

func (s *Server) handleV5CausalCounterfactual(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.counterfactualFn == nil {
		http.Error(w, "counterfactual not available", http.StatusServiceUnavailable)
		return
	}

	var body struct {
		Names    []string               `json:"names"`
		Rows     [][]float64            `json:"rows"`
		Parents  map[string][]string    `json:"parents"`
		RowIndex int                    `json:"row_index"`
		Cause    string                 `json:"cause"`
		Do       float64                `json:"do"`
		Outcome  string                 `json:"outcome"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	ds := causal.NewDataset(body.Names)
	for _, row := range body.Rows {
		m := make(map[string]float64, len(body.Names))
		for i, name := range body.Names {
			if i < len(row) {
				m[name] = row[i]
			}
		}
		if err := ds.Add(m); err != nil {
			http.Error(w, "dataset error: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Build CausalGraph from parents map
	g := causal.CausalGraph{Nodes: body.Names, Directed: make(map[string][]string)}
	for child, pars := range body.Parents {
		for _, par := range pars {
			g.Directed[par] = append(g.Directed[par], child)
		}
	}

	m, err := causal.FitSCM(ds, g)
	if err != nil {
		http.Error(w, "SCM fit error: "+err.Error(), http.StatusBadRequest)
		return
	}

	result, err := s.counterfactualFn(ds, m, body.RowIndex, body.Cause, body.Do, body.Outcome)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleV5AgenticMemoryRecall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.semanticMemory == nil {
		http.Error(w, "semantic memory not available", http.StatusServiceUnavailable)
		return
	}

	var body struct {
		Message  string `json:"message"`
		Source   string `json:"source"`
		Severity string `json:"severity"`
		K        int    `json:"k"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if body.K <= 0 {
		body.K = 5
	}

	query := agentic.Incident{
		Message:  body.Message,
		Source:   body.Source,
		Severity: body.Severity,
	}
	entries := s.semanticMemory.Recall(query, body.K)
	if entries == nil {
		entries = []agentic.MemoryEntry{}
	}
	writeJSON(w, map[string]interface{}{"entries": entries})
}

func (s *Server) handleV5AgenticMetaInvestigate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.metaAgent == nil {
		http.Error(w, "meta-agent not available", http.StatusServiceUnavailable)
		return
	}

	var body struct {
		Type            string   `json:"type"`
		Severity        string   `json:"severity"`
		Message         string   `json:"message"`
		Source          string   `json:"source"`
		HypothesisTypes []string `json:"hypothesis_types"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	ev := event.New(event.Type(body.Type), event.Severity(body.Severity), body.Message)
	ev.Source = body.Source

	// Build hypotheses from requested types using built-in generators with no-op checkers.
	hypotheses := make([]agentic.Hypothesis, 0, len(body.HypothesisTypes))
	for _, ht := range body.HypothesisTypes {
		switch ht {
		case "resource_exhaustion":
			hypotheses = append(hypotheses, agentic.HypothesisResourceExhaustion(
				body.Source,
				func(target, metric string) (float64, error) { return 0, nil },
			))
		case "dependency_failure":
			hypotheses = append(hypotheses, agentic.HypothesisDependencyFailure(
				body.Source,
				func(target string) ([]string, error) { return nil, nil },
			))
		case "recent_deployment":
			hypotheses = append(hypotheses, agentic.HypothesisRecentDeployment(
				body.Source,
				func(target string) ([]string, error) { return nil, nil },
			))
		}
	}

	result := s.metaAgent.Investigate(ev, hypotheses)
	writeJSON(w, result)
}

func (s *Server) handleV5FederatedClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.aggregatorAdvanced == nil {
		http.Error(w, "federated aggregator not available", http.StatusServiceUnavailable)
		return
	}

	var body struct {
		Round int `json:"round"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	gm, err := s.aggregatorAdvanced.Close(body.Round)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, gm)
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
