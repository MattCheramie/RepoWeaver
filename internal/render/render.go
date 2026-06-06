// Package render turns a generated post body into displayable output. It expands
// the visual fences authors embed (```chart, ```mermaid, ```d3) and prepends a
// deterministic hero banner, producing either safe HTML for the in-app preview
// or portable Markdown for export. Charts become inline SVG in both paths so the
// published artifact needs no image hosting; mermaid diagrams render via the
// vendored library in preview and as native ```mermaid fences in exports (which
// GitHub and many static-site generators render themselves).
package render

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/yuin/goldmark"

	"github.com/mattcheramie/repoweaver/internal/seo"
	"github.com/mattcheramie/repoweaver/internal/store"
	"github.com/mattcheramie/repoweaver/internal/visual"
)

// mdParser renders ordinary prose. Raw HTML in the source is escaped (the
// default), which is appropriate for LLM-generated Markdown.
var mdParser = goldmark.New()

// block is one parsed segment of a post body: either prose (lang == "") or a
// visual fence whose info string is chart/mermaid/d3.
type block struct {
	lang string
	text string
}

// visualLangs are the fence info strings render treats specially.
var visualLangs = map[string]bool{"chart": true, "mermaid": true, "d3": true}

// splitBlocks separates visual fences from surrounding prose. Ordinary fenced
// code blocks (```go, ```bash, plain ```) are left untouched inside the prose so
// goldmark renders them as normal code.
func splitBlocks(md string) []block {
	lines := strings.Split(md, "\n")
	var blocks []block
	var prose []string
	flush := func() {
		if len(prose) > 0 {
			blocks = append(blocks, block{text: strings.Join(prose, "\n")})
			prose = nil
		}
	}
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "```") {
			lang := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "```")))
			if visualLangs[lang] {
				var inner []string
				i++
				for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
					inner = append(inner, lines[i])
					i++
				}
				flush()
				blocks = append(blocks, block{lang: lang, text: strings.Join(inner, "\n")})
				continue // i now points at the closing fence (or EOF); loop advances it
			}
			// Ordinary code block: copy verbatim, fences included.
			prose = append(prose, lines[i])
			i++
			for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
				prose = append(prose, lines[i])
				i++
			}
			if i < len(lines) {
				prose = append(prose, lines[i])
			}
			continue
		}
		prose = append(prose, lines[i])
	}
	flush()
	return blocks
}

// HeroSpecFor builds a hero spec from a content row and its repo.
func HeroSpecFor(c store.Content, repo store.Repo) visual.HeroSpec {
	var repoName string
	if repo.Owner != "" && repo.Name != "" {
		repoName = repo.FullName()
	}
	return visual.HeroSpec{
		Title:  c.Title,
		Format: c.Format,
		Tags:   seo.Parse(c.SEOMeta).Tags,
		Repo:   repoName,
	}
}

// HTML renders a post to safe HTML for the in-app preview: a hero banner, then
// prose, with chart fences as inline SVG, mermaid fences as mermaid containers
// (the vendored library renders them client-side), and d3 fences as data
// containers a small vendored script can pick up.
func HTML(c store.Content, repo store.Repo) template.HTML {
	var b strings.Builder
	b.WriteString(`<div class="rw-hero">`)
	b.WriteString(visual.Hero(HeroSpecFor(c, repo)))
	b.WriteString("</div>\n")

	for _, blk := range splitBlocks(c.Body) {
		switch blk.lang {
		case "":
			b.WriteString(markdownToHTML(blk.text))
		case "chart":
			b.WriteString(chartHTML(blk.text))
		case "mermaid":
			// mermaid reads textContent, so escaping keeps the browser from
			// parsing diagram syntax as markup while preserving the source.
			b.WriteString(`<pre class="mermaid">` + template.HTMLEscapeString(blk.text) + "</pre>\n")
		case "d3":
			b.WriteString(`<div class="rw-d3" data-spec="` + template.HTMLEscapeString(blk.text) + `"></div>` + "\n")
		}
	}
	return template.HTML(b.String())
}

// Markdown returns the post body rewritten for portable publishing: the hero is
// embedded as inline SVG at the top and ```chart fences become inline SVG, so
// the artifact renders anywhere with no image hosting. ```mermaid (and ```d3)
// fences are preserved as-is.
func Markdown(c store.Content, repo store.Repo) string {
	var b strings.Builder
	b.WriteString(visual.Hero(HeroSpecFor(c, repo)))
	b.WriteString("\n\n")

	for _, blk := range splitBlocks(c.Body) {
		switch blk.lang {
		case "":
			b.WriteString(blk.text)
			b.WriteString("\n")
		case "chart":
			b.WriteString(chartMarkdown(blk.text))
			b.WriteString("\n")
		case "mermaid", "d3":
			b.WriteString("```" + blk.lang + "\n" + blk.text + "\n```\n")
		}
	}
	return b.String()
}

func markdownToHTML(src string) string {
	var buf bytes.Buffer
	if err := mdParser.Convert([]byte(src), &buf); err != nil {
		return template.HTMLEscapeString(src)
	}
	return buf.String()
}

// chartHTML renders a chart fence to an inline-SVG figure, degrading to a small
// inline note (never a hard failure) when the spec is unusable.
func chartHTML(spec string) string {
	cs, err := visual.ParseChart(spec)
	if err != nil {
		return visualError(err.Error())
	}
	svg, err := visual.Chart(cs)
	if err != nil {
		return visualError(err.Error())
	}
	return `<figure class="rw-chart">` + svg + "</figure>\n"
}

// chartMarkdown renders a chart fence to inline SVG for export, falling back to
// the original fenced block when the spec can't be rendered.
func chartMarkdown(spec string) string {
	cs, err := visual.ParseChart(spec)
	if err == nil {
		if svg, e := visual.Chart(cs); e == nil {
			return svg + "\n"
		}
	}
	return "```chart\n" + spec + "\n```\n"
}

func visualError(msg string) string {
	return `<div class="rw-visual-error">⚠ Could not render visual: ` + template.HTMLEscapeString(msg) + `</div>` + "\n"
}
