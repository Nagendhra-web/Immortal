// Package main implements the immortal-demo binary — a self-contained
// orchestrator that spins up an in-memory Immortal engine, injects chaos
// scenarios, and prints a real-time healing narrative to stdout.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/immortal-engine/immortal/internal/engine"
	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/healing"
	"github.com/immortal-engine/immortal/internal/twin"
)

// ── ANSI colour helpers ───────────────────────────────────────────────────────

const (
	colReset   = "\033[0m"
	colCyan    = "\033[36m"
	colYellow  = "\033[33m"
	colGreen   = "\033[32m"
	colMagenta = "\033[35m"
	colBold    = "\033[1m"
)

// printer handles all output, respecting --no-color and --quiet flags.
type printer struct {
	noColor bool
	quiet   bool
}

func (p printer) col(code, s string) string {
	if p.noColor {
		return s
	}
	return code + s + colReset
}

func (p printer) observe(source, msg string) {
	if p.quiet {
		return
	}
	ts := time.Now().Format("15:04:05.000")
	fmt.Printf("%s %s [%s] %s\n",
		p.col(colCyan, "[OBSERVE]"),
		ts,
		p.col(colBold, source),
		msg,
	)
}

func (p printer) detect(source, msg string) {
	if p.quiet {
		return
	}
	ts := time.Now().Format("15:04:05.000")
	fmt.Printf("%s %s [%s] %s\n",
		p.col(colYellow, "[DETECT]"),
		ts,
		p.col(colBold, source),
		msg,
	)
}

func (p printer) heal(source, msg string) {
	if p.quiet {
		return
	}
	ts := time.Now().Format("15:04:05.000")
	fmt.Printf("%s %s [%s] %s\n",
		p.col(colGreen, "[HEAL]"),
		ts,
		p.col(colBold, source),
		msg,
	)
}

func (p printer) prove(msg string) {
	if p.quiet {
		return
	}
	ts := time.Now().Format("15:04:05.000")
	fmt.Printf("%s %s %s\n",
		p.col(colMagenta, "[PROVE]"),
		ts,
		msg,
	)
}

func (p printer) section(title string) {
	if p.quiet {
		return
	}
	fmt.Printf("\n%s\n%s\n", p.col(colBold, title), strings.Repeat("─", 60))
}

// ── Config & Report ──────────────────────────────────────────────────────────

// Config holds all CLI-settable parameters for a single demo run.
type Config struct {
	Duration time.Duration
	Scenario string
	NoColor  bool
	Quiet    bool
	JSON     bool
	DataDir  string // empty = auto temp dir
}

// Report is the machine-readable summary produced after a run.
type Report struct {
	EventsIngested     int64    `json:"events_ingested"`
	AnomaliesDetected  int64    `json:"anomalies_detected"`
	HealingActions     int64    `json:"healing_actions"`
	TwinSimulations    int64    `json:"twin_simulations"`
	AuditChainEntries  int      `json:"audit_chain_entries"`
	AuditChainVerified bool     `json:"audit_chain_verified"`
	AuditChainLabel    string   `json:"audit_chain_label"`
	TopologyBefore     string   `json:"topology_before,omitempty"`
	TopologyAfter      string   `json:"topology_after,omitempty"`
	ScenariosRun       []string `json:"scenarios_run"`
	DurationMs         int64    `json:"duration_ms"`
}

// ── Engine factory ───────────────────────────────────────────────────────────

func buildEngine(dataDir string) (*engine.Engine, error) {
	return engine.New(engine.Config{
		DataDir:           dataDir,
		ThrottleWindow:    50 * time.Millisecond,
		DedupWindow:       100 * time.Millisecond,
		ConsensusMin:      1,
		EnablePQAudit:     true,
		EnableTwin:        true,
		EnableAgentic:     true,
		EnableCausal:      true,
		EnableTopology:    true,
		EnableFormal:      true,
		FederatedClientID: "demo-node",
	})
}

func registerServices(eng *engine.Engine) {
	eng.RegisterService("api")
	eng.RegisterService("db")
	eng.RegisterService("cache")

	// Dependency edges: api→db, api→cache, db→disk
	eng.AddDependency("api", "db")
	eng.AddDependency("api", "cache")
	eng.AddDependency("db", "disk")
}

// ── Healing rules ────────────────────────────────────────────────────────────

func registerRules(eng *engine.Engine, p printer, healingActions *int64) {
	for _, svc := range []string{"api", "db", "cache", "disk"} {
		svcName := svc
		eng.AddRule(healing.Rule{
			Name:  fmt.Sprintf("heal-%s-critical", svcName),
			Match: healing.MatchAll(healing.MatchSource(svcName), healing.MatchSeverity(event.SeverityCritical)),
			Action: func(e *event.Event) error {
				atomic.AddInt64(healingActions, 1)
				p.heal(e.Source, fmt.Sprintf("restarting %s (triggered by: %s)", svcName, e.Message))
				return nil
			},
		})
		eng.AddRule(healing.Rule{
			Name:  fmt.Sprintf("heal-%s-error", svcName),
			Match: healing.MatchAll(healing.MatchSource(svcName), healing.MatchSeverity(event.SeverityError)),
			Action: func(e *event.Event) error {
				atomic.AddInt64(healingActions, 1)
				p.heal(e.Source, fmt.Sprintf("mitigating %s error: %s", svcName, e.Message))
				return nil
			},
		})
	}
}

// ── Run ──────────────────────────────────────────────────────────────────────

// Run is the core logic. main() is a thin wrapper around it.
func Run(cfg Config) (*Report, error) {
	p := printer{noColor: cfg.NoColor, quiet: cfg.Quiet}
	start := time.Now()

	dataDir := cfg.DataDir
	if dataDir == "" {
		var err error
		dataDir, err = os.MkdirTemp("", "immortal-demo-*")
		if err != nil {
			return nil, fmt.Errorf("tempdir: %w", err)
		}
		defer os.RemoveAll(dataDir)
	}

	eng, err := buildEngine(dataDir)
	if err != nil {
		return nil, fmt.Errorf("engine init: %w", err)
	}

	registerServices(eng)

	var anomaliesDetected int64
	var healingActions int64

	// Anomaly counter rule (runs before per-service heal rules)
	eng.AddRule(healing.Rule{
		Name:  "count-anomalies",
		Match: healing.MatchSeverity(event.SeverityError),
		Action: func(e *event.Event) error {
			atomic.AddInt64(&anomaliesDetected, 1)
			p.detect(e.Source, fmt.Sprintf("anomaly: %s [%s]", e.Message, e.Severity))
			return nil
		},
	})

	registerRules(eng, p, &healingActions)

	if err := eng.Start(); err != nil {
		return nil, fmt.Errorf("engine start: %w", err)
	}
	defer eng.Stop()

	p.section("Immortal Demo — Engine Online")
	if !cfg.Quiet {
		fmt.Printf("  Features: PQAudit=on Twin=on Agentic=on Causal=on Topology=on Formal=on Federated=demo-node\n")
		fmt.Printf("  Services: api, db, cache\n")
		fmt.Printf("  Duration: %s\n", cfg.Duration)
	}

	// ── Topology snapshot BEFORE ─────────────────────────────────────────────
	topoBefore := snapshotTopo(eng)
	if !cfg.Quiet && topoBefore != "" {
		fmt.Printf("  Topology before: %s\n", topoBefore)
	}

	// ── Run scenarios ────────────────────────────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	var scenariosRun []string
	var twinSimulations int64

	for _, name := range resolveScenarios(cfg.Scenario) {
		fn, ok := scenarioRegistry[name]
		if !ok {
			return nil, fmt.Errorf("unknown scenario %q", name)
		}
		p.section(fmt.Sprintf("Scenario: %s", name))

		if runErr := fn(ctx, eng, p); runErr != nil && runErr != context.DeadlineExceeded {
			if !cfg.Quiet {
				fmt.Printf("  scenario %s ended early: %v\n", name, runErr)
			}
		}

		// Give the async bus time to drain
		time.Sleep(300 * time.Millisecond)
		scenariosRun = append(scenariosRun, name)

		// Agentic ReAct loop on a representative critical event
		agEv := event.New(event.TypeError, event.SeverityCritical,
			fmt.Sprintf("scenario %s: agentic healing requested", name)).
			WithSource("api")
		if trace := eng.RunAgentic(agEv); trace != nil && !cfg.Quiet {
			p.detect("agentic", fmt.Sprintf("ReAct loop: %d steps", len(trace.Steps)))
		}

		// Digital twin simulation — build a concrete twin.Plan
		if eng.Twin() != nil {
			plan := twin.Plan{
				ID: fmt.Sprintf("demo-%s", name),
				Actions: []twin.Action{
					{Type: "restart", Target: "db"},
					{Type: "scale", Target: "api", Params: map[string]string{"replicas": "3"}},
				},
			}
			sim := eng.SimulatePlan(plan)
			atomic.AddInt64(&twinSimulations, 1)
			if !cfg.Quiet {
				p.detect("twin", fmt.Sprintf("simulation accepted=%v improvement=%.2f", sim.Accepted, sim.Improvement))
			}
		}

		// Prove: append a checkpoint to the PQ audit ledger
		if eng.PQLedger() != nil {
			if _, aerr := eng.PQLedger().Append("scenario-complete", "demo", name,
				fmt.Sprintf("scenario %s finished", name), true); aerr == nil {
				p.prove(fmt.Sprintf("audit entry appended for scenario %q", name))
			}
		}

		select {
		case <-ctx.Done():
			goto done
		default:
		}
	}
done:

	// ── Topology snapshot AFTER ──────────────────────────────────────────────
	topoAfter := snapshotTopo(eng)

	// ── PQ Audit verification ────────────────────────────────────────────────
	auditOK, auditIssues := eng.VerifyPQAudit()
	auditCount := 0
	if eng.PQLedger() != nil {
		auditCount = eng.PQLedger().Count()
	}
	auditLabel := "verified"
	if !auditOK {
		auditLabel = fmt.Sprintf("FAILED (%d issues)", len(auditIssues))
	}
	p.prove(fmt.Sprintf("PQ audit chain: %d entries, %s", auditCount, auditLabel))

	// ── Assemble report ──────────────────────────────────────────────────────
	history := eng.HealingHistory()
	auditEntries := eng.AuditLog().Count()

	report := &Report{
		EventsIngested:     int64(auditEntries + len(history)),
		AnomaliesDetected:  atomic.LoadInt64(&anomaliesDetected),
		HealingActions:     int64(len(history)),
		TwinSimulations:    atomic.LoadInt64(&twinSimulations),
		AuditChainEntries:  auditCount,
		AuditChainVerified: auditOK,
		AuditChainLabel:    auditLabel,
		TopologyBefore:     topoBefore,
		TopologyAfter:      topoAfter,
		ScenariosRun:       scenariosRun,
		DurationMs:         time.Since(start).Milliseconds(),
	}

	if cfg.JSON {
		return report, json.NewEncoder(os.Stdout).Encode(report)
	}

	printReport(report, p)
	return report, nil
}

func snapshotTopo(eng *engine.Engine) string {
	snap, err := eng.SnapshotTopology()
	if err != nil {
		return ""
	}
	return fmt.Sprintf("nodes=%d edges=%d components=%d cycles=%d maxBlast=%d",
		snap.NodeCount, snap.EdgeCount, snap.Components, snap.Cycles, snap.MaxBlast)
}

func printReport(r *Report, p printer) {
	p.section("Run Report")
	fmt.Printf("  %-30s %d\n", "Events ingested:", r.EventsIngested)
	fmt.Printf("  %-30s %d\n", "Anomalies detected:", r.AnomaliesDetected)
	fmt.Printf("  %-30s %d\n", "Healing actions executed:", r.HealingActions)
	fmt.Printf("  %-30s %d\n", "Twin simulations:", r.TwinSimulations)
	fmt.Printf("  %-30s %d (%s)\n", "Audit chain:", r.AuditChainEntries,
		p.col(colGreen, "Audit chain: "+r.AuditChainLabel))
	if r.TopologyBefore != "" {
		fmt.Printf("  %-30s %s\n", "Topology before:", r.TopologyBefore)
	}
	if r.TopologyAfter != "" {
		fmt.Printf("  %-30s %s\n", "Topology after:", r.TopologyAfter)
	}
	fmt.Printf("  %-30s %v\n", "Scenarios run:", r.ScenariosRun)
	fmt.Printf("  %-30s %dms\n", "Wall time:", r.DurationMs)
	fmt.Println()
}

// resolveScenarios expands "all" into the canonical ordering.
func resolveScenarios(s string) []string {
	if s == "all" {
		return []string{"quiet", "db_failure", "cascade", "flapping"}
	}
	return []string{s}
}

// ── main ─────────────────────────────────────────────────────────────────────

func main() {
	var duration time.Duration
	var scenario string
	var noColor, quiet, jsonOut bool

	flag.DurationVar(&duration, "duration", 30*time.Second, "total demo duration")
	flag.StringVar(&scenario, "scenario", "all", "scenario: db_failure | cascade | flapping | quiet | all")
	flag.BoolVar(&noColor, "no-color", false, "disable ANSI colour output")
	flag.BoolVar(&quiet, "quiet", false, "suppress per-event output (CI mode)")
	flag.BoolVar(&jsonOut, "json", false, "emit machine-readable JSON report")
	flag.Parse()

	report, err := Run(Config{
		Duration: duration,
		Scenario: scenario,
		NoColor:  noColor,
		Quiet:    quiet,
		JSON:     jsonOut,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "demo error: %v\n", err)
		os.Exit(1)
	}
	if !report.AuditChainVerified {
		os.Exit(2)
	}
}
