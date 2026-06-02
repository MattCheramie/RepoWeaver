package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mattcheramie/repoweaver/internal/analytics"
	"github.com/mattcheramie/repoweaver/internal/llm"
	"github.com/mattcheramie/repoweaver/internal/seo"
	"github.com/mattcheramie/repoweaver/internal/store"
)

// pageData is the common shape passed to the layout template.
type pageData struct {
	Title    string
	Active   string
	Provider string
	// page-specific fields below
	Repos          []store.Repo
	Repo           store.Repo
	Counts         map[string]int
	Total          int
	Clusters       []store.Cluster
	Topics         []topicView
	AnyResearching bool
	Content        any
	SEO            seo.Meta
	Keywords       []seo.KeywordStat
	Calendar       *calendarView

	// Analytics dashboard
	AnalyticsName  string
	AnalyticsReady bool
	AnalyticsError string
	Analytics      []analyticsRow
	ChartJSON      template.JS // JSON array of {label,views,bounce} for the chart
	OAuthAvailable bool        // GA4 browser consent flow is configured
}

// analyticsRow pairs a tracked content item with its performance metrics.
type analyticsRow struct {
	Content store.Content
	Slug    string
	Metrics analytics.Metrics
}

// topicView decorates a stored topic with parsed sources for templating.
type topicView struct {
	store.Topic
	Sources []llm.Source
}

// Researching reports whether this topic is mid-research (drives UI polling).
func (t topicView) Researching() bool { return t.Status == store.TopicResearching }

// topicViews loads a repo's topics and parses each one's sources JSON.
func (s *Server) topicViews(repoID int64) []topicView {
	topics, err := s.store.ListTopics(repoID)
	if err != nil {
		return nil
	}
	out := make([]topicView, 0, len(topics))
	for _, t := range topics {
		var src []llm.Source
		_ = json.Unmarshal([]byte(t.Sources), &src)
		out = append(out, topicView{Topic: t, Sources: src})
	}
	return out
}

// anyResearching reports whether any topic is still being researched, so the
// template can keep (or stop) HTMX polling.
func anyResearching(topics []topicView) bool {
	for _, t := range topics {
		if t.Researching() {
			return true
		}
	}
	return false
}

func (s *Server) base(title, active string) pageData {
	return pageData{Title: title, Active: active, Provider: s.provider.Name()}
}

// render renders a full page (layout + the page's "content" block).
func (s *Server) render(w http.ResponseWriter, page string, data pageData) {
	t, ok := s.pages[page]
	if !ok {
		http.Error(w, "unknown page: "+page, http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

// renderFragment renders a named sub-template defined in hub.html (kept for the
// clusters partial).
func (s *Server) renderFragment(w http.ResponseWriter, fragment string, data pageData) {
	s.renderNamed(w, "hub.html", fragment, data)
}

// renderNamed renders a named sub-template (HTMX partial) from a specific
// page's template set.
func (s *Server) renderNamed(w http.ResponseWriter, page, fragment string, data pageData) {
	t, ok := s.pages[page]
	if !ok {
		http.Error(w, "unknown page: "+page, http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, fragment, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

// hint writes a small HTMX status message.
func (s *Server) hint(w http.ResponseWriter, msg string, isErr bool) {
	class := "ok"
	if isErr {
		class = "error"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<span class="%s">%s</span>`, class, template.HTMLEscapeString(msg))
}

// --- Repos ---

func (s *Server) handleRepos(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	repos, err := s.store.ListRepos()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	d := s.base("Repos", "repos")
	d.Repos = repos
	s.render(w, "repos.html", d)
}

func (s *Server) handleAddRepo(w http.ResponseWriter, r *http.Request) {
	owner, name, ok := parseRepoInput(r.FormValue("repo"))
	if !ok {
		s.hint(w, "Enter a repository as owner/name.", true)
		return
	}
	repo, err := s.store.AddRepo(owner, name)
	if err != nil {
		s.hint(w, "Could not add repo: "+err.Error(), true)
		return
	}
	res, err := s.ingester.Run(r.Context(), repo)
	if err != nil {
		s.hint(w, err.Error(), true)
		return
	}
	msg := fmt.Sprintf("Ingested %s — %d items (", repo.FullName(), res.Total)
	var parts []string
	for k, v := range res.Counts {
		parts = append(parts, fmt.Sprintf("%s: %d", k, v))
	}
	msg += strings.Join(parts, ", ") + "). "
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<span class="ok">%s</span> <a class="btn-link" href="/repos/%d/hub">Open Hub →</a>`,
		template.HTMLEscapeString(msg), repo.ID)
}

// --- Hub / analyze / generate ---

func (s *Server) handleHub(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.repoFromPath(w, r)
	if !ok {
		return
	}
	counts, _ := s.store.CountItemsByKind(repo.ID)
	total := 0
	for _, n := range counts {
		total += n
	}
	clusters, _ := s.store.ListClusters(repo.ID)
	topics := s.topicViews(repo.ID)
	d := s.base("Hub — "+repo.FullName(), "repos")
	d.Repo, d.Counts, d.Total, d.Clusters = repo, counts, total, clusters
	d.Topics, d.AnyResearching = topics, anyResearching(topics)
	s.render(w, "hub.html", d)
}

// handleTopics returns the topics fragment for a repo. Used by HTMX polling so
// the Hub reflects background research progress.
func (s *Server) handleTopics(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.repoFromPath(w, r)
	if !ok {
		return
	}
	topics := s.topicViews(repo.ID)
	d := s.base("", "repos")
	d.Repo, d.Topics, d.AnyResearching = repo, topics, anyResearching(topics)
	s.renderNamed(w, "hub.html", "topics", d)
}

// handleResearchTopic kicks off (or retries) research for a single topic in the
// background, then returns the refreshed topics fragment.
func (s *Server) handleResearchTopic(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	topic, err := s.store.TopicByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.analyzer.ResearchTopic(id); err != nil {
		s.hint(w, "Research failed: "+err.Error(), true)
		return
	}
	topics := s.topicViews(topic.RepoID)
	d := s.base("", "repos")
	d.Repo, _ = s.store.RepoByID(topic.RepoID)
	d.Topics, d.AnyResearching = topics, anyResearching(topics)
	s.renderNamed(w, "hub.html", "topics", d)
}

// handleGenerateFromTopic generates a standalone content draft from a
// researched topic and links to it.
func (s *Server) handleGenerateFromTopic(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	contentID, err := s.analyzer.GenerateFromTopic(r.Context(), id)
	if err != nil {
		s.hint(w, "Generation failed: "+err.Error(), true)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<span class="ok">Generated!</span> <a class="btn-link" href="/content/%d">View</a> · <a class="btn-link" href="/library">Library</a>`, contentID)
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	repo, ok := s.repoFromPath(w, r)
	if !ok {
		return
	}
	if _, err := s.analyzer.Run(r.Context(), repo.ID); err != nil {
		s.hint(w, "Analysis failed: "+err.Error(), true)
		return
	}
	// Research the freshly-identified topic gaps in the background; the Hub polls
	// the topics fragment to reflect progress.
	s.analyzer.StartResearch(repo.ID)

	clusters, _ := s.store.ListClusters(repo.ID)
	topics := s.topicViews(repo.ID)
	d := s.base("", "repos")
	d.Repo, d.Clusters = repo, clusters
	d.Topics, d.AnyResearching = topics, anyResearching(topics)
	s.renderFragment(w, "analyze-result", d)
}

func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	contentID, err := s.analyzer.Generate(r.Context(), id)
	if err != nil {
		s.hint(w, "Generation failed: "+err.Error(), true)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<span class="ok">Generated!</span> <a class="btn-link" href="/content/%d">View</a> · <a class="btn-link" href="/library">Library</a>`, contentID)
}

// --- Library / content ---

func (s *Server) handleLibrary(w http.ResponseWriter, r *http.Request) {
	content, err := s.store.ListContent()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	d := s.base("Library", "library")
	d.Content = content
	s.render(w, "library.html", d)
}

func (s *Server) handleContent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	c, err := s.store.ContentByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	d := s.base(c.Title, "library")
	d.Content = c
	d.SEO = seo.Parse(c.SEOMeta)
	d.Keywords = seo.KeywordDensity(c.Body, 10)
	s.render(w, "content.html", d)
}

// handleRegenerateSEO recomputes and persists SEO metadata, returning the SEO
// panel fragment for HTMX swap.
func (s *Server) handleRegenerateSEO(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	meta, err := s.analyzer.RegenerateSEO(r.Context(), id)
	if err != nil {
		s.hint(w, "SEO update failed: "+err.Error(), true)
		return
	}
	c, err := s.store.ContentByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	d := s.base("", "library")
	d.Content = c
	d.SEO = meta
	d.Keywords = seo.KeywordDensity(c.Body, 10)
	s.renderNamed(w, "content.html", "seo-panel", d)
}

func (s *Server) handleSaveContent(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.UpdateContentBody(id, r.FormValue("body")); err != nil {
		s.hint(w, "Save failed: "+err.Error(), true)
		return
	}
	s.hint(w, "Saved.", false)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	c, err := s.store.ContentByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	body := c.Body
	// ?fm=1 prepends YAML frontmatter generated from the SEO metadata.
	if r.URL.Query().Get("fm") == "1" {
		body = seo.Parse(c.SEOMeta).Frontmatter(c.Title) + body
	}
	filename := slugify(c.Title) + ".md"
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	_, _ = w.Write([]byte(body))
}

// handleSchedule sets or clears a content row's publish date. An empty "date"
// unschedules. Responds with the re-rendered calendar fragment for HTMX swap.
func (s *Server) handleSchedule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var when *time.Time
	if raw := strings.TrimSpace(r.FormValue("date")); raw != "" {
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			s.hint(w, "Invalid date.", true)
			return
		}
		when = &t
	}
	if err := s.store.SetSchedule(id, when, time.Now().UTC()); err != nil {
		s.hint(w, "Schedule failed: "+err.Error(), true)
		return
	}
	month := r.FormValue("month")
	d := s.base("", "calendar")
	d.Calendar = s.buildCalendar(month)
	s.renderNamed(w, "calendar.html", "calendar-root", d)
}

// handleAnalytics renders the performance dashboard, mapping analytics metrics
// onto scheduled/published content. When no provider is configured it shows a
// setup prompt instead.
func (s *Server) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	d := s.base("Analytics", "analytics")
	d.AnalyticsName = s.analytics.Name()
	d.AnalyticsReady = s.analytics.Configured()
	d.OAuthAvailable = s.oauthEnabled()

	if !d.AnalyticsReady {
		s.render(w, "analytics.html", d)
		return
	}

	content, err := s.store.ListContent()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Only published/scheduled posts are tracked; key them by SEO slug.
	var slugs []string
	bySlug := map[string]store.Content{}
	for _, c := range content {
		if c.ScheduledFor == nil {
			continue
		}
		slug := seo.Parse(c.SEOMeta).Slug
		if slug == "" {
			slug = slugify(c.Title)
		}
		slugs = append(slugs, slug)
		bySlug[slug] = c
	}

	metrics, err := s.analytics.Report(r.Context(), slugs)
	if err != nil {
		d.AnalyticsError = err.Error()
		s.render(w, "analytics.html", d)
		return
	}

	rows := make([]analyticsRow, 0, len(slugs))
	for _, slug := range slugs {
		rows = append(rows, analyticsRow{Content: bySlug[slug], Slug: slug, Metrics: metrics[slug]})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Metrics.Pageviews > rows[j].Metrics.Pageviews
	})
	d.Analytics = rows
	d.ChartJSON = chartData(rows)
	s.render(w, "analytics.html", d)
}

// chartPoint is one bar in the analytics chart.
type chartPoint struct {
	Label  string  `json:"label"`
	Views  int64   `json:"views"`
	Bounce float64 `json:"bounce"` // 0..1
}

// chartData marshals the dashboard rows for the client-side canvas chart. The
// result is injected as a trusted JS value (template.JS); encoding/json escapes
// <, >, & by default, keeping it safe inside a <script> element.
func chartData(rows []analyticsRow) template.JS {
	pts := make([]chartPoint, 0, len(rows))
	for _, r := range rows {
		pts = append(pts, chartPoint{
			Label:  r.Content.Title,
			Views:  r.Metrics.Pageviews,
			Bounce: r.Metrics.BounceRate,
		})
	}
	b, err := json.Marshal(pts)
	if err != nil {
		return template.JS("[]")
	}
	return template.JS(b)
}
