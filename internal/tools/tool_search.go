package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ToolMetadata describes searchable summary attributes for a tool.
type ToolMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Backend     string `json:"backend,omitempty"`
}

// ToolSearchRunner implements dynamic tool searching leveraging ToolCatalog.
type ToolSearchRunner struct {
	catalog *ToolCatalog
}

// NewToolSearchRunner creates a ToolSearchRunner backed by a ToolCatalog.
func NewToolSearchRunner(catalog *ToolCatalog) *ToolSearchRunner {
	return &ToolSearchRunner{
		catalog: catalog,
	}
}

// Backend implements tool backend identifier.
func (t *ToolSearchRunner) Backend() string {
	return "internal"
}

// Name implements tool identifier.
func (t *ToolSearchRunner) Name() string {
	return "tool_search"
}

// Description implements tool capability description.
func (t *ToolSearchRunner) Description() string {
	return "Search for available tools by keyword or category when needed. Returns matching tool names and short descriptions."
}

// Run executes the search query and returns matching tool metadata serialized as JSON.
func (t *ToolSearchRunner) Run(ctx context.Context, input string) (string, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}

	if t.catalog == nil {
		return "", errors.New("tool catalog not available")
	}

	var req struct {
		Query    string `json:"query"`
		Category string `json:"category"`
	}

	trimmed := strings.TrimSpace(input)
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal([]byte(trimmed), &req); err != nil {
			return "", fmt.Errorf("parsing search arguments: %w", err)
		}
	} else if trimmed != "" {
		req.Query = trimmed
	} else {
		return "", errors.New("at least one search parameter (query or category) is required")
	}

	query := strings.TrimSpace(req.Query)
	category := strings.TrimSpace(req.Category)

	if query == "" && category == "" {
		return "", errors.New("at least one search parameter (query or category) is required")
	}

	entries := t.catalog.List()
	matches := make([]ToolMetadata, 0)
	queryLower := strings.ToLower(query)
	categoryLower := strings.ToLower(category)

	for _, entry := range entries {
		name := entry.Name
		desc := entry.Description
		cat := entry.Category
		if cat == "" {
			cat = "general"
		}

		// Match category if specified
		if category != "" {
			matchCat := strings.EqualFold(cat, categoryLower) ||
				(categoryLower == "subagents" && (strings.EqualFold(cat, "subagent") || strings.EqualFold(cat, "subagents"))) ||
				(categoryLower == "subagent" && (strings.EqualFold(cat, "subagent") || strings.EqualFold(cat, "subagents")))
			if !matchCat {
				continue
			}
		}

		// Match query if specified
		if query != "" {
			nameMatch := strings.Contains(strings.ToLower(name), queryLower)
			descMatch := strings.Contains(strings.ToLower(desc), queryLower)
			if !nameMatch && !descMatch {
				continue
			}
		}

		matches = append(matches, ToolMetadata{
			Name:        name,
			Description: desc,
			Category:    cat,
			Backend:     cat,
		})
	}

	data, err := json.Marshal(matches)
	if err != nil {
		return "", fmt.Errorf("marshaling search results: %w", err)
	}

	return string(data), nil
}
