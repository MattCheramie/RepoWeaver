package server

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/mattcheramie/repoweaver/internal/analytics"
	"github.com/mattcheramie/repoweaver/internal/store"
)

// ga4TokenKey is the settings key under which the GA4 OAuth token is stored.
const ga4TokenKey = "ga4_oauth_token"

// oauthStateCookie holds the CSRF state during the consent round-trip.
const oauthStateCookie = "ga4_oauth_state"

// settingTokenStore adapts the SQLite settings table to analytics.TokenStore.
type settingTokenStore struct {
	store *store.Store
}

// LoadToken implements analytics.TokenStore.
func (s *settingTokenStore) LoadToken() (*oauth2.Token, bool) {
	raw, ok := s.store.GetSetting(ga4TokenKey)
	if !ok {
		return nil, false
	}
	return analytics.DecodeToken(raw)
}

// SaveToken implements analytics.TokenStore.
func (s *settingTokenStore) SaveToken(t *oauth2.Token) error {
	enc, err := analytics.EncodeToken(t)
	if err != nil {
		return err
	}
	return s.store.SetSetting(ga4TokenKey, enc)
}

// oauthEnabled reports whether the browser consent flow is configured.
func (s *Server) oauthEnabled() bool { return s.oauthConf != nil }

// redirectConf clones the base OAuth config and sets the redirect URL derived
// from the incoming request (so it works regardless of host/port).
func (s *Server) redirectConf(r *http.Request) *oauth2.Config {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	c := *s.oauthConf
	c.RedirectURL = scheme + "://" + r.Host + "/analytics/oauth/callback"
	return &c
}

// handleOAuthConnect starts the Google consent flow.
func (s *Server) handleOAuthConnect(w http.ResponseWriter, r *http.Request) {
	if !s.oauthEnabled() {
		http.NotFound(w, r)
		return
	}
	state, err := randomState()
	if err != nil {
		http.Error(w, "could not start OAuth", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})
	// AccessTypeOffline + prompt=consent ensures we receive a refresh token.
	url := s.redirectConf(r).AuthCodeURL(state,
		oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"))
	http.Redirect(w, r, url, http.StatusFound)
}

// handleOAuthCallback completes the consent flow: it validates state, exchanges
// the code for a token, persists it, and returns to the dashboard.
func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if !s.oauthEnabled() {
		http.NotFound(w, r)
		return
	}
	cookie, err := r.Cookie(oauthStateCookie)
	if err != nil || cookie.Value == "" || cookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid OAuth state", http.StatusBadRequest)
		return
	}
	// Clear the state cookie.
	http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Value: "", Path: "/", MaxAge: -1})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}
	tok, err := s.redirectConf(r).Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "token exchange failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	if err := s.tokenStore.SaveToken(tok); err != nil {
		http.Error(w, "could not save token: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/analytics", http.StatusFound)
}

// handleOAuthDisconnect clears the stored GA4 token.
func (s *Server) handleOAuthDisconnect(w http.ResponseWriter, r *http.Request) {
	if s.tokenStore != nil {
		_ = s.store.SetSetting(ga4TokenKey, "")
	}
	http.Redirect(w, r, "/analytics", http.StatusFound)
}

// randomState returns a URL-safe random string for CSRF protection.
func randomState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
