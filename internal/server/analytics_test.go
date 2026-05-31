package server

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mattcheramie/repoweaver/internal/config"
	"github.com/mattcheramie/repoweaver/internal/store"
)

// newServerWithConfig builds a Server with a custom config (e.g. to select an
// analytics provider), backed by a temp store and the real web assets.
func newServerWithConfig(t *testing.T, cfg config.Config) (*Server, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "srv.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	srv, err := New(cfg, st, os.DirFS("../../web/templates"), os.DirFS("../../web/static"))
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return srv, st
}

func TestAnalyticsUnconfigured(t *testing.T) {
	srv, _ := newServerWithConfig(t, config.Config{LLMProvider: "mock"})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	page := get(t, ts, "/analytics")
	if !strings.Contains(page, "Connect Google Analytics") {
		t.Fatalf("expected setup prompt, got:\n%s", page)
	}
}

func TestAnalyticsDemoDashboard(t *testing.T) {
	srv, st := newServerWithConfig(t, config.Config{LLMProvider: "mock", AnalyticsProvider: "demo"})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// A scheduled post is tracked; an unscheduled draft is not.
	repo, _ := st.AddRepo("acme", "widget")
	id, _ := st.CreateContent(store.Content{
		RepoID: repo.ID, Title: "Caching Layer Guide", Format: store.FormatBlog,
		Body: "# Caching Layer Guide\n\nbody", SEOMeta: `{"slug":"caching-layer-guide"}`,
	})
	when := time.Now().UTC().AddDate(0, 0, -2) // published (in the past)
	if err := st.SetSchedule(id, &when, time.Now().UTC()); err != nil {
		t.Fatalf("schedule: %v", err)
	}
	st.CreateContent(store.Content{RepoID: repo.ID, Title: "Unscheduled Draft", Format: store.FormatBlog, Body: "# x"})

	page := get(t, ts, "/analytics")
	if !strings.Contains(page, "Caching Layer Guide") {
		t.Fatalf("dashboard missing tracked post:\n%s", page)
	}
	if strings.Contains(page, "Unscheduled Draft") {
		t.Fatalf("unscheduled draft should not be tracked:\n%s", page)
	}
	if !strings.Contains(page, "Pageviews") {
		t.Fatalf("dashboard missing metrics table:\n%s", page)
	}
}
