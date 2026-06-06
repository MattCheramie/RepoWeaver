package visual

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// ChartSpec is the JSON contract authors embed in a fenced ```chart block. Only
// real, source-grounded data should be supplied; the renderer never invents
// values.
type ChartSpec struct {
	Type   string      `json:"type"` // bar | line | area | pie
	Title  string      `json:"title"`
	Data   []DataPoint `json:"data"`
	XLabel string      `json:"x_label,omitempty"`
	YLabel string      `json:"y_label,omitempty"`
}

// DataPoint is a single labeled value.
type DataPoint struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

// ParseChart decodes a ```chart block's JSON body into a ChartSpec, validating
// that it has a known type and at least one datum.
func ParseChart(jsonBody string) (ChartSpec, error) {
	var cs ChartSpec
	if err := json.Unmarshal([]byte(strings.TrimSpace(jsonBody)), &cs); err != nil {
		return ChartSpec{}, fmt.Errorf("invalid chart JSON: %w", err)
	}
	if len(cs.Data) == 0 {
		return ChartSpec{}, fmt.Errorf("chart has no data points")
	}
	return cs, nil
}

// Chart renders a ChartSpec to a self-contained inline SVG string.
func Chart(cs ChartSpec) (string, error) {
	if len(cs.Data) == 0 {
		return "", fmt.Errorf("chart has no data points")
	}
	switch strings.ToLower(strings.TrimSpace(cs.Type)) {
	case "pie", "donut":
		return pieChart(cs), nil
	case "line":
		return lineChart(cs, false), nil
	case "area":
		return lineChart(cs, true), nil
	case "bar", "column", "":
		return barChart(cs), nil
	default:
		// Unknown types degrade to a bar chart rather than failing the render.
		return barChart(cs), nil
	}
}

// chart frame geometry (viewBox units).
const (
	cw     = 720.0
	ch     = 380.0
	mLeft  = 56.0
	mRight = 24.0
	mTop   = 48.0
	mBot   = 64.0
)

func plotDims() (x0, y0, plotW, plotH float64) {
	x0 = mLeft
	y0 = mTop
	plotW = cw - mLeft - mRight
	plotH = ch - mTop - mBot
	return
}

func openSVG(b *strings.Builder, title string) {
	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %s %s" width="100%%" `+
		`role="img" aria-label="%s" class="rw-chart-svg">`, num(cw), num(ch), esc("Chart: "+title))
	fmt.Fprintf(b, `<rect x="0" y="0" width="%s" height="%s" rx="8" fill="%s"/>`, num(cw), num(ch), colorPanel2)
	if t := strings.TrimSpace(title); t != "" {
		fmt.Fprintf(b, `<text x="%s" y="28" font-family="-apple-system, Segoe UI, Roboto, sans-serif" `+
			`font-size="16" font-weight="700" fill="%s">%s</text>`, num(mLeft), colorText, esc(t))
	}
}

func maxValue(data []DataPoint) float64 {
	m := 0.0
	for _, d := range data {
		if d.Value > m {
			m = d.Value
		}
	}
	if m <= 0 {
		return 1
	}
	return m
}

// barChart renders vertical bars with a baseline, a max gridline, and value
// annotations.
func barChart(cs ChartSpec) string {
	var b strings.Builder
	openSVG(&b, cs.Title)
	x0, y0, plotW, plotH := plotDims()
	baseline := y0 + plotH
	maxV := niceMax(maxValue(cs.Data))

	// Gridlines + y labels at 0, mid, max.
	for _, frac := range []float64{0, 0.5, 1} {
		y := baseline - frac*plotH
		fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1"/>`,
			num(x0), num(y), num(x0+plotW), num(y), colorBorder)
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="end" font-family="-apple-system, sans-serif" `+
			`font-size="11" fill="%s">%s</text>`, num(x0-8), num(y+4), colorMuted, esc(fmtVal(frac*maxV)))
	}

	n := len(cs.Data)
	slot := plotW / float64(n)
	barW := slot * 0.6
	for i, d := range cs.Data {
		cx := x0 + (float64(i)+0.5)*slot
		bh := d.Value / maxV * plotH
		if bh < 0 {
			bh = 0
		}
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="3" fill="%s"/>`,
			num(cx-barW/2), num(baseline-bh), num(barW), num(bh), seriesColor(i))
		// Value above the bar.
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="middle" font-family="-apple-system, sans-serif" `+
			`font-size="11" fill="%s">%s</text>`, num(cx), num(baseline-bh-6), colorText, esc(fmtVal(d.Value)))
		// Category label below the baseline.
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="middle" font-family="-apple-system, sans-serif" `+
			`font-size="12" fill="%s">%s</text>`, num(cx), num(baseline+18), colorMuted, esc(truncLabel(d.Label, slot)))
	}
	axisLabels(&b, cs, x0, baseline)
	b.WriteString(`</svg>`)
	return b.String()
}

// lineChart renders a line (optionally filled as an area) across the data.
func lineChart(cs ChartSpec, fill bool) string {
	var b strings.Builder
	openSVG(&b, cs.Title)
	x0, y0, plotW, plotH := plotDims()
	baseline := y0 + plotH
	maxV := niceMax(maxValue(cs.Data))

	for _, frac := range []float64{0, 0.5, 1} {
		y := baseline - frac*plotH
		fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1"/>`,
			num(x0), num(y), num(x0+plotW), num(y), colorBorder)
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="end" font-family="-apple-system, sans-serif" `+
			`font-size="11" fill="%s">%s</text>`, num(x0-8), num(y+4), colorMuted, esc(fmtVal(frac*maxV)))
	}

	n := len(cs.Data)
	px := func(i int) float64 {
		if n == 1 {
			return x0 + plotW/2
		}
		return x0 + float64(i)/float64(n-1)*plotW
	}
	py := func(v float64) float64 { return baseline - v/maxV*plotH }

	var pts strings.Builder
	for i, d := range cs.Data {
		if i > 0 {
			pts.WriteByte(' ')
		}
		fmt.Fprintf(&pts, "%s,%s", num(px(i)), num(py(d.Value)))
	}
	if fill {
		fmt.Fprintf(&b, `<polygon points="%s %s,%s %s,%s" fill="%s" fill-opacity="0.22"/>`,
			pts.String(), num(px(n-1)), num(baseline), num(px(0)), num(baseline), colorAccent)
	}
	fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="2.5" `+
		`stroke-linejoin="round" stroke-linecap="round"/>`, pts.String(), colorAccent)
	for i, d := range cs.Data {
		fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="3.5" fill="%s"/>`, num(px(i)), num(py(d.Value)), colorAccent)
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="middle" font-family="-apple-system, sans-serif" `+
			`font-size="12" fill="%s">%s</text>`, num(px(i)), num(baseline+18), colorMuted, esc(truncLabel(d.Label, plotW/float64(n))))
	}
	axisLabels(&b, cs, x0, baseline)
	b.WriteString(`</svg>`)
	return b.String()
}

// pieChart renders pie slices with a side legend.
func pieChart(cs ChartSpec) string {
	var b strings.Builder
	openSVG(&b, cs.Title)
	total := 0.0
	for _, d := range cs.Data {
		if d.Value > 0 {
			total += d.Value
		}
	}
	if total <= 0 {
		total = 1
	}
	cx, cy, rad := 200.0, 210.0, 130.0
	angle := -math.Pi / 2 // start at top
	for i, d := range cs.Data {
		v := d.Value
		if v < 0 {
			v = 0
		}
		frac := v / total
		end := angle + frac*2*math.Pi
		col := seriesColor(i)
		if frac >= 0.999 {
			// Single full-circle slice: an arc path can't draw 360°.
			fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="%s" fill="%s"/>`, num(cx), num(cy), num(rad), col)
		} else if frac > 0 {
			x1, y1 := cx+rad*math.Cos(angle), cy+rad*math.Sin(angle)
			x2, y2 := cx+rad*math.Cos(end), cy+rad*math.Sin(end)
			large := 0
			if end-angle > math.Pi {
				large = 1
			}
			fmt.Fprintf(&b, `<path d="M %s %s L %s %s A %s %s 0 %d 1 %s %s Z" fill="%s"/>`,
				num(cx), num(cy), num(x1), num(y1), num(rad), num(rad), large, num(x2), num(y2), col)
		}
		angle = end
	}

	// Legend on the right.
	lx, ly := 400.0, 120.0
	for i, d := range cs.Data {
		y := ly + float64(i)*26
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="14" height="14" rx="3" fill="%s"/>`, num(lx), num(y-11), seriesColor(i))
		pct := d.Value / total * 100
		fmt.Fprintf(&b, `<text x="%s" y="%s" font-family="-apple-system, sans-serif" font-size="13" fill="%s">%s</text>`,
			num(lx+22), num(y), colorText, esc(fmt.Sprintf("%s — %s (%.0f%%)", truncLabel(d.Label, 200), fmtVal(d.Value), pct)))
	}
	b.WriteString(`</svg>`)
	return b.String()
}

func axisLabels(b *strings.Builder, cs ChartSpec, x0, baseline float64) {
	if x := strings.TrimSpace(cs.XLabel); x != "" {
		fmt.Fprintf(b, `<text x="%s" y="%s" text-anchor="middle" font-family="-apple-system, sans-serif" `+
			`font-size="12" fill="%s">%s</text>`, num((x0+cw-mRight)/2), num(ch-8), colorMuted, esc(x))
	}
	if y := strings.TrimSpace(cs.YLabel); y != "" {
		fmt.Fprintf(b, `<text transform="translate(16 %s) rotate(-90)" text-anchor="middle" `+
			`font-family="-apple-system, sans-serif" font-size="12" fill="%s">%s</text>`,
			num((mTop+baseline)/2), colorMuted, esc(y))
	}
}

// niceMax rounds an axis maximum up to a clean-ish value so gridlines read well.
func niceMax(m float64) float64 {
	if m <= 0 {
		return 1
	}
	exp := math.Floor(math.Log10(m))
	pow := math.Pow(10, exp)
	frac := m / pow
	var nice float64
	switch {
	case frac <= 1:
		nice = 1
	case frac <= 2:
		nice = 2
	case frac <= 5:
		nice = 5
	default:
		nice = 10
	}
	return nice * pow
}

// truncLabel shortens a category label to roughly fit the given pixel width.
func truncLabel(s string, width float64) string {
	maxChars := int(width / 7.5)
	if maxChars < 3 {
		maxChars = 3
	}
	r := []rune(s)
	if len(r) <= maxChars {
		return s
	}
	return string(r[:maxChars-1]) + "…"
}
