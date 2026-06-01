package analytics

import (
	"context"
	"encoding/json"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// TokenStore persists the OAuth token obtained from the Google consent flow so
// it survives restarts. Implementations are provided by the server (backed by
// the SQLite settings table).
type TokenStore interface {
	// LoadToken returns the stored token, or false if none is saved.
	LoadToken() (*oauth2.Token, bool)
	// SaveToken persists a token (e.g. after a refresh).
	SaveToken(*oauth2.Token) error
}

// OAuthConfig builds the OAuth2 config for the GA4 Data API authorization-code
// flow. redirectURL is where Google sends the user back after consent.
func OAuthConfig(clientID, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{analyticsReadonlyScope},
		Endpoint:     google.Endpoint,
	}
}

// GA4OAuth is a GA4 provider authenticated via a user OAuth token (consent
// flow) rather than a service account. The token is read from a TokenStore on
// each request, so connecting/disconnecting at runtime takes effect immediately.
type GA4OAuth struct {
	propertyID string
	conf       *oauth2.Config
	tokens     TokenStore
}

// NewGA4OAuth constructs an OAuth-backed GA4 provider.
func NewGA4OAuth(propertyID string, conf *oauth2.Config, tokens TokenStore) *GA4OAuth {
	return &GA4OAuth{propertyID: propertyID, conf: conf, tokens: tokens}
}

// Name implements Provider.
func (g *GA4OAuth) Name() string { return "ga4-oauth(property " + g.propertyID + ")" }

// Configured implements Provider: ready once a property ID is set and a token
// has been obtained through the consent flow.
func (g *GA4OAuth) Configured() bool {
	if g.propertyID == "" || g.conf == nil || g.tokens == nil {
		return false
	}
	_, ok := g.tokens.LoadToken()
	return ok
}

// Report implements Provider. It builds an auto-refreshing token source from
// the stored token and persists any refreshed token back to the store.
func (g *GA4OAuth) Report(ctx context.Context, slugs []string) (map[string]Metrics, error) {
	if g.propertyID == "" || g.conf == nil || g.tokens == nil {
		return nil, ErrNotConfigured
	}
	tok, ok := g.tokens.LoadToken()
	if !ok {
		return nil, ErrNotConfigured
	}
	src := &savingTokenSource{
		base:  g.conf.TokenSource(ctx, tok),
		store: g.tokens,
		last:  tok,
	}
	return runReport(ctx, src, g.propertyID, slugs)
}

// savingTokenSource wraps an oauth2.TokenSource and persists the token whenever
// it changes (i.e. after a refresh), so the refresh token chain survives.
type savingTokenSource struct {
	base  oauth2.TokenSource
	store TokenStore
	last  *oauth2.Token
}

// Token implements oauth2.TokenSource.
func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := s.base.Token()
	if err != nil {
		return nil, err
	}
	if s.last == nil || tok.AccessToken != s.last.AccessToken {
		_ = s.store.SaveToken(tok)
		s.last = tok
	}
	return tok, nil
}

// EncodeToken / DecodeToken serialize a token for storage in the settings table.
func EncodeToken(t *oauth2.Token) (string, error) {
	b, err := json.Marshal(t)
	return string(b), err
}

// DecodeToken parses a stored token; returns false if the input is empty/invalid.
func DecodeToken(s string) (*oauth2.Token, bool) {
	if s == "" {
		return nil, false
	}
	var t oauth2.Token
	if err := json.Unmarshal([]byte(s), &t); err != nil {
		return nil, false
	}
	if t.AccessToken == "" && t.RefreshToken == "" {
		return nil, false
	}
	return &t, true
}
