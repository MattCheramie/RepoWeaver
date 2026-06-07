package analyze

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mattcheramie/repoweaver/internal/llm"
	"github.com/mattcheramie/repoweaver/internal/seo"
	"github.com/mattcheramie/repoweaver/internal/store"
)

// Research tuning.
const (
	maxResearchWorkers   = 3                // concurrent topics researched
	researchBatchTimeout = 15 * time.Minute // overall budget for one batch
	maxInjectedTopics    = 6                // researched topics fed into generation
	injectedExcerpt      = 1200             // per-topic chars injected into a prompt
)

const topicGapSystem = `You are RepoWeaver's gap finder. From a repository's items and docs, identify
distinct topics the project TOUCHES ON but does not cover in detail — adjacent
concepts, assumed prior knowledge, or briefly-mentioned techniques worth a
deeper, standalone explanation. Avoid topics already covered thoroughly.

Respond with ONLY valid JSON in this exact shape:
{"topics":[{"name":"...","rationale":"..."}]}
name is a short topic title; rationale says why it is touched-but-not-covered.`

type topicGapResponse struct {
	Topics []struct {
		Name      string `json:"name"`
		Rationale string `json:"rationale"`
	} `json:"topics"`
}

// IdentifyTopics asks the LLM for topic gaps over the repo's items and persists
// them (idempotently). It returns the number of topics seen in the response.
func (a *Analyzer) IdentifyTopics(ctx context.Context, repoID int64, items []store.Item) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}
	var b strings.Builder
	b.WriteString("Repository items (index] kind: title — body excerpt):\n")
	b.WriteString(formatItems(items))

	raw, err := a.provider.Complete(ctx, topicGapSystem, b.String())
	if err != nil {
		return 0, fmt.Errorf("llm identify topics: %w", err)
	}
	var resp topicGapResponse
	if err := json.Unmarshal([]byte(extractJSON(raw)), &resp); err != nil {
		return 0, fmt.Errorf("parse topics response: %w (raw: %.200s)", err, raw)
	}
	for _, t := range resp.Topics {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			continue
		}
		if _, err := a.store.UpsertTopic(repoID, name, strings.TrimSpace(t.Rationale)); err != nil {
			return 0, err
		}
	}
	return len(resp.Topics), nil
}

// StartResearch launches background research for a repo's not-yet-researched
// topics and returns immediately. It is a no-op if a batch is already running
// for the repo. When the provider can't do live research, identified topics are
// marked unsupported instead.
func (a *Analyzer) StartResearch(repoID int64) {
	a.mu.Lock()
	if a.researching[repoID] {
		a.mu.Unlock()
		return
	}
	a.researching[repoID] = true
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			delete(a.researching, repoID)
			a.mu.Unlock()
		}()
		ctx, cancel := context.WithTimeout(context.Background(), researchBatchTimeout)
		defer cancel()
		a.researchRepo(ctx, repoID)
	}()
}

// researchRepo researches every "identified" topic for a repo with a bounded
// worker pool. Detached from any HTTP request context.
func (a *Analyzer) researchRepo(ctx context.Context, repoID int64) {
	topics, err := a.store.ListTopics(repoID)
	if err != nil {
		return
	}
	researcher, ok := a.provider.(llm.Researcher)
	if !ok {
		for _, t := range topics {
			if t.Status == store.TopicIdentified {
				_ = a.store.SetTopicStatus(t.ID, store.TopicUnsupported)
			}
		}
		return
	}

	sem := make(chan struct{}, maxResearchWorkers)
	var pending []store.Topic
	for _, t := range topics {
		if t.Status == store.TopicIdentified || t.Status == store.TopicUnsupported {
			pending = append(pending, t)
		}
	}
	done := make(chan struct{}, len(pending))
	for _, t := range pending {
		sem <- struct{}{}
		go func(t store.Topic) {
			defer func() {
				<-sem
				// A worker panic would otherwise crash the process.
				if r := recover(); r != nil {
					_ = a.store.SetTopicError(t.ID, fmt.Sprintf("research panicked: %v", r))
				}
				done <- struct{}{}
			}()
			a.researchOne(ctx, researcher, t)
		}(t)
	}
	for range pending {
		<-done
	}
}

// researchOne runs and persists research for a single topic.
func (a *Analyzer) researchOne(ctx context.Context, researcher llm.Researcher, t store.Topic) {
	if err := a.store.SetTopicStatus(t.ID, store.TopicResearching); err != nil {
		return
	}
	res, err := researcher.Research(ctx, t.Name, t.Rationale)
	if err != nil {
		_ = a.store.SetTopicError(t.ID, err.Error())
		return
	}
	_ = a.store.SaveTopicResearch(t.ID, res.Briefing, marshalSources(res.Sources))
}

// ResearchTopic researches a single topic on demand (the manual Research/Retry
// button), in the background. Detached from the request context.
func (a *Analyzer) ResearchTopic(topicID int64) error {
	t, err := a.store.TopicByID(topicID)
	if err != nil {
		return err
	}
	researcher, ok := a.provider.(llm.Researcher)
	if !ok {
		return a.store.SetTopicStatus(topicID, store.TopicUnsupported)
	}
	if err := a.store.SetTopicStatus(topicID, store.TopicResearching); err != nil {
		return err
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), researchBatchTimeout)
		defer cancel()
		defer func() {
			if r := recover(); r != nil {
				_ = a.store.SetTopicError(topicID, fmt.Sprintf("research panicked: %v", r))
			}
		}()
		a.researchOne(ctx, researcher, t)
	}()
	return nil
}

const topicGenerateSystem = `You are RepoWeaver's content generator. Produce a polished, SEO-aware piece of
developer content in clean Markdown that teaches the given topic, grounded in the
provided research briefing and sources. Start with an H1 title. Be concrete and
technical. Do not include front matter; output Markdown body only.

Use visuals to make the piece clearer. A header banner is generated
automatically, so do not create one. Where they genuinely aid understanding,
embed visuals as fenced code blocks and let the subject matter dictate how many
(usually one to three):

- For quantitative comparisons or trends, add a fenced code block tagged 'chart'
  whose body is JSON of the form:
  {"type":"bar|line|area|pie","title":"...","data":[{"label":"...","value":12}]}
  Use only real numbers grounded in the research — never invent data. If you have
  no real figures, omit the chart.
- For architecture, flows, sequences, or state machines, add a fenced code block
  tagged 'mermaid' with valid Mermaid syntax (for example flowchart TD or
  sequenceDiagram).

Place each visual next to the prose it supports.`

// GenerateFromTopic creates a standalone content draft from a researched topic
// (no cluster) and stores it. Mirrors Generate.
func (a *Analyzer) GenerateFromTopic(ctx context.Context, topicID int64) (int64, error) {
	t, err := a.store.TopicByID(topicID)
	if err != nil {
		return 0, err
	}
	if t.Status != store.TopicResearched {
		return 0, fmt.Errorf("topic %q is not researched yet", t.Name)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Topic: %s\n", t.Name)
	if t.Rationale != "" {
		fmt.Fprintf(&b, "Why it matters: %s\n", t.Rationale)
	}
	fmt.Fprintf(&b, "\nResearch briefing:\n%s\n", t.Research)
	if src := formatSources(t.Sources); src != "" {
		fmt.Fprintf(&b, "\nSources:\n%s", src)
	}

	body, err := a.provider.Complete(ctx, topicGenerateSystem, b.String())
	if err != nil {
		return 0, fmt.Errorf("llm generate: %w", err)
	}
	title := firstHeading(body)
	if title == "" {
		title = t.Name
	}
	meta := seo.Generate(ctx, a.provider, title, body)
	return a.store.CreateContent(store.Content{
		RepoID:  t.RepoID, // ClusterID 0 -> NULL: standalone, topic-sourced draft
		Title:   title,
		Format:  store.FormatDeepDive,
		Body:    body,
		SEOMeta: meta.JSON(),
	})
}

// researchContext renders a bounded "Researched background" block from a repo's
// researched topics, for injection into cluster content generation.
func researchContext(s *store.Store, repoID int64) string {
	topics, err := s.ListTopics(repoID)
	if err != nil {
		return ""
	}
	var b strings.Builder
	n := 0
	for _, t := range topics {
		if t.Status != store.TopicResearched || strings.TrimSpace(t.Research) == "" {
			continue
		}
		if n == 0 {
			b.WriteString("\nResearched background (extra context; weave in and cite where relevant):\n")
		}
		brief := t.Research
		if len(brief) > injectedExcerpt {
			brief = brief[:injectedExcerpt] + "…"
		}
		fmt.Fprintf(&b, "- %s: %s\n", t.Name, strings.ReplaceAll(brief, "\n", " "))
		n++
		if n >= maxInjectedTopics {
			break
		}
	}
	return b.String()
}

// marshalSources encodes typed sources to the JSON string the store persists.
func marshalSources(sources []llm.Source) string {
	if len(sources) == 0 {
		return "[]"
	}
	b, err := json.Marshal(sources)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// formatSources renders a sources JSON array as a Markdown list for prompts.
func formatSources(sourcesJSON string) string {
	var sources []llm.Source
	if err := json.Unmarshal([]byte(sourcesJSON), &sources); err != nil {
		return ""
	}
	var b strings.Builder
	for _, s := range sources {
		fmt.Fprintf(&b, "- %s (%s)\n", s.Title, s.URL)
	}
	return b.String()
}
