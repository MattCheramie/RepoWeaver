package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

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

const researchSystem = `You are RepoWeaver's research assistant. Using web search, research the
given topic thoroughly and write a concise, accurate technical briefing (a few
hundred words) that a content team can rely on. Prefer authoritative, current
sources and ground your claims in them. Output plain prose/Markdown — no JSON.`

// anthropicMessage models the heterogeneous content blocks the Messages API
// returns when the server-side web_search tool runs.
type anthropicMessage struct {
	StopReason string `json:"stop_reason"`
	Content    []struct {
		Type string `json:"type"`

		// type == "text"
		Text      string `json:"text"`
		Citations []struct {
			Type  string `json:"type"` // "web_search_result_location"
			URL   string `json:"url"`
			Title string `json:"title"`
		} `json:"citations"`

		// type == "web_search_tool_result"; content is polymorphic (result
		// array or error object), so decode lazily.
		Content json.RawMessage `json:"content"`
	} `json:"content"`
}

type webSearchResult struct {
	Type  string `json:"type"` // "web_search_result"
	URL   string `json:"url"`
	Title string `json:"title"`
}

// Research implements Researcher via the Anthropic web_search server tool. It is
// a single round-trip: Claude runs the searches itself and returns a mix of
// text, citations, and search-result blocks, which we fold into a briefing plus
// a deduped source list.
func (c *Claude) Research(ctx context.Context, topic, contextHint string) (ResearchResult, error) {
	if c.apiKey == "" {
		return ResearchResult{}, errMissingKey("claude")
	}
	// Bound a single hung topic without relying on the client timeout alone.
	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()

	var prompt strings.Builder
	fmt.Fprintf(&prompt, "Topic to research: %s\n", topic)
	if strings.TrimSpace(contextHint) != "" {
		fmt.Fprintf(&prompt, "Why it matters here: %s\n", contextHint)
	}
	prompt.WriteString("\nResearch this topic and produce the briefing.")

	body := map[string]any{
		"model":      c.model,
		"max_tokens": 4096,
		"system":     researchSystem,
		"messages": []map[string]any{
			{"role": "user", "content": prompt.String()},
		},
		"tools": []map[string]any{
			{"type": "web_search_20250305", "name": "web_search", "max_uses": 5},
		},
	}
	headers := map[string]string{
		"x-api-key":         c.apiKey,
		"anthropic-version": "2023-06-01",
	}

	var out anthropicMessage
	if err := postJSONWith(researchClient, ctx, "https://api.anthropic.com/v1/messages", headers, body, &out); err != nil {
		return ResearchResult{}, err
	}

	var briefing strings.Builder
	seen := map[string]bool{}
	var sources []Source
	addSource := func(url, title string) {
		if url == "" || seen[url] {
			return
		}
		seen[url] = true
		sources = append(sources, Source{Title: title, URL: url})
	}

	for _, blk := range out.Content {
		switch blk.Type {
		case "text":
			briefing.WriteString(blk.Text)
			for _, ci := range blk.Citations {
				addSource(ci.URL, ci.Title)
			}
		case "web_search_tool_result":
			// content may be a result array or an error object; ignore the latter
			// so one failed search doesn't sink the whole topic.
			var results []webSearchResult
			if err := json.Unmarshal(blk.Content, &results); err == nil {
				for _, r := range results {
					addSource(r.URL, r.Title)
				}
			}
		}
	}

	return ResearchResult{Briefing: strings.TrimSpace(briefing.String()), Sources: sources}, nil
}
