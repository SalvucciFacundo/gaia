package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gaia/internal/mneme"
)

// MemorySearchRunner implements searching across Mnemosyne long-term memory.
type MemorySearchRunner struct {
	repo mneme.Store
}

// NewMemorySearchRunner creates a new MemorySearchRunner.
func NewMemorySearchRunner(repo mneme.Store) *MemorySearchRunner {
	return &MemorySearchRunner{repo: repo}
}

// Name returns the tool name.
func (r *MemorySearchRunner) Name() string {
	return "search_memory"
}

// Description returns the tool description.
func (r *MemorySearchRunner) Description() string {
	return `Search long-term memory, past conversations, and observations using hybrid search (FTS5 + vector). Arguments: {"query": "<search query>", "limit": <optional integer>}.`
}

// MemorySearchInput represents the input schema for search_memory.
type MemorySearchInput struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// MemorySearchResultItem represents a single formatted search result item.
type MemorySearchResultItem struct {
	DocType string `json:"doc_type"`
	DocID   string `json:"doc_id,omitempty"`
	Content string `json:"content"`
}

// Run executes the search_memory tool.
func (r *MemorySearchRunner) Run(ctx context.Context, input string) (string, error) {
	if r.repo == nil {
		return "", errors.New("memory repository not available")
	}

	var in MemorySearchInput
	trimmed := strings.TrimSpace(input)
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal([]byte(trimmed), &in); err != nil {
			return "", fmt.Errorf("invalid json input: %w", err)
		}
	} else {
		in.Query = trimmed
	}

	query := strings.TrimSpace(in.Query)
	if query == "" {
		return "", errors.New("search query is required")
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 10
	}

	results, err := r.repo.Search(ctx, query, limit)
	if err != nil {
		return "", fmt.Errorf("memory search failed: %w", err)
	}

	if len(results) == 0 {
		return "No memory records found matching query: " + query, nil
	}

	var formatted []MemorySearchResultItem
	for _, res := range results {
		formatted = append(formatted, MemorySearchResultItem{
			DocType: res.DocType,
			DocID:   res.DocID,
			Content: res.Content,
		})
	}

	out, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		return "", fmt.Errorf("formatting memory search results: %w", err)
	}
	return string(out), nil
}
