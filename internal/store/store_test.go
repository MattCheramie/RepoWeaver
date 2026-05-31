package store

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestRepoRoundTrip(t *testing.T) {
	s := newTestStore(t)

	r, err := s.AddRepo("octocat", "Hello-World")
	if err != nil {
		t.Fatalf("add repo: %v", err)
	}
	if r.FullName() != "octocat/Hello-World" {
		t.Fatalf("unexpected full name: %s", r.FullName())
	}

	// Adding again should be idempotent (same id).
	r2, err := s.AddRepo("octocat", "Hello-World")
	if err != nil {
		t.Fatalf("add repo again: %v", err)
	}
	if r2.ID != r.ID {
		t.Fatalf("expected same id, got %d vs %d", r2.ID, r.ID)
	}

	got, err := s.RepoByID(r.ID)
	if err != nil {
		t.Fatalf("repo by id: %v", err)
	}
	if got.Status != "new" {
		t.Fatalf("expected status new, got %s", got.Status)
	}

	if err := s.MarkIngested(r.ID); err != nil {
		t.Fatalf("mark ingested: %v", err)
	}
	got, _ = s.RepoByID(r.ID)
	if got.Status != "ready" || got.LastIngestedAt == nil {
		t.Fatalf("expected ready+timestamp, got %s / %v", got.Status, got.LastIngestedAt)
	}
}

func TestItemsAndClusters(t *testing.T) {
	s := newTestStore(t)
	repo, _ := s.AddRepo("acme", "widget")

	id1, err := s.UpsertItem(Item{RepoID: repo.ID, Kind: KindPR, ExternalID: "1", Title: "Add feature", Body: "body"})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	id2, _ := s.UpsertItem(Item{RepoID: repo.ID, Kind: KindIssue, ExternalID: "2", Title: "Bug", Body: "broken"})

	// Upserting the same PR updates rather than duplicates.
	id1again, _ := s.UpsertItem(Item{RepoID: repo.ID, Kind: KindPR, ExternalID: "1", Title: "Add feature v2"})
	if id1again != id1 {
		t.Fatalf("expected upsert to return same id %d, got %d", id1, id1again)
	}

	counts, _ := s.CountItemsByKind(repo.ID)
	if counts[KindPR] != 1 || counts[KindIssue] != 1 {
		t.Fatalf("unexpected counts: %#v", counts)
	}

	clusters := []Cluster{{Title: "Story A", TargetFormat: FormatBlog}}
	members := [][]int64{{id1, id2}}
	if err := s.ReplaceClusters(repo.ID, clusters, members); err != nil {
		t.Fatalf("replace clusters: %v", err)
	}

	got, _ := s.ListClusters(repo.ID)
	if len(got) != 1 || got[0].ItemCount != 2 {
		t.Fatalf("unexpected clusters: %#v", got)
	}

	items, _ := s.ClusterItems(got[0].ID)
	if len(items) != 2 {
		t.Fatalf("expected 2 cluster items, got %d", len(items))
	}

	// Re-running analysis replaces clusters.
	if err := s.ReplaceClusters(repo.ID, []Cluster{{Title: "Story B", TargetFormat: FormatTutorial}}, [][]int64{{id1}}); err != nil {
		t.Fatalf("replace clusters 2: %v", err)
	}
	got, _ = s.ListClusters(repo.ID)
	if len(got) != 1 || got[0].Title != "Story B" {
		t.Fatalf("expected single replaced cluster, got %#v", got)
	}
}

func TestContentRoundTrip(t *testing.T) {
	s := newTestStore(t)
	repo, _ := s.AddRepo("acme", "widget")

	cid, err := s.CreateContent(Content{
		RepoID: repo.ID, Title: "My Post", Format: FormatBlog, Body: "# My Post\n\nhi",
	})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}

	if err := s.UpdateContentBody(cid, "# My Post\n\nupdated"); err != nil {
		t.Fatalf("update: %v", err)
	}

	c, err := s.ContentByID(cid)
	if err != nil {
		t.Fatalf("content by id: %v", err)
	}
	if c.Body != "# My Post\n\nupdated" || c.SEOMeta != "{}" {
		t.Fatalf("unexpected content: %#v", c)
	}

	all, _ := s.ListContent()
	if len(all) != 1 {
		t.Fatalf("expected 1 content row, got %d", len(all))
	}
}
