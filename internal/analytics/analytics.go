// Package analytics is "the Monitor": it pulls post-publication performance
// metrics (pageviews, average time on page, bounce rate) for generated content
// from an analytics backend. Google Analytics 4 is the primary provider; a
// deterministic demo provider and a not-configured no-op are also available.
package analytics

import (
	"context"
	"errors"
	"strings"

	"github.com/mattcheramie/repoweaver/internal/config"
)

// Metrics holds performance figures for a single page.
type Metrics struct {
	Pageviews     int64
	AvgTimeOnPage float64 // seconds
	BounceRate    float64 // 0..1
}

// Provider is the contract every analytics backend implements.
type Provider interface {
	// Name returns a human-readable provider identifier.
	Name() string
	// Configured reports whether the provider can serve real data.
	Configured() bool
	// Report returns metrics keyed by slug for the requested slugs.
	Report(ctx context.Context, slugs []string) (map[string]Metrics, error)
}

// ErrNotConfigured is returned when analytics has not been set up.
var ErrNotConfigured = errors.New("analytics provider is not configured")

// New constructs a Provider from configuration. Defaults to a not-configured
// no-op so the dashboard renders a setup prompt instead of failing.
func New(cfg config.Config) Provider {
	switch strings.ToLower(cfg.AnalyticsProvider) {
	case "ga4", "google":
		return NewGA4(cfg.GA4PropertyID, cfg.GA4CredentialsJSON, cfg.GA4CredentialsFile)
	case "demo", "mock":
		return NewDemo()
	default:
		return None{}
	}
}

// None is a provider that is never configured.
type None struct{}

// Name implements Provider.
func (None) Name() string { return "not configured" }

// Configured implements Provider.
func (None) Configured() bool { return false }

// Report implements Provider.
func (None) Report(context.Context, []string) (map[string]Metrics, error) {
	return nil, ErrNotConfigured
}
