package ingest

import (
	"context"
	"log"

	"github.com/mattcheramie/repoweaver/internal/store"
)

// Ingester pulls repository data into the store.
type Ingester struct {
	store *store.Store
	token string
}

// New returns an Ingester. token may be empty for unauthenticated access.
func New(s *store.Store, token string) *Ingester {
	return &Ingester{store: s, token: token}
}

// Result summarizes an ingest run.
type Result struct {
	Counts map[string]int
	Total  int
}

// Run ingests history + static files for a repo and persists items. It is
// resilient: a failure in one phase still saves what was collected by the other.
func (in *Ingester) Run(ctx context.Context, repo store.Repo) (Result, error) {
	_ = in.store.SetRepoStatus(repo.ID, "ingesting")
	gh := newGitHubClient(ctx, in.token)

	var all []store.Item

	history, err := fetchHistory(ctx, gh, repo.ID, repo.Owner, repo.Name)
	if err != nil {
		log.Printf("ingest: history error for %s: %v", repo.FullName(), err)
	}
	all = append(all, history...)

	files, err := fetchFiles(ctx, gh, repo.ID, repo.Owner, repo.Name)
	if err != nil {
		log.Printf("ingest: files error for %s: %v", repo.FullName(), err)
	}
	all = append(all, files...)

	for _, it := range all {
		if _, err := in.store.UpsertItem(it); err != nil {
			log.Printf("ingest: upsert item error: %v", err)
		}
	}

	if len(all) == 0 {
		_ = in.store.SetRepoStatus(repo.ID, "error")
		return Result{}, &IngestError{Repo: repo.FullName()}
	}

	if err := in.store.MarkIngested(repo.ID); err != nil {
		return Result{}, err
	}
	counts, _ := in.store.CountItemsByKind(repo.ID)
	total := 0
	for _, n := range counts {
		total += n
	}
	return Result{Counts: counts, Total: total}, nil
}

// IngestError indicates no items could be ingested (e.g. bad repo or rate limit).
type IngestError struct{ Repo string }

func (e *IngestError) Error() string {
	return "ingest: no items collected for " + e.Repo + " (check the repo name, network, or set GITHUB_TOKEN to avoid rate limits)"
}
