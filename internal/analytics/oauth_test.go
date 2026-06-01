package analytics

import (
	"context"
	"testing"

	"golang.org/x/oauth2"
)

// memTokenStore is an in-memory TokenStore for tests.
type memTokenStore struct {
	tok *oauth2.Token
}

func (m *memTokenStore) LoadToken() (*oauth2.Token, bool) {
	return m.tok, m.tok != nil
}
func (m *memTokenStore) SaveToken(t *oauth2.Token) error {
	m.tok = t
	return nil
}

func TestEncodeDecodeToken(t *testing.T) {
	orig := &oauth2.Token{AccessToken: "abc", RefreshToken: "ref"}
	enc, err := EncodeToken(orig)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, ok := DecodeToken(enc)
	if !ok || got.AccessToken != "abc" || got.RefreshToken != "ref" {
		t.Fatalf("decode mismatch: %+v ok=%v", got, ok)
	}

	// Empty / invalid inputs.
	if _, ok := DecodeToken(""); ok {
		t.Fatal("expected empty string to fail decode")
	}
	if _, ok := DecodeToken("not json"); ok {
		t.Fatal("expected invalid JSON to fail decode")
	}
	if _, ok := DecodeToken(`{"token_type":"Bearer"}`); ok {
		t.Fatal("expected token without access/refresh to fail decode")
	}
}

func TestOAuthConfig(t *testing.T) {
	c := OAuthConfig("cid", "secret", "http://localhost:8080/cb")
	if c.ClientID != "cid" || c.ClientSecret != "secret" || c.RedirectURL != "http://localhost:8080/cb" {
		t.Fatalf("unexpected config: %+v", c)
	}
	if len(c.Scopes) != 1 || c.Scopes[0] != analyticsReadonlyScope {
		t.Fatalf("expected analytics readonly scope, got %v", c.Scopes)
	}
	// AuthCodeURL should include our client and scope.
	url := c.AuthCodeURL("state123")
	for _, want := range []string{"client_id=cid", "state=state123", "analytics.readonly"} {
		if !contains(url, want) {
			t.Fatalf("auth URL missing %q: %s", want, url)
		}
	}
}

func TestGA4OAuthConfigured(t *testing.T) {
	conf := OAuthConfig("cid", "secret", "")
	store := &memTokenStore{}
	p := NewGA4OAuth("123456", conf, store)

	// Not configured until a token is stored.
	if p.Configured() {
		t.Fatal("should be unconfigured without a token")
	}
	if _, err := p.Report(context.Background(), []string{"x"}); err != ErrNotConfigured {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}

	store.tok = &oauth2.Token{AccessToken: "abc"}
	if !p.Configured() {
		t.Fatal("should be configured once a token is present")
	}

	// Missing property ID is never configured.
	p2 := NewGA4OAuth("", conf, store)
	if p2.Configured() {
		t.Fatal("should be unconfigured without a property ID")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
