package rules

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Nagendhra-web/Immortal/internal/event"
	"github.com/Nagendhra-web/Immortal/internal/healing"
)

type RuleConfig struct {
	Name   string       `json:"name"`
	Match  MatchConfig  `json:"match"`
	Action ActionConfig `json:"action"`
}

type MatchConfig struct {
	Severity string `json:"severity,omitempty"`
	Source   string `json:"source,omitempty"`
	Contains string `json:"contains,omitempty"`
}

type ActionConfig struct {
	Type    string `json:"type"` // "exec", "log", "webhook"
	Command string `json:"command,omitempty"`
	URL     string `json:"url,omitempty"`
	Message string `json:"message,omitempty"`
}

type RulesFile struct {
	Rules []RuleConfig `json:"rules"`
}

func LoadFromFile(path string) ([]healing.Rule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("rules: read file: %w", err)
	}
	return Parse(data)
}

func Parse(data []byte) ([]healing.Rule, error) {
	var file RulesFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("rules: parse: %w", err)
	}

	var rules []healing.Rule
	for _, rc := range file.Rules {
		rule, err := buildRule(rc)
		if err != nil {
			return nil, fmt.Errorf("rules: build rule '%s': %w", rc.Name, err)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func buildRule(rc RuleConfig) (healing.Rule, error) {
	matcher, err := buildMatcher(rc.Match)
	if err != nil {
		return healing.Rule{}, err
	}
	action, err := buildAction(rc.Action)
	if err != nil {
		return healing.Rule{}, err
	}
	return healing.Rule{
		Name:   rc.Name,
		Match:  matcher,
		Action: action,
	}, nil
}

func buildMatcher(mc MatchConfig) (healing.MatchFunc, error) {
	var matchers []healing.MatchFunc

	if mc.Severity != "" {
		sev, err := parseSeverity(mc.Severity)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, healing.MatchSeverity(sev))
	}
	if mc.Source != "" {
		matchers = append(matchers, healing.MatchSource(mc.Source))
	}
	if mc.Contains != "" {
		matchers = append(matchers, healing.MatchContains(mc.Contains))
	}

	if len(matchers) == 0 {
		return nil, fmt.Errorf("no match conditions specified")
	}
	if len(matchers) == 1 {
		return matchers[0], nil
	}
	return healing.MatchAll(matchers...), nil
}

func buildAction(ac ActionConfig) (healing.ActionFunc, error) {
	switch ac.Type {
	case "exec":
		if ac.Command == "" {
			return nil, fmt.Errorf("exec action requires command")
		}
		return healing.ActionExec(ac.Command), nil
	case "log":
		msg := ac.Message
		if msg == "" {
			msg = "healing triggered"
		}
		return healing.ActionLog(msg), nil
	default:
		return nil, fmt.Errorf("unknown action type '%s'", ac.Type)
	}
}

func parseSeverity(s string) (event.Severity, error) {
	switch strings.ToLower(s) {
	case "debug":
		return event.SeverityDebug, nil
	case "info":
		return event.SeverityInfo, nil
	case "warning", "warn":
		return event.SeverityWarning, nil
	case "error":
		return event.SeverityError, nil
	case "critical":
		return event.SeverityCritical, nil
	case "fatal":
		return event.SeverityFatal, nil
	default:
		return "", fmt.Errorf("unknown severity '%s'", s)
	}
}
