package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mattcheramie/repoweaver/internal/config"
	"github.com/mattcheramie/repoweaver/internal/store"
)

// newTestServer builds a Server backed by a temp store and the real web assets,
// using the keyless mock LLM provider. GitHub ingestion is not exercised here
// (it requires network); instead items are seeded directly.
func newTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "srv.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	// Locate the repo's web/ assets relative to this test file.
	templatesFS := os.DirFS("../../web/templates")
	staticFS := os.DirFS("../../web/static")

	srv, err := New(config.Config{LLMProvider: "mock"}, st, templatesFS, staticFS)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return srv, st
}

func TestPipelineHTTP(t *testing.T) {
	srv, st := newTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Seed a repo with items (stands in for a GitHub ingest).
	repo, _ := st.AddRepo("acme", "widget")
	for i, title := range []string{"Add caching layer", "Fix flaky test", "Write docs"} {
		st.UpsertItem(store.Item{
			RepoID: repo.ID, Kind: store.KindPR, ExternalID: itoa(i),
			Title: title, Body: "context for " + title, State: "merged",
		})
	}
	st.MarkIngested(repo.ID)

	// Home page lists the repo.
	if body := get(t, ts, "/"); !strings.Contains(body, "acme/widget") {
		t.Fatalf("home page missing repo:\n%s", body)
	}

	// Hub shows item counts.
	hub := get(t, ts, "/repos/1/hub")
	if !strings.Contains(hub, "pr: 3") {
		t.Fatalf("hub missing item counts:\n%s", hub)
	}

	// Analyze produces clusters.
	clusters := post(t, ts, "/repos/1/analyze", nil)
	if !strings.Contains(clusters, "<h3>") {
		t.Fatalf("analyze produced no clusters:\n%s", clusters)
	}

	// Generate content for the first cluster.
	cl, _ := st.ListClusters(repo.ID)
	if len(cl) == 0 {
		t.Fatal("no clusters persisted")
	}
	gen := post(t, ts, "/clusters/"+itoa64(cl[0].ID)+"/generate", nil)
	if !strings.Contains(gen, "Generated") {
		t.Fatalf("generate failed:\n%s", gen)
	}

	// Library lists the generated content.
	lib := get(t, ts, "/library")
	if !strings.Contains(lib, "Download .md") {
		t.Fatalf("library missing content:\n%s", lib)
	}

	// Download returns markdown with the attachment header.
	all, _ := st.ListContent()
	if len(all) != 1 {
		t.Fatalf("expected 1 content row, got %d", len(all))
	}
	resp, err := http.Get(ts.URL + "/content/" + itoa64(all[0].ID) + "/download")
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer resp.Body.Close()
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, ".md") {
		t.Fatalf("expected .md attachment, got %q", cd)
	}
	md, _ := io.ReadAll(resp.Body)
	if !strings.HasPrefix(string(md), "#") {
		t.Fatalf("expected markdown body, got %.40q", md)
	}

	// Save edits to the content body.
	save := post(t, ts, "/content/"+itoa64(all[0].ID), url.Values{"body": {"# Edited\n\nnew"}})
	if !strings.Contains(save, "Saved") {
		t.Fatalf("save failed:\n%s", save)
	}
	updated, _ := st.ContentByID(all[0].ID)
	if updated.Body != "# Edited\n\nnew" {
		t.Fatalf("edit not persisted: %q", updated.Body)
	}
}

func TestStubPages(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	for _, path := range []string{"/calendar", "/analytics"} {
		if body := get(t, ts, path); !strings.Contains(body, "Coming soon") {
			t.Fatalf("%s not rendered as stub:\n%s", path, body)
		}
	}
}

func TestParseRepoInput(t *testing.T) {
	cases := map[string][2]string{
		"octocat/Hello-World":              {"octocat", "Hello-World"},
		"https://github.com/golang/go":     {"golang", "go"},
		"https://github.com/golang/go.git": {"golang", "go"},
		"  acme/widget/extra ":             {"acme", "widget"},
	}
	for in, want := range cases {
		o, n, ok := parseRepoInput(in)
		if !ok || o != want[0] || n != want[1] {
			t.Fatalf("parseRepoInput(%q) = %q,%q,%v; want %q,%q", in, o, n, ok, want[0], want[1])
		}
	}
	if _, _, ok := parseRepoInput("noslash"); ok {
		t.Fatal("expected failure on input without slash")
	}
}

// helpers

func get(t *testing.T, ts *httptest.Server, path string) string {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d:\n%s", path, resp.StatusCode, b)
	}
	return string(b)
}

func post(t *testing.T, ts *httptest.Server, path string, form url.Values) string {
	t.Helper()
	resp, err := http.PostForm(ts.URL+path, form)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s: status %d:\n%s", path, resp.StatusCode, b)
	}
	return string(b)
}

func itoa(i int) string { return string(rune('0' + i)) }
func itoa64(i int64) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
