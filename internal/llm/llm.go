// Package llm defines a pluggable LLM provider interface and implementations
// for Anthropic Claude (default), OpenAI, Google Gemini, and a keyless mock.
package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/mattcheramie/repoweaver/internal/config"
)

// Provider is the minimal contract every LLM backend implements.
type Provider interface {
	// Complete sends a system instruction and a user prompt and returns the
	// model's text response.
	Complete(ctx context.Context, system, prompt string) (string, error)
	// Name returns a human-readable provider identifier.
	Name() string
}

// New constructs a Provider from configuration. Unknown or empty providers
// fall back to the mock so the app always runs.
func New(cfg config.Config) Provider {
	switch strings.ToLower(cfg.LLMProvider) {
	case "claude", "anthropic":
		return NewClaude(cfg.LLMAPIKey, cfg.LLMModel)
	case "openai":
		return NewOpenAI(cfg.LLMAPIKey, cfg.LLMModel)
	case "gemini", "google":
		return NewGemini(cfg.LLMAPIKey, cfg.LLMModel)
	case "mock", "":
		return NewMock()
	default:
		return NewMock()
	}
}

// errMissingKey is returned by real providers when no API key is configured.
func errMissingKey(provider string) error {
	return fmt.Errorf("%s provider requires LLM_API_KEY to be set", provider)
}
