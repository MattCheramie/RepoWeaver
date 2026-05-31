package llm

import (
	"context"
	"errors"
	"net/url"
)

// Gemini calls the Google Gemini generateContent API.
type Gemini struct {
	apiKey string
	model  string
}

// NewGemini constructs a Gemini provider.
func NewGemini(apiKey, model string) *Gemini {
	if model == "" {
		model = "gemini-1.5-pro"
	}
	return &Gemini{apiKey: apiKey, model: model}
}

// Name implements Provider.
func (g *Gemini) Name() string { return "gemini(" + g.model + ")" }

// Complete implements Provider via the generateContent endpoint. The system
// instruction is supplied via systemInstruction.
func (g *Gemini) Complete(ctx context.Context, system, prompt string) (string, error) {
	if g.apiKey == "" {
		return "", errMissingKey("gemini")
	}
	endpoint := "https://generativelanguage.googleapis.com/v1beta/models/" +
		g.model + ":generateContent?key=" + url.QueryEscape(g.apiKey)
	body := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []map[string]any{{"text": system}},
		},
		"contents": []map[string]any{
			{"role": "user", "parts": []map[string]any{{"text": prompt}}},
		},
	}
	var out struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := postJSON(ctx, endpoint, nil, body, &out); err != nil {
		return "", err
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("gemini: empty response")
	}
	var sb string
	for _, p := range out.Candidates[0].Content.Parts {
		sb += p.Text
	}
	return sb, nil
}
