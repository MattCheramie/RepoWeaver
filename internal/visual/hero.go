package visual

import (
	"fmt"
	"strings"
)

// HeroSpec describes the inputs for a post's header/hero banner.
type HeroSpec struct {
	Title  string
	Format string // store.Format* — influences accent color and glyph
	Tags   []string
	Repo   string // "owner/name", optional
}

// formatStyle maps a content format to a hero accent color, a short glyph, and a
// human label.
func formatStyle(format string) (accent, glyph, label string) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "tutorial":
		return colorGood, "{ }", "Tutorial"
	case "video_script":
		return colorWarn, "▶", "Video script"
	case "deep_dive":
		return colorViolet, "◆", "Deep dive"
	default:
		return colorAccent, "✶", "Blog"
	}
}

// Hero renders a deterministic, self-contained SVG banner for a post. The same
// spec always yields the same image (seeded by title + format), so it is stable
// across regenerations and safe to embed directly in exported Markdown.
func Hero(spec HeroSpec) string {
	const w, h = 960.0, 260.0
	seed := hashSeed(spec.Title + "|" + spec.Format)
	r := newRNG(seed)
	accent, glyph, label := formatStyle(spec.Format)

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %s %s" width="100%%" `+
		`role="img" aria-label="%s" preserveAspectRatio="xMidYMid slice" class="rw-hero-svg">`,
		num(w), num(h), esc("Header banner: "+spec.Title))

	// Definitions: a subtle diagonal background gradient tinted toward the accent.
	fmt.Fprintf(&b, `<defs>`+
		`<linearGradient id="rwbg" x1="0" y1="0" x2="1" y2="1">`+
		`<stop offset="0" stop-color="%s"/>`+
		`<stop offset="0.6" stop-color="%s"/>`+
		`<stop offset="1" stop-color="%s"/>`+
		`</linearGradient></defs>`,
		colorPanel, colorBG, mix(colorBG, accent, 0.18))

	// Background.
	fmt.Fprintf(&b, `<rect x="0" y="0" width="%s" height="%s" fill="url(#rwbg)"/>`, num(w), num(h))

	// Decorative "threads": deterministic flowing curves echoing the brand 🧵.
	threads := 5 + int(r.f()*4) // 5..8
	for i := 0; i < threads; i++ {
		y0 := r.frange(-20, h+20)
		y1 := r.frange(-20, h+20)
		cy := r.frange(0, h)
		col := accent
		if i%2 == 1 {
			col = colorAccent2
		}
		op := r.frange(0.05, 0.20)
		sw := r.frange(1.2, 3.0)
		fmt.Fprintf(&b, `<path d="M %s %s Q %s %s %s %s" fill="none" stroke="%s" stroke-width="%s" stroke-opacity="%s"/>`,
			num(-10), num(y0), num(w*0.5), num(cy), num(w+10), num(y1), col, num(sw), num(op))
	}

	// Accent edge on the left.
	fmt.Fprintf(&b, `<rect x="0" y="0" width="6" height="%s" fill="%s"/>`, num(h), accent)

	pad := 56.0

	// Format chip (glyph + label) near the top-left.
	chipY := 44.0
	fmt.Fprintf(&b, `<g font-family="ui-monospace, SFMono-Regular, Menlo, monospace">`)
	fmt.Fprintf(&b, `<text x="%s" y="%s" font-size="22" fill="%s" font-weight="700">%s</text>`,
		num(pad), num(chipY), accent, esc(glyph))
	fmt.Fprintf(&b, `<text x="%s" y="%s" font-size="15" letter-spacing="2" fill="%s">%s</text>`,
		num(pad+34), num(chipY-1), colorMuted, esc(strings.ToUpper(label)))
	b.WriteString(`</g>`)

	// Title, wrapped to at most three lines.
	lines := wrapText(spec.Title, 30, 3)
	fontSize := 44.0
	if len(lines) == 3 || longestLen(lines) > 24 {
		fontSize = 36.0
	}
	lineH := fontSize * 1.15
	startY := 118.0
	b.WriteString(`<g font-family="-apple-system, Segoe UI, Roboto, Helvetica, Arial, sans-serif" font-weight="700">`)
	for i, ln := range lines {
		fmt.Fprintf(&b, `<text x="%s" y="%s" font-size="%s" fill="%s">%s</text>`,
			num(pad), num(startY+float64(i)*lineH), num(fontSize), colorText, esc(ln))
	}
	b.WriteString(`</g>`)

	// Footer: repo (left) and tags (right).
	footY := h - 28
	if repo := strings.TrimSpace(spec.Repo); repo != "" && repo != "/" {
		fmt.Fprintf(&b, `<text x="%s" y="%s" font-family="ui-monospace, SFMono-Regular, Menlo, monospace" `+
			`font-size="14" fill="%s">%s</text>`, num(pad), num(footY), colorMuted, esc(repo))
	}
	if tagLine := joinTags(spec.Tags, 4); tagLine != "" {
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="end" `+
			`font-family="-apple-system, Segoe UI, Roboto, sans-serif" font-size="13" fill="%s">%s</text>`,
			num(w-pad), num(footY), accent, esc(tagLine))
	}

	b.WriteString(`</svg>`)
	return b.String()
}

// wrapText greedily wraps s into at most maxLines lines of roughly maxChars
// runes, ellipsizing the final line when the text overflows.
func wrapText(s string, maxChars, maxLines int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	cur := ""
	for _, word := range words {
		cand := word
		if cur != "" {
			cand = cur + " " + word
		}
		if len([]rune(cand)) > maxChars && cur != "" {
			lines = append(lines, cur)
			cur = word
			if len(lines) == maxLines {
				cur = ""
				break
			}
			continue
		}
		cur = cand
	}
	if cur != "" && len(lines) < maxLines {
		lines = append(lines, cur)
	} else if cur != "" {
		// Overflow beyond the last allowed line: fold remainder into an ellipsis.
		last := lines[len(lines)-1]
		lines[len(lines)-1] = ellipsize(last+" "+cur, maxChars)
	}
	if len(lines) == 0 {
		lines = []string{ellipsize(s, maxChars)}
	}
	return lines
}

func ellipsize(s string, maxChars int) string {
	r := []rune(s)
	if len(r) <= maxChars {
		return s
	}
	if maxChars < 1 {
		return "…"
	}
	return strings.TrimSpace(string(r[:maxChars-1])) + "…"
}

func longestLen(lines []string) int {
	m := 0
	for _, l := range lines {
		if n := len([]rune(l)); n > m {
			m = n
		}
	}
	return m
}

func joinTags(tags []string, max int) string {
	out := make([]string, 0, max)
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		out = append(out, "#"+t)
		if len(out) >= max {
			break
		}
	}
	return strings.Join(out, "  ")
}

// mix linearly blends two #rrggbb hex colors by t in [0,1].
func mix(a, b string, t float64) string {
	ar, ag, ab := hexRGB(a)
	br, bg, bb := hexRGB(b)
	r := ar + (br-ar)*t
	g := ag + (bg-ag)*t
	bl := ab + (bb-ab)*t
	return fmt.Sprintf("#%02x%02x%02x", clampByte(r), clampByte(g), clampByte(bl))
}

func hexRGB(h string) (r, g, b float64) {
	h = strings.TrimPrefix(h, "#")
	if len(h) != 6 {
		return 0, 0, 0
	}
	var ri, gi, bi int
	fmt.Sscanf(h, "%02x%02x%02x", &ri, &gi, &bi)
	return float64(ri), float64(gi), float64(bi)
}

func clampByte(v float64) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return int(v + 0.5)
}
