package engine

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/immortal-engine/immortal/internal/agentic"
	"github.com/immortal-engine/immortal/internal/alert"
	"github.com/immortal-engine/immortal/internal/audit"
	"github.com/immortal-engine/immortal/internal/autolearn"
	"github.com/immortal-engine/immortal/internal/bus"
	"github.com/immortal-engine/immortal/internal/capacity"
	"github.com/immortal-engine/immortal/internal/causal"
	"github.com/immortal-engine/immortal/internal/causality"
	"github.com/immortal-engine/immortal/internal/chaos"
	"github.com/immortal-engine/immortal/internal/consensus"
	"github.com/immortal-engine/immortal/internal/correlation"
	"github.com/immortal-engine/immortal/internal/dedup"
	"github.com/immortal-engine/immortal/internal/dependency"
	"github.com/immortal-engine/immortal/internal/dna"
	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/export"
	"github.com/immortal-engine/immortal/internal/federated"
	"github.com/immortal-engine/immortal/internal/healing"
	"github.com/immortal-engine/immortal/internal/health"
	"github.com/immortal-engine/immortal/internal/incident"
	"github.com/immortal-engine/immortal/internal/llm"
	"github.com/immortal-engine/immortal/internal/logger"
	"github.com/immortal-engine/immortal/internal/pattern"
	"github.com/immortal-engine/immortal/internal/playbook"
	"github.com/immortal-engine/immortal/internal/pqaudit"
	"github.com/immortal-engine/immortal/internal/predict"
	"github.com/immortal-engine/immortal/internal/rollback"
	"github.com/immortal-engine/immortal/internal/sandbox"
	"github.com/immortal-engine/immortal/internal/selfmonitor"
	"github.com/immortal-engine/immortal/internal/sla"
	"github.com/immortal-engine/immortal/internal/storage"
	"github.com/immortal-engine/immortal/internal/stream"
	"github.com/immortal-engine/immortal/internal/throttle"
	"github.com/immortal-engine/immortal/internal/timetravel"
	"github.com/immortal-engine/immortal/internal/twin"
)

type Config struct {
	DataDir        string
	GhostMode      bool
	ThrottleWindow time.Duration
	DedupWindow    time.Duration
	ConsensusMin   int

	// v0.4.0 — agentic & advanced intelligence (all opt-in).
	EnablePQAudit     bool        // cryptographic, tamper-evident audit chain
	EnableTwin        bool        // digital-twin plan simulator
	EnableAgentic     bool        // multi-step ReAct healing loop
	EnableCausal      bool        // causal inference on observed metrics
	FederatedClientID string      // non-empty enables federated anomaly learning
	LLMConfig         *llm.Config // optional; if set, agentic loop uses LLM planner
}

type Engine struct {
	mu sync.RWMutex

	// Core
	config  Config
	bus     *bus.Bus
	store   *storage.Store
	healer  *healing.Healer
	log     *logger.Logger
	running bool

	// Intelligence
	dna       *dna.DNA
	causality *causality.Graph
	recorder  *timetravel.Recorder
	sandbox   *sandbox.Sandbox
	consensus *consensus.Engine

	// Infrastructure
	throttler   *throttle.Throttler
	deduper     *dedup.Deduplicator
	rollbacker  *rollback.Manager
	alertMgr    *alert.Manager
	registry    *health.Registry
	monitor     *selfmonitor.Monitor
	exporter    *export.PrometheusExporter
	liveStream  *stream.Stream

	// Advanced intelligence
	patternDet *pattern.Detector
	predictor  *predict.Predictor
	slaTracker *sla.Tracker
	auditLog   *audit.Logger
	depGraph   *dependency.Graph

	// v0.3.0 — next-gen intelligence
	chaosEng    *chaos.Engine
	autoLearner *autolearn.Learner
	incidents   *incident.Manager
	capacityPln *capacity.Planner
	correlator  *correlation.Engine
	playbookRun *playbook.Runner

	// v0.4.0 — agentic & advanced intelligence (nil when disabled)
	pqLedger  *pqaudit.Ledger
	twin      *twin.Twin
	agent     *agentic.Agent
	fedClient *federated.Client
	llmClient *llm.Client

	causalMu   sync.Mutex
	causalData map[string][]float64 // rolling per-metric window
	causalCap  int

	// State
	recommendations []healing.Recommendation
}

func New(cfg Config) (*Engine, error) {
	if cfg.ThrottleWindow == 0 {
		cfg.ThrottleWindow = 5 * time.Second
	}
	if cfg.DedupWindow == 0 {
		cfg.DedupWindow = 10 * time.Second
	}
	if cfg.ConsensusMin == 0 {
		cfg.ConsensusMin = 1
	}

	dbPath := filepath.Join(cfg.DataDir, "immortal.db")
	store, err := storage.New(dbPath)
	if err != nil {
		return nil, err
	}

	h := healing.NewHealer()
	h.SetGhostMode(cfg.GhostMode)

	log := logger.New(logger.LevelInfo)

	// Consensus engine with default verifiers
	cons := consensus.New(consensus.Config{MinAgreement: cfg.ConsensusMin})
	cons.AddVerifier("rule-match", func(e *event.Event) bool {
		return e.Severity.Level() >= event.SeverityError.Level()
	})

	e := &Engine{
		config: cfg,
		bus:    bus.New(),
		store:  store,
		healer: h,
		log:    log,

		dna:       dna.New("immortal"),
		causality: causality.New(),
		recorder:  timetravel.New(10000),
		sandbox:   sandbox.New(),
		consensus: cons,

		throttler:  throttle.New(cfg.ThrottleWindow),
		deduper:    dedup.New(cfg.DedupWindow),
		rollbacker: rollback.New(1000),
		alertMgr:   alert.NewManager(),
		registry:   health.NewRegistry(),
		monitor:    selfmonitor.New(),
		exporter:   export.NewPrometheus(),
		liveStream: stream.New(1000),

		patternDet: pattern.New(5*time.Minute, 3),
		predictor:  predict.New(),
		slaTracker: sla.New(),
		auditLog:   audit.New(10000),
		depGraph:   dependency.New(),

		autoLearner: autolearn.New(5),
		incidents:   incident.New(),
		capacityPln: capacity.New(),
		correlator:  correlation.New(),
		playbookRun: playbook.New(),
	}

	if err := e.initAdvanced(cfg); err != nil {
		return nil, err
	}
	return e, nil
}

// initAdvanced wires optional v0.4.0 features based on Config flags.
func (e *Engine) initAdvanced(cfg Config) error {
	if cfg.EnablePQAudit {
		signer, err := pqaudit.NewEd25519Signer("immortal-engine")
		if err != nil {
			return fmt.Errorf("pqaudit signer: %w", err)
		}
		led, err := pqaudit.New(pqaudit.Config{Signer: signer, MaxEntries: 100000})
		if err != nil {
			return fmt.Errorf("pqaudit ledger: %w", err)
		}
		e.pqLedger = led
	}
	if cfg.EnableTwin {
		e.twin = twin.New(twin.Config{})
	}
	if cfg.EnableAgentic {
		ac := agentic.Config{MaxIterations: 8, StepTimeout: 30 * time.Second}
		if cfg.LLMConfig != nil {
			e.llmClient = llm.New(*cfg.LLMConfig)
			ac.Planner = newLLMPlanner(e.llmClient)
		} else {
			ac.Planner = &heuristicPlanner{}
		}
		e.agent = agentic.New(ac)
	}
	if cfg.EnableCausal {
		e.causalData = make(map[string][]float64)
		e.causalCap = 2000
	}
	if cfg.FederatedClientID != "" {
		e.fedClient = federated.NewClient(cfg.FederatedClientID)
	}
	return nil
}

func (e *Engine) initChaos() {
	e.chaosEng = chaos.New(e.Ingest)
}

func (e *Engine) AddRule(rule healing.Rule) {
	e.healer.AddRule(rule)
}

func (e *Engine) AddAlertRule(rule alert.AlertRule) {
	e.alertMgr.AddRule(rule)
}

func (e *Engine) AddAlertChannel(ch alert.Channel) {
	e.alertMgr.AddChannel(ch)
}

func (e *Engine) AddConsensusVerifier(name string, fn func(*event.Event) bool) {
	e.consensus.AddVerifier(name, fn)
}

func (e *Engine) RegisterService(name string) {
	e.registry.Register(name)
}

func (e *Engine) Start() error {
	e.log.Info("immortal engine starting (mode=%s)", e.modeString())

	// Subscribe to all events on the bus
	e.bus.Subscribe("*", func(ev *event.Event) {
		e.processEvent(ev)
	})

	e.mu.Lock()
	e.running = true
	e.mu.Unlock()

	e.logAudit("engine-start", "engine", "immortal", fmt.Sprintf("mode=%s", e.modeString()), true)
	e.log.Info("immortal engine started — all systems online")
	return nil
}

func (e *Engine) processEvent(ev *event.Event) {
	// 1. Throttle — skip if seen too recently
	if !e.throttler.Allow(ev) {
		e.exporter.IncCounter("immortal_events_throttled_total")
		return
	}

	// 2. Dedup — skip if duplicate content
	if e.deduper.IsDuplicate(ev) {
		e.exporter.IncCounter("immortal_events_deduped_total")
		return
	}

	// 3. Store in SQLite
	e.store.Save(ev)
	e.exporter.IncCounter("immortal_events_stored_total")

	// Stream: emit event to live log
	e.liveStream.Detect(string(ev.Type), ev.Source, ev.Message)

	// 4. Record in time-travel
	e.recorder.Record(ev)

	// 5. Add to causality graph
	e.causality.Add(ev)

	// 6. Feed DNA (metrics only)
	if ev.Type == event.TypeMetric {
		for key, val := range ev.Meta {
			if fval, ok := toFloat64(val); ok {
				e.dna.Record(key, fval)
			}
		}
	}

	// 7. Track self-monitor
	e.monitor.RecordEvent()
	e.exporter.IncCounter("immortal_events_processed_total")

	// 8. Update health registry + SLA tracking
	if ev.Source != "" {
		status := health.StatusHealthy
		isHealthy := true
		if ev.Severity.Level() >= event.SeverityCritical.Level() {
			status = health.StatusUnhealthy
			isHealthy = false
		} else if ev.Severity.Level() >= event.SeverityWarning.Level() {
			status = health.StatusDegraded
		}
		e.registry.Update(ev.Source, status, ev.Message)
		e.slaTracker.RecordStatus(ev.Source, isHealthy)
	}

	// 9. Pattern detection — track recurring failures
	if ev.Severity.Level() >= event.SeverityError.Level() {
		patternKey := fmt.Sprintf("%s:%s", ev.Source, ev.Message)
		e.patternDet.Record(patternKey, string(ev.Severity))
		if e.patternDet.IsRepeating(patternKey) {
			e.exporter.IncCounter("immortal_patterns_detected_total")
			e.liveStream.Detect("pattern", ev.Source, fmt.Sprintf("recurring failure: %s (%dx)", ev.Message, e.patternDet.Count(patternKey)))
		}
	}

	// 10. Predictive healing — feed metrics to predictor
	if ev.Type == event.TypeMetric {
		for key, val := range ev.Meta {
			if fval, ok := toFloat64(val); ok {
				e.predictor.Feed(key, fval)
			}
		}
	}

	// 10.5 v0.4.0 — federated learning + causal inference + digital twin observers.
	// Each feature is a no-op when its corresponding field is nil.
	if ev.Type == event.TypeMetric {
		for key, val := range ev.Meta {
			if fval, ok := toFloat64(val); ok {
				if e.fedClient != nil {
					e.fedClient.Observe(key, fval)
				}
				if e.causalData != nil {
					e.recordCausal(key, fval)
				}
			}
		}
		e.twinObserve(ev)
	}

	// 11. Run through healer (rule matching)
	recs := e.healer.Handle(ev)
	if len(recs) > 0 {
		e.mu.Lock()
		e.recommendations = append(e.recommendations, recs...)
		// Cap recommendations to prevent unbounded growth
		if len(e.recommendations) > 10000 {
			e.recommendations = e.recommendations[len(e.recommendations)-5000:]
		}
		e.mu.Unlock()

		// Ghost mode stops here — recommendations only, no actions
		if e.config.GhostMode {
			e.log.Info("[ghost] would heal: %s (matched %d rules)", ev.Message, len(recs))
			e.liveStream.Ghost("would heal: " + ev.Message)
			e.exporter.IncCounter("immortal_ghost_recommendations_total")
			e.logAudit("ghost-recommend", "engine", ev.Source, ev.Message, true)
			return
		}

		// 12. Consensus check
		consResult := e.consensus.Evaluate(ev)
		if !consResult.Approved {
			e.log.Warn("consensus rejected healing for: %s (votes=%d/%d)",
				ev.Message, consResult.Votes, consResult.Total)
			e.exporter.IncCounter("immortal_consensus_rejected_total")
			e.logAudit("consensus-reject", "consensus", ev.Source, fmt.Sprintf("%s (votes=%d/%d)", ev.Message, consResult.Votes, consResult.Total), false)
			return
		}

		// 13. Log the heal + audit trail + auto-learn
		e.log.Info("healing: %s (consensus %d/%d approved)",
			ev.Message, consResult.Votes, consResult.Total)
		e.liveStream.Heal(ev.Source, ev.Message)
		e.monitor.RecordHeal()
		e.exporter.IncCounter("immortal_heals_executed_total")
		e.logAudit("heal", "healer", ev.Source, ev.Message, true)
		e.autoLearner.Record(recs[0].RuleName, ev.Source, ev.Message, string(ev.Severity), true)

		// 14. Feed capacity planner + correlator (metrics)
		if ev.Type == event.TypeMetric {
			for key, val := range ev.Meta {
				if fval, ok := toFloat64(val); ok {
					e.capacityPln.Record(key, fval)
					e.correlator.Record(key, fval)
				}
			}
		}

		// 15. Fire alerts
		e.alertMgr.Process(ev)
		e.liveStream.Alert(ev.Source, ev.Message)
	}

	// 15. Export current metrics
	e.exporter.SetGauge("immortal_health_score", e.dna.HealthScore(nil))
	e.exporter.SetGauge("immortal_goroutines", float64(e.monitor.Stats().Goroutines))
	e.exporter.SetGauge("immortal_patterns_active", float64(len(e.patternDet.Patterns())))
	e.exporter.SetGauge("immortal_predictions_active", float64(len(e.predictor.AllPredictions())))
}

func (e *Engine) Stop() error {
	e.log.Info("immortal engine shutting down")
	e.logAudit("engine-stop", "engine", "immortal", "graceful shutdown", true)
	e.mu.Lock()
	e.running = false
	e.mu.Unlock()
	e.bus.Close() // Drain queue and stop worker pool
	return e.store.Close()
}

func (e *Engine) Ingest(ev *event.Event) {
	e.bus.Publish(ev)
}

// Accessors

func (e *Engine) Recommendations() []healing.Recommendation {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]healing.Recommendation, len(e.recommendations))
	copy(out, e.recommendations)
	return out
}

func (e *Engine) HealingHistory() []healing.HealRecord {
	return e.healer.History()
}

func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

func (e *Engine) HealthRegistry() *health.Registry {
	return e.registry
}

func (e *Engine) Store() *storage.Store {
	return e.store
}

func (e *Engine) Healer() *healing.Healer {
	return e.healer
}

func (e *Engine) DNA() *dna.DNA {
	return e.dna
}

func (e *Engine) Monitor() *selfmonitor.Monitor {
	return e.monitor
}

func (e *Engine) Exporter() *export.PrometheusExporter {
	return e.exporter
}

func (e *Engine) CausalityGraph() *causality.Graph {
	return e.causality
}

func (e *Engine) TimeTravel() *timetravel.Recorder {
	return e.recorder
}

func (e *Engine) AlertManager() *alert.Manager {
	return e.alertMgr
}

func (e *Engine) LiveStream() *stream.Stream {
	return e.liveStream
}

func (e *Engine) PatternDetector() *pattern.Detector {
	return e.patternDet
}

func (e *Engine) Predictor() *predict.Predictor {
	return e.predictor
}

func (e *Engine) SLATracker() *sla.Tracker {
	return e.slaTracker
}

func (e *Engine) AuditLog() *audit.Logger {
	return e.auditLog
}

func (e *Engine) DependencyGraph() *dependency.Graph {
	return e.depGraph
}

func (e *Engine) ChaosEngine() *chaos.Engine {
	if e.chaosEng == nil {
		e.initChaos()
	}
	return e.chaosEng
}

func (e *Engine) AutoLearner() *autolearn.Learner {
	return e.autoLearner
}

func (e *Engine) IncidentManager() *incident.Manager {
	return e.incidents
}

func (e *Engine) CapacityPlanner() *capacity.Planner {
	return e.capacityPln
}

func (e *Engine) Correlator() *correlation.Engine {
	return e.correlator
}

func (e *Engine) PlaybookRunner() *playbook.Runner {
	return e.playbookRun
}

// SetCapacity sets the max capacity for a metric (for capacity forecasting).
func (e *Engine) SetCapacity(metric string, max float64) {
	e.capacityPln.SetCapacity(metric, max)
}

// SetPredictThreshold configures a prediction threshold for a metric.
func (e *Engine) SetPredictThreshold(metric string, value float64) {
	e.predictor.SetThreshold(metric, value)
}

// AddDependency registers a service dependency (from depends on to).
func (e *Engine) AddDependency(from, to string) {
	e.depGraph.AddDependency(from, to)
}

// SetSLATarget sets the SLA target for a service (e.g., 99.9).
func (e *Engine) SetSLATarget(service string, percent float64) {
	e.slaTracker.SetTarget(service, percent)
}

func (e *Engine) modeString() string {
	if e.config.GhostMode {
		return "ghost"
	}
	return "autonomous"
}

// logAudit writes to the legacy in-memory audit logger and mirrors to the
// cryptographic pqaudit ledger when it is enabled. Both calls are best-effort;
// a pqaudit signer failure is logged but does not block the primary path.
func (e *Engine) logAudit(action, actor, target, detail string, success bool) {
	e.auditLog.Log(action, actor, target, detail, success)
	if e.pqLedger == nil {
		return
	}
	if _, err := e.pqLedger.Append(action, actor, target, detail, success); err != nil {
		e.log.Warn("pqaudit append failed: %v", err)
	}
}

// recordCausal appends a metric value to the rolling causal window.
func (e *Engine) recordCausal(key string, val float64) {
	if e.causalData == nil {
		return
	}
	e.causalMu.Lock()
	defer e.causalMu.Unlock()
	s := e.causalData[key]
	s = append(s, val)
	if len(s) > e.causalCap {
		s = s[len(s)-e.causalCap:]
	}
	e.causalData[key] = s
}

// twinObserve projects a metric event into the digital twin's state for ev.Source.
func (e *Engine) twinObserve(ev *event.Event) {
	if e.twin == nil || ev.Source == "" {
		return
	}
	cur, _ := e.twin.Get(ev.Source)
	cur.Service = ev.Source
	cur.Healthy = ev.Severity.Level() < event.SeverityCritical.Level()
	for key, val := range ev.Meta {
		fval, ok := toFloat64(val)
		if !ok {
			continue
		}
		switch strings.ToLower(key) {
		case "cpu":
			cur.CPU = fval
		case "memory", "mem":
			cur.Memory = fval
		case "latency":
			cur.Latency = fval
		case "error_rate", "errorrate":
			cur.ErrorRate = fval
		case "replicas":
			cur.Replicas = int(fval)
		}
	}
	e.twin.Observe(cur)
}

// PQLedger returns the post-quantum-ready audit ledger, or nil if disabled.
func (e *Engine) PQLedger() *pqaudit.Ledger { return e.pqLedger }

// Twin returns the digital twin simulator, or nil if disabled.
func (e *Engine) Twin() *twin.Twin { return e.twin }

// AgenticAgent returns the ReAct agentic healer, or nil if disabled.
func (e *Engine) AgenticAgent() *agentic.Agent { return e.agent }

// FederatedClient returns the federated learning client, or nil if disabled.
func (e *Engine) FederatedClient() *federated.Client { return e.fedClient }

// LLMClient returns the LLM client used by the agentic planner, or nil if none.
func (e *Engine) LLMClient() *llm.Client { return e.llmClient }

// VerifyPQAudit runs integrity verification over the cryptographic audit chain.
// Returns (true, nil) when the ledger is disabled or clean.
func (e *Engine) VerifyPQAudit() (bool, []pqaudit.VerificationIssue) {
	if e.pqLedger == nil {
		return true, nil
	}
	return e.pqLedger.Verify()
}

// RunAgentic executes the multi-step ReAct healing loop on an incident event.
// Returns nil if agentic mode is disabled.
func (e *Engine) RunAgentic(ev *event.Event) *agentic.Trace {
	if e.agent == nil {
		return nil
	}
	return e.agent.Run(ev)
}

// SimulatePlan runs a healing plan against the digital twin without mutating it.
// Returns a zero-value Simulation if the twin is disabled.
func (e *Engine) SimulatePlan(p twin.Plan) twin.Simulation {
	if e.twin == nil {
		return twin.Simulation{}
	}
	return e.twin.Simulate(p)
}

// CausalRootCause runs PC-algorithm discovery and ranks ancestors of outcome
// by absolute ACE. When variables is empty, all recorded metrics are used.
// The sliding causal window is aligned to the shortest series.
func (e *Engine) CausalRootCause(outcome string, variables []string) (*causal.RootCauseResult, error) {
	if e.causalData == nil {
		return nil, errors.New("causal inference not enabled")
	}
	e.causalMu.Lock()
	defer e.causalMu.Unlock()

	if len(variables) == 0 {
		for k := range e.causalData {
			variables = append(variables, k)
		}
	}
	if len(variables) < 2 {
		return nil, errors.New("need at least 2 variables")
	}
	if _, ok := e.causalData[outcome]; !ok {
		return nil, fmt.Errorf("no data for outcome %q", outcome)
	}

	minLen := -1
	for _, v := range variables {
		d, ok := e.causalData[v]
		if !ok {
			return nil, fmt.Errorf("no data for variable %q", v)
		}
		if minLen == -1 || len(d) < minLen {
			minLen = len(d)
		}
	}
	if minLen < 20 {
		return nil, fmt.Errorf("insufficient data (need ≥20 rows, have %d)", minLen)
	}

	ds := causal.NewDataset(variables)
	for i := 0; i < minLen; i++ {
		row := make(map[string]float64, len(variables))
		for _, v := range variables {
			d := e.causalData[v]
			row[v] = d[len(d)-minLen+i]
		}
		if err := ds.Add(row); err != nil {
			return nil, err
		}
	}

	g, err := causal.Discover(ds, causal.DiscoverConfig{Alpha: 0.05, MaxCondSetSize: 2})
	if err != nil {
		return nil, err
	}
	rc, err := causal.RootCause(ds, g, outcome)
	if err != nil {
		return nil, err
	}
	return &rc, nil
}

// FederatedSnapshot returns a privacy-preserving Update for the configured
// round. Nil when federated mode is disabled.
func (e *Engine) FederatedSnapshot(round int, epsilon float64) *federated.Update {
	if e.fedClient == nil {
		return nil
	}
	u := e.fedClient.Snapshot(round, epsilon)
	return &u
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint64:
		return float64(val), true
	default:
		return 0, false
	}
}
