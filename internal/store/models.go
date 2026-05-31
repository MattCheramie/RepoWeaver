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
