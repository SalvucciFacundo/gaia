package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gaia/internal/mneme"
	"gaia/internal/tools/web/fetch"
	"gaia/internal/tools/web/search"
)

func TestWebSearchRunner_EmptyQueryErrors(t *testing.T) {
	runner := NewWebSearchRunner(nil, "duckduckgo", search.WebProviders{})
	_, err := runner.Run(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty search query, got nil")
	}
}

func TestWebSearchRunner_Formatting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<html><body>
			<div class="result web-result">
				<h2 class="result__title"><a class="result__a" href="https://example.com">Example Title</a></h2>
				<a class="result__snippet">Example Snippet</a>
			</div>
		</body></html>`
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	ddg := search.NewDuckDuckGoSearcher(search.WithDuckDuckGoEndpoint(server.URL))
	runner := NewWebSearchRunner(ddg, "duckduckgo", search.WebProviders{})

	out, err := runner.Run(context.Background(), `{"query": "golang", "max_results": 1}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Example Title") || !strings.Contains(out, "https://example.com") {
		t.Errorf("output does not contain expected search result, got:\n%s", out)
	}
}

func TestWebFetchRunner_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body><h1>Hello World</h1><p>Test paragraph.</p></body></html>"))
	}))
	defer server.Close()

	fetcher := fetch.NewFetcher(fetch.FetchOptions{
		AllowPrivateIPs: true,
	})
	runner := NewWebFetchRunner(fetcher, 1024*1024)

	out, err := runner.Run(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "# Hello World") || !strings.Contains(out, "Test paragraph.") {
		t.Errorf("expected markdown content, got:\n%s", out)
	}
}

type fakeSearchStore struct {
	mneme.Store
	results []mneme.SearchResult
}

func (f *fakeSearchStore) Search(ctx context.Context, query string, limit int) ([]mneme.SearchResult, error) {
	return f.results, nil
}

func TestMemorySearchRunner(t *testing.T) {
	store := &fakeSearchStore{
		results: []mneme.SearchResult{
			{DocType: "observation", DocID: "obs-1", Content: "User prefers Go for backend development"},
		},
	}
	runner := NewMemorySearchRunner(store)

	out, err := runner.Run(context.Background(), `{"query": "backend"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "User prefers Go for backend development") {
		t.Errorf("expected search output to contain result, got:\n%s", out)
	}
}
