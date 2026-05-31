// Package ingest pulls a repository's dynamic history (via the GitHub API) and
// static files, caching everything as items in the store.
package ingest

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/go-github/v66/github"
	"github.com/mattcheramie/repoweaver/internal/store"
	"golang.org/x/oauth2"
)

// maxPages caps pagination per resource to keep MVP ingests bounded.
const maxPages = 5

// perPage is the GitHub API page size.
const perPage = 50

// newGitHubClient returns an authenticated client if a token is provided,
// otherwise an unauthenticated client (subject to low rate limits).
func newGitHubClient(ctx context.Context, token string) *github.Client {
	if token == "" {
		return github.NewClient(nil)
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	return github.NewClient(oauth2.NewClient(ctx, ts))
}

// fetchHistory pulls issues+PRs (with comments) and commits, returning items.
// Total capture: includes bots, dependency bumps, and unresolved issues.
func fetchHistory(ctx context.Context, gh *github.Client, repoID int64, owner, name string) ([]store.Item, error) {
	var items []store.Item

	// Issues endpoint returns both issues and PRs.
	issueOpts := &github.IssueListByRepoOptions{
		State:       "all",
		ListOptions: github.ListOptions{PerPage: perPage},
	}
	for page := 0; page < maxPages; page++ {
		issues, resp, err := gh.Issues.ListByRepo(ctx, owner, name, issueOpts)
		if err != nil {
			return nil, fmt.Errorf("list issues: %w", err)
		}
		for _, is := range issues {
			kind := store.KindIssue
			state := is.GetState()
			if is.IsPullRequest() {
				kind = store.KindPR
				if is.GetPullRequestLinks().GetMergedAt().Time.IsZero() {
					if state == "closed" {
						state = "closed"
					}
				} else {
					state = "merged"
				}
			}
			items = append(items, store.Item{
				RepoID:     repoID,
				Kind:       kind,
				ExternalID: strconv.Itoa(is.GetNumber()),
				Title:      is.GetTitle(),
				Body:       is.GetBody(),
				State:      state,
				Author:     is.GetUser().GetLogin(),
				URL:        is.GetHTMLURL(),
				CreatedAt:  is.GetCreatedAt().Time,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		issueOpts.Page = resp.NextPage
	}

	// Comments across all issues/PRs.
	commentOpts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: perPage},
	}
	for page := 0; page < maxPages; page++ {
		comments, resp, err := gh.Issues.ListComments(ctx, owner, name, 0, commentOpts)
		if err != nil {
			// Comments are best-effort; don't fail the whole ingest.
			break
		}
		for _, cm := range comments {
			items = append(items, store.Item{
				RepoID:     repoID,
				Kind:       store.KindComment,
				ExternalID: strconv.FormatInt(cm.GetID(), 10),
				Body:       cm.GetBody(),
				Author:     cm.GetUser().GetLogin(),
				URL:        cm.GetHTMLURL(),
				CreatedAt:  cm.GetCreatedAt().Time,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		commentOpts.Page = resp.NextPage
	}

	// Commit messages.
	commitOpts := &github.CommitsListOptions{
		ListOptions: github.ListOptions{PerPage: perPage},
	}
	for page := 0; page < maxPages; page++ {
		commits, resp, err := gh.Repositories.ListCommits(ctx, owner, name, commitOpts)
		if err != nil {
			break
		}
		for _, c := range commits {
			items = append(items, store.Item{
				RepoID:     repoID,
				Kind:       store.KindCommit,
				ExternalID: c.GetSHA(),
				Title:      firstLine(c.GetCommit().GetMessage()),
				Body:       c.GetCommit().GetMessage(),
				Author:     c.GetCommit().GetAuthor().GetName(),
				URL:        c.GetHTMLURL(),
				CreatedAt:  c.GetCommit().GetAuthor().GetDate().Time,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		commitOpts.Page = resp.NextPage
	}

	return items, nil
}

func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}
