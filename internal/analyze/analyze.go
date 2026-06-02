// Package analyze is "the Brain": it passes ingested context to an LLM to map
// the problem-solving narrative, cluster related items, and generate content.
package analyze

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mattcheramie/repoweaver/internal/llm"
	"github.com/mattcheramie/repoweaver/internal/seo"
	"github.com/mattcheramie/repoweaver/internal/store"
)

// maxItemsForContext bounds how many items we include in a single prompt.
const maxItemsForContext = 60

// bodyExcerpt caps each item's body length in the prompt.
const bodyExcerpt = 600

// Analyzer orchestrates LLM-driven synthesis over a repo's ingested items.
type Analyzer struct {
	store    *store.Store
	provider llm.Provider

	mu          sync.Mutex     // guards researching
	researching map[int64]bool // repoID -> background research in flight
}

// New returns an Analyzer.
func New(s *store.Store, p llm.Provider) *Analyzer {
	return &Analyzer{store: s, provider: p, researching: map[int64]bool{}}
}

const clusterSystem = `You are RepoWeaver's analysis engine. You map an open-source repository's
organic problem-solving history (PRs, issues, comments, commits) against its
formal documentation. Group related items into coherent story clusters, each
suitable for a single piece of content. Also run a "salvage" pass: surface
usable snippets or theoretical solutions from unresolved issues.

Respond with ONLY valid JSON in this exact shape:
{"clusters":[{"title":"...","summary":"...","narrative":"...","target_format":"blog|tutorial|video_script|deep_dive","item_indices":[0,2,5]}]}
item_indices reference the numbered items in the user message.`

// clusterResponse is the JSON contract returned by the provider.
type clusterResponse struct {
	Clusters []struct {
		Title        string `json:"title"`
		Summary      string `json:"summary"`
		Narrative    string `json:"narrative"`
		TargetFormat string `json:"target_format"`
		ItemIndices  []int  `json:"item_indices"`
	} `json:"clusters"`
}

// Run analyzes a repo's items, producing and persisting clusters.
func (a *Analyzer) Run(ctx context.Context, repoID int64) (int, error) {
	items, err := a.store.ListItems(repoID, maxItemsForContext)
	if err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, fmt.Errorf("no items to analyze; ingest the repo first")
	}

	prompt := buildPrompt(items)
	raw, err := a.provider.Complete(ctx, clusterSystem, prompt)
	if err != nil {
		return 0, fmt.Errorf("llm complete: %w", err)
	}

	var resp clusterResponse
	if err := json.Unmarshal([]byte(extractJSON(raw)), &resp); err != nil {
		return 0, fmt.Errorf("parse llm response: %w (raw: %.200s)", err, raw)
	}

	clusters := make([]store.Cluster, 0, len(resp.Clusters))
	members := make([][]int64, 0, len(resp.Clusters))
	for _, c := range resp.Clusters {
		clusters = append(clusters, store.Cluster{
			RepoID:       repoID,
			Title:        c.Title,
			Summary:      c.Summary,
			Narrative:    c.Narrative,
			TargetFormat: normalizeFormat(c.TargetFormat),
		})
		var ids []int64
		for _, idx := range c.ItemIndices {
			if idx >= 0 && idx < len(items) {
				ids = append(ids, items[idx].ID)
			}
		}
		members = append(members, ids)
	}

	if err := a.store.ReplaceClusters(repoID, clusters, members); err != nil {
		return 0, err
	}

	// Identify topic gaps in the same pass (fast, one LLM call). Failure here is
	// non-fatal: clustering already succeeded and is what the caller awaits.
	if _, err := a.IdentifyTopics(ctx, repoID, items); err != nil {
		return len(clusters), nil
	}
	return len(clusters), nil
}

const generateSystem = `You are RepoWeaver's content generator. Produce a polished, SEO-aware piece
of developer content in clean Markdown based on the provided repository
narrative and source items. Start with an H1 title. Be concrete and technical.
Do not include front matter; output Markdown body only.`

// Generate produces Markdown content for a single cluster and stores it.
func (a *Analyzer) Generate(ctx context.Context, clusterID int64) (int64, error) {
	cluster, err := a.store.ClusterByID(clusterID)
	if err != nil {
		return 0, err
	}
	items, err := a.store.ClusterItems(clusterID)
	if err != nil {
		return 0, err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Target format: %s\n", cluster.TargetFormat)
	fmt.Fprintf(&b, "Cluster title: %s\n", cluster.Title)
	fmt.Fprintf(&b, "Summary: %s\n", cluster.Summary)
	fmt.Fprintf(&b, "Narrative: %s\n\nSource items:\n", cluster.Narrative)
	b.WriteString(formatItems(items))
	b.WriteString(researchContext(a.store, cluster.RepoID))

	body, err := a.provider.Complete(ctx, generateSystem, b.String())
	if err != nil {
		return 0, fmt.Errorf("llm generate: %w", err)
	}

	title := firstHeading(body)
	if title == "" {
		title = cluster.Title
	}
	meta := seo.Generate(ctx, a.provider, title, body)
	return a.store.CreateContent(store.Content{
		ClusterID: cluster.ID,
		RepoID:    cluster.RepoID,
		Title:     title,
		Format:    cluster.TargetFormat,
		Body:      body,
		SEOMeta:   meta.JSON(),
	})
}

// RegenerateSEO recomputes SEO metadata for an existing content row from its
// current body and persists it.
func (a *Analyzer) RegenerateSEO(ctx context.Context, contentID int64) (seo.Meta, error) {
	c, err := a.store.ContentByID(contentID)
	if err != nil {
		return seo.Meta{}, err
	}
	meta := seo.Generate(ctx, a.provider, c.Title, c.Body)
	if err := a.store.UpdateContentSEO(contentID, meta.JSON()); err != nil {
		return seo.Meta{}, err
	}
	return meta, nil
}

// buildPrompt renders numbered items for the clustering prompt.
func buildPrompt(items []store.Item) string {
	var b strings.Builder
	b.WriteString("Repository items (index] kind: title — body excerpt):\n")
	b.WriteString(formatItems(items))
	return b.String()
}

func formatItems(items []store.Item) string {
	var b strings.Builder
	for i, it := range items {
		body := strings.ReplaceAll(it.Body, "\n", " ")
		if len(body) > bodyExcerpt {
			body = body[:bodyExcerpt] + "…"
		}
		title := it.Title
		if title == "" {
			title = "(no title)"
		}
		fmt.Fprintf(&b, "%d] %s [%s]: %s — %s\n", i, it.Kind, it.State, title, body)
	}
	return b.String()
}

func normalizeFormat(f string) string {
	switch strings.ToLower(strings.TrimSpace(f)) {
	case store.FormatTutorial:
		return store.FormatTutorial
	case store.FormatVideoScript, "video", "script":
		return store.FormatVideoScript
	case store.FormatDeepDive, "deep-dive", "deepdive":
		return store.FormatDeepDive
	default:
		return store.FormatBlog
	}
}

// extractJSON pulls the first {...} JSON object out of a model response, which
// may be wrapped in prose or code fences.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

func firstHeading(md string) string {
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}
