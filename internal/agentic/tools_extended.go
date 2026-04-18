package agentic

import (
	"fmt"
	"strings"
	"time"
)

// ExtendedToolsConfig holds optional real implementations for extended tools.
// Any nil field gets a safe stub that returns a canned response.
type ExtendedToolsConfig struct {
	Rollback         func(target, version string) (string, error)
	Scale            func(target string, replicas int) (string, error)
	Canary           func(target string, percent int) (string, error)
	Failover         func(target string) (string, error)
	QueryMetric      func(target, metric string) (float64, error)
	ListDependencies func(target string) ([]string, error)
	QueryHistory     func(target string) ([]string, error)
	Wait             func(d time.Duration) error
	DryRun           func(tool string, args map[string]any) (string, error)
}

// RegisterExtendedTools adds the production-grade tool set to an existing Agent.
func RegisterExtendedTools(a *Agent, cfg ExtendedToolsConfig) {
	// rollback — CostReversible
	rollbackFn := cfg.Rollback
	if rollbackFn == nil {
		rollbackFn = func(target, version string) (string, error) {
			return fmt.Sprintf("stub:rollback:%s->%s", target, version), nil
		}
	}
	a.RegisterTool(Tool{
		Name:        "rollback",
		Description: "Roll back a deployment target to the specified version.",
		Schema:      map[string]string{"target": "string", "version": "string"},
		CostTier:    CostReversible,
		Reversible:  true,
		BlastRadius: 1,
		Fn: func(args map[string]any) (string, error) {
			target, _ := args["target"].(string)
			version, _ := args["version"].(string)
			if target == "" {
				return "", fmt.Errorf("rollback: target is required")
			}
			return rollbackFn(target, version)
		},
	})

	// scale — CostDisruptive
	scaleFn := cfg.Scale
	if scaleFn == nil {
		scaleFn = func(target string, replicas int) (string, error) {
			return fmt.Sprintf("stub:scale:%s:%d", target, replicas), nil
		}
	}
	a.RegisterTool(Tool{
		Name:        "scale",
		Description: "Scale a deployment target to the given replica count.",
		Schema:      map[string]string{"target": "string", "replicas": "int"},
		CostTier:    CostDisruptive,
		Reversible:  true,
		BlastRadius: 2,
		Fn: func(args map[string]any) (string, error) {
			target, _ := args["target"].(string)
			if target == "" {
				return "", fmt.Errorf("scale: target is required")
			}
			replicas := 1
			switch v := args["replicas"].(type) {
			case int:
				replicas = v
			case float64:
				replicas = int(v)
			}
			return scaleFn(target, replicas)
		},
	})

	// canary — CostDestructive (traffic-shifting)
	canaryFn := cfg.Canary
	if canaryFn == nil {
		canaryFn = func(target string, percent int) (string, error) {
			return fmt.Sprintf("stub:canary:%s:%d%%", target, percent), nil
		}
	}
	a.RegisterTool(Tool{
		Name:        "canary",
		Description: "Shift a percentage of traffic to a canary version of the target.",
		Schema:      map[string]string{"target": "string", "percent": "int"},
		CostTier:    CostDestructive,
		Reversible:  false,
		BlastRadius: 3,
		Fn: func(args map[string]any) (string, error) {
			target, _ := args["target"].(string)
			if target == "" {
				return "", fmt.Errorf("canary: target is required")
			}
			percent := 10
			switch v := args["percent"].(type) {
			case int:
				percent = v
			case float64:
				percent = int(v)
			}
			return canaryFn(target, percent)
		},
	})

	// failover — CostDestructive
	failoverFn := cfg.Failover
	if failoverFn == nil {
		failoverFn = func(target string) (string, error) {
			return fmt.Sprintf("stub:failover:%s", target), nil
		}
	}
	a.RegisterTool(Tool{
		Name:        "failover",
		Description: "Trigger failover for the named target to its standby.",
		Schema:      map[string]string{"target": "string"},
		CostTier:    CostDestructive,
		Reversible:  false,
		BlastRadius: 5,
		Fn: func(args map[string]any) (string, error) {
			target, _ := args["target"].(string)
			if target == "" {
				return "", fmt.Errorf("failover: target is required")
			}
			return failoverFn(target)
		},
	})

	// query_metric — CostRead
	queryMetricFn := cfg.QueryMetric
	if queryMetricFn == nil {
		queryMetricFn = func(target, metric string) (float64, error) {
			return 0, nil
		}
	}
	a.RegisterTool(Tool{
		Name:        "query_metric",
		Description: "Query a specific metric value for a target.",
		Schema:      map[string]string{"target": "string", "metric": "string"},
		CostTier:    CostRead,
		Reversible:  true,
		BlastRadius: 0,
		Fn: func(args map[string]any) (string, error) {
			target, _ := args["target"].(string)
			metric, _ := args["metric"].(string)
			v, err := queryMetricFn(target, metric)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s/%s=%.4g", target, metric, v), nil
		},
	})

	// list_dependencies — CostRead
	listDepsFn := cfg.ListDependencies
	if listDepsFn == nil {
		listDepsFn = func(target string) ([]string, error) {
			return []string{}, nil
		}
	}
	a.RegisterTool(Tool{
		Name:        "list_dependencies",
		Description: "List the upstream/downstream dependencies of a target service.",
		Schema:      map[string]string{"target": "string"},
		CostTier:    CostRead,
		Reversible:  true,
		BlastRadius: 0,
		Fn: func(args map[string]any) (string, error) {
			target, _ := args["target"].(string)
			deps, err := listDepsFn(target)
			if err != nil {
				return "", err
			}
			return strings.Join(deps, ","), nil
		},
	})

	// query_incident_history — CostRead
	queryHistoryFn := cfg.QueryHistory
	if queryHistoryFn == nil {
		queryHistoryFn = func(target string) ([]string, error) {
			return []string{}, nil
		}
	}
	a.RegisterTool(Tool{
		Name:        "query_incident_history",
		Description: "Retrieve recent incident history for a target.",
		Schema:      map[string]string{"target": "string"},
		CostTier:    CostRead,
		Reversible:  true,
		BlastRadius: 0,
		Fn: func(args map[string]any) (string, error) {
			target, _ := args["target"].(string)
			history, err := queryHistoryFn(target)
			if err != nil {
				return "", err
			}
			return strings.Join(history, ";"), nil
		},
	})

	// wait — CostReversible
	waitFn := cfg.Wait
	if waitFn == nil {
		waitFn = func(d time.Duration) error {
			time.Sleep(d)
			return nil
		}
	}
	a.RegisterTool(Tool{
		Name:        "wait",
		Description: "Wait for the specified duration in seconds before continuing.",
		Schema:      map[string]string{"seconds": "float"},
		CostTier:    CostReversible,
		Reversible:  true,
		BlastRadius: 0,
		Fn: func(args map[string]any) (string, error) {
			seconds := 1.0
			switch v := args["seconds"].(type) {
			case float64:
				seconds = v
			case int:
				seconds = float64(v)
			}
			d := time.Duration(seconds * float64(time.Second))
			if err := waitFn(d); err != nil {
				return "", err
			}
			return fmt.Sprintf("waited:%.2fs", seconds), nil
		},
	})

	// dry_run — CostReversible
	dryRunFn := cfg.DryRun
	if dryRunFn == nil {
		dryRunFn = func(tool string, args map[string]any) (string, error) {
			return fmt.Sprintf("dry_run:%s:ok", tool), nil
		}
	}
	a.RegisterTool(Tool{
		Name:        "dry_run",
		Description: "Simulate a tool call without side effects. Use before disruptive tools.",
		Schema:      map[string]string{"tool": "string"},
		CostTier:    CostReversible,
		Reversible:  true,
		BlastRadius: 0,
		Fn: func(args map[string]any) (string, error) {
			tool, _ := args["tool"].(string)
			if tool == "" {
				return "", fmt.Errorf("dry_run: tool is required")
			}
			return dryRunFn(tool, args)
		},
	})
}
