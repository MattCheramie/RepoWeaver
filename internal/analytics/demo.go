package analytics

import (
	"context"
	"hash/fnv"
)

// Demo is a deterministic provider that fabricates plausible metrics from each
// slug. It lets the performance dashboard be explored without connecting a real
// Google Analytics property.
type Demo struct{}

// NewDemo returns a Demo provider.
func NewDemo() Demo { return Demo{} }

// Name implements Provider.
func (Demo) Name() string { return "demo" }

// Configured implements Provider.
func (Demo) Configured() bool { return true }

// Report implements Provider, returning deterministic metrics per slug.
func (Demo) Report(_ context.Context, slugs []string) (map[string]Metrics, error) {
	out := make(map[string]Metrics, len(slugs))
	for _, s := range slugs {
		h := fnv.New32a()
		_, _ = h.Write([]byte(s))
		n := h.Sum32()
		out[s] = Metrics{
			Pageviews:     int64(50 + n%950),               // 50..999
			AvgTimeOnPage: float64(30 + (n>>4)%240),        // 30..269s
			BounceRate:    0.20 + float64((n>>8)%50)/100.0, // 0.20..0.69
		}
	}
	return out, nil
}
