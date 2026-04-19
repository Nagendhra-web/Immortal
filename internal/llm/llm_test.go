package llm_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/llm"
)

func TestDisabledClient(t *testing.T) {
	c := llm.NewDisabled()
	if c.IsEnabled() {
		t.Error("should be disabled")
	}

	resp, err := c.Analyze("system", "user")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "LLM not configured" {
		t.Error("wrong response for disabled")
	}
}

func TestDisabledIncidentAnalysisCritical(t *testing.T) {
	c := llm.NewDisabled()
	decision, err := c.AnalyzeIncident("api", "crash", "critical", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !decision.ShouldHeal {
		t.Error("critical severity should trigger heal")
	}
	if decision.Confidence != 0.8 {
		t.Errorf("critical confidence should be 0.8, got %f", decision.Confidence)
	}
}

func TestDisabledIncidentAnalysisWarning(t *testing.T) {
	c := llm.NewDisabled()
	decision, err := c.AnalyzeIncident("api", "high latency", "warning", nil)
	if err != nil {
		t.Fatal(err)
	}
	if decision.ShouldHeal {
		t.Error("warning severity should NOT trigger heal")
	}
	if decision.Confidence != 0.2 {
		t.Errorf("warning confidence should be 0.2, got %f", decision.Confidence)
	}
}

func TestDisabledIncidentAnalysisInfo(t *testing.T) {
	c := llm.NewDisabled()
	decision, err := c.AnalyzeIncident("api", "started", "info", nil)
	if err != nil {
		t.Fatal(err)
	}
	if decision.ShouldHeal {
		t.Error("info severity should NOT trigger heal")
	}
	if decision.Confidence > 0.2 {
		t.Error("info confidence should be low")
	}
}

func TestOpenAICompatibleProvider(t *testing.T) {
	// Mock OpenAI-compatible server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		if req["model"] != "test-model" {
			t.Error("wrong model")
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{
					"content": `{"should_heal": true, "confidence": 0.9, "action": "restart", "reasoning": "service is down"}`,
				}},
			},
		})
	}))
	defer server.Close()

	c := llm.New(llm.Config{
		Provider: llm.ProviderOpenAI,
		Model:    "test-model",
		Endpoint: server.URL,
		APIKey:   "test-key",
	})

	if !c.IsEnabled() {
		t.Error("should be enabled")
	}

	decision, err := c.AnalyzeIncident("api", "crash", "critical", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !decision.ShouldHeal {
		t.Error("should recommend healing")
	}
	if decision.Confidence != 0.9 {
		t.Errorf("wrong confidence: %f", decision.Confidence)
	}
	if decision.Action != "restart" {
		t.Error("wrong action")
	}
}

func TestClaudeCompatibleProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Claude headers
		if r.Header.Get("x-api-key") != "test-key" {
			t.Error("missing API key")
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Error("missing version header")
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": "Analysis complete: restart the service"},
			},
		})
	}))
	defer server.Close()

	c := llm.New(llm.Config{
		Provider: llm.ProviderClaude,
		Model:    "claude-test",
		Endpoint: server.URL,
		APIKey:   "test-key",
	})

	resp, err := c.Analyze("system prompt", "analyze this")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "Analysis complete: restart the service" {
		t.Errorf("wrong content: %s", resp.Content)
	}
}

func TestOllamaProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": map[string]interface{}{
				"content": "local model response",
			},
		})
	}))
	defer server.Close()

	c := llm.New(llm.Config{
		Provider: llm.ProviderOllama,
		Model:    "llama3",
		Endpoint: server.URL,
	})

	resp, err := c.Analyze("system", "question")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "local model response" {
		t.Error("wrong content")
	}
}

func TestHistory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "response"}},
			},
		})
	}))
	defer server.Close()

	c := llm.New(llm.Config{Provider: llm.ProviderOpenAI, Endpoint: server.URL})
	c.Analyze("s", "q1")
	c.Analyze("s", "q2")
	if len(c.History()) != 2 {
		t.Errorf("expected 2 history, got %d", len(c.History()))
	}
}

func TestProviderName(t *testing.T) {
	c := llm.New(llm.Config{Provider: llm.ProviderClaude})
	if c.Provider() != llm.ProviderClaude {
		t.Error("wrong provider")
	}
}
