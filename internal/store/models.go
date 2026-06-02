package store

import "time"

// Repo is a GitHub repository that has been (or will be) ingested.
type Repo struct {
	ID             int64
	Owner          string
	Name           string
	AddedAt        time.Time
	LastIngestedAt *time.Time
	Status         string // "new", "ingesting", "ready", "error"
}

// FullName returns "owner/name".
func (r Repo) FullName() string { return r.Owner + "/" + r.Name }

// Item kinds ingested from a repository. "Total capture" per the README —
// bot PRs, dependency bumps, and unresolved issues are all retained.
const (
	KindPR      = "pr"
	KindIssue   = "issue"
	KindComment = "comment"
	KindCommit  = "commit"
	KindDoc     = "doc"
	KindPDF     = "pdf"
)

// Item is a single raw unit of ingested intelligence.
type Item struct {
	ID         int64
	RepoID     int64
	Kind       string
	ExternalID string // PR/issue number, commit SHA, or file path
	Title      string
	Body       string
	State      string // "open", "closed", "merged", "" for docs/commits
	Author     string
	URL        string
	CreatedAt  time.Time
}

// Cluster target output formats.
const (
	FormatBlog        = "blog"
	FormatTutorial    = "tutorial"
	FormatVideoScript = "video_script"
	FormatDeepDive    = "deep_dive"
)

// Cluster is a group of related items the LLM identified as one story.
type Cluster struct {
	ID           int64
	RepoID       int64
	Title        string
	Summary      string
	Narrative    string
	TargetFormat string
	CreatedAt    time.Time
	ItemCount    int // populated on read for convenience
}

// Topic research lifecycle states.
const (
	TopicIdentified  = "identified"  // found during analysis, not yet researched
	TopicResearching = "researching" // research in progress (background)
	TopicResearched  = "researched"  // briefing + sources stored
	TopicError       = "error"       // research failed; see Error
	TopicUnsupported = "unsupported" // provider can't do live web research
)

// Topic is a subject the repository touches on but does not cover in detail.
// Once researched, its briefing and sources become a reusable knowledge-base
// entry for content creation. The sources column is stored as a JSON string;
// the analyze package converts it to/from typed source records to keep this
// package free of an llm dependency.
type Topic struct {
	ID           int64
	RepoID       int64
	Name         string
	Rationale    string
	Status       string
	Research     string
	Sources      string // JSON array of {"title","url"}
	Error        string
	CreatedAt    time.Time
	ResearchedAt *time.Time
}

// Content is a generated, publishable output stored in the library.
type Content struct {
	ID           int64
	ClusterID    int64
	RepoID       int64
	Title        string
	Format       string
	Body         string // markdown
	SEOMeta      string // JSON: meta_description, keywords, slug, tags
	Status       string // "draft", "scheduled", "published"
	ScheduledFor *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
