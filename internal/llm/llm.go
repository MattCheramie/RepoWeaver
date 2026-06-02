// Package llm defines a pluggable LLM provider interface and implementations
// for Anthropic Claude (default), OpenAI, Google Gemini, and a keyless mock.
package llm

import (
	"context"
	"errors"
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

// Source is a web reference the model used while researching a topic.
type Source struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// ResearchResult is the output of researching a single topic.
type ResearchResult struct {
	Briefing string   `json:"briefing"`
	Sources  []Source `json:"sources"`
}

// Researcher is an OPTIONAL provider capability: backends that can research a
// topic against live web sources implement it. Callers type-assert and fall
// back (e.g. marking the topic unsupported) when it is absent, rather than
// fabricating sources via a plain completion.
type Researcher interface {
	Research(ctx context.Context, topic, contextHint string) (ResearchResult, error)
}

// ErrResearchUnsupported distinguishes "this provider cannot research" from a
// transient research failure.
var ErrResearchUnsupported = errors.New("provider does not support live web research")

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
