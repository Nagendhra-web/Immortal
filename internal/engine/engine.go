package engine

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/immortal-engine/immortal/internal/alert"
	"github.com/immortal-engine/immortal/internal/audit"
	"github.com/immortal-engine/immortal/internal/bus"
	"github.com/immortal-engine/immortal/internal/causality"
	"github.com/immortal-engine/immortal/internal/consensus"
	"github.com/immortal-engine/immortal/internal/dedup"
	"github.com/immortal-engine/immortal/internal/dependency"
	"github.com/immortal-engine/immortal/internal/dna"
	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/export"
	"github.com/immortal-engine/immortal/internal/healing"
	"github.com/immortal-engine/immortal/internal/health"
	"github.com/immortal-engine/immortal/internal/logger"
	"github.com/immortal-engine/immortal/internal/pattern"
	"github.com/immortal-engine/immortal/internal/predict"
	"github.com/immortal-engine/immortal/internal/rollback"
	"github.com/immortal-engine/immortal/internal/sandbox"
	"github.com/immortal-engine/immortal/internal/selfmonitor"
	"github.com/immortal-engine/immortal/internal/sla"
	"github.com/immortal-engine/immortal/internal/storage"
	"github.com/immortal-engine/immortal/internal/stream"
	"github.com/immortal-engine/immortal/internal/throttle"
	"github.com/immortal-engine/immortal/internal/timetravel"
)

type Config struct {
	DataDir        string
	GhostMode      bool
	ThrottleWindow time.Duration
	DedupWindow    time.Duration
	ConsensusMin   int
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

	return &Engine{
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
	}, nil
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

	e.auditLog.Log("engine-start", "engine", "immortal", fmt.Sprintf("mode=%s", e.modeString()), true)
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
			e.auditLog.Log("ghost-recommend", "engine", ev.Source, ev.Message, true)
			return
		}

		// 12. Consensus check
		consResult := e.consensus.Evaluate(ev)
		if !consResult.Approved {
			e.log.Warn("consensus rejected healing for: %s (votes=%d/%d)",
				ev.Message, consResult.Votes, consResult.Total)
			e.exporter.IncCounter("immortal_consensus_rejected_total")
			e.auditLog.Log("consensus-reject", "consensus", ev.Source, fmt.Sprintf("%s (votes=%d/%d)", ev.Message, consResult.Votes, consResult.Total), false)
			return
		}

		// 13. Log the heal + audit trail
		e.log.Info("healing: %s (consensus %d/%d approved)",
			ev.Message, consResult.Votes, consResult.Total)
		e.liveStream.Heal(ev.Source, ev.Message)
		e.monitor.RecordHeal()
		e.exporter.IncCounter("immortal_heals_executed_total")
		e.auditLog.Log("heal", "healer", ev.Source, ev.Message, true)

		// 14. Fire alerts
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
	e.auditLog.Log("engine-stop", "engine", "immortal", "graceful shutdown", true)
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
