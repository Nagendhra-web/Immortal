package agentic

import "fmt"

// registerBuiltins adds the default tool set to the agent.
func registerBuiltins(a *Agent) {
	a.RegisterTool(Tool{
		Name:        "get_metric",
		Description: "Fetch the current value of a named metric.",
		Schema:      map[string]string{"name": "string"},
		Fn: func(args map[string]any) (string, error) {
			name, _ := args["name"].(string)
			return a.cfg.MetricProvider(name)
		},
	})

	a.RegisterTool(Tool{
		Name:        "restart_service",
		Description: "Restart a named service.",
		Schema:      map[string]string{"name": "string"},
		Fn: func(args map[string]any) (string, error) {
			name, _ := args["name"].(string)
			if name == "" {
				return "", fmt.Errorf("restart_service: name is required")
			}
			return "restarted:" + name, nil
		},
	})

	a.RegisterTool(Tool{
		Name:        "scale_service",
		Description: "Scale a service to the given replica count.",
		Schema:      map[string]string{"name": "string", "replicas": "int"},
		Fn: func(args map[string]any) (string, error) {
			name, _ := args["name"].(string)
			replicas := fmt.Sprintf("%v", args["replicas"])
			if name == "" {
				return "", fmt.Errorf("scale_service: name is required")
			}
			return fmt.Sprintf("scaled:%s:%s", name, replicas), nil
		},
	})

	a.RegisterTool(Tool{
		Name:        "check_health",
		Description: "Check the health of a target (returns 'healthy' or 'unhealthy').",
		Schema:      map[string]string{"target": "string"},
		Fn: func(args map[string]any) (string, error) {
			target, _ := args["target"].(string)
			return a.cfg.HealthChecker(target)
		},
	})

	// "finish" is handled by the loop directly, but registering it makes it
	// visible via Tools() for introspection and documentation.
	a.RegisterTool(Tool{
		Name:        "finish",
		Description: "Signal that the incident is resolved and exit the loop.",
		Schema:      map[string]string{"reason": "string"},
		Fn: func(args map[string]any) (string, error) {
			reason, _ := args["reason"].(string)
			return "finished:" + reason, nil
		},
	})
}
