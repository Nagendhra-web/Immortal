package narrator

import "github.com/Nagendhra-web/Immortal/internal/llm"

// FromLLMClient wraps an *llm.Client so it satisfies the Analyzer interface.
// Callers that already have an *llm.Client can pass it into Enrich() without
// writing their own adapter.
//
//	client := llm.New(cfg)
//	pm := narrator.Enrich(ctx, inc, verdict, narrator.FromLLMClient(client))
func FromLLMClient(c *llm.Client) Analyzer {
	if c == nil {
		return nil
	}
	return &llmClientAdapter{c: c}
}

type llmClientAdapter struct {
	c *llm.Client
}

func (a *llmClientAdapter) IsEnabled() bool {
	if a.c == nil {
		return false
	}
	return a.c.IsEnabled()
}

func (a *llmClientAdapter) Analyze(system, user string) (*AnalyzerResponse, error) {
	if a.c == nil {
		return nil, nil
	}
	r, err := a.c.Analyze(system, user)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, nil
	}
	return &AnalyzerResponse{
		Content:    r.Content,
		Model:      r.Model,
		TokensUsed: r.TokensUsed,
	}, nil
}
