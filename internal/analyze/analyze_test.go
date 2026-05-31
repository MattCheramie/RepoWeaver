package analyze

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattcheramie/repoweaver/internal/llm"
	"github.com/mattcheramie/repoweaver/internal/store"
)

// setup creates a store seeded with a repo and a few items, plus a mock-backed
// analyzer.
func setup(t *testing.T) (*store.Store, *Analyzer, store.Repo) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	repo, _ := s.AddRepo("acme", "widget")
	for i, title := range []string{"Add caching", "Fix race", "Document API", "Refactor store"} {
		if _, err := s.UpsertItem(store.Item{
			RepoID: repo.ID, Kind: store.KindPR, ExternalID: itoa(i),
			Title: title, Body: "details about " + title, State: "merged",
		}); err != nil {
			t.Fatalf("seed item: %v", err)
		}
	}
	return s, New(s, llm.NewMock()), repo
}

func itoa(i int) string { return string(rune('0' + i)) }

func TestAnalyzeRunProducesClusters(t *testing.T) {
	s, a, repo := setup(t)

	n, err := a.Run(context.Background(), repo.ID)
	if err != nil {
		t.Fatalf("analyze run: %v", err)
	}
	if n == 0 {
		t.Fatal("expected at least one cluster")
	}

	clusters, _ := s.ListClusters(repo.ID)
	if len(clusters) != n {
		t.Fatalf("expected %d persisted clusters, got %d", n, len(clusters))
	}
	// At least one cluster should reference some items.
	total := 0
	for _, c := range clusters {
		total += c.ItemCount
	}
	if total == 0 {
		t.Fatal("expected clusters to reference items")
	}
}

func TestGenerateCreatesContent(t *testing.T) {
	s, a, repo := setup(t)
	if _, err := a.Run(context.Background(), repo.ID); err != nil {
		t.Fatalf("analyze: %v", err)
	}
	clusters, _ := s.ListClusters(repo.ID)

	contentID, err := a.Generate(context.Background(), clusters[0].ID)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	c, err := s.ContentByID(contentID)
	if err != nil {
		t.Fatalf("content by id: %v", err)
	}
	if !strings.HasPrefix(c.Body, "#") {
		t.Fatalf("expected markdown body starting with heading, got %.40q", c.Body)
	}
	if c.Title == "" {
		t.Fatal("expected a derived title")
	}
}

func TestAnalyzeNoItems(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	repo, _ := s.AddRepo("empty", "repo")

	a := New(s, llm.NewMock())
	if _, err := a.Run(context.Background(), repo.ID); err == nil {
		t.Fatal("expected error analyzing repo with no items")
	}
}
