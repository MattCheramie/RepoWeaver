package seo

import (
	"context"
	"strings"
	"testing"
)

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Hello, World!":            "hello-world",
		"  Go & SQLite: A Guide  ": "go-sqlite-a-guide",
		"Multiple   spaces":        "multiple-spaces",
		"---trim---":               "trim",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestKeywordDensity(t *testing.T) {
	text := "# Caching\n\nThe caching layer caches results. Caching is fast.\n\n" +
		"```go\nfunc cache() {}\n```\n\nThe cache stores values."
	stats := KeywordDensity(text, 5)
	if len(stats) == 0 {
		t.Fatal("expected keywords")
	}
	// "caching" appears 3x and should rank at or near the top; stopwords excluded.
	if stats[0].Word != "caching" {
		t.Fatalf("expected 'caching' first, got %q (%+v)", stats[0].Word, stats)
	}
	if stats[0].Count != 3 {
		t.Fatalf("expected caching count 3, got %d", stats[0].Count)
	}
	for _, s := range stats {
		if stopwords[s.Word] {
			t.Fatalf("stopword leaked into results: %q", s.Word)
		}
		if s.Word == "func" || s.Word == "cache" {
			// code-fence content should be stripped
			if s.Word == "func" {
				t.Fatalf("code-block token %q leaked into keywords", s.Word)
			}
		}
	}
}

// fakeCompleter returns canned SEO JSON.
type fakeCompleter struct{ out string }

func (f fakeCompleter) Complete(_ context.Context, _, _ string) (string, error) {
	return f.out, nil
}

func TestGenerateWithProvider(t *testing.T) {
	body := "# Title\n\nThis article explains the caching layer and its tradeoffs in detail."
	c := fakeCompleter{out: `{"meta_description":"How the caching layer works.","tags":["caching","performance","go"]}`}

	m := Generate(context.Background(), c, "The Caching Layer", body)
	if m.Slug != "the-caching-layer" {
		t.Fatalf("slug = %q", m.Slug)
	}
	if m.MetaDescription != "How the caching layer works." {
		t.Fatalf("meta = %q", m.MetaDescription)
	}
	if len(m.Tags) != 3 || m.Tags[0] != "caching" {
		t.Fatalf("tags = %v", m.Tags)
	}
	if len(m.Keywords) == 0 {
		t.Fatal("expected keywords from density")
	}
}

func TestGenerateFallsBackWithoutProvider(t *testing.T) {
	body := "# Title\n\nThe quick brown fox jumps over the lazy dog repeatedly."
	m := Generate(context.Background(), nil, "My Post", body)
	if m.MetaDescription == "" {
		t.Fatal("expected heuristic meta description")
	}
	if len(m.Tags) == 0 {
		t.Fatal("expected heuristic tags from keywords")
	}
}

func TestFrontmatterRoundTrip(t *testing.T) {
	m := Meta{
		MetaDescription: "A guide.",
		Keywords:        []string{"go", "sqlite"},
		Slug:            "a-guide",
		Tags:            []string{"go", "database"},
	}
	fm := m.Frontmatter("A Guide")
	for _, want := range []string{"---", `title: "A Guide"`, `description: "A guide."`, `slug: "a-guide"`, `"go"`, `"database"`} {
		if !strings.Contains(fm, want) {
			t.Fatalf("frontmatter missing %q:\n%s", want, fm)
		}
	}

	// Parse/JSON round trip.
	parsed := Parse(m.JSON())
	if parsed.Slug != m.Slug || len(parsed.Tags) != 2 {
		t.Fatalf("round trip mismatch: %+v", parsed)
	}
}

func TestParseTolerant(t *testing.T) {
	for _, in := range []string{"", "{}", "not json"} {
		m := Parse(in)
		if m.Keywords == nil || m.Tags == nil {
			t.Fatalf("Parse(%q) returned nil slices", in)
		}
	}
}
