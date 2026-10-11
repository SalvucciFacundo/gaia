package search

import "time"

// WebConfig configures web search providers.
type WebConfig struct {
	DefaultProvider string        `yaml:"default_provider"` // "duckduckgo", "brave", "tavily", "searxng"
	FetchTimeout    time.Duration `yaml:"fetch_timeout"`
	MaxFetchBytes   int64         `yaml:"max_fetch_bytes"`
	UserAgent       string        `yaml:"user_agent"`
	Providers       WebProviders  `yaml:"providers"`
}

// WebProviders configures credentials and endpoints for search providers.
type WebProviders struct {
	BraveAPIKey  string `yaml:"brave_api_key,omitempty"`
	TavilyAPIKey string `yaml:"tavily_api_key,omitempty"`
	SearxngURL   string `yaml:"searxng_url,omitempty"`
}

// GetBraveAPIKey returns the Brave API key.
func (p WebProviders) GetBraveAPIKey() string {
	return p.BraveAPIKey
}

// GetTavilyAPIKey returns the Tavily API key.
func (p WebProviders) GetTavilyAPIKey() string {
	return p.TavilyAPIKey
}

// GetSearxngURL returns the SearXNG BaseURL.
func (p WebProviders) GetSearxngURL() string {
	return p.SearxngURL
}
