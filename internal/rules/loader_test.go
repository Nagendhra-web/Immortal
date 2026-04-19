package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/event"
	"github.com/Nagendhra-web/Immortal/internal/rules"
)

func TestParseRules(t *testing.T) {
	json := []byte(`{
		"rules": [
			{
				"name": "restart-on-crash",
				"match": {"severity": "critical"},
				"action": {"type": "log", "message": "would restart"}
			}
		]
	}`)

	rules, err := rules.Parse(json)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Name != "restart-on-crash" {
		t.Error("wrong rule name")
	}

	// Test that matcher works
	e := event.New(event.TypeError, event.SeverityCritical, "crash")
	if !rules[0].Match(e) {
		t.Error("rule should match critical event")
	}

	// Test non-match
	e2 := event.New(event.TypeError, event.SeverityInfo, "info")
	if rules[0].Match(e2) {
		t.Error("rule should not match info event")
	}
}

func TestParseMultipleMatchers(t *testing.T) {
	json := []byte(`{
		"rules": [
			{
				"name": "api-crash",
				"match": {"severity": "error", "source": "api", "contains": "timeout"},
				"action": {"type": "log"}
			}
		]
	}`)

	rules, err := rules.Parse(json)
	if err != nil {
		t.Fatal(err)
	}

	// Must match ALL conditions
	e := event.New(event.TypeError, event.SeverityError, "connection timeout").WithSource("api")
	if !rules[0].Match(e) {
		t.Error("should match when all conditions met")
	}

	// Wrong source
	e2 := event.New(event.TypeError, event.SeverityError, "connection timeout").WithSource("db")
	if rules[0].Match(e2) {
		t.Error("should not match wrong source")
	}
}

func TestParseExecAction(t *testing.T) {
	json := []byte(`{
		"rules": [
			{
				"name": "restart",
				"match": {"severity": "critical"},
				"action": {"type": "exec", "command": "echo healed"}
			}
		]
	}`)

	rules, err := rules.Parse(json)
	if err != nil {
		t.Fatal(err)
	}

	e := event.New(event.TypeError, event.SeverityCritical, "crash")
	err = rules[0].Action(e)
	if err != nil {
		t.Errorf("exec action should succeed: %v", err)
	}
}

func TestLoadFromFile(t *testing.T) {
	content := `{
		"rules": [
			{
				"name": "file-rule",
				"match": {"severity": "warning"},
				"action": {"type": "log", "message": "from file"}
			}
		]
	}`

	path := filepath.Join(t.TempDir(), "rules.json")
	os.WriteFile(path, []byte(content), 0644)

	rules, err := rules.LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Name != "file-rule" {
		t.Error("wrong name from file")
	}
}

func TestParseInvalidJSON(t *testing.T) {
	_, err := rules.Parse([]byte("not json"))
	if err == nil {
		t.Error("should error on invalid JSON")
	}
}

func TestParseNoMatch(t *testing.T) {
	json := []byte(`{"rules": [{"name": "bad", "match": {}, "action": {"type": "log"}}]}`)
	_, err := rules.Parse(json)
	if err == nil {
		t.Error("should error when no match conditions")
	}
}

func TestParseUnknownAction(t *testing.T) {
	json := []byte(`{"rules": [{"name": "bad", "match": {"severity": "error"}, "action": {"type": "unknown"}}]}`)
	_, err := rules.Parse(json)
	if err == nil {
		t.Error("should error on unknown action type")
	}
}

func TestParseInvalidSeverity(t *testing.T) {
	json := []byte(`{"rules": [{"name": "bad", "match": {"severity": "banana"}, "action": {"type": "log"}}]}`)
	_, err := rules.Parse(json)
	if err == nil {
		t.Error("should error on invalid severity")
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := rules.LoadFromFile("/nonexistent/rules.json")
	if err == nil {
		t.Error("should error for nonexistent file")
	}
}
