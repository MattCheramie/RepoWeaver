package llm

import (
	"context"
	"errors"
)

// OpenAI calls the OpenAI Chat Completions API.
type OpenAI struct {
	apiKey string
	model  string
}

// NewOpenAI constructs an OpenAI provider.
func NewOpenAI(apiKey, model string) *OpenAI {
	if model == "" {
		model = "gpt-4o"
	}
	return &OpenAI{apiKey: apiKey, model: model}
}

// Name implements Provider.
func (o *OpenAI) Name() string { return "openai(" + o.model + ")" }

// Complete implements Provider via the Chat Completions API.
func (o *OpenAI) Complete(ctx context.Context, system, prompt string) (string, error) {
	if o.apiKey == "" {
		return "", errMissingKey("openai")
	}
	body := map[string]any{
		"model": o.model,
		"messages": []map[string]any{
			{"role": "system", "content": system},
			{"role": "user", "content": prompt},
		},
	}
	headers := map[string]string{"Authorization": "Bearer " + o.apiKey}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := postJSON(ctx, "https://api.openai.com/v1/chat/completions", headers, body, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", errors.New("openai: empty response")
	}
	return out.Choices[0].Message.Content, nil
}
