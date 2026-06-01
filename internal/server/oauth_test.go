package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattcheramie/repoweaver/internal/config"
)

func oauthConfig() config.Config {
	return config.Config{
		LLMProvider:          "mock",
		AnalyticsProvider:    "ga4",
		GA4PropertyID:        "123456",
		GA4OAuthClientID:     "test-client",
		GA4OAuthClientSecret: "test-secret",
	}
}

// noRedirectClient prevents the test client from following redirects so we can
// assert on Location headers.
func noRedirectClient() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

func TestAnalyticsShowsConnectButton(t *testing.T) {
	srv, _ := newServerWithConfig(t, oauthConfig())
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	page := get(t, ts, "/analytics")
	if !strings.Contains(page, "/analytics/connect") {
		t.Fatalf("expected Connect button when OAuth configured:\n%s", page)
	}
}

func TestOAuthConnectRedirectsToGoogle(t *testing.T) {
	srv, _ := newServerWithConfig(t, oauthConfig())
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := noRedirectClient().Get(ts.URL + "/analytics/connect")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	for _, want := range []string{"accounts.google.com", "client_id=test-client", "analytics.readonly", "state="} {
		if !strings.Contains(loc, want) {
			t.Fatalf("auth redirect missing %q: %s", want, loc)
		}
	}
	// A CSRF state cookie must be set.
	var hasState bool
	for _, c := range resp.Cookies() {
		if c.Name == oauthStateCookie && c.Value != "" {
			hasState = true
		}
	}
	if !hasState {
		t.Fatal("expected oauth state cookie to be set")
	}
}

func TestOAuthCallbackRejectsBadState(t *testing.T) {
	srv, _ := newServerWithConfig(t, oauthConfig())
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// No state cookie + a state param => must be rejected.
	resp, err := noRedirectClient().Get(ts.URL + "/analytics/oauth/callback?state=forged&code=xyz")
	if err != nil {
		t.Fatalf("callback: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad state, got %d", resp.StatusCode)
	}
}

func TestOAuthDisconnectClearsToken(t *testing.T) {
	srv, st := newServerWithConfig(t, oauthConfig())
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Seed a stored token, then disconnect.
	if err := st.SetSetting(ga4TokenKey, `{"access_token":"abc"}`); err != nil {
		t.Fatalf("seed token: %v", err)
	}
	if !srv.analytics.Configured() {
		t.Fatal("provider should be configured with a stored token")
	}

	resp, err := noRedirectClient().PostForm(ts.URL+"/analytics/disconnect", nil)
	if err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected redirect, got %d", resp.StatusCode)
	}
	if _, ok := st.GetSetting(ga4TokenKey); ok {
		t.Fatal("token should have been cleared")
	}
	if srv.analytics.Configured() {
		t.Fatal("provider should be unconfigured after disconnect")
	}
}

func TestOAuthRoutesDisabledWithoutClient(t *testing.T) {
	// Without an OAuth client, /analytics/connect should 404.
	srv, _ := newServerWithConfig(t, config.Config{LLMProvider: "mock", AnalyticsProvider: "demo"})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := noRedirectClient().Get(ts.URL + "/analytics/connect")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when OAuth not configured, got %d", resp.StatusCode)
	}
}
