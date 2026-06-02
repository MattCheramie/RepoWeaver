package analyze

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mattcheramie/repoweaver/internal/llm"
	"github.com/mattcheramie/repoweaver/internal/store"
)

func newTestAnalyzer(t *testing.T) (*Analyzer, *store.Store, store.Repo) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	repo, _ := st.AddRepo("acme", "widget")
	for i, title := range []string{"Add caching", "Fix retry logic", "Document API"} {
		st.UpsertItem(store.Item{
			RepoID: repo.ID, Kind: store.KindPR, ExternalID: itoa(i),
			Title: title, Body: "context for " + title, State: "merged",
		})
	}
	return New(st, llm.NewMock()), st, repo
}

func TestIdentifyResearchAndGenerate(t *testing.T) {
	a, st, repo := newTestAnalyzer(t)
	ctx := context.Background()

	items, _ := st.ListItems(repo.ID, 60)
	n, err := a.IdentifyTopics(ctx, repo.ID, items)
	if err != nil {
		t.Fatalf("identify: %v", err)
	}
	if n == 0 {
		t.Fatal("expected identified topics")
	}
	topics, _ := st.ListTopics(repo.ID)
	if len(topics) == 0 || topics[0].Status != store.TopicIdentified {
		t.Fatalf("unexpected topics after identify: %#v", topics)
	}

	// Background research with the mock Researcher completes quickly.
	a.StartResearch(repo.ID)
	researched := waitFor(t, func() bool {
		ts, _ := st.ListTopics(repo.ID)
		for _, tp := range ts {
			if tp.Status != store.TopicResearched {
				return false
			}
		}
		return len(ts) > 0
	})
	if !researched {
		ts, _ := st.ListTopics(repo.ID)
		t.Fatalf("topics not researched in time: %#v", ts)
	}

	topics, _ = st.ListTopics(repo.ID)
	first := topics[0]
	if first.Research == "" || first.Sources == "[]" {
		t.Fatalf("research not stored: %#v", first)
	}

	// Generate a standalone draft from the researched topic.
	cid, err := a.GenerateFromTopic(ctx, first.ID)
	if err != nil {
		t.Fatalf("generate from topic: %v", err)
	}
	c, _ := st.ContentByID(cid)
	if c.RepoID != repo.ID || c.ClusterID != 0 || c.Body == "" {
		t.Fatalf("unexpected standalone content: %#v", c)
	}
}

func TestResearchUnsupportedProvider(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "u.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	repo, _ := st.AddRepo("acme", "widget")
	a := New(st, noResearchProvider{})

	id, _ := st.UpsertTopic(repo.ID, "Sharding", "mentioned once")
	a.StartResearch(repo.ID)
	got := waitFor(t, func() bool {
		tp, _ := st.TopicByID(id)
		return tp.Status == store.TopicUnsupported
	})
	if !got {
		tp, _ := st.TopicByID(id)
		t.Fatalf("expected unsupported, got %s", tp.Status)
	}
}

// noResearchProvider is a Provider that does NOT implement llm.Researcher.
type noResearchProvider struct{}

func (noResearchProvider) Name() string { return "noresearch" }
func (noResearchProvider) Complete(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

func waitFor(t *testing.T, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}
