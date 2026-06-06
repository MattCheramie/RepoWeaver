package render

import (
	"strings"
	"testing"

	"github.com/mattcheramie/repoweaver/internal/store"
)

const sampleBody = "# Title\n\n" +
	"Intro prose.\n\n" +
	"```chart\n" +
	`{"type":"bar","title":"Counts","data":[{"label":"a","value":3},{"label":"b","value":5}]}` + "\n" +
	"```\n\n" +
	"More prose.\n\n" +
	"```mermaid\n" +
	"flowchart TD\n  A-->B\n" +
	"```\n\n" +
	"```go\nfmt.Println(\"hi\")\n```\n"

func sampleContent() store.Content {
	return store.Content{ID: 1, Title: "Title", Format: "blog", Body: sampleBody, SEOMeta: `{"tags":["go"]}`}
}

func TestHTMLExpandsVisuals(t *testing.T) {
	html := string(HTML(sampleContent(), store.Repo{Owner: "o", Name: "n"}))

	if !strings.Contains(html, "rw-hero") || !strings.Contains(html, "<svg") {
		t.Error("expected a hero SVG in the rendered HTML")
	}
	if !strings.Contains(html, "rw-chart-svg") {
		t.Error("expected the chart fence to become inline SVG")
	}
	if !strings.Contains(html, `<pre class="mermaid">`) {
		t.Error("expected the mermaid fence to become a .mermaid container")
	}
	// Ordinary code fences stay as code, not visuals.
	if strings.Contains(html, "rw-chart") && strings.Count(html, "rw-chart-svg") != 1 {
		t.Error("only the chart fence should render as a chart")
	}
	if !strings.Contains(html, "Println") {
		t.Error("ordinary code block should be preserved as code")
	}
	// Prose was rendered by goldmark.
	if !strings.Contains(html, "<p>Intro prose.</p>") {
		t.Error("expected prose to be rendered to HTML paragraphs")
	}
}

func TestHTMLBadChartDegradesGracefully(t *testing.T) {
	c := store.Content{Title: "T", Format: "blog", Body: "```chart\nnot json\n```\n"}
	html := string(HTML(c, store.Repo{}))
	if !strings.Contains(html, "rw-visual-error") {
		t.Error("invalid chart should render an inline error, not panic or vanish")
	}
}

func TestMarkdownEmbedsInlineSVGAndKeepsMermaid(t *testing.T) {
	md := Markdown(sampleContent(), store.Repo{Owner: "o", Name: "n"})

	if !strings.HasPrefix(md, "<svg") {
		t.Error("export should start with the inline hero SVG")
	}
	if !strings.Contains(md, "rw-chart-svg") {
		t.Error("chart fence should be baked to inline SVG in the export")
	}
	if strings.Contains(md, "```chart") {
		t.Error("chart fence should be replaced, not left as a code block")
	}
	if !strings.Contains(md, "```mermaid") {
		t.Error("mermaid fence should be preserved for native rendering")
	}
}

func TestMarkdownBadChartFallsBackToFence(t *testing.T) {
	c := store.Content{Title: "T", Format: "blog", Body: "```chart\nbroken\n```\n"}
	md := Markdown(c, store.Repo{})
	if !strings.Contains(md, "```chart") {
		t.Error("unrenderable chart should fall back to its original fence")
	}
}

func TestSplitBlocks(t *testing.T) {
	blocks := splitBlocks(sampleBody)
	var langs []string
	for _, b := range blocks {
		langs = append(langs, b.lang)
	}
	// prose, chart, prose(+code), mermaid  — ordinary ```go stays inside prose.
	got := strings.Join(langs, ",")
	if !strings.Contains(got, "chart") || !strings.Contains(got, "mermaid") {
		t.Fatalf("expected chart and mermaid blocks, got langs: %q", got)
	}
	// The ```go block must remain prose, never a visual lang.
	for _, b := range blocks {
		if b.lang == "go" {
			t.Error("ordinary code fence should not be split out as a visual")
		}
	}
}
