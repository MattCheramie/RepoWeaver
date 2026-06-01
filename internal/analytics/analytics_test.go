package analytics

import (
	"context"
	"testing"

	"github.com/mattcheramie/repoweaver/internal/config"
)

func TestNewDefaultsToNone(t *testing.T) {
	p := New(config.Config{})
	if p.Configured() {
		t.Fatal("expected unconfigured provider by default")
	}
	if _, err := p.Report(context.Background(), []string{"a"}); err != ErrNotConfigured {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestNewSelectsDemo(t *testing.T) {
	p := New(config.Config{AnalyticsProvider: "demo"})
	if !p.Configured() || p.Name() != "demo" {
		t.Fatalf("expected configured demo, got %s/%v", p.Name(), p.Configured())
	}
}

func TestGA4NotConfiguredWithoutCreds(t *testing.T) {
	p := New(config.Config{AnalyticsProvider: "ga4", GA4PropertyID: "123"})
	if p.Configured() {
		t.Fatal("ga4 should be unconfigured without credentials")
	}
	if _, err := p.Report(context.Background(), []string{"x"}); err != ErrNotConfigured {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestGA4ConfiguredWithInlineCreds(t *testing.T) {
	p := New(config.Config{
		AnalyticsProvider:  "ga4",
		GA4PropertyID:      "123456",
		GA4CredentialsJSON: `{"type":"service_account"}`,
	})
	if !p.Configured() {
		t.Fatal("ga4 should be configured with property + inline creds")
	}
}

func TestDemoIsDeterministic(t *testing.T) {
	d := NewDemo()
	a, _ := d.Report(context.Background(), []string{"my-post", "other"})
	b, _ := d.Report(context.Background(), []string{"my-post", "other"})
	if a["my-post"] != b["my-post"] {
		t.Fatalf("demo metrics not deterministic: %+v vs %+v", a["my-post"], b["my-post"])
	}
	m := a["my-post"]
	if m.Pageviews <= 0 || m.AvgTimeOnPage <= 0 || m.BounceRate <= 0 {
		t.Fatalf("demo metrics look unset: %+v", m)
	}
	if m.BounceRate >= 1 {
		t.Fatalf("bounce rate should be a fraction <1, got %v", m.BounceRate)
	}
}

func TestMapRowsToSlugs(t *testing.T) {
	rr := runReportResponse{}
	rr.Rows = append(rr.Rows, struct {
		DimensionValues []struct {
			Value string `json:"value"`
		} `json:"dimensionValues"`
		MetricValues []struct {
			Value string `json:"value"`
		} `json:"metricValues"`
	}{
		DimensionValues: []struct {
			Value string `json:"value"`
		}{{Value: "/blog/my-post"}},
		MetricValues: []struct {
			Value string `json:"value"`
		}{{Value: "100"}, {Value: "60"}, {Value: "0.4"}},
	})

	out := mapRowsToSlugs(rr, []string{"my-post", "missing"})
	if out["my-post"].Pageviews != 100 {
		t.Fatalf("expected 100 pageviews, got %d", out["my-post"].Pageviews)
	}
	if out["my-post"].AvgTimeOnPage != 60 || out["my-post"].BounceRate != 0.4 {
		t.Fatalf("unexpected metrics: %+v", out["my-post"])
	}
	if out["missing"].Pageviews != 0 {
		t.Fatalf("expected zero for unmatched slug, got %d", out["missing"].Pageviews)
	}
}
