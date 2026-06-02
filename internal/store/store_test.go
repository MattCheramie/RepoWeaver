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

func TestTopicsRoundTrip(t *testing.T) {
	s := newTestStore(t)
	repo, _ := s.AddRepo("acme", "widget")

	id, err := s.UpsertTopic(repo.ID, "Rate limiting", "Mentioned but not explained")
	if err != nil {
		t.Fatalf("upsert topic: %v", err)
	}

	// Upsert with the same name is idempotent and only refreshes rationale,
	// preserving status/research.
	if err := s.SaveTopicResearch(id, "A briefing.", `[{"title":"T","url":"https://e.com"}]`); err != nil {
		t.Fatalf("save research: %v", err)
	}
	idAgain, _ := s.UpsertTopic(repo.ID, "Rate limiting", "Updated rationale")
	if idAgain != id {
		t.Fatalf("expected same id %d, got %d", id, idAgain)
	}
	got, _ := s.TopicByID(id)
	if got.Status != TopicResearched {
		t.Fatalf("re-upsert clobbered status: %s", got.Status)
	}
	if got.Rationale != "Updated rationale" {
		t.Fatalf("rationale not refreshed: %q", got.Rationale)
	}
	if got.Research != "A briefing." || got.Sources == "[]" || got.ResearchedAt == nil {
		t.Fatalf("unexpected research state: %#v", got)
	}

	// Error + status transitions.
	id2, _ := s.UpsertTopic(repo.ID, "Backpressure", "")
	if err := s.SetTopicError(id2, "boom"); err != nil {
		t.Fatalf("set error: %v", err)
	}
	got2, _ := s.TopicByID(id2)
	if got2.Status != TopicError || got2.Error != "boom" {
		t.Fatalf("unexpected error state: %#v", got2)
	}
	if err := s.SetTopicStatus(id2, TopicResearching); err != nil {
		t.Fatalf("set status: %v", err)
	}

	// ResetStuckResearch reverts in-flight rows.
	if err := s.ResetStuckResearch(); err != nil {
		t.Fatalf("reset: %v", err)
	}
	got2, _ = s.TopicByID(id2)
	if got2.Status != TopicIdentified {
		t.Fatalf("expected reset to identified, got %s", got2.Status)
	}

	list, _ := s.ListTopics(repo.ID)
	if len(list) != 2 {
		t.Fatalf("expected 2 topics, got %d", len(list))
	}

	// Cascade delete with the repo.
	if _, err := s.db.Exec(`DELETE FROM repos WHERE id=?`, repo.ID); err != nil {
		t.Fatalf("delete repo: %v", err)
	}
	if list, _ := s.ListTopics(repo.ID); len(list) != 0 {
		t.Fatalf("expected topics cascade-deleted, got %d", len(list))
	}
}

func TestSettings(t *testing.T) {
	s := newTestStore(t)

	if _, ok := s.GetSetting("missing"); ok {
		t.Fatal("expected missing key to return false")
	}
	if err := s.SetSetting("k", "v1"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if v, ok := s.GetSetting("k"); !ok || v != "v1" {
		t.Fatalf("get: %q %v", v, ok)
	}
	// Upsert overwrites.
	if err := s.SetSetting("k", "v2"); err != nil {
		t.Fatalf("set 2: %v", err)
	}
	if v, _ := s.GetSetting("k"); v != "v2" {
		t.Fatalf("expected v2, got %q", v)
	}
	// Empty value deletes.
	if err := s.SetSetting("k", ""); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := s.GetSetting("k"); ok {
		t.Fatal("expected key deleted")
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
