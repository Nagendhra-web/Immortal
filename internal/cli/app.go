package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/immortal-engine/immortal/internal/alert"
	"github.com/immortal-engine/immortal/internal/api/rest"
	"github.com/immortal-engine/immortal/internal/collector"
	"github.com/immortal-engine/immortal/internal/connector"
	"github.com/immortal-engine/immortal/internal/engine"
	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/healing"
	"github.com/immortal-engine/immortal/internal/middleware"
	"github.com/immortal-engine/immortal/internal/rules"
	"github.com/immortal-engine/immortal/internal/version"
)

func Execute() {
	root := &cobra.Command{
		Use:   "immortal",
		Short: "Your apps never die",
		Long:  "Immortal — the self-healing engine that monitors, protects, and heals your applications 24/7.",
	}

	root.AddCommand(versionCmd())
	root.AddCommand(startCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(logsCmd())
	root.AddCommand(healthCmd())
	root.AddCommand(historyCmd())
	root.AddCommand(recommendCmd())
	root.AddCommand(metricsCmd())
	root.AddCommand(auditCmd())
	root.AddCommand(slaCmd())
	root.AddCommand(predictCmd())
	root.AddCommand(depsCmd())
	root.AddCommand(patternsCmd())
	root.AddCommand(causalityCmd())
	root.AddCommand(timetravelCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version.Full())
		},
	}
}

func startCmd() *cobra.Command {
	var (
		ghostMode      bool
		dataDir        string
		watchProcesses []string
		watchURLs      []string
		watchLogs      []string
		apiPort        int
		rulesFile      string
	)

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Immortal engine",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dataDir == "" {
				home, _ := os.UserHomeDir()
				dataDir = home + "/.immortal"
			}
			os.MkdirAll(dataDir, 0755)

			cfg := engine.Config{
				DataDir:   dataDir,
				GhostMode: ghostMode,
			}

			eng, err := engine.New(cfg)
			if err != nil {
				return fmt.Errorf("failed to create engine: %w", err)
			}

			// Load rules from file if provided
			if rulesFile != "" {
				loaded, err := rules.LoadFromFile(rulesFile)
				if err != nil {
					return fmt.Errorf("failed to load rules: %w", err)
				}
				for _, r := range loaded {
					eng.AddRule(r)
					fmt.Printf("  Rule loaded: %s\n", r.Name)
				}
			}

			// Add default healing rule (logs + alerts)
			eng.AddRule(healing.Rule{
				Name:  "auto-log-critical",
				Match: healing.MatchSeverity(event.SeverityCritical),
				Action: func(e *event.Event) error {
					fmt.Printf("  [HEAL] %s — %s\n", e.Source, e.Message)
					return nil
				},
			})

			// Add alert rules
			eng.AddAlertRule(alert.AlertRule{
				Name:     "critical-alert",
				Match:    func(e *event.Event) bool { return e.Severity.Level() >= event.SeverityCritical.Level() },
				Level:    alert.LevelCritical,
				Title:    "Critical Event Detected",
				Cooldown: 30 * time.Second,
			})
			eng.AddAlertChannel(&alert.LogChannel{})

			// Set default prediction thresholds
			eng.SetPredictThreshold("cpu_percent", 90.0)
			eng.SetPredictThreshold("memory_percent", 90.0)
			eng.SetPredictThreshold("disk_percent", 95.0)

			// Start engine
			if err := eng.Start(); err != nil {
				return err
			}

			// Start metric collector
			mc := collector.NewMetricCollector(10*time.Second, eng.Ingest)
			mc.Start()
			eng.RegisterService("system-metrics")

			// Start process watchers
			for _, proc := range watchProcesses {
				pc := connector.NewProcessConnector(connector.ProcessConfig{
					Name:     proc,
					Interval: 5 * time.Second,
					Callback: eng.Ingest,
				})
				pc.Start()
				eng.RegisterService("process:" + proc)
				fmt.Printf("  Watching process: %s\n", proc)
			}

			// Start URL watchers
			for _, url := range watchURLs {
				hc := connector.NewHTTPConnector(connector.HTTPConfig{
					URL:      url,
					Interval: 10 * time.Second,
					Callback: eng.Ingest,
				})
				hc.Start()
				eng.RegisterService("http:" + url)
				fmt.Printf("  Watching URL: %s\n", url)
			}

			// Start log watchers
			for _, logFile := range watchLogs {
				lc := collector.NewLogCollector(logFile, eng.Ingest)
				lc.Start()
				eng.RegisterService("log:" + logFile)
				fmt.Printf("  Watching log: %s\n", logFile)
			}

			// Start REST API with full feature set
			apiServer := rest.NewFull(rest.ServerConfig{
				Store:           eng.Store(),
				Registry:        eng.HealthRegistry(),
				Healer:          eng.Healer(),
				LiveStream:      eng.LiveStream(),
				DNA:             eng.DNA(),
				Causality:       eng.CausalityGraph(),
				TimeTravel:      eng.TimeTravel(),
				Monitor:         eng.Monitor(),
				Exporter:        eng.Exporter(),
				PatternDet:      eng.PatternDetector(),
				Predictor:       eng.Predictor(),
				SLATracker:      eng.SLATracker(),
				AuditLog:        eng.AuditLog(),
				DepGraph:        eng.DependencyGraph(),
				Recommendations: eng.Recommendations,
			})
			handler := middleware.Chain(
				middleware.Recovery,
				middleware.CORS("*"),
			)(apiServer.Handler())

			go func() {
				addr := fmt.Sprintf("127.0.0.1:%d", apiPort)
				fmt.Printf("  API: http://%s\n", addr)
				http.ListenAndServe(addr, handler)
			}()

			mode := "autonomous"
			if ghostMode {
				mode = "ghost (observe only)"
			}

			// Animated startup sequence
			steps := []string{
				"  Initializing engine...",
				"  Loading healing rules...",
				"  Starting collectors...",
				"  Connecting monitors...",
				"  Activating security...",
				"  Calibrating DNA...",
				"  Initializing predictive models...",
				"  Loading SLA trackers...",
				"  System online.",
			}
			for _, step := range steps {
				fmt.Printf("\r%s", step)
				time.Sleep(100 * time.Millisecond)
				fmt.Println()
			}

			fmt.Println()
			fmt.Println("  ██╗███╗   ███╗███╗   ███╗ ██████╗ ██████╗ ████████╗ █████╗ ██╗")
			fmt.Println("  ██║████╗ ████║████╗ ████║██╔═══██╗██╔══██╗╚══██╔══╝██╔══██╗██║")
			fmt.Println("  ██║██╔████╔██║██╔████╔██║██║   ██║██████╔╝   ██║   ███████║██║")
			fmt.Println("  ██║██║╚██╔╝██║██║╚██╔╝██║██║   ██║██╔══██╗   ██║   ██╔══██║██║")
			fmt.Println("  ██║██║ ╚═╝ ██║██║ ╚═╝ ██║╚██████╔╝██║  ██║   ██║   ██║  ██║███████╗")
			fmt.Println("  ╚═╝╚═╝     ╚═╝╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═╝   ╚═╝   ╚═╝  ╚═╝╚══════╝")
			fmt.Println()
			fmt.Println("  Your apps never die.")
			fmt.Println()
			fmt.Printf("  Mode:        %s\n", mode)
			fmt.Printf("  Data:        %s\n", dataDir)
			fmt.Printf("  Version:     %s\n", version.Version)
			fmt.Printf("  Processes:   %d watched\n", len(watchProcesses))
			fmt.Printf("  URLs:        %d watched\n", len(watchURLs))
			fmt.Printf("  Logs:        %d watched\n", len(watchLogs))
			fmt.Printf("  API:         http://127.0.0.1:%d\n", apiPort)
			fmt.Printf("  Predictions: cpu>90%% mem>90%% disk>95%%\n")
			fmt.Println()
			fmt.Println("  Engine is LIVE. Press Ctrl+C to stop.")
			fmt.Println()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh

			fmt.Println("\n  Shutting down gracefully...")
			mc.Stop()
			return eng.Stop()
		},
	}

	cmd.Flags().BoolVar(&ghostMode, "ghost", false, "Run in ghost mode (observe only, no actions)")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "Data directory (default: ~/.immortal)")
	cmd.Flags().StringSliceVar(&watchProcesses, "watch-process", nil, "Process names to monitor")
	cmd.Flags().StringSliceVar(&watchURLs, "watch-url", nil, "URLs to health-check")
	cmd.Flags().StringSliceVar(&watchLogs, "watch-log", nil, "Log files to tail")
	cmd.Flags().IntVar(&apiPort, "api-port", 7777, "REST API port")
	cmd.Flags().StringVar(&rulesFile, "rules", "", "JSON rules file for healing actions")

	return cmd
}

func statusCmd() *cobra.Command {
	var apiURL string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show engine status",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := http.Get(apiURL + "/api/status")
			if err != nil {
				fmt.Println("  Immortal is not running.")
				fmt.Printf("  (Could not connect to %s)\n", apiURL)
				return nil
			}
			defer resp.Body.Close()

			var data map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&data)

			fmt.Println()
			fmt.Println("  IMMORTAL STATUS")
			fmt.Println("  ═══════════════")
			fmt.Printf("  Engine:           %v\n", data["engine"])
			fmt.Printf("  Version:          %v\n", data["version"])
			if v, ok := data["uptime"]; ok {
				fmt.Printf("  Uptime:           %v\n", v)
			}
			if v, ok := data["events_processed"]; ok {
				fmt.Printf("  Events processed: %v\n", v)
			}
			if v, ok := data["heals_executed"]; ok {
				fmt.Printf("  Heals executed:   %v\n", v)
			}
			if v, ok := data["active_patterns"]; ok {
				fmt.Printf("  Active patterns:  %v\n", v)
			}
			if v, ok := data["active_predictions"]; ok {
				fmt.Printf("  Predictions:      %v\n", v)
			}
			if v, ok := data["audit_entries"]; ok {
				fmt.Printf("  Audit entries:    %v\n", v)
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api", "http://127.0.0.1:7777", "API URL")
	return cmd
}

func logsCmd() *cobra.Command {
	var apiURL string
	var follow bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Watch Immortal's live activity stream",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println()
			fmt.Println("  IMMORTAL LIVE LOGS")
			fmt.Println("  ══════════════════")

			if follow {
				fmt.Println("  Streaming live events (Ctrl+C to stop)...")
				fmt.Println()

				resp, err := http.Get(apiURL + "/api/logs/stream")
				if err != nil {
					fmt.Println("  Cannot connect to Immortal.")
					fmt.Printf("  (Is it running? Try: immortal start)\n")
					return nil
				}
				defer resp.Body.Close()

				buf := make([]byte, 4096)
				for {
					n, err := resp.Body.Read(buf)
					if n > 0 {
						fmt.Print(string(buf[:n]))
					}
					if err != nil {
						break
					}
				}
			} else {
				resp, err := http.Get(apiURL + "/api/logs/history")
				if err != nil {
					fmt.Println("  Cannot connect to Immortal.")
					return nil
				}
				defer resp.Body.Close()

				buf := make([]byte, 64*1024)
				n, _ := resp.Body.Read(buf)
				fmt.Println(string(buf[:n]))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api", "http://127.0.0.1:7777", "API URL")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Stream live logs (like tail -f)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func healthCmd() *cobra.Command {
	var apiURL string
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Show detailed service health",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet(apiURL + "/api/health")
			if err != nil {
				return err
			}
			var data map[string]interface{}
			json.Unmarshal(body, &data)

			fmt.Println()
			fmt.Println("  SERVICE HEALTH")
			fmt.Println("  ══════════════")
			fmt.Printf("  Overall: %v\n", data["status"])
			fmt.Println()

			if services, ok := data["services"]; ok {
				if svcList, ok := services.([]interface{}); ok {
					for _, svc := range svcList {
						if s, ok := svc.(map[string]interface{}); ok {
							icon := "+"
							if s["status"] == "unhealthy" {
								icon = "X"
							} else if s["status"] == "degraded" {
								icon = "!"
							}
							fmt.Printf("  [%s] %-30s %s\n", icon, s["name"], s["status"])
							if msg, ok := s["last_message"]; ok && msg != "" {
								fmt.Printf("      %v\n", msg)
							}
						}
					}
				}
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&apiURL, "api", "http://127.0.0.1:7777", "API URL")
	return cmd
}

func historyCmd() *cobra.Command {
	var apiURL string
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show healing action history",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet(apiURL + "/api/healing/history")
			if err != nil {
				return err
			}
			var records []map[string]interface{}
			json.Unmarshal(body, &records)

			fmt.Println()
			fmt.Println("  HEALING HISTORY")
			fmt.Println("  ═══════════════")
			if len(records) == 0 {
				fmt.Println("  No healing actions recorded yet.")
			}
			for i, r := range records {
				fmt.Printf("  %d. [%s] %s — %s\n", i+1, r["rule_name"], r["event_message"], statusIcon(r["success"]))
				if ts, ok := r["timestamp"]; ok {
					fmt.Printf("     %v\n", ts)
				}
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&apiURL, "api", "http://127.0.0.1:7777", "API URL")
	return cmd
}

func recommendCmd() *cobra.Command {
	var apiURL string
	cmd := &cobra.Command{
		Use:   "recommend",
		Short: "Show ghost mode recommendations",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet(apiURL + "/api/recommendations")
			if err != nil {
				return err
			}
			var recs []map[string]interface{}
			json.Unmarshal(body, &recs)

			fmt.Println()
			fmt.Println("  RECOMMENDATIONS")
			fmt.Println("  ═══════════════")
			if len(recs) == 0 {
				fmt.Println("  No recommendations. (Run in ghost mode to collect.)")
			}
			for i, r := range recs {
				fmt.Printf("  %d. [%s] %s\n", i+1, r["RuleName"], r["Message"])
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&apiURL, "api", "http://127.0.0.1:7777", "API URL")
	return cmd
}

func metricsCmd() *cobra.Command {
	var apiURL string
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Show Prometheus metrics",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet(apiURL + "/api/metrics")
			if err != nil {
				return err
			}
			fmt.Println()
			fmt.Println("  PROMETHEUS METRICS")
			fmt.Println("  ══════════════════")
			fmt.Println(string(body))
			return nil
		},
	}
	cmd.Flags().StringVar(&apiURL, "api", "http://127.0.0.1:7777", "API URL")
	return cmd
}

func auditCmd() *cobra.Command {
	var apiURL string
	var limit int
	var action string
	var query string

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Show audit trail of all engine actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			url := fmt.Sprintf("%s/api/audit?limit=%d", apiURL, limit)
			if action != "" {
				url += "&action=" + action
			}
			if query != "" {
				url += "&q=" + query
			}
			body, err := apiGet(url)
			if err != nil {
				return err
			}
			var entries []map[string]interface{}
			json.Unmarshal(body, &entries)

			fmt.Println()
			fmt.Println("  AUDIT TRAIL")
			fmt.Println("  ═══════════")
			if len(entries) == 0 {
				fmt.Println("  No audit entries.")
			}
			for _, e := range entries {
				icon := "+"
				if success, ok := e["success"].(bool); ok && !success {
					icon = "X"
				}
				fmt.Printf("  [%s] %-20s %-15s -> %-20s %s\n",
					icon, e["action"], e["actor"], e["target"], e["detail"])
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&apiURL, "api", "http://127.0.0.1:7777", "API URL")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max entries to show")
	cmd.Flags().StringVar(&action, "action", "", "Filter by action type")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Search audit entries")
	return cmd
}

func slaCmd() *cobra.Command {
	var apiURL string
	cmd := &cobra.Command{
		Use:   "sla",
		Short: "Show SLA report per service",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet(apiURL + "/api/sla")
			if err != nil {
				return err
			}
			var report []map[string]interface{}
			json.Unmarshal(body, &report)

			fmt.Println()
			fmt.Println("  SLA REPORT")
			fmt.Println("  ══════════")
			if len(report) == 0 {
				fmt.Println("  No SLA data collected yet.")
			}
			fmt.Printf("  %-30s %10s %10s %10s %s\n", "SERVICE", "UPTIME %", "CHECKS", "FAILED", "STATUS")
			fmt.Println("  " + repeat("─", 80))
			for _, s := range report {
				uptime := 0.0
				if v, ok := s["uptime_percent"].(float64); ok {
					uptime = v
				}
				icon := "OK"
				if violations, ok := s["violations"].(float64); ok && violations > 0 {
					icon = "VIOLATION"
				}
				fmt.Printf("  %-30s %9.2f%% %10v %10v %s\n",
					s["name"], uptime, s["total_checks"], s["failed_checks"], icon)
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&apiURL, "api", "http://127.0.0.1:7777", "API URL")
	return cmd
}

func predictCmd() *cobra.Command {
	var apiURL string
	cmd := &cobra.Command{
		Use:   "predict",
		Short: "Show failure predictions based on metric trends",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet(apiURL + "/api/predictions")
			if err != nil {
				return err
			}
			var preds []map[string]interface{}
			json.Unmarshal(body, &preds)

			fmt.Println()
			fmt.Println("  FAILURE PREDICTIONS")
			fmt.Println("  ═══════════════════")
			if len(preds) == 0 {
				fmt.Println("  No predictions yet. (Metrics need time to build trends.)")
			}
			for i, p := range preds {
				severity := p["severity"]
				icon := "i"
				if severity == "critical" {
					icon = "!"
				} else if severity == "warning" {
					icon = "~"
				}
				fmt.Printf("  %d. [%s] %s: %.1f -> %.1f (confidence: %.0f%%)\n",
					i+1, icon, p["metric"], p["current_value"], p["predicted_value"],
					toPercent(p["confidence"]))
				if ttl, ok := p["time_to_threshold"]; ok && ttl != "" {
					fmt.Printf("     Breach in: %v\n", ttl)
				}
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&apiURL, "api", "http://127.0.0.1:7777", "API URL")
	return cmd
}

func depsCmd() *cobra.Command {
	var apiURL string
	var service string
	cmd := &cobra.Command{
		Use:   "deps",
		Short: "Show service dependency graph and blast radius",
		RunE: func(cmd *cobra.Command, args []string) error {
			if service != "" {
				body, err := apiGet(fmt.Sprintf("%s/api/dependencies/impact?service=%s", apiURL, service))
				if err != nil {
					return err
				}
				var data map[string]interface{}
				json.Unmarshal(body, &data)

				fmt.Println()
				fmt.Printf("  IMPACT ANALYSIS: %s\n", service)
				fmt.Println("  " + repeat("═", 40))
				fmt.Printf("  Blast radius: %v services affected\n", data["impact_count"])
				if affected, ok := data["affected"].([]interface{}); ok && len(affected) > 0 {
					fmt.Println("  Affected:")
					for _, a := range affected {
						fmt.Printf("    -> %v\n", a)
					}
				}
				if deps, ok := data["depends_on"].([]interface{}); ok && len(deps) > 0 {
					fmt.Println("  Depends on:")
					for _, d := range deps {
						fmt.Printf("    <- %v\n", d)
					}
				}
				fmt.Println()
				return nil
			}

			body, err := apiGet(apiURL + "/api/dependencies")
			if err != nil {
				return err
			}
			var data map[string]interface{}
			json.Unmarshal(body, &data)

			fmt.Println()
			fmt.Println("  DEPENDENCY GRAPH")
			fmt.Println("  ════════════════")
			if nodes, ok := data["nodes"].([]interface{}); ok {
				for _, n := range nodes {
					if node, ok := n.(map[string]interface{}); ok {
						fmt.Printf("  %s\n", node["name"])
						if deps, ok := node["dependencies"].([]interface{}); ok {
							for _, d := range deps {
								fmt.Printf("    <- %v\n", d)
							}
						}
						if depnts, ok := node["dependents"].([]interface{}); ok {
							for _, d := range depnts {
								fmt.Printf("    -> %v\n", d)
							}
						}
					}
				}
			}
			if cp, ok := data["critical_path"].([]interface{}); ok && len(cp) > 0 {
				fmt.Println()
				fmt.Println("  Critical path (highest impact first):")
				for i, s := range cp {
					fmt.Printf("    %d. %v\n", i+1, s)
				}
			}
			if hasCycle, ok := data["has_cycle"].(bool); ok && hasCycle {
				fmt.Println()
				fmt.Println("  WARNING: Circular dependency detected!")
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&apiURL, "api", "http://127.0.0.1:7777", "API URL")
	cmd.Flags().StringVarP(&service, "service", "s", "", "Analyze impact of a specific service")
	return cmd
}

func patternsCmd() *cobra.Command {
	var apiURL string
	cmd := &cobra.Command{
		Use:   "patterns",
		Short: "Show detected recurring failure patterns",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := apiGet(apiURL + "/api/patterns")
			if err != nil {
				return err
			}
			var patterns []map[string]interface{}
			json.Unmarshal(body, &patterns)

			fmt.Println()
			fmt.Println("  RECURRING PATTERNS")
			fmt.Println("  ══════════════════")
			if len(patterns) == 0 {
				fmt.Println("  No recurring patterns detected.")
			}
			for i, p := range patterns {
				fmt.Printf("  %d. %s (count: %v, severity: %v)\n",
					i+1, p["key"], p["count"], p["severity"])
				fmt.Printf("     First: %v  Last: %v\n", p["first_seen"], p["last_seen"])
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&apiURL, "api", "http://127.0.0.1:7777", "API URL")
	return cmd
}

func causalityCmd() *cobra.Command {
	var apiURL string
	var eventID string
	cmd := &cobra.Command{
		Use:   "causality",
		Short: "Show causality graph or trace root cause of an event",
		RunE: func(cmd *cobra.Command, args []string) error {
			if eventID != "" {
				body, err := apiGet(fmt.Sprintf("%s/api/causality/root-cause?event_id=%s", apiURL, eventID))
				if err != nil {
					return err
				}
				var chain []map[string]interface{}
				json.Unmarshal(body, &chain)

				fmt.Println()
				fmt.Println("  ROOT CAUSE ANALYSIS")
				fmt.Println("  ═══════════════════")
				for i, e := range chain {
					fmt.Printf("  %d. [%s] %s — %s\n", i+1, e["severity"], e["source"], e["message"])
				}
				fmt.Println()
				return nil
			}

			body, err := apiGet(apiURL + "/api/causality/graph")
			if err != nil {
				return err
			}
			var data map[string]interface{}
			json.Unmarshal(body, &data)

			fmt.Println()
			fmt.Println("  CAUSALITY GRAPH")
			fmt.Println("  ═══════════════")
			fmt.Printf("  Nodes: %v\n", data["nodes"])
			if edges, ok := data["edges"].([]interface{}); ok {
				fmt.Printf("  Edges: %d\n", len(edges))
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&apiURL, "api", "http://127.0.0.1:7777", "API URL")
	cmd.Flags().StringVarP(&eventID, "event", "e", "", "Trace root cause of event ID")
	return cmd
}

func timetravelCmd() *cobra.Command {
	var apiURL string
	var count int
	var before string
	cmd := &cobra.Command{
		Use:   "timetravel",
		Short: "Replay events before a point in time",
		RunE: func(cmd *cobra.Command, args []string) error {
			url := fmt.Sprintf("%s/api/timetravel?count=%d", apiURL, count)
			if before != "" {
				url += "&before=" + before
			}
			body, err := apiGet(url)
			if err != nil {
				return err
			}
			var events []map[string]interface{}
			json.Unmarshal(body, &events)

			fmt.Println()
			fmt.Println("  TIME TRAVEL")
			fmt.Println("  ═══════════")
			if len(events) == 0 {
				fmt.Println("  No events to replay.")
			}
			for i, e := range events {
				fmt.Printf("  %d. [%s] %s — %s\n", i+1, e["severity"], e["source"], e["message"])
			}
			fmt.Println()
			return nil
		},
	}
	cmd.Flags().StringVar(&apiURL, "api", "http://127.0.0.1:7777", "API URL")
	cmd.Flags().IntVarP(&count, "count", "n", 10, "Number of events to replay")
	cmd.Flags().StringVarP(&before, "before", "b", "", "Replay events before this time (RFC3339)")
	return cmd
}

// --- Helpers ---

func apiGet(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("  Cannot connect to Immortal.")
		fmt.Println("  (Is it running? Try: immortal start)")
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func statusIcon(v interface{}) string {
	if b, ok := v.(bool); ok && b {
		return "OK"
	}
	return "FAILED"
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

func toPercent(v interface{}) float64 {
	if f, ok := v.(float64); ok {
		return f * 100
	}
	return 0
}
