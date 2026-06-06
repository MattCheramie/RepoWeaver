// Package visual renders deterministic, self-contained SVG for posts: a header
// hero banner and data charts. Output is plain inline <svg> with colors inlined
// as hex, so it renders in the in-app preview, in exported Markdown, and on
// GitHub/static-site generators with no external stylesheet, script, or image
// hosting. Given the same input, every renderer produces identical output, which
// keeps results stable across regenerations and easy to golden-test.
package visual

import (
	"crypto/sha1"
	"html"
	"math"
	"strconv"
)

// Brand palette mirrors the CSS custom properties in web/static/css/app.css,
// inlined here as hex so generated SVG carries its own colors.
const (
	colorBG      = "#0f1117"
	colorPanel   = "#181b24"
	colorPanel2  = "#1f2330"
	colorBorder  = "#2a2f3d"
	colorText    = "#e6e9ef"
	colorMuted   = "#9aa3b2"
	colorAccent  = "#7c9cff"
	colorAccent2 = "#5b76d6"
	colorGood    = "#3fb950"
	colorWarn    = "#d29922"
	colorBad     = "#f85149"
	colorViolet  = "#a371f7"
)

// series is the ordered categorical palette for multi-datum charts.
var series = []string{colorAccent, colorGood, colorWarn, colorViolet, colorBad, "#3fb0c9", "#d2a8ff", "#56d364"}

// seriesColor returns a stable color for index i.
func seriesColor(i int) string { return series[((i%len(series))+len(series))%len(series)] }

// esc escapes text for safe inclusion in SVG/XML text nodes and attributes.
func esc(s string) string { return html.EscapeString(s) }

// hashSeed derives a deterministic 32-bit seed from s.
func hashSeed(s string) uint32 {
	h := sha1.Sum([]byte(s))
	return uint32(h[0])<<24 | uint32(h[1])<<16 | uint32(h[2])<<8 | uint32(h[3])
}

// rng is a tiny deterministic LCG used to lay out the hero's decorative threads.
type rng struct{ state uint32 }

func newRNG(seed uint32) *rng { return &rng{state: seed | 1} }

func (r *rng) next() uint32 {
	r.state = r.state*1664525 + 1013904223
	return r.state
}

// f returns a deterministic float in [0,1).
func (r *rng) f() float64 { return float64(r.next()) / float64(1<<32) }

// frange returns a deterministic float in [lo,hi).
func (r *rng) frange(lo, hi float64) float64 { return lo + r.f()*(hi-lo) }

// num formats a coordinate/length with at most two decimals, trimming trailing
// zeros to keep SVG output compact and stable.
func num(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	for len(s) > 0 && s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}
	if len(s) > 0 && s[len(s)-1] == '.' {
		s = s[:len(s)-1]
	}
	if s == "-0" {
		return "0"
	}
	return s
}

// fmtVal formats a data value: integers without a decimal point, otherwise two
// decimals.
func fmtVal(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatFloat(v, 'f', 0, 64)
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}
