// Package seo implements RepoWeaver's SEO toolkit: keyword density analysis,
// URL slug suggestions, AI-assisted meta-description generation, and semantic
// tag extraction for blog-post frontmatter.
package seo

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Meta is the SEO metadata persisted on a content row (content.seo_meta) and
// emitted into Markdown frontmatter. JSON tags match the stored schema.
type Meta struct {
	MetaDescription string   `json:"meta_description"`
	Keywords        []string `json:"keywords"`
	Slug            string   `json:"slug"`
	Tags            []string `json:"tags"`
}

// KeywordStat is a single keyword-density measurement.
type KeywordStat struct {
	Word    string
	Count   int
	Density float64 // percent of total meaningful words
}

// Completer is the subset of llm.Provider that seo needs. Declaring it locally
// avoids an import cycle and keeps the package easy to test.
type Completer interface {
	Complete(ctx context.Context, system, prompt string) (string, error)
}

// Empty returns a zero-value Meta serialized as JSON ("{}"-equivalent shape).
func Empty() string {
	b, _ := json.Marshal(Meta{Keywords: []string{}, Tags: []string{}})
	return string(b)
}

// Parse decodes stored seo_meta JSON, tolerating empty/legacy values.
func Parse(s string) Meta {
	var m Meta
	if strings.TrimSpace(s) == "" {
		return Meta{Keywords: []string{}, Tags: []string{}}
	}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return Meta{Keywords: []string{}, Tags: []string{}}
	}
	if m.Keywords == nil {
		m.Keywords = []string{}
	}
	if m.Tags == nil {
		m.Tags = []string{}
	}
	return m
}

// JSON serializes a Meta for storage.
func (m Meta) JSON() string {
	if m.Keywords == nil {
		m.Keywords = []string{}
	}
	if m.Tags == nil {
		m.Tags = []string{}
	}
	b, _ := json.Marshal(m)
	return string(b)
}

const metaSystem = `You are an SEO assistant. Given a Markdown article, respond with ONLY valid
JSON of the form {"meta_description":"...","tags":["tag1","tag2"]}.
The meta_description must be a compelling summary of at most 155 characters.
Provide 3-6 concise, lower-case semantic tags relevant to developers.`

// metaResponse is the JSON contract for the AI-assisted step.
type metaResponse struct {
	MetaDescription string   `json:"meta_description"`
	Tags            []string `json:"tags"`
}

// Generate builds SEO metadata for an article. Keyword density and the slug are
// computed deterministically (offline); the meta description and semantic tags
// are AI-assisted via the provider, with a heuristic fallback if the call fails
// or returns unusable output.
func Generate(ctx context.Context, c Completer, title, body string) Meta {
	stats := KeywordDensity(body, 10)
	keywords := make([]string, 0, len(stats))
	for _, s := range stats {
		keywords = append(keywords, s.Word)
	}

	m := Meta{
		Slug:     Slug(title),
		Keywords: keywords,
	}

	if c != nil {
		if raw, err := c.Complete(ctx, metaSystem, "Title: "+title+"\n\nArticle:\n"+body); err == nil {
			var mr metaResponse
			if err := json.Unmarshal([]byte(extractJSON(raw)), &mr); err == nil {
				m.MetaDescription = truncate(strings.TrimSpace(mr.MetaDescription), 155)
				m.Tags = cleanTags(mr.Tags)
			}
		}
	}

	// Heuristic fallbacks keep the toolkit useful offline.
	if m.MetaDescription == "" {
		m.MetaDescription = heuristicDescription(body)
	}
	if len(m.Tags) == 0 {
		m.Tags = topN(keywords, 5)
	}
	return m
}

// Frontmatter renders the metadata as a YAML frontmatter block suitable for
// prepending to a Markdown file for static-site generators.
func (m Meta) Frontmatter(title string) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %q\n", title)
	fmt.Fprintf(&b, "description: %q\n", m.MetaDescription)
	if m.Slug != "" {
		fmt.Fprintf(&b, "slug: %q\n", m.Slug)
	}
	b.WriteString("tags: [")
	b.WriteString(quoteJoin(m.Tags))
	b.WriteString("]\n")
	b.WriteString("keywords: [")
	b.WriteString(quoteJoin(m.Keywords))
	b.WriteString("]\n")
	b.WriteString("---\n\n")
	return b.String()
}

// KeywordDensity returns the top-n meaningful keywords by frequency, with each
// word's share of total meaningful words. Markdown syntax and stopwords are
// stripped. Results are sorted by count (desc), then alphabetically.
func KeywordDensity(text string, n int) []KeywordStat {
	words := tokenize(stripMarkdown(text))
	counts := map[string]int{}
	total := 0
	for _, w := range words {
		if len(w) < 3 || stopwords[w] {
			continue
		}
		counts[w]++
		total++
	}
	if total == 0 {
		return nil
	}
	stats := make([]KeywordStat, 0, len(counts))
	for w, c := range counts {
		stats = append(stats, KeywordStat{
			Word:    w,
			Count:   c,
			Density: float64(c) / float64(total) * 100,
		})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count != stats[j].Count {
			return stats[i].Count > stats[j].Count
		}
		return stats[i].Word < stats[j].Word
	})
	if n > 0 && len(stats) > n {
		stats = stats[:n]
	}
	return stats
}

// Slug converts a title into a URL-safe slug.
func Slug(title string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// --- internals ---

func tokenize(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
}

// stripMarkdown removes fenced code blocks, inline code, and common Markdown
// punctuation so keyword counts reflect prose.
func stripMarkdown(text string) string {
	var b strings.Builder
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	s := b.String()
	for _, ch := range []string{"`", "#", ">", "*", "_", "[", "]", "(", ")"} {
		s = strings.ReplaceAll(s, ch, " ")
	}
	return s
}

// heuristicDescription extracts the first sentence(s) of prose up to 155 chars.
func heuristicDescription(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ">") ||
			strings.HasPrefix(line, "```") || strings.HasPrefix(line, "-") {
			continue
		}
		line = strings.NewReplacer("`", "", "*", "", "_", "").Replace(line)
		return truncate(strings.TrimSpace(line), 155)
	}
	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	if i := strings.LastIndex(cut, " "); i > max/2 {
		cut = cut[:i]
	}
	return strings.TrimSpace(cut) + "…"
}

func cleanTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
		if len(out) >= 6 {
			break
		}
	}
	return out
}

func topN(words []string, n int) []string {
	if len(words) > n {
		return append([]string{}, words[:n]...)
	}
	return append([]string{}, words...)
}

func quoteJoin(items []string) string {
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = fmt.Sprintf("%q", it)
	}
	return strings.Join(parts, ", ")
}

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
