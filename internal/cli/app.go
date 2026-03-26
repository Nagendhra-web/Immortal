package cli

import (
	"fmt"
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

			// Start REST API
			apiServer := rest.NewWithStream(eng.Store(), eng.HealthRegistry(), eng.Healer(), eng.LiveStream())
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
				"  System online.",
			}
			for _, step := range steps {
				fmt.Printf("\r%s", step)
				time.Sleep(150 * time.Millisecond)
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
			fmt.Printf("  Mode:      %s\n", mode)
			fmt.Printf("  Data:      %s\n", dataDir)
			fmt.Printf("  Version:   %s\n", version.Version)
			fmt.Printf("  Processes: %d watched\n", len(watchProcesses))
			fmt.Printf("  URLs:      %d watched\n", len(watchURLs))
			fmt.Printf("  Logs:      %d watched\n", len(watchLogs))
			fmt.Printf("  API:       http://127.0.0.1:%d\n", apiPort)
			fmt.Printf("  Logs:      immortal logs -f\n")
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

			fmt.Println()
			fmt.Println("  IMMORTAL STATUS")
			fmt.Println("  ═══════════════")

			if resp.StatusCode == 200 {
				fmt.Println("  Status: RUNNING")
			} else {
				fmt.Printf("  Status: ERROR (HTTP %d)\n", resp.StatusCode)
			}

			// Check health
			healthResp, err := http.Get(apiURL + "/api/health")
			if err == nil {
				defer healthResp.Body.Close()
				if healthResp.StatusCode == 200 {
					fmt.Println("  Health: OK")
				}
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

				// Connect to SSE endpoint
				resp, err := http.Get(apiURL + "/api/logs/stream")
				if err != nil {
					fmt.Println("  Cannot connect to Immortal.")
					fmt.Printf("  (Is it running? Try: immortal start)\n")
					return nil
				}
				defer resp.Body.Close()

				// Read SSE stream
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
				// Fetch recent logs
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
