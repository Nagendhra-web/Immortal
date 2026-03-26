package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type Provider string

const (
	ProviderClaude Provider = "claude"
	ProviderOpenAI Provider = "openai"
	ProviderOllama Provider = "ollama"
	ProviderCustom Provider = "custom"
	ProviderNone   Provider = "none"
)

type Config struct {
	Provider Provider
	APIKey   string
	Model    string
	Endpoint string
	Timeout  time.Duration
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Response struct {
	Content    string        `json:"content"`
	Model      string        `json:"model"`
	TokensUsed int           `json:"tokens_used"`
	Latency    time.Duration `json:"latency"`
}

type Decision struct {
	Action     string  `json:"action"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning"`
	ShouldHeal bool    `json:"should_heal"`
}

type Client struct {
	mu      sync.RWMutex
	config  Config
	http    *http.Client
	history []Response
}

func New(config Config) *Client {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.Model == "" {
		switch config.Provider {
		case ProviderClaude:
			config.Model = "claude-sonnet-4-20250514"
		case ProviderOpenAI:
			config.Model = "gpt-4o"
		case ProviderOllama:
			config.Model = "llama3"
		}
	}
	if config.Endpoint == "" {
		switch config.Provider {
		case ProviderClaude:
			config.Endpoint = "https://api.anthropic.com/v1/messages"
		case ProviderOpenAI:
			config.Endpoint = "https://api.openai.com/v1/chat/completions"
		case ProviderOllama:
			config.Endpoint = "http://localhost:11434/api/chat"
		}
	}
	return &Client{
		config: config,
		http:   &http.Client{Timeout: config.Timeout},
	}
}

func NewDisabled() *Client {
	return &Client{config: Config{Provider: ProviderNone}}
}

func (c *Client) IsEnabled() bool {
	return c.config.Provider != ProviderNone && c.config.Provider != ""
}

func (c *Client) Analyze(systemPrompt, userPrompt string) (*Response, error) {
	if !c.IsEnabled() {
		return &Response{Content: "LLM not configured"}, nil
	}

	start := time.Now()

	var body []byte
	var err error

	switch c.config.Provider {
	case ProviderClaude:
		body, err = c.buildClaudeRequest(systemPrompt, userPrompt)
	case ProviderOpenAI:
		body, err = c.buildOpenAIRequest(systemPrompt, userPrompt)
	case ProviderOllama:
		body, err = c.buildOllamaRequest(systemPrompt, userPrompt)
	default:
		body, err = c.buildOpenAIRequest(systemPrompt, userPrompt) // default format
	}
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.config.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	switch c.config.Provider {
	case ProviderClaude:
		req.Header.Set("x-api-key", c.config.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	case ProviderOpenAI:
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("llm returned %d: %s", resp.StatusCode, string(respBody))
	}

	content, err := c.parseResponse(respBody)
	if err != nil {
		return nil, err
	}

	response := &Response{
		Content: content,
		Model:   c.config.Model,
		Latency: time.Since(start),
	}

	c.mu.Lock()
	c.history = append(c.history, *response)
	c.mu.Unlock()

	return response, nil
}

func (c *Client) AnalyzeIncident(source, message, severity string, metrics map[string]float64) (*Decision, error) {
	if !c.IsEnabled() {
		// Safe default: only heal critical/fatal, scale confidence by severity
		shouldHeal := severity == "critical" || severity == "fatal"
		confidence := 0.0
		switch severity {
		case "fatal":
			confidence = 0.95
		case "critical":
			confidence = 0.8
		case "error":
			confidence = 0.4
		case "warning":
			confidence = 0.2
		default:
			confidence = 0.1
		}
		return &Decision{
			ShouldHeal: shouldHeal,
			Confidence: confidence,
			Reasoning:  fmt.Sprintf("LLM not configured — severity-based default (severity=%s)", severity),
			Action:     "auto",
		}, nil
	}

	prompt := fmt.Sprintf(
		"Incident detected:\nSource: %s\nSeverity: %s\nMessage: %s\nMetrics: %v\n\nShould we auto-heal? What action should be taken? Respond in JSON with fields: should_heal (bool), confidence (0-1), action (string), reasoning (string).",
		source, severity, message, metrics,
	)

	resp, err := c.Analyze(
		"You are Immortal, an autonomous self-healing engine. Analyze incidents and recommend healing actions. Always respond with valid JSON.",
		prompt,
	)
	if err != nil {
		return nil, err
	}

	var decision Decision
	if err := json.Unmarshal([]byte(resp.Content), &decision); err != nil {
		// If LLM doesn't return valid JSON, use the raw response
		decision = Decision{
			ShouldHeal: true,
			Confidence: 0.5,
			Reasoning:  resp.Content,
			Action:     "auto",
		}
	}
	return &decision, nil
}

func (c *Client) History() []Response {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Response, len(c.history))
	copy(out, c.history)
	return out
}

func (c *Client) Provider() Provider {
	return c.config.Provider
}

func (c *Client) buildClaudeRequest(system, user string) ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"model":      c.config.Model,
		"max_tokens": 1024,
		"system":     system,
		"messages":   []Message{{Role: "user", Content: user}},
	})
}

func (c *Client) buildOpenAIRequest(system, user string) ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"model":      c.config.Model,
		"max_tokens": 1024,
		"messages": []Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
}

func (c *Client) buildOllamaRequest(system, user string) ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"model":  c.config.Model,
		"stream": false,
		"messages": []Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
}

func (c *Client) parseResponse(body []byte) (string, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", err
	}

	switch c.config.Provider {
	case ProviderClaude:
		if content, ok := raw["content"].([]interface{}); ok && len(content) > 0 {
			if block, ok := content[0].(map[string]interface{}); ok {
				if text, ok := block["text"].(string); ok {
					return text, nil
				}
			}
		}
	case ProviderOpenAI:
		if choices, ok := raw["choices"].([]interface{}); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]interface{}); ok {
				if msg, ok := choice["message"].(map[string]interface{}); ok {
					if content, ok := msg["content"].(string); ok {
						return content, nil
					}
				}
			}
		}
	case ProviderOllama:
		if msg, ok := raw["message"].(map[string]interface{}); ok {
			if content, ok := msg["content"].(string); ok {
				return content, nil
			}
		}
	}

	return string(body), nil
}
