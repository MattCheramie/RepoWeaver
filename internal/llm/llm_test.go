package llm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mattcheramie/repoweaver/internal/config"
)

func TestNewDefaultsToMock(t *testing.T) {
	p := New(config.Config{})
	if p.Name() != "mock" {
		t.Fatalf("expected mock by default, got %s", p.Name())
	}
	p = New(config.Config{LLMProvider: "unknown-provider"})
	if p.Name() != "mock" {
		t.Fatalf("expected fallback to mock, got %s", p.Name())
	}
}

func TestNewSelectsProviders(t *testing.T) {
	cases := map[string]string{
		"claude": "claude",
		"openai": "openai",
		"gemini": "gemini",
	}
	for provider, wantPrefix := range cases {
		p := New(config.Config{LLMProvider: provider, LLMAPIKey: "x"})
		if !strings.HasPrefix(p.Name(), wantPrefix) {
			t.Fatalf("provider %s: got name %s", provider, p.Name())
		}
	}
}

func TestMockClusterJSONIsValid(t *testing.T) {
	m := NewMock()
	// Mirrors the real clustering system prompt, which asks for {"clusters":[...]}.
	out, err := m.Complete(context.Background(), `respond with ONLY valid JSON {"clusters":[...]}`, "0] pr: a\n1] issue: b\n2] commit: c\n")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	var doc struct {
		Clusters []map[string]any `json:"clusters"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out)
	}
	if len(doc.Clusters) == 0 {
		t.Fatal("expected at least one cluster")
	}
}

func TestMockProseIsMarkdown(t *testing.T) {
	m := NewMock()
	out, err := m.Complete(context.Background(), "generate content", "some context")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if !strings.HasPrefix(out, "#") {
		t.Fatalf("expected markdown heading, got %.40q", out)
	}
}

func TestRealProvidersErrorWithoutKey(t *testing.T) {
	for _, p := range []Provider{NewClaude("", ""), NewOpenAI("", ""), NewGemini("", "")} {
		if _, err := p.Complete(context.Background(), "s", "p"); err == nil {
			t.Fatalf("%s: expected error without API key", p.Name())
		}
	}
}
