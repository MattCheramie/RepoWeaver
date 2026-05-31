package llm

import "context"

// Claude calls the Anthropic Messages API.
type Claude struct {
	apiKey string
	model  string
}

// NewClaude constructs a Claude provider. Defaults to a current Sonnet model.
func NewClaude(apiKey, model string) *Claude {
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	return &Claude{apiKey: apiKey, model: model}
}

// Name implements Provider.
func (c *Claude) Name() string { return "claude(" + c.model + ")" }

// Complete implements Provider via the Anthropic Messages API.
func (c *Claude) Complete(ctx context.Context, system, prompt string) (string, error) {
	if c.apiKey == "" {
		return "", errMissingKey("claude")
	}
	body := map[string]any{
		"model":      c.model,
		"max_tokens": 4096,
		"system":     system,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
	}
	headers := map[string]string{
		"x-api-key":         c.apiKey,
		"anthropic-version": "2023-06-01",
	}
	var out struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := postJSON(ctx, "https://api.anthropic.com/v1/messages", headers, body, &out); err != nil {
		return "", err
	}
	var sb string
	for _, blk := range out.Content {
		sb += blk.Text
	}
	return sb, nil
}
