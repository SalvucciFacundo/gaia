package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gaia/internal/tools/web/search"
)

// WebSearchRunner executes web search queries across supported search providers.
type WebSearchRunner struct {
	searcher        search.Searcher
	defaultProvider string
	providers       search.WebProviders
	providerFactory func(provider string, cfg search.WebConfig) (search.Searcher, error)
}

// NewWebSearchRunner creates an initialized WebSearchRunner.
func NewWebSearchRunner(searcher search.Searcher, defaultProvider string, providers search.WebProviders) *WebSearchRunner {
	if defaultProvider == "" {
		defaultProvider = "duckduckgo"
	}
	return &WebSearchRunner{
		searcher:        searcher,
		defaultProvider: defaultProvider,
		providers:       providers,
		providerFactory: search.NewSearcher,
	}
}

// Name implements the tool identifier.
func (r *WebSearchRunner) Name() string {
	return "web_search"
}

// Description returns the tool capability summary.
func (r *WebSearchRunner) Description() string {
	return "Search the web for real-time information, documentation, news, and technical references. Returns top search results containing title, URL, and snippet."
}

type searchArgs struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
	Provider   string `json:"provider"`
}

func parseSearchArgs(input string) (searchArgs, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return searchArgs{}, fmt.Errorf("search query is required")
	}

	var args searchArgs
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
			return searchArgs{}, fmt.Errorf("invalid json arguments: %w", err)
		}
	} else {
		args.Query = trimmed
	}

	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return searchArgs{}, fmt.Errorf("search query is required")
	}

	if args.MaxResults <= 0 {
		args.MaxResults = 5
	} else if args.MaxResults > 20 {
		args.MaxResults = 20
	}

	return args, nil
}

// Run executes a web search query with optional JSON parameters.
func (r *WebSearchRunner) Run(ctx context.Context, command string) (string, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}

	args, err := parseSearchArgs(command)
	if err != nil {
		return "", err
	}

	searcher := r.searcher
	if args.Provider != "" && strings.ToLower(args.Provider) != strings.ToLower(r.defaultProvider) {
		cfg := search.WebConfig{
			DefaultProvider: args.Provider,
			Providers:       r.providers,
		}
		override, err := r.providerFactory(args.Provider, cfg)
		if err != nil {
			return "", fmt.Errorf("search provider %q error: %w", args.Provider, err)
		}
		searcher = override
	}

	if searcher == nil {
		cfg := search.WebConfig{
			DefaultProvider: r.defaultProvider,
			Providers:       r.providers,
		}
		fallback, err := r.providerFactory(r.defaultProvider, cfg)
		if err != nil {
			return "", fmt.Errorf("failed to initialize default search provider %q: %w", r.defaultProvider, err)
		}
		searcher = fallback
	}

	results, err := searcher.Search(ctx, args.Query, search.SearchOptions{MaxResults: args.MaxResults})
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return "No results found for query: " + args.Query, nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Search results for %q via %s:\n\n", args.Query, searcher.Name()))
	for i, res := range results {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, res.Title))
		b.WriteString(fmt.Sprintf("   URL: %s\n", res.URL))
		if res.Snippet != "" {
			b.WriteString(fmt.Sprintf("   %s\n", res.Snippet))
		}
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n"), nil
}
