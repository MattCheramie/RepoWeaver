package llm

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"strings"
)

// Mock is a deterministic, keyless provider used for offline development and
// tests. It inspects the prompt and returns plausible, well-formed output so
// the full ingest -> analyze -> generate pipeline can run end to end.
type Mock struct{}

// NewMock returns a Mock provider.
func NewMock() *Mock { return &Mock{} }

// Name implements Provider.
func (m *Mock) Name() string { return "mock" }

// Complete implements Provider. It detects whether the caller wants SEO JSON,
// clustering JSON, or prose (content generation) based on the system prompt.
func (m *Mock) Complete(_ context.Context, system, prompt string) (string, error) {
	low := strings.ToLower(system)
	switch {
	case strings.Contains(low, "meta_description"):
		return m.seoJSON(), nil
	case strings.Contains(low, "json"):
		return m.clusterJSON(prompt), nil
	default:
		return m.prose(prompt), nil
	}
}

// seoJSON returns a small, valid SEO metadata document.
func (m *Mock) seoJSON() string {
	doc := map[string]any{
		"meta_description": "A mock-generated summary. Configure a real LLM provider for tailored SEO copy.",
		"tags":             []string{"mock", "repoweaver", "documentation"},
	}
	b, _ := json.Marshal(doc)
	return string(b)
}

// clusterJSON returns a small, valid clusters JSON document. It always
// produces at least one cluster so downstream steps have something to work on.
func (m *Mock) clusterJSON(prompt string) string {
	doc := map[string]any{
		"clusters": []map[string]any{
			{
				"title":         "Core Feature Evolution",
				"summary":       "How the project's primary capabilities took shape across PRs and issues.",
				"narrative":     "The narrative below is mock-generated. Set a real LLM_PROVIDER to synthesize the actual problem-solving story from the ingested history.",
				"target_format": "blog",
				"item_indices":  pickIndices(prompt, 0),
			},
			{
				"title":         "Fixes & Salvaged Insights",
				"summary":       "Bug fixes and useful snippets recovered from unresolved discussions.",
				"narrative":     "Mock salvage operation: real providers scan open issues for usable code and theory.",
				"target_format": "tutorial",
				"item_indices":  pickIndices(prompt, 1),
			},
		},
	}
	b, _ := json.Marshal(doc)
	return string(b)
}

// pickIndices deterministically chooses a couple of item indices from the
// prompt so each cluster references some real ingested items.
func pickIndices(prompt string, offset int) []int {
	// Count how many "[n]" item markers appear in the prompt.
	n := strings.Count(prompt, "] ")
	if n == 0 {
		return []int{}
	}
	var out []int
	for i := offset; i < n; i += 2 {
		out = append(out, i)
		if len(out) >= 5 {
			break
		}
	}
	return out
}

func (m *Mock) prose(prompt string) string {
	h := sha1.Sum([]byte(prompt))
	return fmt.Sprintf(`# Mock-Generated Content

> This document was produced by RepoWeaver's **mock** LLM provider. Configure a
> real provider (Claude, OpenAI, or Gemini) via `+"`LLM_PROVIDER`"+` and
> `+"`LLM_API_KEY`"+` to generate genuine content.

## Overview

This is a placeholder article synthesized from the supplied repository context.
A production run would weave the ingested PRs, issues, and documentation into a
polished narrative here.

## Key Points

- Mock content is deterministic for reproducible tests.
- The structure mirrors what a real generation would emit.
- Markdown is valid and ready to download.

_Context fingerprint: %x_
`, h[:6])
}
