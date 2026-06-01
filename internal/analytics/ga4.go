package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// analyticsReadonlyScope grants read access to the GA4 Data API.
const analyticsReadonlyScope = "https://www.googleapis.com/auth/analytics.readonly"

// GA4 pulls performance metrics from a Google Analytics 4 property via the
// Analytics Data API (runReport). Authentication uses a service-account
// credentials JSON, supplied either inline or by file path.
type GA4 struct {
	propertyID string
	credsJSON  []byte
}

// NewGA4 constructs a GA4 provider. credsInline takes precedence over
// credsFile; if neither yields credentials the provider reports unconfigured.
func NewGA4(propertyID, credsInline, credsFile string) *GA4 {
	var creds []byte
	if strings.TrimSpace(credsInline) != "" {
		creds = []byte(credsInline)
	} else if credsFile != "" {
		if data, err := os.ReadFile(credsFile); err == nil {
			creds = data
		}
	}
	return &GA4{propertyID: strings.TrimSpace(propertyID), credsJSON: creds}
}

// Name implements Provider.
func (g *GA4) Name() string { return "ga4(property " + g.propertyID + ")" }

// Configured implements Provider.
func (g *GA4) Configured() bool { return g.propertyID != "" && len(g.credsJSON) > 0 }

// runReportRequest is the GA4 Data API request body.
type runReportRequest struct {
	Dimensions []dimension `json:"dimensions"`
	Metrics    []metric    `json:"metrics"`
	DateRanges []dateRange `json:"dateRanges"`
}

type dimension struct {
	Name string `json:"name"`
}
type metric struct {
	Name string `json:"name"`
}
type dateRange struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type runReportResponse struct {
	Rows []struct {
		DimensionValues []struct {
			Value string `json:"value"`
		} `json:"dimensionValues"`
		MetricValues []struct {
			Value string `json:"value"`
		} `json:"metricValues"`
	} `json:"rows"`
}

// Report implements Provider using service-account credentials.
func (g *GA4) Report(ctx context.Context, slugs []string) (map[string]Metrics, error) {
	if !g.Configured() {
		return nil, ErrNotConfigured
	}
	creds, err := google.CredentialsFromJSON(ctx, g.credsJSON, analyticsReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("ga4 credentials: %w", err)
	}
	return runReport(ctx, creds.TokenSource, g.propertyID, slugs)
}

// runReport calls the GA4 Data API runReport endpoint with the given token
// source and maps the resulting rows onto the requested slugs. It is shared by
// the service-account and OAuth providers.
func runReport(ctx context.Context, ts oauth2.TokenSource, propertyID string, slugs []string) (map[string]Metrics, error) {
	reqBody := runReportRequest{
		Dimensions: []dimension{{Name: "pagePath"}},
		Metrics: []metric{
			{Name: "screenPageViews"},
			{Name: "averageSessionDuration"},
			{Name: "bounceRate"},
		},
		DateRanges: []dateRange{{StartDate: "28daysAgo", EndDate: "today"}},
	}
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := "https://analyticsdata.googleapis.com/v1beta/properties/" +
		propertyID + ":runReport"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if ts != nil {
		tok, err := ts.Token()
		if err != nil {
			return nil, fmt.Errorf("ga4 token: %w", err)
		}
		tok.SetAuthHeader(httpReq)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ga4 runReport failed (%d): %s", resp.StatusCode, string(data))
	}

	var rr runReportResponse
	if err := json.Unmarshal(data, &rr); err != nil {
		return nil, err
	}
	return mapRowsToSlugs(rr, slugs), nil
}

// accumulator tracks weighted sums so rates can be averaged by pageviews.
type accumulator struct {
	views     int64
	timeSum   float64
	bounceSum float64
}

// mapRowsToSlugs aggregates report rows onto the requested slugs by substring
// match on the page path, weighting rate metrics by pageviews.
func mapRowsToSlugs(rr runReportResponse, slugs []string) map[string]Metrics {
	acc := make(map[string]*accumulator, len(slugs))
	for _, s := range slugs {
		acc[s] = &accumulator{}
	}
	for _, row := range rr.Rows {
		if len(row.DimensionValues) == 0 || len(row.MetricValues) < 3 {
			continue
		}
		path := row.DimensionValues[0].Value
		views := parseInt(row.MetricValues[0].Value)
		avgTime := parseFloat(row.MetricValues[1].Value)
		bounce := parseFloat(row.MetricValues[2].Value)
		for _, s := range slugs {
			if s != "" && strings.Contains(path, s) {
				a := acc[s]
				a.views += views
				a.timeSum += avgTime * float64(views)
				a.bounceSum += bounce * float64(views)
			}
		}
	}

	out := make(map[string]Metrics, len(slugs))
	for s, a := range acc {
		m := Metrics{Pageviews: a.views}
		if a.views > 0 {
			m.AvgTimeOnPage = a.timeSum / float64(a.views)
			m.BounceRate = a.bounceSum / float64(a.views)
		}
		out[s] = m
	}
	return out
}

func parseInt(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}
