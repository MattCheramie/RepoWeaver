// Package server wires HTTP routes to the store, ingester, and analyzer,
// rendering HTMX-driven HTML.
package server

import (
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"golang.org/x/oauth2"

	"github.com/mattcheramie/repoweaver/internal/analytics"
	"github.com/mattcheramie/repoweaver/internal/analyze"
	"github.com/mattcheramie/repoweaver/internal/config"
	"github.com/mattcheramie/repoweaver/internal/ingest"
	"github.com/mattcheramie/repoweaver/internal/llm"
	"github.com/mattcheramie/repoweaver/internal/store"
)

// pageTemplates lists the page templates; each defines a "content" block and
// is parsed together with layout.html into its own template set so the
// "content" definitions don't collide.
var pageTemplates = []string{
	"repos.html", "hub.html", "library.html", "content.html", "calendar.html",
	"analytics.html",
}

// Server holds dependencies shared by handlers.
type Server struct {
	cfg        config.Config
	store      *store.Store
	ingester   *ingest.Ingester
	analyzer   *analyze.Analyzer
	provider   llm.Provider
	analytics  analytics.Provider
	oauthConf  *oauth2.Config       // nil unless GA4 OAuth is configured
	tokenStore analytics.TokenStore // backs the GA4 OAuth provider
	pages      map[string]*template.Template
	staticFS   fs.FS
}

// New builds a Server. templatesFS must contain the *.html templates and
// staticFS must contain web/static/* (typically supplied via embed).
func New(cfg config.Config, st *store.Store, templatesFS, staticFS fs.FS) (*Server, error) {
	funcs := template.FuncMap{
		// pct converts a 0..1 rate into a 0..100 percentage.
		"pct": func(rate float64) float64 { return rate * 100 },
	}
	pages := make(map[string]*template.Template, len(pageTemplates))
	for _, page := range pageTemplates {
		t, err := template.New("layout.html").Funcs(funcs).ParseFS(templatesFS, "layout.html", page)
		if err != nil {
			return nil, err
		}
		pages[page] = t
	}
	provider := llm.New(cfg)

	// Analytics: prefer the GA4 OAuth (browser consent) flow when an OAuth
	// client is configured and no service-account credentials are supplied.
	analyticsProvider := analytics.New(cfg)
	var oauthConf *oauth2.Config
	var tokenStore analytics.TokenStore
	if strings.EqualFold(cfg.AnalyticsProvider, "ga4") && cfg.GA4OAuthClientID != "" &&
		cfg.GA4CredentialsJSON == "" && cfg.GA4CredentialsFile == "" {
		oauthConf = analytics.OAuthConfig(cfg.GA4OAuthClientID, cfg.GA4OAuthClientSecret, "")
		tokenStore = &settingTokenStore{store: st}
		analyticsProvider = analytics.NewGA4OAuth(cfg.GA4PropertyID, oauthConf, tokenStore)
	}

	return &Server{
		cfg:        cfg,
		store:      st,
		ingester:   ingest.New(st, cfg.GitHubToken),
		analyzer:   analyze.New(st, provider),
		provider:   provider,
		analytics:  analyticsProvider,
		oauthConf:  oauthConf,
		tokenStore: tokenStore,
		pages:      pages,
		staticFS:   staticFS,
	}, nil
}

// Handler returns the configured HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", http.StripPrefix("/static/",
		http.FileServer(http.FS(s.staticFS))))

	mux.HandleFunc("GET /", s.handleRepos)
	mux.HandleFunc("POST /repos", s.handleAddRepo)
	mux.HandleFunc("GET /repos/{id}/hub", s.handleHub)
	mux.HandleFunc("POST /repos/{id}/analyze", s.handleAnalyze)
	mux.HandleFunc("POST /clusters/{id}/generate", s.handleGenerate)

	mux.HandleFunc("GET /library", s.handleLibrary)
	mux.HandleFunc("GET /content/{id}", s.handleContent)
	mux.HandleFunc("POST /content/{id}", s.handleSaveContent)
	mux.HandleFunc("POST /content/{id}/seo", s.handleRegenerateSEO)
	mux.HandleFunc("POST /content/{id}/schedule", s.handleSchedule)
	mux.HandleFunc("GET /content/{id}/download", s.handleDownload)

	mux.HandleFunc("GET /calendar", s.handleCalendar)
	mux.HandleFunc("GET /analytics", s.handleAnalytics)
	mux.HandleFunc("GET /analytics/connect", s.handleOAuthConnect)
	mux.HandleFunc("GET /analytics/oauth/callback", s.handleOAuthCallback)
	mux.HandleFunc("POST /analytics/disconnect", s.handleOAuthDisconnect)

	return mux
}
