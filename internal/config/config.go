package config

import (
	"os"
	"strconv"
)

// Config holds runtime configuration sourced from environment variables.
// Sensible defaults let the app run with zero setup using the mock LLM provider.
type Config struct {
	Port        string
	DBPath      string
	GitHubToken string

	// LLMProvider selects the AI backend: "mock" (default, keyless),
	// "claude", "openai", or "gemini".
	LLMProvider string
	LLMAPIKey   string
	LLMModel    string

	// Analytics (Phase 6). AnalyticsProvider: "none" (default), "ga4", or
	// "demo". GA4 needs a property ID and service-account credentials.
	AnalyticsProvider  string
	GA4PropertyID      string
	GA4CredentialsJSON string // inline service-account JSON
	GA4CredentialsFile string // path to service-account JSON

	// OpenBrowser controls whether the server tries to open the default
	// browser on startup.
	OpenBrowser bool
}

// Load reads configuration from the environment, applying defaults.
func Load() Config {
	return Config{
		Port:        envOr("PORT", "8080"),
		DBPath:      envOr("DB_PATH", "./repoweaver.db"),
		GitHubToken: os.Getenv("GITHUB_TOKEN"),
		LLMProvider: envOr("LLM_PROVIDER", "mock"),
		LLMAPIKey:   os.Getenv("LLM_API_KEY"),
		LLMModel:    os.Getenv("LLM_MODEL"),

		AnalyticsProvider:  envOr("ANALYTICS_PROVIDER", "none"),
		GA4PropertyID:      os.Getenv("GA4_PROPERTY_ID"),
		GA4CredentialsJSON: os.Getenv("GA4_CREDENTIALS_JSON"),
		GA4CredentialsFile: os.Getenv("GA4_CREDENTIALS_FILE"),

		OpenBrowser: envBool("OPEN_BROWSER", false),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
